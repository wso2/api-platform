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
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package apiportal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keyScopedLabelAPI   = "uiAPIPortalScopedLabelAPI"
	keyScopedDefaultAPI = "uiAPIPortalScopedDefaultAPI"
	keyThemedView       = "uiAPIPortalThemedView"
	keyUnthemedView     = "uiAPIPortalUnthemedView"

	// themeColour is the body colour the journey's theme sets, and themeColourRGB how the browser
	// reports it.
	themeColour    = "#123456"
	themeColourRGB = "rgb(18, 52, 86)"
)

// registerContentSteps wires the views, labels and theming journey steps.
func (u *Steps) registerContentSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal has an API with the settings label and an API with only the default label$`, u.seedLabelScopedAPIs)
	sc.Step(`^the settings view lists only the API with its label$`, u.viewListsOnlyLabelledAPI)
	sc.Step(`^the user adds the default label to the settings view$`, u.addDefaultLabelToView)
	sc.Step(`^the settings view lists both APIs$`, u.viewListsBothAPIs)
	sc.Step(`^the old address of the renamed view is not found$`, u.oldViewAddressIsNotFound)
	sc.Step(`^the user renames the settings label$`, u.renameSettingsLabel)
	sc.Step(`^the user deletes the settings label, which an API uses$`, u.deleteUsedLabel)
	sc.Step(`^the API that used the deleted label drops out of the settings view$`, u.apiDropsOutOfView)
	sc.Step(`^the user deletes the settings view$`, u.deleteSettingsView)
	sc.Step(`^the deleted view's address is not found$`, u.deletedViewIsNotFound)
	sc.Step(`^the API Portal has two views to theme$`, u.seedThemeViews)
	sc.Step(`^the user applies a theme to the first view in the settings$`, u.applyTheme)
	sc.Step(`^the first view shows the theme, and the second view keeps the default look$`, u.themeShowsOnFirstViewOnly)
	sc.Step(`^the downloaded theme of the first view holds the applied stylesheet$`, u.downloadedThemeMatches)
	sc.Step(`^a theme that holds only a script is refused, and the first view keeps its theme$`, u.scriptThemeRefused)
	sc.Step(`^the user resets the theme of the first view$`, u.resetTheme)
	sc.Step(`^the first view is back to the default look$`, u.firstViewIsDefault)
}

// seedLabelScopedAPIs creates one API carrying only the settings label and one carrying only the
// default label.
func (u *Steps) seedLabelScopedAPIs(ctx context.Context) error {
	label, err := contextString(ctx, keyAPIPortalSettingsLabel)
	if err != nil {
		return err
	}
	for key, labels := range map[string][]string{keyScopedLabelAPI: {label}, keyScopedDefaultAPI: {"default"}} {
		id, err := unique.Unique(ctx, "label-scoped")
		if err != nil {
			return err
		}
		if err := u.createLabelledAPI(ctx, id, labels); err != nil {
			return err
		}
		if err := tcontext.Set(ctx, key, id); err != nil {
			return err
		}
	}
	return nil
}

// createLabelledAPI creates a REST API with the given labels and registers it for cleanup.
func (u *Steps) createLabelledAPI(ctx context.Context, id string, labels []string) error {
	name := "IT Label Scoped " + id
	metadata := map[string]any{
		"id": id, "name": name, "version": "v1.0", "type": "REST", "status": "PUBLISHED", "labels": labels,
		"endPoints": map[string]string{"productionURL": "https://backend.example.invalid/" + id},
	}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/apis", metadata, catalogueRESTDefinition(name)); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalAPI, ID: id, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", id)
		return fmt.Errorf("registering API %q for cleanup: %w", id, err)
	}
	return nil
}

// viewCards opens a view's APIs page and reports which of the ids have a card, waiting until the
// listing matches want.
func (u *Steps) viewCards(ctx context.Context, view string, want map[string]bool) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{}, func(ctx context.Context) (bool, error) {
		if err := u.navigate(ctx, base+"/api-portal/default/views/"+view+"/apis"); err != nil {
			return false, retry.Transient(err)
		}
		for id, listed := range want {
			count, err := page.Locator("#apiCard-" + id).Count()
			if err != nil {
				return false, retry.Transient(err)
			}
			if (count == 1) != listed {
				return false, nil
			}
		}
		return true, nil
	}, func(ok bool) bool { return ok }, fmt.Sprintf("waiting for view %q to list %v", view, want))
}

// scopedAPIs returns the label-scoped API and the default-label API.
func scopedAPIs(ctx context.Context) (string, string, error) {
	labelled, err := contextString(ctx, keyScopedLabelAPI)
	if err != nil {
		return "", "", err
	}
	other, err := contextString(ctx, keyScopedDefaultAPI)
	if err != nil {
		return "", "", err
	}
	return labelled, other, nil
}

// viewListsOnlyLabelledAPI checks the settings view's catalogue: the API with the view's label is
// listed, the one with only the default label is not, while the default view lists the latter.
func (u *Steps) viewListsOnlyLabelledAPI(ctx context.Context) error {
	view, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	labelled, other, err := scopedAPIs(ctx)
	if err != nil {
		return err
	}
	if err := u.viewCards(ctx, "default", map[string]bool{other: true, labelled: false}); err != nil {
		return err
	}
	return u.viewCards(ctx, view, map[string]bool{labelled: true, other: false})
}

// openViewModal opens the edit dialog of the settings view.
func (u *Steps) openViewModal(ctx context.Context, view string) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, "cfg-views")
	if err != nil {
		return nil, err
	}
	if err := page.Locator("#cfg-view-row-" + view + " .cfg-view-edit-btn").First().Click(); err != nil {
		return nil, fmt.Errorf("opening the view editor: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-view-modal")).ToBeVisible(); err != nil {
		return nil, err
	}
	return page, nil
}

// addDefaultLabelToView adds the default label to the settings view in its edit dialog.
func (u *Steps) addDefaultLabelToView(ctx context.Context) error {
	view, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	page, err := u.openViewModal(ctx, view)
	if err != nil {
		return err
	}
	toggle := page.Locator(`#view-labels .cfg-label-toggle[data-value="default"]`)
	if err := toggle.Click(); err != nil {
		return fmt.Errorf("adding the default label: %w", err)
	}
	if err := u.expect.Locator(toggle).ToHaveAttribute("aria-pressed", "true"); err != nil {
		return err
	}
	if err := page.Locator("#cfg-view-modal-save").Click(); err != nil {
		return fmt.Errorf("saving the view: %w", err)
	}
	chip := page.Locator("#cfg-view-row-" + view + " .cfg-name-chip").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile("^default$")})
	return u.expect.Locator(chip).ToHaveCount(1)
}

// viewListsBothAPIs checks that the settings view lists the APIs of both of its labels.
func (u *Steps) viewListsBothAPIs(ctx context.Context) error {
	view, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	labelled, other, err := scopedAPIs(ctx)
	if err != nil {
		return err
	}
	return u.viewCards(ctx, view, map[string]bool{labelled: true, other: true})
}

// oldViewAddressIsNotFound checks that the view's address before the rename is a 404 page, and
// the new one opens.
func (u *Steps) oldViewAddressIsNotFound(ctx context.Context) error {
	old, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	renamed, err := contextString(ctx, keyAPIPortalRenamedView)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	if err := u.navigate(ctx, base+"/api-portal/default/views/"+renamed+"/apis"); err != nil {
		return err
	}
	if status, err := navigationStatus(ctx); err != nil || status != http.StatusOK {
		return fmt.Errorf("the renamed view's address answered %d (%v), want 200", status, err)
	}
	if err := u.navigate(ctx, base+"/api-portal/default/views/"+old+"/apis"); err != nil {
		return err
	}
	status, err := navigationStatus(ctx)
	if err != nil {
		return err
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("the old view address answered %d, want 404", status)
	}
	return tcontext.Set(ctx, keyAPIPortalSettingsView, renamed)
}

// renameSettingsLabel changes the settings label's display name and checks its row.
func (u *Steps) renameSettingsLabel(ctx context.Context) error {
	label, err := contextString(ctx, keyAPIPortalSettingsLabel)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-labels")
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-label-row-" + label + " .cfg-label-edit-btn").First().Click(); err != nil {
		return fmt.Errorf("opening the label editor: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-label-modal")).ToBeVisible(); err != nil {
		return err
	}
	renamed := "IT Renamed Label " + label
	if err := page.Locator("#lbl-display").Fill(renamed); err != nil {
		return err
	}
	if err := page.Locator("#cfg-label-modal-save").Click(); err != nil {
		return fmt.Errorf("saving the label: %w", err)
	}
	row := page.Locator("#cfg-label-row-" + label)
	if err := u.expect.Locator(row).ToContainText(renamed); err != nil {
		return err
	}
	return u.expect.Locator(row).ToContainText(label)
}

// deleteUsedLabel deletes the settings label, which the labelled API carries.
func (u *Steps) deleteUsedLabel(ctx context.Context) error {
	label, err := contextString(ctx, keyAPIPortalSettingsLabel)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-labels")
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-label-row-" + label + " .cfg-label-delete-btn").Click(); err != nil {
		return fmt.Errorf("deleting the label: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-delete-label-modal")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-label-confirm").Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-label-row-" + label)).ToHaveCount(0); err != nil {
		return err
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalLabel, label)
	return nil
}

// apiDropsOutOfView checks that the API lost the deleted label and so left the view, while the
// API with the default label stays.
func (u *Steps) apiDropsOutOfView(ctx context.Context) error {
	view, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	label, err := contextString(ctx, keyAPIPortalSettingsLabel)
	if err != nil {
		return err
	}
	labelled, other, err := scopedAPIs(ctx)
	if err != nil {
		return err
	}
	if err := u.viewCards(ctx, view, map[string]bool{labelled: false, other: true}); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	status, body, err := u.portalCall(page, http.MethodGet, "/apis/"+labelled, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /apis/%s = %d, want 200", labelled, status)
	}
	if strings.Contains(body, `"`+label+`"`) {
		return fmt.Errorf("the API still carries the deleted label %q", label)
	}
	return nil
}

// deleteSettingsView deletes the settings view from its row.
func (u *Steps) deleteSettingsView(ctx context.Context) error {
	view, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-views")
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-view-row-" + view + " .cfg-view-delete-btn").Click(); err != nil {
		return fmt.Errorf("deleting the view: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-delete-view-modal")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-view-confirm").Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-view-row-" + view)).ToHaveCount(0); err != nil {
		return err
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalView, view)
	return nil
}

// deletedViewIsNotFound checks that the deleted view's address is a 404 while the default view
// still opens.
func (u *Steps) deletedViewIsNotFound(ctx context.Context) error {
	view, err := contextString(ctx, keyAPIPortalSettingsView)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	for target, want := range map[string]int{"default": http.StatusOK, view: http.StatusNotFound} {
		if err := u.navigate(ctx, base+"/api-portal/default/views/"+target+"/apis"); err != nil {
			return err
		}
		status, err := navigationStatus(ctx)
		if err != nil {
			return err
		}
		if status != want {
			return fmt.Errorf("view %q answered %d, want %d", target, status, want)
		}
	}
	return nil
}

// seedThemeViews creates two views with the default label for the theming journey.
func (u *Steps) seedThemeViews(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, key := range []string{keyThemedView, keyUnthemedView} {
		view, err := unique.Unique(ctx, "theme-view")
		if err != nil {
			return err
		}
		if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalView, ID: view, Actor: u.topo.Admin.Username}); err != nil {
			return fmt.Errorf("registering view %q for cleanup: %w", view, err)
		}
		status, body, err := u.portalCall(page, http.MethodPost, "/views", map[string]any{
			"id": view, "displayName": "IT Theme View " + view, "labels": []string{"default"},
		})
		if err != nil {
			return err
		}
		if status != http.StatusCreated {
			return fmt.Errorf("creating view %q: status %d %s", view, status, body)
		}
		if err := tcontext.Set(ctx, key, view); err != nil {
			return err
		}
	}
	return nil
}

// openViewPanel opens a view-scoped settings panel and chooses the view in its selector.
func (u *Steps) openViewPanel(ctx context.Context, panel, view, display string) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, panel)
	if err != nil {
		return nil, err
	}
	combo := page.Locator("#" + panel + " .cfg-view-combo")
	if err := combo.Locator(".cfg-view-combo-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the view selector: %w", err)
	}
	if err := combo.Locator(".cfg-view-combo-input").Fill(view); err != nil {
		return nil, fmt.Errorf("filtering the views: %w", err)
	}
	if err := combo.Locator(`.cfg-view-combo-option[data-value="` + view + `"]`).Click(); err != nil {
		return nil, fmt.Errorf("choosing the view: %w", err)
	}
	// Choosing a view reloads the settings page for it, which takes longer than an assertion's
	// default wait.
	if err := page.WaitForURL(regexp.MustCompile(`[?&]view=`+regexp.QuoteMeta(view)+`(#|&|$)`), playwright.PageWaitForURLOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return nil, fmt.Errorf("waiting for the settings of view %q: %w", view, err)
	}
	if err := u.expect.Locator(page.Locator("#" + panel + " .cfg-view-combo-current")).ToHaveText(display); err != nil {
		return nil, err
	}
	return page, nil
}

// openThemingPanel opens the Theming panel for the first view.
func (u *Steps) openThemingPanel(ctx context.Context) (playwright.Page, string, error) {
	view, err := contextString(ctx, keyThemedView)
	if err != nil {
		return nil, "", err
	}
	page, err := u.openViewPanel(ctx, "cfg-theming", view, "IT Theme View "+view)
	return page, view, err
}

// themeArchive builds a theme ZIP: a stylesheet that sets the body colour and an image, or, for
// a script-only theme, nothing but a script.
func themeArchive(scriptOnly bool) (playwright.InputFile, error) {
	entries := map[string][]byte{"theme/scripts/app.js": []byte("window.themeScript = true;")}
	if !scriptOnly {
		image, err := hex.DecodeString(wizardPNGHex)
		if err != nil {
			return playwright.InputFile{}, err
		}
		entries = map[string][]byte{
			"theme/styles/main.css":       []byte("body { color: " + themeColour + "; }"),
			"theme/images/brand-mark.png": image,
		}
	}
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for name, content := range entries {
		entry, err := archive.Create(name)
		if err != nil {
			return playwright.InputFile{}, err
		}
		if _, err := entry.Write(content); err != nil {
			return playwright.InputFile{}, err
		}
	}
	if err := archive.Close(); err != nil {
		return playwright.InputFile{}, err
	}
	return wizardFile("theme.zip", "application/zip", buf.Bytes()), nil
}

// applyTheme uploads the theme ZIP for the first view, refusing a file that is not a ZIP first.
func (u *Steps) applyTheme(ctx context.Context) error {
	page, _, err := u.openThemingPanel(ctx)
	if err != nil {
		return err
	}
	theme, err := themeArchive(false)
	if err != nil {
		return err
	}
	return wizardAll(
		u.wizardContains(page, "#themingStatusBadge", "Default"),
		func() error { return u.expect.Locator(page.Locator("#theming-apply-btn")).ToBeDisabled() },
		wizardChoose(page, "#theming-zip-input", wizardNotes()),
		u.wizardHasText(page, "#theming-zip-error", "Unsupported file type. Accepted: .zip"),
		func() error { return u.expect.Locator(page.Locator("#theming-apply-btn")).ToBeDisabled() },
		wizardChoose(page, "#theming-zip-input", theme),
		u.wizardHasText(page, "#theming-zip-name", "theme.zip"),
		wizardClick(page, "#theming-apply-btn"),
		func() error { return u.expectToast(page, "Theme applied successfully") },
		u.wizardContains(page, "#themingStatusBadge", "Custom"),
	)
}

// bodyColour opens a view's home page and returns the computed body colour.
func (u *Steps) bodyColour(ctx context.Context, view string) (string, error) {
	base, err := u.apiPortalURL()
	if err != nil {
		return "", err
	}
	if err := u.navigate(ctx, base+"/api-portal/default/views/"+view); err != nil {
		return "", err
	}
	page, err := u.page(ctx)
	if err != nil {
		return "", err
	}
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateLoad}); err != nil {
		return "", err
	}
	colour, err := page.Evaluate(`() => getComputedStyle(document.body).color`)
	if err != nil {
		return "", fmt.Errorf("reading the body colour of view %q: %w", view, err)
	}
	text, _ := colour.(string)
	return text, nil
}

// themeShowsOnFirstViewOnly checks the themed body colour on the first view and its absence on
// the second.
func (u *Steps) themeShowsOnFirstViewOnly(ctx context.Context) error {
	themed, err := contextString(ctx, keyThemedView)
	if err != nil {
		return err
	}
	other, err := contextString(ctx, keyUnthemedView)
	if err != nil {
		return err
	}
	if err := u.waitForColour(ctx, themed, true); err != nil {
		return err
	}
	return u.waitForColour(ctx, other, false)
}

// waitForColour waits until a view's body colour is, or is not, the theme's colour.
func (u *Steps) waitForColour(ctx context.Context, view string, themed bool) error {
	return retry.Await(ctx, retry.Options{}, func(ctx context.Context) (bool, error) {
		colour, err := u.bodyColour(ctx, view)
		if err != nil {
			return false, retry.Transient(err)
		}
		return (colour == themeColourRGB) == themed, nil
	}, func(ok bool) bool { return ok }, fmt.Sprintf("waiting for view %q to be themed=%t", view, themed))
}

// downloadedThemeMatches downloads the first view's theme and checks that the archive holds the
// stylesheet that was applied.
func (u *Steps) downloadedThemeMatches(ctx context.Context) error {
	page, view, err := u.openThemingPanel(ctx)
	if err != nil {
		return err
	}
	download, err := page.ExpectDownload(func() error { return page.Locator("#theming-download-btn").Click() })
	if err != nil {
		return fmt.Errorf("downloading the theme: %w", err)
	}
	if name := download.SuggestedFilename(); name != "theme-"+view+".zip" {
		return fmt.Errorf("the theme downloaded as %q, want theme-%s.zip", name, view)
	}
	dir, err := os.MkdirTemp("", "apiportal-theme-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, "theme.zip")
	if err := download.SaveAs(path); err != nil {
		return fmt.Errorf("saving the downloaded theme: %w", err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("opening the downloaded theme: %w", err)
	}
	defer func() { _ = archive.Close() }()
	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, "styles/main.css") {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		css, err := io.ReadAll(io.LimitReader(reader, 1<<20))
		_ = reader.Close()
		if err != nil {
			return err
		}
		if strings.Contains(string(css), themeColour) {
			return nil
		}
		return fmt.Errorf("the downloaded stylesheet does not hold the applied colour")
	}
	return fmt.Errorf("the downloaded theme holds no styles/main.css")
}

// scriptThemeRefused uploads a theme that holds only a script and checks that it is refused and
// the first view keeps its theme.
func (u *Steps) scriptThemeRefused(ctx context.Context) error {
	page, view, err := u.openThemingPanel(ctx)
	if err != nil {
		return err
	}
	theme, err := themeArchive(true)
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardChoose(page, "#theming-zip-input", theme),
		wizardClick(page, "#theming-apply-btn"),
		func() error {
			return u.expect.Locator(page.Locator("#alertToast")).ToHaveClass(regexpWord("error"))
		},
		u.wizardContains(page, "#themingStatusBadge", "Custom"),
	); err != nil {
		return err
	}
	return u.waitForColour(ctx, view, true)
}

// resetTheme resets the first view's theme through the confirmation dialog.
func (u *Steps) resetTheme(ctx context.Context) error {
	page, view, err := u.openThemingPanel(ctx)
	if err != nil {
		return err
	}
	return wizardAll(
		wizardClick(page, "#theming-reset-btn"),
		u.wizardHasText(page, "#cfg-reset-theme-view-txt", view),
		wizardClick(page, "#cfg-reset-theme-confirm"),
		func() error { return u.expectToast(page, "Theme reset to default") },
		u.wizardContains(page, "#themingStatusBadge", "Default"),
	)
}

// firstViewIsDefault checks that the first view no longer has the theme's colour.
func (u *Steps) firstViewIsDefault(ctx context.Context) error {
	view, err := contextString(ctx, keyThemedView)
	if err != nil {
		return err
	}
	return u.waitForColour(ctx, view, false)
}
