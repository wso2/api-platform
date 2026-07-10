package analytics

import (
	"context"
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestDeriveMCPCapability(t *testing.T) {
	cases := []struct {
		method string
		want   string
	}{
		{"tools/call", "TOOL"},
		{"tools/list", "TOOL"},
		{"resources/read", "RESOURCE"},
		{"resources/list", "RESOURCE"},
		{"prompts/get", "PROMPT"},
		{"prompts/list", "PROMPT"},
		{"initialize", ""},
		{"ping", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := deriveMCPCapability(c.method); got != c.want {
			t.Errorf("deriveMCPCapability(%q) = %q, want %q", c.method, got, c.want)
		}
	}
}

// OnResponseHeaders must capture the response content type for every API kind (not just
// MCP), since the Envoy access log carries no response headers. It reads it from the live
// response headers and emits it as response_content_type analytics metadata.
func TestOnResponseHeaders_CapturesContentTypeForAllKinds(t *testing.T) {
	cases := []struct {
		name    string
		apiKind policy.APIKind
	}{
		{"rest", policy.APIKindRestApi},
		{"llm provider", policy.APIKindLlmProvider},
		{"mcp", policy.APIKindMCP},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			respCtx := &policy.ResponseHeaderContext{
				SharedContext: &policy.SharedContext{APIKind: c.apiKind},
				ResponseHeaders: policy.NewHeaders(map[string][]string{
					"content-type": {"application/json"},
				}),
				ResponseStatus: 200,
			}

			action := (&AnalyticsPolicy{}).OnResponseHeaders(context.Background(), respCtx, nil)

			mods, ok := action.(policy.DownstreamResponseHeaderModifications)
			if !ok {
				t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", action)
			}
			if got := mods.AnalyticsMetadata["response_content_type"]; got != "application/json" {
				t.Errorf("response_content_type = %v, want application/json", got)
			}
		})
	}
}

func TestPopulateAuthAnalyticsMetadata_AllFieldsPopulated(t *testing.T) {
	authCtx := &policy.AuthContext{
		Authenticated: true,
		Authorized:    true,
		Subject:       "alice",
		AuthType:      "jwt",
		Issuer:        "https://issuer.example.com",
		CredentialID:  "client-123",
		TokenId:       "jti-abc",
		Audience:      []string{"aud1", "aud2"},
		Scopes:        map[string]bool{"read": true, "write": true, "admin": true},
		Properties:    map[string]string{"tenant": "acme"},
	}

	metadata := make(map[string]any)
	populateAuthAnalyticsMetadata(metadata, authCtx)

	if metadata[AuthUserIDMetadataKey] != "alice" {
		t.Errorf("%s = %v, want alice", AuthUserIDMetadataKey, metadata[AuthUserIDMetadataKey])
	}
	if metadata[AuthAuthorizedMetadataKey] != "true" {
		t.Errorf("%s = %v, want true", AuthAuthorizedMetadataKey, metadata[AuthAuthorizedMetadataKey])
	}
	if metadata[AuthTypeMetadataKey] != "jwt" {
		t.Errorf("%s = %v, want jwt", AuthTypeMetadataKey, metadata[AuthTypeMetadataKey])
	}
	if metadata[AuthIssuerMetadataKey] != "https://issuer.example.com" {
		t.Errorf("%s = %v, want issuer URL", AuthIssuerMetadataKey, metadata[AuthIssuerMetadataKey])
	}
	if metadata[AuthCredentialIDMetadataKey] != "client-123" {
		t.Errorf("%s = %v, want client-123", AuthCredentialIDMetadataKey, metadata[AuthCredentialIDMetadataKey])
	}
	if metadata[AuthTokenIDMetadataKey] != "jti-abc" {
		t.Errorf("%s = %v, want jti-abc", AuthTokenIDMetadataKey, metadata[AuthTokenIDMetadataKey])
	}
	if metadata[AuthAudienceMetadataKey] != "aud1,aud2" {
		t.Errorf("%s = %v, want aud1,aud2", AuthAudienceMetadataKey, metadata[AuthAudienceMetadataKey])
	}
	if metadata[AuthScopesMetadataKey] != "admin read write" {
		t.Errorf("%s = %v, want sorted+space-joined 'admin read write'", AuthScopesMetadataKey, metadata[AuthScopesMetadataKey])
	}
	var props map[string]string
	raw, ok := metadata[AuthPropertiesMetadataKey].(string)
	if !ok {
		t.Fatalf("%s is not a string: %v", AuthPropertiesMetadataKey, metadata[AuthPropertiesMetadataKey])
	}
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", AuthPropertiesMetadataKey, err)
	}
	if props["tenant"] != "acme" {
		t.Errorf("auth properties[tenant] = %v, want acme", props["tenant"])
	}
}

func TestPopulateAuthAnalyticsMetadata_OptionalFieldsOmittedWhenEmpty(t *testing.T) {
	authCtx := &policy.AuthContext{
		Authenticated: true,
		Subject:       "bob",
		// AuthType, Issuer, CredentialID, TokenId, Audience, Scopes, Properties all zero.
	}

	metadata := make(map[string]any)
	populateAuthAnalyticsMetadata(metadata, authCtx)

	if metadata[AuthUserIDMetadataKey] != "bob" {
		t.Errorf("%s = %v, want bob", AuthUserIDMetadataKey, metadata[AuthUserIDMetadataKey])
	}
	for _, key := range []string{
		AuthTypeMetadataKey, AuthIssuerMetadataKey, AuthCredentialIDMetadataKey,
		AuthTokenIDMetadataKey, AuthAudienceMetadataKey, AuthScopesMetadataKey, AuthPropertiesMetadataKey,
	} {
		if _, present := metadata[key]; present {
			t.Errorf("expected %s to be omitted when empty, got %v", key, metadata[key])
		}
	}
}

// Unlike the optional fields above, Authorized is always stamped — even false — since it
// is a distinct concept from Authenticated (which gates the whole block) and a false value
// is meaningful information (e.g. authenticated but not authorized by mcp-authz), not an
// absence to be omitted.
func TestPopulateAuthAnalyticsMetadata_AuthorizedAlwaysStampedEvenFalse(t *testing.T) {
	authCtx := &policy.AuthContext{Authenticated: true, Subject: "bob", Authorized: false}

	metadata := make(map[string]any)
	populateAuthAnalyticsMetadata(metadata, authCtx)

	got, present := metadata[AuthAuthorizedMetadataKey]
	if !present {
		t.Fatal("expected AuthAuthorizedMetadataKey to be present even when false")
	}
	if got != "false" {
		t.Errorf("%s = %v, want false", AuthAuthorizedMetadataKey, got)
	}
}

func TestPopulateAuthAnalyticsMetadata_UnauthenticatedSkipped(t *testing.T) {
	authCtx := &policy.AuthContext{Authenticated: false, Subject: "carol"}

	metadata := make(map[string]any)
	populateAuthAnalyticsMetadata(metadata, authCtx)

	if len(metadata) != 0 {
		t.Errorf("expected no metadata for unauthenticated context, got %v", metadata)
	}
}

func TestPopulateAuthAnalyticsMetadata_EmptySubjectSkipped(t *testing.T) {
	authCtx := &policy.AuthContext{Authenticated: true, Subject: ""}

	metadata := make(map[string]any)
	populateAuthAnalyticsMetadata(metadata, authCtx)

	if len(metadata) != 0 {
		t.Errorf("expected no metadata when subject is empty, got %v", metadata)
	}
}

func TestPopulateAuthAnalyticsMetadata_NilAuthContextNoop(t *testing.T) {
	metadata := make(map[string]any)

	if got := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		populateAuthAnalyticsMetadata(metadata, nil)
		return false
	}(); got {
		t.Fatal("populateAuthAnalyticsMetadata must not panic on a nil AuthContext")
	}
	if len(metadata) != 0 {
		t.Errorf("expected no metadata for nil AuthContext, got %v", metadata)
	}
}

// Layered auth (Previous chain): the first layer that is both authenticated and has a
// non-empty subject wins, matching the pre-existing single-Subject behavior this helper
// replaced.
func TestPopulateAuthAnalyticsMetadata_WalksPreviousChainToFirstAuthenticated(t *testing.T) {
	inner := &policy.AuthContext{Authenticated: false, Subject: ""}
	outer := &policy.AuthContext{Authenticated: true, Subject: "dave", AuthType: "mcp/oauth", Previous: inner}

	metadata := make(map[string]any)
	populateAuthAnalyticsMetadata(metadata, outer)

	if metadata[AuthUserIDMetadataKey] != "dave" {
		t.Errorf("%s = %v, want dave", AuthUserIDMetadataKey, metadata[AuthUserIDMetadataKey])
	}
	if metadata[AuthTypeMetadataKey] != "mcp/oauth" {
		t.Errorf("%s = %v, want mcp/oauth", AuthTypeMetadataKey, metadata[AuthTypeMetadataKey])
	}
}

// OnResponseHeaders wires populateAuthAnalyticsMetadata through end to end.
func TestOnResponseHeaders_PopulatesAuthMetadata(t *testing.T) {
	respCtx := &policy.ResponseHeaderContext{
		SharedContext: &policy.SharedContext{
			AuthContext: &policy.AuthContext{
				Authenticated: true,
				Subject:       "erin",
				AuthType:      "jwt",
				Scopes:        map[string]bool{"read": true},
			},
		},
		ResponseStatus: 200,
	}

	action := (&AnalyticsPolicy{}).OnResponseHeaders(context.Background(), respCtx, nil)

	mods, ok := action.(policy.DownstreamResponseHeaderModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", action)
	}
	if mods.AnalyticsMetadata[AuthUserIDMetadataKey] != "erin" {
		t.Errorf("%s = %v, want erin", AuthUserIDMetadataKey, mods.AnalyticsMetadata[AuthUserIDMetadataKey])
	}
	if mods.AnalyticsMetadata[AuthScopesMetadataKey] != "read" {
		t.Errorf("%s = %v, want read", AuthScopesMetadataKey, mods.AnalyticsMetadata[AuthScopesMetadataKey])
	}
}

func TestPopulateGenericMetadata_PassesThroughArbitraryKeys(t *testing.T) {
	metadata := make(map[string]any)
	shared := map[string]interface{}{
		"applicationId": "app-42",
		"isTrial":       true,
	}

	populateGenericMetadata(metadata, shared)

	raw, ok := metadata[GenericMetadataKey].(string)
	if !ok {
		t.Fatalf("expected %s to be a JSON string, got %T", GenericMetadataKey, metadata[GenericMetadataKey])
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode %s: %v", GenericMetadataKey, err)
	}
	if decoded["applicationId"] != "app-42" {
		t.Errorf("applicationId = %v, want app-42", decoded["applicationId"])
	}
	if decoded["isTrial"] != true {
		t.Errorf("isTrial = %v, want true", decoded["isTrial"])
	}
}

// The streaming-body accumulator is internal scratch space (see analyticsStreamAccKey's
// doc comment) and must never be exported, regardless of what a policy writes elsewhere
// in SharedContext.Metadata.
func TestPopulateGenericMetadata_ExcludesStreamAccumulator(t *testing.T) {
	metadata := make(map[string]any)
	shared := map[string]interface{}{
		"applicationId":       "app-42",
		analyticsStreamAccKey: []byte("partial response body chunk data"),
	}

	populateGenericMetadata(metadata, shared)

	raw := metadata[GenericMetadataKey].(string)
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode %s: %v", GenericMetadataKey, err)
	}
	if _, present := decoded[analyticsStreamAccKey]; present {
		t.Errorf("internal stream accumulator key must never be exported, got: %v", decoded)
	}
	if decoded["applicationId"] != "app-42" {
		t.Errorf("applicationId = %v, want app-42", decoded["applicationId"])
	}
}

func TestPopulateGenericMetadata_EmptyOrNilMetadataNoop(t *testing.T) {
	metadata := make(map[string]any)
	populateGenericMetadata(metadata, nil)
	if _, present := metadata[GenericMetadataKey]; present {
		t.Errorf("nil SharedContext.Metadata must not set %s", GenericMetadataKey)
	}

	populateGenericMetadata(metadata, map[string]interface{}{})
	if _, present := metadata[GenericMetadataKey]; present {
		t.Errorf("empty SharedContext.Metadata must not set %s", GenericMetadataKey)
	}

	// Only the excluded stream-accumulator key present -- still nothing to export.
	populateGenericMetadata(metadata, map[string]interface{}{analyticsStreamAccKey: []byte("x")})
	if _, present := metadata[GenericMetadataKey]; present {
		t.Errorf("SharedContext.Metadata containing only the excluded key must not set %s", GenericMetadataKey)
	}
}

// OnResponseHeaders wires populateGenericMetadata through end to end, alongside the
// existing auth/subscription metadata copies.
func TestOnResponseHeaders_PopulatesGenericMetadata(t *testing.T) {
	respCtx := &policy.ResponseHeaderContext{
		SharedContext: &policy.SharedContext{
			Metadata: map[string]interface{}{
				"applicationId":       "app-42",
				analyticsStreamAccKey: []byte("should never be exported"),
			},
		},
		ResponseStatus: 200,
	}

	action := (&AnalyticsPolicy{}).OnResponseHeaders(context.Background(), respCtx, nil)

	mods, ok := action.(policy.DownstreamResponseHeaderModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", action)
	}
	raw, ok := mods.AnalyticsMetadata[GenericMetadataKey].(string)
	if !ok {
		t.Fatalf("expected %s to be set as a JSON string", GenericMetadataKey)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode %s: %v", GenericMetadataKey, err)
	}
	if decoded["applicationId"] != "app-42" {
		t.Errorf("applicationId = %v, want app-42", decoded["applicationId"])
	}
	if _, present := decoded[analyticsStreamAccKey]; present {
		t.Errorf("internal stream accumulator key must never be exported, got: %v", decoded)
	}
}

func TestExtractMCPResponseAnalyticsProps_IsError(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    bool
	}{
		{"protocol error", `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`, true},
		{"tool result error", `{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[]}}`, true},
		{"tool result success", `{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[]}}`, false},
		{"result without isError", `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`, false},
		{"null error is not an error", `{"jsonrpc":"2.0","id":1,"error":null,"result":{}}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(c.payload), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			props := extractMCPResponseAnalyticsProps(payload)
			if props == nil {
				t.Fatal("expected non-nil props")
			}
			if props.IsError == nil {
				t.Fatal("expected IsError to always be set")
			}
			if *props.IsError != c.want {
				t.Errorf("IsError = %v, want %v", *props.IsError, c.want)
			}
		})
	}
}
