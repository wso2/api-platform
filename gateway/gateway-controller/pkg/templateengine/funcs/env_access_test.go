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
	"errors"
	"fmt"
	"os"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain admits the variable names the package's existing env tests use;
// production is deny-all until the controller installs its configured policy.
func TestMain(m *testing.M) {
	policy, err := NewEnvAccessPolicy([]string{"TEST_*", "NONEXISTENT_*", "SENSITIVE_*"})
	if err != nil {
		panic(err)
	}
	SetEnvAccessPolicy(policy)
	os.Exit(m.Run())
}

// withEnvPolicy installs a policy for one test and restores the previous one.
func withEnvPolicy(t *testing.T, entries ...string) {
	t.Helper()
	policy, err := NewEnvAccessPolicy(entries)
	require.NoError(t, err)
	prev := SetEnvAccessPolicy(policy)
	t.Cleanup(func() { SetEnvAccessPolicy(prev) })
}

func executeTemplate(t *testing.T, tmplStr string) (string, error) {
	t.Helper()
	tmpl, err := template.New("test").Funcs(BuildFuncMap(newTestDeps(nil))).Parse(tmplStr)
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	return buf.String(), err
}

func TestNewEnvAccessPolicy_RejectsMalformedEntries(t *testing.T) {
	for _, entry := range []string{
		"*",       // no name before the '*'
		"FOO*BAR", // '*' not at the end
		"FOO**",   // more than one '*'
		"1BAD",    // not a variable name
		"BAD-NAME",
		"BAD NAME",
	} {
		t.Run(entry, func(t *testing.T) {
			_, err := NewEnvAccessPolicy([]string{entry})
			assert.Error(t, err)
		})
	}
}

func TestEnvAccessPolicy_Allows(t *testing.T) {
	policy, err := NewEnvAccessPolicy([]string{" BACKEND_URL ", "", "APP_*"})
	require.NoError(t, err)

	assert.False(t, policy.Unrestricted())
	assert.True(t, policy.Allows("BACKEND_URL"))
	assert.True(t, policy.Allows("APP_"))
	assert.True(t, policy.Allows("APP_TIMEOUT"))
	assert.False(t, policy.Allows("APPLE"))
	assert.False(t, policy.Allows("backend_url"), "names are case-sensitive")
	assert.False(t, policy.Allows("HOME"))
	assert.False(t, policy.Allows("APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN"))

	exact, prefixes := policy.Counts()
	assert.Equal(t, 1, exact)
	assert.Equal(t, 1, prefixes)
}

func TestEnvAccessPolicy_NilAndEmptyDenyAll(t *testing.T) {
	var nilPolicy *EnvAccessPolicy
	assert.False(t, nilPolicy.Allows("HOME"))

	empty, err := NewEnvAccessPolicy(nil)
	require.NoError(t, err)
	assert.False(t, empty.Allows("HOME"))
}

func TestEnvAccessPolicy_UnrestrictedAllowsEverything(t *testing.T) {
	policy := UnrestrictedEnvAccessPolicy()
	assert.True(t, policy.Unrestricted())
	assert.True(t, policy.Allows("HOME"))
	assert.True(t, policy.Allows("ANY_NAME"))
}

func TestEnv_UnrestrictedPolicyReadsAnyVariable(t *testing.T) {
	prev := SetEnvAccessPolicy(UnrestrictedEnvAccessPolicy())
	t.Cleanup(func() { SetEnvAccessPolicy(prev) })
	t.Setenv("BACKEND_URL", "http://backend")

	out, err := executeTemplate(t, `{{ env "BACKEND_URL" }}`)
	require.NoError(t, err)
	assert.Equal(t, "http://backend", out)
}

func TestEnv_UnsetPolicyDeniesAll(t *testing.T) {
	prev := SetEnvAccessPolicy(nil)
	t.Cleanup(func() { SetEnvAccessPolicy(prev) })
	t.Setenv("TEST_ENV_VAR", "hello")

	_, err := executeTemplate(t, `{{ env "TEST_ENV_VAR" }}`)
	var envErr *EnvAccessError
	assert.ErrorAs(t, err, &envErr)
}

func TestEnv_DisallowedVariableIsRefusedWithoutLeaking(t *testing.T) {
	withEnvPolicy(t, "BACKEND_URL")
	t.Setenv("APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN", "cp-token-value")
	t.Setenv("DB_PASSWORD", "db-password-value")

	for _, name := range []string{"APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN", "DB_PASSWORD"} {
		t.Run(name, func(t *testing.T) {
			out, err := executeTemplate(t, `{{ env "`+name+`" }}`)
			var envErr *EnvAccessError
			require.ErrorAs(t, err, &envErr)
			assert.Empty(t, out)
			assert.NotContains(t, envErr.Error(), name)
			assert.NotContains(t, err.Error(), "value")
		})
	}
}

func TestEnv_DisallowedSetAndUnsetAreIndistinguishable(t *testing.T) {
	withEnvPolicy(t, "BACKEND_URL")
	t.Setenv("EXISTING_SECRET", "s3cr3t")

	_, errSet := executeTemplate(t, `{{ env "EXISTING_SECRET" }}`)
	_, errUnset := executeTemplate(t, `{{ env "MISSING_SECRET_12345" }}`)

	var setErr, unsetErr *EnvAccessError
	require.True(t, errors.As(errSet, &setErr))
	require.True(t, errors.As(errUnset, &unsetErr))
	assert.Equal(t, setErr.Error(), unsetErr.Error())
}

func TestEnv_AllowedButUnsetFallsBackToDefault(t *testing.T) {
	withEnvPolicy(t, "BACKEND_URL")
	// t.Setenv registers restoration of any pre-existing value; then unset it so
	// the test does not depend on the caller's environment.
	t.Setenv("BACKEND_URL", "")
	require.NoError(t, os.Unsetenv("BACKEND_URL"))
	out, err := executeTemplate(t, `{{ env "BACKEND_URL" | default "http://fallback" }}`)
	require.NoError(t, err)
	assert.Equal(t, "http://fallback", out)
}

func TestEnv_AllowedByPrefix(t *testing.T) {
	withEnvPolicy(t, "BACKEND_*")
	t.Setenv("BACKEND_HOST", "backend.internal")
	out, err := executeTemplate(t, `{{ env "BACKEND_HOST" }}`)
	require.NoError(t, err)
	assert.Equal(t, "backend.internal", out)
}

func TestIsEnvAccessError(t *testing.T) {
	wrapped := fmt.Errorf("render config: %w", fmt.Errorf("execute template: %w", &EnvAccessError{}))
	assert.True(t, IsEnvAccessError(wrapped))
	assert.True(t, IsEnvAccessError(&EnvAccessError{}))
	assert.False(t, IsEnvAccessError(&SecretError{Key: "k", Cause: errors.New("missing")}))
	assert.False(t, IsEnvAccessError(errors.New("other")))
	assert.False(t, IsEnvAccessError(nil))
}
