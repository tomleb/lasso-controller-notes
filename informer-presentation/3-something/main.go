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
			return dynClient.Resource(gvr).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return dynClient.Resource(gvr).Watch(ctx, options)
		},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	fifo := cache.NewRealFIFO(cache.MetaNamespaceKeyFunc, indexer, nil)
	defer fifo.Close()

	go func() {
		for {
			// Essentially replicating s.HandleDeltas
			// https://github.com/kubernetes/client-go/blob/v0.33.0/tools/cache/controller.go#L564-L580
			_, err := fifo.Pop(func(obj interface{}, isInInitialList bool) error {
				deltas, ok := obj.(cache.Deltas)
				if !ok {
					return fmt.Errorf("not a deltas")
				}
				for _, delta := range deltas {
					fmt.Println(delta.Type, toString(delta.Object))
					switch delta.Type {
					case cache.Added, cache.Updated, cache.Replaced:
						_, exists, err := indexer.Get(delta.Object)
						must(err)
						if exists {
							indexer.Update(delta.Object)
							fmt.Println("Updated in indexer")
						} else {
							indexer.Add(delta.Object)
							fmt.Println("Added in indexer")
						}
					case cache.Deleted:
						indexer.Delete(delta.Object)
						fmt.Println("Deleted in indexer")
					}
				}
				return nil
			})
			if err != nil {
				if err == cache.ErrFIFOClosed {
					return
				}
			}
		}
	}()

	reflectorOpts := cache.ReflectorOptions{}
	reflector := cache.NewReflectorWithOptions(lw, nil, fifo, reflectorOpts)
	reflector.RunWithContext(ctx)

}
