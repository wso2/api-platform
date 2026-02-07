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

package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// Kernel Creation Tests
// =============================================================================

func TestNewKernel(t *testing.T) {
	kernel := NewKernel()

	require.NotNil(t, kernel)
	assert.NotNil(t, kernel.Routes)
	assert.Empty(t, kernel.Routes)
}

// =============================================================================
// Route Registration Tests
// =============================================================================

func TestRegisterRoute(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{}

	kernel.RegisterRoute("test-route", chain)

	assert.Len(t, kernel.Routes, 1)
	assert.Equal(t, chain, kernel.Routes["test-route"])
}

func TestRegisterRoute_Multiple(t *testing.T) {
	kernel := NewKernel()
	chain1 := &registry.PolicyChain{}
	chain2 := &registry.PolicyChain{}

	kernel.RegisterRoute("route-1", chain1)
	kernel.RegisterRoute("route-2", chain2)

	assert.Len(t, kernel.Routes, 2)
	assert.Equal(t, chain1, kernel.Routes["route-1"])
	assert.Equal(t, chain2, kernel.Routes["route-2"])
}

func TestRegisterRoute_OverwriteExisting(t *testing.T) {
	kernel := NewKernel()
	chain1 := &registry.PolicyChain{}
	chain2 := &registry.PolicyChain{}

	kernel.RegisterRoute("test-route", chain1)
	kernel.RegisterRoute("test-route", chain2)

	assert.Len(t, kernel.Routes, 1)
	assert.Equal(t, chain2, kernel.Routes["test-route"])
}

// =============================================================================
// GetPolicyChainForKey Tests
// =============================================================================

func TestGetPolicyChainForKey_Exists(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{}

	kernel.RegisterRoute("my-route", chain)

	result := kernel.GetPolicyChainForKey("my-route")

	assert.Equal(t, chain, result)
}

func TestGetPolicyChainForKey_NotExists(t *testing.T) {
	kernel := NewKernel()

	result := kernel.GetPolicyChainForKey("nonexistent")

	assert.Nil(t, result)
}

func TestGetPolicyChainForKey_EmptyKey(t *testing.T) {
	kernel := NewKernel()

	result := kernel.GetPolicyChainForKey("")

	assert.Nil(t, result)
}

// =============================================================================
// UnregisterRoute Tests
// =============================================================================

func TestUnregisterRoute(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{}

	kernel.RegisterRoute("test-route", chain)
	assert.Len(t, kernel.Routes, 1)

	kernel.UnregisterRoute("test-route")
	assert.Empty(t, kernel.Routes)
}

func TestUnregisterRoute_NonExistent(t *testing.T) {
	kernel := NewKernel()

	// Should not panic
	kernel.UnregisterRoute("nonexistent")

	assert.Empty(t, kernel.Routes)
}

func TestUnregisterRoute_PreservesOthers(t *testing.T) {
	kernel := NewKernel()
	chain1 := &registry.PolicyChain{}
	chain2 := &registry.PolicyChain{}

	kernel.RegisterRoute("route-1", chain1)
	kernel.RegisterRoute("route-2", chain2)

	kernel.UnregisterRoute("route-1")

	assert.Len(t, kernel.Routes, 1)
	assert.Nil(t, kernel.Routes["route-1"])
	assert.Equal(t, chain2, kernel.Routes["route-2"])
}

// =============================================================================
// ApplyWholeRoutes Tests
// =============================================================================

func TestApplyWholeRoutes_Empty(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{}
	kernel.RegisterRoute("existing", chain)

	kernel.ApplyWholeRoutes(map[string]*registry.PolicyChain{})

	assert.Empty(t, kernel.Routes)
}

func TestApplyWholeRoutes_ReplaceAll(t *testing.T) {
	kernel := NewKernel()
	oldChain := &registry.PolicyChain{}
	kernel.RegisterRoute("old-route", oldChain)

	newChain := &registry.PolicyChain{}
	newRoutes := map[string]*registry.PolicyChain{
		"new-route": newChain,
	}

	kernel.ApplyWholeRoutes(newRoutes)

	assert.Len(t, kernel.Routes, 1)
	assert.Nil(t, kernel.Routes["old-route"])
	assert.Equal(t, newChain, kernel.Routes["new-route"])
}

func TestApplyWholeRoutes_MultipleRoutes(t *testing.T) {
	kernel := NewKernel()

	chain1 := &registry.PolicyChain{}
	chain2 := &registry.PolicyChain{}
	chain3 := &registry.PolicyChain{}

	newRoutes := map[string]*registry.PolicyChain{
		"route-1": chain1,
		"route-2": chain2,
		"route-3": chain3,
	}

	kernel.ApplyWholeRoutes(newRoutes)

	assert.Len(t, kernel.Routes, 3)
}

// =============================================================================
// DumpRoutes Tests
// =============================================================================

func TestDumpRoutes_Empty(t *testing.T) {
	kernel := NewKernel()

	dump := kernel.DumpRoutes()

	assert.NotNil(t, dump)
	assert.Empty(t, dump)
}

func TestDumpRoutes_ReturnsAllRoutes(t *testing.T) {
	kernel := NewKernel()
	chain1 := &registry.PolicyChain{}
	chain2 := &registry.PolicyChain{}

	kernel.RegisterRoute("route-1", chain1)
	kernel.RegisterRoute("route-2", chain2)

	dump := kernel.DumpRoutes()

	assert.Len(t, dump, 2)
	assert.Equal(t, chain1, dump["route-1"])
	assert.Equal(t, chain2, dump["route-2"])
}

func TestDumpRoutes_ReturnsCopy(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{}
	kernel.RegisterRoute("route", chain)

	dump := kernel.DumpRoutes()

	// Modify the dump
	dump["new-route"] = &registry.PolicyChain{}

	// Original should be unchanged
	assert.Len(t, kernel.Routes, 1)
	assert.Nil(t, kernel.Routes["new-route"])
}

// =============================================================================
// Body Mode Tests
// =============================================================================

func TestDetermineRequestBodyMode_RequiresBody(t *testing.T) {
	chain := &registry.PolicyChain{
		RequiresRequestBody: true,
	}

	mode := determineRequestBodyMode(chain)

	assert.Equal(t, BodyModeBuffered, mode)
}

func TestDetermineRequestBodyMode_NoBodyRequired(t *testing.T) {
	chain := &registry.PolicyChain{
		RequiresRequestBody: false,
	}

	mode := determineRequestBodyMode(chain)

	assert.Equal(t, BodyModeSkip, mode)
}

func TestDetermineResponseBodyMode_RequiresBody(t *testing.T) {
	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
	}

	mode := determineResponseBodyMode(chain)

	assert.Equal(t, BodyModeBuffered, mode)
}

func TestDetermineResponseBodyMode_NoBodyRequired(t *testing.T) {
	chain := &registry.PolicyChain{
		RequiresResponseBody: false,
	}

	mode := determineResponseBodyMode(chain)

	assert.Equal(t, BodyModeSkip, mode)
}

func TestGetRequestBodyMode_RouteExists(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{
		RequiresRequestBody: true,
	}
	kernel.RegisterRoute("test-route", chain)

	mode := kernel.GetRequestBodyMode("test-route")

	assert.Equal(t, BodyModeBuffered, mode)
}

func TestGetRequestBodyMode_RouteNotExists(t *testing.T) {
	kernel := NewKernel()

	mode := kernel.GetRequestBodyMode("nonexistent")

	assert.Equal(t, BodyModeSkip, mode)
}

func TestGetResponseBodyMode_RouteExists(t *testing.T) {
	kernel := NewKernel()
	chain := &registry.PolicyChain{
		RequiresResponseBody: true,
	}
	kernel.RegisterRoute("test-route", chain)

	mode := kernel.GetResponseBodyMode("test-route")

	assert.Equal(t, BodyModeBuffered, mode)
}

func TestGetResponseBodyMode_RouteNotExists(t *testing.T) {
	kernel := NewKernel()

	mode := kernel.GetResponseBodyMode("nonexistent")

	assert.Equal(t, BodyModeSkip, mode)
}

// =============================================================================
// Body Mode Constants Tests
// =============================================================================

func TestBodyModeConstants(t *testing.T) {
	assert.Equal(t, BodyMode(0), BodyModeSkip)
	assert.Equal(t, BodyMode(1), BodyModeBuffered)
}

// =============================================================================
// Analytics Constants Tests
// =============================================================================

func TestAnalyticsConstants(t *testing.T) {
	assert.Equal(t, "x-wso2-", Wso2MetadataPrefix)
	assert.Equal(t, "x-wso2-api-id", APIIDKey)
	assert.Equal(t, "x-wso2-api-name", APINameKey)
	assert.Equal(t, "x-wso2-api-version", APIVersionKey)
	assert.Equal(t, "x-wso2-api-type", APITypeKey)
	assert.Equal(t, "x-wso2-api-context", APIContextKey)
	assert.Equal(t, "x-wso2-operation-path", OperationPathKey)
	assert.Equal(t, "x-wso2-api-kind", APIKindKey)
}

// =============================================================================
// convertToStructValue Tests
// =============================================================================

func TestConvertToStructValue_SimpleString(t *testing.T) {
	val, err := convertToStructValue("hello")

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, "hello", val.GetStringValue())
}

func TestConvertToStructValue_SimpleInt(t *testing.T) {
	val, err := convertToStructValue(42)

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, float64(42), val.GetNumberValue())
}

func TestConvertToStructValue_SimpleBool(t *testing.T) {
	val, err := convertToStructValue(true)

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.True(t, val.GetBoolValue())
}

func TestConvertToStructValue_SimpleFloat(t *testing.T) {
	val, err := convertToStructValue(3.14)

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.InDelta(t, 3.14, val.GetNumberValue(), 0.001)
}

func TestConvertToStructValue_NilValue(t *testing.T) {
	val, err := convertToStructValue(nil)

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.NotNil(t, val.GetNullValue())
}

func TestConvertToStructValue_SimpleSlice(t *testing.T) {
	val, err := convertToStructValue([]interface{}{"a", "b", "c"})

	require.NoError(t, err)
	require.NotNil(t, val)

	listVal := val.GetListValue()
	require.NotNil(t, listVal)
	assert.Len(t, listVal.Values, 3)
}

func TestConvertToStructValue_SimpleMap(t *testing.T) {
	val, err := convertToStructValue(map[string]interface{}{
		"key": "value",
	})

	require.NoError(t, err)
	require.NotNil(t, val)

	structVal := val.GetStructValue()
	require.NotNil(t, structVal)
	assert.Contains(t, structVal.Fields, "key")
}

func TestConvertToStructValue_ComplexMapStringSlice(t *testing.T) {
	// map[string][]string is not directly supported by protobuf
	// Should serialize to JSON string
	val, err := convertToStructValue(map[string][]string{
		"headers": {"val1", "val2"},
	})

	require.NoError(t, err)
	require.NotNil(t, val)
	// Should be serialized as JSON string
	strVal := val.GetStringValue()
	assert.Contains(t, strVal, "headers")
	assert.Contains(t, strVal, "val1")
}

// =============================================================================
// extractMetadataFromRouteMetadata Tests
// =============================================================================

func TestExtractMetadataFromRouteMetadata_AllFields(t *testing.T) {
	routeMeta := RouteMetadata{
		APIName:       "PetStore",
		APIVersion:    "v1.0.0",
		Context:       "/petstore",
		OperationPath: "/pets/{id}",
		APIKind:       "REST",
	}

	result := extractMetadataFromRouteMetadata(routeMeta)

	assert.Equal(t, "PetStore", result[APINameKey])
	assert.Equal(t, "v1.0.0", result[APIVersionKey])
	assert.Equal(t, "/petstore", result[APIContextKey])
	assert.Equal(t, "/pets/{id}", result[OperationPathKey])
	assert.Equal(t, "REST", result[APIKindKey])
}

func TestExtractMetadataFromRouteMetadata_EmptyFields(t *testing.T) {
	routeMeta := RouteMetadata{}

	result := extractMetadataFromRouteMetadata(routeMeta)

	assert.Empty(t, result)
}

func TestExtractMetadataFromRouteMetadata_PartialFields(t *testing.T) {
	routeMeta := RouteMetadata{
		APIName:    "TestAPI",
		APIVersion: "v2.0",
	}

	result := extractMetadataFromRouteMetadata(routeMeta)

	assert.Len(t, result, 2)
	assert.Equal(t, "TestAPI", result[APINameKey])
	assert.Equal(t, "v2.0", result[APIVersionKey])
	assert.NotContains(t, result, APIContextKey)
	assert.NotContains(t, result, OperationPathKey)
	assert.NotContains(t, result, APIKindKey)
}

func TestExtractMetadataFromRouteMetadata_OnlyContext(t *testing.T) {
	routeMeta := RouteMetadata{
		Context: "/api/v1",
	}

	result := extractMetadataFromRouteMetadata(routeMeta)

	assert.Len(t, result, 1)
	assert.Equal(t, "/api/v1", result[APIContextKey])
}

// =============================================================================
// buildAnalyticsStruct Tests
// =============================================================================

func TestBuildAnalyticsStruct_EmptyData(t *testing.T) {
	data := map[string]any{}

	result, err := buildAnalyticsStruct(data, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Fields)
}

func TestBuildAnalyticsStruct_SimpleData(t *testing.T) {
	data := map[string]any{
		"requestId": "req-123",
		"statusCode": 200,
	}

	result, err := buildAnalyticsStruct(data, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Fields, 2)
	assert.Equal(t, "req-123", result.Fields["requestId"].GetStringValue())
	assert.Equal(t, float64(200), result.Fields["statusCode"].GetNumberValue())
}

func TestBuildAnalyticsStruct_NilContext(t *testing.T) {
	data := map[string]any{
		"key": "value",
	}

	result, err := buildAnalyticsStruct(data, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Fields, 1)
}

func TestBuildAnalyticsStruct_MultipleTypes(t *testing.T) {
	data := map[string]any{
		"string":  "hello",
		"number":  42,
		"float":   3.14,
		"boolean": true,
	}

	result, err := buildAnalyticsStruct(data, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Fields, 4)
	assert.Equal(t, "hello", result.Fields["string"].GetStringValue())
	assert.Equal(t, float64(42), result.Fields["number"].GetNumberValue())
	assert.InDelta(t, 3.14, result.Fields["float"].GetNumberValue(), 0.001)
	assert.True(t, result.Fields["boolean"].GetBoolValue())
}

// =============================================================================
// buildAnalyticsStruct with ExecutionContext Tests
// =============================================================================

func TestBuildAnalyticsStruct_WithExecutionContext(t *testing.T) {
	// Create a mock execution context with SharedContext
	execCtx := &PolicyExecutionContext{
		requestContext: &policy.RequestContext{
			SharedContext: &policy.SharedContext{
				APIId:         "api-123",
				APIName:       "PetStore",
				APIVersion:    "v1.0.0",
				APIContext:    "/petstore",
				OperationPath: "/pets/{id}",
				APIKind:       "REST",
				ProjectID:     "proj-456",
			},
		},
	}

	data := map[string]any{
		"customKey": "customValue",
	}

	result, err := buildAnalyticsStruct(data, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check custom data is included
	assert.Equal(t, "customValue", result.Fields["customKey"].GetStringValue())

	// Check system metadata from SharedContext is included
	assert.Equal(t, "api-123", result.Fields[APIIDKey].GetStringValue())
	assert.Equal(t, "PetStore", result.Fields[APINameKey].GetStringValue())
	assert.Equal(t, "v1.0.0", result.Fields[APIVersionKey].GetStringValue())
	assert.Equal(t, "/petstore", result.Fields[APIContextKey].GetStringValue())
	assert.Equal(t, "/pets/{id}", result.Fields[OperationPathKey].GetStringValue())
	assert.Equal(t, "REST", result.Fields[APIKindKey].GetStringValue())
	assert.Equal(t, "proj-456", result.Fields[ProjectIDKey].GetStringValue())
}

func TestBuildAnalyticsStruct_WithPartialSharedContext(t *testing.T) {
	// Create execution context with only some fields populated
	execCtx := &PolicyExecutionContext{
		requestContext: &policy.RequestContext{
			SharedContext: &policy.SharedContext{
				APIName:    "TestAPI",
				APIVersion: "v2.0",
				// Other fields empty
			},
		},
	}

	data := map[string]any{}

	result, err := buildAnalyticsStruct(data, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Only populated fields should be present
	assert.Equal(t, "TestAPI", result.Fields[APINameKey].GetStringValue())
	assert.Equal(t, "v2.0", result.Fields[APIVersionKey].GetStringValue())

	// Empty fields should not be present
	_, hasAPIId := result.Fields[APIIDKey]
	assert.False(t, hasAPIId)
}

func TestBuildAnalyticsStruct_WithNilSharedContext(t *testing.T) {
	execCtx := &PolicyExecutionContext{
		requestContext: &policy.RequestContext{
			SharedContext: nil,
		},
	}

	data := map[string]any{
		"key": "value",
	}

	result, err := buildAnalyticsStruct(data, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Fields, 1)
	assert.Equal(t, "value", result.Fields["key"].GetStringValue())
}

func TestBuildAnalyticsStruct_WithNilRequestContext(t *testing.T) {
	execCtx := &PolicyExecutionContext{
		requestContext: nil,
	}

	data := map[string]any{
		"key": "value",
	}

	result, err := buildAnalyticsStruct(data, execCtx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Fields, 1)
}

// =============================================================================
// convertToStructValue Error Path Tests
// =============================================================================

// unmarshallableType is a type that cannot be marshaled to JSON
type unmarshallableType struct {
	Ch chan int // channels cannot be marshaled
}

func TestConvertToStructValue_UnmarshallableType(t *testing.T) {
	// Create a value that cannot be converted directly or marshaled to JSON
	val := unmarshallableType{Ch: make(chan int)}

	_, err := convertToStructValue(val)

	// Should return an error because the type cannot be marshaled
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal value to JSON")
}

func TestConvertToStructValue_FunctionType(t *testing.T) {
	// Functions cannot be marshaled to JSON
	fn := func() {}

	_, err := convertToStructValue(fn)

	assert.Error(t, err)
}

// =============================================================================
// extractMetadataFromRouteMetadata Additional Tests
// =============================================================================

func TestExtractMetadataFromRouteMetadata_WithProjectID(t *testing.T) {
	routeMeta := RouteMetadata{
		APIName:   "TestAPI",
		ProjectID: "project-123",
	}

	result := extractMetadataFromRouteMetadata(routeMeta)

	assert.Equal(t, "TestAPI", result[APINameKey])
	assert.Equal(t, "project-123", result[ProjectIDKey])
}

// =============================================================================
// ConfigLoader Creation Tests
// =============================================================================

func TestNewConfigLoader(t *testing.T) {
	// Create a minimal kernel for testing
	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	loader := NewConfigLoader(kernel, reg)

	require.NotNil(t, loader)
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err := loader.LoadFromFile("/nonexistent/path/config.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	// Write invalid YAML that will fail to unmarshal into []policyenginev1.PolicyChain
	invalidYaml := `
this is not valid yaml as a list
  - broken structure
    missing: proper formatting
  invalid: [unclosed bracket
`
	err := os.WriteFile(configPath, []byte(invalidYaml), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestLoadFromFile_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty.yaml")

	err := os.WriteFile(configPath, []byte(""), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	// Empty file should be parsed successfully with no routes to add
	assert.NoError(t, err)
}

func TestLoadFromFile_EmptyList(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-list.yaml")

	// Empty list
	err := os.WriteFile(configPath, []byte("[]"), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.NoError(t, err)
}

func TestLoadFromFile_ValidationError_EmptyRouteKey(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-route-key.yaml")

	// Valid YAML but empty route_key should fail validation
	yamlContent := `
- route_key: ""
  policies: []
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "route_key is required")
}

func TestLoadFromFile_ValidationError_EmptyPolicyName(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-policy-name.yaml")

	// Valid route_key but policy with empty name
	yamlContent := `
- route_key: "test-route"
  policies:
    - name: ""
      version: "v1.0.0"
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoadFromFile_ValidationError_EmptyPolicyVersion(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-policy-version.yaml")

	// Valid route_key but policy with empty version
	yamlContent := `
- route_key: "test-route"
  policies:
    - name: "test-policy"
      version: ""
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version is required")
}

func TestLoadFromFile_ValidationError_PolicyNotInRegistry(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "unknown-policy.yaml")

	// Valid config but policy not registered
	yamlContent := `
- route_key: "test-route"
  policies:
    - name: "unknown-policy"
      version: "v1.0.0"
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-policy")
}

func TestLoadFromFile_ValidationError_PolicyDefinitionExistsButNoFactory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "no-factory.yaml")

	// Valid config with policy name and version
	yamlContent := `
- route_key: "test-route"
  policies:
    - name: "test-policy"
      version: "v1.0.0"
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	kernel := &Kernel{
		Routes: make(map[string]*registry.PolicyChain),
	}
	// Register definition but NOT factory
	reg := &registry.PolicyRegistry{
		Definitions: map[string]*policy.PolicyDefinition{
			"test-policy:v1.0.0": {
				Name:    "test-policy",
				Version: "v1.0.0",
			},
		},
		Factories: make(map[string]policy.PolicyFactory), // No factory registered
	}
	loader := NewConfigLoader(kernel, reg)

	err = loader.LoadFromFile(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "factory not found")
}
