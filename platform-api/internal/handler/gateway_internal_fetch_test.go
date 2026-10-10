/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package handler

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// The gateway in the secret test env reports version 1.0, so platform-api
// inlines secrets into what it delivers. A fetch of a deployment whose secret
// cannot be resolved is refused with a 422 whose body the gateway logs: it
// names the status reason, never the handle. The deployment is marked FAILED.

// insertDeployment stores a DEPLOYING deployment (desired DEPLOYED) of the
// artifact on the gateway with the given content.
func insertDeployment(t *testing.T, db *database.DB, orgID, artifactUUID, gatewayID, deploymentID string, content []byte) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, content, metadata, created_by)
		VALUES (?, 'dep', ?, ?, ?, ?, '{}', 'tester')`, deploymentID, artifactUUID, orgID, gatewayID, content); err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid, status, status_desired)
		VALUES (?, ?, ?, ?, 'DEPLOYING', 'DEPLOYED')`, artifactUUID, orgID, gatewayID, deploymentID); err != nil {
		t.Fatalf("insert deployment status: %v", err)
	}
}

func providerContent(handle string) []byte {
	return []byte("apiVersion: gateway.api-platform.wso2.com/v1\nkind: LlmProvider\nspec:\n  upstream:\n    main:\n      auth:\n        value: '{{ secret \"" + handle + "\" }}'\n")
}

func TestGatewayInternalFetch_UnresolvableSecretIsRefusedWith422(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()
	const artifactID = "art-fetch-001"
	insertArtifact(t, env.db, env.orgID, artifactID, artifactID)
	insertDeployment(t, env.db, env.orgID, artifactID, env.gatewayID, "dep-fetch-001", providerContent("gone-handle"))

	w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/llm-providers/"+artifactID, env.plainToken)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", w.Code, w.Body.String())
	}
	desc, _ := parseGWBody(w)["description"].(string)
	if !strings.Contains(desc, "SECRET_RESOLUTION_FAILED") {
		t.Errorf("description %q should name the status reason", desc)
	}
	if strings.Contains(w.Body.String(), "gone-handle") {
		t.Errorf("response must not disclose the secret handle: %s", w.Body.String())
	}

	var status, desired, reason string
	if err := env.db.QueryRow(`SELECT status, status_desired, COALESCE(status_reason, '') FROM deployment_status WHERE deployment_uuid = 'dep-fetch-001'`).
		Scan(&status, &desired, &reason); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "FAILED" || desired != "DEPLOYED" || reason != "SECRET_RESOLUTION_FAILED" {
		t.Errorf("status row = %s/%s/%s, want FAILED/DEPLOYED/SECRET_RESOLUTION_FAILED", status, desired, reason)
	}

	t.Run("a resolvable secret is delivered", func(t *testing.T) {
		createSecretDirect(t, env.svc, env.orgID, "live-handle", "sk-live")
		const okID = "art-fetch-002"
		insertArtifact(t, env.db, env.orgID, okID, okID)
		insertDeployment(t, env.db, env.orgID, okID, env.gatewayID, "dep-fetch-002", providerContent("live-handle"))

		w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/llm-providers/"+okID, env.plainToken)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
		}
	})

	t.Run("an unknown provider is still a 404", func(t *testing.T) {
		w := doGWRequest(env.router, http.MethodGet, "/api/internal/v1/llm-providers/no-such-provider", env.plainToken)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body %s", w.Code, w.Body.String())
		}
	})
}

// Lookup-only fakes so every kind's handler reaches the shared delivery path.
type stubMCPRepo struct{ repository.MCPProxyRepository }

func (stubMCPRepo) GetByUUID(uuid, _ string) (*model.MCPProxy, error) {
	return &model.MCPProxy{UUID: uuid}, nil
}

type stubAgentRepo struct {
	repository.AgentProxyRepository
}

func (stubAgentRepo) GetByUUID(uuid, _ string) (*model.AgentProxy, error) {
	return &model.AgentProxy{UUID: uuid}, nil
}

type stubWebSubRepo struct{ repository.WebSubAPIRepository }

func (stubWebSubRepo) GetByUUID(uuid, _ string) (*model.WebSubAPI, error) {
	return &model.WebSubAPI{UUID: uuid}, nil
}

type stubWebBrokerRepo struct {
	repository.WebBrokerAPIRepository
}

func (stubWebBrokerRepo) GetByUUID(uuid, _ string) (*model.WebBrokerAPI, error) {
	return &model.WebBrokerAPI{UUID: uuid}, nil
}

// allKindsRouter serves the internal API over env's database with every
// artifact kind's lookup wired.
func allKindsRouter(env *gatewaySecretTestEnv) http.Handler {
	identity := service.NewIdentityService(repository.NewUserIdentityMappingRepo(env.db))
	gatewayRepo := repository.NewGatewayRepo(env.db)
	gatewaySvc := service.NewGatewayService(gatewayRepo, nil, nil, nil, nil, slog.Default(), false, false, nil, identity)
	svc := service.NewGatewayInternalAPIService(nil, nil, nil, nil, nil, stubMCPRepo{}, stubAgentRepo{},
		repository.NewDeploymentRepo(env.db, repository.NewArtifactTableRegistry()), gatewayRepo,
		nil, nil, nil, nil, repository.NewSecretRepo(env.db), &config.Server{}, slog.Default())
	svc.SetSecretService(env.svc)
	svc.SetEventArtifactRepos(stubWebSubRepo{}, stubWebBrokerRepo{})
	mux := http.NewServeMux()
	NewGatewayInternalAPIHandler(gatewaySvc, svc, nil, env.svc, slog.Default()).RegisterRoutes(mux)
	return mux
}

var fetchRoutes = []struct{ kind, path string }{
	{"API", "/api/internal/v1/apis/"},
	{"LLM provider", "/api/internal/v1/llm-providers/"},
	{"LLM proxy", "/api/internal/v1/llm-proxies/"},
	{"MCP proxy", "/api/internal/v1/mcp-proxies/"},
	{"Agent", "/api/internal/v1/agents/"},
	{"WebSub API", "/api/internal/v1/websub-apis/"},
	{"WebBroker API", "/api/internal/v1/webbroker-apis/"},
}

func doFetch(router http.Handler, path, apiKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil) // RemoteAddr 192.0.2.1:1234
	req.Header.Set("api-key", apiKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// Every kind refuses an unresolvable secret with the same 422.
func TestGatewayInternalFetch_EveryKindRefusesUnresolvableSecret(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()
	router := allKindsRouter(env)
	for i, rt := range fetchRoutes {
		t.Run(rt.kind, func(t *testing.T) {
			id := "art-kind-" + string(rune('a'+i))
			insertArtifact(t, env.db, env.orgID, id, id)
			insertDeployment(t, env.db, env.orgID, id, env.gatewayID, "dep-"+id, providerContent("gone-handle"))
			w := doFetch(router, rt.path+id, env.plainToken)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %s", w.Code, w.Body.String())
			}
		})
	}
}

// Any other failure is a 500 that names the kind, never the cause.
func TestGatewayInternalFetch_EveryKindAnswers500OnOtherFailures(t *testing.T) {
	env, cleanup := setupGatewaySecretTestEnv(t)
	defer cleanup()
	router := allKindsRouter(env)
	if _, err := env.db.Exec(`ALTER TABLE deployment_status RENAME TO deployment_status_gone`); err != nil {
		t.Fatalf("break the deployment lookup: %v", err)
	}
	for _, rt := range fetchRoutes {
		t.Run(rt.kind, func(t *testing.T) {
			w := doFetch(router, rt.path+"art-any", env.plainToken)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body %s", w.Code, w.Body.String())
			}
			desc, _ := parseGWBody(w)["description"].(string)
			if desc != "Failed to get "+rt.kind {
				t.Errorf("description = %q, want %q", desc, "Failed to get "+rt.kind)
			}
			if strings.Contains(w.Body.String(), "deployment_status") {
				t.Errorf("response leaks the internal error: %s", w.Body.String())
			}
		})
	}
}
