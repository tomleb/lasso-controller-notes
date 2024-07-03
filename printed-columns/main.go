package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/resources/common"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func main() {
	if err := mainErr(); err != nil {
		slog.Error("Exiting", "err", err)
		os.Exit(1)
	}
}

// We show how to add multiple handlers to ConfigMaps with a controller factory.
func mainErr() error {
	ctx := context.Background()

	// Get the kubeconfig from your environment variable
	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	colsClient, err := common.NewDynamicColumns(restConfig)
	if err != nil {
		return err
	}

	schema := types.APISchema{
		Schema: &schemas.Schema{
			ID: "apigroup",
			Attributes: map[string]interface{}{
				"group":    "",
				"version":  "v1",
				"resource": "configmaps",
			},
		},
	}
	colsClient.SetColumns(ctx, &schema)

	raw1, err := json.Marshal(attributes.Columns(&schema))
	if err != nil {
		return err
	}
	fmt.Println(string(raw1))

	gvr := attributes.GVR(&schema)

	withObj, err := getDynClient(restConfig, true)
	if err != nil {
		return err
	}

	unstructured, err := withObj.Resource(gvr).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	raw2, err := json.Marshal(unstructured)
	if err != nil {
		return err
	}
	fmt.Println(string(raw2))

	withoutObj, err := getDynClient(restConfig, false)
	if err != nil {
		return err
	}

	unstructured2, err := withoutObj.Resource(gvr).Namespace("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	raw3, err := json.Marshal(unstructured2)
	if err != nil {
		return err
	}
	fmt.Println(string(raw3))

	return nil
}

func getDynClient(restConfig *rest.Config, includeObj bool) (*dynamic.DynamicClient, error) {
	setTable := func(includeObj bool) func(rt http.RoundTripper) http.RoundTripper {
		val := "None"
		if includeObj {
			val = "Object"
		}
		return func(rt http.RoundTripper) http.RoundTripper {
			return &addQuery{
				values: map[string]string{
					"includeObject": val,
				},
				next: rt,
			}
		}
	}

	tableClientCfg := rest.CopyConfig(restConfig)
	tableClientCfg.Wrap(setTable(includeObj))
	tableClientCfg.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io"
	return dynamic.NewForConfig(tableClientCfg)
}

// Taken as-is from steve
type addQuery struct {
	values map[string]string
	next   http.RoundTripper
}

func (a *addQuery) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	for k, v := range a.values {
		q.Set(k, v)
	}
	req.Header.Set("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io")
	req.URL.RawQuery = q.Encode()
	return a.next.RoundTrip(req)
}
