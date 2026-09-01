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

package platformapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvImagePlatformAPI overrides the platform-api image, for CI to pin a build.
//
// Read by the compose file as ${PA_IMAGE}, not by a Definition.Image: a compose-backed
// component must not declare an image, because then two places would name it and only one
// would win. Same arrangement as platform-gateway's PG_*_IMAGE.
const EnvImagePlatformAPI = "PA_IMAGE"

const svcPlatformAPI = "platform-api"

// PlatformAPI is the control plane: the REAL product, built from source.
//
// It exists as a component because the alternative was mocking it, and the legacy suite did.
// A 596-line mock of our own control plane asserts what we BELIEVED the contract to be: if
// platform-api changes /api/internal/v1 or the gateway websocket, the mock keeps answering the
// old shape and every cross-plane test stays green. That is the one case the mock rule in
// docs/migration-policy.md exists to forbid — mock a backend, never a product we ship.
//
// Auth runs in FILE mode, so no identity provider is needed to stand this up. The test-only
// overlay supplies the fixed admin credentials.
func PlatformAPI() *components.Definition {
	generated := shared.ControlPlaneCrypto()
	env := map[string]string{EnvImagePlatformAPI: shared.PlatformAPIImage()}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}

	return &components.Definition{
		Name: "platform-api",
		// Fixed, because the gateway addresses the control plane by name in its own
		// configuration (APIP_GW_CONTROLLER_CONTROLPLANE_HOST=platform-api:9243) rather than
		// through an accessor this framework controls.
		Alias:        svcPlatformAPI,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/platformapi/docker-compose.yaml",

			// Image pinned from the product's own VERSION file, so the ${PlatformAPI}
			// default in the YAML is only a fallback for reading the file by hand. See
			// version.go: :latest is not dependable here — api-portal's build never tags it.
			Env:            env,
			PrimaryService: svcPlatformAPI,
			Services:       []string{svcPlatformAPI},
			StagedFiles: map[string]string{
				// The SHIPPED mapping. Copying a test-local variant here would mean the suite
				// grants scopes no operator gets, and the authorization tests would prove
				// nothing about the product's own defaults.
				"role-to-scope-mapping.yaml": "platform-api/resources/role-to-scope-mapping.yaml",
			},
			GeneratedFiles: generated,
			CoverageServices: []components.CoverageService{{
				Name: svcPlatformAPI, Types: []string{"go"},
			}},
		},

		Endpoints: []components.Endpoint{
			// HTTPS only. The plain-HTTP listener ships disabled, and turning it on to avoid
			// generating a certificate would mean testing a configuration no operator runs.
			{Name: "https", Port: 9243, Scheme: "https", AwaitListening: true},
		},

		Health: &components.HealthCheck{
			Endpoint: "https", Path: "/health", ExpectStatus: 200,
			Timeout: 120 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath:    "platform-api/config/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/overlays/platform-api-storage.toml",
			ContainerPath:     "/config.toml",
			Format:            components.TOML,
		},

		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			// platform-api auto-runs its DDL for SQLITE ONLY. Against an external database it
			// expects the schema to be pre-provisioned, exactly as an operator would — so the
			// framework applies it, and the product's own startup seeding then finds its
			// tables. Getting this wrong does not fail at connect time: the connection
			// succeeds and the first query dies with `relation "organizations" does not
			// exist`, which names the table rather than the missing migration.
			Schema: map[components.DBType][]string{
				components.Postgres:  {"platform-api/internal/database/schema.postgres.sql"},
				components.SQLServer: {"platform-api/internal/database/schema.sqlserver.sql"},
			},
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          platformAPIDBEnv,
		},

		Provisions: provisionGatewayRegistration,

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

// insecureClient dials the control plane's per-block self-signed certificate.
//
// Verification is skipped because the certificate is generated at boot and trusted by nobody;
// the cross-plane features assert on control-plane BEHAVIOUR, not on TLS trust.
func insecureClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // see doc comment
		},
	}
}

// platformAPIDBEnv renders a DSN into platform-api's environment vocabulary.
//
// Note the APIP_CP_ prefix — the control plane's, not the gateway's APIP_GW_CONTROLLER_. The
// two products share a topology and a database server but not a naming scheme, and a key
// spelled with the wrong prefix is INERT: platform-api has no env-override layer, so it takes
// only names its config file spells as {{ env }}, and an unrecognised one is silently ignored
// rather than rejected.
func platformAPIDBEnv(d components.DSN) map[string]string {
	env := map[string]string{"APIP_CP_DATABASE_DRIVER": string(d.Type)}
	switch d.Type {
	case components.SQLite:
		// The product's driver name, not the framework's type name — it accepts
		// "sqlite3" and rejects "sqlite".
		env["APIP_CP_DATABASE_DRIVER"] = "sqlite3"
		env["APIP_CP_DATABASE_PATH"] = d.FilePath
	case components.Postgres:
		env["APIP_CP_DATABASE_DRIVER"] = "postgres"
		env["APIP_CP_DATABASE_HOST"] = d.Host
		env["APIP_CP_DATABASE_PORT"] = strconv.Itoa(d.Port)
		env["APIP_CP_DATABASE_NAME"] = d.Database
		env["APIP_CP_DATABASE_USER"] = d.User
		env["APIP_CP_DATABASE_PASSWORD"] = d.Password
		env["APIP_CP_DATABASE_SSL_MODE"] = d.SSLMode
	case components.SQLServer:
		env["APIP_CP_DATABASE_DRIVER"] = "sqlserver"
		env["APIP_CP_DATABASE_HOST"] = d.Host
		env["APIP_CP_DATABASE_PORT"] = strconv.Itoa(d.Port)
		env["APIP_CP_DATABASE_NAME"] = d.Database
		env["APIP_CP_DATABASE_USER"] = d.User
		env["APIP_CP_DATABASE_PASSWORD"] = d.Password
	}
	return env
}

// provisionGatewayRegistration mints the token the gateway uses to reach the control plane.
//
// This is why Definition.Provisions exists. The token is a ROW in platform-api's database,
// stored hashed and salted, created by two calls against a RUNNING control plane:
//
//	POST /gateways                -> gateway id
//	POST /gateways/{id}/tokens    -> the token, returned once, in clear
//
// So it cannot be declared in YAML (it does not exist until platform-api is up) and it cannot
// be generated in isolation the way a password is. Inserting the row directly was rejected: it
// would duplicate the product's hashing scheme in test code, break silently when the product
// changed it, and leave the registration endpoint itself untested.
//
// The returned pairs land in the environment of every component naming platform-api in
// DependsOn — in practice the gateway, whose controller reads them at boot.
func provisionGatewayRegistration(
	ctx context.Context, inst *components.Instance,
) (map[string]string, error) {
	base, err := inst.URL("https")
	if err != nil {
		return nil, err
	}

	client := insecureClient()

	bearer, err := platformAPILogin(ctx, client, base, actor.Administrator())
	if err != nil {
		return nil, err
	}

	// The gateway registers itself under this handle. Fixed rather than unique-per-block
	// because a block has exactly one control plane and one gateway; two blocks never share
	// a platform-api, so the name cannot collide across them.
	const gatewayHandle = "it-gateway"

	// Versioned API base. NOT /api/v1 — the control plane serves v0.9, and a wrong version
	// answers 401 "authorization header missing" rather than 404, because the auth middleware
	// runs before routing and an unmatched path never reaches a handler that would say so.
	const apiBase = "/api/v0.9"

	var created struct {
		ID   string `json:"id"`
		UUID string `json:"uuid"`
	}
	if err := platformAPICall(ctx, client, http.MethodPost, base+apiBase+"/gateways", bearer,
		map[string]any{
			"id":                gatewayHandle,
			"displayName":       gatewayHandle,
			"endpoints":         []string{"http://gateway-runtime:8080"},
			"functionalityType": shared.GatewayFunctionalityType(),
		}, &created); err != nil {
		return nil, fmt.Errorf("registering the gateway: %w", err)
	}

	id := created.UUID
	if id == "" {
		id = created.ID
	}
	if id == "" {
		return nil, fmt.Errorf("the control plane returned no gateway id")
	}

	var minted struct {
		Token string `json:"token"`
	}
	if err := platformAPICall(ctx, client, http.MethodPost,
		base+apiBase+"/gateways/"+id+"/tokens", bearer, map[string]any{}, &minted); err != nil {
		return nil, fmt.Errorf("minting the gateway token: %w", err)
	}
	if minted.Token == "" {
		return nil, fmt.Errorf("the control plane returned an empty gateway token")
	}

	return map[string]string{
		// Read by gateway-controller at boot. INSECURE_SKIP_VERIFY because the control plane
		// serves a per-block self-signed certificate: this asserts nothing about TLS trust,
		// which is not what the cross-plane features are testing.
		"APIP_GW_CONTROLLER_CONTROLPLANE_HOST":                 svcPlatformAPI + ":9243",
		"APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN":                minted.Token,
		"APIP_GW_CONTROLLER_CONTROLPLANE_INSECURE_SKIP_VERIFY": "true",
	}, nil
}

// ControlPlaneLogin exchanges credentials for a bearer token against a running control plane.
//
// Exported because a STEP needs it too: asserting that the control plane lists a gateway
// requires authenticating exactly as the provisioner does, and duplicating the form-encoding
// and path quirks in the step package is how the two drift apart.
func ControlPlaneLogin(ctx context.Context, base, username, password string) (string, error) {
	return platformAPILogin(ctx, insecureClient(), base,
		actor.Credentials{Username: username, Password: password})
}

// platformAPILogin exchanges the test admin credentials for a bearer token.
func platformAPILogin(
	ctx context.Context, client *http.Client, base string, admin actor.Credentials,
) (string, error) {
	// FORM-encoded, not JSON, and on the portal base rather than the versioned API base.
	// Both differ from every other call the provisioner makes; sending JSON here fails in a
	// way that reports a credential problem rather than a content-type one.
	form := url.Values{"username": {admin.Username}, "password": {admin.Password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/api/portal/v0.9/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("logging in to the control plane: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK || out.Token == "" {
		return "", fmt.Errorf("logging in as %q -> %d: %s", admin.Username, resp.StatusCode, bytes.TrimSpace(raw))
	}
	return out.Token, nil
}

// platformAPICall issues one JSON request and decodes the response.
//
// Errors carry the STATUS AND BODY. A provisioning failure happens before any scenario runs,
// so the only diagnostic anyone gets is this message; "unexpected status 400" would send the
// reader to the container logs for something the response already said.
func platformAPICall(
	ctx context.Context, client *http.Client, method, url, bearer string, body, out any,
) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s -> %d: %s", method, url, resp.StatusCode, bytes.TrimSpace(raw))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s %s returned unparseable JSON: %w: %s", method, url, err, raw)
	}
	return nil
}
