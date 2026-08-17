/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */
 
package llmusage

import (
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func templateWithProviderFields() map[string]interface{} {
	return map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.inputTokens",
		},
		"providerFields": map[string]interface{}{
			"cacheDetails": map[string]interface{}{
				"location": "payload", "identifier": "$.usage.cacheDetails",
			},
			"tier": map[string]interface{}{
				"location": "payload", "identifier": "$.serviceTier.type",
			},
		},
	}
}

const bodyWithList = `{
  "usage": {"inputTokens": 100,
            "cacheDetails": [{"ttl":"5m","inputTokens":300},{"ttl":"1h","inputTokens":200}]},
  "serviceTier": {"type": "priority"}
}`

func TestProviderField_ReturnsListAsDecodedJSON(t *testing.T) {
	storeTestTemplate(t, "pf", templateWithProviderFields())
	sc := &policy.SharedContext{Metadata: map[string]interface{}{MetadataTemplateHandle: "pf"}}

	raw, ok := ProviderField(sc, []byte(bodyWithList), "/converse", "cacheDetails")
	if !ok {
		t.Fatal("cacheDetails not found")
	}
	list, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("got %T, want []interface{}", raw)
	}
	if len(list) != 2 {
		t.Fatalf("got %d entries, want 2", len(list))
	}
	first, _ := list[0].(map[string]interface{})
	if first["ttl"] != "5m" {
		t.Errorf("first ttl = %v, want 5m", first["ttl"])
	}
}

func TestProviderField_ReturnsScalar(t *testing.T) {
	storeTestTemplate(t, "pf2", templateWithProviderFields())
	sc := &policy.SharedContext{Metadata: map[string]interface{}{MetadataTemplateHandle: "pf2"}}

	raw, ok := ProviderField(sc, []byte(bodyWithList), "/converse", "tier")
	if !ok {
		t.Fatal("tier not found")
	}
	if raw != "priority" {
		t.Errorf("tier = %v, want priority", raw)
	}
}

func TestProviderField_UndeclaredNameReportsMissing(t *testing.T) {
	storeTestTemplate(t, "pf3", templateWithProviderFields())
	sc := &policy.SharedContext{Metadata: map[string]interface{}{MetadataTemplateHandle: "pf3"}}

	if _, ok := ProviderField(sc, []byte(bodyWithList), "/converse", "nope"); ok {
		t.Error("an undeclared name must report missing")
	}
}

func TestProviderField_NoTemplateReportsMissing(t *testing.T) {
	sc := &policy.SharedContext{Metadata: map[string]interface{}{}}
	if _, ok := ProviderField(sc, []byte(bodyWithList), "/converse", "cacheDetails"); ok {
		t.Error("no template must report missing")
	}
	if _, ok := ProviderField(nil, []byte(bodyWithList), "/converse", "cacheDetails"); ok {
		t.Error("nil context must report missing")
	}
}

func TestProviderField_AbsentInBodyReportsMissing(t *testing.T) {
	storeTestTemplate(t, "pf4", templateWithProviderFields())
	sc := &policy.SharedContext{Metadata: map[string]interface{}{MetadataTemplateHandle: "pf4"}}

	if _, ok := ProviderField(sc, []byte(`{"usage":{"inputTokens":1}}`), "/converse", "cacheDetails"); ok {
		t.Error("a declared location absent from the body must report missing")
	}
}
