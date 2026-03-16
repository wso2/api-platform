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
	"sync"

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
	mode          policy.ProcessingMode
	metadata      policy.PolicyMetadata
	streamManager *StreamManager
	translator    *Translator
	slogger       *slog.Logger
	instanceID    string
	closeOnce     sync.Once
	closeErr      error
}

// Mode returns the policy's processing mode (declared statically for Python policies).
func (b *PythonBridge) Mode() policy.ProcessingMode {
	return b.mode
}

// OnRequest executes the policy during request phase by delegating to Python.
func (b *PythonBridge) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	req, err := b.buildRequest(ctx, "on_request", params)
	if err != nil {
		b.slogger.Error("Failed to build request", "error", err, "phase", "on_request")
		return b.errorResponse(err)
	}

	resp, err := b.streamManager.Execute(context.Background(), req)
	if err != nil {
		b.slogger.Error("Failed to execute Python policy", "error", err, "phase", "on_request")
		return b.errorResponse(err)
	}

	if resp.UpdatedMetadata != nil {
		b.mergeMetadata(ctx.SharedContext, resp.UpdatedMetadata)
	}

	return b.translator.ToGoRequestAction(resp)
}

// OnResponse executes the policy during response phase by delegating to Python.
func (b *PythonBridge) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	req, err := b.buildResponseRequest(ctx, params)
	if err != nil {
		b.slogger.Error("Failed to build request", "error", err, "phase", "on_response")
		return b.errorResponseAction(err)
	}

	resp, err := b.streamManager.Execute(context.Background(), req)
	if err != nil {
		b.slogger.Error("Failed to execute Python policy", "error", err, "phase", "on_response")
		return b.errorResponseAction(err)
	}

	if resp.UpdatedMetadata != nil {
		b.mergeMetadata(ctx.SharedContext, resp.UpdatedMetadata)
	}

	return b.translator.ToGoResponseAction(resp)
}

// buildRequest creates an ExecutionRequest for the request phase.
func (b *PythonBridge) buildRequest(ctx *policy.RequestContext, phase string, params map[string]interface{}) (*proto.ExecutionRequest, error) {
	reqID := uuid.New().String()

	headers := make(map[string]string)
	if ctx.Headers != nil {
		ctx.Headers.Iterate(func(name string, values []string) {
			headers[name] = strings.Join(values, ", ")
		})
	}

	reqCtx := &proto.RequestContext{
		Headers: headers,
		Path:    ctx.Path,
		Method:  ctx.Method,
		Scheme:  ctx.Scheme,
	}

	if ctx.Body != nil && ctx.Body.Present {
		reqCtx.Body = ctx.Body.Content
		reqCtx.BodyPresent = true
		reqCtx.EndOfStream = ctx.Body.EndOfStream
	}

	sharedCtx, err := b.buildSharedContext(ctx.SharedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to build shared context: %w", err)
	}

	policyMeta := b.buildPolicyMetadata()

	protoParams, err := toProtoStruct(params)
	if err != nil {
		return nil, fmt.Errorf("failed to convert params: %w", err)
	}

	return &proto.ExecutionRequest{
		RequestId:      reqID,
		PolicyName:     b.policyName,
		PolicyVersion:  b.policyVersion,
		Phase:          phase,
		Params:         protoParams,
		Context:        &proto.ExecutionRequest_RequestContext{RequestContext: reqCtx},
		SharedContext:  sharedCtx,
		PolicyMetadata: policyMeta,
		InstanceId:     b.instanceID,
	}, nil
}

// buildResponseRequest creates an ExecutionRequest for the response phase.
func (b *PythonBridge) buildResponseRequest(ctx *policy.ResponseContext, params map[string]interface{}) (*proto.ExecutionRequest, error) {
	reqID := uuid.New().String()

	requestHeaders := make(map[string]string)
	if ctx.RequestHeaders != nil {
		ctx.RequestHeaders.Iterate(func(name string, values []string) {
			requestHeaders[name] = strings.Join(values, ", ")
		})
	}

	responseHeaders := make(map[string]string)
	if ctx.ResponseHeaders != nil {
		ctx.ResponseHeaders.Iterate(func(name string, values []string) {
			responseHeaders[name] = strings.Join(values, ", ")
		})
	}

	respCtx := &proto.ResponseContext{
		RequestHeaders:  requestHeaders,
		RequestPath:     ctx.RequestPath,
		RequestMethod:   ctx.RequestMethod,
		ResponseHeaders: responseHeaders,
		ResponseStatus:  int32(ctx.ResponseStatus),
	}

	if ctx.RequestBody != nil && ctx.RequestBody.Present {
		respCtx.RequestBody = ctx.RequestBody.Content
	}

	if ctx.ResponseBody != nil && ctx.ResponseBody.Present {
		respCtx.ResponseBody = ctx.ResponseBody.Content
		respCtx.ResponseBodyPresent = true
	}

	sharedCtx, err := b.buildSharedContext(ctx.SharedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to build shared context: %w", err)
	}

	policyMeta := b.buildPolicyMetadata()

	protoParams, err := toProtoStruct(params)
	if err != nil {
		return nil, fmt.Errorf("failed to convert params: %w", err)
	}

	return &proto.ExecutionRequest{
		RequestId:      reqID,
		PolicyName:     b.policyName,
		PolicyVersion:  b.policyVersion,
		Phase:          "on_response",
		Params:         protoParams,
		Context:        &proto.ExecutionRequest_ResponseContext{ResponseContext: respCtx},
		SharedContext:  sharedCtx,
		PolicyMetadata: policyMeta,
		InstanceId:     b.instanceID,
	}, nil
}

// buildSharedContext converts the Go SharedContext to proto.
func (b *PythonBridge) buildSharedContext(shared *policy.SharedContext) (*proto.SharedContext, error) {
	if shared == nil {
		return &proto.SharedContext{}, nil
	}

	metadataStruct, err := toProtoStruct(shared.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata: %w", err)
	}

	return &proto.SharedContext{
		ProjectId:     shared.ProjectID,
		RequestId:     shared.RequestID,
		Metadata:      metadataStruct,
		ApiId:         shared.APIId,
		ApiName:       shared.APIName,
		ApiVersion:    shared.APIVersion,
		ApiKind:       shared.APIKind,
		ApiContext:    shared.APIContext,
		OperationPath: shared.OperationPath,
		AuthContext:   authContextToMap(shared.AuthContext),
	}, nil
}

// authContextToMap flattens the structured AuthContext into a map[string]string
// for the proto wire format consumed by Python policies.
func authContextToMap(ac *policy.AuthContext) map[string]string {
	if ac == nil {
		return nil
	}
	m := map[string]string{
		"subject":   ac.Subject,
		"issuer":    ac.Issuer,
		"auth_type": ac.AuthType,
	}
	if ac.Authenticated {
		m["authenticated"] = "true"
	}
	if ac.Authorized {
		m["authorized"] = "true"
	}
	if ac.CredentialID != "" {
		m["credential_id"] = ac.CredentialID
	}
	// Flatten properties
	for k, v := range ac.Properties {
		m[k] = v
	}
	return m
}

// buildPolicyMetadata converts the Go PolicyMetadata to proto.
func (b *PythonBridge) buildPolicyMetadata() *proto.PolicyMetadata {
	return &proto.PolicyMetadata{
		RouteName:  b.metadata.RouteName,
		ApiId:      b.metadata.APIId,
		ApiName:    b.metadata.APIName,
		ApiVersion: b.metadata.APIVersion,
		AttachedTo: string(b.metadata.AttachedTo),
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
// Safe to call concurrently and multiple times.
// Go side is not implemented yet.
func (b *PythonBridge) Close() error {
	b.closeOnce.Do(func() {
		if b.instanceID == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), getTimeout())
		defer cancel()

		req := &proto.DestroyPolicyRequest{
			InstanceId: b.instanceID,
		}

		resp, err := b.streamManager.DestroyPolicy(ctx, req)
		if err != nil {
			b.slogger.Warn("DestroyPolicy RPC failed", "error", err, "instance_id", b.instanceID)
			b.closeErr = fmt.Errorf("DestroyPolicy failed: %w", err)
			return
		}
		if !resp.Success {
			b.slogger.Warn("DestroyPolicy returned error", "error", resp.ErrorMessage, "instance_id", b.instanceID)
		}

		b.slogger.Info("Python policy instance destroyed", "instance_id", b.instanceID)
	})
	return b.closeErr
}
