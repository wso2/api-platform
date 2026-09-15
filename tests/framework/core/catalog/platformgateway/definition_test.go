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

package platformgateway

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
)

// composeFile is the gateway stack the suite actually runs.
func composeFile(t *testing.T) map[string]any {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	path := filepath.Join(filepath.Dir(thisFile), "docker-compose.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "cannot read %s", path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &doc), "%s is not valid YAML", path)
	return doc
}

// composeServiceEnv returns a service's `environment:` list as a map.
//
// Only the KEY=VALUE list form is understood, which is the form this file uses. A mapping-form
// environment block would silently return nothing, so the caller asserts non-emptiness rather
// than trusting a zero result.
func composeServiceEnv(t *testing.T, service string) map[string]string {
	t.Helper()

	doc := composeFile(t)
	services, ok := doc["services"].(map[string]any)
	require.True(t, ok, "compose file has no services map")

	svc, ok := services[service].(map[string]any)
	require.True(t, ok, "compose file has no %q service", service)

	env := map[string]string{}
	entries, ok := svc["environment"].([]any)
	if !ok {
		return env
	}
	for _, e := range entries {
		s, ok := e.(string)
		if !ok {
			continue
		}
		if k, v, found := strings.Cut(s, "="); found {
			env[k] = v
		}
	}
	return env
}

// TestComposeCarriesTheControllerITVars verifies parity between the service definition and
// the Compose stack used by integration tests.
func TestComposeCarriesTheControllerITVars(t *testing.T) {
	composeEnv := composeServiceEnv(t, "gateway-controller")
	require.NotEmpty(t, composeEnv,
		"the gateway-controller service has no parseable environment: list — if it was "+
			"converted to the mapping form, composeServiceEnv needs updating, otherwise this "+
			"test silently guards nothing")

	definitionEnv := GatewayController().Env

	for key, want := range definitionEnv {
		if !strings.HasPrefix(key, "IT_") {
			continue
		}
		got, present := composeEnv[key]
		require.True(t, present,
			"%s is missing from docker-compose.yaml", key)
		require.Equal(t, want, got,
			"%s disagrees between GatewayController() (%q) and the compose file (%q)",
			key, want, got)
	}
}

func TestPlatformGatewayDefinition(t *testing.T) {
	definition := PlatformGateway()
	require.Equal(t, "platform-gateway", definition.Name)
	require.True(t, definition.IsCompose())
	require.Equal(t, "gateway-runtime", definition.Compose.PrimaryService)
	require.ElementsMatch(t, []string{"gateway-controller", "gateway-runtime"}, definition.Compose.Services)
	for _, endpoint := range []string{"http", "https", "rest", "admin"} {
		_, ok := definition.Endpoint(endpoint)
		require.True(t, ok, endpoint)
	}
}

func TestPlatformGatewayVersionUpdatesBothServices(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	definition := PlatformGateway().WithImageVersion("legacy")

	require.Equal(t, "ghcr.io/wso2/api-platform/gateway-controller:legacy",
		definition.Compose.Env["PG_CONTROLLER_IMAGE"])
	require.Equal(t, "ghcr.io/wso2/api-platform/gateway-runtime:legacy",
		definition.Compose.Env["PG_RUNTIME_IMAGE"])
}

// TestComposeRuntimeBoundsItsShutdownDrain verifies the configured graceful shutdown period.
//
// Asserted on the compose file rather than trusted to a comment because the failure is silent —
// tests still pass, teardown just costs a forced 10s per container and no process exits cleanly.
func TestComposeRuntimeBoundsItsShutdownDrain(t *testing.T) {
	env := composeServiceEnv(t, "gateway-runtime")

	drain, ok := env["ROUTER_DRAIN_TIME_SECONDS"]
	require.True(t, ok,
		"gateway-runtime does not set ROUTER_DRAIN_TIME_SECONDS. The entrypoint defaults it to "+
			"15s, which does not fit docker's 10s stop timeout, so every teardown is a SIGKILL")
	require.Equal(t, "2", drain,
		"ROUTER_DRAIN_TIME_SECONDS is %q; it must stay well inside docker's stop timeout", drain)

	doc := composeFile(t)
	services := doc["services"].(map[string]any)
	runtime := services["gateway-runtime"].(map[string]any)
	require.Equal(t, "30s", runtime["stop_grace_period"],
		"gateway-runtime must raise stop_grace_period above the drain, so a later increase to "+
			"ROUTER_DRAIN_TIME_SECONDS cannot silently reintroduce the mid-drain SIGKILL")
}

func TestPlatformGatewayCoverageEnvironmentFollowsRunMode(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	require.NotContains(t, PlatformGateway().Compose.Env, "GOCOVERDIR")

	t.Setenv(shared.EnvCoverageMode, "true")
	require.Equal(t, "/coverage", PlatformGateway().Compose.Env["GOCOVERDIR"])
}
