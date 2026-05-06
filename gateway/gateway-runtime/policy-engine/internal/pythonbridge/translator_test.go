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
			Scopes:        map[string]bool{"read:pets": true},
			Properties:    map[string]string{"tenant": "demo"},
			Previous: &policy.AuthContext{
				Authenticated: true,
				AuthType:      "apikey",
				Subject:       "legacy-client",
			},
		},
	}

	result, err := translator.ToProtoSharedContext(shared)
	require.NoError(t, err)

	assert.Equal(t, "project-1", result.GetProjectId())
	assert.Equal(t, true, result.GetMetadata().GetFields()["flag"].GetBoolValue())
	require.NotNil(t, result.GetAuthContext())
	assert.Equal(t, "jwt", result.GetAuthContext().GetAuthType())
	assert.Equal(t, true, result.GetAuthContext().GetScopes()["read:pets"])
	require.NotNil(t, result.GetAuthContext().GetPrevious())
	assert.Equal(t, "apikey", result.GetAuthContext().GetPrevious().GetAuthType())
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
