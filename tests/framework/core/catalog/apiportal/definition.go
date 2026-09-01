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

// EnvImageAPIPortal overrides the api-portal image, read by the compose file as ${AP_IMAGE}.
const EnvImageAPIPortal = "AP_IMAGE"

const svcAPIPortal = "api-portal"

// APIPortal is the developer portal: the REAL product, built from source.
//
// It exists in this catalog for one reason — it is the only thing that issues a SUBSCRIPTION
// TOKEN. platform-api cannot mint one: POST /subscriptions passes an empty token, and the sole
// writer is RegenerateToken, reachable only from the webhook receiver. The portal creates the
// token, posts it to the control plane inside a signed webhook with the value encrypted, and
// the control plane persists it verbatim and broadcasts it to deployed gateways.
//
// The portal is provisioned as the real product so subscription behavior exercises the
// production control-plane contract.
func APIPortal() *components.Definition {
	env := map[string]string{EnvImageAPIPortal: shared.APIPortalImage()}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name: svcAPIPortal,
		// Fixed: platform-api and the portal address each other by name in configuration
		// neither this framework nor a feature rewrites.
		Alias:        svcAPIPortal,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/apiportal/docker-compose.yaml",

			// Image pinned from the product's own VERSION file, so the ${APIPortal}
			// default in the YAML is only a fallback for reading the file by hand. See
			// version.go: :latest is not dependable here — api-portal's build never tags it.
			Env:            env,
			PrimaryService: svcAPIPortal,
			Services:       []string{svcAPIPortal},
			CoverageServices: []components.CoverageService{{
				Name: svcAPIPortal, Types: []string{"node-v8"},
			}},
			// The control plane's OWN key material, not a second draw. The portal verifies
			// RS256 tokens platform-api signed and dials its self-signed TLS endpoint, so a
			// separately generated pair would reject every token and the failure would read
			// as an authentication problem rather than a key mismatch.
			GeneratedFiles: portalCryptoFiles(),
		},

		Endpoints: []components.Endpoint{
			// Plain HTTP. The portal's own TLS is disabled in this topology (see the overlay):
			// it adds a second self-signed certificate to trust and tests nothing the control
			// plane's TLS does not already cover.
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
			// The portal does NOT create its own tables — same as platform-api, and I assumed
			// otherwise for both. It seeds a default organization at startup and dies with
			// `relation "organizations" does not exist` if the schema is absent, which names
			// the table rather than the missing migration.
			Schema: map[components.DBType][]string{
				components.Postgres: {"portals/api-portal/database/schema.postgres.sql"},
			},
			Env: apiPortalDBEnv,
		},

		// Boots AFTER the control plane: it verifies tokens the control plane signs and calls it
		// during startup, so starting first means failing for a reason unrelated to the test.
		DependsOn: []string{"platform-api"},

		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 1000},
	}
}

func runtimeCoverageEnvironment() map[string]string {
	if !shared.CoverageMode() {
		return nil
	}
	spec, _ := BuildSpec("")
	env := make(map[string]string, len(spec.Coverage.Environment))
	for key, value := range spec.Coverage.Environment {
		env[key] = value
	}
	return env
}

// portalCryptoFiles selects the two pieces of the control plane's identity the portal needs.
//
// Only the PUBLIC halves. The portal verifies signatures and trusts a certificate; handing it
// the signing key would let a portal bug forge control-plane tokens, and nothing here needs it.
func portalCryptoFiles() map[string][]byte {
	cp := shared.ControlPlaneCrypto()
	return map[string][]byte{
		"keys/jwt_public.pem": cp["keys/jwt_public.pem"],
		"certs/cert.pem":      cp["certs/cert.pem"],
	}
}

// apiPortalDBEnv renders a DSN into the portal's environment vocabulary.
//
// APIP_AP_, distinct from the control plane's APIP_CP_ and the gateway's APIP_GW_CONTROLLER_.
// Three products, three prefixes, one topology — a key with the wrong prefix is silently
// ignored rather than rejected.
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
