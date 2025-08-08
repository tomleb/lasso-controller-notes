// This program generates the code for the Rancher types and clients.
package main

import (
	"os"

	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	examplev1 "github.com/tomleb/lasso-controller-notes/pkg/apis/example.com/v1"
)

func main() {
	os.Unsetenv("GOPATH")

	controllergen.Run(args.Options{
		OutputPackage: "github.com/tomleb/lasso-controller-notes/pkg/generated",
		Boilerplate:   "boilerplate.go.txt",
		Groups: map[string]args.Group{
			"example.com": {
				PackageName: "example.com",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					examplev1.Foo{},
					examplev1.Bar{},
				},
				GenerateTypes: true,
			},
		},
	})
}
