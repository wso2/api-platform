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

package funcs

import (
	"bytes"
	"fmt"
	"log/slog"
	"sort"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/redact"
)

// mockSecretResolver is a test double for SecretResolver.
type mockSecretResolver struct {
	secrets map[string]string
}

func (m *mockSecretResolver) Resolve(handle string) (string, error) {
	val, ok := m.secrets[handle]
	if !ok {
		return "", fmt.Errorf("secret %q not found", handle)
	}
	return val, nil
}

func newTestDeps(secrets map[string]string) *Deps {
	return &Deps{
		SecretResolver: &mockSecretResolver{secrets: secrets},
		Tracker:        redact.NewSecretTracker(),
		Logger:         slog.Default(),
	}
}

func renderTemplate(t *testing.T, tmplStr string, deps *Deps) string {
	t.Helper()
	fm := BuildFuncMap(deps)
	tmpl, err := template.New("test").Funcs(fm).Parse(tmplStr)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tmpl.Execute(&buf, nil))
	return buf.String()
}

// --- env ---

func TestEnv_Found(t *testing.T) {
	t.Setenv("TEST_ENV_VAR", "hello")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_ENV_VAR" }}`, deps)
	assert.Equal(t, "hello", result)
}

func TestEnv_NotFound(t *testing.T) {
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "NONEXISTENT_VAR_12345" }}`, deps)
	assert.Equal(t, "", result)
}

// --- secret ---

func TestSecret_Found(t *testing.T) {
	deps := newTestDeps(map[string]string{"my-api-key": "sk-secret-123"})
	result := renderTemplate(t, `{{ secret "my-api-key" }}`, deps)
	assert.Equal(t, "sk-secret-123", result)

	// Verify tracker recorded the value
	vals := deps.Tracker.Values()
	assert.Equal(t, []string{"sk-secret-123"}, vals)
}

func TestSecret_NotFound(t *testing.T) {
	deps := newTestDeps(map[string]string{})
	fm := BuildFuncMap(deps)
	tmpl, err := template.New("test").Funcs(fm).Parse(`{{ secret "missing" }}`)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestSecret_NilResolver(t *testing.T) {
	deps := &Deps{
		SecretResolver: nil,
		Tracker:        redact.NewSecretTracker(),
		Logger:         slog.Default(),
	}
	fm := BuildFuncMap(deps)
	tmpl, err := template.New("test").Funcs(fm).Parse(`{{ secret "key" }}`)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

// --- redact ---

func TestRedact_TracksValue(t *testing.T) {
	t.Setenv("SENSITIVE_TOKEN", "bearer-xyz")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "SENSITIVE_TOKEN" | redact }}`, deps)
	assert.Equal(t, "bearer-xyz", result)

	vals := deps.Tracker.Values()
	assert.Equal(t, []string{"bearer-xyz"}, vals)
}

// --- default ---

func TestDefault_UsesValue(t *testing.T) {
	t.Setenv("TEST_DEFAULT_VAR", "actual-value")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_DEFAULT_VAR" | default "fallback" }}`, deps)
	assert.Equal(t, "actual-value", result)
}

func TestDefault_UsesFallback(t *testing.T) {
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "NONEXISTENT_VAR_12345" | default "fallback" }}`, deps)
	assert.Equal(t, "fallback", result)
}

// --- required ---

func TestRequired_ValuePresent(t *testing.T) {
	t.Setenv("TEST_REQ_VAR", "present")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_REQ_VAR" | required "must be set" }}`, deps)
	assert.Equal(t, "present", result)
}

func TestRequired_ValueMissing(t *testing.T) {
	deps := newTestDeps(nil)
	fm := BuildFuncMap(deps)
	tmpl, err := template.New("test").Funcs(fm).Parse(`{{ env "NONEXISTENT_VAR_12345" | required "VAR must be set" }}`)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VAR must be set")
}

// --- string helpers ---

func TestUpper(t *testing.T) {
	t.Setenv("TEST_CASE_VAR", "hello")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_CASE_VAR" | upper }}`, deps)
	assert.Equal(t, "HELLO", result)
}

func TestLower(t *testing.T) {
	t.Setenv("TEST_CASE_VAR", "HELLO")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_CASE_VAR" | lower }}`, deps)
	assert.Equal(t, "hello", result)
}

func TestTrim(t *testing.T) {
	t.Setenv("TEST_TRIM_VAR", "  spaced  ")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_TRIM_VAR" | trim }}`, deps)
	assert.Equal(t, "spaced", result)
}

func TestReplace(t *testing.T) {
	t.Setenv("TEST_REPLACE_VAR", "hello-world-test")
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "TEST_REPLACE_VAR" | replace "-" "_" }}`, deps)
	assert.Equal(t, "hello_world_test", result)
}

// --- pipe chains ---

func TestPipeChain_EnvDefaultUpper(t *testing.T) {
	deps := newTestDeps(nil)
	result := renderTemplate(t, `{{ env "NONEXISTENT_VAR_12345" | default "fallback" | upper }}`, deps)
	assert.Equal(t, "FALLBACK", result)
}

func TestPipeChain_SecretRedact(t *testing.T) {
	deps := newTestDeps(map[string]string{"token": "my-secret-token"})
	result := renderTemplate(t, `{{ secret "token" }}`, deps)
	assert.Equal(t, "my-secret-token", result)

	// secret auto-tracks, verify
	vals := deps.Tracker.Values()
	assert.Equal(t, []string{"my-secret-token"}, vals)
}

// --- registry ---

func TestBuildFuncMap_AllFunctionsRegistered(t *testing.T) {
	deps := newTestDeps(nil)
	fm := BuildFuncMap(deps)

	expectedFuncs := []string{
		"env", "secret", "redact", "default", "required",
		"upper", "lower", "trim", "replace",
	}
	sort.Strings(expectedFuncs)

	var registered []string
	for name := range fm {
		registered = append(registered, name)
	}
	sort.Strings(registered)

	assert.Equal(t, expectedFuncs, registered)
}
