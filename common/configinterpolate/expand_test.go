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

package configinterpolate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpand_NoOpOnTokenFreeConfig(t *testing.T) {
	raw := map[string]any{
		"controller": map[string]any{
			"logging": map[string]any{"level": "info"},
			"users": []any{
				map[string]any{"username": "admin", "roles": []any{"admin", "reader"}},
			},
		},
		"port":    int64(8080),
		"enabled": true,
		"ratio":   1.5,
		"nothing": nil,
	}

	out, stats, err := Expand(raw, Options{})
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Fields, "no fields should be counted for a token-free config")
	assert.Equal(t, raw, out, "token-free config must round-trip deep-equal")
}

func TestExpand_DoesNotMutateInput(t *testing.T) {
	t.Setenv("CI_TOK", "resolved")
	raw := map[string]any{"a": `{{ env "CI_TOK" }}`}
	_, _, err := Expand(raw, Options{})
	require.NoError(t, err)
	assert.Equal(t, `{{ env "CI_TOK" }}`, raw["a"], "input map must not be mutated")
}

func TestExpand_ArrayOfTables(t *testing.T) {
	t.Setenv("ADMIN_PW", "sup3r")
	raw := map[string]any{
		"users": []any{
			map[string]any{"username": "admin", "password": `{{ env "ADMIN_PW" }}`},
			map[string]any{"username": "guest", "password": `{{ env "GUEST_PW" "open" }}`},
		},
	}

	out, stats, err := Expand(raw, Options{})
	require.NoError(t, err)

	users := out["users"].([]any)
	assert.Equal(t, "sup3r", users[0].(map[string]any)["password"])
	assert.Equal(t, "open", users[1].(map[string]any)["password"])
	assert.Equal(t, 2, stats.EnvRefs)
	assert.Equal(t, 2, stats.Fields)
}

func TestExpand_Stats(t *testing.T) {
	t.Setenv("H", "example.com")
	raw := map[string]any{
		"url":   `https://{{ env "H" }}/v1`,
		"plain": "no tokens here",
		"port":  int64(9090),
	}
	_, stats, err := Expand(raw, Options{})
	require.NoError(t, err)
	assert.Equal(t, Stats{EnvRefs: 1, FileRefs: 0, Fields: 1}, stats)
}

func TestExpand_PartialSubstitution(t *testing.T) {
	t.Setenv("H", "api.example.com")
	t.Setenv("P", "8443")
	out, _, err := Expand(map[string]any{"u": `https://{{ env "H" }}:{{ env "P" }}/`}, Options{})
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com:8443/", out["u"])
}

func TestExpand_LeavesCELDollarConfigUntouched(t *testing.T) {
	// The policy-engine uses ${config.x} (dollar-brace) at request time. Config
	// interpolation uses {{ }}; it must not touch ${...}.
	raw := map[string]any{"expr": "${config.jwtauth.allowedalgorithms}"}
	out, stats, err := Expand(raw, Options{})
	require.NoError(t, err)
	assert.Equal(t, "${config.jwtauth.allowedalgorithms}", out["expr"])
	assert.Equal(t, 0, stats.Fields)
}

func TestExpand_EscapedLiteralBraces(t *testing.T) {
	// {{ "{{" }} renders a literal "{{" — lets a config carry a literal token.
	out, _, err := Expand(map[string]any{"lit": `{{ "{{" }} env "X" }}`}, Options{})
	require.NoError(t, err)
	assert.Equal(t, `{{ env "X" }}`, out["lit"])
}

func TestExpand_ScalarsPassThrough(t *testing.T) {
	raw := map[string]any{"i": int64(1), "f": 2.5, "b": false, "n": nil}
	out, _, err := Expand(raw, Options{})
	require.NoError(t, err)
	assert.Equal(t, raw, out)
}

func TestExpand_ExecErrorCarriesFieldPath(t *testing.T) {
	raw := map[string]any{
		"controller": map[string]any{
			"controlplane": map[string]any{"token": `{{ env "MISSING_TOKEN" }}`},
		},
	}
	_, _, err := Expand(raw, Options{})
	require.Error(t, err)

	var execErr *ExecError
	require.True(t, errors.As(err, &execErr), "expected *ExecError, got %T", err)
	assert.Equal(t, "controller.controlplane.token", execErr.Field)
	assert.Contains(t, err.Error(), `required env var "MISSING_TOKEN" is not found`)
}

func TestExpand_ParseError(t *testing.T) {
	// Unknown function -> parse error, surfaced without Go's "template:" prefix.
	_, _, err := Expand(map[string]any{"x": `{{ nope "y" }}`}, Options{})
	require.Error(t, err)

	var parseErr *ParseError
	require.True(t, errors.As(err, &parseErr), "expected *ParseError in chain, got %v", err)
	assert.Contains(t, err.Error(), `function "nope" not defined`)
	assert.NotContains(t, err.Error(), "template:")
}

func TestResolveAllowlist(t *testing.T) {
	defaults := []string{"/etc/gateway-controller", "/secrets/gateway-controller"}

	t.Run("unset returns defaults", func(t *testing.T) {
		t.Setenv(EnvFileSourceAllowlist, "")
		assert.Equal(t, defaults, ResolveAllowlist(defaults))
	})

	t.Run("set replaces defaults and trims blanks", func(t *testing.T) {
		t.Setenv(EnvFileSourceAllowlist, " /a , ,/b/c ")
		assert.Equal(t, []string{"/a", "/b/c"}, ResolveAllowlist(defaults))
	})

	t.Run("only-blanks falls back to defaults", func(t *testing.T) {
		t.Setenv(EnvFileSourceAllowlist, " , , ")
		assert.Equal(t, defaults, ResolveAllowlist(defaults))
	})
}
