/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package apiportal

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	catalogapiportal "github.com/wso2/api-platform/tests/framework/core/catalog/apiportal"
	"github.com/wso2/api-platform/tests/framework/core/components"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func portalResponseContext(response *httpx.Response) context.Context {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("api-portal"))
	if err := tcontext.Set(ctx, keyAPIPortalResponse, response); err != nil {
		panic(err)
	}
	return ctx
}

func TestAPIPortalResponseStatus(t *testing.T) {
	ctx := portalResponseContext(&httpx.Response{StatusCode: 302})
	steps := &Steps{}

	if err := steps.apiPortalResponseStatus(ctx, "302"); err != nil {
		t.Fatalf("apiPortalResponseStatus() returned an error: %v", err)
	}
	if err := steps.apiPortalResponseStatus(ctx, "200"); err == nil {
		t.Fatal("apiPortalResponseStatus() accepted an unexpected status")
	}
}

func TestAPIPortalResponseBodyMatch(t *testing.T) {
	ctx := portalResponseContext(&httpx.Response{Body: []byte("plain response")})
	steps := &Steps{}

	if err := steps.apiPortalResponseBodyMatch(ctx, "plain", true); err != nil {
		t.Fatalf("apiPortalResponseBodyMatch() returned an error: %v", err)
	}
	if err := steps.apiPortalResponseBodyMatch(ctx, "html", false); err != nil {
		t.Fatalf("apiPortalResponseBodyMatch() negative assertion returned an error: %v", err)
	}
}

func TestPortalListingMatches(t *testing.T) {
	tests := []struct {
		name           string
		texts          []string
		mustContain    string
		mustNotContain string
		want           bool
	}{
		{name: "required value present", texts: []string{"REST API", "GraphQL API"}, mustContain: "GraphQL", want: true},
		{name: "required value absent", texts: []string{"REST API"}, mustContain: "GraphQL", want: false},
		{name: "excluded value absent", texts: []string{"REST API"}, mustNotContain: "GraphQL", want: true},
		{name: "excluded value present", texts: []string{"REST API", "GraphQL API"}, mustNotContain: "GraphQL", want: false},
		{name: "required present and excluded absent", texts: []string{"REST API", "GraphQL API"}, mustContain: "GraphQL", mustNotContain: "MCP", want: true},
		{name: "required present while excluded present", texts: []string{"REST API", "GraphQL API", "MCP Server"}, mustContain: "GraphQL", mustNotContain: "MCP", want: false},
		{name: "both expectations empty", texts: nil, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portalListingMatches(tt.texts, tt.mustContain, tt.mustNotContain); got != tt.want {
				t.Fatalf("portalListingMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBillingWebhookBaseNamesTheBlocksOwnPortal(t *testing.T) {
	portals := components.NewSet()
	inst, err := components.NewInstance(catalogapiportal.APIPortal(), 0, 1, "127.0.0.1", nil)
	if err != nil {
		t.Fatalf("NewInstance() returned an error: %v", err)
	}
	if err := portals.Add(inst); err != nil {
		t.Fatalf("Add() returned an error: %v", err)
	}
	steps := &Steps{topo: &frameworkruntime.Topology{Instances: portals}}

	// Every block's portal answers to the plain alias on the shared testbench's networks, so a
	// topology without its block network has no address to give Stripe.
	if got, err := steps.billingWebhookBase(); err == nil {
		t.Fatalf("billingWebhookBase() = %q, want an error without the block's network", got)
	}
}

// TestNavigationStatusRequiresAnOpenedPage checks that the navigation status is an error until a
// page has been opened, and is read back once stored.
func TestNavigationStatusRequiresAnOpenedPage(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("api-portal"))

	if _, err := navigationStatus(ctx); err == nil {
		t.Fatal("navigationStatus() returned no error before any page was opened")
	}
	if err := tcontext.Set(ctx, keyAPIPortalNavigationStatus, "404"); err != nil {
		t.Fatal(err)
	}
	if _, err := navigationStatus(ctx); err == nil {
		t.Fatal("navigationStatus() accepted a status that is not an int")
	}
	if err := tcontext.Set(ctx, keyAPIPortalNavigationStatus, 404); err != nil {
		t.Fatal(err)
	}
	if status, err := navigationStatus(ctx); err != nil || status != 404 {
		t.Fatalf("navigationStatus() = (%d, %v), want (404, nil)", status, err)
	}
}

// TestContextStringRejectsMissingEmptyAndNonStringValues checks that only a non-empty string is
// returned from the scenario context.
func TestContextStringRejectsMissingEmptyAndNonStringValues(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("api-portal"))

	if _, err := contextString(ctx, "missing"); err == nil {
		t.Fatal("contextString() returned no error for a missing value")
	}
	for name, value := range map[string]any{"empty": "", "number": 7} {
		if err := tcontext.Set(ctx, name, value); err != nil {
			t.Fatal(err)
		}
		if _, err := contextString(ctx, name); err == nil {
			t.Fatalf("contextString() accepted the %s value", name)
		}
	}
	if err := tcontext.Set(ctx, "set", "value"); err != nil {
		t.Fatal(err)
	}
	if got, err := contextString(ctx, "set"); err != nil || got != "value" {
		t.Fatalf("contextString() = (%q, %v), want (\"value\", nil)", got, err)
	}
}

// TestNumberOfConvertsEvaluatedNumbers checks that numeric values from page evaluation convert,
// and that a string does not.
func TestNumberOfConvertsEvaluatedNumbers(t *testing.T) {
	for _, value := range []any{float64(3), 3, int64(3)} {
		if got, err := numberOf(value); err != nil || got != 3 {
			t.Fatalf("numberOf(%T) = (%v, %v), want (3, nil)", value, got, err)
		}
	}
	if _, err := numberOf("3"); err == nil {
		t.Fatal("numberOf() accepted a string")
	}
	if _, err := numberOf(nil); err == nil {
		t.Fatal("numberOf() accepted nil")
	}
}

// TestCatalogueDefinitions checks the catalogue fixtures' definitions: an unsecured REST document,
// a keyed one, and the other API types.
func TestCatalogueDefinitions(t *testing.T) {
	rest := catalogueRESTDefinition("Pets")
	if rest["openapi"] != "3.0.3" || rest["security"] != nil {
		t.Fatalf("catalogueRESTDefinition() = %v, want an unsecured OpenAPI 3.0.3 document", rest)
	}
	keyed := apiKeyRESTDefinition("Pets")
	if keyed["security"] == nil || keyed["components"] == nil {
		t.Fatalf("apiKeyRESTDefinition() = %v, want API key security", keyed)
	}
	if catalogueRESTDefinition("Pets")["security"] != nil {
		t.Fatal("apiKeyRESTDefinition() changed the plain definition it starts from")
	}
	async := asyncDefinition("Events")
	channels, ok := async["channels"].(map[string]any)
	if !ok || channels[asyncChannel] == nil {
		t.Fatalf("asyncDefinition() channels = %v, want %s", async["channels"], asyncChannel)
	}
	arazzo := galleryArazzo("Flow")
	if arazzo["arazzo"] != "1.0.0" || arazzo["workflows"] == nil {
		t.Fatalf("galleryArazzo() = %v, want an Arazzo 1.0.0 document with a workflow", arazzo)
	}
}

// TestThemeArchiveHoldsAStylesheetOrOnlyAScript checks the two theme archives the journeys upload.
func TestThemeArchiveHoldsAStylesheetOrOnlyAScript(t *testing.T) {
	for _, scriptOnly := range []bool{false, true} {
		file, err := themeArchive(scriptOnly)
		if err != nil {
			t.Fatalf("themeArchive(%t) returned an error: %v", scriptOnly, err)
		}
		archive, err := zip.NewReader(bytes.NewReader(file.Buffer), int64(len(file.Buffer)))
		if err != nil {
			t.Fatalf("themeArchive(%t) is not a ZIP: %v", scriptOnly, err)
		}
		names := map[string]bool{}
		for _, entry := range archive.File {
			names[entry.Name] = true
			if !strings.HasPrefix(entry.Name, "theme/") {
				t.Fatalf("themeArchive(%t) entry %q is outside the theme folder", scriptOnly, entry.Name)
			}
		}
		if names["theme/styles/main.css"] == scriptOnly || names["theme/scripts/app.js"] != scriptOnly {
			t.Fatalf("themeArchive(%t) entries = %v", scriptOnly, names)
		}
	}
}

// TestJourneyNamingHelpers checks the names and paths the journeys derive from a view handle.
func TestJourneyNamingHelpers(t *testing.T) {
	name, description := llmText("view-1")
	if !strings.Contains(name, "view-1") || !strings.Contains(description, "view-1") {
		t.Fatalf("llmText() = (%q, %q), want both to name the view", name, description)
	}
	if got := llmsTxtPath("view-1"); got != "/api-portal/default/views/view-1/llms.txt" {
		t.Fatalf("llmsTxtPath() = %q", got)
	}
	values := organizationValues("x1")
	if len(values) != 4 || !strings.Contains(values["#org-owner-email"], "@") {
		t.Fatalf("organizationValues() = %v, want four fields with an email", values)
	}
}

// TestRegexpHelpersMatchWholeValues checks that the regexp helpers match whole class names and
// values, not substrings.
func TestRegexpHelpersMatchWholeValues(t *testing.T) {
	if !regexpWord("is-invalid").MatchString("form-control is-invalid") || regexpWord("invalid").MatchString("is-invalid") {
		t.Fatal("regexpWord() does not match whole class names")
	}
	if !regexpExact("600").MatchString("600") || regexpExact("600").MatchString("6000") {
		t.Fatal("regexpExact() does not match the whole value")
	}
	if !regexpEscapedSuffix("/a.b").MatchString("https://host/x/a.b") || regexpEscapedSuffix("/a.b").MatchString("https://host/x/aXb") {
		t.Fatal("regexpEscapedSuffix() does not escape its suffix")
	}
}
