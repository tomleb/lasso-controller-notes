package main

import (
	"context"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

// This example shows how to decode runtime.Object with the k8s schemes and why
// it's important.
func mainErr() error {
	ctx := context.Background()

	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	// runtime.NewScheme creates a scheme, which basically allow registering
	// encoding/decoding functions for k8s objects (eg: json->ConfigMap,
	// ConfigMap->json, etc)
	scheme := runtime.NewScheme()

	//
	// Uncomment the following line so that the program is able to successfully
	// decode ConfigMaps into *corev1.ConfigMap objects.
	//
	// This adds all of core v1 objects to the scheme, essentially allowing us
	// to decode/encode ConfigMaps, Secrets, etc. But this doesn't register
	// other objects like apps/v1, management.cattle.io/v3, etc
	//
	//corev1.AddToScheme(scheme)

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
	cmCtrl := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)
	cmCtrl.RegisterHandler(ctx, "configmap-controller", controller.SharedControllerHandlerFunc(func(_ string, obj runtime.Object) (runtime.Object, error) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			// Without uncommenting the corev1.AddToScheme line above, the
			// program will crash.
			log.Fatal("Unable to decode configmap object. It's type is ", reflect.TypeOf(obj), ". Add ConfigMap to scheme by editing the main.go file.")
		}

		// If you uncomment the corev1.AddToScheme line above, then the app won't
		// crash and you'll list all the ConfigMaps objects.
		log.Println("Received ConfigMap", cm.Name)
		return nil, nil
	}))

	log.Println("Starting controller factory")
	controllerFactory.Start(ctx, 1)
	log.Println("Started controller factory")

	time.Sleep(10 * time.Second)

	return nil
}
