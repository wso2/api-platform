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
package platform

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// A library consumer boots through New and Run, never through cmd/main.go, so New itself
// must leave the shared outbound HTTP client ready.
func TestNewInitializesSharedHTTPClient(t *testing.T) {
	utils.InitSharedHTTPClient(nil, 0)
	t.Cleanup(func() { utils.InitSharedHTTPClient(nil, 0) })

	if _, err := New(WithConfig(&config.Server{})); err != nil {
		t.Fatalf("New: %v", err)
	}
	if client, err := utils.NewUpstreamFetchClient(0); err != nil || client == nil {
		t.Fatalf("want the shared client after New, got %v, %v", client, err)
	}
}

func TestNewRejectsInvalidHTTPClientConfig(t *testing.T) {
	cfg := &config.Server{}
	cfg.HTTPClient.Proxy.Mode = "socks"

	if _, err := New(WithConfig(cfg)); err == nil {
		t.Fatal("want New to fail on an invalid http_client configuration")
	}
}
