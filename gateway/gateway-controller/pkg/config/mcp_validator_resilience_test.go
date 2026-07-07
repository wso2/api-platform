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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

func mcpWithResilience(r *api.Resilience) api.MCPProxyConfiguration {
	ctx := "/everything"
	specVersion := constants.SPEC_VERSION_2025_JUNE
	return api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "everything"},
		Spec: api.MCPProxyConfigData{
			DisplayName: "Everything",
			Version:     "v1.0",
			Context:     &ctx,
			SpecVersion: &specVersion,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:3001")},
			Resilience:  r,
		},
	}
}

func TestValidateMCP_Resilience(t *testing.T) {
	validator := NewMCPValidator()

	t.Run("valid timeout and idleTimeout", func(t *testing.T) {
		errs := validator.Validate(mcpWithResilience(&api.Resilience{
			Timeout:     stringPtr("30s"),
			IdleTimeout: stringPtr("0s"),
		}))
		assert.Empty(t, errs)
	})

	t.Run("nil resilience is fine (defaults applied downstream)", func(t *testing.T) {
		errs := validator.Validate(mcpWithResilience(nil))
		assert.Empty(t, errs)
	})

	t.Run("0s is accepted (explicit disable)", func(t *testing.T) {
		errs := validator.Validate(mcpWithResilience(&api.Resilience{Timeout: stringPtr("0s")}))
		assert.Empty(t, errs)
	})

	t.Run("malformed timeout is rejected", func(t *testing.T) {
		errs := validator.Validate(mcpWithResilience(&api.Resilience{Timeout: stringPtr("30")}))
		assertHasFieldError(t, errs, "spec.resilience.timeout")
	})

	t.Run("compound timeout is rejected (must match CRD pattern)", func(t *testing.T) {
		errs := validator.Validate(mcpWithResilience(&api.Resilience{Timeout: stringPtr("1h30m")}))
		assertHasFieldError(t, errs, "spec.resilience.timeout")
	})

	t.Run("negative idleTimeout is rejected", func(t *testing.T) {
		errs := validator.Validate(mcpWithResilience(&api.Resilience{IdleTimeout: stringPtr("-5s")}))
		assertHasFieldError(t, errs, "spec.resilience.idleTimeout")
	})
}
