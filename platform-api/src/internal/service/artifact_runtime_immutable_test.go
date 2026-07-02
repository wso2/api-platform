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

package service

import (
	"errors"
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

func rtStrPtr(s string) *string { return &s }

// TestEnsureRuntimeArtifactUnchanged covers the shared guard: CP artifacts are always
// mutable, DP artifacts are mutable only when the runtime artifact is byte-identical,
// and a rejection is the distinct ErrArtifactRuntimeImmutable (mapped to 403 separately).
func TestEnsureRuntimeArtifactUnchanged(t *testing.T) {
	a := []byte("artifact-a")
	b := []byte("artifact-b")

	if err := ensureRuntimeArtifactUnchanged(constants.OriginCP, a, b); err != nil {
		t.Errorf("CP origin with differing artifacts: got %v, want nil", err)
	}
	if err := ensureRuntimeArtifactUnchanged(constants.OriginDP, a, a); err != nil {
		t.Errorf("DP origin with identical artifacts: got %v, want nil", err)
	}
	err := ensureRuntimeArtifactUnchanged(constants.OriginDP, a, b)
	if !errors.Is(err, constants.ErrArtifactRuntimeImmutable) {
		t.Errorf("DP origin with differing artifacts: got %v, want ErrArtifactRuntimeImmutable", err)
	}
	// It is a distinct sentinel, NOT wrapping the blanket read-only error.
	if errors.Is(err, constants.ErrArtifactReadOnly) {
		t.Errorf("ErrArtifactRuntimeImmutable must be distinct from ErrArtifactReadOnly")
	}
}

func baseRESTAPI(origin string) *model.API {
	return &model.API{
		ID:             "uuid-1",
		Handle:         "my-api",
		Name:           "My API",
		Kind:           constants.RestApi,
		Version:        "v1.0",
		ProjectID:      "proj-1",
		OrganizationID: "org-1",
		Origin:         origin,
		Configuration: model.RestAPIConfig{
			Name:      "My API",
			Version:   "v1.0",
			Context:   rtStrPtr("/my-api"),
			Transport: []string{"http", "https"},
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://backend.example.com"},
			},
			Operations: []model.Operation{
				{Name: "get", Request: &model.OperationRequest{Method: "GET", Path: "/items"}},
			},
		},
	}
}

func TestRESTRuntimeArtifactGuard(t *testing.T) {
	svc := &APIService{apiUtil: &utils.APIUtil{}, identity: newTestIdentityService()}

	t.Run("metadata-only edit allowed", func(t *testing.T) {
		existing := baseRESTAPI(constants.OriginDP)
		updated := baseRESTAPI(constants.OriginDP)
		updated.Description = "a shiny new description"     // not in the deployment YAML
		updated.LifeCycleStatus = "PUBLISHED"               // not in the deployment YAML
		updated.Configuration.Transport = []string{"https"} // not in the deployment YAML
		if err := svc.ensureRESTRuntimeArtifactUnchanged(existing, updated); err != nil {
			t.Errorf("metadata-only edit rejected: %v", err)
		}
	})

	t.Run("upstream edit rejected", func(t *testing.T) {
		existing := baseRESTAPI(constants.OriginDP)
		updated := baseRESTAPI(constants.OriginDP)
		updated.Configuration.Upstream.Main.URL = "https://new-backend.example.com"
		if err := svc.ensureRESTRuntimeArtifactUnchanged(existing, updated); !errors.Is(err, constants.ErrArtifactRuntimeImmutable) {
			t.Errorf("upstream edit: got %v, want read-only", err)
		}
	})

	t.Run("operation edit rejected", func(t *testing.T) {
		existing := baseRESTAPI(constants.OriginDP)
		updated := baseRESTAPI(constants.OriginDP)
		updated.Configuration.Operations = append(updated.Configuration.Operations,
			model.Operation{Name: "post", Request: &model.OperationRequest{Method: "POST", Path: "/items"}})
		if err := svc.ensureRESTRuntimeArtifactUnchanged(existing, updated); !errors.Is(err, constants.ErrArtifactRuntimeImmutable) {
			t.Errorf("operation edit: got %v, want read-only", err)
		}
	})

	t.Run("CP artifact always mutable", func(t *testing.T) {
		existing := baseRESTAPI(constants.OriginCP)
		updated := baseRESTAPI(constants.OriginCP)
		updated.Configuration.Upstream.Main.URL = "https://new-backend.example.com"
		if err := svc.ensureRESTRuntimeArtifactUnchanged(existing, updated); err != nil {
			t.Errorf("CP artifact runtime edit rejected: %v", err)
		}
	})
}

func baseMCPProxy(origin string) *model.MCPProxy {
	projectID := "proj-1"
	return &model.MCPProxy{
		UUID:             "uuid-1",
		Handle:           "my-mcp",
		OrganizationUUID: "org-1",
		ProjectUUID:      &projectID,
		Name:             "My MCP",
		Version:          "v1.0",
		Origin:           origin,
		Configuration: model.MCPProxyConfiguration{
			Name:        "My MCP",
			Version:     "v1.0",
			Context:     rtStrPtr("/my-mcp"),
			SpecVersion: "2025-06-18",
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://mcp.example.com"},
			},
		},
	}
}

// mcpFingerprint mirrors the inline diff in MCPProxyService.Update.
func mcpFingerprint(t *testing.T, proxy *model.MCPProxy) []byte {
	t.Helper()
	artifact, err := (&utils.MCPUtils{}).BuildMCPDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("BuildMCPDeploymentYAML: %v", err)
	}
	fp, err := runtimeArtifactFingerprint(artifact)
	if err != nil {
		t.Fatalf("runtimeArtifactFingerprint: %v", err)
	}
	return fp
}

func TestMCPProxyRuntimeArtifactGuard(t *testing.T) {
	t.Run("metadata-only edit allowed", func(t *testing.T) {
		existing := baseMCPProxy(constants.OriginDP)
		existingFP := mcpFingerprint(t, existing)
		existing.Description = "new description"                            // not in the deployment spec
		existing.Configuration.Capabilities = &model.MCPProxyCapabilities{} // fetched out-of-band, not in spec
		updatedFP := mcpFingerprint(t, existing)
		if err := ensureRuntimeArtifactUnchanged(constants.OriginDP, existingFP, updatedFP); err != nil {
			t.Errorf("metadata-only edit rejected: %v", err)
		}
	})

	t.Run("context edit rejected", func(t *testing.T) {
		existing := baseMCPProxy(constants.OriginDP)
		existingFP := mcpFingerprint(t, existing)
		existing.Configuration.Context = rtStrPtr("/my-mcp-v2")
		updatedFP := mcpFingerprint(t, existing)
		if err := ensureRuntimeArtifactUnchanged(constants.OriginDP, existingFP, updatedFP); !errors.Is(err, constants.ErrArtifactRuntimeImmutable) {
			t.Errorf("context edit: got %v, want read-only", err)
		}
	})
}

func baseTemplate(origin string) *model.LLMProviderTemplate {
	return &model.LLMProviderTemplate{
		UUID:         "uuid-1",
		ID:           "openai-template",
		Name:         "OpenAI Template",
		Version:      "v1.0",
		Origin:       origin,
		Metadata:     &model.LLMProviderTemplateMetadata{EndpointURL: "https://api.openai.com/v1/chat/completions"},
		PromptTokens: &model.ExtractionIdentifier{Location: "body", Identifier: "usage.prompt_tokens"},
	}
}

func TestTemplateRuntimeArtifactGuard(t *testing.T) {
	t.Run("name and description edit allowed", func(t *testing.T) {
		existing := baseTemplate(constants.OriginDP)
		updated := baseTemplate(constants.OriginDP)
		updated.Name = "Renamed Template"
		updated.Description = "new description"
		updated.ManagedBy = "someone"
		updated.OpenAPISpec = "{...}"
		if err := ensureTemplateRuntimeArtifactUnchanged(existing, updated); err != nil {
			t.Errorf("name/description edit rejected: %v", err)
		}
	})

	t.Run("extraction identifier edit rejected", func(t *testing.T) {
		existing := baseTemplate(constants.OriginDP)
		updated := baseTemplate(constants.OriginDP)
		updated.PromptTokens = &model.ExtractionIdentifier{Location: "header", Identifier: "x-prompt-tokens"}
		if err := ensureTemplateRuntimeArtifactUnchanged(existing, updated); !errors.Is(err, constants.ErrArtifactRuntimeImmutable) {
			t.Errorf("extraction identifier edit: got %v, want read-only", err)
		}
	})

	t.Run("resource mapping edit rejected", func(t *testing.T) {
		existing := baseTemplate(constants.OriginDP)
		updated := baseTemplate(constants.OriginDP)
		updated.ResourceMappings = &model.LLMProviderTemplateResourceMappings{
			Resources: []model.LLMProviderTemplateResourceMapping{{
				Resource: "/responses",
				LLMProviderTemplateExtractionFields: model.LLMProviderTemplateExtractionFields{
					PromptTokens: &model.ExtractionIdentifier{Location: "payload", Identifier: "$.usage.input_tokens"},
				},
			}},
		}
		if err := ensureTemplateRuntimeArtifactUnchanged(existing, updated); !errors.Is(err, constants.ErrArtifactRuntimeImmutable) {
			t.Errorf("resource mapping edit: got %v, want read-only", err)
		}
	})

	t.Run("metadata edit allowed (not consumed by gateway runtime)", func(t *testing.T) {
		existing := baseTemplate(constants.OriginDP)
		updated := baseTemplate(constants.OriginDP)
		// endpointUrl / openapiSpecUrl / logoUrl / auth are authoring/reference/display
		// data the gateway never reads at request time, so editing them is allowed.
		updated.Metadata = &model.LLMProviderTemplateMetadata{
			EndpointURL:    "https://api.openai.com/v2/chat",
			OpenapiSpecURL: "https://example.com/openapi.yaml",
			LogoURL:        "https://example.com/logo.png",
		}
		if err := ensureTemplateRuntimeArtifactUnchanged(existing, updated); err != nil {
			t.Errorf("metadata edit rejected: %v", err)
		}
	})

	t.Run("CP artifact always mutable", func(t *testing.T) {
		existing := baseTemplate(constants.OriginCP)
		updated := baseTemplate(constants.OriginCP)
		updated.PromptTokens = &model.ExtractionIdentifier{Location: "header", Identifier: "x-prompt-tokens"}
		if err := ensureTemplateRuntimeArtifactUnchanged(existing, updated); err != nil {
			t.Errorf("CP artifact runtime edit rejected: %v", err)
		}
	})
}
