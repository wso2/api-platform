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
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// clusterDenyRule is the deny rule these tests run under: in-cluster Kubernetes service
// names are refused, everything else is allowed except the always-refused addresses.
var clusterDenyRule = netguard.GuardConfig{
	Enabled:   true,
	Mode:      "deny",
	Hostnames: []string{`.*\.svc\.cluster\.local`},
}

// useRequestAccessControl installs server.network_security.access_control built from gc
// for the duration of the test. Tests that use it must not run in parallel.
func useRequestAccessControl(t *testing.T, gc netguard.GuardConfig) {
	t.Helper()
	policy, err := gc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cfg := testSharedHTTPClientConfig()
	cfg.SSRF.Policy = policy
	if err := utils.InitUserURLClientFromConfig(cfg); err != nil {
		t.Fatalf("InitUserURLClientFromConfig: %v", err)
	}
	t.Cleanup(func() {
		if err := utils.InitSharedHTTPClientFromConfig(testSharedHTTPClientConfig(), 0); err != nil {
			t.Errorf("restore the shared HTTP client: %v", err)
		}
	})
}

// assertRefusedField fails unless err is DESTINATION_NOT_ALLOWED naming exactly field,
// with a message that names neither the host nor the rule.
func assertRefusedField(t *testing.T, err error, field string) {
	t.Helper()
	if !apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("err = %v, want DESTINATION_NOT_ALLOWED", err)
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("err = %T, want *apperror.Error", err)
	}
	want := []apperror.FieldError{{Field: field, Message: refusedFieldMessage}}
	if !reflect.DeepEqual(appErr.FieldErrors, want) {
		t.Fatalf("field errors = %+v, want %+v", appErr.FieldErrors, want)
	}
	for _, leak := range []string{"cluster.local", "svc", "169.254", "netguard"} {
		if strings.Contains(appErr.Message, leak) || strings.Contains(appErr.FieldErrors[0].Message, leak) {
			t.Fatalf("client-facing message %q / %q names %q", appErr.Message, appErr.FieldErrors[0].Message, leak)
		}
	}
}

func upstreamOf(main, sandbox string) api.Upstream {
	up := api.Upstream{Main: api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty(main)}}
	if sandbox != "" {
		up.Sandbox = &api.UpstreamDefinition{Url: utils.StringPtrIfNotEmpty(sandbox)}
	}
	return up
}

func endpointOverride(gatewayID, endpoint string) *[]api.AssociatedGateway {
	conf := map[string]interface{}{"endpointUrl": endpoint}
	return &[]api.AssociatedGateway{{Id: gatewayID, Configurations: &conf}}
}

// Each artifact kind's own request type is used, so a renamed JSON field fails here.
func TestCheckRequestURLs_RefusesEveryURLFieldOfEachKind(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	const refused = "http://orders.default.svc.cluster.local:8080"
	const allowed = "https://backend.example.com"

	cases := []struct {
		name    string
		payload any
		field   string
	}{
		{"REST main upstream", &api.CreateRESTAPIRequest{Upstream: upstreamOf(refused, "")}, "upstream.main.url"},
		{"REST sandbox upstream", &api.CreateRESTAPIRequest{Upstream: upstreamOf(allowed, refused)}, "upstream.sandbox.url"},
		{"GraphQL upstream", &api.CreateGraphQLAPIRequest{Upstream: upstreamOf(refused, "")}, "upstream.main.url"},
		{"GraphQL SDL URL", &api.CreateGraphQLAPIRequest{Upstream: upstreamOf(allowed, ""), SdlUrl: utils.StringPtrIfNotEmpty(refused + "/schema.graphql")}, "sdlUrl"},
		{"MCP proxy upstream", &api.MCPProxy{Upstream: upstreamOf(refused, "")}, "upstream.main.url"},
		{"MCP proxy gateway override", &api.MCPProxy{Upstream: upstreamOf(allowed, ""), AssociatedGateways: endpointOverride("gw-1", refused)}, "associatedGateways[0].configurations.endpointUrl"},
		{"LLM provider upstream", &api.LLMProvider{Upstream: upstreamOf(refused, "")}, "upstream.main.url"},
		{"LLM provider gateway override", &api.LLMProvider{Upstream: upstreamOf(allowed, ""), AssociatedGateways: endpointOverride("gw-1", refused)}, "associatedGateways[0].configurations.endpointUrl"},
		{"Agent proxy upstream", &api.A2AAgentProxy{Upstream: upstreamOf(refused, "")}, "upstream.main.url"},
		{"LLM template endpoint", &api.LLMProviderTemplate{Metadata: &api.LLMProviderTemplateMetadata{EndpointUrl: utils.StringPtrIfNotEmpty(refused)}}, "metadata.endpointUrl"},
		{"LLM template spec", &api.LLMProviderTemplate{Metadata: &api.LLMProviderTemplateMetadata{OpenapiSpecUrl: utils.StringPtrIfNotEmpty(refused + "/openapi.yaml")}}, "metadata.openapiSpecUrl"},
		{"LLM template version spec", &api.CreateLLMProviderTemplateVersionRequest{Metadata: &api.LLMProviderTemplateMetadata{OpenapiSpecUrl: utils.StringPtrIfNotEmpty(refused + "/openapi.yaml")}}, "metadata.openapiSpecUrl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertRefusedField(t, CheckRequestURLs(context.Background(), "org-1", tc.payload), tc.field)
		})
	}
}

// The positive control for every case above: the same shapes with permitted URLs pass.
func TestCheckRequestURLs_AllowsPermittedURLs(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	const allowed = "https://backend.example.com"
	for name, payload := range map[string]any{
		"REST":           &api.CreateRESTAPIRequest{Upstream: upstreamOf(allowed, allowed+"/sandbox")},
		"GraphQL":        &api.CreateGraphQLAPIRequest{Upstream: upstreamOf(allowed, ""), SdlUrl: utils.StringPtrIfNotEmpty(allowed + "/schema.graphql")},
		"MCP override":   &api.MCPProxy{Upstream: upstreamOf(allowed, ""), AssociatedGateways: endpointOverride("gw-1", allowed+"/mcp")},
		"LLM template":   &api.LLMProviderTemplate{Metadata: &api.LLMProviderTemplateMetadata{EndpointUrl: utils.StringPtrIfNotEmpty(allowed), OpenapiSpecUrl: utils.StringPtrIfNotEmpty(allowed + "/openapi.yaml")}},
		"agent upstream": &api.A2AAgentProxy{Upstream: upstreamOf(allowed, "")},
		// The pattern is anchored: a host that only contains the suffix is not refused.
		"suffix look-alike": &api.CreateRESTAPIRequest{Upstream: upstreamOf("https://orders.svc.cluster.local.example.com", "")},
	} {
		if err := CheckRequestURLs(context.Background(), "org-1", payload); err != nil {
			t.Errorf("%s: err = %v, want nil", name, err)
		}
	}
}

// Fields that are out of scope by design are never refused, even when they name a
// refused host; each is paired with a field of the same request that is refused.
func TestCheckRequestURLs_LeavesOutOfScopeFieldsAlone(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	const refused = "http://orders.default.svc.cluster.local"

	logo := &api.LLMProviderTemplate{Metadata: &api.LLMProviderTemplateMetadata{LogoUrl: utils.StringPtrIfNotEmpty(refused + "/logo.png")}}
	if err := CheckRequestURLs(context.Background(), "org-1", logo); err != nil {
		t.Errorf("a template logoUrl is only displayed: err = %v, want nil", err)
	}
	logo.Metadata.EndpointUrl = utils.StringPtrIfNotEmpty(refused)
	assertRefusedField(t, CheckRequestURLs(context.Background(), "org-1", logo), "metadata.endpointUrl")

	// A URL built from a placeholder is checked by the gateway once it is resolved.
	templated := &api.CreateRESTAPIRequest{Upstream: upstreamOf(`{{ secret "backend-url" }}`, "")}
	if err := CheckRequestURLs(context.Background(), "org-1", templated); err != nil {
		t.Errorf("a templated upstream: err = %v, want nil", err)
	}

	// Policy parameters are not checked by platform-api.
	params := map[string]interface{}{"endpoint": refused}
	withPolicy := &api.CreateRESTAPIRequest{
		Upstream: upstreamOf("https://backend.example.com", ""),
		Policies: &[]api.Policy{{Name: "interceptor-service", Version: "v1", Params: &params}},
	}
	if err := CheckRequestURLs(context.Background(), "org-1", withPolicy); err != nil {
		t.Errorf("a policy parameter: err = %v, want nil", err)
	}
}

// The always-refused addresses apply with no hostname rule configured at all.
func TestCheckRequestURLs_RefusesTheMetadataAddressByDefault(t *testing.T) {
	useRequestAccessControl(t, netguard.GuardConfig{Enabled: true, Mode: "deny"})
	assertRefusedField(t,
		CheckRequestURLs(context.Background(), "org-1", &api.CreateRESTAPIRequest{Upstream: upstreamOf("http://169.254.169.254/latest", "")}),
		"upstream.main.url")
	if err := CheckRequestURLs(context.Background(), "org-1", &api.CreateRESTAPIRequest{Upstream: upstreamOf("http://10.0.0.5:8080", "")}); err != nil {
		t.Errorf("a private address with deny_private_networks off: err = %v, want nil", err)
	}
}

// With access control off, nothing is refused.
func TestCheckRequestURLs_OffWhenAccessControlIsOff(t *testing.T) {
	payload := &api.CreateRESTAPIRequest{Upstream: upstreamOf("http://orders.default.svc.cluster.local", "")}
	if err := CheckRequestURLs(context.Background(), "org-1", payload); err != nil {
		t.Errorf("err = %v, want nil with no access-control client installed", err)
	}
	var nilReq *api.CreateRESTAPIRequest
	if err := CheckRequestURLs(context.Background(), "org-1", nilReq); err != nil {
		t.Errorf("nil request: err = %v, want nil", err)
	}
}

// The refusal reaches the REST API service's create and update, and nothing is stored or
// changed; the same API with a permitted upstream is created and updated.
func TestAPIService_RefusesUpstreamsAccessControlRefuses(t *testing.T) {
	apiSvc, _, cleanup := setupAPISecretTestEnv(t)
	defer cleanup()
	useRequestAccessControl(t, clusterDenyRule)

	refusedReq := &api.CreateRESTAPIRequest{
		Id: utils.StringPtrIfNotEmpty("netac-refused"), DisplayName: "Refused", Context: "/netac-refused",
		Version: "v1", ProjectId: "api-secret-it-proj",
		Upstream: upstreamOf("http://orders.default.svc.cluster.local", ""),
	}
	_, _, err := apiSvc.CreateAPI(refusedReq, apiSecretITOrgUUID, "alice")
	assertRefusedField(t, err, "upstream.main.url")
	if got, err := apiSvc.GetAPIByHandle("netac-refused", apiSecretITOrgUUID); err == nil && got != nil {
		t.Fatal("a refused create stored the API")
	}

	allowedReq := &api.CreateRESTAPIRequest{
		Id: utils.StringPtrIfNotEmpty("netac-allowed"), DisplayName: "Allowed", Context: "/netac-allowed",
		Version: "v1", ProjectId: "api-secret-it-proj",
		Upstream: upstreamOf("https://backend.example.com", ""),
	}
	created, apiUUID, err := apiSvc.CreateAPI(allowedReq, apiSecretITOrgUUID, "alice")
	if err != nil {
		t.Fatalf("CreateAPI with a permitted upstream: %v", err)
	}

	update := *created
	update.Upstream = upstreamOf("https://backend.example.com", "http://orders.default.svc.cluster.local")
	_, err = apiSvc.UpdateAPI(apiUUID, &update, apiSecretITOrgUUID, "alice")
	assertRefusedField(t, err, "upstream.sandbox.url")
	stored, err := apiSvc.GetAPIByUUID(apiUUID, apiSecretITOrgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID: %v", err)
	}
	if stored.Upstream.Sandbox != nil && stored.Upstream.Sandbox.Url != nil && *stored.Upstream.Sandbox.Url != "" {
		t.Fatalf("a refused update stored sandbox URL %q", *stored.Upstream.Sandbox.Url)
	}

	update.Upstream = upstreamOf("https://backend.example.com", "https://sandbox.example.com")
	if _, err := apiSvc.UpdateAPI(apiUUID, &update, apiSecretITOrgUUID, "alice"); err != nil {
		t.Fatalf("UpdateAPI with a permitted sandbox: %v", err)
	}
}
