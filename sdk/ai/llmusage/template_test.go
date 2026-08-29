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

// buildTemplate mirrors the shape the lazy-resource store holds: the template
// spec decoded into a generic map.
func buildTemplate() map[string]interface{} {
	return map[string]interface{}{
		"cacheAccounting": "additive",
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.input_tokens",
		},
		"completionTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.output_tokens",
		},
		"serviceTier": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.service_tier",
		},
		"resourceMappings": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"resource": "/responses",
					"promptTokens": map[string]interface{}{
						"location": "payload", "identifier": "$.usage.prompt_tokens",
					},
					"cacheAccounting": "inclusive",
				},
				map[string]interface{}{
					"resource": "/v1/*",
					"completionTokens": map[string]interface{}{
						"location": "payload", "identifier": "$.usage.wildcard_tokens",
					},
				},
			},
		},
	}
}

func TestResolveFields_NoMappingUsesTemplateRoot(t *testing.T) {
	fields, accounting := resolveFields(buildTemplate(), "/chat/completions")

	if got := fields["promptTokens"].Identifier; got != "$.usage.input_tokens" {
		t.Errorf("promptTokens = %q, want the template root value", got)
	}
	if accounting != "additive" {
		t.Errorf("cacheAccounting = %q, want additive", accounting)
	}
}

func TestResolveFields_MappingOverridesOnlyItsOwnFields(t *testing.T) {
	fields, accounting := resolveFields(buildTemplate(), "/responses")

	if got := fields["promptTokens"].Identifier; got != "$.usage.prompt_tokens" {
		t.Errorf("promptTokens = %q, want the mapping override", got)
	}
	// completionTokens is not set on this mapping, so it must fall back
	// individually rather than being lost with the rest of the object.
	if got := fields["completionTokens"].Identifier; got != "$.usage.output_tokens" {
		t.Errorf("completionTokens = %q, want the inherited root value", got)
	}
	if got := fields["serviceTier"].Identifier; got != "$.usage.service_tier" {
		t.Errorf("serviceTier = %q, want the inherited root value", got)
	}
	if accounting != "inclusive" {
		t.Errorf("cacheAccounting = %q, want the mapping override", accounting)
	}
}

func TestResolveFields_MappingWithoutAccountingInheritsIt(t *testing.T) {
	// The /v1/* mapping sets no cacheAccounting, so the template-level value
	// must be inherited rather than defaulting to inclusive.
	_, accounting := resolveFields(buildTemplate(), "/v1/anything")

	if accounting != "additive" {
		t.Errorf("cacheAccounting = %q, want the inherited additive", accounting)
	}
}

func TestResolveFields_ExactMappingBeatsWildcard(t *testing.T) {
	tmpl := map[string]interface{}{
		"resourceMappings": map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"resource": "/v1/*",
					"promptTokens": map[string]interface{}{
						"location": "payload", "identifier": "$.wildcard",
					},
				},
				map[string]interface{}{
					"resource": "/v1/exact",
					"promptTokens": map[string]interface{}{
						"location": "payload", "identifier": "$.exact",
					},
				},
			},
		},
	}

	fields, _ := resolveFields(tmpl, "/v1/exact")

	if got := fields["promptTokens"].Identifier; got != "$.exact" {
		t.Errorf("promptTokens = %q, want the exact mapping to win over the wildcard", got)
	}
}

func TestResolveFields_MalformedEntriesIgnored(t *testing.T) {
	tmpl := map[string]interface{}{
		"promptTokens":     "not-an-object",
		"completionTokens": map[string]interface{}{"location": "payload"},
		"resourceMappings": "not-an-object",
	}

	fields, accounting := resolveFields(tmpl, "/chat/completions")

	if _, ok := fields["promptTokens"]; ok {
		t.Error("promptTokens should be absent when it is not an object")
	}
	if _, ok := fields["completionTokens"]; ok {
		t.Error("completionTokens should be absent when it has no identifier")
	}
	if accounting != "" {
		t.Errorf("cacheAccounting = %q, want empty", accounting)
	}
}

func TestResolveFields_NilTemplate(t *testing.T) {
	fields, accounting := resolveFields(nil, "/chat/completions")

	if len(fields) != 0 {
		t.Errorf("fields = %v, want empty", fields)
	}
	if accounting != "" {
		t.Errorf("cacheAccounting = %q, want empty", accounting)
	}
}

func TestReadFieldSpec_ReadsValueMap(t *testing.T) {
	source := map[string]interface{}{
		"serviceTier": map[string]interface{}{
			"location":   "payload",
			"identifier": "$.usageMetadata.trafficType",
			"valueMap": map[string]interface{}{
				"ON_DEMAND_PRIORITY": "priority",
				"ON_DEMAND_FLEX":     "flex",
			},
		},
	}

	spec, ok := readFieldSpec(source, "serviceTier")
	if !ok {
		t.Fatal("readFieldSpec reported no spec")
	}
	if spec.ValueMap["ON_DEMAND_PRIORITY"] != "priority" {
		t.Errorf("ON_DEMAND_PRIORITY = %q, want priority", spec.ValueMap["ON_DEMAND_PRIORITY"])
	}
	if spec.ValueMap["ON_DEMAND_FLEX"] != "flex" {
		t.Errorf("ON_DEMAND_FLEX = %q, want flex", spec.ValueMap["ON_DEMAND_FLEX"])
	}
}

func TestReadFieldSpec_AbsentValueMapIsNil(t *testing.T) {
	spec, ok := readFieldSpec(buildTemplate(), "serviceTier")
	if !ok {
		t.Fatal("readFieldSpec reported no spec")
	}
	if spec.ValueMap != nil {
		t.Errorf("ValueMap = %v, want nil", spec.ValueMap)
	}
}

func TestReadFieldSpec_MalformedValueMapEntriesIgnored(t *testing.T) {
	source := map[string]interface{}{
		"serviceTier": map[string]interface{}{
			"location":   "payload",
			"identifier": "$.service_tier",
			"valueMap": map[string]interface{}{
				"fast":  "priority",
				"bogus": 42,
			},
		},
	}

	spec, _ := readFieldSpec(source, "serviceTier")
	if spec.ValueMap["fast"] != "priority" {
		t.Errorf("fast = %q, want priority", spec.ValueMap["fast"])
	}
	if _, present := spec.ValueMap["bogus"]; present {
		t.Error("non-string value should be skipped")
	}
}

func TestFieldPaths_ReturnsPayloadIdentifiers(t *testing.T) {
	storeTestTemplate(t, "fp-basic", buildTemplate())
	sc := &policy.SharedContext{Metadata: map[string]interface{}{
		MetadataTemplateHandle: "fp-basic",
	}}

	paths := FieldPaths(sc, "/chat/completions")

	if paths["promptTokens"] != "$.usage.input_tokens" {
		t.Errorf("promptTokens = %q, want $.usage.input_tokens", paths["promptTokens"])
	}
	if paths["completionTokens"] != "$.usage.output_tokens" {
		t.Errorf("completionTokens = %q, want $.usage.output_tokens", paths["completionTokens"])
	}
}

func TestFieldPaths_HonoursResourceMappings(t *testing.T) {
	storeTestTemplate(t, "fp-mapped", buildTemplate())
	sc := &policy.SharedContext{Metadata: map[string]interface{}{
		MetadataTemplateHandle: "fp-mapped",
	}}

	paths := FieldPaths(sc, "/responses")

	if paths["promptTokens"] != "$.usage.prompt_tokens" {
		t.Errorf("promptTokens = %q, want the /responses override $.usage.prompt_tokens",
			paths["promptTokens"])
	}
}

func TestFieldPaths_NoTemplateReturnsEmpty(t *testing.T) {
	sc := &policy.SharedContext{Metadata: map[string]interface{}{}}

	if got := FieldPaths(sc, "/chat/completions"); len(got) != 0 {
		t.Errorf("FieldPaths = %v, want empty", got)
	}
}

func TestFieldPaths_NilContextReturnsEmpty(t *testing.T) {
	if got := FieldPaths(nil, "/chat/completions"); len(got) != 0 {
		t.Errorf("FieldPaths = %v, want empty", got)
	}
}
