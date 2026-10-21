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
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// loadBedrockTemplateSpec returns the spec of the gateway's built-in awsbedrock
// template. testdata holds a JSON copy because this module has no YAML parser;
// gateway-controller's TestAWSBedrockBuiltInTemplate_MatchesSDKGolden fails if
// the copy and the shipped YAML drift apart.
func loadBedrockTemplateSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "awsbedrock-template-spec.json"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	var spec map[string]interface{}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return spec
}

// A Converse response with prompt caching, shaped as documented in the
// Bedrock TokenUsage and ServiceTier API reference.
const bedrockConverseCachedResponse = `{
  "output": {"message": {"role": "assistant", "content": [{"text": "hi"}]}},
  "stopReason": "end_turn",
  "usage": {
    "inputTokens": 30,
    "outputTokens": 628,
    "totalTokens": 1158,
    "cacheReadInputTokens": 400,
    "cacheWriteInputTokens": 100,
    "cacheDetails": [{"ttl": "1h", "inputTokens": 60}, {"ttl": "5m", "inputTokens": 40}]
  },
  "metrics": {"latencyMs": 1275},
  "serviceTier": {"type": "priority"}
}`

func TestAWSBedrockTemplate_ConverseUsageWithPromptCache(t *testing.T) {
	storeTestTemplate(t, "awsbedrock-usage", loadBedrockTemplateSpec(t))
	sc := &policy.SharedContext{Metadata: map[string]interface{}{MetadataTemplateHandle: "awsbedrock-usage"}}
	path := "/bedrock/model/arn%3Aaws%3Abedrock%3Aus-east-1%3A123456789012%3Ainference-profile%2Fus.anthropic.claude-3-5-sonnet-20240620-v1%3A0/converse"

	got, err := Get(sc, []byte(bedrockConverseCachedResponse), nil, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Bedrock's inputTokens excludes cached tokens, so the template is additive:
	// total input = inputTokens + cacheReadInputTokens + cacheWriteInputTokens.
	if got.TotalInputTokens != 530 {
		t.Errorf("TotalInputTokens = %d, want 530", got.TotalInputTokens)
	}
	if got.UncachedInputTokens != 30 {
		t.Errorf("UncachedInputTokens = %d, want 30", got.UncachedInputTokens)
	}
	if got.CachedReadTokens != 400 {
		t.Errorf("CachedReadTokens = %d, want 400", got.CachedReadTokens)
	}
	if got.CacheWriteTokens != 100 {
		t.Errorf("CacheWriteTokens = %d, want 100", got.CacheWriteTokens)
	}
	if got.OutputTokens != 628 {
		t.Errorf("OutputTokens = %d, want 628", got.OutputTokens)
	}
	if got.TotalTokens != 1158 {
		t.Errorf("TotalTokens = %d, want the reported 1158", got.TotalTokens)
	}
	if got.ServiceTier != "priority" || !got.IsPriority {
		t.Errorf("ServiceTier = %q IsPriority = %v, want priority/true", got.ServiceTier, got.IsPriority)
	}
	wantModel := "arn:aws:bedrock:us-east-1:123456789012:inference-profile/us.anthropic.claude-3-5-sonnet-20240620-v1:0"
	if got.Model != wantModel {
		t.Errorf("Model = %q, want %q", got.Model, wantModel)
	}

	details, ok := ProviderField(sc, []byte(bedrockConverseCachedResponse), path, "cacheDetails")
	if !ok {
		t.Fatal("cacheDetails provider field not found")
	}
	if list, _ := details.([]interface{}); len(list) != 2 {
		t.Errorf("cacheDetails = %v, want two per-TTL entries", details)
	}
}

func TestAWSBedrockTemplate_ConverseUsageWithoutCache(t *testing.T) {
	spec := loadBedrockTemplateSpec(t)
	body := []byte(`{"usage":{"inputTokens":30,"outputTokens":628,"totalTokens":658},"metrics":{"latencyMs":1}}`)

	got, err := extractUsage(spec, body, nil, "/model/anthropic.claude-3-sonnet-20240229-v1:0/converse")
	if err != nil {
		t.Fatalf("extractUsage: %v", err)
	}
	if got.TotalInputTokens != 30 || got.UncachedInputTokens != 30 || got.CachedReadTokens != 0 || got.CacheWriteTokens != 0 {
		t.Errorf("got input=%d uncached=%d read=%d write=%d, want 30/30/0/0",
			got.TotalInputTokens, got.UncachedInputTokens, got.CachedReadTokens, got.CacheWriteTokens)
	}
	if got.TotalTokens != 658 {
		t.Errorf("TotalTokens = %d, want 658", got.TotalTokens)
	}
	if got.ServiceTier != "" {
		t.Errorf("ServiceTier = %q, want standard (empty) when absent", got.ServiceTier)
	}
}

func TestAWSBedrockTemplate_ServiceTiers(t *testing.T) {
	spec := loadBedrockTemplateSpec(t)
	tests := []struct {
		reported string
		want     string
	}{
		{"priority", "priority"},
		{"flex", "flex"},
		// default and reserved carry no separate rate and report as standard.
		{"default", ""},
		{"reserved", ""},
	}
	for _, tt := range tests {
		t.Run(tt.reported, func(t *testing.T) {
			body := []byte(`{"usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2},"serviceTier":{"type":"` + tt.reported + `"}}`)
			got, err := extractUsage(spec, body, nil, "/model/amazon.nova-lite-v1:0/converse")
			if err != nil {
				t.Fatalf("extractUsage: %v", err)
			}
			if got.ServiceTier != tt.want {
				t.Errorf("ServiceTier = %q, want %q", got.ServiceTier, tt.want)
			}
		})
	}
}

func TestAWSBedrockTemplate_ModelFromPath(t *testing.T) {
	spec := loadBedrockTemplateSpec(t)
	requestModel := spec["requestModel"].(map[string]interface{})["identifier"].(string)
	responseModel := spec["responseModel"].(map[string]interface{})["identifier"].(string)
	if requestModel != responseModel {
		t.Fatalf("requestModel %q and responseModel %q must match: both read the same path", requestModel, responseModel)
	}

	tests := []struct {
		name    string
		segment string
		want    string
	}{
		{"base model id", "anthropic.claude-3-sonnet-20240229-v1:0", "anthropic.claude-3-sonnet-20240229-v1:0"},
		{"model id with encoded colon", "anthropic.claude-3-sonnet-20240229-v1%3A0", "anthropic.claude-3-sonnet-20240229-v1:0"},
		{"cross-region inference profile", "us.anthropic.claude-3-5-sonnet-20240620-v1:0", "us.anthropic.claude-3-5-sonnet-20240620-v1:0"},
		{"global inference profile", "global.anthropic.claude-sonnet-4-5-20250929-v1:0", "global.anthropic.claude-sonnet-4-5-20250929-v1:0"},
		{
			"foundation model arn",
			"arn%3Aaws%3Abedrock%3Aus-east-1%3A%3Afoundation-model%2Famazon.nova-pro-v1%3A0",
			"arn:aws:bedrock:us-east-1::foundation-model/amazon.nova-pro-v1:0",
		},
		{
			"application inference profile arn",
			"arn%3Aaws%3Abedrock%3Aeu-west-1%3A123456789012%3Aapplication-inference-profile%2Fa1b2c3d4e5f6",
			"arn:aws:bedrock:eu-west-1:123456789012:application-inference-profile/a1b2c3d4e5f6",
		},
		{
			"prompt management version arn",
			"arn%3Aaws%3Abedrock%3Aus-west-2%3A123456789012%3Aprompt%2FPROMPT12345%3A1",
			"arn:aws:bedrock:us-west-2:123456789012:prompt/PROMPT12345:1",
		},
		{"underscore in identifier", "my_marketplace_endpoint", "my_marketplace_endpoint"},
	}
	for _, tt := range tests {
		for _, op := range []string{"converse", "converse-stream"} {
			t.Run(tt.name+"/"+op, func(t *testing.T) {
				if got := extractFromPath("/bedrock/model/"+tt.segment+"/"+op, requestModel); got != tt.want {
					t.Errorf("extractFromPath() = %q, want %q", got, tt.want)
				}
			})
		}
	}
}

// ConverseStream responses use AWS event stream binary framing rather than SSE.
// The extractor does not decode that framing, so it reports the body as
// unusable instead of inventing counts. Update this test if a decoder is added.
func TestAWSBedrockTemplate_ConverseStreamEventStreamIsNotDecoded(t *testing.T) {
	spec := loadBedrockTemplateSpec(t)
	payload := []byte(`{"usage":{"inputTokens":11,"outputTokens":7,"totalTokens":18},"metrics":{"latencyMs":5}}`)
	frame := make([]byte, 12, 12+len(payload)+4)
	binary.BigEndian.PutUint32(frame[0:4], uint32(12+len(payload)+4))
	frame = append(frame, payload...)
	frame = append(frame, 0, 0, 0, 0)

	got, err := extractUsage(spec, frame, nil, "/model/amazon.nova-lite-v1:0/converse-stream")
	if err == nil {
		t.Fatalf("expected an error for an event-stream body, got usage %+v", got)
	}
	if got.TotalInputTokens != 0 || got.OutputTokens != 0 {
		t.Errorf("got non-zero usage %+v from an undecoded stream", got)
	}
}
