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
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/featureusage"
	"github.com/wso2/api-platform/platform-api/internal/handler"
)

// TestFeatureUsageBindingsResolveToRealRoutes is the build-time guard on
// featureusage/bindings.go: every bound operation must be served by the real
// handlers under its OpenAPI path, with the path parameter names its binding
// reads. StartPlatformAPIServer resolves the same routes at startup but only
// logs one it cannot find (a build may leave out a plugin's routes); this
// catches a renamed route or parameter before a release goes out. The
// services are nil: RegisterRoutes only closes over the handler and its
// logger.
func TestFeatureUsageBindingsResolveToRealRoutes(t *testing.T) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	mux := http.NewServeMux()
	handler.NewProjectHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewAPIHandler(nil, nil, nil, logger, nil).RegisterRoutes(mux)
	handler.NewAPIDocumentHandler(nil, nil, logger, nil).RegisterRoutes(mux)
	handler.NewAPIThumbnailHandler(nil, nil, logger, nil).RegisterRoutes(mux)
	handler.NewAPIInvokeHandler(nil, nil, 0, 0, logger).RegisterRoutes(mux)
	handler.NewGatewayHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewSubscriptionPlanHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewMonetizationHandler(nil, logger).RegisterRoutes(mux)
	handler.NewPublicationHandler(nil, nil, 0, 0, logger).RegisterRoutes(mux)
	handler.NewAPIKeyHandler(nil, nil, "", logger).RegisterRoutes(mux)
	handler.NewDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGraphQLAPIHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGraphQLAPIKeyHandler(nil, nil, "", logger).RegisterRoutes(mux)
	handler.NewGraphQLAPIDeploymentHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewServiceAccountHandler(nil, nil, nil, 0, logger).RegisterRoutes(mux)

	spec, err := os.ReadFile("../../resources/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tax, err := featureusage.DefaultTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	index, err := featureusage.NewRouteIndex(spec, mux, tax, logger)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), "will not be reported") {
		t.Fatalf("some bound operations are not served by the real handlers:\n%s", logs.String())
	}
	if index.Len() == 0 {
		t.Fatal("no bound operation resolved to a route")
	}
}
