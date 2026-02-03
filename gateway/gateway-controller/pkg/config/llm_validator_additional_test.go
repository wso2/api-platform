package config

import (
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// Tests for lines 56-57, 60-61, 64-65: Validate method with value types (not pointers)
func TestLLMValidator_ValidateValueTypes(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("LLMProviderTemplate as value", func(t *testing.T) {
		template := api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata:   api.Metadata{Name: "test-template"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "Test Template"},
		}

		errors := validator.Validate(template)
		// Should handle value type correctly
		if len(errors) > 0 {
			// Some errors expected (e.g., missing fields) but should not panic
			t.Logf("Got expected validation errors: %d", len(errors))
		}
	})

	t.Run("LLMProviderConfiguration as value", func(t *testing.T) {
		provider := api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProvider,
			Metadata:   api.Metadata{Name: "test-provider"},
		}

		errors := validator.Validate(provider)
		// Should handle value type correctly
		if len(errors) == 0 {
			t.Error("Expected validation errors for incomplete provider")
		}
	})

	t.Run("LLMProxyConfiguration as value", func(t *testing.T) {
		proxy := api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProxy,
			Metadata:   api.Metadata{Name: "test-proxy"},
		}

		errors := validator.Validate(proxy)
		// Should handle value type correctly
		if len(errors) == 0 {
			t.Error("Expected validation errors for incomplete proxy")
		}
	})
}

// Tests for lines 81-85: validateLLMProviderTemplate with nil template
func TestLLMValidator_ValidateLLMProviderTemplate_Nil(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil template", func(t *testing.T) {
		errors := validator.validateLLMProviderTemplate(nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil template, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "Template cannot be nil" {
			t.Errorf("Expected 'Template cannot be nil' error, got %s", errors[0].Message)
		}
	})
}

// Tests for lines 105-119: metadata name validation
func TestLLMValidator_MetadataNameValidation(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Empty metadata name", func(t *testing.T) {
		template := &api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata:   api.Metadata{Name: ""},
			Spec:       api.LLMProviderTemplateData{DisplayName: "Test"},
		}

		errors := validator.validateLLMProviderTemplate(template)
		found := false
		for _, err := range errors {
			if err.Field == "metadata.name" && err.Message == "metadata.name is required" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for empty metadata.name")
		}
	})

	t.Run("Metadata name exceeds 253 characters", func(t *testing.T) {
		longName := ""
		for i := 0; i < 254; i++ {
			longName += "a"
		}

		template := &api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata:   api.Metadata{Name: longName},
			Spec:       api.LLMProviderTemplateData{DisplayName: "Test"},
		}

		errors := validator.validateLLMProviderTemplate(template)
		found := false
		for _, err := range errors {
			if err.Field == "metadata.name" && err.Message == "metadata.name must not exceed 253 characters" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for metadata.name exceeding 253 characters")
		}
	})

	t.Run("Invalid metadata name format", func(t *testing.T) {
		template := &api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata:   api.Metadata{Name: "Invalid_Name_With_Underscore"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "Test"},
		}

		errors := validator.validateLLMProviderTemplate(template)
		found := false
		for _, err := range errors {
			if err.Field == "metadata.name" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for invalid metadata.name format")
		}
	})
}

// Tests for lines 133-137: validateTemplateSpec with nil spec
func TestLLMValidator_ValidateTemplateSpec_Nil(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil spec", func(t *testing.T) {
		errors := validator.validateTemplateSpec(nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil spec, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "Template spec cannot be nil" {
			t.Errorf("Expected 'Template spec cannot be nil' error, got %s", errors[0].Message)
		}
	})
}

// Tests for lines 212-216: validateLLMProvider with nil provider
func TestLLMValidator_ValidateLLMProvider_Nil(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil provider", func(t *testing.T) {
		errors := validator.validateLLMProvider(nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil provider, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "provider cannot be nil" {
			t.Errorf("Expected 'provider cannot be nil' error, got %s", errors[0].Message)
		}
	})
}

// Tests for lines 241-246: Provider metadata name validation
func TestLLMValidator_ProviderMetadataValidation(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Provider metadata name exceeds 253 characters", func(t *testing.T) {
		longName := ""
		for i := 0; i < 254; i++ {
			longName += "a"
		}

		url := "https://api.openai.com"
		provider := &api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProvider,
			Metadata:   api.Metadata{Name: longName},
			Spec: api.LLMProviderConfigData{
				DisplayName: "Test",
				Template:    "openai",
				Upstream:    api.LLMProviderConfigData_Upstream{Url: &url},
				AccessControl: api.LLMAccessControl{
					Mode: "allow_all",
				},
			},
		}

		errors := validator.validateLLMProvider(provider)
		found := false
		for _, err := range errors {
			if err.Field == "metadata.name" && err.Message == "metadata.name must not exceed 253 characters" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for provider metadata.name exceeding 253 characters")
		}
	})
}

// Tests for lines 269-273: validateProviderSpec with nil spec
func TestLLMValidator_ValidateProviderSpec_Nil(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil provider spec", func(t *testing.T) {
		errors := validator.validateProviderSpec(nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil spec, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "Provider spec is required" {
			t.Errorf("Expected 'Provider spec is required' error, got %s", errors[0].Message)
		}
	})
}

// Tests for lines 290-294: Version format validation
func TestLLMValidator_ProviderVersionValidation(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Invalid version format", func(t *testing.T) {
		url := "https://api.openai.com"
		spec := &api.LLMProviderConfigData{
			DisplayName: "Test Provider",
			Version:     "invalid-version",
			Template:    "openai",
			Upstream:    api.LLMProviderConfigData_Upstream{Url: &url},
			AccessControl: api.LLMAccessControl{
				Mode: "allow_all",
			},
		}

		errors := validator.validateProviderSpec(spec)
		found := false
		for _, err := range errors {
			if err.Field == "spec.version" && err.Message == "Provider version format is invalid (expected vX.Y.Z)" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for invalid version format")
		}
	})
}

// Tests for lines 319-324: validateUpstreamWithAuth with nil upstream
func TestLLMValidator_ValidateUpstreamWithAuth_Nil(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil upstream", func(t *testing.T) {
		errors := validator.validateUpstreamWithAuth("test", nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil upstream, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "Upstream is required" {
			t.Errorf("Expected 'Upstream is required' error, got %s", errors[0].Message)
		}
	})
}

// Tests for lines 335-340: URL validation errors
func TestLLMValidator_UpstreamURLValidation(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Malformed URL", func(t *testing.T) {
		malformed := "ht!tp://invalid url with spaces"
		upstream := &api.LLMProviderConfigData_Upstream{
			Url: &malformed,
		}

		errors := validator.validateUpstreamWithAuth("test", upstream)
		found := false
		for _, err := range errors {
			if err.Field == "test.url" && err.Message != "" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for malformed URL")
		}
	})

	t.Run("Invalid URL scheme", func(t *testing.T) {
		invalidScheme := "ftp://example.com"
		upstream := &api.LLMProviderConfigData_Upstream{
			Url: &invalidScheme,
		}

		errors := validator.validateUpstreamWithAuth("test", upstream)
		found := false
		for _, err := range errors {
			if err.Field == "test.url" && err.Message == "Upstream URL must use http or https scheme" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for invalid URL scheme")
		}
	})
}

// Tests for lines 394-432: validateLLMProxy with nil and metadata validation
func TestLLMValidator_ValidateLLMProxy_NilAndMetadata(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil proxy", func(t *testing.T) {
		errors := validator.validateLLMProxy(nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil proxy, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "proxy cannot be nil" {
			t.Errorf("Expected 'proxy cannot be nil' error, got %s", errors[0].Message)
		}
	})

	t.Run("Proxy metadata name exceeds 253 characters", func(t *testing.T) {
		longName := ""
		for i := 0; i < 254; i++ {
			longName += "a"
		}

		proxy := &api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProxy,
			Metadata:   api.Metadata{Name: longName},
			Spec: api.LLMProxyConfigData{
				DisplayName: "Test",
				Provider:    "test-provider",
			},
		}

		errors := validator.validateLLMProxy(proxy)
		found := false
		for _, err := range errors {
			if err.Field == "metadata.name" && err.Message == "metadata.name must not exceed 253 characters" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for proxy metadata.name exceeding 253 characters")
		}
	})
}

// Tests for lines 451-476: validateProxyData with nil spec and version
func TestLLMValidator_ValidateProxyData_NilAndVersion(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil proxy data", func(t *testing.T) {
		errors := validator.validateProxyData(nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil proxy data, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "Proxy data is required" {
			t.Errorf("Expected 'Proxy data is required' error, got %s", errors[0].Message)
		}
	})

	t.Run("Invalid proxy version format", func(t *testing.T) {
		spec := &api.LLMProxyConfigData{
			DisplayName: "Test Proxy",
			Version:     "invalid-version",
			Provider:    "test-provider",
		}

		errors := validator.validateProxyData(spec)
		found := false
		for _, err := range errors {
			if err.Field == "spec.version" && err.Message == "Proxy version format is invalid (expected vX.Y.Z)" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for invalid proxy version format")
		}
	})
}

// Tests for lines 486-490: Provider name format validation
func TestLLMValidator_ProxyProviderValidation(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Invalid provider name format", func(t *testing.T) {
		spec := &api.LLMProxyConfigData{
			DisplayName: "Test Proxy",
			Provider:    "Invalid_Provider_Name",
		}

		errors := validator.validateProxyData(spec)
		found := false
		for _, err := range errors {
			if err.Field == "spec.provider" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected error for invalid provider name format")
		}
	})
}

// Tests for lines 501-506: validateAccessControl with nil
func TestLLMValidator_ValidateAccessControl_Nil(t *testing.T) {
	validator := NewLLMValidator()

	t.Run("Nil access control", func(t *testing.T) {
		errors := validator.validateAccessControl("test", nil)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error for nil access control, got %d", len(errors))
		}

		if len(errors) > 0 && errors[0].Message != "Access control is required" {
			t.Errorf("Expected 'Access control is required' error, got %s", errors[0].Message)
		}
	})
}
