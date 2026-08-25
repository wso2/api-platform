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

package server

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/handler"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	eghandler "github.com/wso2/api-platform/platform-api/plugins/eventgateway/handler"
)

const (
	realSpecPath   = "../../resources/openapi.yaml"
	pluginSpecPath = "../../plugins/eventgateway/openapi.yaml"
)

// registerAllRoutes wires every handler's routes onto a mux, in the same order
// New() does. The services are nil: RegisterRoutes only closes over the handler
// and its logger, so no dependency is dereferenced at registration time.
func registerAllRoutes(mux *http.ServeMux) {
	logger := slog.Default()

	handler.NewOrganizationHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewProjectHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewApplicationHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewAPIHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGatewayHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewSubscriptionHandler(nil, nil, nil, logger).RegisterRoutes(mux)
	handler.NewSubscriptionPlanHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewAPIKeyHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewLLMHandler(nil, nil, nil, nil, logger).RegisterRoutes(mux)
	handler.NewLLMProviderDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewLLMProxyDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewLLMProviderAPIKeyHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewLLMProxyAPIKeyHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewAPIKeyUserHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewMCPProxyHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewMCPProxyDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGraphQLAPIHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGraphQLAPIDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGraphQLAPIKeyHandler(nil, nil, "scope", logger).RegisterRoutes(mux)
	handler.NewSecretHandler(nil, nil, logger).RegisterRoutes(mux)

	// Plugin routes are registered on the same mux and their specs merged into
	// the same registry, so they are held to the same check.
	eghandler.NewWebSubAPIHandler(nil, nil, logger).RegisterRoutes(mux)
	eghandler.NewWebSubAPIDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	eghandler.NewWebSubAPIHmacSecretHandler(nil, nil, logger).RegisterRoutes(mux)
	eghandler.NewWebSubAPIKeyHandler(nil, nil, nil, "scope", logger).RegisterRoutes(mux)
	eghandler.NewWebBrokerAPIHandler(nil, nil, logger).RegisterRoutes(mux)
	eghandler.NewWebBrokerAPIDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	eghandler.NewWebBrokerAPIKeyHandler(nil, nil, nil, "scope", logger).RegisterRoutes(mux)
}

// loadMergedRegistry loads the shipped spec plus the event-gateway plugin's
// embedded spec, the same merge New() performs.
func loadMergedRegistry(t *testing.T) *middleware.ScopeRegistry {
	t.Helper()

	registry, err := middleware.LoadScopeRegistry(realSpecPath)
	if err != nil {
		t.Fatalf("LoadScopeRegistry(%q): %v", realSpecPath, err)
	}

	pluginSpec, err := os.ReadFile(pluginSpecPath)
	if err != nil {
		t.Fatalf("read %q: %v", pluginSpecPath, err)
	}
	pluginRegistry, err := middleware.LoadScopeRegistryFromBytes(pluginSpec)
	if err != nil {
		t.Fatalf("LoadScopeRegistryFromBytes(%q): %v", pluginSpecPath, err)
	}
	registry.Merge(pluginRegistry)

	return registry
}

// TestScopeRegistryCoversEveryRegisteredRoute is the guard on the enforcer's
// deny-by-default behaviour: every operation the shipped OpenAPI spec declares
// a scope for must resolve to a route registered under exactly that pattern.
// A path parameter renamed on one side only, or an operation declared but never
// wired up, would otherwise become a 403 on a live endpoint.
//
// New() runs the same check at startup; this test catches the drift at build
// time, before a release goes out.
func TestScopeRegistryCoversEveryRegisteredRoute(t *testing.T) {
	registry := loadMergedRegistry(t)
	if registry.Len() == 0 {
		t.Fatalf("scope registry loaded from %q is empty", realSpecPath)
	}

	mux := http.NewServeMux()
	registerAllRoutes(mux)

	if err := middleware.ValidateScopeRegistryRoutes(mux, registry); err != nil {
		t.Fatal(err)
	}
}

// TestSecretsRoutesAreRegisteredOnTheBasePath asserts the secrets API is served
// from the standard base path and that every one of its routes resolves to a
// declared scope. Removing the legacy "/api/v1" alias must not have taken the
// real routes with it.
func TestSecretsRoutesAreRegisteredOnTheBasePath(t *testing.T) {
	registry := loadMergedRegistry(t)

	mux := http.NewServeMux()
	registerAllRoutes(mux)

	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, constants.APIBasePath + "/secrets"},
		{http.MethodPost, constants.APIBasePath + "/secrets"},
		{http.MethodGet, constants.APIBasePath + "/secrets/s-1"},
		{http.MethodPut, constants.APIBasePath + "/secrets/s-1"},
		{http.MethodDelete, constants.APIBasePath + "/secrets/s-1"},
	} {
		req, err := http.NewRequest(probe.method, probe.path, nil)
		if err != nil {
			t.Fatalf("build probe request: %v", err)
		}
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("%s %s is not registered", probe.method, probe.path)
			continue
		}
		_, matchedPath, _ := strings.Cut(pattern, " ")
		if _, found := registry.Lookup(probe.method, matchedPath); !found {
			t.Errorf("%s %s resolves to %q, which declares no scope and would be denied",
				probe.method, probe.path, pattern)
		}
	}
}
