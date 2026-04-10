/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

	return &proto.SharedContext{
		ProjectId:     shared.ProjectID,
		RequestId:     shared.RequestID,
		Metadata:      metadata,
		ApiId:         shared.APIId,
		ApiName:       shared.APIName,
		ApiVersion:    shared.APIVersion,
		ApiKind:       string(shared.APIKind),
		ApiContext:    shared.APIContext,
		OperationPath: shared.OperationPath,
		AuthContext:   t.toProtoAuthContext(shared.AuthContext),
	}, nil
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

func (t *Translator) toProtoAuthContext(ctx *policy.AuthContext) *proto.AuthContext {
	if ctx == nil {
		return nil
	}

	scopes := make(map[string]bool, len(ctx.Scopes))
	for key, value := range ctx.Scopes {
		scopes[key] = value
	}

	properties := make(map[string]string, len(ctx.Properties))
	for key, value := range ctx.Properties {
		properties[key] = value
	}

	return &proto.AuthContext{
		Authenticated: ctx.Authenticated,
		Authorized:    ctx.Authorized,
		AuthType:      ctx.AuthType,
		Subject:       ctx.Subject,
		Issuer:        ctx.Issuer,
		Audience:      append([]string(nil), ctx.Audience...),
		Scopes:        scopes,
		CredentialId:  ctx.CredentialID,
		Properties:    properties,
		Previous:      t.toProtoAuthContext(ctx.Previous),
	}
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
	}
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
