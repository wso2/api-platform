/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

// These tests keep bindings.go, the embedded taxonomy and the OpenAPI
// document in agreement. When one fails after a taxonomy or API change, the
// add-feature-usage-tracking skill (.agents/skills) says what to change.

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// featureRESTAPICreate is a feature code the tests use. Production code names
// feature codes only in bindings.go.
const featureRESTAPICreate = "apip.manage-apis.rest-api-create"

// openAPIDocument is the Platform API's REST API description, the source of
// the operationIds bindings.go names.
const openAPIDocument = "../../resources/openapi.yaml"

func TestBindingsAgreeWithTheTaxonomy(t *testing.T) {
	if err := checkBindings(mustTaxonomy(t), bindings); err != nil {
		t.Fatal(err)
	}
	if err := checkEnabledFeatures(mustTaxonomy(t), enabledFeatures); err != nil {
		t.Fatal(err)
	}
}

// Every bound operation is declared by the OpenAPI document, and every path
// parameter a binding reads is a parameter of the operation's path.
func TestBindingsAgreeWithTheOpenAPIDocument(t *testing.T) {
	spec, err := os.ReadFile(openAPIDocument)
	if err != nil {
		t.Fatal(err)
	}
	ops, err := specOperations(spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bindings {
		op, ok := ops[b.Op]
		if !ok {
			t.Errorf("operation %q is bound but not declared in %s", b.Op, openAPIDocument)
			continue
		}
		if op.method == http.MethodGet {
			t.Errorf("operation %q is a GET; only actions (writes) are reported", b.Op)
		}
		if missing := missingPathParams(b, op.path); missing != "" {
			t.Errorf("operation %q reads path parameter %q, which %s %s does not have", b.Op, missing, op.method, op.path)
		}
	}
	index, err := newRouteIndex(spec, nil, mustTaxonomy(t), bindings, nil)
	if err != nil {
		t.Fatal(err)
	}
	if index.Len() != len(bindings) {
		t.Errorf("route index binds %d routes, want one per binding (%d)", index.Len(), len(bindings))
	}
}

// Every feature the taxonomy says Platform API emits is reported, or
// explicitly not, with a reason. A new Platform API feature in the taxonomy
// fails here until it is bound.
func TestEveryPlatformAPIFeatureIsBound(t *testing.T) {
	tax := mustTaxonomy(t)
	bound := boundFeatures(bindings)
	for _, code := range sortedKeys(tax.features) {
		f := tax.features[code]
		if !f.Platforms[PlatformAPI] || !strings.Contains(f.Description, platformAPIEmitter) {
			continue
		}
		_, exempt := notBound[code]
		switch {
		case bound[code] && exempt:
			t.Errorf("%s is bound and also listed in notBound; remove one", code)
		case !bound[code] && !exempt:
			t.Errorf("%s is emitted by Platform API per the taxonomy but bound to no operation: "+
				"add it to bindings in bindings.go, or to notBound with the reason", code)
		}
	}
	for _, code := range sortedKeys(notBound) {
		f, ok := tax.Feature(code)
		if !ok || !strings.Contains(f.Description, platformAPIEmitter) {
			t.Errorf("notBound lists %s, which is not a Platform API feature in the taxonomy; remove it", code)
		}
		if strings.TrimSpace(notBound[code]) == "" {
			t.Errorf("notBound lists %s without a reason", code)
		}
	}
}

func TestDocumentTypesCoverTheTaxonomy(t *testing.T) {
	dim := mustTaxonomy(t).features["apip.manage-apis.api-document-create"].Dimensions["document_type"]
	sent := map[string]bool{}
	for _, v := range documentTypes {
		sent[v] = true
	}
	for v := range dim.Values {
		if !sent[v] {
			t.Errorf("document_type %q is in the taxonomy but no document type maps to it", v)
		}
	}
}

func TestCheckBindingsRejectsAMistake(t *testing.T) {
	tax := mustTaxonomy(t)
	tests := map[string]Binding{
		"no operation":    {Feature: featureRESTAPICreate},
		"unknown feature": {Op: "X", Feature: "apip.manage-apis.no-such-feature"},
		"console feature": {Op: "X", Feature: "apip.manage-apis.rest-api-create-click"},
		"emitted by another producer": {Op: "X", Feature: "apip.gateway.kubernetes-operator",
			Set: Dims{"resource_kind": "rest-api"}},
		"undeclared dimension": {Op: "X", Feature: featureRESTAPICreate, Set: Dims{"api_type": "rest-api"}},
		"outcome bound":        {Op: "X", Feature: featureRESTAPICreate, Set: Dims{"outcome": "success"}},
		"Set value outside the enum": {Op: "X", Feature: "apip.manage-apis.api-update",
			Set: Dims{"api_type": "soap-api"}},
		"Set value of the wrong type": {Op: "X", Feature: "apip.manage-apis.api-update",
			Set: Dims{"api_type": true}},
		"required dimension without a value": {Op: "X", Feature: "apip.manage-apis.api-update"},
		"both Set and From": {Op: "X", Feature: "apip.manage-apis.api-update",
			Set: Dims{"api_type": "rest-api"}, From: Sources{"api_type": Path("apiType")}},
		"Map to a value outside the enum": {Op: "X", Feature: "apip.monetization.subscription-plan-update",
			From: Sources{"status": JSON("/status").Map(map[string]string{"ACTIVE": "enabled"})}},
		"Default outside the enum": {Op: "X", Feature: "apip.gateway.gateway-update",
			From: Sources{"gateway_type": JSON("/functionalityType").Default("edge")}},
		"boolean from a form": {Op: "X", Feature: "apip.gateway.gateway-register",
			From: Sources{"gateway_type": JSON("/t"), "is_critical": Form("critical")}},
		"boolean Default that is not a bool": {Op: "X", Feature: "apip.gateway.gateway-register",
			From: Sources{"gateway_type": JSON("/t"), "is_critical": JSON("/c").Default("no")}},
		"source not built with a constructor": {Op: "X", Feature: "apip.manage-apis.api-update",
			From: Sources{"api_type": Source{}}},
		"bad JSON Pointer": {Op: "X", Feature: "apip.gateway.gateway-update",
			From: Sources{"gateway_type": JSON("functionalityType")}},
	}
	for name, b := range tests {
		t.Run(name, func(t *testing.T) {
			if err := checkBindings(tax, []Binding{b}); err == nil {
				t.Fatal("the binding was accepted")
			}
		})
	}
	dup := []Binding{{Op: "X", Feature: featureRESTAPICreate}, {Op: "X", Feature: featureRESTAPICreate}}
	if err := checkBindings(tax, dup); err == nil {
		t.Error("an operation bound twice was accepted")
	}
}

// Free text is never read from a request into a string dimension, even when
// the taxonomy declares one.
func TestCheckBindingsRejectsAStringDimensionFromTheRequest(t *testing.T) {
	tax, err := ParseTaxonomy([]byte(`product:
  id: "apip"
  name: "API Platform"
features:
  - code: "apip.policies.policy-attach"
    name: "Policy attach"
    description: "Attach a policy. Emitted by Platform API."
    category: "policies"
    tier: "core"
    tracking:
      event_name: "apip.policies.policy-attach"
      event_category: "adoption"
      platforms: ["api"]
      dimensions:
        - name: "policy_name"
          type: "string"
          required: true
`))
	if err != nil {
		t.Fatal(err)
	}
	fromRequest := Binding{Op: "X", Feature: "apip.policies.policy-attach", From: Sources{"policy_name": JSON("/name")}}
	if err := checkBindings(tax, []Binding{fromRequest}); err == nil {
		t.Error("a string dimension read from the request was accepted")
	}
	fixed := Binding{Op: "X", Feature: "apip.policies.policy-attach", Set: Dims{"policy_name": "basic-ratelimit"}}
	if err := checkBindings(tax, []Binding{fixed}); err != nil {
		t.Errorf("a fixed code for a string dimension was rejected: %v", err)
	}
}

func TestSourceResolve(t *testing.T) {
	form := &formView{values: map[string][]string{
		"type":     {"  HowTo "},
		"lower":    {"supportforum"},
		"empty":    {"  "},
		"metadata": {`{"schemaSource":"file","nested":{"a~b":{"c/d":"deep"}},"list":["x","y"],"none":null}`},
	}}
	v := &requestView{
		pathValues: map[string]string{"apiType": "rest-api", "kind": "GraphQLApi"},
		form:       form,
		body:       []byte(`{"functionalityType":"ai","isCritical":false,"count":3}`),
	}
	noForm := &requestView{}
	tests := map[string]struct {
		src  Source
		view *requestView
		want any // nil: no value
	}{
		"path":                     {Path("apiType"), v, "rest-api"},
		"path mapped":              {Path("kind").Map(apiTypes), v, "graphql-api"},
		"path not matched":         {Path("apiId"), v, nil},
		"form trimmed":             {Form("type"), v, "HowTo"},
		"form mapped":              {Form("type").Map(documentTypes), v, "how-to"},
		"form mapped any case":     {Form("lower").Map(documentTypes), v, "support-forum"},
		"form unmapped":            {Form("type").Map(map[string]string{"Samples": "samples"}), v, nil},
		"form empty is absent":     {Form("empty").Default("d"), v, "d"},
		"form missing default":     {Form("nope").Default("d"), v, "d"},
		"no form ignores default":  {Form("type").Default("d"), noForm, nil},
		"form json":                {FormJSON("metadata", "/schemaSource"), v, "file"},
		"form json escaped tokens": {FormJSON("metadata", "/nested/a~0b/c~1d"), v, "deep"},
		"form json array index":    {FormJSON("metadata", "/list/1"), v, "y"},
		"form json null is absent": {FormJSON("metadata", "/none").Default("d"), v, "d"},
		"form json missing field":  {FormJSON("nope", "/x").Default("d"), v, nil},
		"json string":              {JSON("/functionalityType"), v, "ai"},
		"json bool":                {JSON("/isCritical"), v, false},
		"json absent default":      {JSON("/missing").Default(true), v, true},
		"json not captured":        {JSON("/functionalityType").Default("regular"), noForm, nil},
		"present first wins":       {Present("empty", "type", "metadata"), v, "type"},
		"present none":             {Present("nope"), v, nil},
		"present no form":          {Present("type"), noForm, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := tc.src.resolve(tc.view)
			if tc.want == nil {
				if ok {
					t.Fatalf("got %v, want no value", got)
				}
				return
			}
			if !ok || got != tc.want {
				t.Fatalf("got %v (%v), want %v", got, ok, tc.want)
			}
		})
	}
}

// An operation the OpenAPI document declares but the mux does not serve (a
// build without the plugin that serves it) is skipped, not a startup error.
func TestRouteIndexSkipsAnOperationNobodyServes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v0.9/rest-apis", func(http.ResponseWriter, *http.Request) {})
	index, err := newRouteIndex([]byte(testSpec), mux, mustTaxonomy(t), testBindings, nil)
	if err != nil {
		t.Fatal(err)
	}
	if index.Len() != 1 {
		t.Fatalf("route index binds %d routes, want only the served one", index.Len())
	}
}

// The mux decides a path parameter's name, since that is what the handler
// reads; a binding reading a name the route does not have is an error.
func TestRouteIndexRejectsAPathParameterTheRouteLacks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v0.9/apis/{kind}/{apiId}/docs", func(http.ResponseWriter, *http.Request) {})
	bs := []Binding{{Op: "CreateAPIDocument", Feature: "apip.manage-apis.api-document-create",
		From: Sources{"api_type": Path("apiType")}}}
	if _, err := newRouteIndex([]byte(testSpec), mux, mustTaxonomy(t), bs, nil); err == nil {
		t.Fatal("a binding reading a missing path parameter was accepted")
	}
}
