package main

import (
	"context"
	"log/slog"
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
		slog.Error("Exiting", "err", err)
		os.Exit(1)
	}
}

// We show how to add multiple handlers to ConfigMaps with a controller factory.
func mainErr() error {
	scheme := runtime.NewScheme()
	ctx := context.Background()

	// Get the kubeconfig from your environment variable
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
	// The first call to .ForResourceKind() creates the shared controller for
	// this ConfigMap object.
	cmCtrl := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)
	// The second call to .ForResourceKind() will get the same shared
	// controller. We will register the 2nd handler with this object.
	cmCtrl2 := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)

	// We can register multiple handlers for a shared controller by calling
	// .RegisterHandler() multiple times.
	//
	// For example, we are registering 3 handlers. We'll get to managing
	// concurrency later but in this specific configuration, each handlers will
	// run sequentially.
	//
	// When running this example, you should see something like:
	//
	// 2023/12/27 15:56:27 INFO Handler key=default/kube-root-ca.crt factory=1 controller=configmap-controller-1
	// <sleep 1 second>
	// 2023/12/27 15:56:27 INFO Handler key=default/kube-root-ca.crt factory=2 controller=configmap-controller-2
	// <sleep 2 second>
	// 2023/12/27 15:56:27 INFO Handler key=default/kube-root-ca.crt factory=1 controller=configmap-controller-3
	// <sleep 3 second>
	//
	// You can see that each handler runs sequentially and in order of
	// registration.
	cmCtrl.RegisterHandler(ctx, "configmap-controller-1", controller.SharedControllerHandlerFunc(func(key string, _ runtime.Object) (runtime.Object, error) {
		slog.Info("Handler", "key", key, "factory", "1", "controller", "configmap-controller-1")
		time.Sleep(1 * time.Second)
		return nil, nil
	}))

	cmCtrl2.RegisterHandler(ctx, "configmap-controller-2", controller.SharedControllerHandlerFunc(func(key string, _ runtime.Object) (runtime.Object, error) {
		slog.Info("Handler", "key", key, "factory", "2", "controller", "configmap-controller-2")
		time.Sleep(2 * time.Second)
		return nil, nil
	}))

	cmCtrl.RegisterHandler(ctx, "configmap-controller-3", controller.SharedControllerHandlerFunc(func(key string, _ runtime.Object) (runtime.Object, error) {
		slog.Info("Handler", "key", key, "factory", "1", "controller", "configmap-controller-3")
		time.Sleep(3 * time.Second)
		return nil, nil
	}))

	slog.Info("Starting controller factory")
	controllerFactory.Start(ctx, 1)

	slog.Info("Started controller factory")

	time.Sleep(10 * time.Second)

	return nil
}
