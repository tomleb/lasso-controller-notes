package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

// This example shows the behavior when registering new handlers AFTER a controller
// has been started.
func mainErr() error {
	scheme := runtime.NewScheme()
	ctx := context.Background()
	corev1.AddToScheme(scheme)

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

	genOpts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}

	coreCtrl, err := wcore.NewFactoryFromConfigWithOptions(restConfig, genOpts)
	if err != nil {
		return err
	}

	coreCtrl.Core().V1().ConfigMap().OnChange(ctx, "before-start", func(key string, obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
		log.Println("Received key", key)
		return obj, nil
	})

	log.Println("Starting controller factory")
	// Start will:
	// 1. Start all the caches (one per GVK)
	// 2. Wait for all of them to sync successfully (have latest from k8s)
	// 3. Start all the controllers (making the registered handlers ready to run)
	controllerFactory.Start(ctx, 1)

	log.Println("Started controller factory")

	time.Sleep(10 * time.Second)

	// We can add more handlers AFTER having started the controller factory.
	// In this case, since the ConfigMap controller is already started, the new
	// handlers will be ran right away. One caveat is that ALL handlers registered for
	// ConfigMap will re-run.
	log.Println("Registering new ConfigMap handler")
	coreCtrl.Core().V1().ConfigMap().OnChange(ctx, "after-start", func(key string, obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
		log.Println("New sync func key", key)
		return obj, nil
	})

	time.Sleep(10 * time.Second)

	log.Println("Registering new Secret handler")
	// Registering a controller on Secret will not automatically start running the handlers because at this point
	// the Secret controller never started.
	coreCtrl.Core().V1().Secret().OnChange(ctx, "secret-after-start", func(key string, obj *corev1.Secret) (*corev1.Secret, error) {
		log.Println("New sync func key for secret", key)
		return obj, nil
	})
	// We must therefore start the controller AGAIN. This will ensure any new
	// GVK registered will be started. (We could also do it per object with coreCtrl.Start() instead.)
	controllerFactory.Start(ctx, 1)

	time.Sleep(10 * time.Second)

	return nil
}
