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

package apiportal

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

func TestAPIPortalDefinition(t *testing.T) {
	definition := APIPortal()
	require.Equal(t, "api-portal", definition.Name)
	require.True(t, definition.IsCompose())
	require.Equal(t, []string{"platform-api"}, definition.DependsOn)
	require.NotNil(t, definition.DB)
	require.NotEmpty(t, definition.Compose.GeneratedFiles)
	for _, key := range []string{
		"APIP_AP_SECURITY_ENCRYPTION_KEY",
		"APIP_AP_SECURITY_SESSION_SECRET",
	} {
		value, ok := definition.Compose.Env[key]
		require.True(t, ok, "%s must be injected into the portal runtime", key)
		require.Len(t, value, 64, "%s must be a 32-byte hex secret", key)
		_, err := hex.DecodeString(value)
		require.NoError(t, err, "%s must be hexadecimal", key)
	}
	_, ok := definition.Endpoint("http")
	require.True(t, ok)
}

func TestAPIPortalOtherOrgUsesBaseComposeWithOrganizationOverride(t *testing.T) {
	definition := APIPortalOtherOrg()
	require.Equal(t, "api-portal-other-org", definition.Name)
	require.Equal(t, "api-portal", definition.Compose.PrimaryService)
	require.Equal(t, []string{"api-portal"}, definition.Compose.Services)
	require.Equal(t, []string{"tests/framework/core/catalog/apiportal/docker-compose.other-org.yaml"},
		definition.Compose.ComposeOverrideFiles)
	require.Equal(t, "api-portal-other-org", definition.Compose.CoverageServices[0].OutputName)
}

func TestAPIPortalCoverageEnvironmentFollowsRunMode(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	require.NotContains(t, APIPortal().Compose.Env, "NODE_V8_COVERAGE")

	t.Setenv(shared.EnvCoverageMode, "true")
	require.Equal(t, "/coverage", APIPortal().Compose.Env["NODE_V8_COVERAGE"])
}

func TestAPIPortalBuildDeclaresBrowserCoverage(t *testing.T) {
	spec, err := BuildSpec("test")
	require.NoError(t, err)
	require.Contains(t, spec.Coverage.Types, builder.NodeV8Coverage)
	require.Contains(t, spec.Coverage.Types, builder.BrowserJSCoverage)
	require.Equal(t, "portals/api-portal/src/scripts", spec.Coverage.Browser.SourceRoot)
	require.NotEmpty(t, spec.Coverage.Browser.Include)
}

func TestAPIPortalBuildIncludesCoverageScriptsContext(t *testing.T) {
	spec, err := BuildSpec("test")
	require.NoError(t, err)
	commands, err := spec.Plan("/repo", "test", spec.Coverage)
	require.NoError(t, err)
	require.Len(t, commands, 1)
	require.Contains(t, strings.Join(commands[0].Args, " "),
		"--build-context coverage-scripts=../../tests/framework/tools")
}

func TestAPIPortalSupportsPostgresAndSQLServer(t *testing.T) {
	definition := APIPortal()
	for _, engine := range []components.DBType{components.Postgres, components.SQLServer} {
		require.True(t, definition.DB.Supports(engine), engine)
		schema, ok := definition.DB.SchemaFor(engine)
		require.True(t, ok, engine)
		require.Len(t, schema, 1)
	}
	require.False(t, definition.DB.Supports(components.SQLite))

	dsn := components.DSN{Host: "db", Port: 1433, Database: "portal", User: "u", Password: "p"}
	dsn.Type = components.Postgres
	require.Equal(t, "postgres", definition.DB.Env(dsn)["APIP_AP_DATABASE_DRIVER"])
	dsn.Type = components.SQLServer
	env := definition.DB.Env(dsn)
	require.Equal(t, "mssql", env["APIP_AP_DATABASE_DRIVER"])
	require.Equal(t, "1433", env["APIP_AP_DATABASE_PORT"])
	require.Equal(t, "portal", env["APIP_AP_DATABASE_NAME"])
}

func TestTheMultiTenancyPortalOwnsItsDatabaseAndSignsInThroughTheTestbench(t *testing.T) {
	definition := APIPortalMultiTenancy()
	require.True(t, definition.DB.Owns())
	require.Equal(t, []string{"testbench"}, definition.DependsOn,
		"the multi-tenancy portal signs in through the testbench, not platform-api")
	require.Equal(t, MultiTenancyPortalID, definition.Compose.Env["APIP_AP_ORGANIZATION_PORTAL_ID"])
	got, ok := PortalID(definition.Name)
	require.True(t, ok)
	require.Equal(t, MultiTenancyPortalID, got)
	_, ok = PortalID("api-portal")
	require.False(t, ok)
}

func TestTheMultiTenancyPortalTrustsTheIdentityProvider(t *testing.T) {
	for _, definition := range []*components.Definition{APIPortalMultiTenancy()} {
		bundle := string(definition.Compose.GeneratedFiles["certs/cert.pem"])
		require.Contains(t, bundle, string(shared.ControlPlaneCrypto()["certs/cert.pem"]), definition.Name)
		require.Contains(t, bundle, string(shared.IdentityProviderTLS().CertPEM), definition.Name)
		require.Equal(t, 2, strings.Count(bundle, "BEGIN CERTIFICATE"), definition.Name)

		require.Equal(t, "api-portal", definition.Product(), "%s is built from the api-portal source", definition.Name)
		require.Equal(t, []string{multiTenancyOverlay}, definition.Config.ExtraOverlays, definition.Name)
		require.Equal(t, portalRoleMapping, definition.Compose.StagedFiles["role-to-scope-mapping.yaml"], definition.Name)
		require.Equal(t, "http://"+definition.Name+":9543/api-portal/default/callback",
			definition.Compose.Env["APIP_AP_AUTH_IDP_CALLBACK_URL"], definition.Name)
		require.Equal(t, definition.Name, definition.Alias)
		require.Equal(t, "/health", definition.Health.Path, "%s must not probe a page that redirects to sign-in", definition.Name)
	}
	require.Equal(t, "/", APIPortal().Health.Path, "the default portal keeps its own readiness probe")
	require.NotContains(t, string(APIPortal().Compose.GeneratedFiles["certs/cert.pem"]),
		string(shared.IdentityProviderTLS().CertPEM), "the default portal keeps trusting only the control plane")
}
