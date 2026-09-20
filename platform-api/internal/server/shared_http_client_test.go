/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

func resetSharedHTTPClient(t *testing.T) {
	t.Helper()
	utils.InitSharedHTTPClient(nil, 0)
	t.Cleanup(func() { utils.InitSharedHTTPClient(nil, 0) })
}

func TestInitSharedHTTPClientMakesClientAvailable(t *testing.T) {
	resetSharedHTTPClient(t)
	cfg := &config.Server{}
	cfg.HTTPClient.SSRF = config.HTTPClientSSRFConfig{
		Enabled:        true,
		Preset:         "permit_private_block_metadata",
		AllowedSchemes: []string{"https"},
		MaxRedirects:   3,
	}

	if err := InitSharedHTTPClient(cfg); err != nil {
		t.Fatalf("InitSharedHTTPClient: %v", err)
	}
	client, err := utils.NewUpstreamFetchClient(0)
	if err != nil || client == nil {
		t.Fatalf("want the shared client after init, got %v, %v", client, err)
	}
}

func TestInitSharedHTTPClientRejectsInvalidConfig(t *testing.T) {
	cases := map[string]config.HTTPClientConfig{
		"unknown proxy mode":  {Proxy: config.HTTPClientProxyConfig{Mode: "socks"}},
		"unknown ssrf preset": {SSRF: config.HTTPClientSSRFConfig{Enabled: true, Preset: "everything"}},
	}
	for name, hc := range cases {
		t.Run(name, func(t *testing.T) {
			resetSharedHTTPClient(t)
			if err := InitSharedHTTPClient(&config.Server{HTTPClient: hc}); err == nil {
				t.Fatal("want an error for the invalid configuration")
			}
			if _, err := utils.NewUpstreamFetchClient(0); err == nil {
				t.Fatal("want no shared client after a failed init")
			}
		})
	}
}
