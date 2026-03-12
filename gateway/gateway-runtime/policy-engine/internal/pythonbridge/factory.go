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
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// BridgeFactory creates PythonBridge instances. It is registered as the PolicyFactory
// for Python policies in the generated plugin_registry.go.
//
// BridgeFactory holds:
//   - streamManager: shared StreamManager singleton
//   - mode: ProcessingMode parsed from policy-definition.yaml at build time
type BridgeFactory struct {
	StreamManager *StreamManager
	Mode          policy.ProcessingMode
	PolicyName    string
	PolicyVersion string
}

// GetPolicy creates a PythonBridge instance - conforms to policy.PolicyFactory signature.
func (f *BridgeFactory) GetPolicy(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
	slogger := slog.With(
		"component", "pythonbridge",
		"policy", f.PolicyName,
		"version", f.PolicyVersion,
		"route", metadata.RouteName,
	)

	// Build InitPolicy request
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
		Params: toProtoStruct(params),
	}

	// Call InitPolicy RPC
	ctx, cancel := context.WithTimeout(context.Background(), getTimeout())
	defer cancel()

	resp, err := f.StreamManager.InitPolicy(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("InitPolicy RPC failed for %s:%s: %w", f.PolicyName, f.PolicyVersion, err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("InitPolicy failed for %s:%s: %s", f.PolicyName, f.PolicyVersion, resp.ErrorMessage)
	}

	slogger.Info("Python policy instance created", "instance_id", resp.InstanceId)

	return &PythonBridge{
		policyName:    f.PolicyName,
		policyVersion: f.PolicyVersion,
		mode:          f.Mode,
		metadata:      metadata,
		streamManager: f.StreamManager,
		translator:    NewTranslator(),
		slogger:       slogger,
		instanceID:    resp.InstanceId,
	}, nil
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
	return resp.Ready, resp.LoadedPolicies, nil
}
