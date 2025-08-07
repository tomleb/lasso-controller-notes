package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/controller"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	corev1.AddToScheme(scheme)

	// Get the kubeconfig from your environment variable
	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	opts := &controller.SharedControllerFactoryOptions{
		CacheOptions: &cache.SharedCacheFactoryOptions{
			KindTweakList: map[schema.GroupVersionKind]cache.TweakListOptionsFunc{
				corev1.SchemeGroupVersion.WithKind("ConfigMap"): func(opts *metav1.ListOptions) {
					opts.LabelSelector = "foo=bar"
					// Can filter by namespace and name too
					opts.FieldSelector = "metadata.namespace=default"
				},
			},
		},
	}
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

	log.Println("Started controller")
	coreCtrl.Core().V1().ConfigMap().OnChange(ctx, "before-start", func(key string, obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
		items, err := coreCtrl.Core().V1().ConfigMap().Cache().List(corev1.NamespaceAll, labels.Everything())
		fmt.Println(len(items), err)
		return obj, nil
	})
	controllerFactory.Start(ctx, 1)
	<-ctx.Done()
	return nil
}
