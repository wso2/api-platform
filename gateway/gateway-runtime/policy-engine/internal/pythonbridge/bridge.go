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
	"context"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"log/slog"
	"sync"
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
