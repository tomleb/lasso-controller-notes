package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/schemes"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var Scheme = runtime.NewScheme()

func init() {
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(schemes.AddToScheme(Scheme))

}

func main() {
	restConfig, err := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG")).ClientConfig()
	must(err)

	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restConfig, Scheme, nil)
	must(err)

	opts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}

	rbac, err := rbac.NewFactoryFromConfigWithOptions(restConfig, opts)
	must(err)

	ctx := context.Background()

	Register(ctx, rbac)

	fmt.Println("SharedCacheFactory.Start")
	err = controllerFactory.SharedCacheFactory().Start(ctx)
	must(err)

	fmt.Println("WaitForCacheSync")
	controllerFactory.SharedCacheFactory().WaitForCacheSync(ctx)

	fmt.Println("ControllerFactory.Start")
	err = controllerFactory.Start(ctx, 10)
	must(err)

	<-ctx.Done()
}

func Register(ctx context.Context, rbac *rbac.Factory) {
	rbac.Rbac().V1().ClusterRoleBinding().OnRemove(ctx, "custom-on-remove", func(_ string, crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
		// ErrSkip will prevent the finalizer to be removed so the CRB will not be deleted, just try to delete any CRB => it won't work
		fmt.Println("Always skipping")
		return nil, generic.ErrSkip
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
