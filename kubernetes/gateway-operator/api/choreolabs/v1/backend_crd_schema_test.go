/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

const backendExample = `
apiVersion: gateway.choreolabs.com/v1
kind: Backend
metadata:
  name: orders-eu
  labels:
    team: orders
spec:
  displayName: Orders (EU)
  basePath: /v1
  timeout:
    connect: 2s
  endpoints:
    - url: https://orders-eu-1:8443
      weight: 70
    - url: https://orders-eu-2:8443
  tls:
    identity: gateway-orders
    trustedCAs: [orders-ca]
    verifyHostName: true
`

// The generated Backend CRD accepts the gateway's Backend shape and rejects what the gateway
// would reject: a name outside the Backend name rule, no endpoints, a non-HTTP URL, a weight
// over 100, a malformed base path.
func TestBackendCRD_Validation(t *testing.T) {
	schemaValidator, celValidator := backendValidators(t)
	validate := func(obj map[string]interface{}) field.ErrorList {
		errs := validation.ValidateCustomResource(nil, obj, schemaValidator)
		celErrs, _ := celValidator.Validate(context.Background(), nil, nil, obj, nil, celconfig.RuntimeCELCostBudget)
		return append(errs, celErrs...)
	}
	example := func() map[string]interface{} {
		obj := map[string]interface{}{}
		if err := yaml.Unmarshal([]byte(backendExample), &obj); err != nil {
			t.Fatalf("unmarshal example: %v", err)
		}
		return obj
	}
	spec := func(obj map[string]interface{}) map[string]interface{} { return obj["spec"].(map[string]interface{}) }

	if errs := validate(example()); len(errs) != 0 {
		t.Fatalf("the example was rejected: %v", errs.ToAggregate())
	}

	cases := []struct {
		name    string
		mutate  func(obj map[string]interface{})
		wantErr string
	}{
		{"upper-case name", func(o map[string]interface{}) { o["metadata"].(map[string]interface{})["name"] = "Orders-EU" }, "metadata.name must be"},
		{"name with a dot", func(o map[string]interface{}) { o["metadata"].(map[string]interface{})["name"] = "orders.eu" }, "metadata.name must be"},
		{"name too short", func(o map[string]interface{}) { o["metadata"].(map[string]interface{})["name"] = "ab" }, "metadata.name must be"},
		{"name too long", func(o map[string]interface{}) {
			o["metadata"].(map[string]interface{})["name"] = strings.Repeat("a", 41)
		}, "metadata.name must be"},
		{"spec missing", func(o map[string]interface{}) { delete(o, "spec") }, "spec: Required value"},
		{"no endpoints", func(o map[string]interface{}) { spec(o)["endpoints"] = []interface{}{} }, "spec.endpoints"},
		{"endpoints missing", func(o map[string]interface{}) { delete(spec(o), "endpoints") }, "spec.endpoints"},
		{"non-http url", func(o map[string]interface{}) {
			spec(o)["endpoints"] = []interface{}{map[string]interface{}{"url": "ftp://orders:21"}}
		}, "spec.endpoints[0].url"},
		{"weight over 100", func(o map[string]interface{}) {
			spec(o)["endpoints"] = []interface{}{map[string]interface{}{"url": "http://orders:80", "weight": int64(101)}}
		}, "spec.endpoints[0].weight"},
		{"base path with a trailing slash", func(o map[string]interface{}) { spec(o)["basePath"] = "/v1/" }, "spec.basePath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := example()
			tc.mutate(obj)
			errs := validate(obj)
			if len(errs) == 0 {
				t.Fatalf("the CRD accepted an invalid Backend")
			}
			if !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
				t.Errorf("rejected, but not for %q: %v", tc.wantErr, errs.ToAggregate())
			}
		})
	}
}

// backendValidators builds the schema and CEL validators of the generated Backend CRD's only
// version.
func backendValidators(t *testing.T) (validation.SchemaValidator, *cel.Validator) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	// api/choreolabs/v1/<thisfile> -> ../../../config/crd/bases/...
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "config", "crd", "bases",
		"gateway.choreolabs.com_backends.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run `make manifests`?): %v", path, err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Name != "v1" {
		t.Fatalf("%s should serve v1 only", crd.Name)
	}
	internal := &apiextensions.JSONSchemaProps{}
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema, internal, nil); err != nil {
		t.Fatalf("convert schema: %v", err)
	}
	schemaValidator, _, err := validation.NewSchemaValidator(internal)
	if err != nil {
		t.Fatalf("build schema validator: %v", err)
	}
	structural, err := structuralschema.NewStructural(internal)
	if err != nil {
		t.Fatalf("build structural schema: %v", err)
	}
	return schemaValidator, cel.NewValidator(structural, true, celconfig.PerCallLimit)
}
