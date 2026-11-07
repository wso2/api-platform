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

import "testing"

func TestEmbeddedTaxonomyParses(t *testing.T) {
	tax, err := DefaultTaxonomy()
	if err != nil {
		t.Fatalf("embedded taxonomy: %v", err)
	}
	if tax.ProductID != "apip" || tax.ProductName != "WSO2 API Platform" {
		t.Errorf("product = %q / %q, want apip / WSO2 API Platform", tax.ProductID, tax.ProductName)
	}
	if tax.Len() == 0 {
		t.Fatal("embedded taxonomy has no features")
	}
}

// The producer's typed builder must agree with the taxonomy entry it emits:
// this is what stops a code change from quietly inventing a dimension or value.
func TestCreateRESTAPIMatchesTaxonomy(t *testing.T) {
	tax, err := DefaultTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	f, ok := tax.Feature(FeatureCreateRESTAPI)
	if !ok {
		t.Fatalf("%s is not in the taxonomy", FeatureCreateRESTAPI)
	}
	if f.EventName != f.Code || f.Name != "REST API Creation" || f.Category != "manage-apis" ||
		f.Tier != "core" || f.EventCategory != "lifecycle" || !f.Platforms[PlatformAPI] {
		t.Errorf("unexpected taxonomy entry: %+v", f)
	}

	method, ok := f.Dimensions["creation_method"]
	if !ok || method.Type != DimensionEnum || !method.Required {
		t.Fatalf("creation_method = %+v, want a required enum", method)
	}
	for _, m := range []CreationMethod{CreationMethodScratch, CreationMethodContract} {
		if !method.Values[string(m)] {
			t.Errorf("creation method %q is not a value of the taxonomy enum", m)
		}
	}
	if len(method.Values) != 2 {
		t.Errorf("creation_method has %d values, want 2 (a new value needs a CreationMethod constant)", len(method.Values))
	}
	sandbox, ok := f.Dimensions["has_sandbox_upstream"]
	if !ok || sandbox.Type != DimensionBoolean || sandbox.Required {
		t.Errorf("has_sandbox_upstream = %+v, want an optional boolean", sandbox)
	}

	// Every dimension the defaults and setters write is declared.
	for _, name := range []string{dimCreationMethod, dimHasSandboxUpstream} {
		if _, declared := f.Dimensions[name]; !declared {
			t.Errorf("the code writes undeclared dimension %q", name)
		}
	}
	for name, value := range RESTAPICreationDefaults() {
		if d, declared := f.Dimensions[name]; !declared || (d.Type == DimensionEnum && !d.Values[value.(string)]) {
			t.Errorf("default %s=%v is not declared by the taxonomy", name, value)
		}
	}
}

func TestEnabledFeaturesAreInTaxonomy(t *testing.T) {
	tax, err := DefaultTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkEnabledFeatures(tax, enabledFeatures); err != nil {
		t.Fatal(err)
	}
	if err := checkEnabledFeatures(tax, map[string]bool{"apip.manage-apis.no-such-feature": true}); err == nil {
		t.Error("an allowlisted code missing from the taxonomy must be a startup error")
	}
}

func TestParseTaxonomyRejectsMalformedDocuments(t *testing.T) {
	const header = "product:\n  id: \"apip\"\n  name: \"WSO2 API Platform\"\nfeatures:\n"
	featureOn := func(platforms, code, dims string) string {
		return "  - code: \"" + code + "\"\n    name: \"N\"\n    category: \"c\"\n    tier: \"core\"\n" +
			"    tracking:\n      event_name: \"" + code + "\"\n      event_category: \"lifecycle\"\n" +
			"      platforms: " + platforms + "\n      dimensions:\n" + dims
	}
	feature := func(code, dims string) string { return featureOn(`["web", "api"]`, code, dims) }
	tests := map[string]string{
		"reserved dimension name": header + feature("apip.x.y", "        - name: \"action\"\n          type: \"string\"\n"),
		"unsupported type":        header + feature("apip.x.y", "        - name: \"n\"\n          type: \"integer\"\n"),
		"enum without values":     header + feature("apip.x.y", "        - name: \"n\"\n          type: \"enum\"\n"),
		"malformed enum value":    header + feature("apip.x.y", "        - name: \"n\"\n          type: \"enum\"\n          values: [\"Has Space\"]\n"),
		"duplicate code":          header + feature("apip.x.y", "        []\n") + feature("apip.x.y", "        []\n"),
		"code outside product":    header + feature("other.x.y", "        []\n"),
		"malformed code":          header + feature("apip.X_Y", "        []\n"),
		"no product":              "features:\n" + feature("apip.x.y", "        []\n"),
		"no product name":         "product:\n  id: \"apip\"\nfeatures:\n" + feature("apip.x.y", "        []\n"),
		"unknown platform":        header + featureOn(`["cli"]`, "apip.x.y", "        []\n"),
		"reserved growth_action":  header + feature("apip.x.y", "        - name: \"growth_action\"\n          type: \"string\"\n"),
	}
	for name, doc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTaxonomy([]byte(doc)); err == nil {
				t.Errorf("ParseTaxonomy accepted a document with %s", name)
			}
		})
	}

	valid := header + feature("apip.x.y", "        - name: \"n\"\n          type: \"enum\"\n          values: [\"a\", \"b-c\"]\n")
	if _, err := ParseTaxonomy([]byte(valid)); err != nil {
		t.Errorf("ParseTaxonomy rejected a valid document: %v", err)
	}
}
