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
)

func validProviderWithResilience(r *api.Resilience) api.LLMProviderConfiguration {
	return api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "openai"},
		Spec: api.LLMProviderConfigData{
			DisplayName:   "my-provider",
			Version:       "v1.0",
			Template:      "openai",
			Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.openai.com")},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			Resilience:    r,
		},
	}
}

func validProxyWithResilience(r *api.Resilience) api.LLMProxyConfiguration {
	return api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "openai-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "my-proxy",
			Version:     "v1.0",
			Provider:    api.LLMProxyProvider{Id: "openai"},
			Resilience:  r,
		},
	}
}

func TestValidateLLMProvider_Resilience(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("valid timeout and idleTimeout", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(&api.Resilience{
			Timeout:     stringPtr("30s"),
			IdleTimeout: stringPtr("0s"),
		}))
		assert.Empty(t, errs)
	})

	t.Run("nil resilience is fine", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(nil))
		assert.Empty(t, errs)
	})

	t.Run("malformed timeout is rejected", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(&api.Resilience{Timeout: stringPtr("30")}))
		assertHasFieldError(t, errs, "spec.resilience.timeout")
	})

	t.Run("compound timeout is rejected (must match CRD pattern)", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(&api.Resilience{Timeout: stringPtr("1h30m")}))
		assertHasFieldError(t, errs, "spec.resilience.timeout")
	})

	t.Run("0s is accepted (disables)", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(&api.Resilience{Timeout: stringPtr("0s")}))
		assert.Empty(t, errs)
	})

	t.Run("negative timeout is rejected", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(&api.Resilience{Timeout: stringPtr("-5s")}))
		assertHasFieldError(t, errs, "spec.resilience.timeout")
	})

	t.Run("malformed idleTimeout is rejected", func(t *testing.T) {
		errs := validator.Validate(validProviderWithResilience(&api.Resilience{IdleTimeout: stringPtr("abc")}))
		assertHasFieldError(t, errs, "spec.resilience.idleTimeout")
	})
}

func TestValidateLLMProxy_Resilience(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("valid timeout", func(t *testing.T) {
		errs := validator.Validate(validProxyWithResilience(&api.Resilience{Timeout: stringPtr("75s")}))
		assert.Empty(t, errs)
	})

	t.Run("nil resilience is fine", func(t *testing.T) {
		errs := validator.Validate(validProxyWithResilience(nil))
		assert.Empty(t, errs)
	})

	t.Run("malformed timeout is rejected", func(t *testing.T) {
		errs := validator.Validate(validProxyWithResilience(&api.Resilience{Timeout: stringPtr("fast")}))
		assertHasFieldError(t, errs, "spec.resilience.timeout")
	})

	t.Run("negative idleTimeout is rejected", func(t *testing.T) {
		errs := validator.Validate(validProxyWithResilience(&api.Resilience{IdleTimeout: stringPtr("-1s")}))
		assertHasFieldError(t, errs, "spec.resilience.idleTimeout")
	})
}

func assertHasFieldError(t *testing.T, errs []ValidationError, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field {
			return
		}
	}
	t.Fatalf("expected a validation error on field %q, got %+v", field, errs)
}
