package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type sqlStore struct {
	lock    sync.Mutex
	items   map[string]any
	keyFunc cache.KeyFunc
}

func newSqlStore() *sqlStore {
	return &sqlStore{
		items:   make(map[string]any),
		keyFunc: cache.DeletionHandlingMetaNamespaceKeyFunc,
	}
}

func (s *sqlStore) Update(obj any) error {
	key, err := s.keyFunc(obj)
	if err != nil {
		return cache.KeyError{Obj: obj, Err: err}
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.items[key] = obj
	return nil
}

func (s *sqlStore) Delete(obj any) error {
	key, err := s.keyFunc(obj)
	if err != nil {
		return cache.KeyError{Obj: obj, Err: err}
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	if obj, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		delete(s.items, obj.Key)
	} else {
		delete(s.items, key)
	}
	return nil
}

func (s *sqlStore) ListKeys() []string {
	s.lock.Lock()
	defer s.lock.Unlock()
	keys := []string{}
	for key := range s.items {
		keys = append(keys, key)
	}
	return keys
}

func (s *sqlStore) GetByKey(key string) (value interface{}, exists bool, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	obj, exists := s.items[key]
	if !exists {
		return nil, false, nil
	}
	return obj, true, nil
}

func runSQLController(dynClient dynamic.ResourceInterface, store *sqlStore) {
	opts := cache.DeltaFIFOOptions{
		KnownObjects:          store,
		EmitDeltaTypeReplaced: true,
	}
	fifo := cache.NewDeltaFIFOWithOptions(opts)
	defer fifo.Close()

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			obj, err := dynClient.List(context.TODO(), options)
			return obj, err
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			watcher, err := dynClient.Watch(context.TODO(), options)
			return watcher, err
		},
	}
	var cm v1.ConfigMapList
	reflectorOpts := cache.ReflectorOptions{}
	reflector := cache.NewReflectorWithOptions(lw, cm, fifo, reflectorOpts)
	stopCh := make(chan struct{}, 1)
	go func() {
		reflector.Run(stopCh)
		fifo.Close()
	}()

	for {
		processDeltas := func(deltas cache.Deltas) error {
			for _, delta := range deltas {
				switch delta.Type {
				case cache.Added, cache.Replaced, cache.Sync, cache.Updated:
					if err := store.Update(delta.Object); err != nil {
						return err
					}
				case cache.Deleted:
					if err := store.Delete(delta.Object); err != nil {
						return err
					}
				}
			}
			return nil
		}
		_, err := fifo.Pop(func(obj interface{}, isInInitialList bool) error {
			if deltas, ok := obj.(cache.Deltas); ok {
				return processDeltas(deltas)
			}
			return fmt.Errorf("not deltas")
		})
		if err != nil {
			if err == cache.ErrFIFOClosed {
				return
			}
			// XXX: Handle it
			log.Fatal(err)
		}
	}
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	must(err)

	dynClient, err := dynamic.NewForConfig(config)
	must(err)

	store := newSqlStore()

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	go runSQLController(dynClient.Resource(gvr), store)
	for {
		fmt.Println(store.ListKeys())
		time.Sleep(time.Second)
	}

}
