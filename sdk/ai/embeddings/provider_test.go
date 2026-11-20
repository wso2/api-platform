/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package embeddings

import (
	"strings"
	"testing"
)

func TestValidateEmbeddingProviderConfigProps(t *testing.T) {
	const endpoint = "https://r.openai.azure.com/openai/deployments/d/embeddings?api-version=2024-10-21"
	const guid = "00000000-0000-0000-0000-000000000001"
	azure := func(mod func(*EmbeddingProviderConfig)) EmbeddingProviderConfig {
		c := EmbeddingProviderConfig{EmbeddingProvider: "AZURE_OPENAI", EmbeddingEndpoint: endpoint}
		mod(&c)
		return c
	}
	tests := []struct {
		name    string
		config  EmbeddingProviderConfig
		wantErr string
	}{
		{"api-key, as today", azure(func(c *EmbeddingProviderConfig) { c.AuthHeaderName, c.APIKey = "api-key", "k" }), ""},
		{"explicit api-key", azure(func(c *EmbeddingProviderConfig) {
			c.AuthHeaderName, c.APIKey, c.AuthenticationType = "api-key", "k", "api-key"
		}), ""},
		{"api-key without key", azure(func(c *EmbeddingProviderConfig) { c.AuthHeaderName = "api-key" }), "missing API key"},
		{"api-key without header", azure(func(c *EmbeddingProviderConfig) { c.APIKey = "k" }), "missing auth header name"},
		{"workload identity, no key or header", azure(func(c *EmbeddingProviderConfig) { c.AuthenticationType = "workload-identity" }), ""},
		{"managed identity", azure(func(c *EmbeddingProviderConfig) { c.AuthenticationType = "managed-identity" }), ""},
		{"container apps", azure(func(c *EmbeddingProviderConfig) { c.AuthenticationType = "container-apps" }), ""},
		{"client ID GUID", azure(func(c *EmbeddingProviderConfig) { c.AuthenticationType, c.ClientID = "workload-identity", guid }), ""},
		{"client ID not GUID", azure(func(c *EmbeddingProviderConfig) { c.AuthenticationType, c.ClientID = "managed-identity", "abc" }), "GUID"},
		{"custom scope", azure(func(c *EmbeddingProviderConfig) {
			c.AuthenticationType, c.TokenScope = "container-apps", "https://ai.azure.com/.default"
		}), ""},
		{"scope without /.default", azure(func(c *EmbeddingProviderConfig) {
			c.AuthenticationType, c.TokenScope = "container-apps", "https://ai.azure.com"
		}), "/.default"},
		{"identity with OPENAI", azure(func(c *EmbeddingProviderConfig) {
			c.EmbeddingProvider, c.EmbeddingModel, c.AuthenticationType = "OPENAI", "m", "workload-identity"
		}), "workload-identity authentication is only supported for the AZURE_OPENAI embedding provider"},
		{"unknown type", azure(func(c *EmbeddingProviderConfig) { c.AuthenticationType = "client-secret" }), `"client-secret" is not supported`},
		{"unknown type with OPENAI", azure(func(c *EmbeddingProviderConfig) {
			c.EmbeddingProvider, c.EmbeddingModel, c.AuthenticationType = "OPENAI", "m", "client-secret"
		}), "is not supported"},
		{"identity, no endpoint", azure(func(c *EmbeddingProviderConfig) {
			c.AuthenticationType, c.EmbeddingEndpoint = "workload-identity", ""
		}), "missing embedding endpoint"},
		{"OPENAI without model, api-key", azure(func(c *EmbeddingProviderConfig) {
			c.EmbeddingProvider, c.AuthHeaderName, c.APIKey = "OPENAI", "Authorization", "k"
		}), "missing embedding model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmbeddingProviderConfigProps(tt.config)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestUsesIdentity(t *testing.T) {
	for typ, want := range map[string]bool{"": false, "api-key": false, "workload-identity": true} {
		if got := (EmbeddingProviderConfig{AuthenticationType: typ}).UsesIdentity(); got != want {
			t.Errorf("UsesIdentity(%q) = %v, want %v", typ, got, want)
		}
	}
}
