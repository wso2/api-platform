/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package upstreamref centralizes resolution of per-op and API-level upstream
// references against the spec.upstreamDefinitions block. Both the xDS translator
// and the RDC transformer consume the same definitions and must agree on lookup
// and timeout-parsing semantics; this package exists so they share one source of
// truth.
package upstreamref

import (
	"fmt"
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// FindByName returns the UpstreamDefinition whose Name matches ref (after
// trimming whitespace). Returns an error if ref is empty, defs is nil/empty, or
// no matching definition exists.
func FindByName(ref string, defs *[]api.UpstreamDefinition) (*api.UpstreamDefinition, error) {
	refName := strings.TrimSpace(ref)
	if refName == "" {
		return nil, fmt.Errorf("upstream ref is empty")
	}
	if defs == nil || len(*defs) == 0 {
		return nil, fmt.Errorf("upstream definition '%s' referenced but no definitions provided", refName)
	}
	for i, def := range *defs {
		if strings.TrimSpace(def.Name) == refName {
			return &(*defs)[i], nil
		}
	}
	return nil, fmt.Errorf("upstream definition '%s' not found", refName)
}

// ParseConnectTimeout parses an UpstreamTimeout.Connect string. Empty/nil input
// returns (nil, nil). A parse failure or a non-positive duration returns an
// error so xDS and RDC paths fail consistently rather than silently dropping.
func ParseConnectTimeout(timeoutStr *string) (*time.Duration, error) {
	if timeoutStr == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*timeoutStr)
	if trimmed == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %w", err)
	}
	if d <= 0 {
		return nil, fmt.Errorf("timeout must be positive, got: %v", d)
	}
	return &d, nil
}

// HasContent returns true if the API-level upstream has non-empty configuration.
func HasContent(up *api.Upstream) bool {
	if up == nil {
		return false
	}
	return (up.Url != nil && strings.TrimSpace(*up.Url) != "") ||
		(up.Ref != nil && strings.TrimSpace(*up.Ref) != "")
}

// SandboxActive returns true if the sandbox environment is active for the API.
// It is active if the API-level sandbox has content OR if any operation-level override has a sandbox ref.
func SandboxActive(sandbox *api.Upstream, ops []api.Operation) bool {
	if HasContent(sandbox) {
		return true
	}
	for _, op := range ops {
		if op.Upstream != nil && op.Upstream.Sandbox != nil {
			if strings.TrimSpace(op.Upstream.Sandbox.Ref) != "" {
				return true
			}
		}
	}
	return false
}
