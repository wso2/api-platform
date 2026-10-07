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

// End-to-end Agent proxy builds over the real route -> handler -> service ->
// repository stack, backed by SQLite: preparing a build, deploying it by id, and
// the conflict on deleting a build a gateway is serving.

package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
)

func setupAgentBuildEnv(t *testing.T) *agentDeployEnv {
	t.Helper()
	env := newAgentProxyTestEnv(t, &config.Server{
		Deployments: config.Deployments{MaxPerAPIGateway: 3, MaxBuildsPerAPI: 5},
	})
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost, agentProxyBase,
		fullAgentProxyBody("weather-agent")), http.StatusCreated)
	return &agentDeployEnv{
		agentProxyTestEnv: env,
		proxy:             "weather-agent",
		gateway:           "ai-gw",
		gatewayUUID:       seedAgentGateway(t, env.db, agentProxyOrg, "ai-gw", gwversion.MinAgentKindGatewayVersion),
	}
}

func (e *agentDeployEnv) buildsPath() string {
	return agentProxyBase + "/" + e.proxy + "/builds"
}

func TestAgentProxyBuild_CreateListGetWithResolvableLocation(t *testing.T) {
	env := setupAgentBuildEnv(t)

	rec := callAgentProxy(t, env.handler, http.MethodPost, env.buildsPath(), `{"description": "first cut"}`)
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)
	buildID, _ := body["buildId"].(string)
	if buildID == "" {
		t.Fatalf("build response has no buildId: %s", rec.Body.String())
	}
	wantLocation := env.buildsPath() + "/" + buildID
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Fatalf("Location = %q, want %q", got, wantLocation)
	}

	got := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, wantLocation, ""), http.StatusOK)
	if got["buildId"] != buildID || got["description"] != "first cut" {
		t.Fatalf("GET Location = %v, want build %s with its description", got, buildID)
	}

	// The body is optional.
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost, env.buildsPath(), ""), http.StatusCreated)

	list := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, env.buildsPath(), ""), http.StatusOK)
	if list["count"].(float64) != 2 {
		t.Fatalf("list count = %v, want 2", list["count"])
	}
}

func TestAgentProxyBuild_DeployShipsTheBuildNotLaterEdits(t *testing.T) {
	env := setupAgentBuildEnv(t)
	buildID := decodeAgentProxyJSON(t,
		callAgentProxy(t, env.handler, http.MethodPost, env.buildsPath(), ""), http.StatusCreated)["buildId"].(string)

	// Deploy from current once to learn what the unedited proxy renders to.
	baseline := env.storedContent(t, env.deploy(t, env.gateway))

	updated := strings.Replace(fullAgentProxyBody(env.proxy), `"displayName": "Weather Agent"`, `"displayName": "Renamed"`, 1)
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPut, agentProxyBase+"/"+env.proxy, updated), http.StatusOK)

	rec := callAgentProxy(t, env.handler, http.MethodPost, env.deploymentsPath(),
		fmt.Sprintf(`{"name": "from-build", "base": "build", "buildId": %q, "gatewayId": %q}`, buildID, env.gateway))
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)
	if body["buildId"] != buildID {
		t.Fatalf("deployment buildId = %v, want %s", body["buildId"], buildID)
	}
	if got := env.storedContent(t, body["deploymentId"].(string)); !bytes.Equal(got, baseline) {
		t.Fatalf("deploy from build picked up a later edit:\n got: %s\nwant: %s", got, baseline)
	}
}

func TestAgentProxyBuild_DeleteRefusedWhileDeployed(t *testing.T) {
	env := setupAgentBuildEnv(t)
	buildID := decodeAgentProxyJSON(t,
		callAgentProxy(t, env.handler, http.MethodPost, env.buildsPath(), ""), http.StatusCreated)["buildId"].(string)
	itemPath := env.buildsPath() + "/" + buildID

	deploymentID := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost, env.deploymentsPath(),
		fmt.Sprintf(`{"name": "dep", "base": "build", "buildId": %q, "gatewayId": %q}`, buildID, env.gateway)),
		http.StatusCreated)["deploymentId"].(string)
	env.ack(t, deploymentID, "DEPLOYED", "")

	assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodDelete, itemPath, ""),
		http.StatusConflict, apperror.CodeBuildInUse)

	env.ack(t, deploymentID, "UNDEPLOYED", "")

	rec := callAgentProxy(t, env.handler, http.MethodDelete, itemPath, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodGet, itemPath, ""),
		http.StatusNotFound, apperror.CodeBuildNotFound)
}

func TestAgentProxyBuild_UnknownProxyAndOtherOrganization(t *testing.T) {
	env := setupAgentBuildEnv(t)
	buildID := decodeAgentProxyJSON(t,
		callAgentProxy(t, env.handler, http.MethodPost, env.buildsPath(), ""), http.StatusCreated)["buildId"].(string)

	rec := callAgentProxy(t, env.handler, http.MethodPost, agentProxyBase+"/no-such-agent/builds", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("build of unknown proxy: status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}

	rec = callAgentProxyAs(t, env.handler, agentProxyOtherOrg, agentProxyActor, http.MethodGet,
		env.buildsPath()+"/"+buildID, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-organization build read: status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}
