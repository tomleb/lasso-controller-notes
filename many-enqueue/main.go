package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"k8s.io/apimachinery/pkg/api/meta"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

// This example demonstrates the different ways handlers will be called.
//
// Handlers are driven by a workqueue. The controller will pull a key from the
// workqueue and run the handlers, passing the key and the object of the key if
// any, to the handlers.
//
// There are 3 ways that keys are enqueued:
//  1. When the cache is synced or resynced. The resync behavior is explained
//     below.
//  2. When receiving object changes from k8s (via the Watch API). This is the
//     most common way keys are enqueued.
//  3. Manually, via one of .Enqueue(), .EnqueueKey() or .EnqueueAfter(). Also
//     explained below.
func mainErr() error {
	scheme := runtime.NewScheme()
	ctx := context.Background()

	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	opts := &controller.SharedControllerFactoryOptions{
		CacheOptions: &cache.SharedCacheFactoryOptions{
			// 1. Cache sync and resync
			//
			// This configures the cache resync period. The cache will
			// re-enqueue all keys in all workqueue, causing all handlers to
			// run again for each keys. In this example, you should see
			// "default/kube-root-ca.crt" reappear every 5 seconds.
			//
			// This value can also be set via the environment variable
			// CATTLE_RESYNC_DEFAULT.
			//
			// By default, Rancher specifies neither of those so the value is 0
			// which means there isn't any resync done.
			DefaultResync: 5 * time.Second,
		},
	}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restConfig, scheme, opts)
	if err != nil {
		return err
	}

	cmGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	// 2. k8s Watch API
	//
	// This will use the k8s Watch API when the controller starts. All object
	// changes will overwrite the cache AND will add the object's key to the
	// workqueue for this Shard Controller.
	cmCtrl := controllerFactory.ForResourceKind(cmGVR, "ConfigMap", true)

	// 3. Manual enqueue
	//
	// We can manually enqueue with the following methods.
	//
	// Why would one do this? Well, sometimes you want to trigger sync
	// functions based on events external to k8s. For example, you might be
	// using netlink to subscribe to network events on the local machine, etc.
	//
	// Note: You will see here that the object passed to the handler is nil for
	// the manually enqueued keys. This is just because the keys my-ns/enqueue,
	// my-ns/enqueue-key and my-ns/enqueue-after don't exist in our cluster, so
	// they are not in the cache.

	// .Enqueue() enqueues the key <namespace>/<name>
	cmCtrl.Enqueue("my-ns", "enqueue")
	// .EnqueueKey() also enqueues the key <namespace>/<name>
	cmCtrl.EnqueueKey("my-ns/enqueue-key")
	// .EnqueueAfter() will enqueue <namespace/<name> after the duration given.
	// In this case, my-ns/enqueue-after will be enqueued in 4 seconds. The 4
	// second timer _seem_ to start after the cache has been synced.
	cmCtrl.EnqueueAfter("my-ns", "enqueue-after", 4*time.Second)

	// The handlers are only started when the cache has been synced. This means
	// it's fine to enqueue keys before all handlers are registered.
	cmCtrl.RegisterHandler(ctx, "configmap-controller", controller.SharedControllerHandlerFunc(func(key string, obj runtime.Object) (runtime.Object, error) {
		if obj != nil {
			metadata, err := meta.Accessor(obj)
			if err != nil {
				return nil, err
			}
			if metadata.GetNamespace() != "default" {
				return nil, nil
			}
			log.Println("Received key", key)
			return nil, nil
		}

		log.Println("Received key", key, obj)
		return nil, nil
	}))

	// The secret Shared Controller below is just an example to show that
	// enqueuing in the ConfigMap controller doesn't affect the queue for the
	// Secret controller.
	secretGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}
	secretCtrl := controllerFactory.ForResourceKind(secretGVR, "Secret", true)
	secretCtrl.RegisterHandler(ctx, "secret-controller", controller.SharedControllerHandlerFunc(func(_ string, obj runtime.Object) (runtime.Object, error) {
		if obj != nil {
			return nil, nil
		}
		panic("unexpected key")
	}))

	log.Println("Starting controller factory")
	controllerFactory.Start(ctx, 1)
	log.Println("Started controller factory")

	time.Sleep(20 * time.Second)

	return nil
}
