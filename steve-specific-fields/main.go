package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type SteveConfigMap struct {
	*v1.ConfigMap
	customMetadataState string
}

func configMapToSteveConfigMap(obj any) (any, error) {
	configMap := obj.(*v1.ConfigMap)
	customState := fmt.Sprintf("Custom state for %s", configMap.GetName())
	return &SteveConfigMap{
		ConfigMap:           configMap,
		customMetadataState: customState,
	}, nil
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	must(err)

	client, err := typedv1.NewForConfig(config)
	must(err)

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			obj, err := client.ConfigMaps("").List(context.TODO(), options)
			return obj, err
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			watcher, err := client.ConfigMaps("").Watch(context.TODO(), options)
			return watcher, err
		},
	}
	eventHandler := cache.ResourceEventHandlerFuncs{}
	indexers := cache.Indexers{
		"steve-custom-state": func(obj any) ([]string, error) {
			steveCM := obj.(*SteveConfigMap)
			return []string{
				steveCM.customMetadataState,
			}, nil
		},
	}
	indexer, ctrl := cache.NewTransformingIndexerInformer(lw, &SteveConfigMap{}, 0, eventHandler, indexers, configMapToSteveConfigMap)

	stopCh := make(chan struct{}, 1)
	go ctrl.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, ctrl.HasSynced) {
		log.Fatal("Nope")
	}

	for {
		kubeRootCAs, err := indexer.ByIndex("steve-custom-state", "Custom state for kube-root-ca.crt")
		must(err)
		for _, obj := range kubeRootCAs {
			steveCM := obj.(*SteveConfigMap)
			fmt.Println("Found", steveCM.GetName())
		}
		time.Sleep(time.Second * 5)
	}
}
