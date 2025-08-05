package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	wapiextensions "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io"
	wapiextensionsv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io/v1"
	wapps "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps"
	wappsv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func main() {
	if err := mainErr(); err != nil {
		log.Fatal(err)
	}
}

type Step1Context struct {
	ConfigMap wcorev1.ConfigMapController
	CRD       wapiextensionsv1.CustomResourceDefinitionController

	Completed chan struct{}
}

func RegisterStep1(ctx context.Context, ctrlContext Step1Context) error {
	var once sync.Once
	wanted := map[string]bool{
		"foos.test.io": false,
		"bars.test.io": false,
	}
	var lock sync.Mutex

	ctrlContext.ConfigMap.OnChange(ctx, "step-1-configmap", func(key string, obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
		log.Println("Received configmap step 1 key", key)
		return obj, nil
	})
	ctrlContext.CRD.OnChange(ctx, "step-1-configmap", func(key string, obj *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
		lock.Lock()
		defer lock.Unlock()

		if _, ok := wanted[key]; ok {
			wanted[key] = true
		}

		gotAll := true
		for _, targetGot := range wanted {
			if !targetGot {
				gotAll = false
				break
			}
		}
		if gotAll {
			once.Do(func() {
				close(ctrlContext.Completed)
			})
		}

		return obj, nil
	})

	return nil
}

type Step2Context struct {
	ConfigMap  wcorev1.ConfigMapController
	Deployment wappsv1.DeploymentController

	Completed chan struct{}
}

func RegisterStep2(ctx context.Context, ctrlContext Step2Context) error {
	var once sync.Once

	replicas := int32(2)
	if _, err := ctrlContext.Deployment.Create(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.14.2",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}); err != nil {
		return err
	}

	ctrlContext.ConfigMap.OnChange(ctx, "step-2-configmap", func(key string, obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
		log.Println("Received configmap step 2 key", key)
		return obj, nil
	})
	ctrlContext.Deployment.OnChange(ctx, "step-2-deployments", func(key string, obj *appsv1.Deployment) (*appsv1.Deployment, error) {
		if obj == nil || obj.Namespace != "default" || obj.Name != "nginx" {
			return obj, nil
		}

		str, ready, err := DeploymentStatus(obj)
		if err != nil {
			return obj, err
		}
		log.Println(str)

		if ready {
			once.Do(func() {
				close(ctrlContext.Completed)
			})
		}
		return obj, nil
	})

	return nil
}

type Step3Context struct {
	ConfigMap  wcorev1.ConfigMapController
	Secret     wcorev1.SecretController
	Deployment wappsv1.DeploymentController

	Completed chan struct{}
}

func RegisterStep3(ctx context.Context, ctrlContext Step3Context) error {
	if _, err := ctrlContext.Secret.Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"bar": []byte("toto"),
		},
	}); err != nil {
		return err
	}
	log.Println("Created Secret!")
	close(ctrlContext.Completed)
	return nil
}

// This example shows how to have initialization done in locksteps:
// 1. Step 1 waits for both foo.yaml and bar.yaml to be applied (eg: CAPI case)
// 2. Step 2 waits for nginx deployment to be ready
// 3. Step 3 creates a secret now that foo/bars CRDs exist and "webhook" (nginx) is ready
func mainErr() error {
	scheme := runtime.NewScheme()
	ctx := context.Background()
	corev1.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)
	apiextensionsv1.AddToScheme(scheme)

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
	apiExtensionsCtrl, err := wapiextensions.NewFactoryFromConfigWithOptions(restConfig, genOpts)
	if err != nil {
		return err
	}
	appsCtrl, err := wapps.NewFactoryFromConfigWithOptions(restConfig, genOpts)
	if err != nil {
		return err
	}

	// Step 1 would be things with no dependencies like.... Settings
	ctrlStep1 := Step1Context{
		ConfigMap: coreCtrl.Core().V1().ConfigMap(),
		CRD:       apiExtensionsCtrl.Apiextensions().V1().CustomResourceDefinition(),
		Completed: make(chan struct{}, 1),
	}
	ctxStep1 := controller.NewHandlerTransaction(ctx)
	log.Println("Registering Step 1")
	err = RegisterStep1(ctxStep1, ctrlStep1)
	if err != nil {
		ctxStep1.Rollback()
		return err
	}
	log.Println("Starting controller factory for step 1")
	ctxStep1.Commit()
	controllerFactory.Start(ctx, 1)

	log.Println("Waiting for step 1 to be completed")
	<-ctrlStep1.Completed

	// Step 2 has access to things initialized in step 1.... Like.. I don't know.. Secrets?
	ctrlStep2 := Step2Context{
		ConfigMap:  coreCtrl.Core().V1().ConfigMap(),
		Deployment: appsCtrl.Apps().V1().Deployment(),
		Completed:  make(chan struct{}, 1),
	}
	ctxStep2 := controller.NewHandlerTransaction(ctx)
	log.Println("Registering Step 2")
	err = RegisterStep2(ctxStep2, ctrlStep2)
	if err != nil {
		ctxStep2.Rollback()
		return err
	}
	log.Println("Starting controller factory for step 2")
	ctxStep2.Commit()
	controllerFactory.Start(ctx, 1)
	<-ctrlStep2.Completed

	ctrlStep3 := Step3Context{
		ConfigMap:  coreCtrl.Core().V1().ConfigMap(),
		Secret:     coreCtrl.Core().V1().Secret(),
		Deployment: appsCtrl.Apps().V1().Deployment(),
		Completed:  make(chan struct{}, 1),
	}
	ctxStep3 := controller.NewHandlerTransaction(ctx)
	log.Println("Registering Step 3")
	err = RegisterStep3(ctxStep3, ctrlStep3)
	if err != nil {
		ctxStep3.Rollback()
		return err
	}
	log.Println("Starting controller factory for step 3")
	ctxStep3.Commit()
	controllerFactory.Start(ctx, 1)
	<-ctrlStep3.Completed

	time.Sleep(10 * time.Second)

	return nil
}

// Taken from kubectl codebase
func DeploymentStatus(obj *appsv1.Deployment) (string, bool, error) {
	if obj.Generation <= obj.Status.ObservedGeneration {
		cond := GetDeploymentCondition(obj.Status, appsv1.DeploymentProgressing)
		if cond != nil && cond.Reason == timedOutReason {
			return "", false, fmt.Errorf("deployment %q exceeded its progress deadline", obj.Name)
		}
		if obj.Spec.Replicas != nil && obj.Status.UpdatedReplicas < *obj.Spec.Replicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated...\n", obj.Name, obj.Status.UpdatedReplicas, *obj.Spec.Replicas), false, nil
		}
		if obj.Status.Replicas > obj.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination...\n", obj.Name, obj.Status.Replicas-obj.Status.UpdatedReplicas), false, nil
		}
		if obj.Status.AvailableReplicas < obj.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available...\n", obj.Name, obj.Status.AvailableReplicas, obj.Status.UpdatedReplicas), false, nil
		}
		return fmt.Sprintf("deployment %q successfully rolled out\n", obj.Name), true, nil
	}
	return fmt.Sprintf("Waiting for deployment spec update to be observed...\n"), false, nil
}

var timedOutReason = "ProgressDeadlineExceeded"

func GetDeploymentCondition(status appsv1.DeploymentStatus, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}
