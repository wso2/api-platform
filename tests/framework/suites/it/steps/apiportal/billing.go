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
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// The keys billing is set up with. They are shaped as Stripe's test keys and Moesif's keys are,
// and the testbench's Stripe and Moesif services accept them.
var billingSetupValues = func() map[string]string {
	application := base64.RawURLEncoding.EncodeToString([]byte(`{"app":"524:75"}`))
	return map[string]string{
		"moesifCollectorApplicationId": application + ".collector",
		"moesifManagementKey":          "management." + application + ".signature",
		"stripeApiKey":                 "sk_test_apiportalbilling",
		"stripePublishableKey":         "pk_test_apiportalbilling",
	}
}()

var portalBillingKind = cleanup.Kind{Name: "api-portal-billing-configuration", Order: 22}

// paidSubscriptionKind is the cleanup kind of a role's paid subscriptions: a subscription is
// ended by the user who bought it.
func paidSubscriptionKind(role string) cleanup.Kind {
	return cleanup.Kind{Name: "api-portal-paid-subscription-" + role, Order: 19}
}

func (s *Steps) registerBillingSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I set up billing in the API Portal$`, s.setUpBilling)
	sc.Step(`^I set up billing in the API Portal with values:$`, s.setUpBillingWithValues)
	sc.Step(`^the checkout "([^"]*)" is paid in Stripe by "([^"]*)" and the subscription is stored as "([^"]*)"$`,
		s.payCheckout)
}

func (s *Steps) setUpBilling(ctx context.Context) error {
	return s.setUpBillingWithValues(ctx, nil)
}

// setUpBillingWithValues sets up billing as the administrator with the testbench keys, the
// portal's address inside the test network for Stripe's events, and the table's values in place
// of those; an empty value leaves the field out. Billing that is set up is removed when the
// scenario ends.
func (s *Steps) setUpBillingWithValues(ctx context.Context, table *godog.Table) error {
	inst, err := s.topo.Component(portalService)
	if err != nil {
		return err
	}
	webhookBase, err := inst.InternalURL("https")
	if err != nil {
		return err
	}
	values := map[string]string{"webhookBaseUrl": webhookBase}
	for key, value := range billingSetupValues {
		values[key] = value
	}
	if table != nil {
		for i, row := range table.Rows {
			if len(row.Cells) != 2 {
				return fmt.Errorf("billing setup values row %d must have exactly two cells", i+1)
			}
			key, err := stepscommon.Expand(ctx, strings.TrimSpace(row.Cells[0].Value))
			if err != nil {
				return err
			}
			value, err := stepscommon.Expand(ctx, row.Cells[1].Value)
			if err != nil {
				return err
			}
			if key == "" {
				return fmt.Errorf("billing setup values row %d has an empty key", i+1)
			}
			values[key] = value
		}
	}
	body := map[string]string{}
	for key, value := range values {
		if value != "" {
			body[key] = value
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding the billing setup: %w", err)
	}
	if err := s.sendPortal(ctx, http.MethodPost, "/billing-configuration", "admin", payload); err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusCreated {
		return nil
	}
	return s.registerPortalResource(ctx, portalBillingKind, orgHandle, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, target: func(string) string { return "/billing-configuration" }})
}

// stripeControlURL is the address of one of the testbench Stripe service's /test endpoints for
// this block.
func (s *Steps) stripeControlURL(path string) (string, error) {
	base, err := s.topo.URL("testbench", "stripe")
	if err != nil {
		return "", err
	}
	if s.topo.Block == nil {
		return "", fmt.Errorf("the Stripe service is partitioned by block, but the topology has no block")
	}
	return base + "/" + s.topo.Block.PartitionKey() + path, nil
}

// payCheckout pays an open checkout as its customer would, through the testbench Stripe service,
// and waits for the portal to create the subscription the checkout buys. Every event Stripe sends
// for the payment must be accepted. The subscription is stored, and ended at once when the
// scenario ends.
func (s *Steps) payCheckout(ctx context.Context, checkoutRef, role, storeAs string) error {
	checkout, err := stepscommon.Expand(ctx, checkoutRef)
	if err != nil {
		return err
	}
	complete, err := s.stripeControlURL("/test/checkout-sessions/" + url.PathEscape(checkout) + "/complete")
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodPost, URL: complete}, 0, 0)
	if err != nil {
		return fmt.Errorf("paying checkout %q in Stripe: %w", checkout, err)
	}
	if !response.Succeeded() {
		return fmt.Errorf("paying checkout %q in Stripe failed: %s", checkout, response.Describe())
	}
	var paid struct {
		Deliveries []struct {
			URL    string `json:"url"`
			Status int    `json:"status"`
			Error  string `json:"error"`
		} `json:"deliveries"`
	}
	if err := json.Unmarshal(response.Body, &paid); err != nil {
		return fmt.Errorf("decoding Stripe's payment of checkout %q: %w", checkout, err)
	}
	for _, delivery := range paid.Deliveries {
		if delivery.Status < 200 || delivery.Status >= 300 {
			return fmt.Errorf("Stripe's event for checkout %q was not accepted by %s: status %d %s",
				checkout, delivery.URL, delivery.Status, delivery.Error)
		}
	}

	path := "/checkout-sessions/" + url.PathEscape(checkout)
	subscription, err := retry.Until(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (string, error) {
			if err := s.sendPortal(ctx, http.MethodGet, path, role, nil); err != nil {
				return "", retry.Transient(err)
			}
			read, err := httpx.Published(ctx)
			if err != nil {
				return "", err
			}
			var state struct {
				Status         string `json:"status"`
				SubscriptionID string `json:"subscriptionId"`
			}
			if !read.Succeeded() || json.Unmarshal(read.Body, &state) != nil {
				return "", retry.Transient(fmt.Errorf("reading checkout %q: %s", checkout, read.Describe()))
			}
			switch state.Status {
			case "COMPLETED":
			case "EXPIRED":
				return "", fmt.Errorf("checkout %q expired instead of creating a subscription", checkout)
			default:
				return "", nil
			}
			return state.SubscriptionID, nil
		}, func(id string) bool { return id != "" })
	if err != nil {
		return fmt.Errorf("waiting for paid checkout %q to create its subscription: %w", checkout, err)
	}
	if err := s.registerPortalResource(ctx, paidSubscriptionKind(role), subscription, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, role: role, target: func(id string) string {
			return "/subscriptions/" + url.PathEscape(id) + "?immediate=true"
		}}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, subscription)
}
