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

	store := &reflectorStore{}

	reflectorOpts := cache.ReflectorOptions{}
	reflector := cache.NewReflectorWithOptions(lw, nil, store, reflectorOpts)
	reflector.RunWithContext(ctx)
}
