package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/schemes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
)

// This small tutorial attempts to show resourceVersion (RV) conflict checking
// for both Patch and Update.
//
// Both have the same behavior where user can opt-in or opt-out of the conflict
// checking based on the body of the request.
//
// We also have a quick look at how Update overwrites everything, and so is not
// very safe for concurrent updates without conflict checking. In comparison,
// Patch is safe for concurrent updates without conflict checking.
//
// In general, (imo), devs should probably favor patches when concurrent clients
// try to update the same object (but different fields). In that case, using patch,
// and opt-out of RV conflict checking will allow controllers to run without the
// need to retry on conflict (and constantly fetch the latest state from k8s)
//
// See below for more details.
func main() {
	scheme := runtime.NewScheme()
	utilruntime.Must(schemes.AddToScheme(scheme))

	restCfg, err := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG")).ClientConfig()
	must(err)

	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restCfg, scheme, nil)
	must(err)

	opts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}

	core, err := core.NewFactoryFromConfigWithOptions(restCfg, opts)
	must(err)

	configMapCtrl := core.Core().V1().ConfigMap()
	// Look at tutorialPatch's comment for more info
	tutorialPatch(configMapCtrl)
	fmt.Println("")
	// Look at tutorialUpdate's comment for more info
	tutorialUpdate(configMapCtrl)
}

// tutorialPatch shows the following:
//
//  1. Calling Patch with a patch that has a resourceVersion will opt in to
//     conflict checking.
//
//     If resourceVersion is most up-to-date, then it succeeds. Otherwise, the
//     Patch call fails with a conflict error.
//
//  2. Calling Patch with a patch that DOES NOT have a resourceVersion will
//     opt out of conflict checking.
//
//     In that case, the Patch call can never fail due to resourceVersion conflict.
//
//  3. A successful Patch only affect fields in the patch. There are different rules based on
//     the type of patch, etc. But it allows us to have multiple controllers patch fields
//     concurrently (as long as they touch different fields)
//
//     Because of this, we do not need retry.RetryOnConflict since it's really easy AND SAFE
//     to create a patch that opt-out of conflict and doesn't lose information. (compared to update)
func tutorialPatch(client generic.ClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList]) {
	fmt.Println("Patch tutorial")
	// Setup code where we create a new ConfigMap named "patch"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "patch",
			Namespace: "default",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}

	err := client.Delete(cm.Namespace, cm.Name, &metav1.DeleteOptions{})
	must(ignoreNotFound(err))

	// Note that we received an updated ConfigMap in the response. This ConfigMap will
	// have a .metadata.resourceVersion (RV) defined.
	cm2, err := client.Create(cm)
	must(err)
	fmt.Printf("ConfigMap created with rv=%s, new rv=%s\n", cm.ResourceVersion, cm2.ResourceVersion)

	cm2.Data["hello"] = "world"

	// We create a patch here. The patch will contain a RV because `cm` doesn't have one
	// but `cm2` does.
	patch, err := makePatch(cm, cm2)
	must(err)
	cm3, err := client.Patch(cm2.Namespace, cm2.Name, types.MergePatchType, patch)
	must(err)
	fmt.Printf("ConfigMap patched new rv=%s,data=%v\n", cm3.ResourceVersion, cm3.Data)

	// If we try to create a patch again using the previous ConfigMap, we will
	// get a conflict because cm2 has a stale RV
	patch, err = makePatch(cm, cm2)
	must(err)
	_, err = client.Patch(cm2.Namespace, cm2.Name, types.MergePatchType, patch)
	mustBeConflict(err)
	fmt.Printf("ConfigMap not patched due to conflict, stale rv=%s\n", cm2.ResourceVersion)

	// Similar to Update, we can opt-out of conflict checking of RV with Patch
	// by making sure the resourceVersion is NOT in the patch. There are a few
	// ways of doing so.
	//
	// The simplest imo is to DeepCopy the version you have and modify it. It doesn't matter
	// if the version is stale. The example uses cm2 which has a stale version as an example.
	//
	// This has the benefit of making the patch very small AND we can avoid
	// one or multiple round trip to k8s to get the latest version.
	modified := cm2.DeepCopy()
	modified.Data["hello"] = "toto"
	patch, err = makePatch(cm2, modified)
	must(err)
	cm4, err := client.Patch(cm2.Namespace, cm2.Name, types.MergePatchType, patch)
	must(err)
	fmt.Printf("ConfigMap patched new rv=%s,data=%v\n", cm4.ResourceVersion, cm4.Data)
}

// tutorialUpdate shows the following:
//
//  1. Calling Update with an object that has a resourceVersion will opt in to
//     conflict checking.
//
//     If resourceVersion is most up-to-date, then it succeeds. Otherwise, the
//     Update call fails with a conflict error.
//
//  2. Calling Update with an object that DOES NOT have a resourceVersion will
//     opt out of conflict checking.
//
//     In that case, the Update call can never fail due to resourceVersion conflict.
//
//  3. A successful Update overrides the whole object. So if there's some field missing,
//     then it's possible to lose information (field validation/manager helps but this
//     tutorial doesn't go into that).
//
//  4. Using retry.RetryOnConflict to retry updates
func tutorialUpdate(client generic.ClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList]) {
	fmt.Println("Update tutorial")
	// Setup code where we create a new ConfigMap named "update"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "update",
			Namespace: "default",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}

	err := client.Delete(cm.Namespace, cm.Name, &metav1.DeleteOptions{})
	must(ignoreNotFound(err))

	// Note that we received an updated ConfigMap in the response. This ConfigMap will
	// have a .metadata.resourceVersion (RV) defined.
	cm2, err := client.Create(cm)
	must(err)
	fmt.Printf("ConfigMap created with rv=%s, new rv=%s %v\n", cm.ResourceVersion, cm2.ResourceVersion, cm2.Data)

	cm2.Data["baz"] = "toto"

	// We update the ConfigMap by modifying the ConfigMap we received from the Create call. This
	// ConfigMap has the most up-to-date RV, so no conflict is expected.
	//
	// We again receive an updated ConfigMap in the response. It has a new RV.
	cm3, err := client.Update(cm2)
	must(err)
	fmt.Printf("ConfigMap updated with rv=%s, new rv=%s %v\n", cm2.ResourceVersion, cm3.ResourceVersion, cm3.Data)

	cm2.Data = map[string]string{
		"foo": "bar",
	}

	// Trying to update with an old ConfigMap with a stale RV will fail with a conflict error:
	//
	//     Operation cannot be fulfilled on configmaps "update": the object has been modified; please apply your changes to the latest version and try again
	_, err = client.Update(cm2)
	mustBeConflict(err)
	fmt.Printf("ConfigMap not updated due to conflict, stale rv=%s\n", cm2.ResourceVersion)

	// We can opt-out of this RV conflict check simply by sending an update with no RV defined
	cm2.SetResourceVersion("")

	cm4, err := client.Update(cm2)
	must(err)
	// Notice that we completely overwrote the .data field, which is why it can be
	// dangerous to opt-out of resourceVersion checking.
	fmt.Printf("ConfigMap updated with rv=%s, new rv=%s %v\n", cm2.ResourceVersion, cm4.ResourceVersion, cm4.Data)

	// One trick to opt-in resourceVersion checking but be a bit more robust
	// to conflict errors is to retry like this
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err = client.Get(cm.Namespace, cm.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		// Do the change here
		cm.Data["hello"] = "world"

		// This Update call can fail if another client updates the ConfigMap
		// between the Get and the Update
		cm, err = client.Update(cm)
		if err != nil {
			return err
		}

		return nil
	})
	must(err)
}

func makePatch(original, modified *corev1.ConfigMap) ([]byte, error) {
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}
	modifiedJSON, err := json.Marshal(modified)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)
	if err != nil {
		return nil, err
	}

	fmt.Printf("making patch with original rv=%s,data=%v modified rv=%s,data=%v\nPatch: %s\n", original.ResourceVersion, original.Data, modified.ResourceVersion, modified.Data, string(patch))
	return patch, nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func mustBeConflict(err error) {
	if apierrors.IsConflict(err) {
		return
	}

	if err != nil {
		panic(err)
	}

	panic(errors.New("expected conflict, got no error"))
}

func ignoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
