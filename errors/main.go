package main

import (
	"context"
	"fmt"
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

// This example shows how controllers react when an error occurs in a sync
// function.
func mainErr() error {
	scheme := runtime.NewScheme()
	ctx := context.Background()

	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	opts := &controller.SharedControllerFactoryOptions{}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restConfig, scheme, opts)
	if err != nil {
		return err
	}

	cmGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	// The first controller will be returning an error for the
	// kube-system/kube-root-ca.crt ConfigMap.
	//
	// Upon error in ANY handlers, the shared controller will re-enqueue the key
	// so that all handlers (sync functions) run again.
	//
	// The re-enqueue mechanism uses a rate limiter with exponential backoff.
	// You can see the timestamps in the logs while running this example, the
	// time doubles every time an error occurs.
	cmCtrl := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)
	cmCtrl.RegisterHandler(ctx, "configmap-controller-1", controller.SharedControllerHandlerFunc(func(key string, obj runtime.Object) (runtime.Object, error) {
		if key != "kube-system/kube-root-ca.crt" {
			return obj, nil
		}
		return obj, fmt.Errorf("fake error from controller 1 for kube-system/kube-root-ca.crt")
	}))

	// Since the controller above always fails, this second controller will
	// re-run and re-run and re-run, etc until ALL handlers succeed. This is
	// why it is important for the sync function to be idempotent - it might
	// run multiple times for reasons external to it.
	//
	// Note handlers share the same rate limiter, so multiple errors from
	// another handler can be problematic if a handler is time-sensitive (eg:
	// Think of a handler that must run within 2 seconds of an update, for
	// example)
	cm2Ctrl := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)
	cm2Ctrl.RegisterHandler(ctx, "configmap-controller-2", controller.SharedControllerHandlerFunc(func(key string, obj runtime.Object) (runtime.Object, error) {
		if key != "kube-system/kube-root-ca.crt" {
			return nil, nil
		}
		fmt.Println("Success from controller 2 for kube-system/kube-root-ca.crt")
		return obj, nil
	}))

	log.Println("Starting controller factory")
	controllerFactory.Start(ctx, 1)

	log.Println("Started controller factory")

	time.Sleep(5 * time.Minute)

	return nil
}
