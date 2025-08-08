package main

//go:generate go run pkg/codegen/main.go

import (
	"fmt"
	"log"
	"os"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	examplev1 "github.com/tomleb/lasso-controller-notes/pkg/apis/example.com/v1"
	wexample "github.com/tomleb/lasso-controller-notes/pkg/generated/controllers/example.com"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	examplev1.AddToScheme(scheme)

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

	exampleCtrl, err := wexample.NewFactoryFromConfigWithOptions(restConfig, genOpts)
	if err != nil {
		return err
	}

	// Foo is able to use UpdateStatus because it has a status subresource
	foo := examplev1.Foo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Status: examplev1.FooStatus{
			Foo: "initial",
		},
	}
	exampleCtrl.Example().V1().Foo().Delete(foo.Namespace, foo.Name, &metav1.DeleteOptions{})
	respFoo, err := exampleCtrl.Example().V1().Foo().Create(&foo)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	respFoo.Status.Foo = "updated"
	_, err = exampleCtrl.Example().V1().Foo().UpdateStatus(respFoo)
	fmt.Println("Updating status of Foo, error is:", err)

	// Bar is NOT able to use UpdateStatus because it does NOT have a status subresource
	bar := examplev1.Bar{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
		Status: examplev1.BarStatus{
			Bar: "initial",
		},
	}
	exampleCtrl.Example().V1().Bar().Delete(bar.Namespace, bar.Name, &metav1.DeleteOptions{})
	respBar, err := exampleCtrl.Example().V1().Bar().Create(&bar)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	respBar.Status.Bar = "updated"
	_, err = exampleCtrl.Example().V1().Bar().UpdateStatus(respBar)
	fmt.Println("Updating status of Bar, error is:", err)

	return nil
}
