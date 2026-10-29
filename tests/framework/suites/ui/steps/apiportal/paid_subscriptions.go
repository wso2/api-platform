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
	"net/url"
	"regexp"
	"time"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// The paid API or MCP server a developer buys a plan on, and what the purchase made.
const (
	keyPaidPagePath     = "apiPortalPaidPagePath"
	keyPaidArtifactName = "apiPortalPaidArtifactName"
	keyPaidPlan         = "apiPortalPaidPlan"
	keyPaidPlanName     = "apiPortalPaidPlanName"
	keyPaidCheckout     = "apiPortalPaidCheckout"
	keyPaidSubscription = "apiPortalPaidSubscription"
)

// stripeJS stands in for Stripe.js, which the portal loads from js.stripe.com to show Stripe's
// checkout form. Its form is a Pay button that reports the payment complete, as Stripe does once
// the buyer has paid.
const stripeJS = `window.Stripe = function () {
  return {
    initEmbeddedCheckout: async function (options) {
      await options.fetchClientSecret();
      var pay = document.createElement('button');
      pay.type = 'button';
      pay.id = 'stripeTestPay';
      pay.textContent = 'Pay';
      pay.addEventListener('click', function () { options.onComplete(); });
      return {
        mount: function (selector) { document.querySelector(selector).appendChild(pay); },
        destroy: function () { pay.remove(); },
      };
    },
  };
};`

const (
	stripeJSAddress = "https://js.stripe.com/**"
	// The Usage page draws its charts with Chart.js from this CDN, and plain bars without it.
	chartJSAddress = "https://cdn.jsdelivr.net/**"
	// The testbench's Stripe service sends the buyer to its billing page on this host.
	stripeBillingAddress = "https://billing.stripe.test/**"
)

// paidSwitchHint is what a plan card says when its plan can't be bought beside a paid subscription.
const paidSwitchHint = "To switch to or from a paid plan, unsubscribe first."

func (u *Steps) registerPaidSubscriptionSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the (administrator|developer) signs in to the API Portal$`, u.signInAs)
	sc.Step(`^the API Portal has an API with a free plan and a paid flat rate plan$`, u.createPaidAPI)
	sc.Step(`^the API Portal has an MCP server with a paid usage based plan$`, u.createPaidMCPServer)
	sc.Step(`^the developer starts paying for the flat rate plan, after checkouts that fail or are not paid$`,
		u.startPayingFlatRatePlan)
	sc.Step(`^the developer pays for the plan and receives its token$`, u.payForPlan)
	sc.Step(`^the developer pages through the invoices, and opens the billing page$`, u.pageThroughInvoices)
	sc.Step(`^the administrator reads the revenue of the paid plan by month$`, u.readRevenue)
	sc.Step(`^the developer unsubscribes from the flat rate plan, and ends it now in Subscriptions$`,
		u.unsubscribeAndEndNow)
	sc.Step(`^the developer starts paying for the usage based plan on the MCP server page$`, u.startPayingUsagePlan)
	sc.Step(`^the developer follows the subscription's usage per hour, day and week$`, u.followUsage)
	sc.Step(`^the developer sees the payment pending in Subscriptions and in Usage while Stripe can't collect it$`,
		u.seePaymentPending)
	sc.Step(`^the developer unsubscribes from the usage based plan on the MCP server page$`, u.unsubscribeFromUsagePlan)
}

// signInAs signs out, and signs in again as the administrator or the developer.
func (u *Steps) signInAs(ctx context.Context, role string) error {
	if role == "developer" {
		return u.loginAsDeveloper(ctx)
	}
	return u.loginAs(ctx, u.topo.Admin.Username, u.topo.Admin.Password)
}

// createPlan adds a plan through the REST API, paid when pricing is given, and registers it for
// cleanup. It returns the plan's id and name.
func (u *Steps) createPlan(ctx context.Context, page playwright.Page, prefix string, pricing map[string]any) (string, string, error) {
	suffix, err := unique.Unique(ctx, prefix)
	if err != nil {
		return "", "", err
	}
	name := "Plan " + suffix
	plan := map[string]any{
		"displayName": name, "status": "ACTIVE",
		"limits": []map[string]any{{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 10000}},
	}
	if pricing != nil {
		plan["pricing"] = pricing
	}
	status, body, err := u.portalCall(page, http.MethodPost, "/subscription-plans", plan)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusCreated {
		return "", "", fmt.Errorf("adding a plan: status %d %s", status, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || created.ID == "" {
		return "", "", fmt.Errorf("reading the new plan's id from %s: %v", body, err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalSubscriptionPlan, ID: created.ID, Actor: u.topo.Admin.Username}); err != nil {
		return "", "", err
	}
	return created.ID, name, nil
}

// rememberPaidArtifact stores the page a plan is bought on, and the plan.
func rememberPaidArtifact(ctx context.Context, path, name, planID, planName string) error {
	for key, value := range map[string]string{
		keyPaidPagePath: path, keyPaidArtifactName: name, keyPaidPlan: planID, keyPaidPlanName: planName,
	} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func paidEndpoints(id string) map[string]string {
	return map[string]string{
		"productionURL": "https://backend.example.invalid/" + id,
		"sandboxURL":    "https://sandbox.example.invalid/" + id,
	}
}

// createPaidAPI publishes a REST API offering a free plan and a flat rate plan of 29 USD a month.
func (u *Steps) createPaidAPI(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	freePlanID, _, err := u.createPlan(ctx, page, "free", nil)
	if err != nil {
		return err
	}
	planID, planName, err := u.createPlan(ctx, page, "monthly", map[string]any{
		"pricingType": "FLAT_RATE", "currency": "USD", "billingPeriod": "MONTH", "flatAmount": "29",
	})
	if err != nil {
		return err
	}
	apiID, err := unique.Unique(ctx, "paid-api")
	if err != nil {
		return err
	}
	apiName := "Paid API " + apiID
	metadata := map[string]any{
		"id": apiID, "name": apiName, "version": "v1.0", "type": "REST",
		"status": "PUBLISHED", "labels": []string{"default"}, "endPoints": paidEndpoints(apiID),
		"subscriptionPlans": []map[string]string{{"id": freePlanID}, {"id": planID}},
	}
	definition := map[string]any{
		"openapi": "3.0.3", "info": map[string]string{"title": apiName, "version": "1.0.0"},
		"paths": map[string]any{"/items": map[string]any{
			"get": map[string]any{"responses": map[string]any{"200": map[string]string{"description": "ok"}}},
		}},
	}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/apis", metadata, definition); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalAPI, ID: apiID, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", apiID)
		return fmt.Errorf("registering the paid API for cleanup: %w", err)
	}
	return rememberPaidArtifact(ctx, "/api/"+apiID, apiName, planID, planName)
}

// createPaidMCPServer publishes an MCP server offering a plan that charges for its successful
// calls in two tiers, counted by the scenario's meter.
func (u *Steps) createPaidMCPServer(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	meterID, err := resolveText(ctx, keyBillingMeterID)
	if err != nil {
		return err
	}
	planID, planName, err := u.createPlan(ctx, page, "metered", map[string]any{
		"pricingType": "USAGE_BASED", "currency": "USD", "billingPeriod": "MONTH",
		"prices": []map[string]any{{
			"meterId": meterID, "pricingModel": "GRADUATED",
			"filters": []map[string]any{{"field": "Status", "operator": "EQUALS", "values": []string{"200"}}},
			"tiers":   []map[string]any{{"upTo": 1000, "unitAmount": "0.02"}, {"upTo": nil, "unitAmount": "0.01"}},
		}},
	})
	if err != nil {
		return err
	}
	serverID, err := unique.Unique(ctx, "paid-mcp")
	if err != nil {
		return err
	}
	serverName := "Paid MCP " + serverID
	metadata := map[string]any{
		"id": serverID, "name": serverName, "version": "v1.0", "type": "MCP",
		"status": "PUBLISHED", "labels": []string{"default"}, "endPoints": paidEndpoints(serverID),
		"subscriptionPlans": []map[string]string{{"id": planID}},
	}
	definition := []map[string]any{{
		"type": "TOOL", "name": "forecast", "description": "Tomorrow's weather",
		"inputSchema": map[string]any{"type": "object"},
	}}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/mcp-servers", metadata, definition); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalMCP, ID: serverID, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "mcp-servers", serverID)
		return fmt.Errorf("registering the paid MCP server for cleanup: %w", err)
	}
	return rememberPaidArtifact(ctx, "/mcp/"+serverID, serverName, planID, planName)
}

// openPaidPage opens the paid API's or MCP server's page, with the query when one is given.
func (u *Steps) openPaidPage(ctx context.Context, query string) (playwright.Page, error) {
	path, err := resolveText(ctx, keyPaidPagePath)
	if err != nil {
		return nil, err
	}
	if err := u.openAPIPortalPath(ctx, path+query); err != nil {
		return nil, err
	}
	return u.page(ctx)
}

// paidSubscribeButton is the Subscribe button of the paid plan's card.
func paidSubscribeButton(page playwright.Page, planID string) playwright.Locator {
	return page.Locator(`.paid-subscribe-btn[data-plan-id="` + planID + `"]`)
}

// serveStripeJS answers the page's requests for Stripe.js with the stand-in, or fails them when
// available is false, as when js.stripe.com can't be reached.
func serveStripeJS(page playwright.Page, available bool) error {
	if err := page.Unroute(stripeJSAddress); err != nil {
		return err
	}
	return page.Route(stripeJSAddress, func(route playwright.Route) {
		if !available {
			_ = route.Abort()
			return
		}
		_ = route.Fulfill(playwright.RouteFulfillOptions{Body: stripeJS, ContentType: playwright.String("text/javascript")})
	})
}

// startCheckout chooses the paid plan, continues to payment and waits for Stripe's form. It
// returns the checkout the portal started.
func (u *Steps) startCheckout(ctx context.Context, page playwright.Page) (string, error) {
	planID, err := resolveText(ctx, keyPaidPlan)
	if err != nil {
		return "", err
	}
	if err := paidSubscribeButton(page, planID).Click(); err != nil {
		return "", fmt.Errorf("choosing the paid plan: %w", err)
	}
	response, err := page.ExpectResponse(regexp.MustCompile(`/checkout-sessions$`), func() error {
		return page.Locator("#paidSubscribeContinue").Click()
	})
	if err != nil {
		return "", fmt.Errorf("continuing to payment: %w", err)
	}
	body, err := response.Body()
	if err != nil {
		return "", err
	}
	var started struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &started); err != nil || started.ID == "" {
		return "", fmt.Errorf("starting a checkout: status %d %s", response.Status(), body)
	}
	if err := u.expect.Locator(page.Locator("#stripeTestPay")).ToBeVisible(); err != nil {
		return "", fmt.Errorf("Stripe's checkout form should open: %w", err)
	}
	return started.ID, tcontext.Set(ctx, keyPaidCheckout, started.ID)
}

// stripeControl sends a request to the testbench Stripe service's /test endpoints, and fails when
// Stripe or an endpoint it sent events to did not accept it.
func (u *Steps) stripeControl(ctx context.Context, path string, body any) error {
	response, err := u.testbenchCall(ctx, "stripe", http.MethodPost, path, body)
	if err != nil {
		return err
	}
	if !response.Succeeded() {
		return fmt.Errorf("Stripe refused %s: %s", path, response.Describe())
	}
	var sent struct {
		Deliveries []struct {
			URL    string `json:"url"`
			Status int    `json:"status"`
			Error  string `json:"error"`
		} `json:"deliveries"`
	}
	if err := json.Unmarshal(response.Body, &sent); err != nil {
		return nil
	}
	for _, delivery := range sent.Deliveries {
		if delivery.Status < 200 || delivery.Status >= 300 {
			return fmt.Errorf("Stripe's event for %s was not accepted by %s: status %d %s",
				path, delivery.URL, delivery.Status, delivery.Error)
		}
	}
	return nil
}

// expectCheckoutStatus waits for the checkout result dialog's title.
func (u *Steps) expectCheckoutStatus(page playwright.Page, title string) error {
	if err := u.expect.Locator(page.Locator("#checkoutStatusModal")).ToBeVisible(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#checkoutStatusTitle")).ToHaveText(title)
}

// startPayingFlatRatePlan starts buying the flat rate plan on the API's page. The plan's dialog
// shows its fee, and closes without buying. Stripe refusing the checkout, and Stripe.js that can't
// be loaded, each stop the purchase with a message. Then Stripe's form opens, and is closed with
// the checkout still open: coming back from Stripe without paying says so and offers to try again,
// and once Stripe has ended the checkout, coming back says it ended. A new checkout is then started.
func (u *Steps) startPayingFlatRatePlan(ctx context.Context) error {
	page, err := u.openPaidPage(ctx, "")
	if err != nil {
		return err
	}
	planID, err := resolveText(ctx, keyPaidPlan)
	if err != nil {
		return err
	}
	planName, err := resolveText(ctx, keyPaidPlanName)
	if err != nil {
		return err
	}
	subscribe := paidSubscribeButton(page, planID)
	dialog := page.Locator("#paidSubscribeModal")
	if err := subscribe.Click(); err != nil {
		return fmt.Errorf("choosing the paid plan: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#paidSubscribeTitle")).ToHaveText("Subscribe to " + planName + " plan"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#paidSubscribePricing")).ToContainText("Monthly fee"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#paidSubscribeTerms")).ToContainText("charged when the subscription starts"); err != nil {
		return err
	}
	if err := dialog.Locator(".paid-modal-close").First().Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(dialog).ToBeHidden(); err != nil {
		return err
	}

	continueButton := page.Locator("#paidSubscribeContinue")
	if err := u.failProviderCall(ctx, "stripe", map[string]any{"method": "POST", "path": "/v1/checkout/sessions", "status": 500}); err != nil {
		return err
	}
	if err := subscribe.Click(); err != nil {
		return err
	}
	if err := continueButton.Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Checkout could not be started: "); err != nil {
		return err
	}
	if err := serveStripeJS(page, false); err != nil {
		return err
	}
	if err := subscribe.Click(); err != nil {
		return err
	}
	if err := continueButton.Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Checkout could not be started. Check your connection and try again."); err != nil {
		return err
	}

	if err := serveStripeJS(page, true); err != nil {
		return err
	}
	checkout, err := u.startCheckout(ctx, page)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#checkoutPayTitle")).ToHaveText("Subscribe to " + planName + " plan"); err != nil {
		return err
	}
	payment := page.Locator("#checkoutPayModal")
	if err := payment.Locator(".checkout-pay-close").First().Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(payment).ToBeHidden(); err != nil {
		return err
	}

	page, err = u.openPaidPage(ctx, "?checkout="+url.QueryEscape(checkout)+"&checkoutResult=cancel")
	if err != nil {
		return err
	}
	if err := u.expectCheckoutStatus(page, "Payment not completed"); err != nil {
		return err
	}
	if err := page.Locator("#checkoutStatusRetry").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(dialog).ToBeVisible(); err != nil {
		return fmt.Errorf("Try again should open the plan's dialog: %w", err)
	}
	if err := dialog.Locator(".paid-modal-close").First().Click(); err != nil {
		return err
	}

	if err := u.stripeControl(ctx, "/test/checkout-sessions/"+url.PathEscape(checkout)+"/expire", nil); err != nil {
		return err
	}
	if err := retry.Await(ctx, retry.Options{Interval: time.Second}, func(context.Context) (int, error) {
		status, _, err := u.portalCall(page, http.MethodGet, "/checkout-sessions/"+url.PathEscape(checkout), nil)
		if err != nil {
			return 0, retry.Transient(err)
		}
		return status, nil
	}, func(status int) bool { return status == http.StatusNotFound }, "waiting for the portal to end the checkout Stripe ended"); err != nil {
		return err
	}
	page, err = u.openPaidPage(ctx, "?checkout="+url.QueryEscape(checkout)+"&checkoutResult=success")
	if err != nil {
		return err
	}
	if err := u.expectCheckoutStatus(page, "Checkout ended"); err != nil {
		return err
	}
	if err := page.Locator("#checkoutStatusModal .checkout-status-close").First().Click(); err != nil {
		return err
	}
	_, err = u.startCheckout(ctx, page)
	return err
}

// startPayingUsagePlan starts buying the usage based plan on the MCP server's page: its dialog
// lists the plan's tiers and says usage is invoiced at the end of each period.
func (u *Steps) startPayingUsagePlan(ctx context.Context) error {
	page, err := u.openPaidPage(ctx, "")
	if err != nil {
		return err
	}
	planID, err := resolveText(ctx, keyPaidPlan)
	if err != nil {
		return err
	}
	if err := paidSubscribeButton(page, planID).Click(); err != nil {
		return fmt.Errorf("choosing the paid plan: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#paidSubscribePricing tr")).ToHaveCount(4); err != nil {
		return fmt.Errorf("the dialog should list the price, its two tiers and the rate limit: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#paidSubscribeTerms")).ToHaveText("Usage is invoiced at the end of each billing period."); err != nil {
		return err
	}
	if err := page.Locator("#paidSubscribeModal .paid-modal-close").First().Click(); err != nil {
		return err
	}
	if err := serveStripeJS(page, true); err != nil {
		return err
	}
	_, err = u.startCheckout(ctx, page)
	return err
}

// payForPlan pays for the open checkout in Stripe and completes Stripe's form: the portal makes the
// subscription, shows its token, and the plan's card then offers the subscription. On an API page
// the free plan can no longer be chosen beside the paid one.
func (u *Steps) payForPlan(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	checkout, err := resolveText(ctx, keyPaidCheckout)
	if err != nil {
		return err
	}
	path, err := resolveText(ctx, keyPaidPagePath)
	if err != nil {
		return err
	}
	planID, err := resolveText(ctx, keyPaidPlan)
	if err != nil {
		return err
	}
	planName, err := resolveText(ctx, keyPaidPlanName)
	if err != nil {
		return err
	}
	if err := u.stripeControl(ctx, "/test/checkout-sessions/"+url.PathEscape(checkout)+"/complete", nil); err != nil {
		return err
	}
	subscription, err := u.paidSubscription(ctx, page, checkout)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalPaidSubscription, ID: subscription, Actor: actor.Developer().Username}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyPaidSubscription, subscription); err != nil {
		return err
	}

	prefix := "api"
	if regexp.MustCompile(`^/mcp/`).MatchString(path) {
		prefix = "mcp"
	}
	if err := page.Locator("#stripeTestPay").Click(); err != nil {
		return fmt.Errorf("paying in Stripe's form: %w", err)
	}
	token := page.Locator("#" + prefix + "TokenModal")
	if err := u.expect.Locator(token).ToBeVisible(); err != nil {
		return fmt.Errorf("the new subscription's token should be shown: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#" + prefix + "TokenPlanName")).ToHaveText(planName); err != nil {
		return err
	}
	if err := page.Locator("#" + prefix + "TokenModalDone").Click(); err != nil {
		return err
	}
	view := page.Locator(`.` + prefix + `-view-sub-btn[data-policy-name="` + planID + `"]`)
	if err := u.expect.Locator(view).ToBeVisible(); err != nil {
		return fmt.Errorf("the paid plan's card should offer the subscription: %w", err)
	}
	if prefix == "mcp" {
		return nil
	}
	return u.expect.Locator(page.Locator("#subscriptionPlans .aov-plan-hint").First()).ToHaveText(paidSwitchHint)
}

// paidSubscription waits for the portal to make the subscription of the paid checkout, and
// returns it.
func (u *Steps) paidSubscription(ctx context.Context, page playwright.Page, checkout string) (string, error) {
	var subscription string
	err := retry.Await(ctx, retry.Options{Interval: time.Second}, func(context.Context) (string, error) {
		status, body, err := u.portalCall(page, http.MethodGet, "/checkout-sessions/"+url.PathEscape(checkout), nil)
		if err != nil {
			return "", retry.Transient(err)
		}
		var state struct {
			Status         string  `json:"status"`
			SubscriptionID *string `json:"subscriptionId"`
		}
		if status != http.StatusOK || json.Unmarshal([]byte(body), &state) != nil {
			return "", retry.Transient(fmt.Errorf("reading checkout %s: status %d %s", checkout, status, body))
		}
		if state.Status != "COMPLETED" || state.SubscriptionID == nil {
			return "", nil
		}
		subscription = *state.SubscriptionID
		return subscription, nil
	}, func(id string) bool { return id != "" }, "waiting for the paid checkout to make its subscription")
	return subscription, err
}

// subscriptionPeriodStart is when the testbench Stripe service started the current period of the
// checkout's subscription, on the testbench's clock.
func (u *Steps) subscriptionPeriodStart(ctx context.Context, checkout string) (time.Time, error) {
	response, err := u.testbenchCall(ctx, "stripe", http.MethodGet, "/test/state", nil)
	if err != nil {
		return time.Time{}, err
	}
	if !response.Succeeded() {
		return time.Time{}, fmt.Errorf("reading Stripe's state: %s", response.Describe())
	}
	var state struct {
		Subscriptions []struct {
			PeriodStart int64             `json:"current_period_start"`
			Metadata    map[string]string `json:"metadata"`
		} `json:"subscriptions"`
	}
	if err := json.Unmarshal(response.Body, &state); err != nil {
		return time.Time{}, fmt.Errorf("decoding Stripe's state: %w", err)
	}
	for _, sub := range state.Subscriptions {
		if sub.Metadata["wso2_checkout"] == checkout {
			return time.Unix(sub.PeriodStart, 0).UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("Stripe has no subscription for checkout %s", checkout)
}

// deregisterPaidSubscription forgets the paid subscription the scenario ended.
func deregisterPaidSubscription(ctx context.Context, subscription string) error {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalPaidSubscription, subscription)
	return nil
}

// pageThroughInvoices shows the developer's invoices: a flat rate plan has no usage to show, Stripe
// has invoiced the subscription every month for two years, all of them are read a page at a time,
// and the billing page opens once Stripe can make it.
func (u *Steps) pageThroughInvoices(ctx context.Context) error {
	checkout, err := resolveText(ctx, keyPaidCheckout)
	if err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/usage"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#usageEmpty")).ToBeVisible(); err != nil {
		return fmt.Errorf("a flat rate plan should have no usage to show: %w", err)
	}

	now := time.Now()
	for month := 1; month <= 24; month++ {
		status := "paid"
		if month == 1 {
			status = "open"
		}
		if err := u.stripeControl(ctx, "/test/invoices", map[string]any{
			"subscription": checkout, "status": status, "created": now.AddDate(0, -month, 0).Unix(),
		}); err != nil {
			return err
		}
	}
	if err := u.openAPIPortalPath(ctx, "/billing/invoices"); err != nil {
		return err
	}
	rows := page.Locator("#billingInvoices tr")
	if err := u.expect.Locator(rows.First()).ToBeVisible(); err != nil {
		return err
	}
	if _, err := page.Locator("#billingInvoicePeriod").SelectOption(playwright.SelectOptionValues{Values: &[]string{"ALL"}}); err != nil {
		return err
	}
	if err := u.expect.Locator(rows).ToHaveCount(20); err != nil {
		return fmt.Errorf("all time should show a first page of invoices: %w", err)
	}
	if err := page.Locator("#billingInvoicesMore").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(rows).ToHaveCount(25); err != nil {
		return fmt.Errorf("Load more should add the rest of the invoices: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#billingInvoicesMore")).ToBeHidden(); err != nil {
		return err
	}

	manage := page.Locator("#billingManage")
	if err := u.failProviderCall(ctx, "stripe", map[string]any{"method": "POST", "path": "/v1/billing_portal/sessions", "status": 500}); err != nil {
		return err
	}
	if err := manage.Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#billingManageError")).ToBeVisible(); err != nil {
		return fmt.Errorf("a billing page Stripe can't make should be reported: %w", err)
	}
	if err := page.Route(stripeBillingAddress, func(route playwright.Route) {
		_ = route.Fulfill(playwright.RouteFulfillOptions{Body: "<title>Billing</title>", ContentType: playwright.String("text/html")})
	}); err != nil {
		return err
	}
	if err := manage.Click(); err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`^https://billing\.stripe\.test/`))
}

// readRevenue reads Settings › Usage & Revenue: the paid subscription, and the invoices since
// billing was set up this month. The report opens on the current month; choosing it on the chart
// shows the whole period and choosing it again shows the month. A longer period still starts at
// that month, and a search that matches no subscriber, API or plan adds up nothing.
func (u *Steps) readRevenue(ctx context.Context) error {
	page, err := u.openSettingsPanel(ctx, "cfg-revenue")
	if err != nil {
		return err
	}
	if err := u.expect.Locator(revenueCard(page, "Paid subscriptions")).ToHaveText("1"); err != nil {
		return err
	}
	slots := page.Locator("#cfgRevenueChart .cfg-revenue-slot")
	if err := u.expect.Locator(slots).ToHaveCount(1); err != nil {
		return fmt.Errorf("the report should start at the month billing was set up: %w", err)
	}
	if err := u.expect.Locator(slots.First()).ToHaveAttribute("aria-pressed", "true"); err != nil {
		return fmt.Errorf("the report should open on the current month: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfgRevenueCards")).ToContainText("so far"); err != nil {
		return err
	}
	if err := slots.First().Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfgRevenueCards")).ToContainText("in the last 3 months"); err != nil {
		return err
	}
	if err := slots.First().Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(slots.First()).ToHaveAttribute("aria-pressed", "true"); err != nil {
		return err
	}
	if _, err := page.Locator("#cfgRevenueMonths").SelectOption(playwright.SelectOptionValues{Values: &[]string{"ALL"}}); err != nil {
		return err
	}
	if err := u.expect.Locator(slots).ToHaveCount(1); err != nil {
		return err
	}
	if err := fillAndCheck(page, "#cfgRevenueSearch", "no such subscriber"); err != nil {
		return err
	}
	return u.expect.Locator(revenueCard(page, "Invoiced")).ToContainText("0.00")
}

// revenueCard is the value of the revenue report's card with the label.
func revenueCard(page playwright.Page, label string) playwright.Locator {
	return page.Locator("#cfgRevenueCards .cfg-revenue-card", playwright.PageLocatorOptions{HasText: label}).
		Locator(".cfg-revenue-card-value")
}

// openPaidSubscriptionDialog opens a dialog that shows a paid subscription, and waits for its
// billing state.
func (u *Steps) openPaidSubscriptionDialog(page playwright.Page, open playwright.Locator) error {
	_, err := page.ExpectResponse(regexp.MustCompile(`/subscriptions/[^/]+/billing$`), func() error { return open.Click() })
	return err
}

// unsubscribeAndEndNow unsubscribes from the flat rate plan on the API's page, where it keeps
// running to the end of the paid month, then ends it at once from Subscriptions.
func (u *Steps) unsubscribeAndEndNow(ctx context.Context) error {
	planID, err := resolveText(ctx, keyPaidPlan)
	if err != nil {
		return err
	}
	subscription, err := resolveText(ctx, keyPaidSubscription)
	if err != nil {
		return err
	}
	page, err := u.openPaidPage(ctx, "")
	if err != nil {
		return err
	}
	if err := u.openPaidSubscriptionDialog(page, page.Locator(`.api-view-sub-btn[data-policy-name="`+planID+`"]`)); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#apiManageSuspendBtn")).ToBeHidden(); err != nil {
		return fmt.Errorf("a paid subscription should not offer to suspend it: %w", err)
	}
	if err := page.Locator("#apiManageUnsubBtn").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#apiUnsubDesc")).ToContainText("Your subscription will remain active until"); err != nil {
		return err
	}
	if err := page.Locator("#apiUnsubConfirmBtn").Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "You will retain access until"); err != nil {
		return err
	}

	if err := u.openAPIPortalPath(ctx, "/subscriptions"); err != nil {
		return err
	}
	row := page.Locator("#sub-row-" + subscription)
	if err := u.expect.Locator(row).ToHaveAttribute("data-paid", "true"); err != nil {
		return err
	}
	if err := u.openPaidSubscriptionDialog(page, row.Locator(".sub-action-btn--manage")); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#subBillingEnds")).ToContainText("You will retain access until"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#subManageUnsubBtn")).ToBeHidden(); err != nil {
		return err
	}
	endNow := page.Locator("#subManageEndNowBtn")
	if err := endNow.Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#subUnsubTitle")).ToHaveText("End your subscription now?"); err != nil {
		return err
	}
	if err := page.Locator("#subUnsubCancelBtn").Click(); err != nil {
		return err
	}
	if err := endNow.Click(); err != nil {
		return err
	}
	if err := page.Locator("#subUnsubConfirmBtn").Click(); err != nil {
		return err
	}
	if err := u.expectToast(page, "Subscription removed successfully!"); err != nil {
		return err
	}
	if err := u.expect.Locator(row).ToHaveCount(0); err != nil {
		return err
	}
	return deregisterPaidSubscription(ctx, subscription)
}

// followUsage shows the usage Moesif counted for the subscription on the Usage page, in plain bars
// as when Chart.js can't be loaded: the current billing period per day, per hour and per week, and
// the last 24 hours, which only hours can show. A subscription the page address names that is not
// the developer's shows theirs instead.
func (u *Steps) followUsage(ctx context.Context) error {
	checkout, err := resolveText(ctx, keyPaidCheckout)
	if err != nil {
		return err
	}
	name, err := resolveText(ctx, keyPaidArtifactName)
	if err != nil {
		return err
	}
	start, err := u.subscriptionPeriodStart(ctx, checkout)
	if err != nil {
		return err
	}
	for _, usage := range []map[string]any{
		{"subscription": checkout, "value": 250},
		{"subscription": checkout, "value": 40, "time": start.Add(-time.Hour)},
	} {
		response, err := u.testbenchCall(ctx, "moesif", http.MethodPost, "/test/usage", usage)
		if err != nil {
			return err
		}
		if !response.Succeeded() {
			return fmt.Errorf("counting usage in Moesif: %s", response.Describe())
		}
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Route(chartJSAddress, func(route playwright.Route) { _ = route.Abort() }); err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/usage?subscriptionId=not-a-subscription"); err != nil {
		return err
	}
	cards := page.Locator("#usageCards")
	if err := u.expect.Locator(cards).ToContainText("250 requests"); err != nil {
		return fmt.Errorf("the current billing period should count the usage since the purchase: %w", err)
	}
	if err := u.expect.Locator(cards).ToContainText("Estimated total"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#usageCharts h2").First()).ToContainText(name); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#usageCharts .usage-bar").First()).ToBeAttached(); err != nil {
		return err
	}
	for _, bin := range []string{"Hour", "Week", "Day"} {
		button := page.Locator("#usageCharts .usage-bin", playwright.PageLocatorOptions{HasText: bin})
		if err := button.Click(); err != nil {
			return fmt.Errorf("showing usage per %s: %w", bin, err)
		}
		if err := u.expect.Locator(button).ToHaveAttribute("aria-pressed", "true"); err != nil {
			return err
		}
	}
	if _, err := page.Locator("#usagePeriod").SelectOption(playwright.SelectOptionValues{Values: &[]string{"LAST_24_HOURS"}}); err != nil {
		return err
	}
	if err := u.expect.Locator(cards).ToContainText("290 requests"); err != nil {
		return fmt.Errorf("the last 24 hours should count the usage before the purchase too: %w", err)
	}
	day := page.Locator("#usageCharts .usage-bin", playwright.PageLocatorOptions{HasText: "Day"})
	return u.expect.Locator(day).ToBeDisabled()
}

// seePaymentPending has Stripe stop retrying the subscription's payment: Subscriptions and Usage
// say requests are blocked until it is paid. Paying it makes the subscription active again.
func (u *Steps) seePaymentPending(ctx context.Context) error {
	checkout, err := resolveText(ctx, keyPaidCheckout)
	if err != nil {
		return err
	}
	subscription, err := resolveText(ctx, keyPaidSubscription)
	if err != nil {
		return err
	}
	if err := u.stripeControl(ctx, "/test/subscriptions/"+url.PathEscape(checkout),
		map[string]any{"status": "past_due", "retriesEnded": true}); err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/subscriptions"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	row := page.Locator("#sub-row-" + subscription)
	if err := u.openPaidSubscriptionDialog(page, row.Locator(".sub-action-btn--manage")); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#subManageStatusBadge")).ToHaveText("Payment pending"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#subPaymentPendingNote")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#subManageClose").Click(); err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/usage"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#usagePaymentPending")).ToBeVisible(); err != nil {
		return err
	}
	return u.stripeControl(ctx, "/test/subscriptions/"+url.PathEscape(checkout), map[string]any{"status": "active"})
}

// unsubscribeFromUsagePlan unsubscribes from the usage based plan on the MCP server's page: it ends
// at once, with the usage so far on a final invoice, and the plan can be bought again.
func (u *Steps) unsubscribeFromUsagePlan(ctx context.Context) error {
	planID, err := resolveText(ctx, keyPaidPlan)
	if err != nil {
		return err
	}
	subscription, err := resolveText(ctx, keyPaidSubscription)
	if err != nil {
		return err
	}
	page, err := u.openPaidPage(ctx, "")
	if err != nil {
		return err
	}
	if err := u.openPaidSubscriptionDialog(page, page.Locator(`.mcp-view-sub-btn[data-policy-name="`+planID+`"]`)); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#mcpManageSuspendBtn")).ToBeHidden(); err != nil {
		return err
	}
	unsubscribe := page.Locator("#mcpManageUnsubBtn")
	if err := unsubscribe.Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#mcpUnsubDesc")).ToContainText("Usage up to now is billed on a final invoice."); err != nil {
		return err
	}
	if err := page.Locator("#mcpUnsubKeepBtn").Click(); err != nil {
		return err
	}
	if err := unsubscribe.Click(); err != nil {
		return err
	}
	if err := page.Locator("#mcpUnsubConfirmBtn").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(paidSubscribeButton(page, planID)).ToBeVisible(); err != nil {
		return fmt.Errorf("the plan should be offered again: %w", err)
	}
	return deregisterPaidSubscription(ctx, subscription)
}
