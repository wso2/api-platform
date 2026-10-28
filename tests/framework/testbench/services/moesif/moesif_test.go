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

package moesif

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/testbench/services/stripe"
)

const (
	testBlock     = "block-a"
	testStripeKey = "sk_test_abc123"
)

var managementKey = "head." + base64.RawURLEncoding.EncodeToString([]byte(`{"app":"524:75"}`)) + ".sig"

type harness struct {
	t       *testing.T
	service *Service
	moesif  *httptest.Server
	stripe  *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	billing := stripe.New()
	service, err := New(billing)
	require.NoError(t, err)
	h := &harness{t: t, service: service, moesif: httptest.NewServer(service.Handler()),
		stripe: httptest.NewServer(billing.Handler())}
	t.Cleanup(h.moesif.Close)
	t.Cleanup(h.stripe.Close)
	return h
}

func send(t *testing.T, req *http.Request) (int, any) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body any
	if json.Unmarshal(raw, &body) != nil {
		body = string(raw)
	}
	return resp.StatusCode, body
}

// call sends a Moesif API call with a key and a JSON body.
func (h *harness) call(key, method, path, body string) (int, any) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.moesif.URL+"/"+testBlock+path, strings.NewReader(body))
	require.NoError(h.t, err)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Content-Type", "application/json")
	return send(h.t, req)
}

func (h *harness) api(method, path, body string) (int, map[string]any) {
	h.t.Helper()
	status, decoded := h.call(managementKey, method, path, body)
	object, _ := decoded.(map[string]any)
	return status, object
}

func (h *harness) control(method, path, body string) (int, map[string]any) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.moesif.URL+"/"+testBlock+path, strings.NewReader(body))
	require.NoError(h.t, err)
	status, decoded := send(h.t, req)
	object, _ := decoded.(map[string]any)
	return status, object
}

func (h *harness) stripeCall(method, path, form string) (int, map[string]any) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.stripe.URL+"/"+testBlock+path, strings.NewReader(form))
	require.NoError(h.t, err)
	req.Header.Set("Authorization", "Bearer "+testStripeKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status, decoded := send(h.t, req)
	object, _ := decoded.(map[string]any)
	return status, object
}

func (h *harness) state() map[string]any {
	h.t.Helper()
	status, body := h.control(http.MethodGet, "/test/state", "")
	require.Equal(h.t, http.StatusOK, status)
	return body
}

func (h *harness) stripePrice(id string) map[string]any {
	h.t.Helper()
	_, state := h.stripeCall(http.MethodGet, "/test/state", "")
	for _, p := range state["prices"].([]any) {
		if p.(map[string]any)["id"] == id {
			return p.(map[string]any)
		}
	}
	return nil
}

func partnerAccountBody(state, key string) string {
	return fmt.Sprintf(`{"partner_account":{"version":"v2","api_version":"2020-08-27","credentials":{"state":%q,"api_key":%q}},`+
		`"billing_config":{"stripe":{"company_id_field":"id"}}}`, state, key)
}

func (h *harness) connect() {
	h.t.Helper()
	status, body := h.api(http.MethodPost, "/v1/~/billing/partneraccount/stripe", partnerAccountBody("enabled", testStripeKey))
	require.Equal(h.t, http.StatusOK, status, body)
}

func (h *harness) createPlan(name string) string {
	h.t.Helper()
	status, body := h.api(http.MethodPost, "/v1/~/billing/catalog/plans?provider=stripe",
		fmt.Sprintf(`{"name":%q,"provider":"stripe","status":"active","billing_type":"continuous_reporting","reporting_period":"15m"}`, name))
	require.Equal(h.t, http.StatusCreated, status, body)
	return body["id"].(string)
}

// priceBody is a catalog price as the API Portal sends it, with the given pricing fields.
func priceBody(planID, name, pricing string) string {
	return fmt.Sprintf(`{"name":%q,"plan_id":%q,"status":"active","provider":"stripe","currency":"usd","period":1,`+
		`"period_units":"M","usage_aggregator":"sum",%s}`, name, planID, pricing)
}

func (h *harness) createPrice(body string) string {
	h.t.Helper()
	status, created := h.api(http.MethodPost, "/v1/~/billing/catalog/prices?provider=stripe", body)
	require.Equal(h.t, http.StatusCreated, status, created)
	return created["id"].(string)
}

// filterClause is an extra clause a price's filters add to a count.
const filterClause = `{"terms":{"metadata.apiName.raw":["orders"]}}`

func countQuery(customer, subscription, extra string) string {
	clauses := fmt.Sprintf(`{"terms":{"metadata.billingCustomerId.raw":[%q]}},{"terms":{"metadata.billingSubscriptionId.raw":[%q]}}`,
		customer, subscription)
	if extra != "" {
		clauses += "," + extra
	}
	return `{"bool":{"must":[` + clauses + `]}}`
}

func meterBody(name, planID, priceID, extra string) string {
	return fmt.Sprintf(`{"name":%q,"slug":"hourly_usage","url_query":"?seg","status":"active","zero_balance_behavior":"bill_customer",`+
		`"es_query":{"query":%s,"aggs":{"seg":{"filter":{"match_all":{}},"aggs":{"usage_value":{"sum":{"field":"weight","missing":1}}}}},"size":0},`+
		`"billing_plan":{"provider_slug":"stripe","params":{"usage_multiplier":"1.0","stripe_params":{"product":{"id":%q,"name":"Gold","usage_type":"metered"},`+
		`"prices":[{"price_id":%q}],"reporting":{"reporting_period":"15m"}}}}}`,
		name, countQuery("{{company_id}}", "{{subscription_id}}", extra), planID, priceID)
}

func searchBody(customer, subscription, extra, interval string) string {
	return fmt.Sprintf(`{"query":%s,"aggs":{"series":{"date_histogram":{"field":"request.time","calendar_interval":%q,`+
		`"time_zone":"UTC","min_doc_count":0},"aggs":{"usage":{"sum":{"field":"weight","missing":1}}}}},"size":0}`,
		countQuery(customer, subscription, extra), interval)
}

// usagePlan creates a plan with a per-unit usage price and its meter; it returns their ids.
func (h *harness) usagePlan(extra string) (planID, priceID, meterID string) {
	h.t.Helper()
	planID = h.createPlan("Gold calls")
	priceID = h.createPrice(priceBody(planID, "Gold calls", `"pricing_model":"per_unit","price_in_decimal":"0.01"`))
	status, meter := h.api(http.MethodPost, "/v1/~/billing/meters", meterBody("Gold calls", planID, priceID, extra))
	require.Equal(h.t, http.StatusCreated, status, meter)
	return planID, priceID, meter["_id"].(string)
}

// subscribe pays a Stripe checkout for a price; it returns the customer and subscription ids.
func (h *harness) subscribe(priceID string) (string, string) {
	h.t.Helper()
	_, customer := h.stripeCall(http.MethodPost, "/v1/customers", "name=Ann")
	_, sess := h.stripeCall(http.MethodPost, "/v1/checkout/sessions",
		"mode=subscription&customer="+customer["id"].(string)+"&line_items[0][price]="+priceID)
	req, err := http.NewRequest(http.MethodPost, h.stripe.URL+"/"+testBlock+"/test/checkout-sessions/"+sess["id"].(string)+"/complete", nil)
	require.NoError(h.t, err)
	status, done := send(h.t, req)
	require.Equal(h.t, http.StatusOK, status, done)
	return customer["id"].(string), done.(map[string]any)["subscription"].(string)
}

func (h *harness) search(body string, from, to time.Time) (int, []any) {
	h.t.Helper()
	query := url.Values{"from": {from.UTC().Format("2006-01-02T15:04:05.000Z")}, "to": {to.UTC().Format(time.RFC3339)}}
	status, result := h.api(http.MethodPost, "/v1/search/~/search/events?"+query.Encode(), body)
	if status != http.StatusOK {
		return status, nil
	}
	return status, result["aggregations"].(map[string]any)["series"].(map[string]any)["buckets"].([]any)
}

// switchingBilling passes calls to a Stripe account until it is set to refuse them.
type switchingBilling struct {
	*stripe.Service
	mu     sync.Mutex
	refuse bool
}

func (b *switchingBilling) refusing() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.refuse
}

func (b *switchingBilling) AddPrice(key string, spec stripe.PriceSpec) error {
	if b.refusing() {
		return errors.New("refused")
	}
	return b.Service.AddPrice(key, spec)
}

func (b *switchingBilling) ArchivePrice(key, priceID string) error {
	if b.refusing() {
		return errors.New("refused")
	}
	return b.Service.ArchivePrice(key, priceID)
}

func TestServiceDescribesItselfAndNeedsABillingAccount(t *testing.T) {
	_, err := New(nil)
	require.Error(t, err)
	service, err := New(stripe.New())
	require.NoError(t, err)
	require.Equal(t, "moesif", service.Name())
	require.Equal(t, Port, service.Port())
	require.True(t, service.Stateful())
	require.NotEmpty(t, service.PartitionKey())
}

func TestAPICallsNeedAManagementKey(t *testing.T) {
	h := newHarness(t)
	noApp := "head." + base64.RawURLEncoding.EncodeToString([]byte(`{"org":"1"}`)) + ".sig"
	for _, key := range []string{"", "a.b", "a.%%%.c", "a." + base64.RawURLEncoding.EncodeToString([]byte("[]")) + ".c", noApp} {
		status, body := h.call(key, http.MethodGet, "/v1/~/billing/catalog/plans?provider=stripe", "")
		require.Equal(t, http.StatusUnauthorized, status, key)
		require.Equal(t, map[string]any{"message": "Unauthorized"}, body)
	}
	status, _ := h.api(http.MethodGet, "/v1/~/billing/catalog/plans?provider=stripe", "")
	require.Equal(t, http.StatusOK, status)
	status, _ = h.api(http.MethodPost, "/v1/~/billing/catalog/plans?provider=stripe", "[1]")
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodGet, "/v1/~/billing/catalog/plans", "")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestTheStripeConnectionIsSavedReadAndRemoved(t *testing.T) {
	h := newHarness(t)
	path := "/v1/~/billing/partneraccount/stripe"
	status, _ := h.api(http.MethodGet, path, "")
	require.Equal(t, http.StatusNotFound, status)

	for _, body := range []string{
		partnerAccountBody("enabled", "sk_live_abc"),
		partnerAccountBody("enabled", "pk_test_abc"),
		partnerAccountBody("paused", testStripeKey),
		`{"partner_account":{"credentials":{"state":"disabled"}}}`,
	} {
		status, _ = h.api(http.MethodPost, path, body)
		require.Equal(t, http.StatusBadRequest, status, body)
	}
	status, _ = h.api(http.MethodPost, "/v1/~/billing/partneraccount/chargebee", partnerAccountBody("enabled", testStripeKey))
	require.Equal(t, http.StatusBadRequest, status)

	h.connect()
	status, account := h.api(http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, status)
	credentials := account["partner_account"].(map[string]any)["credentials"].(map[string]any)
	require.Equal(t, map[string]any{"state": "enabled"}, credentials)

	status, _ = h.api(http.MethodDelete, "/v1/alerting/~/partners/stripe", "")
	require.Equal(t, http.StatusOK, status)
	status, gone := h.api(http.MethodGet, path, "")
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "Unable to get the Partner account", gone["message"])
	status, _ = h.api(http.MethodDelete, "/v1/alerting/~/partners/stripe", "")
	require.Equal(t, http.StatusNotFound, status)

	status, _ = h.api(http.MethodPost, path, partnerAccountBody("disabled", ""))
	require.Equal(t, http.StatusOK, status)
	status, refused := h.api(http.MethodPost, "/v1/~/billing/catalog/plans?provider=stripe",
		`{"name":"Gold","provider":"stripe","status":"active","billing_type":"continuous_reporting","reporting_period":"15m"}`)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, refused["message"], "connected")
	require.Equal(t, "disabled", h.state()["partnerAccounts"].(map[string]any)["stripe"].(map[string]any)["partner_account"].(map[string]any)["credentials"].(map[string]any)["state"])
}

func TestCatalogPlansAreCreatedListedAndArchived(t *testing.T) {
	h := newHarness(t)
	h.connect()
	for _, body := range []string{
		`{"provider":"stripe","status":"active","billing_type":"x","reporting_period":"15m"}`,
		`{"name":"Gold","provider":"chargebee","status":"active","billing_type":"x","reporting_period":"15m"}`,
		`{"name":"Gold","provider":"stripe","status":"active"}`,
	} {
		status, _ := h.api(http.MethodPost, "/v1/~/billing/catalog/plans?provider=stripe", body)
		require.Equal(t, http.StatusBadRequest, status, body)
	}
	planID := h.createPlan("Gold")
	require.True(t, strings.HasPrefix(planID, "prod_"))
	status, listed := h.call(managementKey, http.MethodGet, "/v1/~/billing/catalog/plans?provider=stripe", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, listed, 1)

	archive := `{"provider":"stripe","status":"archived"}`
	status, _ = h.api(http.MethodPut, "/v1/~/billing/catalog/plans/"+planID+"?provider=stripe", `{"provider":"stripe","status":"active"}`)
	require.Equal(t, http.StatusBadRequest, status)
	status, missing := h.api(http.MethodPut, "/v1/~/billing/catalog/plans/prod_missing?provider=stripe", archive)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, missing["message"], "resource_missing")
	status, archived := h.api(http.MethodPut, "/v1/~/billing/catalog/plans/"+planID+"?provider=stripe", archive)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "archived", archived["status"])
	status, _ = h.api(http.MethodPost, "/v1/~/billing/catalog/prices?provider=stripe",
		priceBody(planID, "Gold fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	require.Equal(t, http.StatusBadRequest, status)
}

func TestCatalogPricesAreCreatedInStripe(t *testing.T) {
	h := newHarness(t)
	h.connect()
	planID := h.createPlan("Gold")

	fee := h.createPrice(priceBody(planID, "Gold fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	price := h.stripePrice(fee)
	require.Equal(t, float64(900), price["unit_amount"])
	require.Equal(t, "licensed", price["recurring"].(map[string]any)["usage_type"])
	require.Equal(t, planID, price["product"])
	require.Equal(t, "Gold fee", price["nickname"])

	perUnit := h.createPrice(priceBody(planID, "Calls",
		`"pricing_model":"per_unit","price_in_decimal":"0.008","transform_quantity":{"divide_by":1000,"round":"up"}`))
	price = h.stripePrice(perUnit)
	require.Equal(t, "0.8", price["unit_amount_decimal"])
	require.Equal(t, "metered", price["recurring"].(map[string]any)["usage_type"])
	require.Equal(t, float64(1000), price["transform_quantity"].(map[string]any)["divide_by"])

	volume := h.createPrice(priceBody(planID, "Volume", `"pricing_model":"volume","tiers":[`+
		`{"up_to":100,"unit_price_in_decimal":"0.05"},{"up_to":"inf","unit_price_in_decimal":"0.01","flat_price_in_decimal":"1"}]`))
	price = h.stripePrice(volume)
	require.Equal(t, "volume", price["tiers_mode"])
	require.Len(t, price["tiers"], 2)

	graduated := h.createPrice(priceBody(planID, "Tiered", `"pricing_model":"tiered","tiers":[{"up_to":"inf","flat_price_in_decimal":"2"}]`))
	require.Equal(t, "graduated", h.stripePrice(graduated)["tiers_mode"])

	yen := h.createPrice(strings.Replace(strings.Replace(priceBody(planID, "Yen", `"pricing_model":"flat","price_in_decimal":"500"`),
		`"usd"`, `"jpy"`, 1), `"M"`, `"y"`, 1))
	price = h.stripePrice(yen)
	require.Equal(t, float64(500), price["unit_amount"])
	require.Equal(t, "year", price["recurring"].(map[string]any)["interval"])
	dinar := h.createPrice(strings.Replace(priceBody(planID, "Dinar", `"pricing_model":"flat","price_in_decimal":"1.234"`), `"usd"`, `"kwd"`, 1))
	require.Equal(t, float64(1234), h.stripePrice(dinar)["unit_amount"])

	status, listed := h.call(managementKey, http.MethodGet, "/v1/~/billing/meters", "")
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, listed)
	require.Len(t, h.state()["prices"], 6)
}

func TestCatalogPricesAreValidated(t *testing.T) {
	h := newHarness(t)
	status, _ := h.api(http.MethodPost, "/v1/~/billing/catalog/prices?provider=stripe",
		priceBody("prod_x", "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	require.Equal(t, http.StatusBadRequest, status)
	h.connect()
	planID := h.createPlan("Gold")
	valid := priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`)
	invalid := []string{
		priceBody(planID, "", `"pricing_model":"flat","price_in_decimal":"9.00"`),
		strings.Replace(valid, `"status":"active"`, `"status":"archived"`, 1),
		strings.Replace(valid, `"usd"`, `"USD"`, 1),
		strings.Replace(valid, `"period":1`, `"period":2`, 1),
		strings.Replace(valid, `"usage_aggregator":"sum"`, `"usage_aggregator":"max"`, 1),
		strings.Replace(valid, `"M"`, `"Y"`, 1),
		priceBody(planID, "Fee", `"pricing_model":"package"`),
		priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"-1"`),
		priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"9.00","transform_quantity":{"divide_by":10,"round":"up"}`),
		priceBody(planID, "Fee", `"pricing_model":"per_unit","price_in_decimal":"1","transform_quantity":{"divide_by":1,"round":"up"}`),
		priceBody(planID, "Fee", `"pricing_model":"per_unit","price_in_decimal":"1","transform_quantity":{"divide_by":2.5,"round":"up"}`),
		priceBody(planID, "Fee", `"pricing_model":"volume"`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"up_to":"inf","unit_price_in_decimal":"1"},{"up_to":"inf","unit_price_in_decimal":"1"}]`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"up_to":5,"unit_price_in_decimal":"1"}]`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"up_to":1.5,"unit_price_in_decimal":"1"},{"up_to":"inf","unit_price_in_decimal":"1"}]`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"unit_price_in_decimal":"1"}]`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"up_to":"inf"}]`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"up_to":"inf","unit_price_in_decimal":"x"}]`),
		priceBody(planID, "Fee", `"pricing_model":"volume","tiers":[{"up_to":"inf","flat_price_in_decimal":"1e3"}]`),
		priceBody("prod_missing", "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`),
	}
	for _, body := range invalid {
		status, _ := h.api(http.MethodPost, "/v1/~/billing/catalog/prices?provider=stripe", body)
		require.Equal(t, http.StatusBadRequest, status, body)
	}
	require.Empty(t, h.state()["prices"])
}

func TestStripeRefusalsFailCatalogCalls(t *testing.T) {
	billing := &switchingBilling{Service: stripe.New()}
	service, err := New(billing)
	require.NoError(t, err)
	h := &harness{t: t, service: service, moesif: httptest.NewServer(service.Handler()), stripe: httptest.NewServer(billing.Handler())}
	t.Cleanup(h.moesif.Close)
	t.Cleanup(h.stripe.Close)
	h.connect()
	planID := h.createPlan("Gold")
	priceID := h.createPrice(priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))

	billing.mu.Lock()
	billing.refuse = true
	billing.mu.Unlock()
	status, _ := h.api(http.MethodPost, "/v1/~/billing/catalog/prices?provider=stripe",
		priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodPut, "/v1/~/billing/catalog/prices/"+priceID+"?provider=stripe", `{"provider":"stripe","status":"archived"}`)
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodDelete, "/v1/~/billing/catalog/prices/"+priceID+"?provider=stripe", "")
	require.Equal(t, http.StatusBadRequest, status)
	require.Len(t, h.state()["prices"], 1)
}

func TestCatalogPricesAreArchivedAndDeletedInStripeToo(t *testing.T) {
	h := newHarness(t)
	h.connect()
	planID := h.createPlan("Gold")
	archived := h.createPrice(priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	deleted := h.createPrice(priceBody(planID, "Fee 2", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	archive := `{"provider":"stripe","status":"archived"}`

	status, body := h.api(http.MethodPut, "/v1/~/billing/catalog/prices/"+archived+"?provider=stripe", archive)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "archived", body["status"])
	require.Equal(t, false, h.stripePrice(archived)["active"])
	status, missing := h.api(http.MethodPut, "/v1/~/billing/catalog/prices/price_missing?provider=stripe", archive)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, missing["message"], "resource_missing")
	status, _ = h.api(http.MethodPut, "/v1/~/billing/catalog/prices/"+archived+"?provider=stripe", `{"status":"archived"}`)
	require.Equal(t, http.StatusBadRequest, status)

	status, _ = h.call(managementKey, http.MethodDelete, "/v1/~/billing/catalog/prices/"+deleted+"?provider=stripe", "")
	require.Equal(t, http.StatusNoContent, status)
	require.Equal(t, false, h.stripePrice(deleted)["active"])
	status, _ = h.api(http.MethodDelete, "/v1/~/billing/catalog/prices/"+deleted+"?provider=stripe", "")
	require.Equal(t, http.StatusNotFound, status)
	status, _ = h.api(http.MethodDelete, "/v1/~/billing/catalog/prices/"+deleted, "")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestBillingMetersAreValidatedCreatedAndDeleted(t *testing.T) {
	h := newHarness(t)
	h.connect()
	planID := h.createPlan("Gold calls")
	priceID := h.createPrice(priceBody(planID, "Calls", `"pricing_model":"per_unit","price_in_decimal":"0.01"`))
	fee := h.createPrice(priceBody(planID, "Fee", `"pricing_model":"flat","price_in_decimal":"9.00"`))
	otherPlan := h.createPlan("Other")
	valid := meterBody("Calls", planID, priceID, "")

	invalid := []string{
		strings.Replace(valid, `"name":"Calls"`, `"name":""`, 1),
		strings.Replace(valid, `"hourly_usage"`, `"daily"`, 1),
		strings.Replace(valid, `"url_query":"?seg",`, ``, 1),
		strings.Replace(valid, `{{company_id}}`, `cus_1`, 1),
		strings.Replace(valid, `"field":"weight",`, ``, 1),
		strings.Replace(valid, `"provider_slug":"stripe"`, `"provider_slug":"chargebee"`, 1),
		strings.Replace(valid, `"usage_type":"metered"`, `"usage_type":"licensed"`, 1),
		meterBody("Calls", planID, fee, ""),
		meterBody("Calls", otherPlan, priceID, ""),
		meterBody("Calls", planID, "price_missing", ""),
		meterBody("Calls", planID, priceID, `{"terms":{"metadata.billingCustomerId.raw":["x"]}}`),
		meterBody("Calls", planID, priceID, `{"terms":{"metadata.billingSubscriptionId.raw":[]}}`),
		meterBody("Calls", planID, priceID, `{"terms":{"metadata.billingSubscriptionId.raw":[1]}}`),
		`{"name":"Calls","slug":"hourly_usage","status":"active","url_query":"?","es_query":{"query":{"bool":{"must":[]}}}}`,
	}
	for _, body := range invalid {
		status, _ := h.api(http.MethodPost, "/v1/~/billing/meters", body)
		require.Equal(t, http.StatusBadRequest, status, body)
	}

	status, created := h.api(http.MethodPost, "/v1/~/billing/meters", valid)
	require.Equal(t, http.StatusCreated, status, created)
	require.NotEmpty(t, created["_id"])
	require.NotContains(t, created, "id")
	status, listed := h.call(managementKey, http.MethodGet, "/v1/~/billing/meters", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, listed, 1)

	status, _ = h.call(managementKey, http.MethodDelete, "/v1/~/billing/meters/"+created["_id"].(string), "")
	require.Equal(t, http.StatusNoContent, status)
	status, _ = h.api(http.MethodDelete, "/v1/~/billing/meters/"+created["_id"].(string), "")
	require.Equal(t, http.StatusNotFound, status)
}

func TestCountedUsageIsReportedToStripeAndFoundByTheMatchingSearch(t *testing.T) {
	h := newHarness(t)
	h.connect()
	_, priceID, meterID := h.usagePlan(filterClause)
	customerID, subscriptionID := h.subscribe(priceID)
	day := time.Date(2026, 10, 20, 9, 30, 0, 0, time.UTC)
	for _, record := range []struct {
		at    time.Time
		value int64
	}{{day, 40}, {day.Add(2 * time.Hour), 5}, {day.AddDate(0, 0, 2), 7}} {
		status, body := h.control(http.MethodPost, "/test/usage", fmt.Sprintf(
			`{"subscription":%q,"customer":%q,"meter":"Gold calls","value":%d,"time":%q}`,
			subscriptionID, customerID, record.value, record.at.Format(time.RFC3339)))
		require.Equal(t, http.StatusCreated, status, body)
		require.Equal(t, meterID, body["meter"])
		require.Equal(t, priceID, body["price"])
	}
	_, stripeState := h.stripeCall(http.MethodGet, "/test/state", "")
	usage := stripeState["subscriptions"].([]any)[0].(map[string]any)["usage"].(map[string]any)
	require.Equal(t, float64(52), usage[priceID])

	from, to := day.AddDate(0, 0, -1), day.AddDate(0, 0, 5)
	status, buckets := h.search(searchBody(customerID, subscriptionID, filterClause, "1d"), from, to)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, buckets, 3)
	first := buckets[0].(map[string]any)
	require.Equal(t, float64(day.Truncate(24*time.Hour).UnixMilli()), first["key"])
	require.Equal(t, float64(2), first["doc_count"])
	require.Equal(t, float64(45), first["usage"].(map[string]any)["value"])
	require.Equal(t, float64(0), buckets[1].(map[string]any)["doc_count"])
	require.Equal(t, float64(7), buckets[2].(map[string]any)["usage"].(map[string]any)["value"])

	_, hours := h.search(searchBody(customerID, subscriptionID, filterClause, "1h"), from, to)
	require.Len(t, hours, 49)
	_, weeks := h.search(searchBody(customerID, subscriptionID, filterClause, "1w"), from, to)
	require.Len(t, weeks, 1)
	require.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), weeks[0].(map[string]any)["key_as_string"])

	for name, body := range map[string]string{
		"another filter":       searchBody(customerID, subscriptionID, "", "1d"),
		"another subscription": searchBody(customerID, "sub_other", filterClause, "1d"),
		"another customer":     searchBody("cus_other", subscriptionID, filterClause, "1d"),
	} {
		status, none := h.search(body, from, to)
		require.Equal(t, http.StatusOK, status, name)
		require.Empty(t, none, name)
	}
	_, outside := h.search(searchBody(customerID, subscriptionID, filterClause, "1d"), day.AddDate(0, 0, 3), to)
	require.Empty(t, outside)
}

func TestUsageWithoutACustomerMatchesAnyCustomerAndTheOnlyMeterIsTheDefault(t *testing.T) {
	h := newHarness(t)
	h.connect()
	_, priceID, _ := h.usagePlan("")
	customerID, subscriptionID := h.subscribe(priceID)
	status, body := h.control(http.MethodPost, "/test/usage", fmt.Sprintf(`{"subscription":%q,"value":3}`, subscriptionID))
	require.Equal(t, http.StatusCreated, status, body)
	_, buckets := h.search(searchBody(customerID, subscriptionID, "", "1d"), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.Len(t, buckets, 1)
	require.Len(t, h.state()["usage"], 1)
}

func TestUsageNeedsOneMeterAndALiveSubscription(t *testing.T) {
	h := newHarness(t)
	h.connect()
	status, _ := h.control(http.MethodPost, "/test/usage", `{"subscription":"sub_1","value":1}`)
	require.Equal(t, http.StatusNotFound, status)
	_, priceID, meterID := h.usagePlan("")
	planID := h.createPlan("Second")
	second := h.createPrice(priceBody(planID, "Second", `"pricing_model":"per_unit","price_in_decimal":"0.01"`))
	status, _ = h.api(http.MethodPost, "/v1/~/billing/meters", meterBody("Gold calls", planID, second, ""))
	require.Equal(t, http.StatusCreated, status)
	_, subscriptionID := h.subscribe(priceID)

	for body, want := range map[string]int{
		fmt.Sprintf(`{"subscription":%q,"value":1}`, subscriptionID):                      http.StatusNotFound,
		fmt.Sprintf(`{"subscription":%q,"meter":"Gold calls","value":1}`, subscriptionID): http.StatusNotFound,
		fmt.Sprintf(`{"subscription":%q,"meter":"missing","value":1}`, subscriptionID):    http.StatusNotFound,
		fmt.Sprintf(`{"subscription":"sub_missing","meter":%q,"value":1}`, meterID):       http.StatusConflict,
		fmt.Sprintf(`{"subscription":%q,"meter":%q,"value":-1}`, subscriptionID, meterID): http.StatusBadRequest,
		fmt.Sprintf(`{"meter":%q,"value":1}`, meterID):                                    http.StatusBadRequest,
		fmt.Sprintf(`{"subscription":%q,"meter":%q,"count":1}`, subscriptionID, meterID):  http.StatusBadRequest,
		``: http.StatusBadRequest,
	} {
		status, _ = h.control(http.MethodPost, "/test/usage", body)
		require.Equal(t, want, status, body)
	}
	status, _ = h.control(http.MethodPost, "/test/usage", fmt.Sprintf(`{"subscription":%q,"meter":%q,"value":1}`, subscriptionID, meterID))
	require.Equal(t, http.StatusCreated, status)
}

func TestSearchesAreValidated(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	valid := searchBody("cus_1", "sub_1", "", "1d")
	status, _ := h.search(valid, now, now.Add(-time.Hour))
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodPost, "/v1/search/~/search/events?from=yesterday&to=today", valid)
	require.Equal(t, http.StatusBadRequest, status)
	for _, body := range []string{
		strings.Replace(valid, `"1d"`, `"1M"`, 1),
		strings.Replace(valid, `"request.time"`, `"response.time"`, 1),
		strings.Replace(valid, `"UTC"`, `"Mars/Base"`, 1),
		strings.Replace(valid, `"field":"weight",`, ``, 1),
		searchBody("", "sub_1", "", "1d"),
		strings.Replace(valid, `["sub_1"]`, `["sub_1","sub_2"]`, 1),
		strings.Replace(valid, `{"terms":{"metadata.billingSubscriptionId.raw":["sub_1"]}}`,
			`{"terms":{"metadata.billingCustomerId.raw":["cus_2"]}}`, 1),
	} {
		status, _ = h.search(body, now.Add(-time.Hour), now)
		require.Equal(t, http.StatusBadRequest, status, body)
	}
	for _, body := range []string{valid, strings.Replace(valid, `"must":[`, `"must":["odd",`, 1)} {
		status, none := h.search(body, now.Add(-time.Hour), now)
		require.Equal(t, http.StatusOK, status, body)
		require.Empty(t, none)
	}
}

func TestStripeEventsSentToMoesifAreKept(t *testing.T) {
	h := newHarness(t)
	status, _ := h.control(http.MethodPost, "/partners/stripe?moesif_application_id=app-1", `{"id":"evt_1","type":"invoice.paid"}`)
	require.Equal(t, http.StatusOK, status)
	for path, body := range map[string]string{
		"/partners/stripe":                         `{"id":"evt_1","type":"invoice.paid"}`,
		"/partners/stripe?moesif_application_id=a": `{"id":"evt_1"}`,
		"/partners/stripe?moesif_application_id=b": `not json`,
	} {
		status, _ = h.control(http.MethodPost, path, body)
		require.Equal(t, http.StatusBadRequest, status, path)
	}
	require.Equal(t, []any{map[string]any{"application": "app-1", "id": "evt_1", "type": "invoice.paid"}}, h.state()["stripeEvents"])

	app := h.service.application(testBlock)
	for i := 0; i < maxRetainedStripeEvents+3; i++ {
		h.control(http.MethodPost, "/partners/stripe?moesif_application_id=app-1", fmt.Sprintf(`{"id":"evt_%d","type":"t"}`, i))
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	require.Len(t, app.stripeEvents, maxRetainedStripeEvents)
}

func TestFailuresFailMatchingCallsTheGivenNumberOfTimes(t *testing.T) {
	h := newHarness(t)
	status, _ := h.control(http.MethodPost, "/test/fail", `{"method":"post","match":"partneraccount","status":503,"times":2}`)
	require.Equal(t, http.StatusCreated, status)
	for i := 0; i < 2; i++ {
		status, body := h.api(http.MethodPost, "/v1/~/billing/partneraccount/stripe", partnerAccountBody("enabled", testStripeKey))
		require.Equal(t, http.StatusServiceUnavailable, status)
		require.Equal(t, "Request failed by the testbench.", body["message"])
	}
	h.connect()
	require.Equal(t, float64(0), h.state()["failures"])

	status, _ = h.control(http.MethodPost, "/test/fail", `{"path":"/v1/~/billing/catalog/plans","status":500}`)
	require.Equal(t, http.StatusCreated, status)
	status, _ = h.api(http.MethodGet, "/v1/~/billing/catalog/plans?provider=stripe", "")
	require.Equal(t, http.StatusInternalServerError, status)
	status, _ = h.api(http.MethodGet, "/v1/~/billing/catalog/plans?provider=stripe", "")
	require.Equal(t, http.StatusOK, status)

	for _, body := range []string{`{"status":500}`, `{"match":"x","status":302}`, `{"match":"x","status":500,"times":-2}`,
		`{"match":"x","status":500,"when":"now"}`, ``} {
		status, _ = h.control(http.MethodPost, "/test/fail", body)
		require.Equal(t, http.StatusBadRequest, status, body)
	}
}

func TestBlocksHaveSeparateApplicationsAndResetClearsOne(t *testing.T) {
	h := newHarness(t)
	var wg sync.WaitGroup
	for _, block := range []string{"block-a", "block-b"} {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(block string) {
				defer wg.Done()
				req, _ := http.NewRequest(http.MethodGet, h.moesif.URL+"/"+block+"/v1/~/billing/meters", nil)
				req.Header.Set("Authorization", "Bearer "+managementKey)
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					_ = resp.Body.Close()
				}
			}(block)
		}
	}
	wg.Wait()
	require.Len(t, h.state()["requests"], 8)

	status, _ := h.control(http.MethodPost, "/test/reset", "")
	require.Equal(t, http.StatusNoContent, status)
	require.Len(t, h.state()["requests"], 0)
	other := h.service.application("block-b")
	other.mu.Lock()
	require.Len(t, other.requests, 8)
	other.mu.Unlock()

	status, health := h.control(http.MethodGet, "/test/health", "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "moesif", health["service"])
	resp, err := http.Get(h.moesif.URL + "/v1/~/billing/meters")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRetainedRecordsAreBounded(t *testing.T) {
	app := newApplication()
	for i := 0; i < maxRetainedRequests+2; i++ {
		app.record(http.MethodGet, "/v1/x", fmt.Sprint(i))
	}
	require.Len(t, app.requests, maxRetainedRequests)
	require.Equal(t, "2", app.requests[0].Query)

	h := newHarness(t)
	h.connect()
	_, priceID, _ := h.usagePlan("")
	_, subscriptionID := h.subscribe(priceID)
	usage := h.service.application(testBlock)
	usage.mu.Lock()
	usage.records = make([]usageRecord, maxRetainedRecords)
	usage.mu.Unlock()
	status, _ := h.control(http.MethodPost, "/test/usage", fmt.Sprintf(`{"subscription":%q,"value":1}`, subscriptionID))
	require.Equal(t, http.StatusCreated, status)
	usage.mu.Lock()
	defer usage.mu.Unlock()
	require.Len(t, usage.records, maxRetainedRecords)
	require.Equal(t, subscriptionID, usage.records[maxRetainedRecords-1].Subscription)
}

func TestOversizedRequestsAreRefused(t *testing.T) {
	h := newHarness(t)
	status, _ := h.api(http.MethodPost, "/v1/~/billing/meters", strings.Repeat("x", maxRequestBodyBytes+1))
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
	status, _ = h.control(http.MethodPost, "/test/usage", strings.Repeat("x", maxRequestBodyBytes+1))
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
	status, _ = h.control(http.MethodPost, "/partners/stripe?moesif_application_id=a", strings.Repeat("x", maxRequestBodyBytes+1))
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
}

func TestBucketsStartOnTheHourDayOrMonday(t *testing.T) {
	at := time.Date(2026, 10, 25, 17, 45, 0, 0, time.UTC)
	require.Equal(t, time.Date(2026, 10, 25, 17, 0, 0, 0, time.UTC), bucketStart(at, "1h", time.UTC))
	require.Equal(t, time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), bucketStart(at, "1d", time.UTC))
	require.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), bucketStart(at, "1w", time.UTC))
	require.Empty(t, seriesBuckets(map[time.Time]*bucket{}, "1d"))
}
