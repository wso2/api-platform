/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// newTestRegistry creates a fresh PolicyRegistry for testing (not using the global singleton)
func newTestRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}
}

// TestCompositeKey tests the compositeKey function
func TestCompositeKey(t *testing.T) {
	tests := []struct {
		name     string
		pName    string
		version  string
		expected string
	}{
		{
			name:     "simple key",
			pName:    "jwt-auth",
			version:  "v1.0.0",
			expected: "jwt-auth:v1.0.0",
		},
		{
			name:     "with different version",
			pName:    "rate-limit",
			version:  "v2.1.0",
			expected: "rate-limit:v2.1.0",
		},
		{
			name:     "empty name",
			pName:    "",
			version:  "v1.0.0",
			expected: ":v1.0.0",
		},
		{
			name:     "empty version",
			pName:    "jwt-auth",
			version:  "",
			expected: "jwt-auth:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compositeKey(tt.pName, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMergeParams tests the mergeParams function
func TestMergeParams(t *testing.T) {
	tests := []struct {
		name       string
		initParams map[string]interface{}
		params     map[string]interface{}
		expected   map[string]interface{}
	}{
		{
			name:       "both empty",
			initParams: map[string]interface{}{},
			params:     map[string]interface{}{},
			expected:   map[string]interface{}{},
		},
		{
			name:       "only init params",
			initParams: map[string]interface{}{"key1": "value1", "key2": 42},
			params:     map[string]interface{}{},
			expected:   map[string]interface{}{"key1": "value1", "key2": 42},
		},
		{
			name:       "only runtime params",
			initParams: map[string]interface{}{},
			params:     map[string]interface{}{"key1": "value1", "key2": 42},
			expected:   map[string]interface{}{"key1": "value1", "key2": 42},
		},
		{
			name:       "runtime overrides init",
			initParams: map[string]interface{}{"key1": "init-value", "key2": 10},
			params:     map[string]interface{}{"key1": "runtime-value"},
			expected:   map[string]interface{}{"key1": "runtime-value", "key2": 10},
		},
		{
			name:       "merge with no conflict",
			initParams: map[string]interface{}{"init-key": "init-value"},
			params:     map[string]interface{}{"runtime-key": "runtime-value"},
			expected:   map[string]interface{}{"init-key": "init-value", "runtime-key": "runtime-value"},
		},
		{
			name:       "nil values",
			initParams: map[string]interface{}{"key1": nil},
			params:     map[string]interface{}{"key2": nil},
			expected:   map[string]interface{}{"key1": nil, "key2": nil},
		},
		{
			name:       "complex values",
			initParams: map[string]interface{}{"arr": []string{"a", "b"}, "num": 100},
			params:     map[string]interface{}{"map": map[string]string{"k": "v"}},
			expected:   map[string]interface{}{"arr": []string{"a", "b"}, "num": 100, "map": map[string]string{"k": "v"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeParams(tt.initParams, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRegister tests the Register function
func TestRegister(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		reg := newTestRegistry()

		def := &policy.PolicyDefinition{
			Name:    "jwt-auth",
			Version: "v1.0.0",
		}
		factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")

		err := reg.Register(def, factory)
		require.NoError(t, err)

		// Verify registration
		assert.Len(t, reg.Definitions, 1)
		assert.Len(t, reg.Factories, 1)
		assert.NotNil(t, reg.Definitions["jwt-auth:v1.0.0"])
		assert.NotNil(t, reg.Factories["jwt-auth:v1.0.0"])
	})

	t.Run("duplicate registration returns error", func(t *testing.T) {
		reg := newTestRegistry()

		def := &policy.PolicyDefinition{
			Name:    "jwt-auth",
			Version: "v1.0.0",
		}
		factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")

		// First registration should succeed
		err := reg.Register(def, factory)
		require.NoError(t, err)

		// Second registration should fail
		err = reg.Register(def, factory)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "policy already registered")
	})

	t.Run("register multiple different policies", func(t *testing.T) {
		reg := newTestRegistry()

		policies := []struct {
			name    string
			version string
		}{
			{"jwt-auth", "v1.0.0"},
			{"jwt-auth", "v2.0.0"},
			{"rate-limit", "v1.0.0"},
			{"cors", "v1.0.0"},
		}

		for _, p := range policies {
			def := &policy.PolicyDefinition{Name: p.name, Version: p.version}
			factory := testutils.NewMockPolicyFactory(p.name, p.version)
			err := reg.Register(def, factory)
			require.NoError(t, err)
		}

		assert.Len(t, reg.Definitions, 4)
		assert.Len(t, reg.Factories, 4)
	})
}

// TestGetDefinition tests the GetDefinition function
func TestGetDefinition(t *testing.T) {
	reg := newTestRegistry()

	// Register a policy
	def := &policy.PolicyDefinition{
		Name:        "jwt-auth",
		Version:     "v1.0.0",
		Description: "JWT Authentication Policy",
	}
	factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")
	err := reg.Register(def, factory)
	require.NoError(t, err)

	t.Run("get existing definition", func(t *testing.T) {
		result, err := reg.GetDefinition("jwt-auth", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "jwt-auth", result.Name)
		assert.Equal(t, "v1.0.0", result.Version)
		assert.Equal(t, "JWT Authentication Policy", result.Description)
	})

	t.Run("get non-existent definition", func(t *testing.T) {
		result, err := reg.GetDefinition("non-existent", "v1.0.0")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "policy definition not found")
	})

	t.Run("get wrong version", func(t *testing.T) {
		result, err := reg.GetDefinition("jwt-auth", "v2.0.0")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "policy definition not found")
	})
}

// TestGetFactory tests the GetFactory function
func TestGetFactory(t *testing.T) {
	reg := newTestRegistry()

	// Register a policy
	def := &policy.PolicyDefinition{
		Name:    "jwt-auth",
		Version: "v1.0.0",
	}
	factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")
	err := reg.Register(def, factory)
	require.NoError(t, err)

	t.Run("get existing factory", func(t *testing.T) {
		result, err := reg.GetFactory("jwt-auth", "v1.0.0")
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify factory works
		instance, err := result(policy.PolicyMetadata{}, nil)
		require.NoError(t, err)
		assert.NotNil(t, instance)
	})

	t.Run("get non-existent factory", func(t *testing.T) {
		result, err := reg.GetFactory("non-existent", "v1.0.0")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "policy factory not found")
	})
}

// TestSetConfig tests the SetConfig function
func TestSetConfig(t *testing.T) {
	t.Run("set valid config", func(t *testing.T) {
		reg := newTestRegistry()

		config := map[string]interface{}{
			"policy_configurations": map[string]interface{}{
				"jwtauth": map[string]interface{}{
					"issuer": "https://auth.example.com",
				},
			},
		}

		err := reg.SetConfig(config)
		require.NoError(t, err)
		assert.NotNil(t, reg.ConfigResolver)
	})

	t.Run("set empty config", func(t *testing.T) {
		reg := newTestRegistry()

		err := reg.SetConfig(map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, reg.ConfigResolver)
	})

	t.Run("set nil config", func(t *testing.T) {
		reg := newTestRegistry()

		err := reg.SetConfig(nil)
		require.NoError(t, err)
		assert.NotNil(t, reg.ConfigResolver)
	})
}

// TestDumpPolicies tests the DumpPolicies function
func TestDumpPolicies(t *testing.T) {
	t.Run("empty registry", func(t *testing.T) {
		reg := newTestRegistry()

		dump := reg.DumpPolicies()
		assert.NotNil(t, dump)
		assert.Len(t, dump, 0)
	})

	t.Run("registry with policies", func(t *testing.T) {
		reg := newTestRegistry()

		// Register policies
		policies := []struct {
			name    string
			version string
		}{
			{"jwt-auth", "v1.0.0"},
			{"rate-limit", "v1.0.0"},
		}

		for _, p := range policies {
			def := &policy.PolicyDefinition{Name: p.name, Version: p.version}
			factory := testutils.NewMockPolicyFactory(p.name, p.version)
			err := reg.Register(def, factory)
			require.NoError(t, err)
		}

		dump := reg.DumpPolicies()
		assert.Len(t, dump, 2)
		assert.NotNil(t, dump["jwt-auth:v1.0.0"])
		assert.NotNil(t, dump["rate-limit:v1.0.0"])
	})

	t.Run("dump returns copy not reference", func(t *testing.T) {
		reg := newTestRegistry()

		def := &policy.PolicyDefinition{Name: "jwt-auth", Version: "v1.0.0"}
		factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")
		err := reg.Register(def, factory)
		require.NoError(t, err)

		dump := reg.DumpPolicies()

		// Modify the dump
		delete(dump, "jwt-auth:v1.0.0")

		// Original should be unchanged
		assert.Len(t, reg.Definitions, 1)
	})
}

// TestCreateInstance tests the CreateInstance function
func TestCreateInstance(t *testing.T) {
	t.Run("create instance with config resolver", func(t *testing.T) {
		reg := newTestRegistry()

		// Set up config
		config := map[string]interface{}{
			"policy_configurations": map[string]interface{}{
				"jwtauth": map[string]interface{}{
					"issuer": "https://auth.example.com",
				},
			},
		}
		err := reg.SetConfig(config)
		require.NoError(t, err)

		// Register policy with system parameters
		def := &policy.PolicyDefinition{
			Name:    "jwt-auth",
			Version: "v1.0.0",
			SystemParameters: map[string]interface{}{
				"issuer": "${config.policy_configurations.jwtauth.issuer}",
			},
		}
		factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")
		err = reg.Register(def, factory)
		require.NoError(t, err)

		// Create instance
		metadata := policy.PolicyMetadata{
			APIName:   "test-api",
			RouteName: "/test",
		}
		params := map[string]interface{}{
			"audience": "my-audience",
		}

		instance, mergedParams, err := reg.CreateInstance("jwt-auth", "v1.0.0", metadata, params)
		require.NoError(t, err)
		assert.NotNil(t, instance)
		assert.NotNil(t, mergedParams)

		// Check merged params contains resolved system param + runtime param
		assert.Equal(t, "https://auth.example.com", mergedParams["issuer"])
		assert.Equal(t, "my-audience", mergedParams["audience"])
	})

	t.Run("create instance without config resolver", func(t *testing.T) {
		reg := newTestRegistry()

		def := &policy.PolicyDefinition{
			Name:    "jwt-auth",
			Version: "v1.0.0",
		}
		factory := testutils.NewMockPolicyFactory("jwt-auth", "v1.0.0")
		err := reg.Register(def, factory)
		require.NoError(t, err)

		// Don't set config resolver
		metadata := policy.PolicyMetadata{}
		params := map[string]interface{}{}

		instance, _, err := reg.CreateInstance("jwt-auth", "v1.0.0", metadata, params)
		assert.Error(t, err)
		assert.Nil(t, instance)
		assert.Contains(t, err.Error(), "ConfigResolver is not initialized")
	})

	t.Run("create instance for non-existent policy", func(t *testing.T) {
		reg := newTestRegistry()
		err := reg.SetConfig(map[string]interface{}{})
		require.NoError(t, err)

		metadata := policy.PolicyMetadata{}
		params := map[string]interface{}{}

		instance, _, err := reg.CreateInstance("non-existent", "v1.0.0", metadata, params)
		assert.Error(t, err)
		assert.Nil(t, instance)
		assert.Contains(t, err.Error(), "policy factory not found")
	})

	t.Run("runtime params override system params", func(t *testing.T) {
		reg := newTestRegistry()

		config := map[string]interface{}{
			"default_timeout": 30,
		}
		err := reg.SetConfig(config)
		require.NoError(t, err)

		def := &policy.PolicyDefinition{
			Name:    "timeout",
			Version: "v1.0.0",
			SystemParameters: map[string]interface{}{
				"timeout": "${config.default_timeout}",
			},
		}
		factory := testutils.NewMockPolicyFactory("timeout", "v1.0.0")
		err = reg.Register(def, factory)
		require.NoError(t, err)

		// Override timeout with runtime param
		metadata := policy.PolicyMetadata{}
		params := map[string]interface{}{
			"timeout": 60, // Override the system param
		}

		instance, mergedParams, err := reg.CreateInstance("timeout", "v1.0.0", metadata, params)
		require.NoError(t, err)
		assert.NotNil(t, instance)
		assert.Equal(t, 60, mergedParams["timeout"])
	})

	t.Run("nil system parameters", func(t *testing.T) {
		reg := newTestRegistry()
		err := reg.SetConfig(map[string]interface{}{})
		require.NoError(t, err)

		def := &policy.PolicyDefinition{
			Name:             "simple",
			Version:          "v1.0.0",
			SystemParameters: nil,
		}
		factory := testutils.NewMockPolicyFactory("simple", "v1.0.0")
		err = reg.Register(def, factory)
		require.NoError(t, err)

		metadata := policy.PolicyMetadata{}
		params := map[string]interface{}{"key": "value"}

		instance, mergedParams, err := reg.CreateInstance("simple", "v1.0.0", metadata, params)
		require.NoError(t, err)
		assert.NotNil(t, instance)
		assert.Equal(t, "value", mergedParams["key"])
	})
}

// TestPolicyChain tests the PolicyChain struct
func TestPolicyChain(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		chain := PolicyChain{
			Policies:             []policy.Policy{},
			PolicySpecs:          []policy.PolicySpec{},
			RequiresRequestBody:  false,
			RequiresResponseBody: false,
		}

		assert.Empty(t, chain.Policies)
		assert.Empty(t, chain.PolicySpecs)
		assert.False(t, chain.RequiresRequestBody)
		assert.False(t, chain.RequiresResponseBody)
	})

	t.Run("chain with policies", func(t *testing.T) {
		chain := PolicyChain{
			Policies: []policy.Policy{
				&testutils.SimpleMockPolicy{Name: "jwt-auth", Version: "v1.0.0"},
				&testutils.SimpleMockPolicy{Name: "rate-limit", Version: "v1.0.0"},
			},
			PolicySpecs: []policy.PolicySpec{
				{Name: "jwt-auth", Version: "v1.0.0", Enabled: true},
				{Name: "rate-limit", Version: "v1.0.0", Enabled: true},
			},
			RequiresRequestBody:  true,
			RequiresResponseBody: false,
		}

		assert.Len(t, chain.Policies, 2)
		assert.Len(t, chain.PolicySpecs, 2)
		assert.True(t, chain.RequiresRequestBody)
		assert.False(t, chain.RequiresResponseBody)
	})
}
