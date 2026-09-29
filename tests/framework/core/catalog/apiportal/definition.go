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
	"strconv"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvImageAPIPortal names the environment variable used to override the API Portal image.
const EnvImageAPIPortal = "AP_IMAGE"

const svcAPIPortal = "api-portal"
const svcAPIPortalOtherOrg = "api-portal-other-org"

const svcAPIPortalMultiTenancy = "api-portal-multi-tenancy"

// MultiTenancyPortalID is the portal_id the multi-tenancy portal serves.
const MultiTenancyPortalID = "portal_id"

// multiTenancyOverlay configures IDP sign-in through the testbench identity provider and
// multi-tenancy mode.
const multiTenancyOverlay = "tests/framework/core/catalog/overlays/api-portal-multi-tenancy.toml"

// portalRoleMapping is the API Portal's own role-to-scope mapping, which also grants the
// platform-api-system role that shared-key publishing calls are authorized as.
const portalRoleMapping = "portals/api-portal/resources/role-to-scope-mapping.yaml"

// APIPortal returns the API Portal component definition.
func APIPortal() *components.Definition {
	return apiPortalDefinition(svcAPIPortal, "tests/framework/core/catalog/apiportal/docker-compose.yaml", "default", "Default", "portal_id", svcAPIPortal)
}

// APIPortalOtherOrg returns an API Portal instance pinned to another organization.
func APIPortalOtherOrg() *components.Definition {
	return apiPortalDefinition(svcAPIPortalOtherOrg, "tests/framework/core/catalog/apiportal/docker-compose.yaml", "other-org", "Other Org", "other_portal_id", svcAPIPortal, "tests/framework/core/catalog/apiportal/docker-compose.other-org.yaml")
}

// APIPortalMultiTenancy returns an API Portal in multi-tenancy mode that signs users in
// through the testbench identity provider.
func APIPortalMultiTenancy() *components.Definition {
	d := apiPortalDefinition(svcAPIPortalMultiTenancy, "tests/framework/core/catalog/apiportal/docker-compose.yaml",
		"default", "Default", MultiTenancyPortalID, svcAPIPortal)
	d.SourceProduct = svcAPIPortal
	d.Compose.Env["APIP_AP_AUTH_IDP_CALLBACK_URL"] = "http://" + svcAPIPortalMultiTenancy + ":9543/api-portal/default/callback"
	d.Compose.StagedFiles = map[string]string{"role-to-scope-mapping.yaml": portalRoleMapping}
	d.Compose.GeneratedFiles["certs/cert.pem"] = multiTenancyTrustBundle()
	d.Config.ExtraOverlays = []string{multiTenancyOverlay}
	d.DependsOn = []string{"testbench"}
	// The portal root redirects an anonymous visitor into silent sign-in at the identity
	// provider in IDP mode, so readiness is the portal's own health endpoint instead.
	health := *d.Health
	health.Path = "/health"
	d.Health = &health
	return d
}

// PortalID returns the portal_id a multi-tenancy API Portal component serves.
func PortalID(component string) (string, bool) {
	if component == svcAPIPortalMultiTenancy {
		return MultiTenancyPortalID, true
	}
	return "", false
}

// multiTenancyTrustBundle is the certificate bundle a multi-tenancy portal trusts: the
// control plane's and the testbench identity provider's.
func multiTenancyTrustBundle() []byte {
	bundle := append([]byte(nil), shared.ControlPlaneCrypto()["certs/cert.pem"]...)
	if len(bundle) > 0 && bundle[len(bundle)-1] != '\n' {
		bundle = append(bundle, '\n')
	}
	return append(bundle, shared.IdentityProviderTLS().CertPEM...)
}

func apiPortalDefinition(name, composeFile, organization, displayName, portalID, serviceName string, overrides ...string) *components.Definition {
	env := map[string]string{EnvImageAPIPortal: shared.Image(EnvImageAPIPortal, shared.APIPortalImage()).Ref}
	env["APIP_AP_ORGANIZATION_HANDLE"] = organization
	env["APIP_AP_ORGANIZATION_DISPLAY_NAME"] = displayName
	env["APIP_AP_ORGANIZATION_PORTAL_ID"] = portalID
	for key, value := range portalSecurityEnv() {
		env[key] = value
	}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name:         name,
		Alias:        name,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile:          composeFile,
			ComposeOverrideFiles: overrides,

			Env: env,
			StagedFiles: map[string]string{
				"role-to-scope-mapping.yaml": "tests/framework/core/catalog/apiportal/resources/api-portal-auth-roles.yaml",
			},
			PrimaryService: serviceName,
			Services:       []string{serviceName},
			CoverageServices: []components.CoverageService{{
				Name: serviceName, OutputName: name, Types: []string{"node-v8"},
			}},
			GeneratedFiles: portalCryptoFiles(),
		},

		Endpoints: []components.Endpoint{
			{Name: "http", Port: 9543, Scheme: "http", AwaitListening: true},
		},

		Health: &components.HealthCheck{
			Endpoint: "http", Path: "/", ExpectStatus: 200,
			Timeout: 180 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath:    "portals/api-portal/configs/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/overlays/api-portal-storage.toml",
			ContainerPath:     "/config.toml",
			Format:            components.TOML,
		},

		DB: &components.DBContract{
			Supported: []components.DBType{components.Postgres, components.SQLServer},
			Schema: map[components.DBType][]string{
				components.Postgres:  {"portals/api-portal/database/schema.postgres.sql"},
				components.SQLServer: {"portals/api-portal/database/schema.sqlserver.sql"},
			},
			Env: apiPortalDBEnv,
		},

		DependsOn: []string{"platform-api"},

		Limits: components.ResourceLimits{CPUs: 1.5, MemoryMB: 2048},
	}
}

// portalSecurityEnv returns fresh per-definition secrets for the portal's test runtime.
func portalSecurityEnv() map[string]string {
	encryptionKey, err := shared.HexKey(32)
	if err != nil {
		panic("catalog: generating API Portal encryption key: " + err.Error())
	}
	sessionSecret, err := shared.HexKey(32)
	if err != nil {
		panic("catalog: generating API Portal session secret: " + err.Error())
	}
	return map[string]string{
		"APIP_AP_SECURITY_ENCRYPTION_KEY": encryptionKey,
		"APIP_AP_SECURITY_SESSION_SECRET": sessionSecret,
	}
}

func runtimeCoverageEnvironment() map[string]string {
	if !shared.CoverageMode() {
		return nil
	}
	spec, err := BuildSpec("")
	if err != nil {
		panic("catalog: building API Portal coverage specification: " + err.Error())
	}
	env := make(map[string]string, len(spec.Coverage.Environment))
	for key, value := range spec.Coverage.Environment {
		env[key] = value
	}
	return env
}

// portalCryptoFiles returns the control-plane public key and certificate required by the portal.
func portalCryptoFiles() map[string][]byte {
	cp := shared.ControlPlaneCrypto()
	return map[string][]byte{
		"keys/jwt_public.pem": cp["keys/jwt_public.pem"],
		"certs/cert.pem":      cp["certs/cert.pem"],
	}
}

// apiPortalDBEnv converts a database DSN to the portal's environment variables.
func apiPortalDBEnv(d components.DSN) map[string]string {
	driver := "postgres"
	if d.Type == components.SQLServer {
		driver = "mssql"
	}
	return map[string]string{
		"APIP_AP_DATABASE_DRIVER":   driver,
		"APIP_AP_DATABASE_HOST":     d.Host,
		"APIP_AP_DATABASE_PORT":     strconv.Itoa(d.Port),
		"APIP_AP_DATABASE_NAME":     d.Database,
		"APIP_AP_DATABASE_USER":     d.User,
		"APIP_AP_DATABASE_PASSWORD": d.Password,
	}
}
