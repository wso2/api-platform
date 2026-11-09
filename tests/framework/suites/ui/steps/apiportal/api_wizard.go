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
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keyAPIPortalWizardItem = "uiAPIPortalWizardItem"

	wizardNext        = "#cfg-wizard-next"
	wizardSaveTimeout = 20000 // the success toast shows for about 3s before the page reloads

	wizardPNGHex  = "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"
	wizardJPEGHex = "ffd8ffe000104a46494600010100000100010000ffd9"
	wizardSpec    = `{"openapi":"3.0.3","info":{"title":"Wizard","version":"1.0.0"},"paths":{}}`
	wizardTools   = `[{"type":"TOOL","name":"ping","description":"Ping.","inputSchema":{"type":"object","properties":{}}}]`
)

func (u *Steps) registerAPIWizardSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the user creates an API in the wizard with a thumbnail and documents$`, u.createWizardAPI)
	sc.Step(`^the wizard API row shows its thumbnail$`, u.wizardRowThumbnail)
	sc.Step(`^the user replaces the thumbnail and documents of the wizard API$`, u.replaceWizardThumbnailAndDocs)
	sc.Step(`^the wizard API shows the new thumbnail and documents$`, u.wizardShowsNewThumbnailAndDocs)
	sc.Step(`^the user creates an API in the wizard with a rejected content package$`, u.createWizardAPIWithRejectedPackage)
	sc.Step(`^the wizard lists every rejected entry and continues as an edit$`, u.wizardListsRejectedEntries)
	sc.Step(`^the user retries with a valid content package$`, u.retryWizardImport)
	sc.Step(`^the user cancels replacing the content of the wizard API$`, u.cancelWizardImport)
	sc.Step(`^the user confirms replacing the content of the wizard API$`, u.confirmWizardImport)
	sc.Step(`^the user creates an MCP server in the wizard$`, u.createWizardMCP)
	sc.Step(`^the wizard MCP server row shows its thumbnail$`, u.wizardRowThumbnail)
}

func wizardImage(encoded string) []byte {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		panic(fmt.Sprintf("wizard image fixture: %v", err))
	}
	return decoded
}

func wizardFile(name, mimeType string, content []byte) playwright.InputFile {
	return playwright.InputFile{Name: name, MimeType: mimeType, Buffer: content}
}

func wizardDoc(name string) playwright.InputFile {
	return wizardFile(name, "text/markdown", []byte("# "+name+"\n"))
}

func wizardZip(entries map[string]string) (playwright.InputFile, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for name, content := range entries {
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

func validWizardPackage() (playwright.InputFile, error) {
	return wizardZip(map[string]string{
		"web/api-content.md":  "# Landing page\n",
		"docs/Guide.md":       "# Guide\n",
		"docs/HowTo/Setup.md": "# Setup\n",
	})
}

// wizardHandle fits a generated name to the wizard's handle rule: lowercase letters, digits,
// hyphens and dots.
func wizardHandle(name string) string {
	return regexp.MustCompile(`[^a-z0-9.-]+`).ReplaceAllString(strings.ToLower(name), "-")
}

func wizardRow(page playwright.Page, id string) playwright.Locator {
	return page.Locator(`[id="cfg-api-row-` + id + `"]`)
}

func (u *Steps) wizardItem(ctx context.Context) (string, error) {
	value, ok := tcontext.Get(ctx, keyAPIPortalWizardItem)
	if !ok {
		return "", fmt.Errorf("no API was created in the wizard")
	}
	return value.(string), nil
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
	page, err := u.openSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="` + panel + `"]`).Click(); err != nil {
		return nil, fmt.Errorf("opening the %s panel: %w", panel, err)
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

// wizardToSpec picks the thumbnail and API type on Details and moves to the Spec step, checking
// that the type is required.
func (u *Steps) wizardToSpec(page playwright.Page) error {
	if err := page.Locator("#wz-thumb-input").SetInputFiles([]playwright.InputFile{
		wizardFile("logo.png", "image/png", wizardImage(wizardPNGHex)),
	}); err != nil {
		return fmt.Errorf("choosing the thumbnail: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#wz-thumb-tag")).ToHaveText("Replaced when you save"); err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#wz-type-error")).ToHaveText("API type is required."); err != nil {
		return err
	}
	if _, err := page.Locator("#wz-type").SelectOption(playwright.SelectOptionValues{Values: &[]string{"RestApi"}}); err != nil {
		return fmt.Errorf("choosing the API type: %w", err)
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#cfg-wstep-1")).ToBeVisible()
}

// wizardToAssets uploads the spec, refusing a file of the wrong type first, and moves to Assets.
func (u *Steps) wizardToAssets(page playwright.Page, spec string) error {
	specInput := page.Locator("#wz-spec-input")
	if err := specInput.SetInputFiles([]playwright.InputFile{wizardFile("notes.txt", "text/plain", []byte("notes"))}); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#wz-spec-chip")).Not().ToBeVisible(); err != nil {
		return err
	}
	if err := specInput.SetInputFiles([]playwright.InputFile{wizardFile("definition.json", "application/json", []byte(spec))}); err != nil {
		return fmt.Errorf("choosing the spec: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#wz-spec-chip")).ToContainText("definition.json"); err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#cfg-wstep-2")).ToBeVisible()
}

// wizardSaved waits for the page reload that follows a successful save.
func (u *Steps) wizardSaved(page playwright.Page) error {
	return u.expect.Locator(page.Locator("#cfg-apis-wizard")).ToBeHidden(
		playwright.LocatorAssertionsToBeHiddenOptions{Timeout: playwright.Float(wizardSaveTimeout)})
}

func (u *Steps) createWizardAPI(ctx context.Context) error {
	page, err := u.startWizard(ctx, "cfg-apis", "#cfg-add-api-btn", "api-wizard", KindAPIPortalAPI)
	if err != nil {
		return err
	}
	if err := u.wizardToSpec(page); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardSpec); err != nil {
		return err
	}
	if err := page.Locator("#wz-docs-input").SetInputFiles([]playwright.InputFile{
		wizardDoc("A.md"), wizardDoc("B.md"), wizardFile("notes.txt", "text/plain", []byte("notes")),
	}); err != nil {
		return fmt.Errorf("choosing the documents: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#wz-docs-list .cfg-docs-item")).ToHaveCount(2); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#wz-docs-summary")).ToHaveText("When you save: 2 documents will be added."); err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.wizardSaved(page)
}

func (u *Steps) wizardRowThumbnail(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(wizardRow(page, id).Locator("img.cfg-api-avatar--img")).ToBeVisible()
}

// openWizardEdit opens the wizard for the scenario's API and checks its type can't be changed.
func (u *Steps) openWizardEdit(ctx context.Context) (playwright.Page, error) {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return nil, err
	}
	page, err := u.openSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-apis"]`).Click(); err != nil {
		return nil, err
	}
	row := wizardRow(page, id)
	if err := row.Locator(".cfg-menu-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the row menu: %w", err)
	}
	if err := row.Locator(".cfg-edit-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the wizard for edit: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-wizard-title")).ToHaveText("Edit API"); err != nil {
		return nil, err
	}
	return page, u.expect.Locator(page.Locator("#wz-type")).ToBeDisabled()
}

func (u *Steps) wizardEditToAssets(page playwright.Page) error {
	for range 2 {
		if err := page.Locator(wizardNext).Click(); err != nil {
			return err
		}
	}
	return u.expect.Locator(page.Locator("#cfg-wstep-2")).ToBeVisible()
}

func (u *Steps) replaceWizardThumbnailAndDocs(ctx context.Context) error {
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-thumb-input").SetInputFiles([]playwright.InputFile{
		wizardFile("photo.jpg", "image/jpeg", wizardImage(wizardJPEGHex)),
	}); err != nil {
		return fmt.Errorf("replacing the thumbnail: %w", err)
	}
	if err := u.wizardEditToAssets(page); err != nil {
		return err
	}
	if err := page.Locator("#wz-docs-input").SetInputFiles([]playwright.InputFile{wizardDoc("A.md"), wizardDoc("C.md")}); err != nil {
		return fmt.Errorf("choosing the documents: %w", err)
	}
	existingB := page.Locator("#wz-existing-docs-list .cfg-docs-item", playwright.PageLocatorOptions{HasText: "B.md"})
	if err := existingB.Locator(".cfg-docs-remove").Click(); err != nil {
		return err
	}
	if err := existingB.Locator(".cfg-docs-undo").Click(); err != nil {
		return err
	}
	if err := existingB.Locator(".cfg-docs-remove").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#wz-docs-summary")).ToHaveText(
		"When you save: 1 document will be added, 1 replaced and 1 removed."); err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.wizardSaved(page)
}

func (u *Steps) wizardShowsNewThumbnailAndDocs(ctx context.Context) error {
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#wz-thumb-name")).ToHaveText("api-icon.jpg"); err != nil {
		return err
	}
	docs := page.Locator("#wz-existing-docs-list")
	if err := u.expect.Locator(docs).ToContainText("C.md"); err != nil {
		return err
	}
	return u.expect.Locator(docs).Not().ToContainText("B.md")
}

func (u *Steps) createWizardAPIWithRejectedPackage(ctx context.Context) error {
	page, err := u.startWizard(ctx, "cfg-apis", "#cfg-add-api-btn", "api-wizard-import", KindAPIPortalAPI)
	if err != nil {
		return err
	}
	if err := u.wizardToSpec(page); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardSpec); err != nil {
		return err
	}
	if err := page.Locator("#wz-assets-to-import").Click(); err != nil {
		return err
	}
	rejected, err := wizardZip(map[string]string{"doc/Setup.md": "# Setup\n", "web/style.css": "a{}"})
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-content-input").SetInputFiles([]playwright.InputFile{rejected}); err != nil {
		return fmt.Errorf("choosing the content package: %w", err)
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#wz-content-chip")).ToContainText("Not uploaded")
}

func (u *Steps) wizardListsRejectedEntries(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	errorText := u.expect.Locator(page.Locator("#wz-content-error"))
	if err := errorText.ToContainText("doc/: unknown top-level entry"); err != nil {
		return err
	}
	if err := errorText.ToContainText("web/style.css: web/ holds only api-content.md, api-content.hbs and images"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-wizard-title")).ToHaveText("Edit API"); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(wizardNext)).ToHaveText("Save changes")
}

func (u *Steps) retryWizardImport(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-content-input").SetInputFiles([]playwright.InputFile{valid}); err != nil {
		return fmt.Errorf("choosing the content package: %w", err)
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.wizardSaved(page)
}

func (u *Steps) cancelWizardImport(ctx context.Context) error {
	page, err := u.openWizardEdit(ctx)
	if err != nil {
		return err
	}
	if err := u.wizardEditToAssets(page); err != nil {
		return err
	}
	if err := page.Locator("#wz-assets-to-import").Click(); err != nil {
		return err
	}
	valid, err := validWizardPackage()
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-content-input").SetInputFiles([]playwright.InputFile{valid}); err != nil {
		return fmt.Errorf("choosing the content package: %w", err)
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	modal := page.Locator("#cfg-replace-content-modal")
	if err := u.expect.Locator(modal).ToContainText("Replace all content?"); err != nil {
		return err
	}
	if err := page.Locator("#cfg-replace-content-cancel").Click(); err != nil {
		return err
	}
	return u.expect.Locator(modal).ToBeHidden()
}

func (u *Steps) confirmWizardImport(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-replace-content-confirm").Click(); err != nil {
		return err
	}
	if err := u.wizardSaved(page); err != nil {
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

func (u *Steps) createWizardMCP(ctx context.Context) error {
	page, err := u.startWizard(ctx, "cfg-mcps", "#cfg-add-mcp-btn", "mcp-wizard", KindAPIPortalMCP)
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-thumb-input").SetInputFiles([]playwright.InputFile{
		wizardFile("logo.png", "image/png", wizardImage(wizardPNGHex)),
	}); err != nil {
		return fmt.Errorf("choosing the thumbnail: %w", err)
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	if err := u.wizardToAssets(page, wizardTools); err != nil {
		return err
	}
	if err := page.Locator("#wz-docs-input").SetInputFiles([]playwright.InputFile{wizardDoc("A.md")}); err != nil {
		return fmt.Errorf("choosing the documents: %w", err)
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return err
	}
	return u.wizardSaved(page)
}
