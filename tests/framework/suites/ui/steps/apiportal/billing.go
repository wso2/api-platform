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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// KindAPIPortalBilling identifies the billing configuration of the API Portal test plane. It is
// removed after the paid subscriptions, before the plans and meters it sells.
var KindAPIPortalBilling = cleanup.Kind{Name: "api-portal-billing", Order: 25}

// KindAPIPortalPaidSubscription identifies a developer's paid subscription, ended at once by the
// developer who bought it.
var KindAPIPortalPaidSubscription = cleanup.Kind{Name: "api-portal-paid-subscription", Order: 15}

// KindAPIPortalSubscriptionPlan identifies a subscription plan, removed after the APIs that offer it.
var KindAPIPortalSubscriptionPlan = cleanup.Kind{Name: "api-portal-subscription-plan", Order: 55}

// KindAPIPortalMeter identifies a meter, removed after the plans that price with it.
var KindAPIPortalMeter = cleanup.Kind{Name: "api-portal-meter", Order: 60}

// The keys billing is set up with: shaped as Stripe's test keys and Moesif's keys are, and accepted
// by the testbench's Stripe and Moesif services.
const (
	billingStripeSecretKey      = "sk_test_apiportalui"
	billingStripePublishableKey = "pk_test_apiportalui"
)

// toastHidden matches the toast once it has gone, so a later toast is not mistaken for it.
var toastHidden = regexp.MustCompile(`alert-toast-hidden`)

var billingMoesifApplication = base64.RawURLEncoding.EncodeToString([]byte(`{"app":"524:75"}`))

func (u *Steps) registerBillingSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the administrator sets up billing in the settings, correcting the keys the form and Stripe refuse$`,
		u.setUpBillingInSettings)
	sc.Step(`^the administrator moves the billing webhook host and removes billing in the settings$`,
		u.moveWebhookHostAndRemoveBilling)
	sc.Step(`^the administrator adds a Tokens meter with a suggested and a custom property in the settings$`,
		u.addMeterInSettings)
	sc.Step(`^the administrator edits that meter, and deletes it$`, u.editAndDeleteMeter)
	sc.Step(`^billing is set up in the API Portal$`, u.setUpBillingByAPI)
	sc.Step(`^the API Portal has a meter of requests by status and method$`, u.createRequestMeterByAPI)
	sc.Step(`^the administrator is sent to set up billing when pricing a plan without it$`, u.paidPlanNeedsBilling)
	sc.Step(`^the administrator prices a flat rate plan in the settings, correcting its limits and amount$`,
		u.priceFlatRatePlanInSettings)
	sc.Step(`^the administrator edits that plan, and deletes it$`, u.editAndDeletePlan)
	sc.Step(`^the administrator prices a usage based plan in the settings, correcting each price$`,
		u.priceUsagePlanInSettings)
}

// testbenchCall sends a request to one of the block's testbench services, such as the Stripe
// service's /test endpoints.
func (u *Steps) testbenchCall(ctx context.Context, service, method, path string, body any) (*httpx.Response, error) {
	base, err := u.topo.URL("testbench", service)
	if err != nil {
		return nil, err
	}
	if u.topo.Block == nil {
		return nil, fmt.Errorf("the %s service is partitioned by block, but the topology has no block", service)
	}
	request := httpx.Request{Method: method, URL: base + "/" + u.topo.Block.PartitionKey() + path}
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		request.Body, request.ContentType = payload, "application/json"
	}
	response, err := u.client.Do(ctx, request, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("%s %s on the %s service: %w", method, path, service, err)
	}
	return response, nil
}

// resetBillingProviders empties the block's Stripe and Moesif accounts.
func (u *Steps) resetBillingProviders(ctx context.Context) error {
	for _, service := range []string{"stripe", "moesif"} {
		response, err := u.testbenchCall(ctx, service, http.MethodPost, "/test/reset", nil)
		if err != nil {
			return err
		}
		if !response.Succeeded() {
			return fmt.Errorf("resetting the %s service: %s", service, response.Describe())
		}
	}
	return nil
}

// failProviderCall makes the next matching call to a billing provider fail with status.
func (u *Steps) failProviderCall(ctx context.Context, service string, failure map[string]any) error {
	response, err := u.testbenchCall(ctx, service, http.MethodPost, "/test/fail", failure)
	if err != nil {
		return err
	}
	if !response.Succeeded() {
		return fmt.Errorf("setting a %s failure: %s", service, response.Describe())
	}
	return nil
}

// portalCall sends a REST request from the page, with its session and CSRF token, and returns
// the status and body.
func (u *Steps) portalCall(page playwright.Page, method, path string, body any) (int, string, error) {
	payload := ""
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, "", err
		}
		payload = string(raw)
	}
	result, err := page.Evaluate(`async ({method, path, payload}) => {
        const token = document.cookie.split('; ').find(v => v.startsWith('XSRF-TOKEN='));
        const headers = { organization: 'default', 'X-CSRF-Token': token ? decodeURIComponent(token.split('=').slice(1).join('=')) : '' };
        if (payload) headers['Content-Type'] = 'application/json';
        const response = await fetch('/api-portal/api/v0.9' + path, { method, headers, body: payload || undefined });
        return { status: response.status, body: await response.text() };
    }`, map[string]any{"method": method, "path": path, "payload": payload})
	if err != nil {
		return 0, "", fmt.Errorf("%s %s from the page: %w", method, path, err)
	}
	fields, _ := result.(map[string]any)
	status, _ := fields["status"].(int)
	if number, ok := fields["status"].(float64); ok {
		status = int(number)
	}
	text, _ := fields["body"].(string)
	return status, text, nil
}

// expectToast waits for the portal's toast to say text.
func (u *Steps) expectToast(page playwright.Page, text string) error {
	return u.expect.Locator(page.Locator("#alertToast .alert-toast-message")).ToContainText(text)
}

// openSettingsPanel opens a panel of the settings page.
func (u *Steps) openSettingsPanel(ctx context.Context, panel string) (playwright.Page, error) {
	page, err := u.openSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="` + panel + `"]`).Click(); err != nil {
		return nil, fmt.Errorf("opening the %s settings: %w", panel, err)
	}
	if err := u.expect.Locator(page.Locator("#" + panel)).ToBeVisible(); err != nil {
		return nil, err
	}
	return page, nil
}

// fillAndCheck fills a field, failing with the field's name.
func fillAndCheck(page playwright.Page, selector, value string) error {
	if err := page.Locator(selector).Fill(value); err != nil {
		return fmt.Errorf("filling %s: %w", selector, err)
	}
	return nil
}

// registerBilling removes the billing configuration when the scenario ends.
func (u *Steps) registerBilling(ctx context.Context) error {
	return cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalBilling, ID: "default", Actor: u.topo.Admin.Username})
}

// setUpBillingInSettings sets up billing in Settings › Billing as the administrator: the form
// refuses keys of the wrong kind, Stripe refuses a key once, and the corrected keys set billing
// up in test mode.
func (u *Steps) setUpBillingInSettings(ctx context.Context) error {
	if err := u.resetBillingProviders(ctx); err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-billing")
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-billing-setup-btn").Click(); err != nil {
		return fmt.Errorf("opening the billing setup form: %w", err)
	}
	modal := page.Locator("#cfg-billing-modal")
	if err := u.expect.Locator(modal).ToBeVisible(); err != nil {
		return err
	}
	next := page.Locator("#bil-form-next")
	if err := u.expect.Locator(next).ToBeDisabled(); err != nil {
		return fmt.Errorf("Next should wait for the Moesif keys: %w", err)
	}
	if err := fillAndCheck(page, "#bil-moesif-app", billingMoesifApplication+".collector"); err != nil {
		return err
	}
	if err := fillAndCheck(page, "#bil-moesif-key", "management."+billingMoesifApplication+".signature"); err != nil {
		return err
	}
	if err := page.Locator(`[data-reveal="bil-moesif-key"]`).Click(); err != nil {
		return fmt.Errorf("showing the Moesif key: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#bil-moesif-key")).ToHaveAttribute("type", "text"); err != nil {
		return err
	}
	if err := next.Click(); err != nil {
		return fmt.Errorf("moving to the Stripe keys: %w", err)
	}
	// Back to the first step and forward again keeps what was entered.
	if err := page.Locator("#bil-form-back").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#bil-moesif-app")).ToHaveValue(billingMoesifApplication + ".collector"); err != nil {
		return err
	}
	if err := next.Click(); err != nil {
		return err
	}

	save := page.Locator("#bil-form-save")
	refusals := []struct{ field, secret, publishable, message string }{
		{"stripeApiKey", "pk_test_notsecret", billingStripePublishableKey, "Enter a restricted key (rk_…) or secret key (sk_…)"},
		{"stripePublishableKey", billingStripeSecretKey, "sk_test_notpublishable", "Enter a publishable key"},
	}
	for _, refusal := range refusals {
		if err := fillAndCheck(page, "#bil-stripe-key", refusal.secret); err != nil {
			return err
		}
		if err := fillAndCheck(page, "#bil-stripe-pk", refusal.publishable); err != nil {
			return err
		}
		if err := save.Click(); err != nil {
			return fmt.Errorf("saving billing: %w", err)
		}
		if err := u.expect.Locator(page.Locator(`[data-error-for="` + refusal.field + `"]`)).ToContainText(refusal.message); err != nil {
			return fmt.Errorf("the form should refuse the %s: %w", refusal.field, err)
		}
	}

	// Stripe refuses the key once: the refusal is shown under the key, and editing it clears it.
	if err := fillAndCheck(page, "#bil-stripe-pk", billingStripePublishableKey); err != nil {
		return err
	}
	if err := u.failProviderCall(ctx, "stripe", map[string]any{"method": "GET", "path": "/v1/webhook_endpoints", "status": 401}); err != nil {
		return err
	}
	if err := save.Click(); err != nil {
		return err
	}
	stripeKeyError := page.Locator(`[data-error-for="stripeApiKey"]`)
	if err := u.expect.Locator(stripeKeyError).ToContainText("This Stripe key is invalid"); err != nil {
		return fmt.Errorf("Stripe's refusal should be shown under the key: %w", err)
	}
	if err := fillAndCheck(page, "#bil-stripe-key", billingStripeSecretKey); err != nil {
		return err
	}
	if err := u.expect.Locator(stripeKeyError).ToBeHidden(); err != nil {
		return err
	}

	if err := u.saveAndReload(page, save, "Billing is set up."); err != nil {
		return err
	}
	if err := u.registerBilling(ctx); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".cfg-billing-mode-badge--test")).ToBeVisible(); err != nil {
		return fmt.Errorf("billing should show Stripe in test mode: %w", err)
	}
	return u.expect.Locator(page.Locator("#cfg-billing-webhook-edit-btn")).ToBeVisible()
}

// billingWebhookBase is the address the shared testbench reaches this block's portal on, where its
// Stripe service sends the portal's payment events.
func (u *Steps) billingWebhookBase() (string, error) {
	return u.topo.CallbackURL("api-portal", "https")
}

// moveWebhookHostAndRemoveBilling moves Stripe's events to the portal's address on the test
// network, refusing an http address first, then removes billing: Moesif failing once keeps it, and
// a second attempt removes it.
func (u *Steps) moveWebhookHostAndRemoveBilling(ctx context.Context) error {
	page, err := u.openSettingsPanel(ctx, "cfg-billing")
	if err != nil {
		return err
	}
	portalBase, err := u.billingWebhookBase()
	if err != nil {
		return err
	}
	if err := page.Locator("#cfg-billing-webhook-edit-btn").Click(); err != nil {
		return fmt.Errorf("opening the webhook host form: %w", err)
	}
	input := page.Locator("#bil-webhook-edit-base")
	save := page.Locator("#bil-webhook-save")
	if err := u.expect.Locator(save).ToBeDisabled(); err != nil {
		return fmt.Errorf("Save should wait for another address: %w", err)
	}
	if err := input.Fill(strings.Replace(portalBase, "https://", "http://", 1)); err != nil {
		return err
	}
	if err := save.Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(`#cfg-billing-webhook-modal [data-error-for="webhookBaseUrl"]`)).ToContainText("https"); err != nil {
		return fmt.Errorf("an http address should be refused under the field: %w", err)
	}
	if err := input.Fill(portalBase); err != nil {
		return err
	}
	if err := u.saveAndReload(page, save, "Webhook host changed."); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-billing-webhook-edit-btn")).ToHaveAttribute("data-base-url", portalBase); err != nil {
		return err
	}

	removeBtn := page.Locator("#cfg-billing-remove-btn")
	if err := removeBtn.Click(); err != nil {
		return fmt.Errorf("opening the remove dialog: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-billing-remove-ask")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-billing-remove-cancel").Click(); err != nil {
		return err
	}
	if err := u.failProviderCall(ctx, "moesif", map[string]any{"method": "GET", "match": "partneraccount", "status": 503}); err != nil {
		return err
	}
	if err := removeBtn.Click(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-billing-remove-confirm").Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Billing could not be removed."); err != nil {
		return fmt.Errorf("a failed removal should keep billing and say so: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#alertToast")).ToHaveClass(toastHidden); err != nil {
		return err
	}
	if err := removeBtn.Click(); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-billing-remove-confirm"), "Billing removed."); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#cfg-billing-setup-btn")).ToBeVisible()
}

const keyBillingMeter = "apiPortalBillingMeter"

// meterRow is the settings row of the meter with the display name.
func meterRow(page playwright.Page, displayName string) playwright.Locator {
	return page.Locator("#cfg-meters tr[data-id]", playwright.PageLocatorOptions{HasText: displayName})
}

// addMeterInSettings adds a Tokens meter in Settings › Meters: the form asks for an event key and
// for each property's name and key, the Token key and a property come from the suggestions (by
// keyboard and by pointer), and a property added by mistake is removed. A second meter with the
// same name is refused by the server.
func (u *Steps) addMeterInSettings(ctx context.Context) error {
	page, err := u.openSettingsPanel(ctx, "cfg-meters")
	if err != nil {
		return err
	}
	id, err := unique.Unique(ctx, "ui-meter")
	if err != nil {
		return err
	}
	name := "Tokens " + id
	if err := page.Locator("#cfg-add-meter-btn").Click(); err != nil {
		return fmt.Errorf("opening the meter form: %w", err)
	}
	save := page.Locator("#cfg-meter-modal-save")
	if err := u.expect.Locator(save).ToBeDisabled(); err != nil {
		return fmt.Errorf("Add Meter should wait for a name: %w", err)
	}
	if err := fillAndCheck(page, "#mtr-display", name); err != nil {
		return err
	}
	if err := fillAndCheck(page, "#mtr-desc", "Tokens the API's language model uses"); err != nil {
		return err
	}
	if _, err := page.Locator("#mtr-type").SelectOption(playwright.SelectOptionValues{Values: &[]string{"TOKENS"}}); err != nil {
		return fmt.Errorf("choosing a Tokens meter: %w", err)
	}
	typeKey := page.Locator("#mtr-type-key")
	if err := u.expect.Locator(page.Locator("#mtr-token-row")).ToBeVisible(); err != nil {
		return err
	}
	defaultKey, err := typeKey.InputValue()
	if err != nil || defaultKey == "" {
		return fmt.Errorf("a Tokens meter should start with the default Token key, got %q: %v", defaultKey, err)
	}
	// An empty Token key is refused.
	if err := typeKey.Fill(""); err != nil {
		return err
	}
	if err := save.Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Enter an event key."); err != nil {
		return err
	}
	// The Token key from the suggestions, by keyboard: Down opens the list, Down and Up move, Enter
	// chooses; Escape closes the list again.
	if err := typeKey.Press("ArrowDown"); err != nil {
		return err
	}
	tokenList := page.Locator("#mtr-token-key-combo .cfg-view-combo-menu")
	if err := u.expect.Locator(tokenList).ToBeVisible(); err != nil {
		return err
	}
	for _, key := range []string{"ArrowDown", "ArrowUp", "Enter"} {
		if err := page.Keyboard().Press(key); err != nil {
			return err
		}
	}
	if err := u.expect.Locator(tokenList).ToBeHidden(); err != nil {
		return err
	}
	if value, err := typeKey.InputValue(); err != nil || value == "" {
		return fmt.Errorf("choosing a suggested Token key should fill it in, got %q: %v", value, err)
	}
	toggle := page.Locator("#mtr-token-key-combo .cfg-key-combo-toggle")
	if err := toggle.Click(); err != nil {
		return err
	}
	if err := typeKey.Press("Escape"); err != nil {
		return err
	}
	if err := u.expect.Locator(tokenList).ToBeHidden(); err != nil {
		return err
	}

	// A property needs a name and a key.
	addFilter := page.Locator("#mtr-add-filter-btn")
	if err := addFilter.Click(); err != nil {
		return fmt.Errorf("adding a property: %w", err)
	}
	if err := save.Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Each filter needs a name and an event key."); err != nil {
		return err
	}
	// The first property from the suggestions, by pointer: its key is filled in.
	rows := page.Locator("#mtr-filters-list .cfg-filter-row")
	first := rows.Nth(0)
	if err := first.Locator(".cfg-key-combo-toggle").Click(); err != nil {
		return fmt.Errorf("opening the suggested properties: %w", err)
	}
	if err := first.Locator(`[role="option"]`).First().Click(); err != nil {
		return fmt.Errorf("choosing a suggested property: %w", err)
	}
	if key, err := first.Locator(".mtr-filter-key").InputValue(); err != nil || key == "" {
		return fmt.Errorf("a suggested property should fill in its key, got %q: %v", key, err)
	}
	// A custom property, and one added by mistake and removed.
	for i, property := range [][2]string{{"Region", "metadata.region"}, {"Mistake", "metadata.mistake"}} {
		if err := addFilter.Click(); err != nil {
			return err
		}
		row := rows.Nth(i + 1)
		if err := row.Locator(".mtr-filter-name").Fill(property[0]); err != nil {
			return err
		}
		if err := row.Locator(".mtr-filter-key").Fill(property[1]); err != nil {
			return err
		}
	}
	if err := rows.Nth(2).Locator("button.cfg-icon-btn--danger").Click(); err != nil {
		return fmt.Errorf("removing a property: %w", err)
	}
	if err := u.expect.Locator(rows).ToHaveCount(2); err != nil {
		return err
	}
	if err := u.saveAndReload(page, save, "Meter created."); err != nil {
		return err
	}
	row := meterRow(page, name)
	if err := u.expect.Locator(row).ToBeVisible(); err != nil {
		return err
	}
	meterID, err := row.GetAttribute("data-id")
	if err != nil || meterID == "" {
		return fmt.Errorf("reading the new meter's id: %v", err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalMeter, ID: meterID, Actor: u.topo.Admin.Username}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyBillingMeter, name); err != nil {
		return err
	}

	// Two properties that read the same event key are refused by the server, and the form says why.
	if err := page.Locator("#cfg-add-meter-btn").Click(); err != nil {
		return err
	}
	if err := fillAndCheck(page, "#mtr-display", name+" again"); err != nil {
		return err
	}
	for i, property := range []string{"Region", "Area"} {
		if err := addFilter.Click(); err != nil {
			return err
		}
		row := rows.Nth(i)
		if err := row.Locator(".mtr-filter-name").Fill(property); err != nil {
			return err
		}
		if err := row.Locator(".mtr-filter-key").Fill("metadata.region"); err != nil {
			return err
		}
	}
	if err := save.Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Meter could not be saved"); err != nil {
		return err
	}
	return page.Locator("#cfg-meter-modal-cancel").Click()
}

// editAndDeleteMeter changes the meter's description, then deletes it after cancelling once.
func (u *Steps) editAndDeleteMeter(ctx context.Context) error {
	value, err := tcontext.Resolve(ctx, keyBillingMeter)
	if err != nil {
		return err
	}
	name, _ := value.(string)
	page, err := u.openSettingsPanel(ctx, "cfg-meters")
	if err != nil {
		return err
	}
	row := meterRow(page, name)
	if err := row.Locator(".cfg-meter-edit-btn").Click(); err != nil {
		return fmt.Errorf("opening the meter: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-meter-modal-title")).ToHaveText("Edit meter"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#mtr-filters-list .cfg-filter-row")).ToHaveCount(2); err != nil {
		return fmt.Errorf("the meter should open with its properties: %w", err)
	}
	if err := fillAndCheck(page, "#mtr-desc", "Tokens counted for billing"); err != nil {
		return err
	}
	if _, err := page.Locator("#mtr-type").SelectOption(playwright.SelectOptionValues{Values: &[]string{"REQUEST_COUNT"}}); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#mtr-token-row")).ToBeHidden(); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-meter-modal-save"), "Meter updated."); err != nil {
		return err
	}
	if err := u.expect.Locator(row).ToContainText("Tokens counted for billing"); err != nil {
		return err
	}
	if err := row.Locator(".cfg-meter-delete-btn").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-meter-name-txt")).ToHaveText(name); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-meter-cancel").Click(); err != nil {
		return err
	}
	if err := row.Locator(".cfg-meter-delete-btn").Click(); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-del-meter-confirm"), "Meter deleted."); err != nil {
		return err
	}
	return u.expect.Locator(row).ToHaveCount(0)
}

const (
	keyBillingPlan      = "apiPortalBillingPlan"
	keyBillingPlanName  = "apiPortalBillingPlanName"
	keyBillingMeterID   = "apiPortalBillingMeterID"
	keyBillingMeterName = "apiPortalBillingMeterName"
)

// setUpBillingByAPI sets up billing through the REST API, with Stripe's events sent to the portal's
// address on the test network; it is removed when the scenario ends.
func (u *Steps) setUpBillingByAPI(ctx context.Context) error {
	if err := u.resetBillingProviders(ctx); err != nil {
		return err
	}
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	portalBase, err := u.billingWebhookBase()
	if err != nil {
		return err
	}
	status, body, err := u.portalCall(page, http.MethodPost, "/billing-configuration", map[string]string{
		"moesifCollectorApplicationId": billingMoesifApplication + ".collector",
		"moesifManagementKey":          "management." + billingMoesifApplication + ".signature",
		"stripeApiKey":                 billingStripeSecretKey,
		"stripePublishableKey":         billingStripePublishableKey,
		"webhookBaseUrl":               portalBase,
	})
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return fmt.Errorf("setting up billing: status %d %s", status, body)
	}
	return u.registerBilling(ctx)
}

// createRequestMeterByAPI adds a Requests meter whose prices may filter on Status and Method.
func (u *Steps) createRequestMeterByAPI(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	suffix, err := unique.Unique(ctx, "ui-calls")
	if err != nil {
		return err
	}
	name := "Calls " + suffix
	status, body, err := u.portalCall(page, http.MethodPost, "/meters", map[string]any{
		"displayName": name, "type": "REQUEST_COUNT",
		"filterFields": []map[string]string{{"field": "Status", "eventKey": "response.status"},
			{"field": "Method", "eventKey": "request.verb"}},
	})
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return fmt.Errorf("adding a meter: status %d %s", status, body)
	}
	var meter struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &meter); err != nil || meter.ID == "" {
		return fmt.Errorf("reading the new meter's id from %s: %v", body, err)
	}
	id := meter.ID
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalMeter, ID: id, Actor: u.topo.Admin.Username}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyBillingMeterID, id); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyBillingMeterName, name)
}

func resolveText(ctx context.Context, key string) (string, error) {
	value, err := tcontext.Resolve(ctx, key)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("%s holds %T, not text", key, value)
	}
	return text, nil
}

// planRow is the settings row of the plan with the display name.
func planRow(page playwright.Page, displayName string) playwright.Locator {
	return page.Locator("#cfg-plans tr[id^=cfg-plan-row-]", playwright.PageLocatorOptions{HasText: displayName})
}

// saveExpecting clicks save and waits for the toast to say text.
func (u *Steps) saveExpecting(page playwright.Page, save, text string) error {
	if err := page.Locator(save).Click(); err != nil {
		return fmt.Errorf("saving: %w", err)
	}
	if err := u.expectToast(page, text); err != nil {
		return fmt.Errorf("saving should say %q: %w", text, err)
	}
	return nil
}

// saveAndReload clicks a control whose success the page reports in a toast before reloading, and
// waits until the reloaded page has loaded: its scripts have bound the controls the next action
// clicks, and what is asserted next is asserted against it, not the page it replaces.
func (u *Steps) saveAndReload(page playwright.Page, control playwright.Locator, toast string) error {
	if _, err := page.ExpectNavigation(func() error {
		if err := control.Click(); err != nil {
			return err
		}
		return u.expectToast(page, toast)
	}, playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateLoad}); err != nil {
		return fmt.Errorf("saving should say %q and reload the page: %w", toast, err)
	}
	return nil
}

// openNewPlan opens the plan form with a unique name, and returns the name.
func (u *Steps) openNewPlan(ctx context.Context, page playwright.Page, prefix string) (string, error) {
	id, err := unique.Unique(ctx, prefix)
	if err != nil {
		return "", err
	}
	name := "Plan " + id
	if err := page.Locator("#cfg-add-plan-btn").Click(); err != nil {
		return "", fmt.Errorf("opening the plan form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-plan-modal-save")).ToBeDisabled(); err != nil {
		return "", fmt.Errorf("Add plan should wait for a name: %w", err)
	}
	return name, fillAndCheck(page, "#pol-display", name)
}

// registerPlan registers the plan with the display name for cleanup, and stores it for later steps.
func (u *Steps) registerPlan(ctx context.Context, page playwright.Page, name string) error {
	row := planRow(page, name)
	if err := u.expect.Locator(row).ToBeVisible(); err != nil {
		return err
	}
	rowID, err := row.GetAttribute("id")
	if err != nil {
		return err
	}
	planID := strings.TrimPrefix(rowID, "cfg-plan-row-")
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalSubscriptionPlan, ID: planID, Actor: u.topo.Admin.Username}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyBillingPlan, planID); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyBillingPlanName, name)
}

// paidPlanNeedsBilling checks that without billing a plan can't be made paid: the switch stays off,
// says why, and its link opens Settings › Billing.
func (u *Steps) paidPlanNeedsBilling(ctx context.Context) error {
	page, err := u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	if _, err := u.openNewPlan(ctx, page, "ui-unbilled"); err != nil {
		return err
	}
	if err := page.Locator("#pol-paid").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#pol-paid")).Not().ToBeChecked(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#pol-paid-notice")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#pol-paid-setup").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-plan-modal")).ToBeHidden(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#cfg-billing")).ToBeVisible()
}

// priceFlatRatePlanInSettings adds a flat rate plan: the form refuses a limit count and a time
// amount that are not positive and amounts that are not prices; the currency is chosen from the
// searchable list by keyboard and pointer.
func (u *Steps) priceFlatRatePlanInSettings(ctx context.Context) error {
	page, err := u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	name, err := u.openNewPlan(ctx, page, "ui-flat")
	if err != nil {
		return err
	}
	if err := fillAndCheck(page, "#pol-desc", "A flat monthly fee"); err != nil {
		return err
	}
	addLimit := page.Locator("#pol-add-limit-btn")
	limits := page.Locator("#pol-limits-list .cfg-limit-row")
	for range 2 {
		if err := addLimit.Click(); err != nil {
			return err
		}
	}
	if err := limits.Nth(1).Locator(".cfg-limit-remove-btn").Click(); err != nil {
		return err
	}
	limit := limits.Nth(0)
	if err := limit.Locator(".cfg-limit-count").Fill("0"); err != nil {
		return err
	}
	if err := u.saveExpecting(page, "#cfg-plan-modal-save", "Limit count must be -1 (unlimited) or a positive number."); err != nil {
		return err
	}
	if err := limit.Locator(".cfg-limit-count").Fill("1000"); err != nil {
		return err
	}
	if err := limit.Locator(".cfg-limit-amount").Fill("0"); err != nil {
		return err
	}
	if err := u.saveExpecting(page, "#cfg-plan-modal-save", "Limit time amount must be a positive number."); err != nil {
		return err
	}
	if err := limit.Locator(".cfg-limit-amount").Fill("1"); err != nil {
		return err
	}
	if _, err := limit.Locator(".cfg-limit-unit").SelectOption(playwright.SelectOptionValues{Values: &[]string{"MINUTE"}}); err != nil {
		return err
	}

	if err := page.Locator("#pol-paid").Check(); err != nil {
		return fmt.Errorf("making the plan paid: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#pol-pricing")).ToBeVisible(); err != nil {
		return err
	}
	// The currency, from the searchable list: typing narrows it, Enter takes the first shown.
	trigger := page.Locator("#pol-currency-trigger")
	search := page.Locator("#pol-currency-combo .cfg-view-combo-input")
	menu := page.Locator("#pol-currency-combo .cfg-currency-menu")
	if err := trigger.Click(); err != nil {
		return err
	}
	if err := search.Fill("zzzz"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#pol-currency-combo .cfg-view-combo-empty")).ToBeVisible(); err != nil {
		return err
	}
	if err := search.Fill("euro"); err != nil {
		return err
	}
	if err := search.Press("Enter"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#pol-currency")).ToHaveValue("EUR"); err != nil {
		return err
	}
	// By keyboard through the options, then closed with Escape, Tab and a click elsewhere.
	if err := trigger.Click(); err != nil {
		return err
	}
	for _, key := range []string{"ArrowDown", "ArrowDown", "ArrowUp", "ArrowUp", "ArrowDown", "Escape"} {
		if err := page.Keyboard().Press(key); err != nil {
			return err
		}
	}
	if err := u.expect.Locator(menu).ToBeHidden(); err != nil {
		return err
	}
	if err := trigger.Click(); err != nil {
		return err
	}
	if err := search.Press("Escape"); err != nil {
		return err
	}
	for _, key := range []string{"ArrowDown", "Tab"} {
		if err := trigger.Click(); err != nil {
			return err
		}
		if key == "ArrowDown" {
			if err := search.Press("ArrowDown"); err != nil {
				return err
			}
			if err := page.Keyboard().Press("Tab"); err != nil {
				return err
			}
		} else if err := search.Press("Tab"); err != nil {
			return err
		}
		if err := u.expect.Locator(menu).ToBeHidden(); err != nil {
			return err
		}
	}
	if err := trigger.Click(); err != nil {
		return err
	}
	if err := page.Locator("#pol-display").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(menu).ToBeHidden(); err != nil {
		return err
	}
	if err := trigger.Click(); err != nil {
		return err
	}
	if err := page.Locator(`#pol-currency-list [role="option"][data-value="USD"]`).Click(); err != nil {
		return err
	}
	if _, err := page.Locator("#pol-period").SelectOption(playwright.SelectOptionValues{Values: &[]string{"YEAR"}}); err != nil {
		return err
	}
	for amount, message := range map[string]string{"ten": "Amount must be a number such as 20.00.", "0.00": "Amount must be more than 0."} {
		if err := fillAndCheck(page, "#pol-flat-amount", amount); err != nil {
			return err
		}
		if err := u.saveExpecting(page, "#cfg-plan-modal-save", message); err != nil {
			return err
		}
	}
	if err := fillAndCheck(page, "#pol-flat-amount", "49.00"); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-plan-modal-save"), "Plan created."); err != nil {
		return err
	}
	return u.registerPlan(ctx, page, name)
}

// editAndDeletePlan renames the plan's description, its pricing shown read-only, then deletes it.
func (u *Steps) editAndDeletePlan(ctx context.Context) error {
	name, err := resolveText(ctx, keyBillingPlanName)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	row := planRow(page, name)
	if err := row.Locator(".cfg-plan-edit-btn").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-plan-modal-title")).ToHaveText("Edit subscription plan"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#pol-pricing-fields input").First()).ToBeDisabled(); err != nil {
		return fmt.Errorf("a saved plan's pricing should be read-only: %w", err)
	}
	if err := fillAndCheck(page, "#pol-desc", "A flat yearly fee"); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-plan-modal-save"), "Plan updated."); err != nil {
		return err
	}
	if err := row.Locator(".cfg-plan-delete-btn").Click(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-plan-cancel").Click(); err != nil {
		return err
	}
	if err := row.Locator(".cfg-plan-delete-btn").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-plan-name-txt")).ToHaveText(name); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-del-plan-confirm"), "Plan deleted."); err != nil {
		return err
	}
	return u.expect.Locator(row).ToHaveCount(0)
}

// priceBox is one usage price of the plan form, numbered from 1.
type priceBox struct {
	box playwright.Locator
}

func (u *Steps) price(page playwright.Page, n int) priceBox {
	return priceBox{box: page.Locator("#pol-prices-list .cfg-price-box").Nth(n - 1)}
}

// filter sets a price's filter on a property: its operator and values; second sets the property's
// second comparison.
func (p priceBox) filter(field, operator, values string, second bool) error {
	selector := `.cfg-filter-row[data-field="` + field + `"]`
	if second {
		selector += "[data-second]"
	} else {
		selector += ":not([data-second])"
	}
	row := p.box.Locator(selector)
	if _, err := row.Locator(".pol-filter-operator").SelectOption(playwright.SelectOptionValues{Values: &[]string{operator}}); err != nil {
		return fmt.Errorf("choosing %s %s: %w", field, operator, err)
	}
	return row.Locator(".pol-filter-values").Fill(values)
}

func (p priceBox) clearFilter(field string) error {
	return p.box.Locator(`.cfg-filter-row[data-field="` + field + `"]:not([data-second]) .pol-filter-values`).Fill("")
}

func (p priceBox) comparisonButton(field string, second bool) playwright.Locator {
	selector := `.cfg-filter-row[data-field="` + field + `"]`
	if second {
		selector += "[data-second]"
	} else {
		selector += ":not([data-second])"
	}
	return p.box.Locator(selector + " .cfg-icon-btn")
}

func (p priceBox) tier(n int) playwright.Locator {
	return p.box.Locator(".pol-tiers-list .cfg-tier-row").Nth(n - 1)
}

// priceUsagePlanInSettings adds a usage based plan with two prices on one meter. The form refuses a
// price without a meter, values its operators can't take, comparisons that can't all be true,
// amounts and units that are not valid, tiers out of order or without a price, and two prices that
// would count the same requests.
func (u *Steps) priceUsagePlanInSettings(ctx context.Context) error {
	meterID, err := resolveText(ctx, keyBillingMeterID)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	name, err := u.openNewPlan(ctx, page, "ui-usage")
	if err != nil {
		return err
	}
	if err := page.Locator("#pol-paid").Check(); err != nil {
		return err
	}
	if err := page.Locator(`input[name="pol-pricing-type"][value="USAGE_BASED"]`).Check(); err != nil {
		return err
	}
	save := "#cfg-plan-modal-save"
	first := u.price(page, 1)
	if err := u.expect.Locator(first.box.Locator(".pol-price-remove")).ToBeHidden(); err != nil {
		return fmt.Errorf("the only price should have no Remove: %w", err)
	}
	if err := u.saveExpecting(page, save, "Select a meter for price 1."); err != nil {
		return err
	}
	if _, err := first.box.Locator(".pol-price-meter").SelectOption(playwright.SelectOptionValues{Values: &[]string{meterID}}); err != nil {
		return err
	}
	if err := u.expect.Locator(first.box.Locator(`.cfg-filter-row[data-field="Method"]`)).ToBeVisible(); err != nil {
		return fmt.Errorf("the meter's properties should be offered as filters: %w", err)
	}

	steps := []struct {
		set     func() error
		message string
	}{
		{func() error { return first.filter("Status", "GREATER_THAN", "many", false) }, "Price 1 compares with a whole number"},
		{func() error {
			if err := first.filter("Status", "GREATER_THAN", "199", false); err != nil {
				return err
			}
			if err := first.comparisonButton("Status", false).Click(); err != nil {
				return err
			}
			return first.filter("Status", "LESS_THAN", "100", true)
		}, "Price 1: the comparisons on Status can't all be true."},
		{func() error { return first.filter("Status", "LESS_THAN", "300", true) }, "Price 1 must be an amount such as 0.01."},
	}
	for _, step := range steps {
		if err := step.set(); err != nil {
			return err
		}
		if err := u.saveExpecting(page, save, step.message); err != nil {
			return err
		}
	}
	if err := first.box.Locator(".pol-price-unit-amount").Fill("0.01"); err != nil {
		return err
	}
	if err := first.box.Locator(".pol-price-unit-quantity").Fill("0"); err != nil {
		return err
	}
	if err := u.saveExpecting(page, save, "Units of price 1 must be a whole number of at least 1."); err != nil {
		return err
	}
	if err := first.box.Locator(".pol-price-unit-quantity").Fill("1000"); err != nil {
		return err
	}
	// The second comparison goes with its Remove, and with the first row no longer comparing.
	if err := first.comparisonButton("Status", true).Click(); err != nil {
		return err
	}
	if err := first.comparisonButton("Status", false).Click(); err != nil {
		return err
	}
	if err := first.filter("Status", "IN", "200, 201, 200", false); err != nil {
		return err
	}
	if err := u.expect.Locator(first.box.Locator(`.cfg-filter-row[data-second]`)).ToHaveCount(0); err != nil {
		return err
	}

	// A second price on the same meter must not count the same requests.
	if err := page.Locator("#pol-add-price-btn").Click(); err != nil {
		return err
	}
	second := u.price(page, 2)
	if _, err := second.box.Locator(".pol-price-meter").SelectOption(playwright.SelectOptionValues{Values: &[]string{meterID}}); err != nil {
		return err
	}
	if err := second.box.Locator(".pol-price-unit-amount").Fill("0.05"); err != nil {
		return err
	}
	overlapping := []func() error{
		func() error { return nil },
		func() error {
			if err := first.clearFilter("Status"); err != nil {
				return err
			}
			if err := first.filter("Method", "STARTS_WITH", "G", false); err != nil {
				return err
			}
			return second.filter("Method", "STARTS_WITH", "GE", false)
		},
		func() error {
			if err := first.filter("Method", "EQUALS", "GET", false); err != nil {
				return err
			}
			return second.filter("Method", "STARTS_WITH", "G", false)
		},
		func() error {
			if err := first.filter("Method", "NOT_EQUALS", "POST", false); err != nil {
				return err
			}
			return second.filter("Method", "STARTS_WITH", "P", false)
		},
		func() error {
			if err := first.clearFilter("Method"); err != nil {
				return err
			}
			if err := second.clearFilter("Method"); err != nil {
				return err
			}
			if err := first.filter("Status", "GREATER_THAN", "199", false); err != nil {
				return err
			}
			if err := first.comparisonButton("Status", false).Click(); err != nil {
				return err
			}
			if err := first.filter("Status", "LESS_THAN", "300", true); err != nil {
				return err
			}
			return second.filter("Status", "EQUALS", "250", false)
		},
		func() error { return second.filter("Status", "GREATER_THAN", "250", false) },
	}
	for _, set := range overlapping {
		if err := set(); err != nil {
			return err
		}
		if err := u.saveExpecting(page, save, "Prices 1 and 2 both count the same requests"); err != nil {
			return err
		}
	}
	// The prices it keeps: 200 and 201 on GET for the first, any other status for the second.
	if err := first.comparisonButton("Status", true).Click(); err != nil {
		return err
	}
	if err := first.filter("Status", "IN", "200, 201", false); err != nil {
		return err
	}
	if err := first.filter("Method", "EQUALS", "GET", false); err != nil {
		return err
	}
	if err := second.filter("Status", "NOT_IN", "200, 201", false); err != nil {
		return err
	}

	// The second price by volume, with tiers.
	if _, err := second.box.Locator(".pol-price-model").SelectOption(playwright.SelectOptionValues{Values: &[]string{"VOLUME"}}); err != nil {
		return err
	}
	if err := u.expect.Locator(second.box.Locator(".pol-tiers-list .cfg-tier-row")).ToHaveCount(2); err != nil {
		return err
	}
	tierSteps := []struct {
		set     func() error
		message string
	}{
		{func() error { return second.tier(1).Locator(".cfg-tier-upto").Fill("") }, "Each tier of price 2 except the last needs an upper bound."},
		{func() error { return second.tier(1).Locator(".cfg-tier-upto").Fill("100") }, "Each tier of price 2 needs a per-unit price, a flat fee or both."},
		{func() error { return second.tier(1).Locator(".cfg-tier-unit").Fill("two") }, "Per-unit price of price 2 must be an amount"},
		{func() error {
			if err := second.tier(1).Locator(".cfg-tier-unit").Fill("0.02"); err != nil {
				return err
			}
			return second.tier(2).Locator(".cfg-tier-flat").Fill("five")
		}, "Flat fee of price 2 must be an amount"},
		{func() error {
			if err := second.tier(2).Locator(".cfg-tier-flat").Fill("5"); err != nil {
				return err
			}
			if err := second.box.Locator(".pol-add-tier-btn").Click(); err != nil {
				return err
			}
			if err := second.tier(2).Locator(".cfg-tier-upto").Fill("50"); err != nil {
				return err
			}
			return second.tier(2).Locator(".cfg-tier-unit").Fill("0.01")
		}, "Each tier of price 2 needs an upper bound greater than the tier before it."},
	}
	for _, step := range tierSteps {
		if err := step.set(); err != nil {
			return err
		}
		if err := u.saveExpecting(page, save, step.message); err != nil {
			return err
		}
	}
	if err := second.tier(2).Locator(".cfg-icon-btn--danger").Click(); err != nil {
		return err
	}
	if _, err := second.box.Locator(".pol-price-model").SelectOption(playwright.SelectOptionValues{Values: &[]string{"GRADUATED"}}); err != nil {
		return err
	}
	if err := u.expect.Locator(second.box.Locator(".pol-tier-range-label")).ToHaveText("For units numbered"); err != nil {
		return err
	}
	// A third price, added and removed again.
	if err := page.Locator("#pol-add-price-btn").Click(); err != nil {
		return err
	}
	if err := u.price(page, 3).box.Locator(".pol-price-remove").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#pol-prices-list .cfg-price-box")).ToHaveCount(2); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator(save), "Plan created."); err != nil {
		return err
	}
	return u.registerPlan(ctx, page, name)
}
