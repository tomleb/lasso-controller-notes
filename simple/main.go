package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	// corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

// This example shows the basic of running a controller for ConfigMap that will
// print the namespace/name of each ConfigMap in the cluster.
func mainErr() error {
	scheme := runtime.NewScheme()
	ctx := context.Background()
	// corev1.AddToScheme(scheme)

	// Get the kubeconfig from your environment variable
	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	// Creates a Shared Controller Factory. What?
	//
	// 	Controller: A _thing_ that can react to k8s object changes.
	// 	Factory: Creates things, in this case, creates controllers.
	// 	Shared: The controllers that are created share resources (eg: caches)
	//
	// Oh, so it's just an object to create controllers that will share a cache,
	// okay.
	opts := &controller.SharedControllerFactoryOptions{}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restConfig, scheme, opts)
	if err != nil {
		return err
	}

	// There's many way to create controllers, one of them is with
	// .ForResourceKind() method.
	// In this case, we create (or get) a controller for ConfigMaps.
	cmGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	cmCtrl := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)

	// Now that we have a controller, we can register handlers. Ignoring a
	// few details for now, handlers are called whenever the object is
	// added/modified/deleted(?) in the cache.
	//
	// In this case, we will simply print out the key of this ConfigMap,
	// which is the namespace/name field.
	cmCtrl.RegisterHandler(ctx, "configmap-controller", controller.SharedControllerHandlerFunc(func(key string, _ runtime.Object) (runtime.Object, error) {
		log.Println("Received key", key)
		return nil, nil
	}))

	log.Println("Starting controller factory")
	// Once all handlers are registered, we can start the controller factory.
	// This will:
	// 1. Start all the caches (one per GVK)
	// 2. Wait for all of them to sync successfully (have latest from k8s)
	// 3. Start all the controllers (making the registered handlers ready to run)
	controllerFactory.Start(ctx, 1)

	log.Println("Started controller factory")

	time.Sleep(10 * time.Second)

	return nil
}
