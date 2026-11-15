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
	"errors"
	"image/jpeg"
	"strings"
	"testing"

	playwright "github.com/mxschmitt/playwright-go"

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

// fetchPage is a page whose Evaluate returns a canned fetch result and records its argument. Every
// other Page method is the embedded nil interface and panics if called.
type fetchPage struct {
	playwright.Page
	result any
	err    error
	arg    any
}

// Evaluate records the argument portalCall passes to its fetch script and returns the canned result.
func (p *fetchPage) Evaluate(_ string, arg ...any) (any, error) {
	if len(arg) > 0 {
		p.arg = arg[0]
	}
	return p.result, p.err
}

// TestPortalCallReturnsThePageResponse covers the browser-session exception to the funnel rule:
// portalCall hands the page the method, path and JSON payload, and returns the status and body the
// page's fetch produced, whether the status arrives as a float or an int.
func TestPortalCallReturnsThePageResponse(t *testing.T) {
	steps := &Steps{}
	for _, tc := range []struct {
		status any
		want   int
	}{{float64(201), 201}, {404, 404}} {
		page := &fetchPage{result: map[string]any{"status": tc.status, "body": `{"id":"x"}`}}
		got, body, err := steps.portalCall(page, "POST", "/subscriptions", map[string]string{"artifactId": "api"})
		if err != nil {
			t.Fatalf("portalCall() with status %v returned an error: %v", tc.status, err)
		}
		if got != tc.want || body != `{"id":"x"}` {
			t.Fatalf("portalCall() = %d %q, want %d and the page's body", got, body, tc.want)
		}
		arg, _ := page.arg.(map[string]any)
		if arg["method"] != "POST" || arg["path"] != "/subscriptions" || arg["payload"] != `{"artifactId":"api"}` {
			t.Fatalf("portalCall() passed %v to the page", page.arg)
		}
	}
	page := &fetchPage{err: errors.New("page closed")}
	if _, _, err := steps.portalCall(page, "GET", "/apis", nil); err == nil || !strings.Contains(err.Error(), "page closed") {
		t.Fatalf("portalCall() error = %v, want the page's error", err)
	}
	if arg, _ := page.arg.(map[string]any); arg["payload"] != "" {
		t.Fatalf("portalCall() without a body sent payload %v", arg["payload"])
	}
}

// TestCreatedIDNeedsTheNamedField checks that a create response yields its id only when the named
// field holds a non-empty string.
func TestCreatedIDNeedsTheNamedField(t *testing.T) {
	if id, err := createdID([]byte(`{"id":"plan-1","other":"x"}`), "id"); err != nil || id != "plan-1" {
		t.Fatalf("createdID() = %q, %v, want plan-1", id, err)
	}
	for _, body := range []string{`{"subscriptionId":"s"}`, `{"id":""}`, `{"id":7}`, `{"id":null}`, `not json`, `null`} {
		if id, err := createdID([]byte(body), "id"); err == nil {
			t.Fatalf("createdID(%s) = %q, want an error", body, id)
		}
	}
}

// TestPortalPathMatchesOnlyTheCollection checks that a create matcher matches the collection, with
// or without a query, and not a member of it.
func TestPortalPathMatchesOnlyTheCollection(t *testing.T) {
	path := portalPath("/subscriptions")
	for _, target := range []string{"https://h/api-portal/api/v0.9/subscriptions", "https://h/api-portal/api/v0.9/subscriptions?x=1"} {
		if !path.MatchString(target) {
			t.Fatalf("portalPath(/subscriptions) does not match %s", target)
		}
	}
	for _, target := range []string{"https://h/api-portal/api/v0.9/subscriptions/s1", "https://h/api-portal/api/v0x9/subscriptions"} {
		if path.MatchString(target) {
			t.Fatalf("portalPath(/subscriptions) matches %s", target)
		}
	}
}

// TestWizardJPEGIsADecodableImage checks that the JPEG thumbnail fixture is a complete image the
// portal's magic-byte check accepts.
func TestWizardJPEGIsADecodableImage(t *testing.T) {
	file := wizardJPEG()
	if file.Name != "photo.jpg" || file.MimeType != "image/jpeg" {
		t.Fatalf("wizardJPEG() = %s %s", file.Name, file.MimeType)
	}
	if !bytes.HasPrefix(file.Buffer, []byte{0xFF, 0xD8, 0xFF}) {
		t.Fatalf("wizardJPEG() does not start with the JPEG magic bytes: % x", file.Buffer[:3])
	}
	decoded, err := jpeg.Decode(bytes.NewReader(file.Buffer))
	if err != nil {
		t.Fatalf("wizardJPEG() is not a decodable JPEG: %v", err)
	}
	if bounds := decoded.Bounds(); bounds.Dx() != 1 || bounds.Dy() != 1 {
		t.Fatalf("wizardJPEG() is %dx%d, want 1x1", bounds.Dx(), bounds.Dy())
	}
}
