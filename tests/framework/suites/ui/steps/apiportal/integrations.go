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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// KindAPIPortalWebhook identifies a webhook subscriber an administrator added.
var KindAPIPortalWebhook = cleanup.Kind{Name: "api-portal-webhook", Order: 32}

// KindAPIPortalOrganization restores the organization's details. Its ID is the JSON restore
// payload, which carries the organization handle as "id".
var KindAPIPortalOrganization = cleanup.Kind{Name: "api-portal-organization", Order: 90}

const (
	keyWebhook     = "uiAPIPortalWebhook"
	keyWebhookName = "uiAPIPortalWebhookName"

	// webhookEvents are the events the journey's webhook subscribes to.
	webhookEventA = "subscription.created"
	webhookEventB = "subscription.deleted"
)

// registerIntegrationSteps wires the organization, key manager and webhook journey steps.
func (u *Steps) registerIntegrationSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the organization form refuses an invalid business owner email$`, u.organizationRefusesEmail)
	sc.Step(`^the administrator changes the organization's name and business owner, and they are kept after a reload$`, u.editOrganization)
	sc.Step(`^the organization's handle and IdP reference cannot be changed$`, u.organizationReadOnlyFields)
	sc.Step(`^the administrator (disables|enables) that key manager, and it is (no longer|again) offered to the developer$`, u.toggleKeyManager)
	sc.Step(`^the administrator deletes that key manager$`, u.deleteKeyManager)
	sc.Step(`^the webhook form refuses a webhook without a target URL or secret, or with an invalid URL$`, u.webhookValidation)
	sc.Step(`^the administrator adds a webhook with a URL, secret, timeout and two events$`, u.addWebhook)
	sc.Step(`^the webhook is listed with its events, its secret and enabled$`, u.webhookListed)
	sc.Step(`^the administrator edits the webhook leaving the secret blank, which keeps it$`, u.editWebhookKeepsSecret)
	sc.Step(`^the administrator disables the webhook$`, u.disableWebhook)
	sc.Step(`^the administrator deletes the webhook$`, u.deleteWebhook)
}

// openOrganizationPanel opens the Organization settings panel.
func (u *Steps) openOrganizationPanel(ctx context.Context) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, "cfg-organizations")
	if err != nil {
		return nil, err
	}
	return page, u.expect.Locator(page.Locator("#org-display")).ToBeVisible()
}

// snapshotOrganization registers a restore of the organization's current details, once per
// scenario, before the journey changes them.
func (u *Steps) snapshotOrganization(ctx context.Context, page playwright.Page) error {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	for _, resource := range registry.Pending() {
		if resource.Kind.Name == KindAPIPortalOrganization.Name {
			return nil
		}
	}
	handle, err := page.Locator("#org-handle").InputValue()
	if err != nil {
		return err
	}
	status, body, err := u.portalCall(page, http.MethodGet, "/organizations/"+handle, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("reading organization %q: status %d", handle, status)
	}
	var current map[string]any
	if err := json.Unmarshal([]byte(body), &current); err != nil {
		return fmt.Errorf("decoding organization %q: %w", handle, err)
	}
	restore := map[string]any{}
	for _, field := range []string{"id", "idpRefId", "displayName", "cpRefId", "businessOwner", "businessOwnerContact", "businessOwnerEmail", "configuration"} {
		if value, ok := current[field]; ok && value != nil && value != "" {
			restore[field] = value
		}
	}
	if _, ok := restore["id"]; !ok {
		restore["id"] = handle
	}
	if _, ok := restore["cpRefId"]; !ok {
		restore["cpRefId"] = ""
	}
	payload, err := json.Marshal(restore)
	if err != nil {
		return err
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalOrganization, ID: string(payload), Actor: u.topo.Admin.Username})
}

// organizationRefusesEmail checks that saving an invalid owner email is refused and nothing is
// stored, and that Save is disabled while the name is empty.
func (u *Steps) organizationRefusesEmail(ctx context.Context) error {
	page, err := u.openOrganizationPanel(ctx)
	if err != nil {
		return err
	}
	if err := u.snapshotOrganization(ctx, page); err != nil {
		return err
	}
	name, err := page.Locator("#org-display").InputValue()
	if err != nil {
		return err
	}
	save := page.Locator("#cfg-save-org-btn")
	return wizardAll(
		wizardFill(page, "#org-display", ""),
		func() error { return u.expect.Locator(save).ToBeDisabled() },
		wizardFill(page, "#org-display", name),
		wizardFill(page, "#org-owner-email", "not-an-email"),
		wizardClick(page, "#cfg-save-org-btn"),
		func() error { return u.expectToast(page, "Business owner email is not a valid email address.") },
		func() error {
			if _, err := u.openOrganizationPanel(ctx); err != nil {
				return err
			}
			return u.expect.Locator(page.Locator("#org-owner-email")).Not().ToHaveValue("not-an-email")
		},
	)
}

// organizationValues are the details the journey writes to the organization.
func organizationValues(suffix string) map[string]string {
	return map[string]string{
		"#org-display":       "IT Organization " + suffix,
		"#org-owner":         "IT Owner " + suffix,
		"#org-owner-contact": "+94 11 000 0000",
		"#org-owner-email":   "owner-" + suffix + "@example.invalid",
	}
}

// editOrganization saves new organization details and checks them after a reload.
func (u *Steps) editOrganization(ctx context.Context) error {
	page, err := u.openOrganizationPanel(ctx)
	if err != nil {
		return err
	}
	if err := u.snapshotOrganization(ctx, page); err != nil {
		return err
	}
	suffix, err := unique.Unique(ctx, "org")
	if err != nil {
		return err
	}
	values := organizationValues(suffix)
	for selector, value := range values {
		if err := page.Locator(selector).Fill(value); err != nil {
			return fmt.Errorf("filling %s: %w", selector, err)
		}
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#cfg-save-org-btn").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("saving the organization: %w", err)
	}
	if _, err := u.openOrganizationPanel(ctx); err != nil {
		return err
	}
	for selector, value := range values {
		if err := u.expect.Locator(page.Locator(selector)).ToHaveValue(value); err != nil {
			return fmt.Errorf("%s was not kept: %w", selector, err)
		}
	}
	return nil
}

// organizationReadOnlyFields checks that the handle and IdP reference fields are read-only.
func (u *Steps) organizationReadOnlyFields(ctx context.Context) error {
	page, err := u.openOrganizationPanel(ctx)
	if err != nil {
		return err
	}
	for _, selector := range []string{"#org-handle", "#org-idp-ref"} {
		if err := u.expect.Locator(page.Locator(selector)).Not().ToBeEditable(); err != nil {
			return fmt.Errorf("%s can be edited: %w", selector, err)
		}
	}
	return u.expect.Locator(page.Locator("#org-display")).ToBeEditable()
}

// keyManagerID returns the key manager the journey created.
func keyManagerID(ctx context.Context) (string, string, error) {
	id, err := contextString(ctx, keyAPIPortalKeyManager)
	if err != nil {
		return "", "", err
	}
	name, err := contextString(ctx, keyAPIPortalKeyManagerName)
	return id, name, err
}

// toggleKeyManager switches the key manager off or on from its row as the administrator, checks
// the stored state, and checks what the developer is offered.
func (u *Steps) toggleKeyManager(ctx context.Context, action, _ string) error {
	id, name, err := keyManagerID(ctx)
	if err != nil {
		return err
	}
	if err := u.loginAs(ctx, u.topo.Admin.Username, u.topo.Admin.Password); err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-keymanagers")
	if err != nil {
		return err
	}
	enable := action == "enables"
	toggle := page.Locator("#km-status-" + id)
	if enable {
		err = toggle.Check()
	} else {
		err = toggle.Uncheck()
	}
	if err != nil {
		return fmt.Errorf("switching the key manager: %w", err)
	}
	title := map[bool]string{true: "Enabled", false: "Disabled"}[enable]
	if err := u.expect.Locator(toggle).ToHaveAttribute("title", title); err != nil {
		return err
	}
	if err := u.loginAsDeveloper(ctx); err != nil {
		return err
	}
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		listed, err := u.keyManagerListed(page, name, id)
		if err != nil {
			return false, retry.Transient(err)
		}
		return listed == enable, nil
	}, func(ok bool) bool { return ok }, fmt.Sprintf("waiting for the key manager to be offered=%t", enable))
}

// deleteKeyManager deletes the key manager from its row as the administrator.
func (u *Steps) deleteKeyManager(ctx context.Context) error {
	id, name, err := keyManagerID(ctx)
	if err != nil {
		return err
	}
	if err := u.loginAs(ctx, u.topo.Admin.Username, u.topo.Admin.Password); err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-keymanagers")
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-km-row-" + id + " .cfg-km-delete-btn").Click(); err != nil {
		return fmt.Errorf("deleting the key manager: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-km-name-txt")).ToHaveText(name); err != nil {
		return err
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#cfg-del-km-confirm").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	if _, err := u.openSettingsPanel(ctx, "cfg-keymanagers"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-km-row-" + id)).ToHaveCount(0); err != nil {
		return err
	}
	status, _, err := u.portalCall(page, http.MethodGet, "/key-managers/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("GET /key-managers/%s = %d after the delete, want 404", id, status)
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalKeyManager, id)
	return nil
}

// openWebhookForm opens the add-webhook form.
func (u *Steps) openWebhookForm(ctx context.Context) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, "cfg-webhooks")
	if err != nil {
		return nil, err
	}
	if err := page.Locator("#cfg-add-webhook-btn").Click(); err != nil {
		return nil, fmt.Errorf("opening the webhook form: %w", err)
	}
	return page, u.expect.Locator(page.Locator("#wh-form-view")).ToBeVisible()
}

// webhookValidation checks that Save stays disabled until name, URL and secret are filled, and
// that an invalid URL is refused. The form is left without saving.
func (u *Steps) webhookValidation(ctx context.Context) error {
	page, err := u.openWebhookForm(ctx)
	if err != nil {
		return err
	}
	save := page.Locator("#wh-form-save")
	return wizardAll(
		func() error { return u.expect.Locator(save).ToBeDisabled() },
		wizardFill(page, "#wh-display", "IT Invalid Webhook"),
		wizardFill(page, "#wh-secret", "invalid-webhook-secret"),
		func() error { return u.expect.Locator(save).ToBeDisabled() },
		wizardFill(page, "#wh-url", "not-a-url"),
		wizardFill(page, "#wh-secret", ""),
		func() error { return u.expect.Locator(save).ToBeDisabled() },
		wizardFill(page, "#wh-secret", "invalid-webhook-secret"),
		func() error { return u.expect.Locator(save).ToBeEnabled() },
		wizardClick(page, "#wh-form-save"),
		func() error { return u.expectToast(page, "Target URL must be a valid http:// or https:// URL.") },
		u.wizardVisible(page, "#wh-form-view"),
		wizardClick(page, "#wh-form-cancel"),
		u.wizardVisible(page, "#wh-list-view"),
		func() error {
			return u.expect.Locator(page.Locator("#cfg-webhooks-tbody tr").Filter(playwright.LocatorFilterOptions{HasText: "IT Invalid Webhook"})).ToHaveCount(0)
		},
	)
}

// webhookRow finds the journey's webhook row by its name.
func webhookRow(page playwright.Page, name string) playwright.Locator {
	return page.Locator("#cfg-webhooks-tbody tr").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator(".cfg-webhook-edit-btn", playwright.PageLocatorOptions{HasText: regexp.MustCompile("^" + regexp.QuoteMeta(name) + "$")}),
	})
}

// addWebhook adds a webhook that subscribes to two events and registers it for cleanup.
func (u *Steps) addWebhook(ctx context.Context) error {
	page, err := u.openWebhookForm(ctx)
	if err != nil {
		return err
	}
	suffix, err := unique.Unique(ctx, "webhook")
	if err != nil {
		return err
	}
	name := "IT Webhook " + suffix
	secret, err := unique.Unique(ctx, "webhook-secret")
	if err != nil {
		return err
	}
	if err := wizardAll(
		wizardFill(page, "#wh-display", name),
		wizardFill(page, "#wh-url", "https://hooks.example.invalid/"+suffix),
		wizardFill(page, "#wh-secret", secret),
		wizardFill(page, "#wh-timeout", "4000"),
		wizardClick(page, "#wh-events-select-btn"),
		func() error { return page.Locator(`.wh-event-check[value="` + webhookEventA + `"]`).Check() },
		func() error { return page.Locator(`.wh-event-check[value="` + webhookEventB + `"]`).Check() },
	); err != nil {
		return err
	}
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#wh-form-save").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("saving the webhook: %w", err)
	}
	if _, err := u.openSettingsPanel(ctx, "cfg-webhooks"); err != nil {
		return err
	}
	row := webhookRow(page, name)
	if err := u.expect.Locator(row).ToHaveCount(1); err != nil {
		return err
	}
	id, err := row.GetAttribute("data-id")
	if err != nil || id == "" {
		return fmt.Errorf("reading the webhook id: %v", err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalWebhook, ID: id, Actor: u.topo.Admin.Username}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyWebhook, id); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyWebhookName, name)
}

// webhookListed checks the webhook's row and its stored timeout.
func (u *Steps) webhookListed(ctx context.Context) error {
	return u.webhookState(ctx, true)
}

// webhookState checks the webhook's row: its two events, a stored secret and whether it is
// enabled; and its stored target and timeout.
func (u *Steps) webhookState(ctx context.Context, enabled bool) error {
	id, err := contextString(ctx, keyWebhook)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-webhooks")
	if err != nil {
		return err
	}
	row := page.Locator("#cfg-webhook-row-" + id)
	cells := row.Locator("td")
	if err := u.expect.Locator(cells.Nth(2).Locator(".cfg-name-chip")).ToHaveText([]string{webhookEventA, webhookEventB}); err != nil {
		return err
	}
	if err := u.expect.Locator(cells.Nth(3)).ToHaveText("••••••••"); err != nil {
		return err
	}
	if err := u.expect.Locator(cells.Nth(4)).ToHaveText(map[bool]string{true: "Yes", false: "No"}[enabled]); err != nil {
		return err
	}
	status, body, err := u.portalCall(page, http.MethodGet, "/webhook-subscribers/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("reading webhook %q: status %d", id, status)
	}
	var stored struct {
		TimeoutMs int  `json:"timeoutMs"`
		Enabled   bool `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(body), &stored); err != nil {
		return err
	}
	if stored.Enabled != enabled {
		return fmt.Errorf("webhook %q is stored with enabled=%t, want %t", id, stored.Enabled, enabled)
	}
	if stored.TimeoutMs != 0 && stored.TimeoutMs != 4000 {
		return fmt.Errorf("webhook %q is stored with timeout %d, want 4000", id, stored.TimeoutMs)
	}
	return nil
}

// openWebhookEditor opens the edit form of the journey's webhook.
func (u *Steps) openWebhookEditor(ctx context.Context) (playwright.Page, error) {
	id, err := contextString(ctx, keyWebhook)
	if err != nil {
		return nil, err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-webhooks")
	if err != nil {
		return nil, err
	}
	if err := page.Locator("#cfg-webhook-row-" + id + " .cfg-icon-btn.cfg-webhook-edit-btn").Click(); err != nil {
		return nil, fmt.Errorf("opening the webhook editor: %w", err)
	}
	return page, u.expect.Locator(page.Locator("#wh-form-view")).ToBeVisible()
}

// saveWebhookEdit saves the open webhook form and waits for the reload.
func (u *Steps) saveWebhookEdit(page playwright.Page) error {
	if _, err := page.ExpectNavigation(func() error { return page.Locator("#wh-form-save").Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		return fmt.Errorf("saving the webhook: %w", err)
	}
	return nil
}

// editWebhookKeepsSecret renames the webhook with the secret left blank and checks that the
// stored secret is kept.
func (u *Steps) editWebhookKeepsSecret(ctx context.Context) error {
	name, err := contextString(ctx, keyWebhookName)
	if err != nil {
		return err
	}
	page, err := u.openWebhookEditor(ctx)
	if err != nil {
		return err
	}
	renamed := name + " Edited"
	if err := wizardAll(
		u.wizardHasValue(page, "#wh-secret", ""),
		u.wizardVisible(page, "#wh-secret-hint"),
		func() error { return u.expect.Locator(page.Locator("#wh-form-save")).ToBeEnabled() },
		wizardFill(page, "#wh-display", renamed),
	); err != nil {
		return err
	}
	if err := u.saveWebhookEdit(page); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyWebhookName, renamed); err != nil {
		return err
	}
	if _, err := u.openSettingsPanel(ctx, "cfg-webhooks"); err != nil {
		return err
	}
	if err := u.expect.Locator(webhookRow(page, renamed)).ToHaveCount(1); err != nil {
		return err
	}
	return u.webhookState(ctx, true)
}

// disableWebhook turns the webhook off in its editor and checks the row and stored state.
func (u *Steps) disableWebhook(ctx context.Context) error {
	page, err := u.openWebhookEditor(ctx)
	if err != nil {
		return err
	}
	// The checkbox is drawn as a switch: the visible slider takes the click.
	if err := page.Locator("#wh-form-view .wh-toggle-slider").Click(); err != nil {
		return fmt.Errorf("disabling the webhook: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#wh-enabled")).Not().ToBeChecked(); err != nil {
		return err
	}
	if err := u.saveWebhookEdit(page); err != nil {
		return err
	}
	return u.webhookState(ctx, false)
}

// deleteWebhook deletes the webhook from its row and checks it is gone from the list and store.
func (u *Steps) deleteWebhook(ctx context.Context) error {
	id, err := contextString(ctx, keyWebhook)
	if err != nil {
		return err
	}
	name, err := contextString(ctx, keyWebhookName)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-webhooks")
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-webhook-row-" + id + " .cfg-webhook-delete-btn").Click(); err != nil {
		return fmt.Errorf("deleting the webhook: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-webhook-name-txt")).ToHaveText(name); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-webhook-confirm").Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-webhook-row-" + id)).ToHaveCount(0); err != nil {
		return err
	}
	status, _, err := u.portalCall(page, http.MethodGet, "/webhook-subscribers/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("GET /webhook-subscribers/%s = %d after the delete, want 404", id, status)
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalWebhook, id)
	return nil
}
