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
	"bytes"
	"testing"
)

func openAITemplate() map[string]interface{} {
	return map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.prompt_tokens",
		},
		"completionTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.completion_tokens",
		},
		"totalTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.total_tokens",
		},
		"cachedTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.prompt_tokens_details.cached_tokens",
		},
		"serviceTier": map[string]interface{}{
			"location": "payload", "identifier": "$.service_tier",
		},
		"responseModel": map[string]interface{}{
			"location": "payload", "identifier": "$.model",
		},
	}
}

func TestExtractUsage_BufferedResponse(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o-mini-2024-07-18",
		"service_tier": "default",
		"usage": {
			"prompt_tokens": 1000,
			"completion_tokens": 200,
			"total_tokens": 1200,
			"prompt_tokens_details": {"cached_tokens": 800}
		}
	}`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 1000 {
		t.Errorf("TotalInputTokens = %d, want 1000", got.TotalInputTokens)
	}
	if got.CachedReadTokens != 800 {
		t.Errorf("CachedReadTokens = %d, want 800", got.CachedReadTokens)
	}
	if got.UncachedInputTokens != 200 {
		t.Errorf("UncachedInputTokens = %d, want 200", got.UncachedInputTokens)
	}
	if got.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", got.OutputTokens)
	}
	if got.TotalTokens != 1200 {
		t.Errorf("TotalTokens = %d, want 1200", got.TotalTokens)
	}
	if got.Model != "gpt-4o-mini-2024-07-18" {
		t.Errorf("Model = %q, want the response model", got.Model)
	}
	if got.IsPriority {
		t.Error("IsPriority = true, want false for service_tier=default")
	}
}

func TestExtractUsage_ModelFallsBackToRequestBody(t *testing.T) {
	tmpl := openAITemplate()
	tmpl["requestModel"] = map[string]interface{}{
		"location": "payload", "identifier": "$.model",
	}

	// The responses API echoes no model, so the request body supplies it.
	body := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	requestBody := []byte(`{"model":"my-deployment"}`)

	got, err := extractUsage(tmpl, body, requestBody, "/responses")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.Model != "my-deployment" {
		t.Errorf("Model = %q, want my-deployment from the request body", got.Model)
	}
	if len(got.ModelCandidates) == 0 {
		t.Error("ModelCandidates is empty, want the tried names recorded")
	}
}

func TestExtractUsage_SSEStreamMergesEvents(t *testing.T) {
	// The model arrives in the first event and usage only in the last, so the
	// merged view must carry both.
	body := []byte(`data: {"model":"gpt-4o-mini","choices":[{"delta":{"content":"hi"}}]}

data: {"usage":{"prompt_tokens":50,"completion_tokens":10,"total_tokens":60}}

data: [DONE]
`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 50 {
		t.Errorf("TotalInputTokens = %d, want 50", got.TotalInputTokens)
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want the model from the first event", got.Model)
	}
}

func TestExtractUsage_SSEEmptyStringDoesNotOverwrite(t *testing.T) {
	// A later event carrying an empty model must not erase an earlier real one.
	body := []byte(`data: {"model":"gpt-4o-mini"}

data: {"model":"","usage":{"prompt_tokens":5,"completion_tokens":1}}
`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want the non-empty earlier value preserved", got.Model)
	}
}

func TestExtractUsage_SSENullDoesNotOverwrite(t *testing.T) {
	// Providers repeat a field as null in the chunks carrying no value for it,
	// including after the chunk that did; that must not erase what was reported.
	body := []byte(`data: {"model":"gpt-4o-mini","usage":{"prompt_tokens":5,"completion_tokens":1}}

data: {"model":"","usage":null}
`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 5 || got.OutputTokens != 1 {
		t.Errorf("usage = %d in / %d out, want 5 / 1 preserved", got.TotalInputTokens, got.OutputTokens)
	}
}

func TestExtractUsage_NoUsageObject(t *testing.T) {
	body := []byte(`{"model":"gpt-4o-mini","choices":[]}`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 0 || got.OutputTokens != 0 {
		t.Errorf("expected zero counts, got in=%d out=%d", got.TotalInputTokens, got.OutputTokens)
	}
}

func TestExtractUsage_UnparseableBody(t *testing.T) {
	if _, err := extractUsage(openAITemplate(), []byte(`not json at all`), nil, "/x"); err == nil {
		t.Error("expected an error for an unparseable body")
	}
}

func TestExtractUsage_AnthropicAdditiveAccounting(t *testing.T) {
	tmpl := map[string]interface{}{
		"cacheAccounting": "additive",
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.input_tokens",
		},
		"completionTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.output_tokens",
		},
		"cachedTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.cache_read_input_tokens",
		},
		"cacheWriteTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.cache_creation.ephemeral_5m_input_tokens",
		},
		"serviceTier": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.service_tier",
		},
	}
	body := []byte(`{
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 200,
			"cache_read_input_tokens": 500,
			"cache_creation": {"ephemeral_5m_input_tokens": 300},
			"service_tier": "priority"
		}
	}`)

	got, err := extractUsage(tmpl, body, nil, "/v1/messages")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 1800 {
		t.Errorf("TotalInputTokens = %d, want 1800 (1000+500+300)", got.TotalInputTokens)
	}
	if got.UncachedInputTokens != 1000 {
		t.Errorf("UncachedInputTokens = %d, want 1000", got.UncachedInputTokens)
	}
	if !got.IsPriority {
		t.Error("IsPriority = false, want true")
	}
}

func TestExtractUsage_FallbackIdentifiersTryInOrder(t *testing.T) {
	// One template covering three Bedrock response shapes.
	ident := func(primary string, fallbacks ...string) map[string]interface{} {
		spec := map[string]interface{}{"location": "payload", "identifier": primary}
		if len(fallbacks) > 0 {
			list := make([]interface{}, 0, len(fallbacks))
			for _, f := range fallbacks {
				list = append(list, f)
			}
			spec["fallbackIdentifiers"] = list
		}
		return spec
	}
	tmpl := map[string]interface{}{
		"promptTokens":     ident("$.usage.inputTokens", "$.usage.input_tokens", "$.usage.prompt_tokens", "$.inputTextTokenCount"),
		"completionTokens": ident("$.usage.outputTokens", "$.usage.output_tokens", "$.usage.completion_tokens", "$.results[0].tokenCount"),
	}

	tests := []struct {
		name    string
		body    string
		wantIn  int64
		wantOut int64
	}{
		{"converse shape", `{"usage":{"inputTokens":100,"outputTokens":50}}`, 100, 50},
		{"anthropic invokemodel shape", `{"usage":{"input_tokens":200,"output_tokens":60}}`, 200, 60},
		{"openai transformed shape", `{"usage":{"prompt_tokens":300,"completion_tokens":70}}`, 300, 70},
		{"titan shape", `{"inputTextTokenCount":400,"results":[{"tokenCount":80}]}`, 400, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractUsage(tmpl, []byte(tt.body), nil, "/model/x/invoke")
			if err != nil {
				t.Fatalf("extractUsage returned error: %v", err)
			}
			if got.TotalInputTokens != tt.wantIn {
				t.Errorf("TotalInputTokens = %d, want %d", got.TotalInputTokens, tt.wantIn)
			}
			if got.OutputTokens != tt.wantOut {
				t.Errorf("OutputTokens = %d, want %d", got.OutputTokens, tt.wantOut)
			}
		})
	}
}

func TestExtractUsage_PrimaryIdentifierWinsOverFallback(t *testing.T) {
	tmpl := map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location":            "payload",
			"identifier":          "$.usage.inputTokens",
			"fallbackIdentifiers": []interface{}{"$.usage.prompt_tokens"},
		},
	}
	// Both present: the primary must win.
	body := []byte(`{"usage":{"inputTokens":100,"prompt_tokens":999}}`)

	got, err := extractUsage(tmpl, body, nil, "/x")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}
	if got.TotalInputTokens != 100 {
		t.Errorf("TotalInputTokens = %d, want 100 from the primary identifier", got.TotalInputTokens)
	}
}

func TestExtractUsage_ValueMapTranslatesServiceTier(t *testing.T) {
	template := map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usageMetadata.promptTokenCount",
		},
		"completionTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usageMetadata.candidatesTokenCount",
		},
		"responseModel": map[string]interface{}{
			"location": "payload", "identifier": "$.modelVersion",
		},
		"serviceTier": map[string]interface{}{
			"location":   "payload",
			"identifier": "$.usageMetadata.trafficType",
			"valueMap": map[string]interface{}{
				"ON_DEMAND_PRIORITY": "priority",
				"ON_DEMAND_FLEX":     "flex",
			},
		},
	}

	body := []byte(`{
		"modelVersion": "gemini-2.0-flash",
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 20,
			"trafficType": "ON_DEMAND_PRIORITY"
		}
	}`)

	usage, err := extractUsage(template, body, nil, "/v1/models/gemini-2.0-flash:generateContent")
	if err != nil {
		t.Fatalf("extractUsage: %v", err)
	}
	if usage.ServiceTier != "priority" {
		t.Errorf("ServiceTier = %q, want priority", usage.ServiceTier)
	}
	if !usage.IsPriority {
		t.Error("IsPriority = false, want true")
	}
}

func TestExtractUsage_UnmappedTierFallsThroughToStandard(t *testing.T) {
	template := map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usageMetadata.promptTokenCount",
		},
		"serviceTier": map[string]interface{}{
			"location":   "payload",
			"identifier": "$.usageMetadata.trafficType",
			"valueMap": map[string]interface{}{
				"ON_DEMAND_PRIORITY": "priority",
			},
		},
	}

	body := []byte(`{"usageMetadata":{"promptTokenCount":100,"trafficType":"PROVISIONED_THROUGHPUT"}}`)

	usage, err := extractUsage(template, body, nil, "/v1/models/x:generateContent")
	if err != nil {
		t.Fatalf("extractUsage: %v", err)
	}
	if usage.ServiceTier != "" {
		t.Errorf("ServiceTier = %q, want empty", usage.ServiceTier)
	}
}

func TestExtractUsage_NoValueMapKeepsExistingBehaviour(t *testing.T) {
	template := map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.prompt_tokens",
		},
		"serviceTier": map[string]interface{}{
			"location": "payload", "identifier": "$.service_tier",
		},
	}

	body := []byte(`{"service_tier":"flex","usage":{"prompt_tokens":100}}`)

	usage, err := extractUsage(template, body, nil, "/v1/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage: %v", err)
	}
	if usage.ServiceTier != "flex" {
		t.Errorf("ServiceTier = %q, want flex", usage.ServiceTier)
	}
}

func TestExtractUsage_SSEEventSplitAcrossDataFields(t *testing.T) {
	// The stream format lets one event spread its payload over several data
	// fields, joined by a newline. Read individually each fragment is invalid
	// JSON, so the fields have to be joined before decoding.
	body := []byte(`data: {"model":"gpt-4o-mini",
data: "usage":{"prompt_tokens":50,
data: "completion_tokens":10,"total_tokens":60}}

data: [DONE]
`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 50 {
		t.Errorf("TotalInputTokens = %d, want 50", got.TotalInputTokens)
	}
	if got.OutputTokens != 10 {
		t.Errorf("OutputTokens = %d, want 10", got.OutputTokens)
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want gpt-4o-mini", got.Model)
	}
}

func TestExtractUsage_SSECompleteObjectPerDataFieldWithoutBlankLine(t *testing.T) {
	// Joining these fields would produce two top-level objects, which is not
	// valid JSON. Each field is a complete document on its own, so both must
	// still be read rather than the event being discarded.
	body := []byte(`data: {"model":"gpt-4o-mini"}
data: {"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}
`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 7 {
		t.Errorf("TotalInputTokens = %d, want 7", got.TotalInputTokens)
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want gpt-4o-mini", got.Model)
	}
}

func TestResolveModel_PathParamRequestModelWithoutRequestBody(t *testing.T) {
	// AWS Bedrock and Gemini declare requestModel as a path param, so it must
	// resolve from the request path even when no request body is available.
	// A response that carries no model of its own relies on this fallback.
	template := map[string]interface{}{
		"promptTokens": map[string]interface{}{
			"location": "payload", "identifier": "$.usage.inputTokens",
		},
		"responseModel": map[string]interface{}{
			"location": "payload", "identifier": "$.model",
		},
		"requestModel": map[string]interface{}{
			"location": "pathParam", "identifier": `model/([A-Za-z0-9.:%-]+)/`,
		},
	}
	body := []byte(`{"usage":{"inputTokens":10}}`) // no model in the response

	got, err := extractUsage(template, body, nil, "/model/anthropic.claude-3-7-sonnet-20250219-v1:0/converse")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	want := "anthropic.claude-3-7-sonnet-20250219-v1:0"
	if got.Model != want {
		t.Errorf("Model = %q, want it resolved from the request path (%q)", got.Model, want)
	}
	if len(got.ModelCandidates) != 1 || got.ModelCandidates[0] != want {
		t.Errorf("ModelCandidates = %v, want [%s]", got.ModelCandidates, want)
	}
}

// A provider may report one field across several events: prompt tokens in an
// early event and completion tokens in a later one. Replacing the object rather
// than merging its members drops the earlier count, so the request bills only
// the tokens the last event happened to carry.
func TestExtractUsage_SSEUsageSplitAcrossEvents(t *testing.T) {
	body := []byte(`data: {"model":"gpt-4o-mini","usage":{"prompt_tokens":50}}

data: {"usage":{"completion_tokens":10}}

data: {"usage":{"total_tokens":60}}

data: [DONE]
`)

	got, err := extractUsage(openAITemplate(), body, nil, "/chat/completions")
	if err != nil {
		t.Fatalf("extractUsage returned error: %v", err)
	}

	if got.TotalInputTokens != 50 {
		t.Errorf("TotalInputTokens = %d, want 50 — an earlier event's count was dropped", got.TotalInputTokens)
	}
	if got.OutputTokens != 10 {
		t.Errorf("OutputTokens = %d, want 10", got.OutputTokens)
	}
	if got.TotalTokens != 60 {
		t.Errorf("TotalTokens = %d, want 60", got.TotalTokens)
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want gpt-4o-mini", got.Model)
	}
}

// Merging must not turn a later scalar or array into a merge target: the newest
// event still wins for anything that is not an object.
func TestExtractUsage_SSELaterScalarsAndArraysStillReplace(t *testing.T) {
	body := []byte(`data: {"model":"first","usage":{"prompt_tokens":1},"choices":[{"index":0}]}

data: {"model":"second","usage":{"prompt_tokens":9},"choices":[{"index":7}]}
`)

	merged, ok := mergeSSEEvents(body)
	if !ok {
		t.Fatal("mergeSSEEvents reported no events")
	}
	for _, want := range []string{`"model":"second"`, `"prompt_tokens":9`, `"index":7`} {
		if !bytes.Contains(merged, []byte(want)) {
			t.Errorf("merged view is missing %s; got %s", want, merged)
		}
	}
}
