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

// REST API creation is reported by Platform API as an endpoint invocation:
// its taxonomy entry is API-only and declares one dimension, outcome, which
// the reporter derives from the status. Any other dimension would have to come
// from the request, and none is declared.
func TestCreateRESTAPIMatchesTaxonomy(t *testing.T) {
	tax, err := DefaultTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	f, ok := tax.Feature(featureRESTAPICreate)
	if !ok {
		t.Fatalf("%s is not in the taxonomy", featureRESTAPICreate)
	}
	if f.EventName != f.Code || f.Name != "REST API Creation" || f.Category != "manage-apis" ||
		f.Tier != "core" || f.EventCategory != "adoption" || !f.Platforms[PlatformAPI] || f.Platforms[PlatformWeb] {
		t.Errorf("unexpected taxonomy entry: %+v", f)
	}
	if len(f.Dimensions) != 1 {
		t.Fatalf("dimensions = %v, want only outcome", f.Dimensions)
	}
	outcome, ok := f.Dimensions[dimOutcome]
	if !ok || outcome.Type != DimensionEnum || !outcome.Required ||
		len(outcome.Values) != 2 || !outcome.Values["success"] || !outcome.Values["failure"] {
		t.Errorf("outcome = %+v, want a required enum of success and failure", outcome)
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
		"upper-case enum value":   header + feature("apip.x.y", "        - name: \"n\"\n          type: \"enum\"\n          values: [\"Client_Credentials\"]\n"),
		"enum value with a dot":   header + feature("apip.x.y", "        - name: \"n\"\n          type: \"enum\"\n          values: [\"a.b\"]\n"),
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
