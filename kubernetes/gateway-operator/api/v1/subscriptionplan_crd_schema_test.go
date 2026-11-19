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
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

const subscriptionPlanWithConditions = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: SubscriptionPlan
metadata:
  name: gold
spec:
  planName: Gold
  stopOnQuotaReach: true
  conditions:
    - id: requests
      type: request-count
      params:
        limits:
          - count: 1000
            timeAmount: 1
            timeUnit: HOUR
        onExceed: reject
    - type: token-count
      params:
        limits:
          - count: 50000
            timeUnit: DAY
        tokenType: total
    - id: region
      type: custom
      params:
        zone: eu
`

// TestSubscriptionPlanCRDConditions checks the conditions schema and CEL rules
// on every served version, as the API server would on create.
func TestSubscriptionPlanCRDConditions(t *testing.T) {
	cases := map[string]struct {
		mutate  func(spec map[string]interface{})
		wantErr string
	}{
		"conditions accepted": {mutate: func(map[string]interface{}) {}},
		"legacy fields only accepted": {mutate: func(spec map[string]interface{}) {
			delete(spec, "conditions")
			spec["throttleLimitCount"] = int64(10)
			spec["throttleLimitUnit"] = "Min"
		}},
		"conditions with throttleLimitCount rejected": {
			mutate:  func(spec map[string]interface{}) { spec["throttleLimitCount"] = int64(10) },
			wantErr: "not both",
		},
		"conditions with throttleLimitUnit rejected": {
			mutate:  func(spec map[string]interface{}) { spec["throttleLimitUnit"] = "Min" },
			wantErr: "not both",
		},
		"unknown type rejected": {
			mutate:  func(spec map[string]interface{}) { conditionAt(spec, 0)["type"] = "byte-count" },
			wantErr: "Unsupported value",
		},
		"custom without id rejected": {
			mutate:  func(spec map[string]interface{}) { delete(conditionAt(spec, 2), "id") },
			wantErr: "a custom condition needs an id",
		},
		"missing params rejected": {
			mutate:  func(spec map[string]interface{}) { delete(conditionAt(spec, 0), "params") },
			wantErr: "Required value",
		},
		"more than 20 conditions rejected": {
			mutate: func(spec map[string]interface{}) {
				list := make([]interface{}, 21)
				for i := range list {
					list[i] = map[string]interface{}{"id": "c", "type": "custom", "params": map[string]interface{}{}}
				}
				spec["conditions"] = list
			},
			wantErr: "Too many",
		},
	}

	crd := subscriptionPlanCRD(t)
	for _, v := range crd.Spec.Versions {
		internal := &apiextensions.JSONSchemaProps{}
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v.Schema.OpenAPIV3Schema, internal, nil); err != nil {
			t.Fatalf("convert %q schema: %v", v.Name, err)
		}
		schemaValidator, _, err := validation.NewSchemaValidator(internal)
		if err != nil {
			t.Fatalf("build validator for %q: %v", v.Name, err)
		}
		structural, err := structuralschema.NewStructural(internal)
		if err != nil {
			t.Fatalf("build structural schema for %q: %v", v.Name, err)
		}
		celValidator := cel.NewValidator(structural, true, celconfig.PerCallLimit)

		for name, tc := range cases {
			t.Run(v.Name+"/"+name, func(t *testing.T) {
				obj := map[string]interface{}{}
				if err := yaml.Unmarshal([]byte(subscriptionPlanWithConditions), &obj); err != nil {
					t.Fatalf("unmarshal example: %v", err)
				}
				tc.mutate(obj["spec"].(map[string]interface{}))

				errs := validation.ValidateCustomResource(nil, obj, schemaValidator)
				celErrs, _ := celValidator.Validate(t.Context(), nil, structural, obj, nil, celconfig.RuntimeCELCostBudget)
				errs = append(errs, celErrs...)
				if tc.wantErr == "" {
					if len(errs) > 0 {
						t.Fatalf("rejected: %v", errs.ToAggregate())
					}
					return
				}
				if len(errs) == 0 || !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, errs.ToAggregate())
				}
			})
		}
	}
}

func conditionAt(spec map[string]interface{}, i int) map[string]interface{} {
	return spec["conditions"].([]interface{})[i].(map[string]interface{})
}

// subscriptionPlanCRD loads the generated SubscriptionPlan CRD.
func subscriptionPlanCRD(t *testing.T) apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "crd", "bases",
		"gateway.api-platform.wso2.com_subscriptionplans.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run `make manifests`?): %v", path, err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return crd
}
