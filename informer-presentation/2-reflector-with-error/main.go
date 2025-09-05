package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func toString(obj interface{}) string {
	acc, err := meta.Accessor(obj)
	must(err)
	return fmt.Sprintf("%s/%s (%s)", acc.GetNamespace(), acc.GetName(), acc.GetResourceVersion())
}

type reflectorStore struct {
}

func (r *reflectorStore) Add(obj interface{}) error {
	fmt.Println("Received ADD", toString(obj))
	return nil
}

func (r *reflectorStore) Update(obj interface{}) error {
	fmt.Println("Received Update", toString(obj))
	return nil
}

func (r *reflectorStore) Delete(obj interface{}) error {
	fmt.Println("Received Delete", toString(obj))
	return nil
}

func (r *reflectorStore) Replace(items []interface{}, initialList string) error {
	fmt.Println("Received Replace for")
	for _, item := range items {
		fmt.Println("-", toString(item))
	}
	return nil
}

func (r *reflectorStore) Resync() error {
	fmt.Println("Received Resync")
	return nil
}

// Optional interface, not implemented by the Queue (FIFO, DeltaFIFO, RealFIFO)
func (r *reflectorStore) UpdateResourceVersion(resourceVersion string) {
	fmt.Println("New RV", resourceVersion)
}

type failingWatcher struct {
	w  watch.Interface
	ch chan watch.Event
}

func (w *failingWatcher) Stop() {
	w.w.Stop()
	// Drain events
	for range w.ch {
	}
}

func (w *failingWatcher) ResultChan() <-chan watch.Event {
	return w.ch
}

func (w *failingWatcher) run() {
	defer close(w.ch)
	i := 0
	for event := range w.w.ResultChan() {
		if event.Type == watch.Bookmark {
			continue
		}

		if i == 1 {
			w.ch <- watch.Event{
				Type: watch.Error,
			}
			w.w.Stop()
			return
		}
		w.ch <- event
		i += 1
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	restConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	must(err)

	dynClient, err := dynamic.NewForConfig(restConfig)
	must(err)

	gvr := v1.SchemeGroupVersion.WithResource("configmaps")
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			fmt.Println("Listing", options)
			return dynClient.Resource(gvr).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			fmt.Println("Watching", options)
			watcher, err := dynClient.Resource(gvr).Watch(ctx, options)
			if err != nil {
				return nil, err
			}
			failing := &failingWatcher{
				w:  watcher,
				ch: make(chan watch.Event),
			}
			go failing.run()
			return failing, err
		},
	}

	store := &reflectorStore{}

	reflectorOpts := cache.ReflectorOptions{}
	reflector := cache.NewReflectorWithOptions(lw, nil, store, reflectorOpts)
	reflector.RunWithContext(ctx)
}
