// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

package apiportal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keyAPIPortalWizardItem = "uiAPIPortalWizardItem"

	wizardNext        = "#cfg-wizard-next"
	wizardSaveTimeout = 20000 // the success toast shows for about 3s before the page reloads

	wizardPanelAPIs = "cfg-apis"
	wizardPanelMCPs = "cfg-mcps"

	wizardCollectionAPIs = "apis"
	wizardCollectionMCPs = "mcp-servers"

	wizardAssetMarketing = "MARKETING"
	wizardAssetImage     = "IMAGE"
	wizardAssetDocOther  = "DOC_Other"
	wizardAssetDocHowTo  = "DOC_HowTo"
	wizardAssetSchema    = "SCHEMA_DEFINITION"

	wizardIconPNG = "api-icon.png"
	wizardIconJPG = "api-icon.jpg"
	wizardLanding = "api-content.md"

	wizardReplaceModal   = "#cfg-replace-content-modal"
	wizardReplaceCancel  = "#cfg-replace-content-cancel"
	wizardReplaceConfirm = "#cfg-replace-content-confirm"
	wizardToImport       = "#wz-assets-to-import"
	wizardContentInput   = "#wz-content-input"
	wizardContentChip    = "#wz-content-chip"
	wizardDocsInput      = "#wz-docs-input"
	wizardDocsSummary    = "#wz-docs-summary"
	wizardExistingDocs   = "#wz-existing-docs-list .cfg-docs-item"
	wizardThumbInput     = "#wz-thumb-input"
	wizardThumbName      = "#wz-thumb-name"
	wizardThumbTag       = "#wz-thumb-tag"
	wizardThumbRemove    = "#wz-thumb-remove"
	wizardThumbPreview   = "#wz-thumb-preview"
	wizardSpecInput      = "#wz-spec-input"
	wizardSpecChip       = "#wz-spec-chip"
	wizardSpecDesc       = "#wz-spec-desc"
	wizardStepLabels     = "#cfg-apis-wizard .cfg-step-label"

	wizardPNGHex  = "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c63606060f80f00010401005fe5c34b0000000049454e44ae426082"
	wizardJPEGHex = "ffd8ffe000104a46494600010100000100010000ffd9"
	wizardSpec    = `{"openapi":"3.0.3","info":{"title":"Wizard","version":"1.0.0"},"paths":{}}`
	wizardTools   = `[{"type":"TOOL","name":"ping","description":"Ping.","inputSchema":{"type":"object","properties":{}}},` +
		`{"type":"RESOURCE","name":"status","description":"Server status.","uri":"status://now","mimeType":"application/json"},` +
		`{"type":"PROMPT","name":"summarize","description":"Summarize a text.","arguments":[{"name":"text","required":true}]}]`

	wizardOpenAPIChip = "OpenAPI · uploaded when you save"
	wizardToolsChip   = "Tools schema · uploaded when you save"
)

var wizardChipShown = regexp.MustCompile(`(^|\s)visible(\s|$)`)

// wizardAsset is a stored asset a journey expects: its response status and, when body is set, a
// fragment of its content.
type wizardAsset struct {
	kind   string
	name   string
	status int
	body   string
}

// registerAPIWizardSteps binds the Settings wizard steps: creating, editing and importing content into APIs and MCP servers.
func (u *Steps) registerAPIWizardSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the user checks the details step of a new API in the wizard$`, u.checkWizardDetailsStep)
	sc.Step(`^the wizard accepts only the spec files of the chosen API type$`, u.wizardAcceptsSpecFilesOfType)
	sc.Step(`^the user creates an API in the wizard with a thumbnail and documents$`, u.createWizardAPI)
	sc.Step(`^the wizard API row shows its thumbnail$`, u.wizardRowThumbnail)
	sc.Step(`^the wizard marks the current spec of the wizard API when a new one is chosen$`, u.wizardMarksCurrentSpec)
	sc.Step(`^the wizard keeps the thumbnail of the wizard API when a new file is rejected$`, u.wizardKeepsThumbnailOnRejectedFile)
	sc.Step(`^switching between documents and import clears the pending changes of the wizard API$`, u.wizardSwitchClearsPendingChanges)
	sc.Step(`^the user replaces the thumbnail and documents of the wizard API$`, u.replaceWizardThumbnailAndDocs)
	sc.Step(`^the wizard API shows the new thumbnail and documents$`, u.wizardShowsNewThumbnailAndDocs)
	sc.Step(`^the user removes the thumbnail of the wizard API$`, u.removeWizardThumbnail)
	sc.Step(`^the wizard API row shows the first two letters of its name$`, u.wizardRowShowsNameBadge)
	sc.Step(`^the user creates an API in the wizard with a rejected content package$`, u.createWizardAPIWithRejectedPackage)
	sc.Step(`^the wizard lists every rejected entry and continues as an edit$`, u.wizardListsRejectedEntries)
	sc.Step(`^the user retries with a valid content package$`, u.retryWizardImport)
	sc.Step(`^the wizard shows a removed thumbnail as gone when the import after it is rejected$`, u.wizardShowsRemovedThumbnailAfterRejectedImport)
	sc.Step(`^the user creates an API in the wizard from a content package without a production URL$`, u.createWizardAPIFromPackage)
	sc.Step(`^the user cancels replacing the content of the wizard API$`, u.cancelWizardImport)
	sc.Step(`^the user confirms replacing the content of the wizard API$`, u.confirmWizardImport)
	sc.Step(`^a later documents-only edit keeps the imported content of the wizard API$`, u.wizardDocsEditKeepsImportedContent)
	sc.Step(`^a content package thumbnail replaces the thumbnail chosen in the same save$`, u.wizardPackageThumbnailWins)
	sc.Step(`^a new document replaces only the document of the same name in its own category$`, u.wizardDocumentReplacesOwnCategory)
	sc.Step(`^the user creates an MCP server in the wizard$`, u.createWizardMCP)
	sc.Step(`^the wizard MCP server row shows its thumbnail$`, u.wizardRowThumbnail)
	sc.Step(`^the wizard MCP server shows its thumbnail when edited$`, u.wizardMCPShowsThumbnailWhenEdited)
	sc.Step(`^the user imports a content package into the wizard MCP server$`, u.importPackageIntoWizardMCP)
}

// wizardImage decodes a hex image fixture; a malformed fixture is a programming error, so it panics.
func wizardImage(encoded string) []byte {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		panic(fmt.Sprintf("wizard image fixture: %v", err))
	}
	return decoded
}

// wizardFile builds an in-memory upload for a file input.
func wizardFile(name, mimeType string, content []byte) playwright.InputFile {
	return playwright.InputFile{Name: name, MimeType: mimeType, Buffer: content}
}

// wizardDoc builds a Markdown document upload whose heading is its name.
func wizardDoc(name string) playwright.InputFile {
	return wizardFile(name, "text/markdown", []byte("# "+name+"\n"))
}

// wizardNotes builds a plain-text upload, a type the wizard's image and spec inputs refuse.
func wizardNotes() playwright.InputFile {
	return wizardFile("notes.txt", "text/plain", []byte("notes"))
}

// wizardPNG builds a PNG thumbnail upload.
func wizardPNG() playwright.InputFile {
	return wizardFile("logo.png", "image/png", wizardImage(wizardPNGHex))
}

// wizardJPEG builds a JPEG thumbnail upload.
func wizardJPEG() playwright.InputFile {
	return wizardFile("photo.jpg", "image/jpeg", wizardImage(wizardJPEGHex))
}

// wizardZip builds a content package from entry names and contents, written in name order so
// the archive is the same on every run.
func wizardZip(entries map[string]string) (playwright.InputFile, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		content := entries[name]
		entry, err := archive.Create(name)
		if err != nil {
			return playwright.InputFile{}, fmt.Errorf("adding %s to the content package: %w", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			return playwright.InputFile{}, fmt.Errorf("writing %s to the content package: %w", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		return playwright.InputFile{}, fmt.Errorf("closing the content package: %w", err)
	}
	return wizardFile("content.zip", "application/zip", buf.Bytes()), nil
}

// validWizardPackage builds a content package the portal accepts: a landing page, an icon and
// documents at the top level and in a subfolder.
func validWizardPackage() (playwright.InputFile, error) {
	return wizardZip(map[string]string{
		"web/" + wizardLanding: "# Landing page\n",
		"web/" + wizardIconPNG: string(wizardImage(wizardPNGHex)),
		"docs/Guide.md":        "# Guide\n",
		"docs/HowTo/Setup.md":  "# Setup\n",
	})
}

// rejectedWizardPackage builds a content package with an unknown top-level folder and a
// stylesheet under web/, both of which the portal refuses.
func rejectedWizardPackage() (playwright.InputFile, error) {
	return wizardZip(map[string]string{"doc/Setup.md": "# Setup\n", "web/style.css": "a{}"})
}

// wizardHandle fits a generated name to the wizard's handle rule: lowercase letters, digits,
// hyphens and dots.
func wizardHandle(name string) string {
	return regexp.MustCompile(`[^a-z0-9.-]+`).ReplaceAllString(strings.ToLower(name), "-")
}

// wizardRow locates the Settings list row of the API or MCP server with the given id.
func wizardRow(page playwright.Page, id string) playwright.Locator {
	return page.Locator(`[id="cfg-api-row-` + id + `"]`)
}

// wizardAll runs a journey's actions and assertions in order and stops at the first failure.
func wizardAll(actions ...func() error) error {
	for _, action := range actions {
		if err := action(); err != nil {
			return err
		}
	}
	return nil
}

// wizardClick returns an action that clicks the selector.
func wizardClick(page playwright.Page, selector string) func() error {
	return func() error {
		if err := page.Locator(selector).Click(); err != nil {
			return fmt.Errorf("clicking %s: %w", selector, err)
		}
		return nil
	}
}

// wizardFill returns an action that types value into the selector.
func wizardFill(page playwright.Page, selector, value string) func() error {
	return func() error {
		if err := page.Locator(selector).Fill(value); err != nil {
			return fmt.Errorf("filling %s: %w", selector, err)
		}
		return nil
	}
}

// wizardSelect returns an action that selects value in the selector's drop-down.
func wizardSelect(page playwright.Page, selector, value string) func() error {
	return func() error {
		if _, err := page.Locator(selector).SelectOption(playwright.SelectOptionValues{Values: &[]string{value}}); err != nil {
			return fmt.Errorf("selecting %s in %s: %w", value, selector, err)
		}
		return nil
	}
}

// wizardChoose returns an action that sets the files of the selector's file input.
func wizardChoose(page playwright.Page, selector string, files ...playwright.InputFile) func() error {
	return func() error {
		if err := page.Locator(selector).SetInputFiles(files); err != nil {
			return fmt.Errorf("choosing files for %s: %w", selector, err)
		}
		return nil
	}
}

// wizardHasText returns an assertion that the selector's text equals expected (a string or pattern).
func (u *Steps) wizardHasText(page playwright.Page, selector string, expected any) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToHaveText(expected) }
}

// wizardContains returns an assertion that the selector's text contains expected.
func (u *Steps) wizardContains(page playwright.Page, selector, expected string) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToContainText(expected) }
}

// wizardVisible returns an assertion that the selector is visible.
func (u *Steps) wizardVisible(page playwright.Page, selector string) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToBeVisible() }
}

// wizardHidden returns an assertion that the selector is hidden or absent.
func (u *Steps) wizardHidden(page playwright.Page, selector string) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToBeHidden() }
}

// wizardHasAttribute returns an assertion that the selector's attribute equals expected.
func (u *Steps) wizardHasAttribute(page playwright.Page, selector, name string, expected any) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToHaveAttribute(name, expected) }
}

// wizardHasValue returns an assertion that the selector's input value equals expected.
func (u *Steps) wizardHasValue(page playwright.Page, selector, expected string) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToHaveValue(expected) }
}

// wizardFocused returns an assertion that the selector has keyboard focus.
func (u *Steps) wizardFocused(page playwright.Page, selector string) func() error {
	return func() error { return u.expect.Locator(page.Locator(selector)).ToBeFocused() }
}

// wizardChipIsShown returns an assertion that the content-package chip is shown.
func (u *Steps) wizardChipIsShown(page playwright.Page) func() error {
	return func() error { return u.expect.Locator(page.Locator(wizardContentChip)).ToHaveClass(wizardChipShown) }
}

// wizardChipIsNotShown returns an assertion that the content-package chip is not shown.
func (u *Steps) wizardChipIsNotShown(page playwright.Page) func() error {
	return func() error {
		return u.expect.Locator(page.Locator(wizardContentChip)).Not().ToHaveClass(wizardChipShown)
	}
}

// wizardErrorToast returns an assertion that the alert toast is an error showing exactly message.
func (u *Steps) wizardErrorToast(page playwright.Page, message string) func() error {
	return func() error {
		toast := page.Locator("#alertToast")
		if err := u.expect.Locator(toast).ToHaveClass(regexp.MustCompile(`(^|\s)error(\s|$)`)); err != nil {
			return err
		}
		return u.expect.Locator(toast.Locator(".alert-toast-message")).ToHaveText(message)
	}
}

// wizardPress sends one key to the focused control.
func wizardPress(page playwright.Page, key string) func() error {
	return func() error {
		if err := page.Keyboard().Press(key); err != nil {
			return fmt.Errorf("pressing %s: %w", key, err)
		}
		return nil
	}
}

// wizardSavedStep waits for the page reload that follows a successful save.
func (u *Steps) wizardSavedStep(page playwright.Page) func() error {
	return func() error { return u.wizardSaved(page) }
}

// wizardSaveFromDetails walks from the Details step through Spec and Assets, which saves.
func wizardSaveFromDetails(page playwright.Page) func() error {
	return func() error {
		return wizardAll(wizardClick(page, wizardNext), wizardClick(page, wizardNext), wizardClick(page, wizardNext))
	}
}

// assertWizardAssets reads each expected asset through the signed-in session.
func (u *Steps) assertWizardAssets(page playwright.Page, collection, id string, expected ...wizardAsset) error {
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	for _, want := range expected {
		target := fmt.Sprintf("%s/api-portal/api/v0.9/%s/%s/assets?type=%s&fileName=%s", base, collection,
			url.PathEscape(id), url.QueryEscape(want.kind), url.QueryEscape(want.name))
		response, err := page.Context().Request().Get(target)
		if err != nil {
			return fmt.Errorf("reading asset %s %s of %s: %w", want.kind, want.name, id, err)
		}
		if response.Status() != want.status {
			return fmt.Errorf("asset %s %s of %s returned HTTP %d, want %d", want.kind, want.name, id, response.Status(), want.status)
		}
		if want.body == "" {
			continue
		}
		body, err := response.Body()
		if err != nil {
			return fmt.Errorf("reading the body of asset %s %s of %s: %w", want.kind, want.name, id, err)
		}
		if !strings.Contains(string(body), want.body) {
			return fmt.Errorf("asset %s %s of %s does not contain %q", want.kind, want.name, id, want.body)
		}
	}
	return nil
}

// wizardStoredAPI reads the API's stored metadata through the signed-in session.
func (u *Steps) wizardStoredAPI(page playwright.Page, id string) (playwright.APIResponse, error) {
	base, err := u.apiPortalURL()
	if err != nil {
		return nil, err
	}
	response, err := page.Context().Request().Get(fmt.Sprintf("%s/api-portal/api/v0.9/apis/%s", base, url.PathEscape(id)))
	if err != nil {
		return nil, fmt.Errorf("reading API %s: %w", id, err)
	}
	return response, nil
}

func (u *Steps) wizardItem(ctx context.Context) (string, error) {
	value, ok := tcontext.Get(ctx, keyAPIPortalWizardItem)
	if !ok {
		return "", fmt.Errorf("no API was created in the wizard")
	}
	return value.(string), nil
}

// imageLoaded waits until the browser has decoded the image, which a broken file never reaches.
func (u *Steps) imageLoaded(ctx context.Context, image playwright.Locator) error {
	return retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		loaded, err := image.Evaluate(`element => element.complete && element.naturalWidth > 0`, nil)
		if err != nil {
			return false, retry.Transient(fmt.Errorf("reading the image state: %w", err))
		}
		done, _ := loaded.(bool)
		return done, nil
	}, func(done bool) bool { return done }, "waiting for the image to load")
}

// startWizard opens the create wizard from the given settings panel and fills the Details step.
// Cleanup is registered before anything is created, so a failure at any later step can't leak the
// resource; deleting one that was never created is a 404, which the deleter accepts.
func (u *Steps) startWizard(ctx context.Context, panel, addButton, prefix string, kind cleanup.Kind) (playwright.Page, error) {
	generated, err := unique.Unique(ctx, prefix)
	if err != nil {
		return nil, err
	}
	id := wizardHandle(generated)
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: kind, ID: id, Actor: u.topo.Admin.Username}); err != nil {
		return nil, fmt.Errorf("registering %s for cleanup: %w", id, err)
	}
	if err := tcontext.Set(ctx, keyAPIPortalWizardItem, id); err != nil {
		return nil, err
	}
	page, err := u.openWizardPanel(ctx, panel)
	if err != nil {
		return nil, err
	}
	if err := page.Locator(addButton).Click(); err != nil {
		return nil, fmt.Errorf("opening the wizard: %w", err)
	}
	for selector, value := range map[string]string{
		"#wz-name": "IT Wizard " + id, "#wz-version": "v1.0", "#wz-handle": id,
		"#wz-prod": "https://backend.example.invalid/" + id,
	} {
		if err := page.Locator(selector).Fill(value); err != nil {
			return nil, fmt.Errorf("filling %s: %w", selector, err)
		}
	}
	return page, nil
}

// openWizardPanel opens Settings and switches to the given list panel.
func (u *Steps) openWizardPanel(ctx context.Context, panel string) (playwright.Page, error) {
	page, err := u.openSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="` + panel + `"]`).Click(); err != nil {
		return nil, fmt.Errorf("opening the %s panel: %w", panel, err)
	}
	return page, nil
}

// wizardToSpec picks the thumbnail and API type on Details and moves to the Spec step, checking
// that the type is required and the production URL is not.
func (u *Steps) wizardToSpec(page playwright.Page) error {
	return wizardAll(
		wizardChoose(page, wizardThumbInput, wizardPNG()),
		u.wizardHasText(page, wizardThumbTag, "Replaced when you save"),
		wizardClick(page, wizardNext),
		u.wizardHasText(page, "#wz-type-error", "API type is required."),
		u.wizardHidden(page, "#wz-prod-error"),
		u.wizardVisible(page, "#cfg-wstep-0"),
		wizardSelect(page, "#wz-type", "RestApi"),
		u.wizardHidden(page, "#wz-type-error"),
		wizardClick(page, wizardNext),
		u.wizardVisible(page, "#cfg-wstep-1"),
	)
}

// wizardToAssets uploads the spec, refusing a file of the wrong type first, and moves to Assets.
func (u *Steps) wizardToAssets(page playwright.Page, spec, chip string) error {
	return wizardAll(
		wizardChoose(page, wizardSpecInput, wizardNotes()),
		u.wizardHidden(page, wizardSpecChip),
		wizardChoose(page, wizardSpecInput, wizardFile("definition.json", "application/json", []byte(spec))),
		u.wizardContains(page, wizardSpecChip, "definition.json"),
		u.wizardContains(page, wizardSpecChip, chip),
		wizardClick(page, wizardNext),
		u.wizardVisible(page, "#cfg-wstep-2"),
	)
}

// wizardSaved waits for the page reload that follows a successful save.
func (u *Steps) wizardSaved(page playwright.Page) error {
	return u.expect.Locator(page.Locator("#cfg-apis-wizard")).ToBeHidden(
		playwright.LocatorAssertionsToBeHiddenOptions{Timeout: playwright.Float(wizardSaveTimeout)})
}

// checkWizardDetailsStep walks the Details step of a new API without saving it.
func (u *Steps) checkWizardDetailsStep(ctx context.Context) error {
	page, err := u.openWizardPanel(ctx, wizardPanelAPIs)
	if err != nil {
		return err
	}
	return wizardAll(
		wizardClick(page, "#cfg-add-api-btn"),
		u.wizardHasText(page, "#cfg-wizard-title", "Create API"),
		u.wizardHasText(page, "#cfg-wstep-0 .cfg-form-card-title",
			[]string{"General", "Endpoints", "Labels and tags", "Ownership", "Subscription plans"}),
		func() error {
			return u.expect.Locator(page.Locator("#wz-card-general .cfg-form-group").First().Locator("#wz-thumb-row")).ToBeAttached()
		},
		func() error { return u.expect.Locator(page.Locator("#wz-card-general #wz-status")).ToBeAttached() },
		u.wizardHasText(page, wizardThumbName, "No thumbnail"),
		u.wizardHasText(page, "#wz-thumb-hint", "PNG or JPG"),
		u.wizardHasAttribute(page, wizardThumbInput, "accept", ".png,.jpg,.jpeg"),
		func() error { return page.Locator(wizardThumbInput).Focus() },
		u.wizardFocused(page, wizardThumbInput),
		u.wizardHidden(page, wizardThumbRemove),
		u.wizardHasText(page, `label[for="wz-type"] .required`, "*"),
		func() error { return u.expect.Locator(page.Locator(`label[for="wz-prod"] .required`)).ToHaveCount(0) },
		u.wizardHasValue(page, "#wz-type", ""),
		u.wizardHasText(page, `#wz-type option[value=""]`, "Select a type"),
		u.wizardHasValue(page, "#wz-status", "PUBLISHED"),
		wizardFill(page, "#wz-name", "Pets API"),
		wizardFill(page, "#wz-version", "v1.0"),
		u.wizardHasText(page, "#cfg-wizard-subtitle", "Pets API · v1.0"),
		u.wizardHasText(page, wizardThumbPreview, "PA"),
		func() error {
			return u.expect.Locator(page.Locator(wizardThumbPreview)).ToHaveClass(regexp.MustCompile(`(^|\s)cfg-api-avatar(\s|$)`))
		},
	)
}

// wizardAcceptsSpecFilesOfType continues the unsaved wizard from Details and checks the Spec step
// against each API type.
func (u *Steps) wizardAcceptsSpecFilesOfType(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	jsonSpec := wizardFile("definition.json", "application/json", []byte(wizardSpec))
	return wizardAll(
		wizardFill(page, "#wz-prod", "https://backend.example.invalid/pets"),
		wizardSelect(page, "#wz-type", "GRAPHQL"),
		wizardClick(page, wizardNext),
		u.wizardHasText(page, wizardSpecDesc, "The contract that defines this API. Required."),
		u.wizardHidden(page, "#wz-spec-onfile"),
		u.wizardContains(page, "#wz-spec-formats", "Accepted formats"),
		u.wizardHasText(page, "#wz-spec-format-name", "GraphQL SDL"),
		u.wizardHasText(page, "#wz-spec-format-exts", ".graphql .gql"),
		u.wizardHasAttribute(page, wizardSpecInput, "accept", ".graphql,.gql"),
		wizardClick(page, wizardNext),
		u.wizardVisible(page, "#wz-spec-error"),
		u.wizardVisible(page, "#cfg-wstep-1"),
		wizardChoose(page, wizardSpecInput, jsonSpec),
		u.wizardErrorToast(page, "The GraphQL SDL spec must be a .graphql or .gql file."),
		u.wizardHidden(page, wizardSpecChip),
		wizardChoose(page, wizardSpecInput, wizardFile("schema.graphql", "application/graphql", []byte("type Query { ping: String }"))),
		u.wizardContains(page, wizardSpecChip, "schema.graphql"),
		u.wizardContains(page, wizardSpecChip, "GraphQL SDL · uploaded when you save"),
		wizardClick(page, "#cfg-wizard-back-step"),
		wizardSelect(page, "#wz-type", "WS"),
		wizardClick(page, wizardNext),
		u.wizardHidden(page, wizardSpecChip),
		u.wizardHasText(page, "#wz-spec-format-name", "AsyncAPI"),
		u.wizardHasText(page, "#wz-spec-format-exts", ".json .yaml .yml"),
		u.wizardHasAttribute(page, wizardSpecInput, "accept", ".json,.yaml,.yml"),
		wizardChoose(page, wizardSpecInput, jsonSpec),
		u.wizardContains(page, wizardSpecChip, "AsyncAPI · uploaded when you save"),
		wizardClick(page, "#wz-spec-remove"),
		u.wizardHidden(page, wizardSpecChip),
	)
}

// createWizardAPI creates a REST API in the wizard with the default label, a plan, a tag, a
// thumbnail and two documents, checking each step's refusals on the way, and its stored assets.
func (u *Steps) createWizardAPI(ctx context.Context) error {
	page, err := u.startWizard(ctx, wizardPanelAPIs, "#cfg-add-api-btn", "api-wizard", KindAPIPortalAPI)
	if err != nil {
		return err
	}
	if err := u.wizardPickLabelPlanAndTag(page); err != nil {
		return err
	}
	if err := u.wizardToSpec(page); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardSpec, wizardOpenAPIChip); err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardHasText(page, wizardStepLabels, []string{"Details", "Spec", "Assets"}),
		u.wizardHasText(page, wizardNext, "Create API"),
		u.wizardHasText(page, "#wz-assets-desc-docs", "Add documents for this API. You can change them later by editing the API."),
		u.wizardHidden(page, "#wz-existing-docs-section"),
		u.wizardContains(page, wizardToImport, "Import content from ZIP"),
		u.wizardHasAttribute(page, wizardDocsInput, "accept", ".md"),
		u.wizardContains(page, "#wz-docs-zone", "Markdown (.md) files"),
		wizardClick(page, wizardToImport),
		wizardChoose(page, wizardContentInput, valid),
		u.wizardChipIsShown(page),
		wizardChoose(page, wizardContentInput, wizardNotes()),
		u.wizardHasText(page, "#wz-content-error", "Unsupported file type. Accepted: .zip"),
		u.wizardChipIsNotShown(page),
		wizardClick(page, "#wz-assets-to-docs"),
		wizardChoose(page, wizardDocsInput, wizardDoc("A.md"), wizardDoc("B.md"), wizardNotes()),
		u.wizardErrorToast(page, "Only Markdown (.md) documents are supported: notes.txt"),
		func() error { return u.expect.Locator(page.Locator("#wz-docs-list .cfg-docs-item")).ToHaveCount(2) },
		u.wizardContains(page, "#wz-docs-list", "New"),
		u.wizardHasText(page, wizardDocsSummary, "When you save: 2 documents will be added."),
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "A.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetDocOther, name: "B.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetDocOther, name: "notes.txt", status: http.StatusNotFound},
		wizardAsset{kind: wizardAssetDocOther, name: "Guide.md", status: http.StatusNotFound},
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK},
	)
}

// wizardRowThumbnail checks that the wizard item's Settings row shows its stored thumbnail and
// that the image loads.
func (u *Steps) wizardRowThumbnail(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := u.revealOnPage(page, wizardRow(page, id)); err != nil {
		return err
	}
	image := wizardRow(page, id).Locator("img.cfg-api-avatar--img")
	if err := u.expect.Locator(image).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(image).ToHaveAttribute("src",
		regexp.MustCompile(`/(apis|mcp-servers)/`+regexp.QuoteMeta(id)+`/assets`)); err != nil {
		return err
	}
	return u.imageLoaded(ctx, image)
}

// openWizardEdit opens the wizard for the scenario's API and checks its type can't be changed.
func (u *Steps) openWizardEdit(ctx context.Context) (playwright.Page, error) {
	page, err := u.openWizardEditIn(ctx, wizardPanelAPIs, "Edit API")
	if err != nil {
		return nil, err
	}
	return page, u.expect.Locator(page.Locator("#wz-type")).ToBeDisabled()
}

// openWizardEditIn opens the wizard for the scenario's item from the given settings panel.
func (u *Steps) openWizardEditIn(ctx context.Context, panel, title string) (playwright.Page, error) {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return nil, err
	}
	page, err := u.openWizardPanel(ctx, panel)
	if err != nil {
		return nil, err
	}
	row := wizardRow(page, id)
	if err := u.revealOnPage(page, row); err != nil {
		return nil, err
	}
	if err := row.Locator(".cfg-menu-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the row menu: %w", err)
	}
	if err := row.Locator(".cfg-edit-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the wizard for edit: %w", err)
	}
	return page, u.expect.Locator(page.Locator("#cfg-wizard-title")).ToHaveText(title)
}

func (u *Steps) wizardEditToAssets(page playwright.Page) error {
	for range 2 {
		if err := page.Locator(wizardNext).Click(); err != nil {
			return err
		}
	}
	return u.expect.Locator(page.Locator("#cfg-wstep-2")).ToBeVisible()
}

// wizardEditToAssetsStep returns wizardEditToAssets as an action for wizardAll.
func (u *Steps) wizardEditToAssetsStep(page playwright.Page) func() error {
	return func() error { return u.wizardEditToAssets(page) }
}

// wizardMarksCurrentSpec checks the Spec step of an existing API without saving it.
func (u *Steps) wizardMarksCurrentSpec(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	return wizardAll(
		u.wizardHasText(page, "#cfg-wizard-subtitle", "IT Wizard "+id+" · v1.0"),
		wizardClick(page, wizardNext),
		u.wizardHasText(page, wizardSpecDesc, "Upload a new spec to replace the current one."),
		u.wizardContains(page, "#wz-spec-onfile", "Current spec"),
		u.wizardContains(page, "#wz-spec-onfile", "On file for this API"),
		u.wizardHidden(page, "#wz-spec-replaced-tag"),
		wizardChoose(page, wizardSpecInput, wizardFile("definition.json", "application/json", []byte(wizardSpec))),
		u.wizardHasText(page, "#wz-spec-replaced-tag", "Will be replaced"),
		u.wizardContains(page, wizardSpecChip, wizardOpenAPIChip),
		wizardClick(page, "#wz-spec-remove"),
		u.wizardHidden(page, "#wz-spec-replaced-tag"),
		wizardClick(page, wizardNext),
		u.wizardVisible(page, "#cfg-wstep-2"),
	)
}

// wizardKeepsThumbnailOnRejectedFile saves a file that is named as an image but is not one.
func (u *Steps) wizardKeepsThumbnailOnRejectedFile(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardChoose(page, wizardThumbInput, wizardFile("logo.png", "image/png", []byte("not an image"))),
		wizardSaveFromDetails(page),
		u.wizardHasText(page, "#wz-thumb-error", "Thumbnail must be a PNG or JPEG image."),
		u.wizardVisible(page, "#cfg-wstep-0"),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK})
}

// wizardSwitchClearsPendingChanges leaves the wizard unsaved: going back to documents discards
// the package, and returning to import discards the document changes.
func (u *Steps) wizardSwitchClearsPendingChanges(ctx context.Context) error {
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	existingA := page.Locator(wizardExistingDocs, playwright.PageLocatorOptions{HasText: "A.md"})
	return wizardAll(
		u.wizardEditToAssetsStep(page),
		u.wizardHidden(page, "#wz-assets-desc-docs"),
		wizardChoose(page, wizardDocsInput, wizardDoc("D.md")),
		func() error { return existingA.Locator(".cfg-docs-remove").Click() },
		u.wizardHasText(page, wizardDocsSummary, "When you save: 1 document will be added and 1 removed."),
		wizardClick(page, wizardToImport),
		u.wizardHidden(page, "#wz-assets-panel-docs"),
		wizardChoose(page, wizardContentInput, valid),
		u.wizardChipIsShown(page),
		u.wizardContains(page, wizardContentChip, "Applied when you save"),
		u.wizardContains(page, "#wz-assets-to-docs", "Back to uploading documents"),
		wizardClick(page, "#wz-assets-to-docs"),
		func() error { return u.expect.Locator(page.Locator("#wz-docs-list .cfg-docs-item")).ToHaveCount(0) },
		func() error { return u.expect.Locator(existingA).Not().ToContainText("Will be removed") },
		u.wizardHasText(page, wizardDocsSummary, ""),
		wizardClick(page, wizardToImport),
		u.wizardChipIsNotShown(page),
	)
}

// replaceWizardThumbnailAndDocs edits the wizard API: it checks the current thumbnail, undoes a
// removal, then saves a new JPEG thumbnail and documents.
func (u *Steps) replaceWizardThumbnailAndDocs(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	previewImage := page.Locator(wizardThumbPreview + " img")
	newA := page.Locator("#wz-docs-list .cfg-docs-item", playwright.PageLocatorOptions{HasText: "A.md"})
	existingB := page.Locator(wizardExistingDocs, playwright.PageLocatorOptions{HasText: "B.md"})
	if err := wizardAll(
		u.wizardHasText(page, wizardThumbName, wizardIconPNG),
		func() error {
			return u.expect.Locator(previewImage).ToHaveAttribute("src", regexp.MustCompile(`fileName=`+regexp.QuoteMeta(wizardIconPNG)))
		},
		func() error { return u.imageLoaded(ctx, previewImage) },
		u.wizardHidden(page, wizardThumbTag),
		wizardClick(page, wizardThumbRemove),
		u.wizardHasText(page, wizardThumbTag, "Removed when you save"),
		wizardClick(page, "#wz-thumb-undo"),
		u.wizardHidden(page, wizardThumbTag),
		func() error { return u.expect.Locator(previewImage).ToBeAttached() },
		wizardClick(page, wizardThumbRemove),
		wizardChoose(page, wizardThumbInput, wizardJPEG()),
		u.wizardHasText(page, wizardThumbTag, "Replaced when you save"),
		u.wizardEditToAssetsStep(page),
		u.wizardContains(page, "#wz-existing-docs-section", "On this API"),
		wizardChoose(page, wizardDocsInput, wizardFile("A.md", "text/markdown", []byte("# New A\n")), wizardDoc("C.md")),
		func() error { return u.expect.Locator(newA).ToContainText("Replaces existing") },
		func() error { return existingB.Locator(".cfg-docs-remove").Click() },
		func() error { return u.expect.Locator(existingB).ToContainText("Will be removed") },
		func() error { return existingB.Locator(".cfg-docs-undo").Click() },
		func() error { return u.expect.Locator(existingB).Not().ToContainText("Will be removed") },
		func() error { return existingB.Locator(".cfg-docs-remove").Click() },
		u.wizardHasText(page, wizardDocsSummary, "When you save: 1 document will be added, 1 replaced and 1 removed."),
		u.wizardHasText(page, wizardNext, "Save changes"),
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "A.md", status: http.StatusOK, body: "New A"},
		wizardAsset{kind: wizardAssetDocOther, name: "B.md", status: http.StatusNotFound},
		wizardAsset{kind: wizardAssetDocOther, name: "C.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconJPG, status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusNotFound},
	)
}

// wizardShowsNewThumbnailAndDocs checks that the catalogue card and the wizard show the replaced
// thumbnail and documents of the wizard API.
func (u *Steps) wizardShowsNewThumbnailAndDocs(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	catalogue, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/apis", id); err != nil {
		return err
	}
	if err := u.expect.Locator(catalogue.Locator("#apiCard-"+id+" img.api-card-img")).ToHaveAttribute("src",
		regexp.MustCompile(`fileName=`+regexp.QuoteMeta(wizardIconJPG))); err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(wizardThumbName)).ToHaveText(wizardIconJPG); err != nil {
		return err
	}
	docs := page.Locator("#wz-existing-docs-list")
	if err := u.expect.Locator(docs).ToContainText("C.md"); err != nil {
		return err
	}
	return u.expect.Locator(docs).Not().ToContainText("B.md")
}

// removeWizardThumbnail removes the wizard API's thumbnail and checks the stored image is gone.
func (u *Steps) removeWizardThumbnail(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, wizardThumbRemove),
		wizardSaveFromDetails(page),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetImage, name: wizardIconJPG, status: http.StatusNotFound})
}

// wizardRowShowsNameBadge checks that the wizard API's row, now without a thumbnail, shows no
// image and a badge of its name's first two letters.
func (u *Steps) wizardRowShowsNameBadge(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardPanel(ctx, wizardPanelAPIs)
	if err != nil {
		return err
	}
	row := wizardRow(page, id)
	if err := u.expect.Locator(row.Locator("img.cfg-api-avatar--img")).ToHaveCount(0); err != nil {
		return err
	}
	// A list row without a thumbnail shows the first two letters of the API's name.
	return u.expect.Locator(row.Locator("span.cfg-api-avatar")).ToHaveText(wizardRowBadge("IT Wizard " + id))
}

// wizardRowBadge is the text badge a Settings list row shows for an item without a thumbnail.
func wizardRowBadge(name string) string {
	runes := []rune(name)
	if len(runes) > 2 {
		runes = runes[:2]
	}
	return strings.ToUpper(string(runes))
}

// createWizardAPIWithRejectedPackage creates an API in the wizard with a content package the
// portal refuses, and checks the chip reports it was not uploaded.
func (u *Steps) createWizardAPIWithRejectedPackage(ctx context.Context) error {
	page, err := u.startWizard(ctx, wizardPanelAPIs, "#cfg-add-api-btn", "api-wizard-import", KindAPIPortalAPI)
	if err != nil {
		return err
	}
	if err := u.wizardToSpec(page); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardSpec, wizardOpenAPIChip); err != nil {
		return err
	}
	rejected, err := rejectedWizardPackage()
	if err != nil {
		return err
	}
	return wizardAll(
		wizardClick(page, wizardToImport),
		wizardChoose(page, wizardContentInput, rejected),
		wizardClick(page, wizardNext),
		u.wizardContains(page, wizardContentChip, "Not uploaded"),
	)
}

// wizardListsRejectedEntries checks that the wizard lists each refused package entry, keeps the
// created API and continues as an edit.
func (u *Steps) wizardListsRejectedEntries(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardContains(page, "#wz-content-error", "doc/: unknown top-level entry"),
		u.wizardContains(page, "#wz-content-error", "web/style.css: web/ holds only api-content.md, api-content.hbs and images"),
		u.wizardContains(page, wizardContentChip, "content.zip"),
		u.wizardHasText(page, "#wz-content-choose", "Choose another file"),
		u.wizardHidden(page, "#wz-content-remove"),
		u.wizardVisible(page, "#cfg-wstep-2"),
		u.wizardHasText(page, "#cfg-wizard-title", "Edit API"),
		u.wizardHasText(page, wizardNext, "Save changes"),
	); err != nil {
		return err
	}
	response, err := u.wizardStoredAPI(page, id)
	if err != nil {
		return err
	}
	if response.Status() != http.StatusOK {
		return fmt.Errorf("API %s returned HTTP %d after its package was rejected, want 200", id, response.Status())
	}
	return nil
}

// retryWizardImport chooses a valid content package after a refused one and saves it.
func (u *Steps) retryWizardImport(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardChoose(page, wizardContentInput, valid),
		u.wizardContains(page, wizardContentChip, "Applied when you save"),
		func() error {
			return u.expect.Locator(page.Locator(wizardContentChip)).Not().ToContainText("Not uploaded")
		},
		u.wizardHidden(page, "#wz-content-error"),
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetMarketing, name: wizardLanding, status: http.StatusOK},
		wizardAsset{kind: wizardAssetDocHowTo, name: "Setup.md", status: http.StatusOK},
	)
}

// wizardShowsRemovedThumbnailAfterRejectedImport removes the thumbnail, saves with a refused
// package, and checks the thumbnail is shown as gone rather than restored.
func (u *Steps) wizardShowsRemovedThumbnailAfterRejectedImport(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	rejected, err := rejectedWizardPackage()
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, wizardThumbRemove),
		u.wizardEditToAssetsStep(page),
		wizardClick(page, wizardToImport),
		wizardChoose(page, wizardContentInput, rejected),
		wizardClick(page, wizardNext),
		wizardClick(page, wizardReplaceConfirm),
		u.wizardContains(page, wizardContentChip, "Not uploaded"),
		u.wizardHasText(page, wizardThumbName, "No thumbnail"),
		u.wizardHasAttribute(page, wizardThumbTag, "hidden", ""),
		u.wizardHasAttribute(page, "#wz-thumb-undo", "hidden", ""),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusNotFound})
}

// createWizardAPIFromPackage imports a package while creating an API, which has no content to
// replace and so never asks, and leaves the optional production URL empty.
func (u *Steps) createWizardAPIFromPackage(ctx context.Context) error {
	page, err := u.startWizard(ctx, wizardPanelAPIs, "#cfg-add-api-btn", "api-wizard-package", KindAPIPortalAPI)
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-prod").Fill(""); err != nil {
		return fmt.Errorf("clearing the production URL: %w", err)
	}
	if err := u.wizardToSpec(page); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardSpec, wizardOpenAPIChip); err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardClick(page, wizardToImport),
		u.wizardContains(page, "#wz-assets-panel-import", "Import a content package"),
		u.wizardContains(page, "#wz-assets-panel-import", "Expected ZIP layout"),
		u.wizardHidden(page, "#wz-assets-desc-import"),
		wizardChoose(page, wizardContentInput, valid),
		wizardClick(page, wizardNext),
		u.wizardHidden(page, wizardReplaceModal),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	if err := u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetMarketing, name: wizardLanding, status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK},
		wizardAsset{kind: wizardAssetDocOther, name: "Guide.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetDocHowTo, name: "Setup.md", status: http.StatusOK},
	); err != nil {
		return err
	}
	return u.assertWizardAPIHasNoProductionURL(page, id)
}

// assertWizardAPIHasNoProductionURL reads the stored API and fails if it has a production URL.
func (u *Steps) assertWizardAPIHasNoProductionURL(page playwright.Page, id string) error {
	response, err := u.wizardStoredAPI(page, id)
	if err != nil {
		return err
	}
	if response.Status() != http.StatusOK {
		return fmt.Errorf("API %s returned HTTP %d, want 200", id, response.Status())
	}
	body, err := response.Body()
	if err != nil {
		return fmt.Errorf("reading API %s: %w", id, err)
	}
	var stored struct {
		EndPoints struct {
			ProductionURL string `json:"productionURL"`
		} `json:"endPoints"`
	}
	if err := json.Unmarshal(body, &stored); err != nil {
		return fmt.Errorf("decoding API %s: %w", id, err)
	}
	if stored.EndPoints.ProductionURL != "" {
		return fmt.Errorf("API %s stored production URL %q, want none", id, stored.EndPoints.ProductionURL)
	}
	return nil
}

// cancelWizardImport chooses a content package for the wizard API, cancels the replace dialog,
// and checks the API's content is unchanged.
func (u *Steps) cancelWizardImport(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardEditToAssetsStep(page),
		wizardClick(page, wizardToImport),
		u.wizardHasText(page, "#wz-assets-desc-import",
			"Saving removes the current documents, landing page and thumbnail, and uses this ZIP’s content instead."),
		wizardChoose(page, wizardContentInput, valid),
		wizardClick(page, wizardNext),
		u.wizardContains(page, wizardReplaceModal, "Replace all content?"),
		u.wizardContains(page, wizardReplaceModal, "the thumbnail of IT Wizard "+id+" with the content of content.zip"),
		u.wizardFocused(page, wizardReplaceCancel),
		wizardPress(page, "Tab"),
		u.wizardFocused(page, wizardReplaceConfirm),
		wizardPress(page, "Shift+Tab"),
		u.wizardFocused(page, wizardReplaceCancel),
		wizardClick(page, wizardReplaceCancel),
		u.wizardHidden(page, wizardReplaceModal),
		u.wizardFocused(page, wizardNext),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "A.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetMarketing, name: wizardLanding, status: http.StatusNotFound},
	)
}

// confirmImportHoldingSave confirms the import with the save request held, so the locked page is
// observed before the request is released.
func (u *Steps) confirmImportHoldingSave(ctx context.Context, page playwright.Page, id string) error {
	held := make(chan playwright.Route, 1)
	saveURL := regexp.MustCompile(`/api/v0\.9/apis/` + regexp.QuoteMeta(url.PathEscape(id)) + `$`)
	if err := page.Route(saveURL, func(route playwright.Route) {
		if route.Request().Method() != http.MethodPut {
			_ = route.Continue()
			return
		}
		held <- route
	}); err != nil {
		return fmt.Errorf("holding the save request: %w", err)
	}
	defer func() { _ = page.Unroute(saveURL) }()
	if err := wizardAll(
		u.wizardHasText(page, wizardReplaceConfirm, "Replace content"),
		wizardClick(page, wizardReplaceConfirm),
	); err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, wizardSaveTimeout*time.Millisecond)
	defer cancel()
	select {
	case route := <-held:
		locked := u.expect.Locator(page.Locator(".cfg-layout")).ToHaveJSProperty("inert", true)
		if err := route.Continue(); err != nil {
			return fmt.Errorf("releasing the save request: %w", err)
		}
		return locked
	case <-waitCtx.Done():
		return fmt.Errorf("the save request was not sent: %w", waitCtx.Err())
	}
}

// confirmWizardImport confirms the replace dialog and checks the package replaced the API's
// documents and thumbnail.
func (u *Steps) confirmWizardImport(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	if err := u.confirmImportHoldingSave(ctx, page, id); err != nil {
		return err
	}
	if err := u.wizardSaved(page); err != nil {
		return err
	}
	if err := u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "A.md", status: http.StatusNotFound},
		wizardAsset{kind: wizardAssetDocOther, name: "Guide.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK},
	); err != nil {
		return err
	}
	page, err = u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	docs := page.Locator("#wz-existing-docs-list")
	if err := u.expect.Locator(docs).ToContainText("Guide.md"); err != nil {
		return err
	}
	return u.expect.Locator(docs).Not().ToContainText("A.md")
}

// wizardDocsEditKeepsImportedContent adds a document to the wizard API and checks the imported
// landing page and thumbnail are kept.
func (u *Steps) wizardDocsEditKeepsImportedContent(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardEditToAssetsStep(page),
		wizardChoose(page, wizardDocsInput, wizardDoc("C.md")),
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "C.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetMarketing, name: wizardLanding, status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK},
	)
}

// wizardPackageThumbnailWins saves a picked JPEG together with a package whose thumbnail is a PNG.
func (u *Steps) wizardPackageThumbnailWins(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardChoose(page, wizardThumbInput, wizardJPEG()),
		u.wizardEditToAssetsStep(page),
		wizardClick(page, wizardToImport),
		wizardChoose(page, wizardContentInput, valid),
		wizardClick(page, wizardNext),
		wizardClick(page, wizardReplaceConfirm),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconJPG, status: http.StatusNotFound},
	)
}

// wizardDocumentReplacesOwnCategory imports the same file name into two categories, replaces the
// Other copy, then removes the How-To copy.
func (u *Steps) wizardDocumentReplacesOwnCategory(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	categories, err := wizardZip(map[string]string{"docs/Guide.md": "# Other\n", "docs/HowTo/Guide.md": "# HowTo\n"})
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardEditToAssetsStep(page),
		wizardClick(page, wizardToImport),
		wizardChoose(page, wizardContentInput, categories),
		wizardClick(page, wizardNext),
		wizardClick(page, wizardReplaceConfirm),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	if page, err = u.openWizardEdit(ctx); err != nil {
		return err
	}
	howTo := page.Locator(wizardExistingDocs, playwright.PageLocatorOptions{HasText: "HowTo/Guide.md"})
	if err := wizardAll(
		u.wizardEditToAssetsStep(page),
		func() error { return u.expect.Locator(howTo).ToBeAttached() },
		wizardChoose(page, wizardDocsInput, wizardFile("Guide.md", "text/markdown", []byte("# New\n"))),
		u.wizardHasText(page, wizardDocsSummary, "When you save: 1 document will be replaced."),
		func() error { return u.expect.Locator(howTo).ToBeAttached() },
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	if err := u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "Guide.md", status: http.StatusOK, body: "New"},
		wizardAsset{kind: wizardAssetDocHowTo, name: "Guide.md", status: http.StatusOK, body: "HowTo"},
	); err != nil {
		return err
	}
	if page, err = u.openWizardEdit(ctx); err != nil {
		return err
	}
	howTo = page.Locator(wizardExistingDocs, playwright.PageLocatorOptions{HasText: "HowTo/Guide.md"})
	if err := wizardAll(
		u.wizardEditToAssetsStep(page),
		func() error { return howTo.Locator(".cfg-docs-remove").Click() },
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionAPIs, id,
		wizardAsset{kind: wizardAssetDocHowTo, name: "Guide.md", status: http.StatusNotFound},
		wizardAsset{kind: wizardAssetDocOther, name: "Guide.md", status: http.StatusOK},
	)
}

// createWizardMCP creates an MCP server in the wizard with the default label, a plan, a tag, a
// thumbnail, a tools schema and a document, and checks its stored assets.
func (u *Steps) createWizardMCP(ctx context.Context) error {
	page, err := u.startWizard(ctx, wizardPanelMCPs, "#cfg-add-mcp-btn", "mcp-wizard", KindAPIPortalMCP)
	if err != nil {
		return err
	}
	if err := u.wizardPickLabelPlanAndTag(page); err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardContains(page, "#wz-card-general", "How this MCP server is named and shown in the portal."),
		wizardChoose(page, wizardThumbInput, wizardPNG()),
		u.wizardHasText(page, wizardThumbTag, "Replaced when you save"),
		wizardClick(page, wizardNext),
		u.wizardContains(page, wizardSpecDesc, "Upload the MCP tools schema that defines this server’s tools."),
		u.wizardHidden(page, "#wz-spec-formats"),
		u.wizardHasAttribute(page, wizardSpecInput, "accept", ".json,.yaml,.yml"),
	); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardTools, wizardToolsChip); err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardHasText(page, wizardStepLabels, []string{"Details", "Tools Schema", "Assets"}),
		u.wizardHasText(page, wizardNext, "Create MCP server"),
		u.wizardContains(page, "#wz-assets-panel-docs", "Add documents for this MCP server. You can change them later by editing the MCP server."),
		wizardChoose(page, wizardDocsInput, wizardDoc("A.md")),
		wizardClick(page, wizardNext),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionMCPs, id,
		wizardAsset{kind: wizardAssetDocOther, name: "A.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetImage, name: wizardIconPNG, status: http.StatusOK},
	)
}

// wizardMCPShowsThumbnailWhenEdited checks that editing the wizard MCP server previews its stored
// thumbnail and that the image loads.
func (u *Steps) wizardMCPShowsThumbnailWhenEdited(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEditIn(ctx, wizardPanelMCPs, "Edit MCP Server")
	if err != nil {
		return err
	}
	preview := page.Locator(wizardThumbPreview + " img")
	if err := u.expect.Locator(preview).ToHaveAttribute("src",
		regexp.MustCompile(`/mcp-servers/`+regexp.QuoteMeta(id)+`/assets`)); err != nil {
		return err
	}
	return u.imageLoaded(ctx, preview)
}

// importPackageIntoWizardMCP imports a valid content package into the wizard MCP server through
// the replace dialog.
func (u *Steps) importPackageIntoWizardMCP(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardEditIn(ctx, wizardPanelMCPs, "Edit MCP Server")
	if err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := wizardAll(
		u.wizardEditToAssetsStep(page),
		wizardClick(page, wizardToImport),
		wizardChoose(page, wizardContentInput, valid),
		wizardClick(page, wizardNext),
		u.wizardContains(page, wizardReplaceModal, "the thumbnail of IT Wizard "+id+" with the content of content.zip"),
		wizardClick(page, wizardReplaceConfirm),
		u.wizardSavedStep(page),
	); err != nil {
		return err
	}
	return u.assertWizardAssets(page, wizardCollectionMCPs, id,
		wizardAsset{kind: wizardAssetMarketing, name: wizardLanding, status: http.StatusOK},
		wizardAsset{kind: wizardAssetDocOther, name: "Guide.md", status: http.StatusOK},
		wizardAsset{kind: wizardAssetSchema, name: "definition.json", status: http.StatusOK},
	)
}
