package analytics

import (
	"context"
	"encoding/json"
	"strings"
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

// A resources/* method addresses its target by URI at params.uri, while tools/*
// and prompts/* name theirs at params.name. Extracting only params.name left
// mcp.resource.uri permanently empty for every resource read.
func TestOnRequestBody_MCPCapabilityTarget(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantName string
		wantURI  string
	}{
		{
			name:     "tools/call carries a name",
			body:     `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_docs"}}`,
			wantName: "search_docs",
		},
		{
			name:     "prompts/get carries a name",
			body:     `{"jsonrpc":"2.0","id":2,"method":"prompts/get","params":{"name":"summarize"}}`,
			wantName: "summarize",
		},
		{
			name:    "resources/read carries a uri",
			body:    `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"file:///docs/readme.md"}}`,
			wantURI: "file:///docs/readme.md",
		},
		{
			// resources/list takes no target at all; neither field may be invented.
			name: "resources/list carries neither",
			body: `{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := mcpRequestProps(t, tc.body)
			if props.CapabilityName != tc.wantName {
				t.Errorf("capabilityName = %q, want %q", props.CapabilityName, tc.wantName)
			}
			if props.ResourceUri != tc.wantURI {
				t.Errorf("resourceUri = %q, want %q", props.ResourceUri, tc.wantURI)
			}
		})
	}
}

// A resources/read request that also happens to carry params.name must not have
// it mistaken for the resource's identity.
func TestOnRequestBody_MCPResourceIgnoresName(t *testing.T) {
	props := mcpRequestProps(t,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read",`+
			`"params":{"uri":"file:///a.md","name":"not-the-target"}}`)

	if props.ResourceUri != "file:///a.md" {
		t.Errorf("resourceUri = %q, want file:///a.md", props.ResourceUri)
	}
	if props.CapabilityName != "" {
		t.Errorf("capabilityName = %q, want empty for a resource", props.CapabilityName)
	}
}

// mcpRequestProps runs OnRequestBody over an MCP request body and returns the
// properties it emitted onto analytics metadata.
func mcpRequestProps(t *testing.T, body string) McpRequestAnalyticsProperties {
	t.Helper()
	action := (&AnalyticsPolicy{}).OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: &policy.SharedContext{APIKind: policy.APIKindMCP},
		Body:          &policy.Body{Content: []byte(body)},
	}, nil)

	mods, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestModifications, got %T", action)
	}
	raw, ok := mods.AnalyticsMetadata["mcp_request_properties"].(string)
	if !ok {
		t.Fatalf("mcp_request_properties absent or not a string: %#v", mods.AnalyticsMetadata)
	}
	var props McpRequestAnalyticsProperties
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		t.Fatalf("unmarshal mcp_request_properties: %v", err)
	}
	return props
}

// mcpPropsFromHeaderPhase runs the header hook and returns what it published, or "" if it
// published no MCP request properties at all.
func mcpPropsFromHeaderPhase(t *testing.T, ctx *policy.RequestHeaderContext, params map[string]any) string {
	t.Helper()
	action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), ctx, params)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		return ""
	}
	props, _ := mods.AnalyticsMetadata["mcp_request_properties"].(string)
	return props
}

// mcpResponseProps parses a JSON-RPC response literal and returns what analytics records for it.
func mcpResponseProps(t *testing.T, payload string) *McpResponseAnalyticsProperties {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	props := extractMCPResponseAnalyticsProps(parsed)
	if props == nil {
		t.Fatal("expected non-nil props")
	}
	return props
}

// resolvedHeaderCtx is an MCP request on a route carrying the operation resolver: the facts the
// resolver published, plus whatever the client mirrored into headers.
func resolvedHeaderCtx(headers map[string][]string, attrs map[string]string) *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{
			APIKind:              policy.APIKindMCP,
			ResolvedOperation:    "mcp",
			ResolutionAttributes: policy.NewResolutionAttributes(attrs),
			Metadata:             map[string]any{metadataBodyResolved: true},
		},
		Headers: policy.NewHeaders(headers),
	}
}

// input_required is incomplete, not failed: IsError stays false and resultType tells them apart.
func TestExtractMCPResponseAnalyticsProps_InputRequiredIsNotAnError(t *testing.T) {
	props := mcpResponseProps(t,
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"input_required","requestState":"blob"}}`)

	if props.IsError == nil || *props.IsError {
		t.Fatalf("IsError = %v, want false: the call paused, it did not fail", props.IsError)
	}
	if props.ErrorCode != nil {
		t.Fatalf("ErrorCode = %v, want none", *props.ErrorCode)
	}
}

// A legacy initialize result states the negotiated version in result.protocolVersion. The record
// is emitted on that alone, since a server that omits serverInfo is exactly the one worth
// recording — before this, the version was read and then dropped.
func TestExtractMCPResponseAnalyticsProps_ProtocolVersionAloneIsRecorded(t *testing.T) {
	props := mcpResponseProps(t,
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}}}}`)
	if props.ServerInfo == nil {
		t.Fatal("expected ServerInfo carrying the negotiated protocol version")
	}
	if props.ServerInfo.ProtocolVersion != "2025-06-18" {
		t.Fatalf("ProtocolVersion = %q, want 2025-06-18", props.ServerInfo.ProtocolVersion)
	}
	if props.ServerInfo.Name != "" || props.ServerInfo.Version != "" {
		t.Fatalf("no serverInfo object was sent, got %+v", *props.ServerInfo)
	}
}

// A malformed or absent _meta is one missing fact, not a bad response: the rest still reads.
func TestExtractMCPResponseAnalyticsProps_MalformedMetaIsHarmless(t *testing.T) {
	for _, payload := range []string{
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","_meta":{}}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","_meta":42}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","_meta":{"io.modelcontextprotocol/serverInfo":"not-an-object"}}}`,
	} {
		props := mcpResponseProps(t, payload)
		if props.ServerInfo != nil {
			t.Fatalf("payload %s: expected no ServerInfo, got %+v", payload, *props.ServerInfo)
		}
		if props.ResultType != "complete" {
			t.Fatalf("payload %s: the rest of the response must still read", payload)
		}
	}
}

// MCP 2026-07-28 states resultType on every result, and an absent one means complete. An error
// response has no result, so it reports no resultType.
func TestExtractMCPResponseAnalyticsProps_ResultType(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "modern tools/call declares complete",
			payload: `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","content":[{"type":"text","text":"Colombo: 31C"}],"isError":false}}`,
			want:    "complete",
		},
		{
			// The spec's own MRTR example: the server paused to ask for input, so this is not
			// a completed call and the client will retry the whole request with a new id.
			name: "MRTR declares input_required",
			payload: `{"jsonrpc":"2.0","id":1,"result":{"resultType":"input_required",` +
				`"inputRequests":{"github_login":{"method":"elicitation/create","params":{"mode":"form","message":"Please provide your GitHub username"}}},` +
				`"requestState":"AEAD-protected blob"}}`,
			want: "input_required",
		},
		{
			name:    "a legacy result has none, which means complete",
			payload: `{"jsonrpc":"2.0","id":1,"result":{"content":[],"isError":false}}`,
			want:    "complete",
		},
		{
			name:    "an error response gets none at all",
			payload: `{"jsonrpc":"2.0","id":1,"error":{"code":-32022,"message":"Unsupported protocol version"}}`,
			want:    "",
		},
		{
			name:    "a result of the wrong shape gets none",
			payload: `{"jsonrpc":"2.0","id":1,"result":"not-an-object"}`,
			want:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			props := mcpResponseProps(t, c.payload)
			if props.ResultType != c.want {
				t.Fatalf("ResultType = %q, want %q", props.ResultType, c.want)
			}
		})
	}
}

// The identity moved from the initialize result into every result's _meta. Both are read, so one
// function serves both eras.
func TestExtractMCPResponseAnalyticsProps_ServerInfoFromEitherEra(t *testing.T) {
	cases := []struct {
		name                  string
		payload               string
		wantName, wantVersion string
	}{
		{
			// The spec's server/discover result, carrying fields we deliberately ignore.
			name: "modern server/discover states it in result._meta",
			payload: `{"jsonrpc":"2.0","id":"discover-1","result":{"resultType":"complete",` +
				`"supportedVersions":["2026-07-28"],"capabilities":{"tools":{},"resources":{}},` +
				`"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"ExampleServer","version":"1.0.0"}},` +
				`"ttlMs":3600000,"cacheScope":"public"}}`,
			wantName: "ExampleServer", wantVersion: "1.0.0",
		},
		{
			name:     "legacy initialize states it in the result",
			payload:  `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"WeatherServer","version":"0.9"}}}`,
			wantName: "WeatherServer", wantVersion: "0.9",
		},
		{
			name: "_meta wins where a dual-era server sends both",
			payload: `{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"Legacy","version":"0.9"},` +
				`"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"Modern","version":"2.0"}}}}`,
			wantName: "Modern", wantVersion: "2.0",
		},
		{
			// Replaced as a whole: serverInfo is one object, so its fields are not mixed across
			// the two locations.
			name: "a partial _meta replaces the legacy block rather than merging with it",
			payload: `{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"Legacy","version":"0.9"},` +
				`"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"Modern"}}}}`,
			wantName: "Modern", wantVersion: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			props := mcpResponseProps(t, c.payload)
			if props.ServerInfo == nil {
				t.Fatal("expected ServerInfo")
			}
			if props.ServerInfo.Name != c.wantName || props.ServerInfo.Version != c.wantVersion {
				t.Fatalf("ServerInfo = %+v, want %s/%s", *props.ServerInfo, c.wantName, c.wantVersion)
			}
		})
	}
}

// A server/discover result lists the versions the server speaks; protocolVersion carries the
// version a legacy handshake agreed. Neither response populates both.
func TestExtractMCPResponseAnalyticsProps_SupportedVersions(t *testing.T) {
	t.Run("server/discover reports them, and protocolVersion stays absent", func(t *testing.T) {
		props := mcpResponseProps(t, `{"jsonrpc":"2.0","id":"discover-1","result":{"resultType":"complete",`+
			`"supportedVersions":["2026-07-28","2025-11-25"],"capabilities":{"tools":{}},`+
			`"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"ExampleServer","version":"1.0.0"}}}}`)

		if props.ServerInfo == nil {
			t.Fatal("expected ServerInfo")
		}
		want := []string{"2026-07-28", "2025-11-25"}
		if len(props.ServerInfo.SupportedVersions) != len(want) {
			t.Fatalf("SupportedVersions = %v, want %v", props.ServerInfo.SupportedVersions, want)
		}
		for i, v := range want {
			if props.ServerInfo.SupportedVersions[i] != v {
				t.Fatalf("SupportedVersions = %v, want %v", props.ServerInfo.SupportedVersions, want)
			}
		}
		if props.ServerInfo.ProtocolVersion != "" {
			t.Fatalf("ProtocolVersion = %q, want empty: modern negotiates nothing",
				props.ServerInfo.ProtocolVersion)
		}
	})

	// The legacy field keeps its own meaning and its own type, unchanged in every gateway version.
	t.Run("a legacy initialize reports protocolVersion and no list", func(t *testing.T) {
		props := mcpResponseProps(t, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18",`+
			`"serverInfo":{"name":"WeatherServer","version":"0.9"}}}`)

		if props.ServerInfo.ProtocolVersion != "2025-06-18" {
			t.Fatalf("ProtocolVersion = %q, want the negotiated version", props.ServerInfo.ProtocolVersion)
		}
		if props.ServerInfo.SupportedVersions != nil {
			t.Fatalf("SupportedVersions = %v, want none", props.ServerInfo.SupportedVersions)
		}
	})

	// An ordinary call carries neither, and must not gain an empty ServerInfo because of it.
	t.Run("an ordinary tools/call reports no server info at all", func(t *testing.T) {
		props := mcpResponseProps(t, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","isError":false}}`)
		if props.ServerInfo != nil {
			t.Fatalf("ServerInfo = %+v, want none", *props.ServerInfo)
		}
	})

	// One bad member is not a bad list, and a list of the wrong shape is simply not a list.
	t.Run("malformed members are skipped", func(t *testing.T) {
		props := mcpResponseProps(t, `{"jsonrpc":"2.0","id":1,"result":{"supportedVersions":["2026-07-28",42,"",null]}}`)
		if len(props.ServerInfo.SupportedVersions) != 1 || props.ServerInfo.SupportedVersions[0] != "2026-07-28" {
			t.Fatalf("SupportedVersions = %v, want just the one readable member", props.ServerInfo.SupportedVersions)
		}
	})

	t.Run("a non-list supportedVersions yields none", func(t *testing.T) {
		props := mcpResponseProps(t, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","supportedVersions":"2026-07-28"}}`)
		if props.ServerInfo != nil {
			t.Fatalf("ServerInfo = %+v, want none", *props.ServerInfo)
		}
	})
}

// The request side is fingerprinted by the resolver and the response side here, so both must
// produce the same value. Asserted against a hash computed outside this code.
func TestHashRequestState_IsPlainSHA256Hex(t *testing.T) {
	const sha256OfMcp = "10182ab855ff772753c05b2fea333666b5f312835d32936b6b03e08ef2cbd6d3"

	got := hashRequestState("mcp")
	if got != sha256OfMcp {
		t.Fatalf("hashRequestState(\"mcp\") = %q, want %q: the resolver hashes the other half "+
			"with plain SHA-256 hex and the two must match", got, sha256OfMcp)
	}
	if len(got) != 64 {
		t.Fatalf("hash is %d characters, which must stay under the 256-char attribute limit", len(got))
	}
	// Same input, same value: the two halves are hashed on different requests, so a hash
	// carrying any per-call state would never correlate.
	if again := hashRequestState("mcp"); again != got {
		t.Fatalf("hashRequestState is not stable: %q then %q", got, again)
	}
}

// An MRTR exchange is two HTTP transactions with different JSON-RPC ids, so two analytics
// events. The echoed requestState is what ties them together.
func TestMCPAnalytics_MRTRExchangeCorrelates(t *testing.T) {
	const state = "eyJsb2NhdGlvbiI6Ik5ldyBZb3JrIn0-AEAD-protected-blob"

	// Event one: the server answers input_required and mints the state.
	answer := mcpResponseProps(t, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"input_required",`+
		`"inputRequests":{"github_login":{"method":"elicitation/create"}},`+
		`"requestState":"`+state+`"}}`)

	// Event two: the client retries, echoing it. The resolver fingerprinted it for the request.
	retry := mcpRequestPropsFromResolver(
		policy.NewHeaders(map[string][]string{
			"mcp-protocol-version": {"2026-07-28"},
			"mcp-method":           {"tools/call"},
			"mcp-name":             {"get_forecast"},
		}),
		&policy.SharedContext{ResolutionAttributes: policy.NewResolutionAttributes(map[string]string{
			"mcp.body.method":             "tools/call",
			"mcp.body.capability.name":    "get_forecast",
			"mcp.body.request.state.hash": hashRequestState(state),
		})},
	)

	if answer.IssuedRequestStateHash == "" {
		t.Fatal("the input_required answer must fingerprint the state it minted")
	}
	if retry.RequestStateHash != answer.IssuedRequestStateHash {
		t.Fatalf("the two halves must match:\n  issued  %q\n  echoed  %q",
			answer.IssuedRequestStateHash, retry.RequestStateHash)
	}
	if strings.Contains(answer.IssuedRequestStateHash, "AEAD") {
		t.Fatal("the blob itself must not travel into analytics")
	}

	// A different exchange must not correlate with this one.
	other := mcpResponseProps(t,
		`{"jsonrpc":"2.0","id":9,"result":{"resultType":"input_required","requestState":"a-different-blob"}}`)
	if other.IssuedRequestStateHash == answer.IssuedRequestStateHash {
		t.Fatal("two exchanges must not share a fingerprint")
	}
}

// Ordinary traffic carries no state, and must not gain an empty field because of it.
func TestMCPAnalytics_NoRequestStateOnOrdinaryTraffic(t *testing.T) {
	props := mcpResponseProps(t, `{"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","isError":false}}`)
	if props.IssuedRequestStateHash != "" {
		t.Fatalf("IssuedRequestStateHash = %q, want none", props.IssuedRequestStateHash)
	}
}

// The two phases must not both publish: on a resolver route the body hook stands aside.
func TestOnRequestBody_StandsAsideOnAResolverRoute(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_forecast"}}`)

	build := func(resolvedOperation string) *policy.RequestContext {
		return &policy.RequestContext{
			SharedContext: &policy.SharedContext{
				APIKind:           policy.APIKindMCP,
				ResolvedOperation: resolvedOperation,
				Metadata:          map[string]any{metadataBodyResolved: resolvedOperation != ""},
			},
			Headers: policy.NewHeaders(nil),
			Body:    &policy.Body{Content: body, Present: true},
		}
	}
	props := func(action policy.RequestAction) string {
		mods, ok := action.(policy.UpstreamRequestModifications)
		if !ok {
			return ""
		}
		s, _ := mods.AnalyticsMetadata["mcp_request_properties"].(string)
		return s
	}

	withResolver := props((&AnalyticsPolicy{}).OnRequestBody(context.Background(), build("mcp"), nil))
	if withResolver != "" {
		t.Fatalf("expected the body phase to stand aside on a resolver route, got %s", withResolver)
	}

	without := props((&AnalyticsPolicy{}).OnRequestBody(context.Background(), build(""), nil))
	if without == "" {
		t.Fatal("expected the body phase to still parse on a route with no resolver")
	}
}

func TestOnRequestHeaders_MCPFactsFromResolver(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string][]string
		attrs   map[string]string
		want    string
	}{
		{
			name: "modern reads the mirrored headers",
			headers: map[string][]string{
				"mcp-protocol-version": {"2026-07-28"},
				"mcp-method":           {"tools/call"},
				"mcp-name":             {"get_forecast"},
			},
			attrs: map[string]string{"mcp.body.method": "tools/call", "mcp.body.capability.name": "get_forecast"},
			want:  `{"jsonRpcMethod":"tools/call","capability":"TOOL","capabilityName":"get_forecast","clientInfo":{"requestedProtocolVersion":"2026-07-28","name":"","version":""}}`,
		},
		{
			name: "a sentinel-encoded name is decoded",
			headers: map[string][]string{
				"mcp-protocol-version": {"2026-07-28"},
				"mcp-method":           {"tools/call"},
				"mcp-name":             {"=?base64?dG9vbF/DvG1sYXV0?="},
			},
			attrs: map[string]string{"mcp.body.method": "tools/call"},
			want:  `{"jsonRpcMethod":"tools/call","capability":"TOOL","capabilityName":"tool_ümlaut","clientInfo":{"requestedProtocolVersion":"2026-07-28","name":"","version":""}}`,
		},
		{
			// Modern reads the name from Mcp-Name alone, so a withheld one leaves it empty.
			name: "a withheld Mcp-Name is not recovered from the body",
			headers: map[string][]string{
				"mcp-protocol-version": {"2026-07-28"},
				"mcp-method":           {"tools/call"},
			},
			attrs: map[string]string{"mcp.body.method": "tools/call", "mcp.body.capability.name": "from_body"},
			want:  `{"jsonRpcMethod":"tools/call","capability":"TOOL","clientInfo":{"requestedProtocolVersion":"2026-07-28","name":"","version":""}}`,
		},
		{
			name:    "legacy ignores mirrored headers it never defined",
			headers: map[string][]string{"mcp-protocol-version": {"2025-06-18"}, "mcp-method": {"tools/list"}},
			attrs:   map[string]string{"mcp.body.method": "tools/call", "mcp.body.capability.name": "real_tool"},
			want:    `{"jsonRpcMethod":"tools/call","capability":"TOOL","capabilityName":"real_tool"}`,
		},
		{
			// The resolver's capability name is family-keyed and carries the uri for a
			// resource, so it lands in resourceUri — the field the body-parsing path fills,
			// so one operation does not report two shapes depending on the gateway.
			name:    "resources/read reports its uri in resourceUri",
			headers: map[string][]string{"mcp-protocol-version": {"2025-06-18"}},
			attrs:   map[string]string{"mcp.body.method": "resources/read", "mcp.body.capability.name": "file:///a.txt"},
			want:    `{"jsonRpcMethod":"resources/read","capability":"RESOURCE","resourceUri":"file:///a.txt"}`,
		},
		{
			// The mirrored header carries the uri on a modern request, and must land in the
			// same field as the legacy case above.
			name:    "a modern resources/read reports its uri in resourceUri too",
			headers: map[string][]string{"mcp-protocol-version": {"2026-07-28"}, "mcp-method": {"resources/read"}, "mcp-name": {"file:///a.txt"}},
			want:    `{"jsonRpcMethod":"resources/read","capability":"RESOURCE","resourceUri":"file:///a.txt","clientInfo":{"requestedProtocolVersion":"2026-07-28","name":"","version":""}}`,
		},
		{
			name:    "clientInfo comes from the resolver in both eras",
			headers: map[string][]string{"mcp-protocol-version": {"2026-07-28"}, "mcp-method": {"tools/list"}},
			attrs: map[string]string{
				"mcp.body.method":           "tools/list",
				"mcp.body.client.name":      "ExampleClient",
				"mcp.body.client.version":   "1.0.0",
				"mcp.body.protocol.version": "2026-07-28",
			},
			want: `{"jsonRpcMethod":"tools/list","capability":"TOOL","clientInfo":{"requestedProtocolVersion":"2026-07-28","name":"ExampleClient","version":"1.0.0"}}`,
		},
		{
			name:    "a legacy initialize reports the version it proposed",
			headers: map[string][]string{},
			attrs: map[string]string{
				"mcp.body.method":           "initialize",
				"mcp.body.protocol.version": "2025-06-18",
				"mcp.body.client.name":      "LegacyClient",
				"mcp.body.client.version":   "0.9",
			},
			want: `{"jsonRpcMethod":"initialize","clientInfo":{"requestedProtocolVersion":"2025-06-18","name":"LegacyClient","version":"0.9"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mcpPropsFromHeaderPhase(t, resolvedHeaderCtx(tc.headers, tc.attrs), nil)
			if got != tc.want {
				t.Fatalf("mcp_request_properties\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// A route with no resolver carries no marker, so the header phase publishes nothing and the body
// phase does the parsing instead.
func TestOnRequestHeaders_NoResolverPublishesNoMCPProperties(t *testing.T) {
	ctx := resolvedHeaderCtx(
		map[string][]string{"mcp-protocol-version": {"2026-07-28"}, "mcp-method": {"tools/call"}},
		map[string]string{"mcp.body.method": "tools/call"},
	)
	ctx.ResolvedOperation = ""
	delete(ctx.Metadata, metadataBodyResolved)
	if got := mcpPropsFromHeaderPhase(t, ctx, nil); got != "" {
		t.Fatalf("expected no mcp_request_properties on a route with no resolver, got %s", got)
	}
}
