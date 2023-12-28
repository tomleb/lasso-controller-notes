package main

import (
	"log/slog"
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
)

func main() {
	singleAndSimple()
	coalescing()
	concurrency()
}

// runWorker is a simplified worker that process items. The lasso library works
// similarly. You can look at this piece of code which shows how lasso uses the
// workqueue:
// https://github.com/rancher/lasso/blob/d684fdeb6f29e221289c6f59fdc390b6adbaaccf/pkg/controller/controller.go#L183-#L233
//
// Note: Lasso uses a rate limiting workqueue instead of the simpler workqueue.
// This is not really relevant here.
func runWorker(group *sync.WaitGroup, name string, wq workqueue.Interface, fn func()) {
	slog.Info("Starting worker", "name", name)
	for {
		// Get key from the queue. This doesn't include "processing" keys,
		// which are keys that are in the queue but that are currently being
		// processed. This prevents concurrent processing of the same key.
		key, shutdown := wq.Get()
		if shutdown {
			break
		}

		if fn != nil {
			fn()
		}
		slog.Info("Processed", "name", name, "key", key)
		// Signal to the queue that we're done with this key. If that key was
		// added to the queue while we were processing it, it can finally go
		// back to the queue, ready to be worked on by an available worker.
		wq.Done(key)
	}
	slog.Info("Stopping worker", "name", name)
	group.Done()
}

// This example shows a single worker running on 3 keys that are added to the
// queue.
//
// The 3 keys will be processed sequentially, in the order that they appear in
// the queue.
func singleAndSimple() {
	slog.Info("=== Single and simple ===")
	var group sync.WaitGroup
	group.Add(1)

	wq := workqueue.New()
	wq.Add("key-1")
	wq.Add("key-2")
	wq.Add("key-3")
	go runWorker(&group, "1", wq, nil)

	wq.ShutDownWithDrain()
	group.Wait()
	slog.Info("===                   ===")
}

// This example shows the coalescing property of the workqueue.
//
// The same key added multiple times will coalesce into a single key in the
// queue, so the handler will only run once.
//
// This also applies to keys that are currently being processed. Adding the key
// to the queue will add it only once in the queue.
func coalescing() {
	// Example 1
	{
		slog.Info("=== Coalescing 1 ===")
		var group sync.WaitGroup
		group.Add(1)

		wq := workqueue.New()
		wq.Add("key-1")
		wq.Add("key-1")
		wq.Add("key-1")
		go runWorker(&group, "1", wq, nil)

		wq.ShutDownWithDrain()
		group.Wait()
		slog.Info("===            ===")
	}

	// Example 2 where we add the same key while that key is being processed.
	// The key will be processed twice, not 3 times.
	{
		slog.Info("=== Coalescing 2 ===")
		var group sync.WaitGroup
		group.Add(1)
		workCh := make(chan struct{}, 2)

		wq := workqueue.New()
		wq.Add("key-1")
		go runWorker(&group, "1", wq, func() {
			workCh <- struct{}{}
			time.Sleep(1 * time.Second)
		})

		<-workCh
		// We add twice key-1 to the queue while a worker is processing key-1.
		// key-1 will be processed only once more, not twice.
		wq.Add("key-1")
		wq.Add("key-1")

		wq.ShutDownWithDrain()
		group.Wait()
		slog.Info("===            ===")
	}
}

// This example demonstrates the concurrency capabilities of the workqueue.
//
// Three workers are started to work on items in the queue. 3 keys are added to
// the workqueue. This function takes a total of 4 seconds instead of 3*4=12
// seconds to run because all 3 workers are working concurrently.
func concurrency() {
	slog.Info("=== Concurrency ===")
	var group sync.WaitGroup
	group.Add(3)

	wq := workqueue.New()
	wq.Add("key-1")
	wq.Add("key-2")
	wq.Add("key-3")

	go runWorker(&group, "1", wq, func() {
		time.Sleep(4 * time.Second)
	})
	go runWorker(&group, "2", wq, func() {
		time.Sleep(4 * time.Second)
	})
	go runWorker(&group, "3", wq, func() {
		time.Sleep(4 * time.Second)
	})

	wq.ShutDownWithDrain()
	group.Wait()
	slog.Info("===             ===")
}
