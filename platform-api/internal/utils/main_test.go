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

package utils

import (
	"fmt"
	"os"
	"testing"

	"github.com/wso2/go-httpkit/httpclient"
	"github.com/wso2/go-httpkit/netguard"
)

// TestMain builds this package's test double for the single shared outbound *http.Client
// (see InitSharedHTTPClient) once, before any test in this package runs. Production wires
// this up in cmd/main.go from platform_api.http_client config; here it is built directly
// with a fixed test configuration that mirrors the shipped default — SSRF enabled under
// netguard.PermitPrivateBlockMetadata() (so tests can reach an httptest loopback server,
// exactly like a real in-cluster/private upstream) and MaxResponseBytes disabled, since
// call sites are responsible for their own size ceiling (see mcp.go, openapi_spec_fetcher.go).
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
	InitSharedHTTPClient(client, 0)

	os.Exit(m.Run())
}
