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
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func endpointWithAuth(url, value string) *model.UpstreamEndpoint {
	return &model.UpstreamEndpoint{URL: url, Auth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: value}}
}

func TestPreserveUpstreamAuthValue_Sandbox(t *testing.T) {
	preservers := map[string]func(existing, updated *model.UpstreamConfig) *model.UpstreamConfig{
		"llm": preserveUpstreamAuthValue,
		"mcp": preserveMCPUpstreamAuthValue,
	}
	for name, preserve := range preservers {
		t.Run(name, func(t *testing.T) {
			stored := func() *model.UpstreamConfig {
				return &model.UpstreamConfig{Main: endpointWithAuth("https://main", "main-secret"), Sandbox: endpointWithAuth("https://sandbox", "sandbox-secret")}
			}

			out := preserve(stored(), &model.UpstreamConfig{Main: endpointWithAuth("https://main", ""), Sandbox: endpointWithAuth("https://sandbox", "")})
			if out.Main.Auth.Value != "main-secret" {
				t.Fatalf("main value = %q, want the stored one", out.Main.Auth.Value)
			}
			if out.Sandbox.Auth.Value != "sandbox-secret" {
				t.Fatalf("sandbox value = %q, want the stored one", out.Sandbox.Auth.Value)
			}

			out = preserve(stored(), &model.UpstreamConfig{Main: endpointWithAuth("https://main", ""), Sandbox: endpointWithAuth("https://sandbox", "new-sandbox")})
			if out.Sandbox.Auth.Value != "new-sandbox" {
				t.Fatalf("sandbox value = %q, want the new one", out.Sandbox.Auth.Value)
			}
			if out.Main.Auth.Value != "main-secret" {
				t.Fatalf("main value = %q, want the stored one", out.Main.Auth.Value)
			}

			out = preserve(stored(), &model.UpstreamConfig{Main: endpointWithAuth("https://main", "new-main"), Sandbox: &model.UpstreamEndpoint{URL: "https://sandbox"}})
			if out.Main.Auth.Value != "new-main" {
				t.Fatalf("main value = %q, want the new one", out.Main.Auth.Value)
			}
			if out.Sandbox.Auth != nil {
				t.Fatalf("sandbox auth omitted in the update must stay cleared")
			}
		})
	}
}
