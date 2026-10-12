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

package pythonbridge

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Translator converts between protobuf messages and Go v1alpha2 policy types.
type Translator struct{}

// NewTranslator creates a new Translator.
func NewTranslator() *Translator {
	return &Translator{}
}

// ToProtoSharedContext converts a Go SharedContext into the transport form.
func (t *Translator) ToProtoSharedContext(shared *policy.SharedContext) (*proto.SharedContext, error) {
	if shared == nil {
		return &proto.SharedContext{}, nil
	}

	metadata, err := toProtoStruct(shared.Metadata)
	if err != nil {
		return nil, fmt.Errorf("convert shared metadata: %w", err)
	}

	authContext, err := t.toProtoAuthContext(shared.AuthContext)
	if err != nil {
		return nil, fmt.Errorf("convert auth context: %w", err)
	}

	return &proto.SharedContext{
		ProjectId:            shared.ProjectID,
		RequestId:            shared.RequestID,
		Metadata:             metadata,
		ApiId:                shared.APIId,
		ApiName:              shared.APIName,
		ApiVersion:           shared.APIVersion,
		ApiKind:              string(shared.APIKind),
		ApiContext:           shared.APIContext,
		OperationPath:        shared.OperationPath,
		AuthContext:          authContext,
		ResolvedOperation:    shared.ResolvedOperation,
		ResolutionAttributes: toProtoResolutionAttributes(shared.ResolutionAttributes),
	}, nil
}

// toProtoResolutionAttributes flattens the resolver's attributes for the wire.
//
// This is where the Go read-only wrapper stops and a plain map takes over, and the
// asymmetry is deliberate. policy.ResolutionAttributes exists because a route resolved
// at deploy time shares one live map with every request on it, so a Go policy holding
// a reference could leak one request's data into the next. Across this boundary that
// cannot happen: the map is serialized, so the Python side deserializes its own copy
// per request, and nothing travels back — SharedContext only ever crosses outbound
// (there is no FromProtoSharedContext, by design).
//
// Copying here is therefore not a defensive copy, it is the serialization itself. A nil
// map yields nil rather than an empty one, so a route that inspected no request sends
// no field at all instead of an empty map on every request.
func toProtoResolutionAttributes(attrs policy.ResolutionAttributes) map[string]string {
	if attrs.Len() == 0 {
		return nil
	}
	out := make(map[string]string, attrs.Len())
	attrs.Iterate(func(name, value string) {
		out[name] = value
	})
	return out
}

// ToProtoHeaders converts read-only Go headers into the multi-value transport form.
func (t *Translator) ToProtoHeaders(headers *policy.Headers) *proto.Headers {
	result := &proto.Headers{Values: map[string]*proto.StringList{}}
	if headers == nil {
		return result
	}
	for name, values := range headers.GetAll() {
		result.Values[name] = &proto.StringList{Values: append([]string(nil), values...)}
	}
	return result
}

// ToProtoDownstream converts the downstream (client) header snapshot into the
// transport form. Returns nil when the snapshot is absent so the field is left
// unset on the wire and older/newer peers can detect its absence and fall back
// to legacy validation.
func (t *Translator) ToProtoDownstream(ds *policy.DownstreamContext) *proto.DownstreamContext {
	if ds == nil || ds.Request == nil {
		return nil
	}
	return &proto.DownstreamContext{
		Request: &proto.DownstreamRequest{
			Headers:   t.ToProtoHeaders(ds.Request.Headers),
			Path:      ds.Request.Path,
			Method:    ds.Request.Method,
			Authority: ds.Request.Authority,
			Scheme:    ds.Request.Scheme,
		},
	}
}

// ToProtoRequestUpstream converts the request-phase resolved upstream target
// into the transport form. Returns nil when absent (see ToProtoDownstream for
// the backward-compat contract).
func (t *Translator) ToProtoRequestUpstream(us *policy.UpstreamRequestContext) *proto.UpstreamRequestContext {
	if us == nil {
		return nil
	}
	return &proto.UpstreamRequestContext{
		Name:     us.Name,
		Url:      us.URL,
		BasePath: us.BasePath,
	}
}

// ToProtoUpstream converts the response-phase resolved upstream target and its
// response header snapshot into the transport form. Returns nil when absent
// (see ToProtoDownstream for the backward-compat contract).
func (t *Translator) ToProtoUpstream(us *policy.UpstreamResponseContext) *proto.UpstreamResponseContext {
	if us == nil {
		return nil
	}
	out := &proto.UpstreamResponseContext{
		Name:     us.Name,
		Url:      us.URL,
		BasePath: us.BasePath,
	}
	if us.Response != nil {
		out.Response = &proto.UpstreamResponse{
			Headers:    t.ToProtoHeaders(us.Response.Headers),
			StatusCode: int32(us.Response.StatusCode),
		}
	}
	return out
}

// ToProtoBody converts buffered body data into the transport form.
func (t *Translator) ToProtoBody(body *policy.Body) *proto.Body {
	if body == nil {
		return nil
	}
	return &proto.Body{
		Content:     append([]byte(nil), body.Content...),
		EndOfStream: body.EndOfStream,
		Present:     body.Present,
	}
}

// ToProtoStreamBody converts streaming chunk data into the transport form.
func (t *Translator) ToProtoStreamBody(body *policy.StreamBody) *proto.StreamBody {
	if body == nil {
		return nil
	}
	return &proto.StreamBody{
		Chunk:       append([]byte(nil), body.Chunk...),
		EndOfStream: body.EndOfStream,
		Index:       body.Index,
	}
}

// ToGoRequestHeaderAction converts a request-header response payload into a Go action.
func (t *Translator) ToGoRequestHeaderAction(resp *proto.StreamResponse) (policy.RequestHeaderAction, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetRequestHeaderAction()
	if payload == nil {
		return nil, nil
	}

	switch action := payload.Action.(type) {
	case *proto.RequestHeaderActionPayload_UpstreamRequestHeaderModifications:
		return t.toGoUpstreamRequestHeaderModifications(action.UpstreamRequestHeaderModifications), nil
	case *proto.RequestHeaderActionPayload_ImmediateResponse:
		return t.toGoImmediateResponse(action.ImmediateResponse), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected request-header action payload: %T", action)
	}
}

// ToGoRequestAction converts a request-body response payload into a Go action.
func (t *Translator) ToGoRequestAction(resp *proto.StreamResponse) (policy.RequestAction, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetRequestAction()
	if payload == nil {
		return nil, nil
	}

	switch action := payload.Action.(type) {
	case *proto.RequestActionPayload_UpstreamRequestModifications:
		return t.toGoUpstreamRequestModifications(action.UpstreamRequestModifications), nil
	case *proto.RequestActionPayload_ImmediateResponse:
		return t.toGoImmediateResponse(action.ImmediateResponse), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected request action payload: %T", action)
	}
}

// ToGoResponseHeaderAction converts a response-header response payload into a Go action.
func (t *Translator) ToGoResponseHeaderAction(resp *proto.StreamResponse) (policy.ResponseHeaderAction, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetResponseHeaderAction()
	if payload == nil {
		return nil, nil
	}

	switch action := payload.Action.(type) {
	case *proto.ResponseHeaderActionPayload_DownstreamResponseHeaderModifications:
		return t.toGoDownstreamResponseHeaderModifications(action.DownstreamResponseHeaderModifications), nil
	case *proto.ResponseHeaderActionPayload_ImmediateResponse:
		return t.toGoImmediateResponse(action.ImmediateResponse), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected response-header action payload: %T", action)
	}
}

// ToGoResponseAction converts a response-body response payload into a Go action.
func (t *Translator) ToGoResponseAction(resp *proto.StreamResponse) (policy.ResponseAction, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetResponseAction()
	if payload == nil {
		return nil, nil
	}

	switch action := payload.Action.(type) {
	case *proto.ResponseActionPayload_DownstreamResponseModifications:
		return t.toGoDownstreamResponseModifications(action.DownstreamResponseModifications), nil
	case *proto.ResponseActionPayload_ImmediateResponse:
		return t.toGoImmediateResponse(action.ImmediateResponse), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected response action payload: %T", action)
	}
}

// ToGoFaultResponse converts an on_fault response payload into a *policy.FaultResponse.
//
// Returns nil for an absent payload, and nil for a payload whose action is unset. Both mean
// the policy changed nothing, which is what nil means in the Go contract — so a Python fault
// policy that only notifies needs no return value at all, exactly like its Go counterpart.
func (t *Translator) ToGoFaultResponse(resp *proto.StreamResponse) (*policy.FaultResponse, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetFaultResponseAction()
	if payload == nil {
		return nil, nil
	}
	fault := payload.GetFaultResponse()
	if fault == nil {
		return nil, nil
	}

	return &policy.FaultResponse{
		StatusCode:            int32PtrValue(fault.GetStatusCode()),
		Body:                  bytesValue(fault.GetBody()),
		Fault:                 toGoErrorResponse(fault.GetFault()),
		HeadersToSet:          cloneStringMap(fault.GetHeadersToSet()),
		HeadersToAppend:       stringListMapToSliceMap(fault.GetHeadersToAppend()),
		HeadersToRemove:       append([]string(nil), fault.GetHeadersToRemove()...),
		AnalyticsMetadata:     structToMap(fault.GetAnalyticsMetadata()),
		DynamicMetadata:       structMapToNestedMap(fault.GetDynamicMetadata()),
		AnalyticsHeaderFilter: t.toGoDropHeaderAction(fault.GetAnalyticsHeaderFilter()),
	}, nil
}

// ToGoNeedsMoreDecision converts a needs-more response payload into a Go boolean.
func (t *Translator) ToGoNeedsMoreDecision(resp *proto.StreamResponse) (bool, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return false, err
	}

	payload := resp.GetNeedsMoreDecision()
	if payload == nil {
		return false, nil
	}
	return payload.GetNeedsMore(), nil
}

// ToGoStreamingRequestAction converts a streaming-request response payload into a Go action.
func (t *Translator) ToGoStreamingRequestAction(resp *proto.StreamResponse) (policy.StreamingRequestAction, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetStreamingRequestAction()
	if payload == nil || payload.ForwardRequestChunk == nil {
		return nil, nil
	}
	return t.toGoForwardRequestChunk(payload.ForwardRequestChunk), nil
}

// ToGoStreamingResponseAction converts a streaming-response response payload into a Go action.
func (t *Translator) ToGoStreamingResponseAction(resp *proto.StreamResponse) (policy.StreamingResponseAction, error) {
	if err := executionErrorFromResponse(resp); err != nil {
		return nil, err
	}

	payload := resp.GetStreamingResponseAction()
	if payload == nil {
		return nil, nil
	}

	switch action := payload.Action.(type) {
	case *proto.StreamingResponseActionPayload_ForwardResponseChunk:
		return t.toGoForwardResponseChunk(action.ForwardResponseChunk), nil
	case *proto.StreamingResponseActionPayload_TerminateResponseChunk:
		return t.toGoTerminateResponseChunk(action.TerminateResponseChunk), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected streaming response action payload: %T", action)
	}
}

func (t *Translator) toProtoAuthContext(ctx *policy.AuthContext) (*proto.AuthContext, error) {
	if ctx == nil {
		return nil, nil
	}

	scopes := make(map[string]bool, len(ctx.Scopes))
	for key, value := range ctx.Scopes {
		scopes[key] = value
	}

	properties := make(map[string]string, len(ctx.Properties))
	for key, value := range ctx.Properties {
		properties[key] = value
	}

	// Fail closed: if the structured claims cannot be serialized (e.g. an unsupported nested
	// value type), surface the error rather than dropping TypedProperties. Silently omitting it
	// would make downstream policies see the claim as absent and could change an authorization
	// decision. Numeric note: google.protobuf.Struct carries numbers as IEEE-754 doubles, so
	// integer-valued claims lose exactness beyond 2^53 — this matches how JSON/JWT claims are
	// already decoded (float64) and introduces no additional loss.
	var typedProperties *structpb.Struct
	if len(ctx.TypedProperties) > 0 {
		s, err := structpb.NewStruct(ctx.TypedProperties)
		if err != nil {
			return nil, fmt.Errorf("convert auth context typed properties: %w", err)
		}
		typedProperties = s
	}

	previous, err := t.toProtoAuthContext(ctx.Previous)
	if err != nil {
		return nil, err
	}

	return &proto.AuthContext{
		Authenticated:   ctx.Authenticated,
		Authorized:      ctx.Authorized,
		AuthType:        ctx.AuthType,
		Subject:         ctx.Subject,
		Issuer:          ctx.Issuer,
		TokenId:         ctx.TokenId,
		Audience:        append([]string(nil), ctx.Audience...),
		Scopes:          scopes,
		CredentialId:    ctx.CredentialID,
		Properties:      properties,
		TypedProperties: typedProperties,
		Previous:        previous,
	}, nil
}

func (t *Translator) toGoImmediateResponse(resp *proto.ImmediateResponse) policy.ImmediateResponse {
	if resp == nil {
		return policy.ImmediateResponse{}
	}
	return policy.ImmediateResponse{
		StatusCode:            int(resp.GetStatusCode()),
		Headers:               cloneStringMap(resp.GetHeaders()),
		Body:                  bytesValue(resp.GetBody()),
		AnalyticsMetadata:     structToMap(resp.GetAnalyticsMetadata()),
		DynamicMetadata:       structMapToNestedMap(resp.GetDynamicMetadata()),
		AnalyticsHeaderFilter: t.toGoDropHeaderAction(resp.GetAnalyticsHeaderFilter()),
		IsFault:               resp.GetIsFault(),
		Fault:                 toGoErrorResponse(resp.GetFault()),
	}
}

func (t *Translator) toGoUpstreamRequestHeaderModifications(mod *proto.UpstreamRequestHeaderModifications) policy.UpstreamRequestHeaderModifications {
	if mod == nil {
		return policy.UpstreamRequestHeaderModifications{}
	}
	return policy.UpstreamRequestHeaderModifications{
		HeadersToSet:            cloneStringMap(mod.GetHeadersToSet()),
		HeadersToRemove:         append([]string(nil), mod.GetHeadersToRemove()...),
		UpstreamName:            stringPtrValue(mod.GetUpstreamName()),
		Path:                    stringPtrValue(mod.GetPath()),
		Host:                    stringPtrValue(mod.GetHost()),
		Method:                  stringPtrValue(mod.GetMethod()),
		QueryParametersToAdd:    stringListMapToSliceMap(mod.GetQueryParametersToAdd()),
		QueryParametersToRemove: append([]string(nil), mod.GetQueryParametersToRemove()...),
		AnalyticsMetadata:       structToMap(mod.GetAnalyticsMetadata()),
		DynamicMetadata:         structMapToNestedMap(mod.GetDynamicMetadata()),
		AnalyticsHeaderFilter:   t.toGoDropHeaderAction(mod.GetAnalyticsHeaderFilter()),
	}
}

func (t *Translator) toGoUpstreamRequestModifications(mod *proto.UpstreamRequestModifications) policy.UpstreamRequestModifications {
	if mod == nil {
		return policy.UpstreamRequestModifications{}
	}
	return policy.UpstreamRequestModifications{
		Body:                    bytesValue(mod.GetBody()),
		HeadersToSet:            cloneStringMap(mod.GetHeadersToSet()),
		HeadersToRemove:         append([]string(nil), mod.GetHeadersToRemove()...),
		UpstreamName:            stringPtrValue(mod.GetUpstreamName()),
		Path:                    stringPtrValue(mod.GetPath()),
		Host:                    stringPtrValue(mod.GetHost()),
		Method:                  stringPtrValue(mod.GetMethod()),
		QueryParametersToAdd:    stringListMapToSliceMap(mod.GetQueryParametersToAdd()),
		QueryParametersToRemove: append([]string(nil), mod.GetQueryParametersToRemove()...),
		AnalyticsMetadata:       structToMap(mod.GetAnalyticsMetadata()),
		DynamicMetadata:         structMapToNestedMap(mod.GetDynamicMetadata()),
		AnalyticsHeaderFilter:   t.toGoDropHeaderAction(mod.GetAnalyticsHeaderFilter()),
	}
}

func (t *Translator) toGoDownstreamResponseHeaderModifications(mod *proto.DownstreamResponseHeaderModifications) policy.DownstreamResponseHeaderModifications {
	if mod == nil {
		return policy.DownstreamResponseHeaderModifications{}
	}
	return policy.DownstreamResponseHeaderModifications{
		HeadersToSet:          cloneStringMap(mod.GetHeadersToSet()),
		HeadersToRemove:       append([]string(nil), mod.GetHeadersToRemove()...),
		AnalyticsMetadata:     structToMap(mod.GetAnalyticsMetadata()),
		DynamicMetadata:       structMapToNestedMap(mod.GetDynamicMetadata()),
		AnalyticsHeaderFilter: t.toGoDropHeaderAction(mod.GetAnalyticsHeaderFilter()),
	}
}

func (t *Translator) toGoDownstreamResponseModifications(mod *proto.DownstreamResponseModifications) policy.DownstreamResponseModifications {
	if mod == nil {
		return policy.DownstreamResponseModifications{}
	}
	return policy.DownstreamResponseModifications{
		Body:                  bytesValue(mod.GetBody()),
		StatusCode:            int32PtrValue(mod.GetStatusCode()),
		HeadersToSet:          cloneStringMap(mod.GetHeadersToSet()),
		HeadersToRemove:       append([]string(nil), mod.GetHeadersToRemove()...),
		AnalyticsMetadata:     structToMap(mod.GetAnalyticsMetadata()),
		DynamicMetadata:       structMapToNestedMap(mod.GetDynamicMetadata()),
		AnalyticsHeaderFilter: t.toGoDropHeaderAction(mod.GetAnalyticsHeaderFilter()),
		IsFault:               mod.GetIsFault(),
		Fault:                 toGoErrorResponse(mod.GetFault()),
	}
}

func (t *Translator) toGoForwardRequestChunk(chunk *proto.ForwardRequestChunk) policy.ForwardRequestChunk {
	if chunk == nil {
		return policy.ForwardRequestChunk{}
	}
	return policy.ForwardRequestChunk{
		Body:              bytesValue(chunk.GetBody()),
		AnalyticsMetadata: structToMap(chunk.GetAnalyticsMetadata()),
		DynamicMetadata:   structMapToNestedMap(chunk.GetDynamicMetadata()),
	}
}

func (t *Translator) toGoForwardResponseChunk(chunk *proto.ForwardResponseChunk) policy.ForwardResponseChunk {
	if chunk == nil {
		return policy.ForwardResponseChunk{}
	}
	return policy.ForwardResponseChunk{
		Body:              bytesValue(chunk.GetBody()),
		AnalyticsMetadata: structToMap(chunk.GetAnalyticsMetadata()),
		DynamicMetadata:   structMapToNestedMap(chunk.GetDynamicMetadata()),
	}
}

func (t *Translator) toGoTerminateResponseChunk(chunk *proto.TerminateResponseChunk) policy.TerminateResponseChunk {
	if chunk == nil {
		return policy.TerminateResponseChunk{}
	}
	return policy.TerminateResponseChunk{
		Body:              bytesValue(chunk.GetBody()),
		AnalyticsMetadata: structToMap(chunk.GetAnalyticsMetadata()),
		DynamicMetadata:   structMapToNestedMap(chunk.GetDynamicMetadata()),
		IsFault:           chunk.GetIsFault(),
		Fault:             toGoErrorResponse(chunk.GetFault()),
	}
}

// toGoErrorResponse carries a Python policy's description of its own failure across the bridge.
//
// nil in, nil out: "described nothing" has to stay distinguishable from "described an empty
// error", because the engine treats a non-nil Error as something to render.
//
// Policy is not read from the wire even though FaultDetails has the field. It is gateway-owned
// — the engine overwrites it from the chain it just executed — so accepting a value here would
// only let a Python policy submit an attribution that is then discarded.
func toGoErrorResponse(err *proto.FaultDetails) *policy.FaultDetails {
	if err == nil {
		return nil
	}
	return &policy.FaultDetails{
		Code:        err.GetCode(),
		Type:        err.GetType(),
		Direction:   err.GetDirection(),
		Message:     err.GetMessage(),
		Description: err.GetDescription(),
		JSONRPC:     toGoJSONRPCError(err.GetJsonrpc()),
		Guardrail:   toGoGuardrailError(err.GetGuardrail()),
	}
}

// toGoGuardrailError carries a Python guardrail's assessment detail out across the bridge.
//
// nil in, nil out: absent means "no guardrail was involved", which is what the renderers test.
// It does NOT mean the operator declined to show the assessment — that is expressed by an
// present block with empty Assessments, and the two must stay distinguishable.
func toGoGuardrailError(err *proto.GuardrailDetails) *policy.GuardrailDetails {
	if err == nil {
		return nil
	}
	return &policy.GuardrailDetails{
		InterveningGuardrail: err.GetInterveningGuardrail(),
		Action:               err.GetAction(),
		ActionReason:         err.GetActionReason(),
		Assessments:          structToMap(err.GetAssessments()),
	}
}

// toGoJSONRPCError carries a Python policy's JSON-RPC detail out across the bridge.
//
// nil in, nil out, and an unset code stays unset: a Python policy that fills in an id but no
// code is saying "echo my id, derive the code from the status", and flattening that to 0 would
// silently override the engine's derivation with an invalid code.
func toGoJSONRPCError(err *proto.JSONRPCError) *policy.JSONRPCError {
	if err == nil {
		return nil
	}
	return &policy.JSONRPCError{
		Code: int32PtrValue(err.GetCode()),
		ID:   err.GetId().AsInterface(), // nil-safe: a nil Value yields a nil any
	}
}

// toProtoErrorResponse carries the gateway's description of a failure INTO a Python fault
// handler — the mirror of toGoErrorResponse, which carries a policy's description back out.
//
// nil in, nil out, for the same reason: a handler must be able to tell "nothing described this
// failure" from "an empty description", since only the former means it has to fall back to the
// status. Policy is omitted from the wire message, so it is carried on FaultContext instead.
func toProtoErrorResponse(err *policy.FaultDetails) *proto.FaultDetails {
	if err == nil {
		return nil
	}
	return &proto.FaultDetails{
		Code:        err.Code,
		Type:        err.Type,
		Direction:   err.Direction,
		Message:     err.Message,
		Description: err.Description,
		Jsonrpc:     toProtoJSONRPCError(err.JSONRPC),
		Guardrail:   toProtoGuardrailError(err.Guardrail),
	}
}

// toProtoGuardrailError carries guardrail detail INTO a Python fault handler, so a handler can
// report which guardrail acted and why without re-deriving it.
//
// An assessment structpb cannot represent is dropped rather than failing the conversion: the
// block's metadata is the part a handler needs most, and losing one field beats losing the
// whole handler call.
func toProtoGuardrailError(err *policy.GuardrailDetails) *proto.GuardrailDetails {
	if err == nil {
		return nil
	}
	out := &proto.GuardrailDetails{
		InterveningGuardrail: err.InterveningGuardrail,
		Action:               err.Action,
		ActionReason:         err.ActionReason,
	}
	if len(err.Assessments) > 0 {
		if s, convErr := structpb.NewStruct(err.Assessments); convErr == nil {
			out.Assessments = s
		}
	}
	return out
}

// toProtoJSONRPCError carries JSON-RPC detail INTO a Python fault handler, so a handler can
// report the code and id the failing policy supplied rather than re-deriving them.
//
// An id that structpb cannot represent is dropped rather than failing the conversion: the id
// is JSON that came off the wire in the first place, so this is unreachable in practice, and
// losing correlation on one error beats losing the whole fault handler call.
func toProtoJSONRPCError(err *policy.JSONRPCError) *proto.JSONRPCError {
	if err == nil {
		return nil
	}
	out := &proto.JSONRPCError{}
	if err.Code != nil {
		out.Code = wrapperspb.Int32(int32(*err.Code))
	}
	if err.ID != nil {
		if v, convErr := structpb.NewValue(err.ID); convErr == nil {
			out.Id = v
		}
	}
	return out
}

func (t *Translator) toGoDropHeaderAction(action *proto.DropHeaderAction) policy.DropHeaderAction {
	if action == nil {
		return policy.DropHeaderAction{}
	}

	var actionValue string
	switch action.GetAction() {
	case proto.DropHeaderActionType_DROP_HEADER_ACTION_TYPE_ALLOW:
		actionValue = "allow"
	case proto.DropHeaderActionType_DROP_HEADER_ACTION_TYPE_DENY:
		actionValue = "deny"
	default:
		actionValue = ""
	}

	return policy.DropHeaderAction{
		Action:  actionValue,
		Headers: append([]string(nil), action.GetHeaders()...),
	}
}

func executionErrorFromResponse(resp *proto.StreamResponse) error {
	if resp == nil {
		return fmt.Errorf("python executor returned nil response")
	}
	if errPayload := resp.GetError(); errPayload != nil {
		return fmt.Errorf(
			"python executor %s for %s:%s: %s",
			errPayload.GetErrorType(),
			errPayload.GetPolicyName(),
			errPayload.GetPolicyVersion(),
			errPayload.GetMessage(),
		)
	}
	return nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func stringListMapToSliceMap(values map[string]*proto.StringList) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string][]string, len(values))
	for key, list := range values {
		if list == nil {
			result[key] = nil
			continue
		}
		result[key] = append([]string(nil), list.GetValues()...)
	}
	return result
}

func structMapToNestedMap(values map[string]*structpb.Struct) map[string]map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]map[string]any, len(values))
	for key, value := range values {
		result[key] = structToMap(value)
	}
	return result
}

func structToMap(value *structpb.Struct) map[string]any {
	if value == nil {
		return nil
	}
	return value.AsMap()
}

func toProtoStruct(values map[string]interface{}) (*structpb.Struct, error) {
	if values == nil {
		return nil, nil
	}
	return structpb.NewStruct(values)
}

func stringPtrValue(value *wrapperspb.StringValue) *string {
	if value == nil {
		return nil
	}
	result := value.GetValue()
	return &result
}

func int32PtrValue(value *wrapperspb.Int32Value) *int {
	if value == nil {
		return nil
	}
	result := int(value.GetValue())
	return &result
}

func bytesValue(value *wrapperspb.BytesValue) []byte {
	if value == nil {
		return nil
	}
	return append([]byte(nil), value.GetValue()...)
}
