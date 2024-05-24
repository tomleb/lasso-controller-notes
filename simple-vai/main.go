package main

import (
	"fmt"
	"log"
	"os"

	db2 "github.com/rancher/lasso/pkg/cache/sql/db"
	"github.com/rancher/lasso/pkg/cache/sql/encryption"
	"github.com/rancher/lasso/pkg/cache/sql/informer"
	_ "github.com/rancher/lasso/pkg/cache/sql/informer/factory"
	"github.com/rancher/wrangler/pkg/kubeconfig"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// This example shows the basic of running a controller for ConfigMap that will
// print the namespace/name of each ConfigMap in the cluster.
func mainErr() error {
	// Get the kubeconfig from your environment variable
	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	must(err)

	dynClient, err := dynamic.NewForConfig(restConfig)
	must(err)

	m, err := encryption.NewManager()
	must(err)

	dbClient, err := db2.NewClient(nil, m, m)
	must(err)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	client := dynClient.Resource(gvr)

	informer, err := informer.NewInformer(client, [][]string{}, gvk, dbClient, false, true)
	must(err)

	stopCh := make(chan struct{})
	fmt.Println("Starting informer")
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Println(obj)
		},
	})
	informer.Run(stopCh)
	<-stopCh

	return nil
}
