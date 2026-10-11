/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"strings"
	"testing"
	"time"
)

func docsAssistantConfig(d DocsAssistantConfig) *Config {
	c := validatableConfig("")
	c.DocsAssistant = d
	c.DocsAssistant.normalize()
	return c
}

func validDocsAssistant() DocsAssistantConfig {
	return DocsAssistantConfig{
		Endpoint: "https://assistant.example.com/godzilla/ai-docs-assistant/v1.0",
		APIKey:   "key",
		Product:  "bijira",
		Branch:   "main",
		Timeout:  60 * time.Second,
	}
}

func TestValidateDocsAssistant(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(d *DocsAssistantConfig)
		wantErr string
	}{
		{"valid", func(d *DocsAssistantConfig) {}, ""},
		{"off when the endpoint is empty, even with nothing else set",
			func(d *DocsAssistantConfig) { *d = DocsAssistantConfig{} }, ""},
		{"http endpoint", func(d *DocsAssistantConfig) { d.Endpoint = "http://assistant.example.com/" }, "endpoint"},
		{"relative endpoint", func(d *DocsAssistantConfig) { d.Endpoint = "/chat" }, "endpoint"},
		{"missing api key", func(d *DocsAssistantConfig) { d.APIKey = "" }, "api_key"},
		{"missing product", func(d *DocsAssistantConfig) { d.Product = " " }, "product"},
		{"missing branch", func(d *DocsAssistantConfig) { d.Branch = "" }, "branch"},
		{"timeout too short", func(d *DocsAssistantConfig) { d.Timeout = 500 * time.Millisecond }, "timeout"},
		{"timeout too long", func(d *DocsAssistantConfig) { d.Timeout = 3 * time.Minute }, "timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDocsAssistant()
			tc.mutate(&d)
			err := docsAssistantConfig(d).validateDocsAssistant()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() error = %v, want none", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "[docs_assistant]") || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validate() error = %v, want a [docs_assistant] %s error", err, tc.wantErr)
			}
		})
	}
}

// validate() runs the docs assistant checks, so a bad section fails startup.
func TestValidateRunsDocsAssistantChecks(t *testing.T) {
	d := validDocsAssistant()
	d.APIKey = ""
	err := docsAssistantConfig(d).validate()
	if err == nil || !strings.Contains(err.Error(), "[docs_assistant] api_key") {
		t.Fatalf("validate() error = %v, want the [docs_assistant] api_key error", err)
	}
}

// A rejected endpoint is reported by scheme and host only: its path could carry
// something an operator would not want in a startup log.
func TestValidateDocsAssistantRedactsEndpoint(t *testing.T) {
	d := validDocsAssistant()
	d.Endpoint = "http://assistant.example.com/private/path"
	err := docsAssistantConfig(d).validateDocsAssistant()
	if err == nil || strings.Contains(err.Error(), "private/path") {
		t.Fatalf("validate() error = %v, want an error without the endpoint path", err)
	}
}

func TestNormalizeDocsAssistantEndpointEndsWithOneSlash(t *testing.T) {
	for _, in := range []string{
		"https://assistant.example.com/v1.0",
		"https://assistant.example.com/v1.0/",
		" https://assistant.example.com/v1.0// ",
	} {
		d := validDocsAssistant()
		d.Endpoint = in
		if got := docsAssistantConfig(d).DocsAssistant.Endpoint; got != "https://assistant.example.com/v1.0/" {
			t.Errorf("endpoint %q normalized to %q, want a single trailing slash", in, got)
		}
	}
}

// Only a bare "enabled" flag reaches the browser; the endpoint, key, product and
// branch never do.
func TestLoad_DocsAssistantRuntimeConfig(t *testing.T) {
	base := `
[ai_workspace.control_plane]
url = "https://platform-api:9243"
`
	t.Run("off", func(t *testing.T) {
		cfg, err := Load(writeConfig(t, base))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if _, ok := cfg.RuntimeConfig["APIP_AIW_DOCS_ASSISTANT_ENABLED"]; ok {
			t.Errorf("APIP_AIW_DOCS_ASSISTANT_ENABLED is set while [docs_assistant] is not configured")
		}
	})

	t.Run("on", func(t *testing.T) {
		cfg, err := Load(writeConfig(t, base+`
[ai_workspace.docs_assistant]
endpoint = "https://assistant.example.com/godzilla/ai-docs-assistant/v1.0"
api_key  = "docs-s3cr3t"
product  = "bijira-product"
branch   = "main-branch"
`))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got := cfg.RuntimeConfig["APIP_AIW_DOCS_ASSISTANT_ENABLED"]; got != "true" {
			t.Errorf("APIP_AIW_DOCS_ASSISTANT_ENABLED = %q, want \"true\"", got)
		}
		if cfg.DocsAssistant.Timeout != 60*time.Second {
			t.Errorf("timeout = %s, want the 60s default", cfg.DocsAssistant.Timeout)
		}
		for k, v := range cfg.RuntimeConfig {
			for _, secret := range []string{"docs-s3cr3t", "assistant.example.com", "bijira-product", "main-branch"} {
				if strings.Contains(v, secret) {
					t.Errorf("runtime config %s leaked a server-side docs assistant value: %q", k, v)
				}
			}
		}
	})
}
