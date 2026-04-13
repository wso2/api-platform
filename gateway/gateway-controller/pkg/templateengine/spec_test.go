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

package templateengine

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

// mockResolver is a test double for funcs.SecretResolver.
type mockResolver struct {
	secrets map[string]string
}

func (m *mockResolver) Resolve(handle string) (string, error) {
	val, ok := m.secrets[handle]
	if !ok {
		return "", fmt.Errorf("secret %q not found", handle)
	}
	return val, nil
}

// testConfig mimics the structure of generated API config types.
type testConfig struct {
	ApiVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   testMetadata `json:"metadata"`
	Spec       testSpec     `json:"spec"`
}

type testMetadata struct {
	Name string `json:"name"`
}

type testSpec struct {
	DisplayName string       `json:"displayName"`
	Upstream    testUpstream `json:"upstream"`
}

type testUpstream struct {
	URL    string  `json:"url"`
	APIKey *string `json:"apiKey,omitempty"`
}

func TestRenderSpec_ResolvesSecretInSpec(t *testing.T) {
	resolver := &mockResolver{secrets: map[string]string{"backend-key": "sk-12345"}}
	apiKey := `{{ secret "backend-key" }}`
	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: "test-api"},
		Spec: testSpec{
			DisplayName: "Test API",
			Upstream: testUpstream{
				URL:    "http://localhost:8080",
				APIKey: &apiKey,
			},
		},
	}

	result, err := RenderSpec(config, resolver, slog.Default())
	require.NoError(t, err)

	rendered, ok := result.Config.(testConfig)
	require.True(t, ok)

	// Spec fields should be resolved
	require.NotNil(t, rendered.Spec.Upstream.APIKey)
	assert.Equal(t, "sk-12345", *rendered.Spec.Upstream.APIKey)

	// Metadata should be untouched
	assert.Equal(t, "test-api", rendered.Metadata.Name)
	assert.Equal(t, "v1", rendered.ApiVersion)

	// Tracker should contain the secret
	vals := result.Tracker.Values()
	assert.Equal(t, 1, len(vals))
	assert.Equal(t, "sk-12345", vals[0])
}

func TestRenderSpec_ResolvesEnvInSpec(t *testing.T) {
	t.Setenv("TEST_BACKEND_URL", "http://backend:9090")

	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: "test-api"},
		Spec: testSpec{
			DisplayName: "Test API",
			Upstream: testUpstream{
				URL: `{{ env "TEST_BACKEND_URL" | default "http://localhost:8080" }}`,
			},
		},
	}

	result, err := RenderSpec(config, &mockResolver{}, slog.Default())
	require.NoError(t, err)

	rendered := result.Config.(testConfig)
	assert.Equal(t, "http://backend:9090", rendered.Spec.Upstream.URL)
}

func TestRenderSpec_DefaultFallback(t *testing.T) {
	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: "test-api"},
		Spec: testSpec{
			DisplayName: "Test API",
			Upstream: testUpstream{
				URL: `{{ env "NONEXISTENT_URL_12345" | default "http://fallback:8080" }}`,
			},
		},
	}

	result, err := RenderSpec(config, &mockResolver{}, slog.Default())
	require.NoError(t, err)

	rendered := result.Config.(testConfig)
	assert.Equal(t, "http://fallback:8080", rendered.Spec.Upstream.URL)
}

func TestRenderSpec_NoSpecField(t *testing.T) {
	type noSpecConfig struct {
		Kind string `json:"kind"`
	}
	config := noSpecConfig{Kind: "Test"}

	result, err := RenderSpec(config, &mockResolver{}, slog.Default())
	require.NoError(t, err)

	rendered := result.Config.(noSpecConfig)
	assert.Equal(t, "Test", rendered.Kind)
	assert.Empty(t, result.Tracker.Values())
}

func TestRenderSpec_SecretNotFound(t *testing.T) {
	resolver := &mockResolver{secrets: map[string]string{}}
	apiKey := `{{ secret "missing-key" }}`
	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: "test-api"},
		Spec: testSpec{
			DisplayName: "Test API",
			Upstream: testUpstream{
				APIKey: &apiKey,
			},
		},
	}

	_, err := RenderSpec(config, resolver, slog.Default())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing-key")
}

func TestRenderSpec_RequiredFailure(t *testing.T) {
	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: "test-api"},
		Spec: testSpec{
			DisplayName: "Test API",
			Upstream: testUpstream{
				URL: `{{ env "NONEXISTENT_12345" | required "BACKEND_URL must be set" }}`,
			},
		},
	}

	_, err := RenderSpec(config, &mockResolver{}, slog.Default())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "BACKEND_URL must be set")
}

func TestRenderSpec_MetadataNotRendered(t *testing.T) {
	// Even if metadata contains template-like syntax, it should NOT be rendered
	// because only spec is processed. However, since we marshal the whole config
	// to JSON and only render the spec portion, metadata is untouched.
	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: `{{ env "SHOULD_NOT_RESOLVE" }}`},
		Spec: testSpec{
			DisplayName: "Test API",
			Upstream: testUpstream{
				URL: "http://localhost",
			},
		},
	}

	result, err := RenderSpec(config, &mockResolver{}, slog.Default())
	require.NoError(t, err)

	rendered := result.Config.(testConfig)
	// Metadata should retain the literal template expression
	assert.Equal(t, `{{ env "SHOULD_NOT_RESOLVE" }}`, rendered.Metadata.Name)
}

// Verify RenderSpec works with a nil SecretResolver when no secrets are used.
func TestRenderSpec_NilResolverNoSecrets(t *testing.T) {
	config := testConfig{
		ApiVersion: "v1",
		Kind:       "TestApi",
		Metadata:   testMetadata{Name: "test"},
		Spec: testSpec{
			DisplayName: "Test",
			Upstream:    testUpstream{URL: "http://localhost"},
		},
	}

	result, err := RenderSpec(config, nil, slog.Default())
	require.NoError(t, err)

	rendered := result.Config.(testConfig)
	assert.Equal(t, "http://localhost", rendered.Spec.Upstream.URL)
}

// Verify the SecretResolver interface is satisfied by the mock.
var _ funcs.SecretResolver = (*mockResolver)(nil)
