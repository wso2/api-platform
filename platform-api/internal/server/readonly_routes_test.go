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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed.

package server

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/handler"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// TestReadOnlyExemptRoutesAreRegistered is the build-time guard on the read-only
// guard's exempt list: every route it exempts must be registered under exactly
// that pattern by the real handlers. StartPlatformAPIServer runs the same check
// at startup; this catches a route rename before a release goes out. The
// services are nil: RegisterRoutes only closes over the handler and its logger.
func TestReadOnlyExemptRoutesAreRegistered(t *testing.T) {
	logger := slog.Default()
	mux := http.NewServeMux()
	handler.NewAPIHandler(nil, nil, nil, logger, nil).RegisterRoutes(mux)
	handler.NewMCPProxyHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewAgentProxyHandler(nil, nil, logger).RegisterRoutes(mux)
	handler.NewGatewayInternalAPIHandler(nil, nil, nil, nil, logger).RegisterRoutes(mux)

	if err := middleware.ValidateReadOnlyExemptRoutes(mux); err != nil {
		t.Fatal(err)
	}

	// And the check is not vacuous: an empty router fails it.
	if err := middleware.ValidateReadOnlyExemptRoutes(http.NewServeMux()); err == nil {
		t.Fatal("expected the exempt-route check to fail against an empty router")
	}
}
