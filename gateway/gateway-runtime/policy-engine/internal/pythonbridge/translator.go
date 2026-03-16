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
	"log/slog"

	"google.golang.org/protobuf/types/known/structpb"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
)

// Translator converts between proto messages and Go SDK types.
type Translator struct{}

// NewTranslator creates a new Translator.
func NewTranslator() *Translator {
	return &Translator{}
}

// ToGoRequestAction converts a proto ExecutionResponse to a Go RequestAction.
func (t *Translator) ToGoRequestAction(resp *proto.ExecutionResponse) policy.RequestAction {
	// Check for error result
	if errResult := resp.GetError(); errResult != nil {
		slog.Error("Python policy returned error",
			"error", errResult.Message,
			"error_type", errResult.ErrorType,
			"policy", errResult.PolicyName,
		)
		return policy.ImmediateResponse{
			StatusCode: 500,
			Headers:    map[string]string{"Content-Type": "text/plain"},
			Body:       []byte(errResult.Message),
		}
	}

	// Get request result
	reqResult := resp.GetRequestResult()
	if reqResult == nil {
		return nil // Pass-through
	}

	// Handle continue request
	if continueReq := reqResult.GetContinueRequest(); continueReq != nil {
		return t.toUpstreamRequestModifications(continueReq)
	}

	// Handle immediate response
	if immediateResp := reqResult.GetImmediateResponse(); immediateResp != nil {
		return t.toImmediateResponse(immediateResp)
	}

	return nil
}

// ToGoResponseAction converts a proto ExecutionResponse to a Go ResponseAction.
func (t *Translator) ToGoResponseAction(resp *proto.ExecutionResponse) policy.ResponseAction {
	// Check for error result
	if errResult := resp.GetError(); errResult != nil {
		slog.Error("Python policy returned error in response phase",
			"error", errResult.Message,
			"error_type", errResult.ErrorType,
			"policy", errResult.PolicyName,
		)
		statusCode := 500
		return policy.UpstreamResponseModifications{
			StatusCode: &statusCode,
		}
	}

	// Get response result
	respResult := resp.GetResponseResult()
	if respResult == nil {
		return nil // Pass-through
	}

	// Handle continue response
	if continueResp := respResult.GetContinueResponse(); continueResp != nil {
		return t.toUpstreamResponseModifications(continueResp)
	}

	return nil
}

// toUpstreamRequestModifications converts proto to Go type.
func (t *Translator) toUpstreamRequestModifications(mod *proto.UpstreamRequestModifications) policy.UpstreamRequestModifications {
	result := policy.UpstreamRequestModifications{
		SetHeaders:        mod.SetHeaders,
		RemoveHeaders:     mod.RemoveHeaders,
		AppendHeaders:     t.stringListMapToSliceMap(mod.AppendHeaders),
		AnalyticsMetadata: t.structToMap(mod.AnalyticsMetadata),
	}

	if mod.BodyPresent {
		result.Body = mod.Body
	}

	if mod.PathPresent {
		result.Path = &mod.Path
	}

	if mod.MethodPresent {
		result.Method = &mod.Method
	}

	return result
}

// toImmediateResponse converts proto to Go type.
func (t *Translator) toImmediateResponse(resp *proto.ImmediateResponseAction) policy.ImmediateResponse {
	return policy.ImmediateResponse{
		StatusCode:        int(resp.StatusCode),
		Headers:           resp.Headers,
		Body:              resp.Body,
		AnalyticsMetadata: t.structToMap(resp.AnalyticsMetadata),
	}
}

// toUpstreamResponseModifications converts proto to Go type.
func (t *Translator) toUpstreamResponseModifications(mod *proto.UpstreamResponseModifications) policy.UpstreamResponseModifications {
	result := policy.UpstreamResponseModifications{
		SetHeaders:        mod.SetHeaders,
		RemoveHeaders:     mod.RemoveHeaders,
		AppendHeaders:     t.stringListMapToSliceMap(mod.AppendHeaders),
		AnalyticsMetadata: t.structToMap(mod.AnalyticsMetadata),
	}

	if mod.BodyPresent {
		result.Body = mod.Body
	}

	if mod.StatusCodePresent {
		statusCode := int(mod.StatusCode)
		result.StatusCode = &statusCode
	}

	return result
}

// stringListMapToSliceMap converts proto StringList map to [][]string map.
func (t *Translator) stringListMapToSliceMap(m map[string]*proto.StringList) map[string][]string {
	result := make(map[string][]string, len(m))
	for k, v := range m {
		if v != nil {
			result[k] = v.Values
		}
	}
	return result
}

// structToMap converts a protobuf Struct to a Go map.
func (t *Translator) structToMap(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

// toProtoStruct converts a Go map to a protobuf Struct.
func toProtoStruct(m map[string]interface{}) (*structpb.Struct, error) {
	if m == nil {
		return nil, nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, fmt.Errorf("failed to convert map to proto struct: %w", err)
	}
	return s, nil
}
