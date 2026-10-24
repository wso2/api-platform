/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// Paths to the shipped AWS Bedrock artifacts, relative to this package.
var (
	bedrockGatewayTemplateDir  = filepath.Join("..", "..", "default-llm-provider-templates")
	bedrockGatewayTemplateFile = filepath.Join(bedrockGatewayTemplateDir, "awsbedrock-template.yaml")
	bedrockPlatformTemplate    = filepath.Join("..", "..", "..", "..", "platform-api", "resources", "default-llm-provider-templates", "awsbedrock-template.yaml")
	bedrockSDKGoldenSpec       = filepath.Join("..", "..", "..", "..", "sdk", "ai", "llmusage", "testdata", "awsbedrock-template-spec.json")
	bedrockOpenAPISpec         = filepath.Join("..", "..", "..", "..", "llm-provider-specs", "awsbedrock", "openapi.yaml")
	bedrockExamplesDir         = filepath.Join("..", "..", "..", "examples")
)

const (
	bedrockConversePath       = "/model/{modelId}/converse"
	bedrockConverseStreamPath = "/model/{modelId}/converse-stream"
	awsAuthenticationPolicy   = "aws-authentication"
)

// readYAMLAsJSONMap decodes a YAML file into the map shape encoding/json
// produces, so it can be compared with JSON documents or unmarshalled into the
// generated API types through their json tags.
func readYAMLAsJSONMap(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	encoded, err := json.Marshal(doc)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &out))
	return out
}

func loadBuiltInBedrockTemplate(t *testing.T) *api.LLMProviderTemplate {
	t.Helper()
	loader := NewLLMTemplateLoader(slog.New(slog.NewTextHandler(io.Discard, nil)))
	templates, err := loader.LoadTemplatesFromDirectory(bedrockGatewayTemplateDir)
	require.NoError(t, err)
	tmpl, ok := templates["awsbedrock"]
	require.True(t, ok, "built-in awsbedrock template not loaded")
	return tmpl
}

func TestAWSBedrockBuiltInTemplate_LoadsAndValidates(t *testing.T) {
	tmpl := loadBuiltInBedrockTemplate(t)
	assert.Empty(t, config.NewLLMValidator().Validate(tmpl))

	spec := tmpl.Spec
	require.NotNil(t, spec.PromptTokens)
	assert.Equal(t, "$.usage.inputTokens", spec.PromptTokens.Identifier)
	require.NotNil(t, spec.CompletionTokens)
	assert.Equal(t, "$.usage.outputTokens", spec.CompletionTokens.Identifier)
	require.NotNil(t, spec.TotalTokens)
	assert.Equal(t, "$.usage.totalTokens", spec.TotalTokens.Identifier)
	require.NotNil(t, spec.CachedTokens)
	assert.Equal(t, "$.usage.cacheReadInputTokens", spec.CachedTokens.Identifier)
	require.NotNil(t, spec.CacheWriteTokens)
	assert.Equal(t, "$.usage.cacheWriteInputTokens", spec.CacheWriteTokens.Identifier)
	require.NotNil(t, spec.CacheAccounting)
	assert.Equal(t, "additive", *spec.CacheAccounting,
		"Bedrock's inputTokens excludes cache reads and writes")
	require.NotNil(t, spec.ServiceTier)
	assert.Equal(t, "$.serviceTier.type", spec.ServiceTier.Identifier)
	require.NotNil(t, spec.ProviderFields)
	assert.Equal(t, "$.usage.cacheDetails", (*spec.ProviderFields)["cacheDetails"].Identifier)

	require.NotNil(t, spec.RequestModel)
	require.NotNil(t, spec.ResponseModel)
	assert.Equal(t, api.PathParam, spec.RequestModel.Location)
	assert.Equal(t, spec.RequestModel.Identifier, spec.ResponseModel.Identifier)
	re, err := regexp.Compile(spec.RequestModel.Identifier)
	require.NoError(t, err, "pathParam identifiers must be RE2")
	assert.Equal(t, 1, re.NumSubexp(), "the model id is read from the first capture group")
}

// The model regex must select the whole modelId segment for every identifier
// shape Bedrock accepts, on both operations. Decoding is the extractor's job
// and is covered in sdk/ai/llmusage and the analytics system policy.
func TestAWSBedrockBuiltInTemplate_ModelRegexCapturesSegment(t *testing.T) {
	re := regexp.MustCompile(loadBuiltInBedrockTemplate(t).Spec.RequestModel.Identifier)
	segments := []string{
		"anthropic.claude-3-sonnet-20240229-v1:0",
		"anthropic.claude-3-sonnet-20240229-v1%3A0",
		"us.anthropic.claude-3-5-sonnet-20240620-v1:0",
		"arn%3Aaws%3Abedrock%3Aus-east-1%3A123456789012%3Ainference-profile%2Fus.anthropic.claude-3-5-sonnet-20240620-v1%3A0",
		"arn%3Aaws%3Abedrock%3Aeu-west-1%3A123456789012%3Aapplication-inference-profile%2Fa1b2c3d4e5f6",
		"arn%3Aaws%3Abedrock%3Aus-west-2%3A123456789012%3Aprompt%2FPROMPT12345%3A1",
		"my_marketplace_endpoint",
	}
	for _, segment := range segments {
		for _, op := range []string{"converse", "converse-stream"} {
			match := re.FindStringSubmatch("/bedrock/model/" + segment + "/" + op)
			if assert.Len(t, match, 2, "%s/%s", segment, op) {
				assert.Equal(t, segment, match[1], op)
			}
		}
	}
}

// sdk/ai/llmusage tests run the shipped template against Bedrock responses
// through a JSON copy; this keeps that copy identical to the YAML.
func TestAWSBedrockBuiltInTemplate_MatchesSDKGolden(t *testing.T) {
	shipped, ok := readYAMLAsJSONMap(t, bedrockGatewayTemplateFile)["spec"].(map[string]interface{})
	require.True(t, ok)

	raw, err := os.ReadFile(bedrockSDKGoldenSpec)
	require.NoError(t, err)
	var golden map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &golden))

	assert.Equal(t, golden, shipped,
		"update sdk/ai/llmusage/testdata/awsbedrock-template-spec.json to match the shipped template")
}

// The Platform API template models a subset of the gateway template's fields.
// Every field both declare must agree, and the identity fields must match so
// a provider created in the Platform API resolves to this gateway template.
func TestAWSBedrockBuiltInTemplate_ConsistentWithPlatformAPI(t *testing.T) {
	gateway := readYAMLAsJSONMap(t, bedrockGatewayTemplateFile)
	platform := readYAMLAsJSONMap(t, bedrockPlatformTemplate)

	assert.Equal(t, gateway["metadata"], platform["metadata"])
	gwSpec := gateway["spec"].(map[string]interface{})
	cpSpec := platform["spec"].(map[string]interface{})
	for _, key := range []string{"groupId", "displayName", "managedBy", "version"} {
		assert.Equal(t, gwSpec[key], cpSpec[key], key)
	}

	shared := []string{"promptTokens", "completionTokens", "totalTokens", "requestModel", "responseModel"}
	for _, field := range shared {
		gw, ok := gwSpec[field].(map[string]interface{})
		require.True(t, ok, "gateway template lacks %s", field)
		cp, ok := cpSpec[field].(map[string]interface{})
		require.True(t, ok, "platform template lacks %s", field)
		assert.Equal(t, gw["location"], cp["location"], field)
		assert.Equal(t, gw["identifier"], cp["identifier"], field)
	}

	// Fields only the gateway models. The Platform API loader has no schema for
	// them, so they stay out of its copy rather than being silently dropped.
	for _, field := range []string{"cachedTokens", "cacheWriteTokens", "serviceTier", "providerFields", "cacheAccounting"} {
		assert.Contains(t, gwSpec, field)
		assert.NotContains(t, cpSpec, field)
	}

	auth := cpSpec["metadata"].(map[string]interface{})["auth"].(map[string]interface{})
	assert.Equal(t, "apiKey", auth["type"])
	assert.Equal(t, "Authorization", auth["header"])
	assert.Equal(t, "Bearer ", auth["valuePrefix"])
}

func loadBedrockOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(bedrockOpenAPISpec)
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))
	return doc
}

func TestAWSBedrockOpenAPISpec_IsValidAndCurrent(t *testing.T) {
	doc := loadBedrockOpenAPI(t)

	converse := doc.Paths.Find(bedrockConversePath)
	require.NotNil(t, converse)
	require.NotNil(t, converse.Post)
	assert.Equal(t, "Converse", converse.Post.OperationID)
	stream := doc.Paths.Find(bedrockConverseStreamPath)
	require.NotNil(t, stream)
	require.NotNil(t, stream.Post)
	assert.Equal(t, "ConverseStream", stream.Post.OperationID)
	assert.NotNil(t, stream.Post.Responses.Status(200).Value.Content.Get("application/vnd.amazon.eventstream"))
	for _, code := range []int{400, 403, 404, 408, 424, 429, 500, 503} {
		assert.NotNil(t, converse.Post.Responses.Status(code), "converse %d", code)
		assert.NotNil(t, stream.Post.Responses.Status(code), "converse-stream %d", code)
	}

	// Both upstream auth methods, and no OAuth flow: Bedrock API keys are not
	// issued by a token endpoint.
	schemes := doc.Components.SecuritySchemes
	require.Contains(t, schemes, "awsSigV4")
	assert.Equal(t, "apiKey", schemes["awsSigV4"].Value.Type)
	assert.Equal(t, "Authorization", schemes["awsSigV4"].Value.Name)
	require.Contains(t, schemes, "bedrockApiKey")
	assert.Equal(t, "http", schemes["bedrockApiKey"].Value.Type)
	assert.Equal(t, "bearer", schemes["bedrockApiKey"].Value.Scheme)
	for name, scheme := range schemes {
		assert.NotEqual(t, "oauth2", scheme.Value.Type, name)
		assert.NotEqual(t, "openIdConnect", scheme.Value.Type, name)
	}
	assert.Len(t, doc.Security, 2, "either scheme alone satisfies a request")

	schemas := doc.Components.Schemas
	assert.Contains(t, schemas["DocumentBlock"].Value.Properties["format"].Value.Enum, "md")
	shared := schemas["ConverseRequestFields"].Value.Properties
	for _, field := range []string{"promptVariables", "requestMetadata", "additionalModelResponseFieldPaths", "serviceTier", "performanceConfig", "outputConfig"} {
		assert.Contains(t, shared, field)
	}
	assert.Contains(t, schemas["GuardrailStreamConfiguration"].Value.Properties, "streamProcessingMode")
	assert.NotContains(t, schemas["GuardrailConfiguration"].Value.Properties, "streamProcessingMode")
	for _, field := range []string{"cacheReadInputTokens", "cacheWriteInputTokens", "cacheDetails"} {
		assert.Contains(t, schemas["TokenUsage"].Value.Properties, field)
	}
	for _, field := range []string{"additionalModelResponseFields", "serviceTier", "performanceConfig", "trace"} {
		assert.Contains(t, schemas["ConverseResponse"].Value.Properties, field)
	}
	assert.Contains(t, schemas["ServiceTier"].Value.Properties["type"].Value.Enum, "reserved")
}

func TestAWSBedrockOpenAPISpec_SchemasAcceptAndRejectPayloads(t *testing.T) {
	schemas := loadBedrockOpenAPI(t).Components.Schemas
	decode := func(raw string) interface{} {
		var v interface{}
		require.NoError(t, json.Unmarshal([]byte(raw), &v))
		return v
	}

	tests := []struct {
		name    string
		schema  string
		payload string
		valid   bool
	}{
		{
			name:   "converse request with md document, guardrail, cache point and metadata",
			schema: "ConverseRequest",
			payload: `{
				"messages": [{"role": "user", "content": [
					{"text": "Summarize"},
					{"document": {"name": "Release notes", "format": "md", "source": {"bytes": "IyBUaXRsZQ=="}}},
					{"cachePoint": {"type": "default", "ttl": "1h"}}
				]}],
				"guardrailConfig": {"guardrailIdentifier": "gr1abc", "guardrailVersion": "DRAFT", "trace": "enabled"},
				"requestMetadata": {"team": "search"},
				"serviceTier": {"type": "flex"},
				"additionalModelResponseFieldPaths": ["/stop_sequence"]
			}`,
			valid: true,
		},
		{
			name:    "prompt management request without messages",
			schema:  "ConverseRequest",
			payload: `{"promptVariables": {"genre": {"text": "pop"}}}`,
			valid:   true,
		},
		{
			name:    "stream request with async guardrail processing",
			schema:  "ConverseStreamRequest",
			payload: `{"messages": [], "guardrailConfig": {"guardrailIdentifier": "gr1abc", "guardrailVersion": "1", "streamProcessingMode": "async"}}`,
			valid:   true,
		},
		{
			name:    "document source from s3",
			schema:  "DocumentBlock",
			payload: `{"name": "Q3 report", "format": "pdf", "source": {"s3Location": {"uri": "s3://my-bucket/q3.pdf", "bucketOwner": "123456789012"}}}`,
			valid:   true,
		},
		{
			name:    "document source with two members",
			schema:  "DocumentBlock",
			payload: `{"name": "doc", "source": {"text": "a", "bytes": "YQ=="}}`,
			valid:   false,
		},
		{
			name:    "document without name",
			schema:  "DocumentBlock",
			payload: `{"format": "txt", "source": {"text": "a"}}`,
			valid:   false,
		},
		{
			name:    "unsupported document format",
			schema:  "DocumentBlock",
			payload: `{"name": "doc", "format": "rtf", "source": {"text": "a"}}`,
			valid:   false,
		},
		{
			name:    "content block with two members",
			schema:  "ContentBlock",
			payload: `{"text": "a", "cachePoint": {"type": "default"}}`,
			valid:   false,
		},
		{
			name:    "s3 uri outside the s3 scheme",
			schema:  "S3Location",
			payload: `{"uri": "https://my-bucket.s3.amazonaws.com/q3.pdf"}`,
			valid:   false,
		},
		{
			name:   "converse response with cache usage and service tier",
			schema: "ConverseResponse",
			payload: `{
				"output": {"message": {"role": "assistant", "content": [{"text": "hi"}]}},
				"stopReason": "model_context_window_exceeded",
				"usage": {"inputTokens": 30, "outputTokens": 628, "totalTokens": 1158,
				          "cacheReadInputTokens": 400, "cacheWriteInputTokens": 100,
				          "cacheDetails": [{"ttl": "1h", "inputTokens": 60}, {"ttl": "5m", "inputTokens": 40}]},
				"metrics": {"latencyMs": 1275},
				"serviceTier": {"type": "priority"},
				"additionalModelResponseFields": {"stop_sequence": "SUCCESS"}
			}`,
			valid: true,
		},
		{
			name:    "stream metadata event",
			schema:  "ConverseStreamOutput",
			payload: `{"metadata": {"usage": {"inputTokens": 11, "outputTokens": 7, "totalTokens": 18, "cacheReadInputTokens": 0}, "metrics": {"latencyMs": 5}, "serviceTier": {"type": "default"}}}`,
			valid:   true,
		},
		{
			name:    "stream frame with two events",
			schema:  "ConverseStreamOutput",
			payload: `{"messageStart": {"role": "assistant"}, "messageStop": {"stopReason": "end_turn"}}`,
			valid:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, ok := schemas[tt.schema]
			require.True(t, ok, "schema %s", tt.schema)
			err := ref.Value.VisitJSON(decode(tt.payload))
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// loadBedrockExample reads a gateway example as an LlmProvider configuration.
func loadBedrockExample(t *testing.T, name string) *api.LLMProviderConfiguration {
	t.Helper()
	encoded, err := json.Marshal(readYAMLAsJSONMap(t, filepath.Join(bedrockExamplesDir, name)))
	require.NoError(t, err)
	var provider api.LLMProviderConfiguration
	require.NoError(t, json.Unmarshal(encoded, &provider))
	return &provider
}

// newBuiltInBedrockTransformer returns a transformer whose store holds the
// shipped awsbedrock template, with the policy versions a default gateway build
// resolves for the policies the Bedrock examples use.
func newBuiltInBedrockTransformer(t *testing.T) *LLMProviderTransformer {
	t.Helper()
	stored := &models.StoredLLMProviderTemplate{
		UUID:          "0000-awsbedrock-template-0000-000000000000",
		Configuration: *loadBuiltInBedrockTemplate(t),
	}
	store := storage.NewConfigStore()
	require.NoError(t, store.AddTemplate(stored))
	db := newTestMockDB()
	require.NoError(t, db.SaveLLMProviderTemplate(stored))
	cfg := loadDummyConfig()
	resolver := NewStaticPolicyVersionResolver(map[string]string{
		constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: testSetHeadersVersion,
		constants.ACCESS_CONTROL_DENY_POLICY_NAME:  testRespondVersion,
		awsAuthenticationPolicy:                    "v0",
	})
	return NewLLMProviderTransformer(store, db, &cfg, resolver)
}

// bedrockOperationPolicies returns the POST policy chain for each Converse
// operation, failing if either route is missing.
func bedrockOperationPolicies(t *testing.T, result *api.RestAPI) map[string][]api.Policy {
	t.Helper()
	chains := map[string][]api.Policy{}
	for _, op := range result.Spec.Operations {
		if op.Method == nil || string(*op.Method) != "POST" || op.Policies == nil {
			continue
		}
		path := op.EffectivePath()
		if path == bedrockConversePath || path == bedrockConverseStreamPath {
			chains[path] = *op.Policies
		}
	}
	require.Contains(t, chains, bedrockConversePath)
	require.Contains(t, chains, bedrockConverseStreamPath)
	return chains
}

func policyNames(chain []api.Policy) []string {
	names := make([]string, 0, len(chain))
	for _, p := range chain {
		names = append(names, p.Name)
	}
	return names
}

func TestAWSBedrockProvider_BearerKeyExample(t *testing.T) {
	provider := loadBedrockExample(t, "bedrock-provider.yaml")
	require.Empty(t, config.NewLLMValidator().Validate(provider))
	require.Equal(t, "awsbedrock", provider.Spec.Template)

	result, err := newBuiltInBedrockTransformer(t).Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	for path, chain := range bedrockOperationPolicies(t, result) {
		require.NotEmpty(t, chain, path)
		last := chain[len(chain)-1]
		assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, last.Name, path)
		require.NotNil(t, last.Params)
		headers := (*last.Params)["request"].(map[string]interface{})["headers"].([]interface{})
		header := headers[0].(map[string]interface{})
		assert.Equal(t, "Authorization", header["name"], path)
		assert.True(t, strings.HasPrefix(header["value"].(string), "Bearer "), path)
		assert.NotContains(t, policyNames(chain), awsAuthenticationPolicy, path)

		// The downstream api-key-auth policy carries the template's extraction
		// fields, including the path-based model identifier.
		first := chain[0]
		assert.Equal(t, "api-key-auth", first.Name, path)
		model := (*first.Params)["requestModel"].(map[string]interface{})
		assert.Equal(t, provider.Spec.Template, "awsbedrock")
		assert.Equal(t, loadBuiltInBedrockTemplate(t).Spec.RequestModel.Identifier, model["identifier"], path)
	}
}

func TestAWSBedrockProvider_SigV4Example(t *testing.T) {
	provider := loadBedrockExample(t, "bedrock-provider-sigv4.yaml")
	require.Empty(t, config.NewLLMValidator().Validate(provider))

	auth := provider.Spec.Upstream.Auth
	require.NotNil(t, auth)
	require.NotNil(t, auth.PolicyParams)
	params := *auth.PolicyParams
	assert.Equal(t, "bedrock", params["service"], "SigV4 signing name for bedrock-runtime")
	assert.Equal(t, "default-credential-chain", params["authenticationType"])
	assert.Contains(t, *provider.Spec.Upstream.Url, ".amazonaws.com")
	assert.Contains(t, *provider.Spec.Upstream.Url, "bedrock-runtime."+params["region"].(string)+".",
		"the signing region must match the endpoint region")
	for _, secret := range []string{"awsAccessKeyID", "awsSecretAccessKey", "awsSessionToken"} {
		assert.NotContains(t, params, secret, "credentials must come from the default chain")
	}
	assert.Nil(t, auth.Header)
	assert.Nil(t, auth.Value)

	result, err := newBuiltInBedrockTransformer(t).Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	for path, chain := range bedrockOperationPolicies(t, result) {
		require.NotEmpty(t, chain, path)
		// SigV4 signs the body, so the signer must run after every other policy.
		last := chain[len(chain)-1]
		assert.Equal(t, awsAuthenticationPolicy, last.Name, path)
		assert.Equal(t, "v0", last.Version, path)
		require.NotNil(t, last.Params)
		assert.Equal(t, params, *last.Params, path)
		assert.NotContains(t, policyNames(chain), constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, path)
		assert.Equal(t, 1, strings.Count(strings.Join(policyNames(chain), ","), awsAuthenticationPolicy), path)
	}
}

func TestAWSBedrockProvider_SigV4RequiresPolicyNameAndParams(t *testing.T) {
	validator := config.NewLLMValidator()

	noName := loadBedrockExample(t, "bedrock-provider-sigv4.yaml")
	noName.Spec.Upstream.Auth.PolicyName = nil
	assert.NotEmpty(t, validator.Validate(noName), "auth type 'other' needs a policyName")

	noParams := loadBedrockExample(t, "bedrock-provider-sigv4.yaml")
	noParams.Spec.Upstream.Auth.PolicyParams = nil
	assert.NotEmpty(t, validator.Validate(noParams), "auth type 'other' needs policyParams")
}

// A gateway build without the aws-authentication policy must refuse the
// provider rather than deploy it unsigned.
func TestAWSBedrockProvider_SigV4FailsWithoutPolicyDefinition(t *testing.T) {
	stored := &models.StoredLLMProviderTemplate{
		UUID:          "0000-awsbedrock-template-0000-000000000000",
		Configuration: *loadBuiltInBedrockTemplate(t),
	}
	store := storage.NewConfigStore()
	require.NoError(t, store.AddTemplate(stored))
	db := newTestMockDB()
	require.NoError(t, db.SaveLLMProviderTemplate(stored))
	cfg := loadDummyConfig()
	transformer := NewLLMProviderTransformer(store, db, &cfg, NewStaticPolicyVersionResolver(map[string]string{
		constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: testSetHeadersVersion,
	}))

	_, err := transformer.Transform(loadBedrockExample(t, "bedrock-provider-sigv4.yaml"), &api.RestAPI{})
	require.Error(t, err)
	assert.True(t, IsPolicyDefinitionMissingError(err), "got %v", err)
}
