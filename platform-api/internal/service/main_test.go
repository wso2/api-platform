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

package service

import (
	"fmt"
	"os"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// TestMain initializes the package-level shared outbound *http.Client (see
// utils.InitSharedHTTPClient) once, before any test in this package runs. Several tests here
// (mcp_test.go's TestFetchServerInfo*, llm_openapi_resolve_test.go's
// TestResolveTemplateOpenAPISpec) reach utils.NewUpstreamFetchClient/FetchOpenAPISpecFromURL
// indirectly through the service layer — an httptest loopback server, or the SSRF guard
// itself, only exercises real behavior if the shared client is actually built, mirroring
// production's cmd/main.go wiring rather than being left nil.
func TestMain(m *testing.M) {
	policy := netguard.PermitPrivateBlockMetadata()
	policy.AllowedSchemes = []string{"http", "https"}

	cfg := httpclient.DefaultConfig()
	cfg.Pooling.DisableKeepAlives = true
	cfg.Timeouts.MaxResponseBytes = -1
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = policy
	cfg.SSRF.MaxRedirects = 5

	client, err := httpclient.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build test double for the shared HTTP client: %v\n", err)
		os.Exit(1)
	}
	utils.InitSharedHTTPClient(client, 0)

	os.Exit(m.Run())
}
