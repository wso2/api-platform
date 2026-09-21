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

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// providerWithUpstreamAuth is a rendered provider definition whose upstream
// authenticates with the given type and credential. authType "" means the upstream
// carries no auth block at all.
func providerWithUpstreamAuth(authType, value string) *dto.LLMProviderDeploymentYAML {
	out := &dto.LLMProviderDeploymentYAML{
		Spec: dto.LLMProviderDeploymentSpec{
			Upstream: dto.LLMUpstreamYAML{URL: "https://api.openai.com"},
		},
	}
	if authType == "" {
		return out
	}
	t := api.UpstreamAuthType(authType)
	auth := &api.UpstreamAuth{Type: &t}
	if value != "" {
		v := value
		auth.Value = &v
	}
	out.Spec.Upstream.Auth = auth
	return out
}

// upstreamValue reads back the credential a rendered definition carries.
func upstreamValue(d *dto.LLMProviderDeploymentYAML) string {
	if d.Spec.Upstream.Auth == nil || d.Spec.Upstream.Auth.Value == nil {
		return ""
	}
	return *d.Spec.Upstream.Auth.Value
}

// A deployment's own credential replaces the provider's, which is what lets one
// provider run on several gateways against different accounts with the same vendor.
func TestUpstreamAuthOverride_ReplacesTheProvidersOwnCredential(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", `{{ secret "shared-key" }}`)

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "eu-key" }}`,
	}, "org-1")
	if err != nil {
		t.Fatalf("applying the override: %v", err)
	}

	if got := upstreamValue(rendered); got != `{{ secret "eu-key" }}` {
		t.Errorf("credential is %q, want the deployment's own reference", got)
	}
}

// Nothing given leaves the provider's own credential in place: a deploy that says
// nothing about the credential must not quietly clear it.
func TestUpstreamAuthOverride_KeepsTheProvidersCredentialWhenNoneIsGiven(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for name, metadata := range map[string]map[string]interface{}{
		"absent": {},
		"empty":  {constants.MetadataKeyUpstreamAuthValue: "   "},
	} {
		t.Run(name, func(t *testing.T) {
			rendered := providerWithUpstreamAuth("api-key", `{{ secret "shared-key" }}`)
			if err := svc.applyUpstreamOverrides(rendered, metadata, "org-1"); err != nil {
				t.Fatalf("applying the override: %v", err)
			}
			if got := upstreamValue(rendered); got != `{{ secret "shared-key" }}` {
				t.Errorf("credential is %q, want the provider's own to be untouched", got)
			}
		})
	}
}

// A credential given literally is refused. Deployment metadata is returned with every
// read of a deployment, so a literal here would be readable by anyone who can list them.
func TestUpstreamAuthOverride_RefusesACredentialGivenLiterally(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", `{{ secret "shared-key" }}`)

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: "sk-live-abc123",
	}, "org-1")

	if err == nil {
		t.Fatal("expected a literal credential to be refused")
	}
	if !strings.Contains(err.Error(), "secret reference") {
		t.Errorf("message %q does not say a secret reference is required", err.Error())
	}
	if got := upstreamValue(rendered); got != `{{ secret "shared-key" }}` {
		t.Errorf("a refused override must leave the definition alone, got %q", got)
	}
}

// A value that merely CONTAINS a reference is refused: a credential with a placeholder
// appended to it would otherwise pass as a reference and be stored in full.
func TestUpstreamAuthOverride_RefusesAValueThatOnlyContainsAReference(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for _, value := range []string{
		`sk-live-abc{{ secret "eu-key" }}`,
		`{{ secret "eu-key" }} sk-live-abc`,
	} {
		rendered := providerWithUpstreamAuth("api-key", "")
		err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
			constants.MetadataKeyUpstreamAuthValue: value,
		}, "org-1")
		if err == nil {
			t.Errorf("expected %q to be refused", value)
		}
	}
}

// An upstream that takes no credential has nothing to override. Setting one would ship
// an auth block the gateway has no use for, and would read as though the deployment
// were authenticating when it is not.
func TestUpstreamAuthOverride_RefusesAnUpstreamThatTakesNoCredential(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for _, authType := range []string{"none", "other", ""} {
		rendered := providerWithUpstreamAuth(authType, "")
		err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
			constants.MetadataKeyUpstreamAuthValue: `{{ secret "eu-key" }}`,
		}, "org-1")
		if err == nil {
			t.Errorf("auth type %q: expected the override to be refused", authType)
		}
	}
}

func TestUpstreamAuthOverride_RefusesAValueThatIsNotAString(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: 42,
	}, "org-1")
	if err == nil {
		t.Fatal("expected a non-string value to be refused")
	}
}

// The reference has to name a secret this organization actually has, so a typo is
// refused at deploy time rather than reaching the gateway as an unresolvable placeholder.
func TestUpstreamAuthOverride_RefusesAReferenceToASecretTheOrganizationDoesNotHave(t *testing.T) {
	svc := &LLMProviderDeploymentService{secretService: NewSecretService(newMockRepo(), nil, nil)}
	rendered := providerWithUpstreamAuth("api-key", "")

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "missing-key" }}`,
	}, "org-1")

	if err == nil {
		t.Fatal("expected a reference to an unknown secret to be refused")
	}
	// The refusal must not carry the handle: it reaches the client verbatim and is
	// logged with the request, and the caller already knows what it sent.
	if strings.Contains(err.Error(), "missing-key") {
		t.Errorf("message %q names the secret handle", err.Error())
	}
	if !apperror.LLMProviderDeploymentValidationFailed.Is(err) {
		t.Errorf("error %v is not a deployment validation failure", err)
	}
}

func TestUpstreamAuthOverride_AcceptsAReferenceToASecretTheOrganizationHas(t *testing.T) {
	repo := newMockRepo()
	if err := repo.Create(&model.Secret{
		Handle: "eu-key",
		Status: model.SecretStatusActive,
	}); err != nil {
		t.Fatalf("seeding the secret: %v", err)
	}
	svc := &LLMProviderDeploymentService{secretService: NewSecretService(repo, nil, nil)}
	rendered := providerWithUpstreamAuth("api-key", "")

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "eu-key" }}`,
	}, "org-1")
	if err != nil {
		t.Fatalf("applying the override: %v", err)
	}
	if got := upstreamValue(rendered); got != `{{ secret "eu-key" }}` {
		t.Errorf("credential is %q, want the deployment's own reference", got)
	}
}

// bearer and basic upstreams carry a credential too, so they can be overridden.
func TestUpstreamAuthOverride_AppliesToEveryUpstreamThatCarriesACredential(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for _, authType := range []string{"api-key", "bearer", "basic"} {
		rendered := providerWithUpstreamAuth(authType, "")
		err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
			constants.MetadataKeyUpstreamAuthValue: `{{ secret "eu-key" }}`,
		}, "org-1")
		if err != nil {
			t.Errorf("auth type %q: %v", authType, err)
			continue
		}
		if got := upstreamValue(rendered); got != `{{ secret "eu-key" }}` {
			t.Errorf("auth type %q: credential is %q", authType, got)
		}
	}
}

// The credential is write-only, exactly as an artifact's own upstream credential is:
// it is written with a deploy and never read back, so no deployment read can hand it
// (or the secret it names) to a caller.
func TestDeploymentMetadata_RedactsTheCredentialOnRead(t *testing.T) {
	metadata := map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "eu-key" }}`,
		constants.MetadataKeyVhostMain:         "api.example.com",
	}

	out := redactWriteOnlyMetadata(metadata)

	if _, present := out[constants.MetadataKeyUpstreamAuthValue]; present {
		t.Error("the credential must not be returned on a read")
	}
	if out[constants.MetadataKeyVhostMain] != "api.example.com" {
		t.Error("the rest of the metadata must survive redaction")
	}
	// The same map is stored on the deployment record, so redacting a response must
	// not empty what was persisted.
	if metadata[constants.MetadataKeyUpstreamAuthValue] != `{{ secret "eu-key" }}` {
		t.Error("redaction must not mutate the caller's metadata")
	}
}

func TestDeploymentMetadata_LeavesMetadataWithoutACredentialAlone(t *testing.T) {
	metadata := map[string]interface{}{constants.MetadataKeyEndpointUrl: "https://api.example.com"}

	out := redactWriteOnlyMetadata(metadata)

	if len(out) != 1 || out[constants.MetadataKeyEndpointUrl] != "https://api.example.com" {
		t.Errorf("metadata carrying no credential should be returned unchanged, got %v", out)
	}
}

// Rotating a gateway's credential releases the secret it moved off, the way updating
// the provider's own credential does — otherwise every re-key would leave a secret
// behind that nothing points at.
func TestRotatedCredential_ReleasesTheSecretTheGatewayMovedOffOf(t *testing.T) {
	repo := newMockRepo()
	released := ""
	repo.findRefsAndSoftDeleteFn = func(_, handle, _ string) ([]model.SecretReference, error) {
		released = handle
		return nil, nil
	}
	svc := &LLMProviderDeploymentService{
		secretService: NewSecretService(repo, nil, nil),
		slogger:       slog.Default(),
	}

	svc.cleanupRotatedCredential("org-1", `{{ secret "old-key" }}`, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "new-key" }}`,
	}, "someone")

	if released != "old-key" {
		t.Errorf("released %q, want the secret the gateway rotated away from", released)
	}
}

// Deploying again on the same credential releases nothing: the gateway is still using
// it, and a redeploy is not a rotation.
func TestRotatedCredential_ReleasesNothingWhenTheCredentialIsUnchanged(t *testing.T) {
	repo := newMockRepo()
	released := ""
	repo.findRefsAndSoftDeleteFn = func(_, handle, _ string) ([]model.SecretReference, error) {
		released = handle
		return nil, nil
	}
	svc := &LLMProviderDeploymentService{
		secretService: NewSecretService(repo, nil, nil),
		slogger:       slog.Default(),
	}

	svc.cleanupRotatedCredential("org-1", `{{ secret "same-key" }}`, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "same-key" }}`,
	}, "someone")

	if released != "" {
		t.Errorf("released %q, want nothing released on a redeploy", released)
	}
}

// A deploy that carries no credential of its own is not a removal. Clients that know
// nothing about per-deployment credentials send metadata on every redeploy (the AI
// Workspace sends the gateway host), and the value is write-only, so treating that as a
// rotation would destroy a secret nobody asked to remove and nobody could resend.
func TestRotatedCredential_ReleasesNothingWhenTheDeploymentCarriesNoCredential(t *testing.T) {
	repo := newMockRepo()
	released := ""
	repo.findRefsAndSoftDeleteFn = func(_, handle, _ string) ([]model.SecretReference, error) {
		released = handle
		return nil, nil
	}
	svc := &LLMProviderDeploymentService{
		secretService: NewSecretService(repo, nil, nil),
		slogger:       slog.Default(),
	}

	for name, metadata := range map[string]map[string]interface{}{
		"other metadata only": {"host": "gw.example.com"},
		"empty metadata":      {},
		"blank credential":    {constants.MetadataKeyUpstreamAuthValue: "  "},
	} {
		released = ""
		svc.cleanupRotatedCredential("org-1", `{{ secret "in-use-key" }}`, metadata, "someone")
		if released != "" {
			t.Errorf("%s: released %q, want the credential left in place", name, released)
		}
	}
}

// A first deployment has nothing to rotate away from.
func TestRotatedCredential_ReleasesNothingOnAFirstDeployment(t *testing.T) {
	repo := newMockRepo()
	released := ""
	repo.findRefsAndSoftDeleteFn = func(_, handle, _ string) ([]model.SecretReference, error) {
		released = handle
		return nil, nil
	}
	svc := &LLMProviderDeploymentService{
		secretService: NewSecretService(repo, nil, nil),
		slogger:       slog.Default(),
	}

	svc.cleanupRotatedCredential("org-1", "", map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue: `{{ secret "new-key" }}`,
	}, "someone")

	if released != "" {
		t.Errorf("released %q, want nothing released", released)
	}
}

// The header the credential is sent in can differ per gateway, because two accounts with
// the same vendor can differ in header convention.
func TestUpstreamAuthHeaderOverride_ReplacesTheHeaderForAnApiKeyUpstream(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthValue:  `{{ secret "eu-key" }}`,
		constants.MetadataKeyUpstreamAuthHeader: "x-api-key",
	}, "org-1")
	if err != nil {
		t.Fatalf("applying the override: %v", err)
	}

	if got := rendered.Spec.Upstream.Auth.Header; got == nil || *got != "x-api-key" {
		t.Errorf("header is %v, want x-api-key", got)
	}
}

// bearer and basic send Authorization by definition, so naming a header for them would
// produce a request the vendor does not recognise.
func TestUpstreamAuthHeaderOverride_RefusesAnUpstreamThatIsNotApiKey(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for _, authType := range []string{"bearer", "basic"} {
		rendered := providerWithUpstreamAuth(authType, "")
		err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
			constants.MetadataKeyUpstreamAuthValue:  `{{ secret "eu-key" }}`,
			constants.MetadataKeyUpstreamAuthHeader: "x-api-key",
		}, "org-1")
		if err == nil {
			t.Errorf("auth type %q: expected the header override to be refused", authType)
		}
	}
}

// A header that is not a valid field name is refused, so an override cannot inject a
// second header or a request line.
func TestUpstreamAuthHeaderOverride_RefusesAHeaderThatIsNotAFieldName(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for _, header := range []string{"x api key", "x-api-key\r\nX-Injected: 1", "x:key"} {
		rendered := providerWithUpstreamAuth("api-key", "")
		err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
			constants.MetadataKeyUpstreamAuthValue:  `{{ secret "eu-key" }}`,
			constants.MetadataKeyUpstreamAuthHeader: header,
		}, "org-1")
		if err == nil {
			t.Errorf("header %q should be refused", header)
		}
	}
}

// A header given without a credential is left alone: it would name where to put a key
// this deployment does not have.
func TestUpstreamAuthHeaderOverride_IgnoredWithoutACredential(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")
	rendered.Spec.Upstream.Auth.Header = nil

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyUpstreamAuthHeader: "x-api-key",
	}, "org-1")
	if err != nil {
		t.Fatalf("applying the override: %v", err)
	}
	if rendered.Spec.Upstream.Auth.Header != nil {
		t.Error("a header without a credential should not be applied")
	}
}

// A deployment can route to its own backend, so one gateway can use a regional or proxied
// endpoint without the provider changing.
func TestUpstreamURLOverride_ReplacesTheProvidersBackend(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")
	rendered.Spec.Upstream.URL = "https://api.openai.com"

	err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyEndpointUrl: "https://eu.api.openai.com/v1",
	}, "org-1")
	if err != nil {
		t.Fatalf("applying the override: %v", err)
	}
	if rendered.Spec.Upstream.URL != "https://eu.api.openai.com/v1" {
		t.Errorf("backend is %q, want the deployment's own", rendered.Spec.Upstream.URL)
	}
}

// The platform requires exactly one of url and ref, so naming a URL drops the ref.
func TestUpstreamURLOverride_ClearsAReferenceItReplaces(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")
	rendered.Spec.Upstream.URL = ""
	rendered.Spec.Upstream.Ref = "shared-openai"

	if err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
		constants.MetadataKeyEndpointUrl: "https://eu.api.openai.com/v1",
	}, "org-1"); err != nil {
		t.Fatalf("applying the override: %v", err)
	}
	if rendered.Spec.Upstream.Ref != "" {
		t.Errorf("ref is %q, want it cleared", rendered.Spec.Upstream.Ref)
	}
}

func TestUpstreamURLOverride_RefusesSomethingThatIsNotAnHTTPURL(t *testing.T) {
	svc := &LLMProviderDeploymentService{}

	for _, value := range []string{"not-a-url", "ftp://example.com", "javascript:alert(1)"} {
		rendered := providerWithUpstreamAuth("api-key", "")
		rendered.Spec.Upstream.URL = "https://api.openai.com"
		err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{
			constants.MetadataKeyEndpointUrl: value,
		}, "org-1")
		if err == nil {
			t.Errorf("value %q should be refused", value)
		}
		if rendered.Spec.Upstream.URL != "https://api.openai.com" {
			t.Errorf("a refused override must leave the backend alone, got %q", rendered.Spec.Upstream.URL)
		}
	}
}

// Nothing given leaves the provider's own backend in place.
func TestUpstreamURLOverride_KeepsTheProvidersBackendWhenNoneIsGiven(t *testing.T) {
	svc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")
	rendered.Spec.Upstream.URL = "https://api.openai.com"

	if err := svc.applyUpstreamOverrides(rendered, map[string]interface{}{}, "org-1"); err != nil {
		t.Fatalf("applying the override: %v", err)
	}
	if rendered.Spec.Upstream.URL != "https://api.openai.com" {
		t.Errorf("backend is %q, want the provider's own", rendered.Spec.Upstream.URL)
	}
}
