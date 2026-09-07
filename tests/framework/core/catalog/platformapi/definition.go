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

// EnvImagePlatformAPI names the environment variable used to override the Platform API image.
const EnvImagePlatformAPI = "PA_IMAGE"

const svcPlatformAPI = "platform-api"

// PlatformAPI returns the Platform API component definition.
func PlatformAPI() *components.Definition {
	generated := shared.ControlPlaneCrypto()
	env := map[string]string{EnvImagePlatformAPI: shared.PlatformAPIImage()}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}

	return &components.Definition{
		Name:         svcPlatformAPI,
		Alias:        svcPlatformAPI,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/platformapi/docker-compose.yaml",

			Env:            env,
			PrimaryService: svcPlatformAPI,
			Services:       []string{svcPlatformAPI},
			StagedFiles: map[string]string{
				"role-to-scope-mapping.yaml": "platform-api/resources/role-to-scope-mapping.yaml",
			},
			GeneratedFiles: generated,
			CoverageServices: []components.CoverageService{{
				Name: svcPlatformAPI, Types: []string{"go"},
			}},
		},

		Endpoints: []components.Endpoint{
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

// insecureClient returns an HTTP client that accepts the control plane's test certificate.
func insecureClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // see doc comment
		},
	}
}

// platformAPIDBEnv converts a database DSN to the Platform API environment variables.
func platformAPIDBEnv(d components.DSN) map[string]string {
	env := map[string]string{"APIP_CP_DATABASE_DRIVER": string(d.Type)}
	switch d.Type {
	case components.SQLite:
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

// provisionGatewayRegistration registers the gateway and returns its control-plane credentials.
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

	const gatewayHandle = "it-gateway"

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
		"APIP_GW_CONTROLLER_CONTROLPLANE_HOST":                 svcPlatformAPI + ":9243",
		"APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN":                minted.Token,
		"APIP_GW_CONTROLLER_CONTROLPLANE_INSECURE_SKIP_VERIFY": "true",
	}, nil
}

// ControlPlaneLogin exchanges credentials for a control-plane bearer token.
func ControlPlaneLogin(ctx context.Context, base, username, password string) (string, error) {
	return platformAPILogin(ctx, insecureClient(), base,
		actor.Credentials{Username: username, Password: password})
}

// platformAPILogin exchanges credentials for a control-plane bearer token.
func platformAPILogin(
	ctx context.Context, client *http.Client, base string, admin actor.Credentials,
) (string, error) {
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

// platformAPICall issues a JSON request and decodes the response.
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
