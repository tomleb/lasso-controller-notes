package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rancher/steve/pkg/stores/proxyalpha/tablelistconvert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

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

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	must(err)

	setTable := func(rt http.RoundTripper) http.RoundTripper {
		return &addQuery{
			values: map[string]string{
				"includeObject": "Object",
			},
			next: rt,
		}
	}

	tableClientCfg := rest.CopyConfig(config)
	tableClientCfg.Wrap(setTable)
	// tableClientCfg.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io"
	dynClient, err := dynamic.NewForConfig(tableClientCfg)
	must(err)

	resInt := dynClient.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	})

	table := tablelistconvert.Client{ResourceInterface: resInt}
	stuff, err := table.List(context.Background(), metav1.ListOptions{})
	must(err)

	bytes, err := json.Marshal(stuff)
	must(err)

	fmt.Println(string(bytes))

}
