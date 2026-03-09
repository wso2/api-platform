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
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
)

// PythonBridge implements the policy.Policy interface.
// It delegates execution to the Python Executor via the StreamManager.
//
// Each PythonBridge instance corresponds to one policy-per-route
// (same as Go policies get one instance per route).
type PythonBridge struct {
	policyName    string
	policyVersion string
	mode          policy.ProcessingMode // Static, from policy-definition.yaml
	metadata      policy.PolicyMetadata
	params        map[string]interface{} // Merged system + user params
	streamManager *StreamManager         // Shared singleton
	translator    *Translator
	slogger       *slog.Logger
	instanceID    string // NEW — Python-side instance identifier
	closed        bool   // NEW — prevents double-close
}

// Mode returns the policy's processing mode (declared statically for Python policies).
func (b *PythonBridge) Mode() policy.ProcessingMode {
	return b.mode
}

// OnRequest executes the policy during request phase by delegating to Python.
func (b *PythonBridge) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Build proto ExecutionRequest
	req := b.buildRequest(ctx, params, "on_request")

	// Execute via stream manager
	resp, err := b.streamManager.Execute(context.Background(), req)
	if err != nil {
		b.slogger.Error("Failed to execute Python policy", "error", err, "phase", "on_request")
		return b.errorResponse(err)
	}

	// Merge updated metadata back into shared context
	if resp.UpdatedMetadata != nil {
		b.mergeMetadata(ctx.SharedContext, resp.UpdatedMetadata)
	}

	// Translate response back to Go action
	return b.translator.ToGoRequestAction(resp)
}

// OnResponse executes the policy during response phase by delegating to Python.
func (b *PythonBridge) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Build proto ExecutionRequest
	req := b.buildResponseRequest(ctx, params)

	// Execute via stream manager
	resp, err := b.streamManager.Execute(context.Background(), req)
	if err != nil {
		b.slogger.Error("Failed to execute Python policy", "error", err, "phase", "on_response")
		return b.errorResponseAction(err)
	}

	// Merge updated metadata back into shared context
	if resp.UpdatedMetadata != nil {
		b.mergeMetadata(ctx.SharedContext, resp.UpdatedMetadata)
	}

	// Translate response back to Go action
	return b.translator.ToGoResponseAction(resp)
}

// buildRequest creates an ExecutionRequest for the request phase.
func (b *PythonBridge) buildRequest(ctx *policy.RequestContext, params map[string]interface{}, phase string) *proto.ExecutionRequest {
	reqID := uuid.New().String()

	// Convert headers - join multiple values with comma
	headers := make(map[string]string)
	if ctx.Headers != nil {
		ctx.Headers.Iterate(func(name string, values []string) {
			headers[name] = strings.Join(values, ", ")
		})
	}

	// Build request context
	reqCtx := &proto.RequestContext{
		Headers: headers,
		Path:    ctx.Path,
		Method:  ctx.Method,
		Scheme:  ctx.Scheme,
	}

	// Add body if present
	if ctx.Body != nil && ctx.Body.Present {
		reqCtx.Body = ctx.Body.Content
		reqCtx.BodyPresent = true
		reqCtx.EndOfStream = ctx.Body.EndOfStream
	}

	// Build shared context
	sharedCtx := b.buildSharedContext(ctx.SharedContext)

	// Build policy metadata
	policyMeta := b.buildPolicyMetadata()

	return &proto.ExecutionRequest{
		RequestId:      reqID,
		PolicyName:     b.policyName,
		PolicyVersion:  b.policyVersion,
		Phase:          phase,
		Params:         toProtoStruct(params),
		Context:        &proto.ExecutionRequest_RequestContext{RequestContext: reqCtx},
		SharedContext:  sharedCtx,
		PolicyMetadata: policyMeta,
		InstanceId:     b.instanceID,
	}
}

// buildResponseRequest creates an ExecutionRequest for the response phase.
func (b *PythonBridge) buildResponseRequest(ctx *policy.ResponseContext, params map[string]interface{}) *proto.ExecutionRequest {
	reqID := uuid.New().String()

	// Convert request headers - join multiple values with comma
	requestHeaders := make(map[string]string)
	if ctx.RequestHeaders != nil {
		ctx.RequestHeaders.Iterate(func(name string, values []string) {
			requestHeaders[name] = strings.Join(values, ", ")
		})
	}

	// Convert response headers - join multiple values with comma
	responseHeaders := make(map[string]string)
	if ctx.ResponseHeaders != nil {
		ctx.ResponseHeaders.Iterate(func(name string, values []string) {
			responseHeaders[name] = strings.Join(values, ", ")
		})
	}

	// Build response context
	respCtx := &proto.ResponseContext{
		RequestHeaders:  requestHeaders,
		RequestPath:     ctx.RequestPath,
		RequestMethod:   ctx.RequestMethod,
		ResponseHeaders: responseHeaders,
		ResponseStatus:  int32(ctx.ResponseStatus),
	}

	// Add request body if present
	if ctx.RequestBody != nil && ctx.RequestBody.Present {
		respCtx.RequestBody = ctx.RequestBody.Content
	}

	// Add response body if present
	if ctx.ResponseBody != nil && ctx.ResponseBody.Present {
		respCtx.ResponseBody = ctx.ResponseBody.Content
		respCtx.ResponseBodyPresent = true
	}

	// Build shared context
	sharedCtx := b.buildSharedContext(ctx.SharedContext)

	// Build policy metadata
	policyMeta := b.buildPolicyMetadata()

	return &proto.ExecutionRequest{
		RequestId:      reqID,
		PolicyName:     b.policyName,
		PolicyVersion:  b.policyVersion,
		Phase:          "on_response",
		Params:         toProtoStruct(params),
		Context:        &proto.ExecutionRequest_ResponseContext{ResponseContext: respCtx},
		SharedContext:  sharedCtx,
		PolicyMetadata: policyMeta,
		InstanceId:     b.instanceID, // NEW
	}
}

// buildSharedContext converts the Go SharedContext to proto.
func (b *PythonBridge) buildSharedContext(shared *policy.SharedContext) *proto.SharedContext {
	if shared == nil {
		return &proto.SharedContext{}
	}

	return &proto.SharedContext{
		ProjectId:     shared.ProjectID,
		RequestId:     shared.RequestID,
		Metadata:      toProtoStruct(shared.Metadata),
		ApiId:         shared.APIId,
		ApiName:       shared.APIName,
		ApiVersion:    shared.APIVersion,
		ApiKind:       shared.APIKind,
		ApiContext:    shared.APIContext,
		OperationPath: shared.OperationPath,
		AuthContext:   shared.AuthContext,
	}
}

// buildPolicyMetadata converts the Go PolicyMetadata to proto.
func (b *PythonBridge) buildPolicyMetadata() *proto.PolicyMetadata {
	return &proto.PolicyMetadata{
		RouteName:   b.metadata.RouteName,
		ApiId:       b.metadata.APIId,
		ApiName:     b.metadata.APIName,
		ApiVersion:  b.metadata.APIVersion,
		AttachedTo:  string(b.metadata.AttachedTo),
	}
}

// mergeMetadata merges updated metadata from Python back into the shared context.
func (b *PythonBridge) mergeMetadata(shared *policy.SharedContext, updated *structpb.Struct) {
	if shared == nil || updated == nil {
		return
	}

	for key, value := range updated.Fields {
		if shared.Metadata == nil {
			shared.Metadata = make(map[string]interface{})
		}
		shared.Metadata[key] = value.AsInterface()
	}
}

// errorResponse returns an ImmediateResponse for errors.
func (b *PythonBridge) errorResponse(err error) policy.RequestAction {
	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte(fmt.Sprintf("Policy execution error: %v", err)),
	}
}

// errorResponseAction returns an error response action.
func (b *PythonBridge) errorResponseAction(err error) policy.ResponseAction {
	// For response phase errors, we return modifications that indicate failure
	statusCode := 500
	return policy.UpstreamResponseModifications{
		StatusCode: &statusCode,
		Body:       []byte(fmt.Sprintf("Policy execution error: %v", err)),
	}
}

// Close destroys the Python policy instance via DestroyPolicy RPC.
// This method will be called by the Kernel when a route is removed or replaced
// (once discussion #734 lands). It is safe to call multiple times.
func (b *PythonBridge) Close() error {
	if b.closed {
		return nil
	}
	b.closed = true

	if b.instanceID == "" {
		return nil // Init was never called or failed
	}

	ctx, cancel := context.WithTimeout(context.Background(), getTimeout())
	defer cancel()

	req := &proto.DestroyPolicyRequest{
		InstanceId: b.instanceID,
	}

	resp, err := b.streamManager.DestroyPolicy(ctx, req)
	if err != nil {
		b.slogger.Warn("DestroyPolicy RPC failed", "error", err, "instance_id", b.instanceID)
		return fmt.Errorf("DestroyPolicy failed: %w", err)
	}
	if !resp.Success {
		b.slogger.Warn("DestroyPolicy returned error", "error", resp.ErrorMessage, "instance_id", b.instanceID)
	}

	b.slogger.Info("Python policy instance destroyed", "instance_id", b.instanceID)
	return nil
}
