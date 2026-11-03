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

// The resolved operation and its request facts have to cross the bridge, or a Python
// policy reads empty defaults while the Go engine has both values — the SDK fields
// would be declared and permanently dead for that whole class of policy.
func TestTranslatorToProtoSharedContextCarriesTheResolvedOperation(t *testing.T) {
	shared := &policy.SharedContext{
		APIKind:           policy.APIKindAgent,
		APIName:           "WeatherAgent",
		OperationPath:     "/",
		ResolvedOperation: "SendMessage",
		ResolutionAttributes: policy.NewResolutionAttributes(map[string]string{
			"a2a.operation":        "SendMessage",
			"a2a.transport":        "JSONRPC",
			"a2a.protocol.version": "1.0",
			"a2a.context.id":       "ctx-1",
			"a2a.task.id":          "task-1",
		}),
	}

	result, err := NewTranslator().ToProtoSharedContext(shared)
	require.NoError(t, err)

	assert.Equal(t, "SendMessage", result.GetResolvedOperation())
	assert.Equal(t, map[string]string{
		"a2a.operation":        "SendMessage",
		"a2a.transport":        "JSONRPC",
		"a2a.protocol.version": "1.0",
		"a2a.context.id":       "ctx-1",
		"a2a.task.id":          "task-1",
	}, result.GetResolutionAttributes())
}

// A JSON-RPC route serves every operation on one path, so OperationPath cannot tell
// them apart. This is the case the plumbing exists for: same route, same path,
// different operation on the wire.
func TestTranslatorToProtoSharedContextDistinguishesOperationsOnOnePath(t *testing.T) {
	protoFor := func(operation string) *proto.SharedContext {
		result, err := NewTranslator().ToProtoSharedContext(&policy.SharedContext{
			APIKind:           policy.APIKindAgent,
			OperationPath:     "/",
			ResolvedOperation: operation,
			ResolutionAttributes: policy.NewResolutionAttributes(
				map[string]string{"a2a.operation": operation}),
		})
		require.NoError(t, err)
		return result
	}

	send, get := protoFor("SendMessage"), protoFor("GetTask")

	assert.Equal(t, send.GetOperationPath(), get.GetOperationPath(),
		"precondition: both operations share one path, which is why OperationPath cannot serve")
	assert.NotEqual(t, send.GetResolvedOperation(), get.GetResolvedOperation())
	assert.Equal(t, "SendMessage", send.GetResolutionAttributes()["a2a.operation"])
	assert.Equal(t, "GetTask", get.GetResolutionAttributes()["a2a.operation"])
}

// Every API kind that shipped before Agent resolves its chain from the route, so it
// sends no operation and no attributes at all. Nil rather than an empty map, so those
// requests do not pay for a field they never populate.
func TestTranslatorToProtoSharedContextOmitsResolutionForADirectRoute(t *testing.T) {
	result, err := NewTranslator().ToProtoSharedContext(&policy.SharedContext{
		APIKind:       policy.APIKindRestApi,
		OperationPath: "/pets/{id}",
	})
	require.NoError(t, err)

	assert.Empty(t, result.GetResolvedOperation())
	assert.Nil(t, result.GetResolutionAttributes())
}

// The wire map must be the translator's own. If it aliased the resolver's map, a
// route resolved at ingest — which shares one map across every request on it — could
// have one request's serialization mutated by another's.
func TestTranslatorToProtoSharedContextDoesNotAliasTheResolverMap(t *testing.T) {
	source := map[string]string{"a2a.context.id": "ctx-1"}
	shared := &policy.SharedContext{
		APIKind:              policy.APIKindAgent,
		ResolvedOperation:    "GetTask",
		ResolutionAttributes: policy.NewResolutionAttributes(source),
	}

	result, err := NewTranslator().ToProtoSharedContext(shared)
	require.NoError(t, err)

	result.GetResolutionAttributes()["a2a.context.id"] = "tampered"
	assert.Equal(t, "ctx-1", source["a2a.context.id"],
		"the resolver's map must be unreachable through the wire form")
}

// A nil shared context still yields a usable message rather than panicking on the
// attribute conversion.
func TestTranslatorToProtoSharedContextNilIsSafe(t *testing.T) {
	result, err := NewTranslator().ToProtoSharedContext(nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.GetResolvedOperation())
	assert.Nil(t, result.GetResolutionAttributes())
}

// A Python policy's fault declaration must survive the bridge, and the three shapes that mean
// different things must stay distinguishable on the Go side.
//
// The error-only case is the one worth pinning: it is NOT a fault, yet the description has to
// arrive, because that is how a Python policy returns a deliberate non-failing response with an
// error-shaped body — a canned 404, an auth challenge, a cache miss.
func TestTranslator_CarriesTheFaultDeclaration(t *testing.T) {
	tr := &Translator{}

	t.Run("declared and described", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 401,
			IsFault:    true,
			Fault: &proto.FaultDetails{
				Code: "900902", Type: "authentication", Direction: "Request",
				Message: "Valid credentials required", Description: "detail",
			},
		})
		assert.True(t, got.IsFault)
		require.NotNil(t, got.Fault)
		assert.Equal(t, "900902", got.Fault.Code)
		assert.Equal(t, "authentication", got.Fault.Type)
		assert.Equal(t, "Request", got.Fault.Direction)
		// Description crosses the bridge even though no renderer emits it: a Python fault
		// handler reporting to an audit sink is exactly who needs it.
		assert.Equal(t, "detail", got.Fault.Description)
		assert.True(t, got.IsFault)
	})

	t.Run("described but not declared stays out of the fault flow", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 404,
			Fault:      &proto.FaultDetails{Code: "961000", Message: "No such widget"},
		})
		assert.False(t, got.IsFault)
		require.NotNil(t, got.Fault, "the description must still arrive — it shapes the body")
		assert.Equal(t, "961000", got.Fault.Code)
		assert.False(t, got.IsFault, "describing an error does not opt in")
	})

	t.Run("nothing declared", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{StatusCode: 404})
		assert.False(t, got.IsFault)
		assert.Nil(t, got.Fault, "absent must stay distinguishable from an empty description")
		assert.False(t, got.IsFault)
	})

	t.Run("a response modification that only sets a status is not rejecting", func(t *testing.T) {
		status := int32(503)
		got := tr.toGoDownstreamResponseModifications(&proto.DownstreamResponseModifications{
			StatusCode: wrapperspb.Int32(status),
		})
		assert.False(t, got.IsFault, "a Python policy relabelling the backend's error is not a fault")
		require.NotNil(t, got.StatusCode)
		assert.Equal(t, 503, *got.StatusCode)
	})

	t.Run("a stream termination declares", func(t *testing.T) {
		got := tr.toGoTerminateResponseChunk(&proto.TerminateResponseChunk{
			IsFault: true,
			Fault:   &proto.FaultDetails{Code: "906000", Type: "guardrail"},
		})
		assert.True(t, got.IsFault)
		require.NotNil(t, got.Fault)
		assert.Equal(t, "906000", got.Fault.Code)
	})
}

// The JSON-RPC block has to survive the bridge in both directions, and "unset" has to stay
// unset. A Python MCP policy that states -32602 loses the distinction between "invalid params"
// and the generic "invalid request" if the code flattens; a policy that echoes the request id
// loses call correlation if the id does.
func TestTranslator_CarriesTheJSONRPCBlock(t *testing.T) {
	tr := &Translator{}

	t.Run("code and id arrive from Python", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 400,
			IsFault:    true,
			Fault: &proto.FaultDetails{
				Message: "Invalid MCP request params",
				Jsonrpc: &proto.JSONRPCError{
					Code: wrapperspb.Int32(-32602),
					Id:   structpb.NewStringValue("call-7"),
				},
			},
		})
		require.NotNil(t, got.Fault)
		require.NotNil(t, got.Fault.JSONRPC)
		require.NotNil(t, got.Fault.JSONRPC.Code)
		assert.Equal(t, -32602, *got.Fault.JSONRPC.Code)
		assert.Equal(t, "call-7", got.Fault.JSONRPC.ID)
	})

	// A numeric id must not become a string on the way across: JSON-RPC lets the client choose,
	// and a client matching on the number it sent would not recognise "7".
	t.Run("a numeric id keeps its type", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 400,
			Fault: &proto.FaultDetails{
				Jsonrpc: &proto.JSONRPCError{Id: structpb.NewNumberValue(7)},
			},
		})
		require.NotNil(t, got.Fault.JSONRPC)
		assert.Equal(t, float64(7), got.Fault.JSONRPC.ID)
	})

	// An absent code must arrive as nil, not 0. Zero is not a valid JSON-RPC code, and treating
	// it as one would suppress the engine's status-derived code with nonsense.
	t.Run("an unset code stays unset", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 429,
			Fault:      &proto.FaultDetails{Jsonrpc: &proto.JSONRPCError{Id: structpb.NewStringValue("x")}},
		})
		require.NotNil(t, got.Fault.JSONRPC)
		assert.Nil(t, got.Fault.JSONRPC.Code, "absent must not flatten to 0")
	})

	// Absent block stays absent — a non-MCP policy must not acquire an empty JSON-RPC block
	// just by describing an error.
	t.Run("no block stays nil", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 500,
			Fault:      &proto.FaultDetails{Message: "boom"},
		})
		require.NotNil(t, got.Fault)
		assert.Nil(t, got.Fault.JSONRPC)
	})

	// The inbound direction: a Python fault handler must see the code and id the failing
	// policy supplied, so it can report the specific failure rather than re-deriving it.
	t.Run("round trips into a Python fault handler", func(t *testing.T) {
		code := -32700
		out := toProtoErrorResponse(&policy.FaultDetails{
			Message: "Parse error",
			JSONRPC: &policy.JSONRPCError{Code: &code, ID: "abc"},
		})
		require.NotNil(t, out.GetJsonrpc())
		assert.Equal(t, int32(-32700), out.GetJsonrpc().GetCode().GetValue())
		assert.Equal(t, "abc", out.GetJsonrpc().GetId().GetStringValue())

		back := toGoErrorResponse(out)
		require.NotNil(t, back.JSONRPC)
		require.NotNil(t, back.JSONRPC.Code)
		assert.Equal(t, code, *back.JSONRPC.Code)
		assert.Equal(t, "abc", back.JSONRPC.ID)
	})
}

// The guardrail block has to survive the bridge in both directions, or a Python guardrail
// cannot return an assessment at all — the gap this closes.
//
// The distinction that matters here is between an ABSENT block and a block with no
// assessments. Absent means "no guardrail was involved", which is what the renderers test.
// Present-but-empty means "a guardrail acted and the operator did not opt into showing what it
// found" — showAssessment: false. Collapsing the two would either hide every intervention or
// disclose every assessment.
func TestTranslator_CarriesTheGuardrailBlock(t *testing.T) {
	tr := &Translator{}

	t.Run("a full block arrives from Python", func(t *testing.T) {
		assessments, err := structpb.NewStruct(map[string]any{
			"assessments": "Expected word count to be between 10 and 500 words.",
		})
		require.NoError(t, err)

		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 422,
			IsFault:    true,
			Fault: &proto.FaultDetails{
				Code: "906000", Type: "guardrail", Direction: "Response",
				Message: "Violation of applied word count constraints detected",
				Guardrail: &proto.GuardrailDetails{
					InterveningGuardrail: "word-count-guardrail-py",
					Action:               policy.GuardrailActionIntervened,
					ActionReason:         "too many words",
					Assessments:          assessments,
				},
			},
		})
		require.NotNil(t, got.Fault)
		require.NotNil(t, got.Fault.Guardrail)
		assert.Equal(t, "word-count-guardrail-py", got.Fault.Guardrail.InterveningGuardrail)
		assert.Equal(t, policy.GuardrailActionIntervened, got.Fault.Guardrail.Action)
		assert.Equal(t, "too many words", got.Fault.Guardrail.ActionReason)
		assert.Equal(t, "Expected word count to be between 10 and 500 words.",
			got.Fault.Guardrail.Assessments["assessments"])
	})

	// showAssessment: false — the block is sent, the evidence is not.
	t.Run("metadata without assessments still arrives", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 422,
			Fault: &proto.FaultDetails{
				Guardrail: &proto.GuardrailDetails{
					InterveningGuardrail: "regex-guardrail-py",
					Action:               policy.GuardrailActionIntervened,
				},
			},
		})
		require.NotNil(t, got.Fault.Guardrail)
		assert.Equal(t, "regex-guardrail-py", got.Fault.Guardrail.InterveningGuardrail)
		assert.Empty(t, got.Fault.Guardrail.Assessments,
			"an unset assessments Struct must not become a populated map")
	})

	// A non-guardrail policy must not acquire an empty block just by describing an error.
	t.Run("no block stays nil", func(t *testing.T) {
		got := tr.toGoImmediateResponse(&proto.ImmediateResponse{
			StatusCode: 401,
			Fault:      &proto.FaultDetails{Code: "900902", Message: "Unauthorized"},
		})
		require.NotNil(t, got.Fault)
		assert.Nil(t, got.Fault.Guardrail)
	})

	// Inbound: a Python fault handler must see which guardrail acted and why.
	t.Run("round trips into a Python fault handler", func(t *testing.T) {
		out := toProtoErrorResponse(&policy.FaultDetails{
			Message: "Blocked",
			Guardrail: &policy.GuardrailDetails{
				InterveningGuardrail: "url-guardrail",
				Action:               policy.GuardrailActionIntervened,
				ActionReason:         "disallowed host",
				Assessments:          map[string]any{"matched": "evil.example"},
			},
		})
		require.NotNil(t, out.GetGuardrail())
		assert.Equal(t, "url-guardrail", out.GetGuardrail().GetInterveningGuardrail())
		assert.Equal(t, "disallowed host", out.GetGuardrail().GetActionReason())

		back := toGoErrorResponse(out)
		require.NotNil(t, back.Guardrail)
		assert.Equal(t, "evil.example", back.Guardrail.Assessments["matched"])
	})
}
