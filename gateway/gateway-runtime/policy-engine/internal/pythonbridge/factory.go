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
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

type policyCapabilities struct {
	requestHeaders    bool
	requestBody       bool
	responseHeaders   bool
	responseBody      bool
	streamingRequest  bool
	streamingResponse bool
}

// BridgeFactory creates Python bridge instances and validates the executor contract.
type BridgeFactory struct {
	StreamManager *StreamManager
	PolicyName    string
	PolicyVersion string
}

// GetPolicy creates a Python-backed policy instance.
func (f *BridgeFactory) GetPolicy(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
	slogger := slog.With(
		"component", "pythonbridge",
		"policy", f.PolicyName,
		"version", f.PolicyVersion,
		"route", metadata.RouteName,
	)

	protoParams, err := toProtoStruct(params)
	if err != nil {
		return nil, fmt.Errorf("convert init params for %s:%s: %w", f.PolicyName, f.PolicyVersion, err)
	}

	req := &proto.InitPolicyRequest{
		PolicyName:    f.PolicyName,
		PolicyVersion: f.PolicyVersion,
		PolicyMetadata: &proto.PolicyMetadata{
			RouteName:  metadata.RouteName,
			ApiId:      metadata.APIId,
			ApiName:    metadata.APIName,
			ApiVersion: metadata.APIVersion,
			AttachedTo: string(metadata.AttachedTo),
		},
		Params: protoParams,
	}

	ctx, cancel := context.WithTimeout(context.Background(), getTimeout())
	defer cancel()

	resp, err := f.StreamManager.InitPolicy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("InitPolicy RPC failed for %s:%s: %w", f.PolicyName, f.PolicyVersion, err)
	}
	if !resp.GetSuccess() {
		return nil, fmt.Errorf("InitPolicy failed for %s:%s: %s", f.PolicyName, f.PolicyVersion, resp.GetErrorMessage())
	}
	if resp.GetProcessingMode() == nil {
		return nil, fmt.Errorf("InitPolicyResponse missing processing_mode for %s:%s", f.PolicyName, f.PolicyVersion)
	}
	if resp.GetCapabilities() == nil {
		return nil, fmt.Errorf("InitPolicyResponse missing capabilities for %s:%s", f.PolicyName, f.PolicyVersion)
	}

	mode, err := processingModeFromProto(resp.GetProcessingMode())
	if err != nil {
		return nil, fmt.Errorf("invalid processing_mode from InitPolicy for %s:%s: %w", f.PolicyName, f.PolicyVersion, err)
	}

	capabilities := capabilitiesFromProto(resp.GetCapabilities())
	if err := validateModeAndCapabilities(mode, capabilities); err != nil {
		return nil, fmt.Errorf("invalid mode/capabilities from InitPolicy for %s:%s: %w", f.PolicyName, f.PolicyVersion, err)
	}

	policyImpl := &bridge{
		policyName:    f.PolicyName,
		policyVersion: f.PolicyVersion,
		mode:          mode,
		metadata:      metadata,
		streamManager: f.StreamManager,
		translator:    NewTranslator(),
		slogger:       slogger,
		instanceID:    resp.GetInstanceId(),
	}

	slogger.Info("Python policy instance created", "instance_id", resp.GetInstanceId())
	return policyImpl, nil
}

func processingModeFromProto(pm *proto.ProcessingMode) (policy.ProcessingMode, error) {
	if pm == nil {
		return policy.ProcessingMode{}, fmt.Errorf("processing_mode is nil")
	}

	requestHeaderMode, err := headerProcessingModeFromProto(pm.GetRequestHeaderMode())
	if err != nil {
		return policy.ProcessingMode{}, fmt.Errorf("request_header_mode: %w", err)
	}
	requestBodyMode, err := bodyProcessingModeFromProto(pm.GetRequestBodyMode())
	if err != nil {
		return policy.ProcessingMode{}, fmt.Errorf("request_body_mode: %w", err)
	}
	responseHeaderMode, err := headerProcessingModeFromProto(pm.GetResponseHeaderMode())
	if err != nil {
		return policy.ProcessingMode{}, fmt.Errorf("response_header_mode: %w", err)
	}
	responseBodyMode, err := bodyProcessingModeFromProto(pm.GetResponseBodyMode())
	if err != nil {
		return policy.ProcessingMode{}, fmt.Errorf("response_body_mode: %w", err)
	}

	return policy.ProcessingMode{
		RequestHeaderMode:  requestHeaderMode,
		RequestBodyMode:    requestBodyMode,
		ResponseHeaderMode: responseHeaderMode,
		ResponseBodyMode:   responseBodyMode,
	}, nil
}

func headerProcessingModeFromProto(mode proto.HeaderProcessingMode) (policy.HeaderProcessingMode, error) {
	switch mode {
	case proto.HeaderProcessingMode_HEADER_PROCESSING_MODE_SKIP:
		return policy.HeaderModeSkip, nil
	case proto.HeaderProcessingMode_HEADER_PROCESSING_MODE_PROCESS:
		return policy.HeaderModeProcess, nil
	default:
		return "", fmt.Errorf("unsupported value %s", mode.String())
	}
}

func bodyProcessingModeFromProto(mode proto.BodyProcessingMode) (policy.BodyProcessingMode, error) {
	switch mode {
	case proto.BodyProcessingMode_BODY_PROCESSING_MODE_SKIP:
		return policy.BodyModeSkip, nil
	case proto.BodyProcessingMode_BODY_PROCESSING_MODE_BUFFER:
		return policy.BodyModeBuffer, nil
	case proto.BodyProcessingMode_BODY_PROCESSING_MODE_STREAM:
		return policy.BodyModeStream, nil
	default:
		return "", fmt.Errorf("unsupported value %s", mode.String())
	}
}

func capabilitiesFromProto(caps *proto.PolicyCapabilities) policyCapabilities {
	if caps == nil {
		return policyCapabilities{}
	}
	return policyCapabilities{
		requestHeaders:    caps.GetRequestHeaders(),
		requestBody:       caps.GetRequestBody(),
		responseHeaders:   caps.GetResponseHeaders(),
		responseBody:      caps.GetResponseBody(),
		streamingRequest:  caps.GetStreamingRequest(),
		streamingResponse: caps.GetStreamingResponse(),
	}
}

func validateModeAndCapabilities(mode policy.ProcessingMode, caps policyCapabilities) error {
	var issues []string

	if (mode.RequestHeaderMode == policy.HeaderModeProcess) != caps.requestHeaders {
		issues = append(issues, "request_header_mode and request_headers capability disagree")
	}
	if (mode.ResponseHeaderMode == policy.HeaderModeProcess) != caps.responseHeaders {
		issues = append(issues, "response_header_mode and response_headers capability disagree")
	}

	switch mode.RequestBodyMode {
	case policy.BodyModeSkip:
		if caps.requestBody || caps.streamingRequest {
			issues = append(issues, "request_body_mode=SKIP requires request-body capabilities to be absent")
		}
	case policy.BodyModeBuffer:
		if !caps.requestBody || caps.streamingRequest {
			issues = append(issues, "request_body_mode=BUFFER requires buffered request_body capability only")
		}
	case policy.BodyModeStream:
		if !caps.requestBody || !caps.streamingRequest {
			issues = append(issues, "request_body_mode=STREAM requires request_body and streaming_request capabilities")
		}
	default:
		issues = append(issues, fmt.Sprintf("unsupported request_body_mode: %q", mode.RequestBodyMode))
	}

	switch mode.ResponseBodyMode {
	case policy.BodyModeSkip:
		if caps.responseBody || caps.streamingResponse {
			issues = append(issues, "response_body_mode=SKIP requires response-body capabilities to be absent")
		}
	case policy.BodyModeBuffer:
		if !caps.responseBody || caps.streamingResponse {
			issues = append(issues, "response_body_mode=BUFFER requires buffered response_body capability only")
		}
	case policy.BodyModeStream:
		if !caps.responseBody || !caps.streamingResponse {
			issues = append(issues, "response_body_mode=STREAM requires response_body and streaming_response capabilities")
		}
	default:
		issues = append(issues, fmt.Sprintf("unsupported response_body_mode: %q", mode.ResponseBodyMode))
	}

	if len(issues) > 0 {
		return fmt.Errorf("%s", joinIssues(issues))
	}
	return nil
}

func joinIssues(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	result := issues[0]
	for _, issue := range issues[1:] {
		result += "; " + issue
	}
	return result
}

// PythonHealthAdapter implements admin.PythonHealthChecker using the StreamManager.
type PythonHealthAdapter struct {
	sm *StreamManager
}

// NewPythonHealthAdapter creates a PythonHealthAdapter from the given StreamManager.
func NewPythonHealthAdapter(sm *StreamManager) *PythonHealthAdapter {
	return &PythonHealthAdapter{sm: sm}
}

// IsPythonHealthy calls the Python executor's HealthCheck RPC.
func (a *PythonHealthAdapter) IsPythonHealthy() (bool, int32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := a.sm.HealthCheck(ctx)
	if err != nil {
		return false, 0, err
	}
	return resp.GetReady(), resp.GetLoadedPolicies(), nil
}
