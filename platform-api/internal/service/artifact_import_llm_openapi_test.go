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

	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// Verifies the DP→CP (bottom-up) import path populates the OpenAPI spec:
//   - a provider imported from a template inherits the template's inline spec;
//   - a proxy imported against that provider inherits the provider's spec;
//   - when the template has no spec, the imported provider/proxy are left empty.
func TestImport_LLMProvider_And_Proxy_OpenAPISpec(t *testing.T) {
	const inlineSpec = "openapi: 3.0.3\ninfo:\n  title: Imported\n  version: v1.0\npaths: {}\n"

	t.Run("template inline spec flows to provider and proxy", func(t *testing.T) {
		d := setupImportTest(t)

		// Import the template, then give it an inline openapi_spec (the URL-fetch branch
		// cannot be exercised here — a test server binds loopback, which the SSRF guard
		// correctly refuses).
		mustImport(t, d, dpTemplateReq("dp-t", "oai-tmpl", "OpenAI"))
		tmpl, err := d.templateRepo.GetByID("oai-tmpl", importTestOrgID)
		if err != nil || tmpl == nil {
			t.Fatalf("load imported template: %v", err)
		}
		tmpl.OpenAPISpec = inlineSpec
		if err := d.templateRepo.Update(tmpl); err != nil {
			t.Fatalf("set inline spec on template: %v", err)
		}

		// Import provider → should inherit the template's inline spec.
		mustImport(t, d, dpProviderReq("dp-p", "oai-prov", "OpenAI Prov", "oai-tmpl"))
		prov, err := repository.NewLLMProviderRepo(d.db).GetByID("oai-prov", importTestOrgID)
		if err != nil || prov == nil {
			t.Fatalf("load imported provider: %v", err)
		}
		if prov.OpenAPISpec != inlineSpec {
			t.Fatalf("provider spec mismatch:\n got: %q\nwant: %q", prov.OpenAPISpec, inlineSpec)
		}

		// Import proxy → should inherit the provider's spec.
		mustImport(t, d, dpProxyReq("dp-x", "oai-proxy", "OpenAI Proxy", "oai-prov"))
		px, err := repository.NewLLMProxyRepo(d.db).GetByID("oai-proxy", importTestOrgID)
		if err != nil || px == nil {
			t.Fatalf("load imported proxy: %v", err)
		}
		if px.OpenAPISpec != inlineSpec {
			t.Fatalf("proxy spec mismatch:\n got: %q\nwant: %q", px.OpenAPISpec, inlineSpec)
		}
	})

	t.Run("template without spec leaves provider and proxy empty", func(t *testing.T) {
		d := setupImportTest(t)

		mustImport(t, d, dpTemplateReq("dp-t2", "bare-tmpl", "Bare"))
		mustImport(t, d, dpProviderReq("dp-p2", "bare-prov", "Bare Prov", "bare-tmpl"))
		prov, err := repository.NewLLMProviderRepo(d.db).GetByID("bare-prov", importTestOrgID)
		if err != nil || prov == nil {
			t.Fatalf("load imported provider: %v", err)
		}
		if prov.OpenAPISpec != "" {
			t.Fatalf("expected empty provider spec, got %q", prov.OpenAPISpec)
		}

		mustImport(t, d, dpProxyReq("dp-x2", "bare-proxy", "Bare Proxy", "bare-prov"))
		px, err := repository.NewLLMProxyRepo(d.db).GetByID("bare-proxy", importTestOrgID)
		if err != nil || px == nil {
			t.Fatalf("load imported proxy: %v", err)
		}
		if px.OpenAPISpec != "" {
			t.Fatalf("expected empty proxy spec, got %q", px.OpenAPISpec)
		}
	})
}
