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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

// budgetWorkedExample is a Budget exercising every field the gateway takes: labels, a
// per-agent split, a provider filter, an attribute filter, exemptions of each type and
// an early block point.
const budgetWorkedExample = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: online-shop-agent-daily
spec:
  labels:
    - finance
    - agents
  status: active
  limit:
    amount: "25.00"
    period: day
    action: block
    blockAt: 90
    splitBy:
      type: agent
  filters:
    - type: llmProvider
      id: mock-openai
    - type: project
      id: online-shop
    - type: attribute
      expression: request.Auth.Claims["email"].endsWith("@wso2.com")
  exemptions:
    - type: agent
      project: online-shop
      id: returns-bot
    - type: project
      id: internal-tools
    - type: attribute
      expression: request.Auth.Claims["email"] == "alice@example.com"
`

// budgetControlPlaneExample is a control plane Budget manifest: it carries displayName,
// description and warnings, which the CRD must accept so the manifest applies unchanged.
const budgetControlPlaneExample = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: org-daily-ceiling
spec:
  displayName: Organization daily ceiling
  description: All LLM spend
  limit:
    amount: "15.00"
    period: day
    action: allow
  warnings:
    thresholds: [80, 100]
    channels: [finance-webhook]
`

// TestBudgetCRDAcceptsExamples asserts the generated Budget CRD schema accepts a worked
// example, a minimal budget and a control plane manifest.
//
// The CRD type and the gateway-controller management-API schema are separately
// maintained representations of one resource with no compile-time link, so this is the
// guard that a marker added on one side does not reject a budget the other side accepts.
func TestBudgetCRDAcceptsExamples(t *testing.T) {
	cases := map[string]string{
		"worked example":     budgetWorkedExample,
		"control plane kind": budgetControlPlaneExample,
		"minimal": `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Budget
metadata:
  name: minimal
spec:
  limit:
    amount: "0.01"
    period: month
    action: allow
`,
	}
	validators := budgetSchemaValidators(t)
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			obj := map[string]interface{}{}
			if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for version, validator := range validators {
				if errs := validator.validate(obj); len(errs) > 0 {
					t.Fatalf("%s schema rejected %q: %v", version, name, errs.ToAggregate())
				}
			}
		})
	}
}

// TestBudgetCRDAcceptsEveryAmountThePatternAllows pins the amount pattern to the
// management API's: up to 12 whole digits and 2 decimals, no leading zeros, above zero.
func TestBudgetCRDAcceptsEveryAmountThePatternAllows(t *testing.T) {
	validators := budgetSchemaValidators(t)
	for _, amount := range []string{"1", "100", "100.5", "100.00", "0.01", "0.1", "0.99", "999999999999.99"} {
		obj := budgetWorkedExampleObject(t)
		budgetLimit(obj)["amount"] = amount
		for version, validator := range validators {
			if errs := validator.validate(obj); len(errs) > 0 {
				t.Errorf("%s schema rejected amount %q: %v", version, amount, errs.ToAggregate())
			}
		}
	}
}

// TestBudgetCRDRejectsInvalidSpecs asserts the constraints mirrored from the management
// API are present in the generated CRD: a marker silently dropped in generation would
// move the error from `kubectl apply` to a status condition.
func TestBudgetCRDRejectsInvalidSpecs(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(spec map[string]interface{})
		wantErr string
	}{
		{"limit missing", func(s map[string]interface{}) { delete(s, "limit") }, "limit"},
		{"amount missing", func(s map[string]interface{}) { delete(budgetLimit2(s), "amount") }, "amount"},
		{"amount zero", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "0" }, "amount"},
		{"amount zero with decimals", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "0.00" }, "amount"},
		{"amount three decimals", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "1.234" }, "amount"},
		{"amount leading zero", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "01.00" }, "amount"},
		{"amount negative", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "-5" }, "amount"},
		{"amount exponent", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "1e3" }, "amount"},
		{"amount thirteen digits", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = "1234567890123" }, "amount"},
		{"amount a number, not a string", func(s map[string]interface{}) { budgetLimit2(s)["amount"] = 25 }, "amount"},
		{"period missing", func(s map[string]interface{}) { delete(budgetLimit2(s), "period") }, "period"},
		{"unknown period", func(s map[string]interface{}) { budgetLimit2(s)["period"] = "year" }, "period"},
		{"action missing", func(s map[string]interface{}) { delete(budgetLimit2(s), "action") }, "action"},
		{"unknown action", func(s map[string]interface{}) { budgetLimit2(s)["action"] = "warn" }, "action"},
		{"blockAt zero", func(s map[string]interface{}) { budgetLimit2(s)["blockAt"] = 0 }, "blockAt"},
		{"blockAt above 100", func(s map[string]interface{}) { budgetLimit2(s)["blockAt"] = 101 }, "blockAt"},
		{"unknown split type", func(s map[string]interface{}) {
			budgetLimit2(s)["splitBy"] = map[string]interface{}{"type": "model"}
		}, "type"},
		{"split type missing", func(s map[string]interface{}) {
			budgetLimit2(s)["splitBy"] = map[string]interface{}{}
		}, "type"},
		{"split expression empty", func(s map[string]interface{}) {
			budgetLimit2(s)["splitBy"] = map[string]interface{}{"type": "attribute", "expression": ""}
		}, "expression"},
		{"unknown status", func(s map[string]interface{}) { s["status"] = "paused" }, "status"},
		{"unknown filter type", func(s map[string]interface{}) {
			s["filters"] = []interface{}{map[string]interface{}{"type": "model", "id": "gpt-4o"}}
		}, "type"},
		{"filter id empty", func(s map[string]interface{}) {
			s["filters"] = []interface{}{map[string]interface{}{"type": "llmProvider", "id": ""}}
		}, "id"},
		{"filter expression empty", func(s map[string]interface{}) {
			s["filters"] = []interface{}{map[string]interface{}{"type": "attribute", "expression": ""}}
		}, "expression"},
		{"too many filters", func(s map[string]interface{}) {
			f := make([]interface{}, 21)
			for i := range f {
				f[i] = map[string]interface{}{"type": "attribute", "expression": "true"}
			}
			s["filters"] = f
		}, "filters"},
		{"unknown exemption type", func(s map[string]interface{}) {
			s["exemptions"] = []interface{}{map[string]interface{}{"type": "team", "id": "k"}}
		}, "type"},
		{"too many exemptions", func(s map[string]interface{}) {
			e := make([]interface{}, 101)
			for i := range e {
				e[i] = map[string]interface{}{"type": "project", "id": fmt.Sprintf("p%d", i)}
			}
			s["exemptions"] = e
		}, "exemptions"},
		{"too many labels", func(s map[string]interface{}) {
			l := make([]interface{}, 11)
			for i := range l {
				l[i] = fmt.Sprintf("label%d", i)
			}
			s["labels"] = l
		}, "labels"},
		{"label with a space", func(s map[string]interface{}) { s["labels"] = []interface{}{"two words"} }, "labels"},
		{"label starting with a dash", func(s map[string]interface{}) { s["labels"] = []interface{}{"-finance"} }, "labels"},
		{"label too long", func(s map[string]interface{}) {
			s["labels"] = []interface{}{strings.Repeat("a", 64)}
		}, "labels"},
		{"label repeated", func(s map[string]interface{}) { s["labels"] = []interface{}{"finance", "finance"} }, "labels"},
	}

	validators := budgetSchemaValidators(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := budgetWorkedExampleObject(t)
			tc.mutate(obj["spec"].(map[string]interface{}))
			for version, validator := range validators {
				errs := validator.validate(obj)
				if len(errs) == 0 {
					t.Fatalf("%s schema accepted an invalid budget (%s)", version, tc.name)
				}
				if !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
					t.Fatalf("%s schema error %q does not mention %q", version, errs.ToAggregate(), tc.wantErr)
				}
			}
		})
	}
}

// TestBudgetCRDHasNoCurrency asserts the schema has no limit.currency: amounts are USD.
func TestBudgetCRDHasNoCurrency(t *testing.T) {
	crd := budgetCRD(t)
	for _, v := range crd.Spec.Versions {
		limit := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["limit"]
		if _, has := limit.Properties["currency"]; has {
			t.Fatalf("version %s declares limit.currency", v.Name)
		}
	}
}

func budgetWorkedExampleObject(t *testing.T) map[string]interface{} {
	t.Helper()
	obj := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(budgetWorkedExample), &obj); err != nil {
		t.Fatalf("unmarshal worked example: %v", err)
	}
	return obj
}

func budgetLimit(obj map[string]interface{}) map[string]interface{} {
	return budgetLimit2(obj["spec"].(map[string]interface{}))
}

func budgetLimit2(spec map[string]interface{}) map[string]interface{} {
	return spec["limit"].(map[string]interface{})
}

// budgetSchema validates a Budget the way the API server does: the structural schema, then
// the uniqueness of x-kubernetes-list-type=set lists (which the schema validator leaves
// to a separate step).
type budgetSchema struct {
	validator  validation.SchemaValidator
	structural *structuralschema.Structural
}

func (s budgetSchema) validate(obj map[string]interface{}) field.ErrorList {
	errs := validation.ValidateCustomResource(nil, obj, s.validator)
	return append(errs, listtype.ValidateListSetsAndMaps(nil, s.structural, obj)...)
}

// budgetSchemaValidators builds one validator per served version of the generated Budget
// CRD.
func budgetSchemaValidators(t *testing.T) map[string]budgetSchema {
	t.Helper()

	crd := budgetCRD(t)
	out := make(map[string]budgetSchema, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			t.Fatalf("version %q of %s has no schema", v.Name, crd.Name)
		}
		internal := &apiextensions.JSONSchemaProps{}
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v.Schema.OpenAPIV3Schema, internal, nil); err != nil {
			t.Fatalf("convert %q schema: %v", v.Name, err)
		}
		validator, _, err := validation.NewSchemaValidator(internal)
		if err != nil {
			t.Fatalf("build validator for %q: %v", v.Name, err)
		}
		structural, err := structuralschema.NewStructural(internal)
		if err != nil {
			t.Fatalf("structural schema for %q: %v", v.Name, err)
		}
		out[v.Name] = budgetSchema{validator: validator, structural: structural}
	}
	return out
}

// budgetCRD loads the generated Budget CRD.
func budgetCRD(t *testing.T) apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "crd", "bases",
		"gateway.api-platform.wso2.com_budgets.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run `make manifests`?): %v", path, err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if len(crd.Spec.Versions) == 0 {
		t.Fatalf("%s serves no versions", crd.Name)
	}
	return crd
}
