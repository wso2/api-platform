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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestNewSecretValidator(t *testing.T) {
	v := NewSecretValidator()
	require.NotNil(t, v)
	assert.NotNil(t, v.urlFriendlyNameRegex)
	assert.Equal(t, []string{"Secret"}, v.supportedKinds)
	assert.Equal(t, []string{"default"}, v.supportedTypes)
	assert.Equal(t, 10, v.maxSecretSize)
}

func TestSecretValidator_Validate_UnsupportedType(t *testing.T) {
	v := NewSecretValidator()
	errs := v.Validate("not a secret config")
	require.Len(t, errs, 1)
	assert.Equal(t, "config", errs[0].Field)
	assert.Contains(t, errs[0].Message, "Unsupported configuration type")
}

func TestSecretValidator_Validate_PointerAndValue(t *testing.T) {
	v := NewSecretValidator()

	cfg := api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "my-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "My Secret",
			Value:       "super-secret-value",
		},
	}

	// Both pointer and value should work with no errors.
	errsPtr := v.Validate(&cfg)
	assert.Empty(t, errsPtr)

	errsVal := v.Validate(cfg)
	assert.Empty(t, errsVal)
}

func TestSecretValidator_ValidateSecretConfiguration_Valid(t *testing.T) {
	v := NewSecretValidator()
	desc := "A test secret"
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "Test Secret",
			Description: &desc,
			Value:       "my-value",
		},
	}
	errs := v.Validate(cfg)
	assert.Empty(t, errs)
}

func TestSecretValidator_InvalidApiVersion(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: "wrong-version",
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "Test Secret",
			Value:       "value",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "version")
}

func TestSecretValidator_InvalidKind(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       "NotSecret",
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "Test Secret",
			Value:       "value",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "kind")
}

func TestSecretValidator_MissingDisplayName(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "",
			Value:       "value",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "spec.displayName")
}

func TestSecretValidator_DisplayNameTooLong(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: strings.Repeat("a", 254),
			Value:       "value",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "spec.displayName")
}

func TestSecretValidator_DisplayNameInvalidChars(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "invalid@name!",
			Value:       "value",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "spec.displayName")
}

func TestSecretValidator_DescriptionTooLong(t *testing.T) {
	v := NewSecretValidator()
	longDesc := strings.Repeat("x", 1025)
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "Test Secret",
			Description: &longDesc,
			Value:       "value",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "spec.description")
}

func TestSecretValidator_MissingValue(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "Test Secret",
			Value:       "",
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "spec.value")
}

func TestSecretValidator_ValueTooLarge(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.SecretConfigurationRequestKindSecret,
		Metadata:   api.Metadata{Name: "test-secret"},
		Spec: api.SecretConfigData{
			DisplayName: "Test Secret",
			Value:       strings.Repeat("v", 10*1024+1),
		},
	}
	errs := v.Validate(cfg)
	require.NotEmpty(t, errs)
	fields := extractFields(errs)
	assert.Contains(t, fields, "spec.value")
}

func TestSecretValidator_MultipleErrors(t *testing.T) {
	v := NewSecretValidator()
	cfg := &api.SecretConfigurationRequest{
		ApiVersion: "bad-version",
		Kind:       "BadKind",
		Metadata:   api.Metadata{Name: "x"},
		Spec: api.SecretConfigData{
			DisplayName: "",
			Value:       "",
		},
	}
	errs := v.Validate(cfg)
	assert.True(t, len(errs) >= 2, "expected multiple errors, got %d", len(errs))
}

// extractFields returns the set of field names from a slice of ValidationErrors.
func extractFields(errs []ValidationError) map[string]bool {
	fields := make(map[string]bool)
	for _, e := range errs {
		fields[e.Field] = true
	}
	return fields
}
