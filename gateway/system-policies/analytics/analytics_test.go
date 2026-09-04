package analytics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

// OnRequestHeaders stamps the internal loopback marker into analytics metadata when the
// proxy's loopback forward carries the x-wso2-internal-loopback header, so the policy-engine
// can drop the duplicate provider event from Moesif. A request without the header does not.
func TestOnRequestHeaders_StampsInternalLoopbackMarker(t *testing.T) {
	t.Run("marker present", func(t *testing.T) {
		reqCtx := &policy.RequestHeaderContext{
			SharedContext: &policy.SharedContext{APIKind: policy.APIKindLlmProvider},
			Headers:       policy.NewHeaders(map[string][]string{InternalLoopbackMetadataKey: {"1"}}),
		}
		action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
		mods, ok := action.(policy.UpstreamRequestHeaderModifications)
		if !ok {
			t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
		}
		if got := mods.AnalyticsMetadata[InternalLoopbackMetadataKey]; got != "true" {
			t.Errorf("%s = %v, want true", InternalLoopbackMetadataKey, got)
		}
	})

	t.Run("marker absent", func(t *testing.T) {
		reqCtx := &policy.RequestHeaderContext{
			SharedContext: &policy.SharedContext{APIKind: policy.APIKindLlmProvider},
			Headers:       policy.NewHeaders(map[string][]string{}),
		}
		action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
		if mods, ok := action.(policy.UpstreamRequestHeaderModifications); ok {
			if _, exists := mods.AnalyticsMetadata[InternalLoopbackMetadataKey]; exists {
				t.Errorf("marker must not be stamped when the header is absent")
			}
		}
	})
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

// The SSE separator space is optional, so the timing scanner (sseBlockHasData) and
// the observation scanner (observeA2AResponse) must accept the same lines. When they
// disagreed, an agent sending "data:{...}" was timed as having delivered a first
// event while yielding no observation at all — isError, payloadType, taskState and
// both identifiers absent, and the outcome reported as UNKNOWN.
func TestSSEDataScannersAgreeOnTheOptionalSeparatorSpace(t *testing.T) {
	const event = `{"jsonrpc":"2.0","id":1,"result":{"id":"task-1","contextId":"ctx-1",` +
		`"status":{"state":"TASK_STATE_COMPLETED"}}}`
	headers := policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}})

	for name, body := range map[string]string{
		"with the separator space":    "data: " + event + "\n\n",
		"without the separator space": "data:" + event + "\n\n",
		"with a CRLF line ending":     "data:" + event + "\r\n\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			// The timing scanner must see an event...
			if _, found := firstSSEDataEventEnd([]byte(body), 0); !found {
				t.Fatalf("timing scanner found no event in %q", body)
			}
			// ...and the observation scanner must read the same one.
			observation := observeA2AResponse([]byte(body), headers)
			if observation.outcomeEnvelope == nil {
				t.Fatalf("observation scanner read nothing from %q — the two scanners disagree", body)
			}
			if observation.taskID != "task-1" {
				t.Errorf("taskID = %q, want task-1", observation.taskID)
			}
			if observation.contextID != "ctx-1" {
				t.Errorf("contextID = %q, want ctx-1", observation.contextID)
			}
			if observation.payloadType != a2aPayloadTask {
				t.Errorf("payloadType = %q, want %q", observation.payloadType, a2aPayloadTask)
			}
			if observation.taskState != "TASK_STATE_COMPLETED" {
				t.Errorf("taskState = %q, want TASK_STATE_COMPLETED", observation.taskState)
			}
		})
	}
}

// Only one space is the separator; a second belongs to the value. Stripping all
// leading whitespace would silently rewrite a payload the agent chose to send.
func TestSSEDataValueStripsExactlyOneSeparatorSpace(t *testing.T) {
	cases := map[string]struct {
		line       string
		wantValue  string
		wantIsData bool
	}{
		"no space":       {"data:{}", "{}", true},
		"one space":      {"data: {}", "{}", true},
		"two spaces":     {"data:  {}", " {}", true},
		"empty value":    {"data:", "", true},
		"a comment line": {": keep-alive", "", false},
		"another field":  {"event: message", "", false},
		"not a field":    {"database: x", "", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			value, isData := sseDataValue(c.line)
			if isData != c.wantIsData {
				t.Fatalf("isData = %v, want %v", isData, c.wantIsData)
			}
			if value != c.wantValue {
				t.Errorf("value = %q, want %q", value, c.wantValue)
			}
		})
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

// The A2A stream marks are the same kind of internal scratch space as the
// accumulator: a request timestamp, a first-event timestamp and a scan offset the
// policy turns into TTFB and stream duration, which it exports under
// A2AResponsePropertiesKey instead. They are also written and cleared in different
// phases from this export, so a2aRequestStartKey is live for every Agent request by
// the time it runs — without the filter, every one of them carries an internal
// timestamp into its traffic-log line.
func TestPopulateGenericMetadata_ExcludesA2AStreamMarks(t *testing.T) {
	metadata := make(map[string]any)
	shared := map[string]interface{}{
		"applicationId":    "app-42",
		a2aRequestStartKey: time.Now(),
		a2aFirstEventKey:   time.Now(),
		a2aStreamScanKey:   512,
	}

	populateGenericMetadata(metadata, shared)

	raw := metadata[GenericMetadataKey].(string)
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode %s: %v", GenericMetadataKey, err)
	}
	for _, key := range []string{a2aRequestStartKey, a2aFirstEventKey, a2aStreamScanKey} {
		if _, present := decoded[key]; present {
			t.Errorf("internal A2A mark %s must never be exported, got: %v", key, decoded)
		}
	}
	if decoded["applicationId"] != "app-42" {
		t.Errorf("applicationId = %v, want app-42", decoded["applicationId"])
	}
}

// A metadata bag holding nothing but internal keys exports nothing at all, rather
// than an empty JSON object a consumer would have to filter out itself.
func TestPopulateGenericMetadata_OnlyInternalKeysExportsNothing(t *testing.T) {
	metadata := make(map[string]any)
	populateGenericMetadata(metadata, map[string]interface{}{
		analyticsStreamAccKey: []byte("chunk"),
		a2aRequestStartKey:    time.Now(),
	})
	if _, present := metadata[GenericMetadataKey]; present {
		t.Errorf("metadata holding only internal keys must not set %s, got %v",
			GenericMetadataKey, metadata[GenericMetadataKey])
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
