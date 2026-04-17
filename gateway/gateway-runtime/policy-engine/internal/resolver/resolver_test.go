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

package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteKeyResolver_Name verifies the resolver reports the correct name.
func TestRouteKeyResolver_Name(t *testing.T) {
	r := &RouteKeyResolver{}
	assert.Equal(t, "route-key", r.Name())
}

// TestRouteKeyResolver_Requirements verifies the resolver reports no buffering or headers needed.
func TestRouteKeyResolver_Requirements(t *testing.T) {
	r := &RouteKeyResolver{}
	reqs := r.Requirements()
	assert.False(t, reqs.BufferBody)
	assert.False(t, reqs.Headers)
}

// TestRouteKeyResolver_Resolve verifies the resolver returns the route key unchanged.
func TestRouteKeyResolver_Resolve(t *testing.T) {
	r := &RouteKeyResolver{}

	tests := []struct {
		name     string
		routeKey string
	}{
		{"simple route key", "GET|/api/v1/users|example.com"},
		{"empty route key", ""},
		{"route key with special chars", "POST|/path/{id}|host.local:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ResolverContext{RouteKey: tt.routeKey}
			got, err := r.Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.routeKey, got)
		})
	}
}

// TestRegister_And_Get verifies that resolvers can be registered and retrieved.
func TestRegister_And_Get(t *testing.T) {
	// Clean up: save and restore Registry state
	original := Registry
	Registry = map[string]PolicyChainResolver{}
	defer func() { Registry = original }()

	mock := &RouteKeyResolver{}
	Register(mock)

	got, err := Get("route-key")
	require.NoError(t, err)
	assert.Equal(t, mock, got)
}

// TestGet_NotFound verifies that Get returns an error for unknown resolvers.
func TestGet_NotFound(t *testing.T) {
	original := Registry
	Registry = map[string]PolicyChainResolver{}
	defer func() { Registry = original }()

	_, err := Get("nonexistent-resolver")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-resolver")
}

// TestInit_RegistersRouteKeyResolver verifies that the init function registers the built-in resolver.
func TestInit_RegistersRouteKeyResolver(t *testing.T) {
	// The package init() should have already run; the route-key resolver should be present.
	r, err := Get("route-key")
	require.NoError(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, "route-key", r.Name())
}

// TestResolverContext_FieldAccess verifies that ResolverContext fields are accessible.
func TestResolverContext_FieldAccess(t *testing.T) {
	headers := map[string][]string{
		"Authorization": {"Bearer token"},
	}
	ctx := ResolverContext{
		RouteKey: "GET|/test|host.local",
		Headers:  headers,
		Body:     []byte(`{"key":"value"}`),
	}

	assert.Equal(t, "GET|/test|host.local", ctx.RouteKey)
	assert.Equal(t, []string{"Bearer token"}, ctx.Headers["Authorization"])
	assert.Equal(t, []byte(`{"key":"value"}`), ctx.Body)
}
