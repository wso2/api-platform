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

// APIPortal returns the API Portal component definition.
func APIPortal() *components.Definition {
	env := map[string]string{EnvImageAPIPortal: shared.Image(EnvImageAPIPortal, shared.APIPortalImage()).Ref}
	for key, value := range portalSecurityEnv() {
		env[key] = value
	}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name:         svcAPIPortal,
		Alias:        svcAPIPortal,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/apiportal/docker-compose.yaml",

			Env:            env,
			PrimaryService: svcAPIPortal,
			Services:       []string{svcAPIPortal},
			CoverageServices: []components.CoverageService{{
				Name: svcAPIPortal, Types: []string{"node-v8"},
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
			Supported: []components.DBType{components.Postgres},
			Schema: map[components.DBType][]string{
				components.Postgres: {"portals/api-portal/database/schema.postgres.sql"},
			},
			Env: apiPortalDBEnv,
		},

		DependsOn: []string{"platform-api"},

		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 1000},
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
	return map[string]string{
		"APIP_AP_DATABASE_DRIVER":   "postgres",
		"APIP_AP_DATABASE_HOST":     d.Host,
		"APIP_AP_DATABASE_PORT":     strconv.Itoa(d.Port),
		"APIP_AP_DATABASE_NAME":     d.Database,
		"APIP_AP_DATABASE_USER":     d.User,
		"APIP_AP_DATABASE_PASSWORD": d.Password,
	}
}
