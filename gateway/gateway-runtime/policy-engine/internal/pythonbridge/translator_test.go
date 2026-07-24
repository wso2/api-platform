package pythonbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestTranslatorToProtoHeadersPreservesRepeatedValues(t *testing.T) {
	headers := policy.NewHeaders(map[string][]string{
		"X-Trace": {"one", "two"},
	})

	result := NewTranslator().ToProtoHeaders(headers)

	require.NotNil(t, result)
	assert.Equal(t, []string{"one", "two"}, result.GetValues()["x-trace"].GetValues())
}

func TestTranslatorToProtoSharedContextPreservesStructuredAuth(t *testing.T) {
	translator := NewTranslator()
	shared := &policy.SharedContext{
		ProjectID:     "project-1",
		RequestID:     "request-1",
		Metadata:      map[string]interface{}{"flag": true},
		APIId:         "api-1",
		APIName:       "PetStore",
		APIVersion:    "v1",
		APIKind:       policy.APIKindRestApi,
		APIContext:    "/petstore",
		OperationPath: "/pets/{id}",
		AuthContext: &policy.AuthContext{
			Authenticated: true,
			Authorized:    true,
			AuthType:      "jwt",
			Subject:       "alice",
			TokenId:       "tok-abc",
			Scopes:        map[string]bool{"read:pets": true},
			Properties:    map[string]string{"tenant": "demo"},
			TypedProperties: map[string]interface{}{
				"roles": []interface{}{"admin", "dev"},
				"dept":  "platform",
			},
			Previous: &policy.AuthContext{
				Authenticated: true,
				AuthType:      "apikey",
				Subject:       "legacy-client",
				TypedProperties: map[string]interface{}{
					"legacy": []interface{}{"x"},
				},
			},
		},
	}

	result, err := translator.ToProtoSharedContext(shared)
	require.NoError(t, err)

	assert.Equal(t, "project-1", result.GetProjectId())
	assert.Equal(t, true, result.GetMetadata().GetFields()["flag"].GetBoolValue())
	require.NotNil(t, result.GetAuthContext())
	assert.Equal(t, "jwt", result.GetAuthContext().GetAuthType())
	assert.Equal(t, "tok-abc", result.GetAuthContext().GetTokenId())
	assert.Equal(t, true, result.GetAuthContext().GetScopes()["read:pets"])
	require.NotNil(t, result.GetAuthContext().GetPrevious())
	assert.Equal(t, "apikey", result.GetAuthContext().GetPrevious().GetAuthType())

	// TypedProperties must cross the boundary with structure preserved: the array-valued
	// "roles" claim stays a list, and the scalar "dept" stays a string.
	tp := result.GetAuthContext().GetTypedProperties()
	require.NotNil(t, tp)
	assert.Equal(t, "platform", tp.GetFields()["dept"].GetStringValue())
	roles := tp.GetFields()["roles"].GetListValue().GetValues()
	require.Len(t, roles, 2)
	assert.Equal(t, "admin", roles[0].GetStringValue())
	assert.Equal(t, "dev", roles[1].GetStringValue())

	// Nested Previous contexts must also carry TypedProperties.
	prevTP := result.GetAuthContext().GetPrevious().GetTypedProperties()
	require.NotNil(t, prevTP)
	require.Len(t, prevTP.GetFields()["legacy"].GetListValue().GetValues(), 1)
}

func TestTranslatorToGoRequestHeaderActionTranslatesCurrentFields(t *testing.T) {
	analytics, err := structpb.NewStruct(map[string]any{"tokens": float64(12)})
	if err != nil {
		t.Fatalf("build analytics struct: %v", err)
	}
	dynamic, err := structpb.NewStruct(map[string]any{"value": "ok"})
	if err != nil {
		t.Fatalf("build dynamic struct: %v", err)
	}

	resp := &proto.StreamResponse{
		RequestId: "req-1",
		Payload: &proto.StreamResponse_RequestHeaderAction{
			RequestHeaderAction: &proto.RequestHeaderActionPayload{
				Action: &proto.RequestHeaderActionPayload_UpstreamRequestHeaderModifications{
					UpstreamRequestHeaderModifications: &proto.UpstreamRequestHeaderModifications{
						HeadersToSet:    map[string]string{"x-added": "value"},
						HeadersToRemove: []string{"x-removed"},
						UpstreamName:    &wrapperspb.StringValue{Value: "blue"},
						Path:            &wrapperspb.StringValue{Value: "/rewritten"},
						Host:            &wrapperspb.StringValue{Value: "backend.internal"},
						Method:          &wrapperspb.StringValue{Value: "POST"},
						QueryParametersToAdd: map[string]*proto.StringList{
							"foo": {Values: []string{"bar", "baz"}},
						},
						QueryParametersToRemove: []string{"drop"},
						AnalyticsMetadata:       analytics,
						DynamicMetadata: map[string]*structpb.Struct{
							"ns": dynamic,
						},
						AnalyticsHeaderFilter: &proto.DropHeaderAction{
							Action:  proto.DropHeaderActionType_DROP_HEADER_ACTION_TYPE_DENY,
							Headers: []string{"authorization"},
						},
					},
				},
			},
		},
	}

	action, err := NewTranslator().ToGoRequestHeaderAction(resp)
	if err != nil {
		t.Fatalf("translate request-header action: %v", err)
	}

	mod, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}
	if got := mod.HeadersToSet["x-added"]; got != "value" {
		t.Fatalf("expected header mutation, got %q", got)
	}
	if mod.UpstreamName == nil || *mod.UpstreamName != "blue" {
		t.Fatalf("expected upstream name mutation, got %#v", mod.UpstreamName)
	}
	if mod.Host == nil || *mod.Host != "backend.internal" {
		t.Fatalf("expected host mutation, got %#v", mod.Host)
	}
	if len(mod.QueryParametersToAdd["foo"]) != 2 {
		t.Fatalf("expected query parameters to add, got %#v", mod.QueryParametersToAdd)
	}
	if mod.AnalyticsMetadata["tokens"] != float64(12) {
		t.Fatalf("expected analytics metadata, got %#v", mod.AnalyticsMetadata)
	}
	if mod.DynamicMetadata["ns"]["value"] != "ok" {
		t.Fatalf("expected dynamic metadata, got %#v", mod.DynamicMetadata)
	}
	if mod.AnalyticsHeaderFilter.Action != "deny" {
		t.Fatalf("expected analytics header filter action, got %#v", mod.AnalyticsHeaderFilter)
	}
}

func TestTranslatorToGoNeedsMoreDecision(t *testing.T) {
	decision, err := NewTranslator().ToGoNeedsMoreDecision(&proto.StreamResponse{
		RequestId: "req-3",
		Payload: &proto.StreamResponse_NeedsMoreDecision{
			NeedsMoreDecision: &proto.NeedsMoreDecisionPayload{NeedsMore: true},
		},
	})
	require.NoError(t, err)
	assert.True(t, decision)
}

func TestTranslatorToGoStreamingResponseAction(t *testing.T) {
	analytics, err := structpb.NewStruct(map[string]any{"done": true})
	if err != nil {
		t.Fatalf("build analytics struct: %v", err)
	}

	resp := &proto.StreamResponse{
		RequestId: "req-2",
		Payload: &proto.StreamResponse_StreamingResponseAction{
			StreamingResponseAction: &proto.StreamingResponseActionPayload{
				Action: &proto.StreamingResponseActionPayload_TerminateResponseChunk{
					TerminateResponseChunk: &proto.TerminateResponseChunk{
						Body:              &wrapperspb.BytesValue{Value: []byte("final")},
						AnalyticsMetadata: analytics,
					},
				},
			},
		},
	}

	action, err := NewTranslator().ToGoStreamingResponseAction(resp)
	if err != nil {
		t.Fatalf("translate streaming response action: %v", err)
	}

	term, ok := action.(policy.TerminateResponseChunk)
	if !ok {
		t.Fatalf("expected TerminateResponseChunk, got %T", action)
	}
	if string(term.Body) != "final" {
		t.Fatalf("expected final chunk body, got %q", string(term.Body))
	}
	if term.AnalyticsMetadata["done"] != true {
		t.Fatalf("expected analytics metadata, got %#v", term.AnalyticsMetadata)
	}
}

// Fail-closed: if TypedProperties holds a value that google.protobuf.Struct cannot represent,
// translation must return an error (so the request is rejected) rather than silently dropping the
// claims — dropping them would make downstream policies see the claim as absent and could change an
// authorization decision. The unsupported value is nested (and also tested inside Previous) to
// exercise the recursive path.
func TestTranslatorToProtoSharedContextFailsClosedOnUnserializableTypedProperties(t *testing.T) {
	unserializable := map[string]interface{}{
		"nested": map[string]interface{}{"bad": make(chan int)}, // channels aren't representable in Struct
	}
	cases := map[string]*policy.AuthContext{
		"top-level": {Authenticated: true, AuthType: "jwt", TypedProperties: unserializable},
		"previous": {
			Authenticated: true, AuthType: "jwt",
			Previous: &policy.AuthContext{Authenticated: true, AuthType: "apikey", TypedProperties: unserializable},
		},
	}
	for name, authCtx := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := NewTranslator().ToProtoSharedContext(&policy.SharedContext{AuthContext: authCtx})
			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}

// Documented numeric regression: google.protobuf.Struct has a single numeric kind (number_value /
// double). Numeric claims — which arrive as float64 from JSON/JWT decoding — are carried as a
// NumberValue; integer values beyond 2^53 are subject to double rounding. This pins the double-typed
// round-trip so a future change to the numeric representation is caught.
func TestTranslatorToProtoAuthContextCarriesNumbersAsDouble(t *testing.T) {
	shared := &policy.SharedContext{
		AuthContext: &policy.AuthContext{
			Authenticated:   true,
			AuthType:        "jwt",
			TypedProperties: map[string]interface{}{"level": float64(42), "big": float64(1 << 53)},
		},
	}
	result, err := NewTranslator().ToProtoSharedContext(shared)
	require.NoError(t, err)

	fields := result.GetAuthContext().GetTypedProperties().GetFields()
	require.NotNil(t, fields["level"])
	assert.Equal(t, float64(42), fields["level"].GetNumberValue())
	assert.Equal(t, float64(1<<53), fields["big"].GetNumberValue())
	// Stored under the number kind — never string or other.
	_, isNumber := fields["level"].GetKind().(*structpb.Value_NumberValue)
	assert.True(t, isNumber)
}
