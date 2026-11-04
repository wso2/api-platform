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
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	bedrockTestEndpoint       = "https://bedrock-runtime.us-east-1.amazonaws.com"
	bedrockConversePath       = "/model/{modelId}/converse"
	bedrockConverseStreamPath = "/model/{modelId}/converse-stream"
)

// bedrockSigV4Params configures aws-authentication to sign for Bedrock with the
// gateway's default AWS credential chain; no credential is part of the config.
func bedrockSigV4Params() map[string]interface{} {
	return map[string]interface{}{
		"service":            "bedrock",
		"region":             "us-east-1",
		"authenticationType": "default-credential-chain",
	}
}

func bedrockSigV4OperationPolicy() api.OperationPolicy {
	return api.OperationPolicy{
		Name:    "aws-authentication",
		Version: "v0",
		Paths: []api.OperationPolicyPath{
			{Path: bedrockConversePath, Methods: []api.OperationPolicyPathMethods{"POST"}, Params: bedrockSigV4Params()},
			{Path: bedrockConverseStreamPath, Methods: []api.OperationPolicyPathMethods{"POST"}, Params: bedrockSigV4Params()},
		},
	}
}

func newBedrockProviderService(now time.Time) (*LLMProviderService, *mockLLMProviderRepo) {
	providerRepo := &mockLLMProviderRepo{}
	providerRepo.getByIDFunc = func(providerID, orgUUID string) (*model.LLMProvider, error) {
		if providerRepo.created == nil {
			return nil, nil
		}
		created := *providerRepo.created
		created.UUID = "prov-uuid"
		created.CreatedAt = now
		created.UpdatedAt = now
		return &created, nil
	}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-bedrock", ID: "awsbedrock", Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1"}}
	svc := NewLLMProviderService(providerRepo, templateRepo, orgRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	return svc, providerRepo
}

func bedrockProviderRequest(auth *api.UpstreamAuth) *api.LLMProvider {
	req := validProviderRequest("awsbedrock")
	req.Upstream = api.Upstream{Main: api.UpstreamDefinition{Url: stringPtr(bedrockTestEndpoint), Auth: auth}}
	req.AccessControl = api.LLMAccessControl{
		Mode: "deny_all",
		Exceptions: &[]api.RouteException{
			{Path: bedrockConversePath, Methods: []api.RouteExceptionMethods{"POST"}},
			{Path: bedrockConverseStreamPath, Methods: []api.RouteExceptionMethods{"POST"}},
		},
	}
	return req
}

func TestLLMProviderServiceCreate_AWSBedrockBearerKey(t *testing.T) {
	svc, repo := newBedrockProviderService(time.Now())
	request := bedrockProviderRequest(&api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr("Bearer test-bedrock-api-key"),
	})

	if _, err := svc.Create("org-1", "alice", request); err != nil {
		t.Fatalf("Create: %v", err)
	}
	auth := repo.created.Configuration.Upstream.Main.Auth
	if auth == nil || auth.Type != "api-key" || auth.Header != "Authorization" || auth.Value != "Bearer test-bedrock-api-key" {
		t.Fatalf("stored upstream auth = %+v, want api-key Authorization: Bearer <key>", auth)
	}

	artifact, err := generateLLMProviderDeploymentYAML(repo.created, "awsbedrock")
	if err != nil {
		t.Fatalf("generateLLMProviderDeploymentYAML: %v", err)
	}
	if artifact.Spec.Template != "awsbedrock" {
		t.Errorf("template = %q, want awsbedrock", artifact.Spec.Template)
	}
	got := artifact.Spec.Upstream.Auth
	if got == nil || got.Type == nil || *got.Type != "api-key" ||
		got.Header == nil || *got.Header != "Authorization" ||
		got.Value == nil || *got.Value != "Bearer test-bedrock-api-key" {
		t.Fatalf("deployed upstream auth = %+v, want api-key Authorization: Bearer <key>", got)
	}
}

// SigV4 for a Platform API managed provider: no upstream credential, and the
// aws-authentication policy attached to both Converse operations.
func TestLLMProviderServiceCreate_AWSBedrockSigV4(t *testing.T) {
	svc, repo := newBedrockProviderService(time.Now())
	request := bedrockProviderRequest(&api.UpstreamAuth{Type: upstreamAuthTypePtr("none")})
	request.OperationPolicies = &[]api.OperationPolicy{bedrockSigV4OperationPolicy()}

	if _, err := svc.Create("org-1", "alice", request); err != nil {
		t.Fatalf("Create: %v", err)
	}
	stored := repo.created.Configuration
	if auth := stored.Upstream.Main.Auth; auth == nil || auth.Type != "none" || auth.Header != "" || auth.Value != "" {
		t.Fatalf("stored upstream auth = %+v, want type none with no credential", auth)
	}
	if len(stored.OperationPolicies) != 1 || stored.OperationPolicies[0].Name != "aws-authentication" {
		t.Fatalf("stored operation policies = %+v, want aws-authentication", stored.OperationPolicies)
	}

	artifact, err := generateLLMProviderDeploymentYAML(repo.created, "awsbedrock")
	if err != nil {
		t.Fatalf("generateLLMProviderDeploymentYAML: %v", err)
	}
	if got := artifact.Spec.Upstream.Auth; got == nil || got.Type == nil || *got.Type != "none" || got.Header != nil || got.Value != nil {
		t.Fatalf("deployed upstream auth = %+v, want type none only", got)
	}

	var signer *api.OperationPolicy
	for i := range artifact.Spec.OperationPolicies {
		if artifact.Spec.OperationPolicies[i].Name == "aws-authentication" {
			signer = &artifact.Spec.OperationPolicies[i]
		}
	}
	if signer == nil {
		t.Fatalf("deployment has no aws-authentication policy: %+v", artifact.Spec.OperationPolicies)
	}
	if signer.Version != "v0" {
		t.Errorf("version = %q, want v0", signer.Version)
	}
	paths := map[string]map[string]interface{}{}
	for _, p := range signer.Paths {
		paths[p.Path] = p.Params
	}
	for _, path := range []string{bedrockConversePath, bedrockConverseStreamPath} {
		params, ok := paths[path]
		if !ok {
			t.Errorf("aws-authentication not attached to %s", path)
			continue
		}
		if params["service"] != "bedrock" || params["region"] != "us-east-1" || params["authenticationType"] != "default-credential-chain" {
			t.Errorf("%s params = %v", path, params)
		}
	}

	out, err := yaml.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal deployment: %v", err)
	}
	for _, secret := range []string{"awsAccessKeyID", "awsSecretAccessKey", "awsSessionToken"} {
		if strings.Contains(string(out), secret) {
			t.Errorf("deployment YAML contains %s; SigV4 must use the default credential chain", secret)
		}
	}
}
