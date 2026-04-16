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
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// bridge is the unified Python policy adapter. Mode() is the single source of
// truth for which phases are active. The engine gates phase execution via Mode()
// at chain-build and execution time. The bridge also guards internally as
// defense-in-depth before issuing gRPC calls to the Python executor.
type bridge struct {
	policyName    string
	policyVersion string
	mode          policy.ProcessingMode
	metadata      policy.PolicyMetadata
	streamManager *StreamManager
	translator    *Translator
	slogger       *slog.Logger
	instanceID    string
	closeOnce     sync.Once
	closeErr      error
}

var (
	_ policy.Policy                  = (*bridge)(nil)
	_ policy.RequestHeaderPolicy     = (*bridge)(nil)
	_ policy.ResponseHeaderPolicy    = (*bridge)(nil)
	_ policy.RequestPolicy           = (*bridge)(nil)
	_ policy.ResponsePolicy          = (*bridge)(nil)
	_ policy.StreamingRequestPolicy  = (*bridge)(nil)
	_ policy.StreamingResponsePolicy = (*bridge)(nil)
)

func (b *bridge) Mode() policy.ProcessingMode {
	return b.mode
}

func (b *bridge) OnRequestHeaders(ctx context.Context, reqCtx *policy.RequestHeaderContext, params map[string]interface{}) policy.RequestHeaderAction {
	if b.mode.RequestHeaderMode != policy.HeaderModeProcess {
		return policy.UpstreamRequestHeaderModifications{}
	}

	req, err := b.buildRequestHeadersRequest(ctx, reqCtx, params)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to build request-header payload", "error", err)
		return b.requestHeaderErrorAction(err)
	}

	resp, err := b.execute(ctx, req, reqCtx.SharedContext)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to execute Python request-header policy", "error", err)
		return b.requestHeaderErrorAction(err)
	}

	action, err := b.translator.ToGoRequestHeaderAction(resp)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to translate Python request-header response", "error", err)
		return b.requestHeaderErrorAction(err)
	}
	return action
}

func (b *bridge) OnRequestBody(ctx context.Context, reqCtx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if b.mode.RequestBodyMode == policy.BodyModeSkip {
		return nil
	}

	req, err := b.buildRequestBodyRequest(ctx, reqCtx, params)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to build request-body payload", "error", err)
		return b.requestBodyErrorAction(err)
	}

	resp, err := b.execute(ctx, req, reqCtx.SharedContext)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to execute Python request-body policy", "error", err)
		return b.requestBodyErrorAction(err)
	}

	action, err := b.translator.ToGoRequestAction(resp)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to translate Python request-body response", "error", err)
		return b.requestBodyErrorAction(err)
	}
	return action
}

func (b *bridge) OnResponseHeaders(ctx context.Context, respCtx *policy.ResponseHeaderContext, params map[string]interface{}) policy.ResponseHeaderAction {
	if b.mode.ResponseHeaderMode != policy.HeaderModeProcess {
		return policy.DownstreamResponseHeaderModifications{}
	}

	req, err := b.buildResponseHeadersRequest(ctx, respCtx, params)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to build response-header payload", "error", err)
		return b.responseHeaderErrorAction(err)
	}

	resp, err := b.execute(ctx, req, respCtx.SharedContext)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to execute Python response-header policy", "error", err)
		return b.responseHeaderErrorAction(err)
	}

	action, err := b.translator.ToGoResponseHeaderAction(resp)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to translate Python response-header response", "error", err)
		return b.responseHeaderErrorAction(err)
	}
	return action
}

func (b *bridge) OnResponseBody(ctx context.Context, respCtx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	if b.mode.ResponseBodyMode == policy.BodyModeSkip {
		return nil
	}

	req, err := b.buildResponseBodyRequest(ctx, respCtx, params)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to build response-body payload", "error", err)
		return b.responseBodyErrorAction(err)
	}

	resp, err := b.execute(ctx, req, respCtx.SharedContext)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to execute Python response-body policy", "error", err)
		return b.responseBodyErrorAction(err)
	}

	action, err := b.translator.ToGoResponseAction(resp)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to translate Python response-body response", "error", err)
		return b.responseBodyErrorAction(err)
	}
	return action
}

func (b *bridge) NeedsMoreRequestData(accumulated []byte) bool {
	if b.mode.RequestBodyMode != policy.BodyModeStream {
		return false
	}

	req, err := b.buildNeedsMoreRequestDataRequest(context.Background(), accumulated)
	if err != nil {
		b.slogger.Error("Failed to build request streaming decision payload", "error", err)
		return false
	}

	resp, err := b.streamManager.Execute(context.Background(), req)
	if err != nil {
		b.slogger.Error("Failed to execute Python request streaming decision", "error", err)
		return false
	}

	decision, err := b.translator.ToGoNeedsMoreDecision(resp)
	if err != nil {
		b.slogger.Error("Failed to translate Python request streaming decision", "error", err)
		return false
	}
	return decision
}

func (b *bridge) OnRequestBodyChunk(ctx context.Context, reqCtx *policy.RequestStreamContext, chunk *policy.StreamBody, params map[string]interface{}) policy.StreamingRequestAction {
	if b.mode.RequestBodyMode != policy.BodyModeStream {
		return policy.ForwardRequestChunk{}
	}

	req, err := b.buildRequestChunkRequest(ctx, reqCtx, chunk, params)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to build request-chunk payload", "error", err)
		return b.streamingRequestErrorAction(err)
	}

	resp, err := b.execute(ctx, req, reqCtx.SharedContext)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to execute Python request-chunk policy", "error", err)
		return b.streamingRequestErrorAction(err)
	}

	action, err := b.translator.ToGoStreamingRequestAction(resp)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to translate Python request-chunk response", "error", err)
		return b.streamingRequestErrorAction(err)
	}
	return action
}

func (b *bridge) NeedsMoreResponseData(accumulated []byte) bool {
	if b.mode.ResponseBodyMode != policy.BodyModeStream {
		return false
	}

	req, err := b.buildNeedsMoreResponseDataRequest(context.Background(), accumulated)
	if err != nil {
		b.slogger.Error("Failed to build response streaming decision payload", "error", err)
		return false
	}

	resp, err := b.streamManager.Execute(context.Background(), req)
	if err != nil {
		b.slogger.Error("Failed to execute Python response streaming decision", "error", err)
		return false
	}

	decision, err := b.translator.ToGoNeedsMoreDecision(resp)
	if err != nil {
		b.slogger.Error("Failed to translate Python response streaming decision", "error", err)
		return false
	}
	return decision
}

func (b *bridge) OnResponseBodyChunk(ctx context.Context, respCtx *policy.ResponseStreamContext, chunk *policy.StreamBody, params map[string]interface{}) policy.StreamingResponseAction {
	if b.mode.ResponseBodyMode != policy.BodyModeStream {
		return policy.ForwardResponseChunk{}
	}

	req, err := b.buildResponseChunkRequest(ctx, respCtx, chunk, params)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to build response-chunk payload", "error", err)
		return b.streamingResponseErrorAction(err)
	}

	resp, err := b.execute(ctx, req, respCtx.SharedContext)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to execute Python response-chunk policy", "error", err)
		return b.streamingResponseErrorAction(err)
	}

	action, err := b.translator.ToGoStreamingResponseAction(resp)
	if err != nil {
		b.slogger.ErrorContext(ctx, "Failed to translate Python response-chunk response", "error", err)
		return b.streamingResponseErrorAction(err)
	}
	return action
}

func (b *bridge) execute(ctx context.Context, req *proto.StreamRequest, shared *policy.SharedContext) (*proto.StreamResponse, error) {
	resp, err := b.streamManager.Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.GetUpdatedMetadata() != nil {
		b.mergeMetadata(shared, resp.GetUpdatedMetadata())
	}
	return resp, nil
}

func (b *bridge) buildRequestHeadersRequest(
	ctx context.Context,
	reqCtx *policy.RequestHeaderContext,
	params map[string]interface{},
) (*proto.StreamRequest, error) {
	payload := &proto.RequestHeadersPayload{
		Context: &proto.RequestHeaderContext{
			Headers:   b.translator.ToProtoHeaders(reqCtx.Headers),
			Path:      reqCtx.Path,
			Method:    reqCtx.Method,
			Authority: reqCtx.Authority,
			Scheme:    reqCtx.Scheme,
			Vhost:     reqCtx.Vhost,
		},
	}
	return b.newStreamRequest(ctx, reqCtx.SharedContext, params, proto.Phase_PHASE_REQUEST_HEADERS, &proto.StreamRequest_RequestHeaders{
		RequestHeaders: payload,
	})
}

func (b *bridge) buildRequestBodyRequest(
	ctx context.Context,
	reqCtx *policy.RequestContext,
	params map[string]interface{},
) (*proto.StreamRequest, error) {
	payload := &proto.RequestBodyPayload{
		Context: &proto.RequestContext{
			Headers:   b.translator.ToProtoHeaders(reqCtx.Headers),
			Body:      b.translator.ToProtoBody(reqCtx.Body),
			Path:      reqCtx.Path,
			Method:    reqCtx.Method,
			Authority: reqCtx.Authority,
			Scheme:    reqCtx.Scheme,
			Vhost:     reqCtx.Vhost,
		},
	}
	return b.newStreamRequest(ctx, reqCtx.SharedContext, params, proto.Phase_PHASE_REQUEST_BODY, &proto.StreamRequest_RequestBody{
		RequestBody: payload,
	})
}

func (b *bridge) buildResponseHeadersRequest(
	ctx context.Context,
	respCtx *policy.ResponseHeaderContext,
	params map[string]interface{},
) (*proto.StreamRequest, error) {
	payload := &proto.ResponseHeadersPayload{
		Context: &proto.ResponseHeaderContext{
			RequestHeaders: b.translator.ToProtoHeaders(respCtx.RequestHeaders),
			RequestBody:    b.translator.ToProtoBody(respCtx.RequestBody),
			RequestPath:    respCtx.RequestPath,
			RequestMethod:  respCtx.RequestMethod,
			ResponseHeaders: b.translator.ToProtoHeaders(
				respCtx.ResponseHeaders,
			),
			ResponseStatus: int32(respCtx.ResponseStatus),
		},
	}
	return b.newStreamRequest(ctx, respCtx.SharedContext, params, proto.Phase_PHASE_RESPONSE_HEADERS, &proto.StreamRequest_ResponseHeaders{
		ResponseHeaders: payload,
	})
}

func (b *bridge) buildResponseBodyRequest(
	ctx context.Context,
	respCtx *policy.ResponseContext,
	params map[string]interface{},
) (*proto.StreamRequest, error) {
	payload := &proto.ResponseBodyPayload{
		Context: &proto.ResponseContext{
			RequestHeaders: b.translator.ToProtoHeaders(respCtx.RequestHeaders),
			RequestBody:    b.translator.ToProtoBody(respCtx.RequestBody),
			RequestPath:    respCtx.RequestPath,
			RequestMethod:  respCtx.RequestMethod,
			ResponseHeaders: b.translator.ToProtoHeaders(
				respCtx.ResponseHeaders,
			),
			ResponseBody:   b.translator.ToProtoBody(respCtx.ResponseBody),
			ResponseStatus: int32(respCtx.ResponseStatus),
		},
	}
	return b.newStreamRequest(ctx, respCtx.SharedContext, params, proto.Phase_PHASE_RESPONSE_BODY, &proto.StreamRequest_ResponseBody{
		ResponseBody: payload,
	})
}

func (b *bridge) buildNeedsMoreRequestDataRequest(
	ctx context.Context,
	accumulated []byte,
) (*proto.StreamRequest, error) {
	return b.newStreamRequest(ctx, nil, nil, proto.Phase_PHASE_NEEDS_MORE_REQUEST_DATA, &proto.StreamRequest_NeedsMoreRequestData{
		NeedsMoreRequestData: &proto.NeedsMoreRequestDataPayload{
			Accumulated: append([]byte(nil), accumulated...),
		},
	})
}

func (b *bridge) buildRequestChunkRequest(
	ctx context.Context,
	reqCtx *policy.RequestStreamContext,
	chunk *policy.StreamBody,
	params map[string]interface{},
) (*proto.StreamRequest, error) {
	payload := &proto.RequestChunkPayload{
		Context: &proto.RequestStreamContext{
			Headers:   b.translator.ToProtoHeaders(reqCtx.Headers),
			Path:      reqCtx.Path,
			Method:    reqCtx.Method,
			Authority: reqCtx.Authority,
			Scheme:    reqCtx.Scheme,
			Vhost:     reqCtx.Vhost,
		},
		Chunk: b.translator.ToProtoStreamBody(chunk),
	}
	return b.newStreamRequest(ctx, reqCtx.SharedContext, params, proto.Phase_PHASE_REQUEST_BODY_CHUNK, &proto.StreamRequest_RequestChunk{
		RequestChunk: payload,
	})
}

func (b *bridge) buildNeedsMoreResponseDataRequest(
	ctx context.Context,
	accumulated []byte,
) (*proto.StreamRequest, error) {
	return b.newStreamRequest(ctx, nil, nil, proto.Phase_PHASE_NEEDS_MORE_RESPONSE_DATA, &proto.StreamRequest_NeedsMoreResponseData{
		NeedsMoreResponseData: &proto.NeedsMoreResponseDataPayload{
			Accumulated: append([]byte(nil), accumulated...),
		},
	})
}

func (b *bridge) buildResponseChunkRequest(
	ctx context.Context,
	respCtx *policy.ResponseStreamContext,
	chunk *policy.StreamBody,
	params map[string]interface{},
) (*proto.StreamRequest, error) {
	payload := &proto.ResponseChunkPayload{
		Context: &proto.ResponseStreamContext{
			RequestHeaders: b.translator.ToProtoHeaders(respCtx.RequestHeaders),
			RequestBody:    b.translator.ToProtoBody(respCtx.RequestBody),
			RequestPath:    respCtx.RequestPath,
			RequestMethod:  respCtx.RequestMethod,
			ResponseHeaders: b.translator.ToProtoHeaders(
				respCtx.ResponseHeaders,
			),
			ResponseStatus: int32(respCtx.ResponseStatus),
		},
		Chunk: b.translator.ToProtoStreamBody(chunk),
	}
	return b.newStreamRequest(ctx, respCtx.SharedContext, params, proto.Phase_PHASE_RESPONSE_BODY_CHUNK, &proto.StreamRequest_ResponseChunk{
		ResponseChunk: payload,
	})
}

func (b *bridge) newStreamRequest(
	ctx context.Context,
	shared *policy.SharedContext,
	params map[string]interface{},
	phase proto.Phase,
	payload any,
) (*proto.StreamRequest, error) {
	protoParams, err := toProtoStruct(params)
	if err != nil {
		return nil, fmt.Errorf("convert params: %w", err)
	}

	sharedCtx, err := b.translator.ToProtoSharedContext(shared)
	if err != nil {
		return nil, fmt.Errorf("convert shared context: %w", err)
	}

	req := &proto.StreamRequest{
		RequestId:         uuid.NewString(),
		InstanceId:        b.instanceID,
		PolicyName:        b.policyName,
		PolicyVersion:     b.policyVersion,
		Params:            protoParams,
		SharedContext:     sharedCtx,
		ExecutionMetadata: b.buildExecutionMetadata(ctx, phase),
	}

	switch concrete := payload.(type) {
	case *proto.StreamRequest_RequestHeaders:
		req.Payload = concrete
	case *proto.StreamRequest_RequestBody:
		req.Payload = concrete
	case *proto.StreamRequest_ResponseHeaders:
		req.Payload = concrete
	case *proto.StreamRequest_ResponseBody:
		req.Payload = concrete
	case *proto.StreamRequest_NeedsMoreRequestData:
		req.Payload = concrete
	case *proto.StreamRequest_RequestChunk:
		req.Payload = concrete
	case *proto.StreamRequest_NeedsMoreResponseData:
		req.Payload = concrete
	case *proto.StreamRequest_ResponseChunk:
		req.Payload = concrete
	case *proto.StreamRequest_CancelExecution:
		req.Payload = concrete
	default:
		return nil, fmt.Errorf("unsupported stream request payload: %T", payload)
	}

	return req, nil
}

func (b *bridge) buildExecutionMetadata(ctx context.Context, phase proto.Phase) *proto.ExecutionMetadata {
	metadata := &proto.ExecutionMetadata{
		Phase:     phase,
		RouteName: b.metadata.RouteName,
	}

	if deadline, ok := ctx.Deadline(); ok {
		metadata.Deadline = timestamppb.New(deadline)
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		metadata.Trace = &proto.TraceMetadata{
			TraceId: spanCtx.TraceID().String(),
			SpanId:  spanCtx.SpanID().String(),
		}
	}

	return metadata
}

func (b *bridge) mergeMetadata(shared *policy.SharedContext, updated *structpb.Struct) {
	if shared == nil || updated == nil {
		return
	}
	if shared.Metadata == nil {
		shared.Metadata = make(map[string]interface{}, len(updated.Fields))
	}
	for key, value := range updated.Fields {
		shared.Metadata[key] = value.AsInterface()
	}
}

func (b *bridge) errorImmediateResponse() policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte("Internal policy error"),
	}
}

func (b *bridge) requestBodyErrorAction(err error) policy.RequestAction {
	return b.errorImmediateResponse()
}

func (b *bridge) responseBodyErrorAction(err error) policy.ResponseAction {
	return b.errorImmediateResponse()
}

func (b *bridge) requestHeaderErrorAction(err error) policy.RequestHeaderAction {
	return b.errorImmediateResponse()
}

func (b *bridge) responseHeaderErrorAction(err error) policy.ResponseHeaderAction {
	return b.errorImmediateResponse()
}

func (b *bridge) streamingRequestErrorAction(err error) policy.StreamingRequestAction {
	return policy.ForwardRequestChunk{}
}

func (b *bridge) streamingResponseErrorAction(err error) policy.StreamingResponseAction {
	return policy.ForwardResponseChunk{}
}

func (b *bridge) Close() error {
	b.closeOnce.Do(func() {
		if b.instanceID == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), getTimeout())
		defer cancel()

		resp, err := b.streamManager.DestroyPolicy(ctx, &proto.DestroyPolicyRequest{
			InstanceId: b.instanceID,
		})
		if err != nil {
			b.slogger.Warn("DestroyPolicy RPC failed", "instance_id", b.instanceID, "error", err)
			b.closeErr = fmt.Errorf("destroy Python policy instance: %w", err)
			return
		}
		if !resp.GetSuccess() {
			b.slogger.Warn("DestroyPolicy returned an executor error", "instance_id", b.instanceID, "error", resp.GetErrorMessage())
		}
	})
	return b.closeErr
}
