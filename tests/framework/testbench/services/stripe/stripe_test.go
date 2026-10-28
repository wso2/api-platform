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

package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testBlock = "block-a"
	testKey   = "sk_test_abc123"
)

type harness struct {
	t       *testing.T
	service *Service
	server  *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	service := New()
	server := httptest.NewServer(service.Handler())
	t.Cleanup(server.Close)
	return &harness{t: t, service: service, server: server}
}

// call sends a Stripe API call to a block with a key; form is form-encoded.
func (h *harness) call(block, key, method, path, form string) (int, map[string]any) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.server.URL+"/"+block+path, strings.NewReader(form))
	require.NoError(h.t, err)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return h.do(req)
}

func (h *harness) api(method, path, form string) (int, map[string]any) {
	h.t.Helper()
	return h.call(testBlock, testKey, method, path, form)
}

func (h *harness) control(method, path, body string) (int, map[string]any) {
	h.t.Helper()
	req, err := http.NewRequest(method, h.server.URL+"/"+testBlock+path, strings.NewReader(body))
	require.NoError(h.t, err)
	return h.do(req)
}

func (h *harness) do(req *http.Request) (int, map[string]any) {
	h.t.Helper()
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(h.t, err)
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		body = map[string]any{"text": string(raw)}
	}
	return resp.StatusCode, body
}

func (h *harness) state() map[string]any {
	h.t.Helper()
	status, body := h.control(http.MethodGet, "/test/state", "")
	require.Equal(h.t, http.StatusOK, status)
	return body
}

func (h *harness) addPrices() {
	h.t.Helper()
	require.NoError(h.t, h.service.AddPrice(testBlock, PriceSpec{
		ID: "price_fee", Currency: "USD", Interval: "month", UnitAmount: 900, Product: "Gold", Nickname: "Gold fee",
	}))
	require.NoError(h.t, h.service.AddPrice(testBlock, PriceSpec{
		ID: "price_calls", Currency: "usd", Interval: "month", Metered: true, UnitAmount: 0.5, Product: "Gold calls",
	}))
}

func (h *harness) customer() string {
	h.t.Helper()
	status, body := h.api(http.MethodPost, "/v1/customers", "name=Ann&email=ann%40example.com&metadata[user]=u1")
	require.Equal(h.t, http.StatusOK, status)
	return body["id"].(string)
}

// subscribe opens and pays a checkout for the fee and the metered price.
func (h *harness) subscribe() (customerID, sessionID, subscriptionID string) {
	h.t.Helper()
	customerID = h.customer()
	form := "mode=subscription&customer=" + customerID + "&client_reference_id=chk-1&metadata[wso2_checkout]=chk-1" +
		"&subscription_data[metadata][wso2_checkout]=chk-1&line_items[0][price]=price_fee&line_items[0][quantity]=1" +
		"&line_items[1][price]=price_calls"
	status, sess := h.api(http.MethodPost, "/v1/checkout/sessions", form)
	require.Equal(h.t, http.StatusOK, status, sess)
	sessionID = sess["id"].(string)
	status, done := h.control(http.MethodPost, "/test/checkout-sessions/"+sessionID+"/complete", "")
	require.Equal(h.t, http.StatusOK, status, done)
	return customerID, sessionID, done["subscription"].(string)
}

// receiver is a webhook endpoint that checks each event's signature against its secret.
type receiver struct {
	mu     sync.Mutex
	secret string
	status int
	events []receivedEvent
	server *httptest.Server
}

type receivedEvent struct {
	ID    string
	Type  string
	Valid bool
	Age   time.Duration
}

func newReceiver(t *testing.T) *receiver {
	t.Helper()
	r := &receiver{status: http.StatusOK}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		var envelope struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}
		_ = json.Unmarshal(body, &envelope)
		r.mu.Lock()
		defer r.mu.Unlock()
		valid, age := verify(r.secret, req.Header.Get("Stripe-Signature"), body)
		r.events = append(r.events, receivedEvent{ID: envelope.ID, Type: envelope.Type, Valid: valid, Age: age})
		w.WriteHeader(r.status)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func verify(secret, header string, body []byte) (bool, time.Duration) {
	var timestamp int64
	var signature string
	for _, part := range strings.Split(header, ",") {
		key, value, _ := strings.Cut(part, "=")
		switch key {
		case "t":
			timestamp, _ = strconv.ParseInt(value, 10, 64)
		case "v1":
			signature = value
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, body)))
	return hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(signature)),
		time.Since(time.Unix(timestamp, 0))
}

func (r *receiver) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

func (r *receiver) received() []receivedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]receivedEvent(nil), r.events...)
}

// listen registers the receiver for events.
func (h *harness) listen(r *receiver, events ...string) string {
	h.t.Helper()
	form := url.Values{"url": {r.server.URL + "/hook"}, "api_version": {"2024-06-20"}}
	for i, e := range events {
		form.Set(fmt.Sprintf("enabled_events[%d]", i), e)
	}
	status, body := h.api(http.MethodPost, "/v1/webhook_endpoints", form.Encode())
	require.Equal(h.t, http.StatusOK, status, body)
	r.mu.Lock()
	r.secret = body["secret"].(string)
	r.mu.Unlock()
	return body["id"].(string)
}

func recordUsage(t *testing.T, service *Service, key, subscription, price string, quantity int64) {
	t.Helper()
	_, err := service.RecordUsage(key, subscription, price, quantity)
	require.NoError(t, err)
}

func refuseUsage(t *testing.T, service *Service, key, subscription, price string, quantity int64) {
	t.Helper()
	_, err := service.RecordUsage(key, subscription, price, quantity)
	require.Error(t, err)
}

func TestServiceDescribesItself(t *testing.T) {
	service := New()
	require.Equal(t, "stripe", service.Name())
	require.Equal(t, Port, service.Port())
	require.True(t, service.Stateful())
	require.NotEmpty(t, service.PartitionKey())
}

func TestAPICallsNeedATestSecretOrRestrictedKey(t *testing.T) {
	h := newHarness(t)
	for _, key := range []string{"", "pk_test_abc", "sk_live_abc", "sk_test_", "Bearer"} {
		status, body := h.call(testBlock, key, http.MethodGet, "/v1/webhook_endpoints", "")
		require.Equal(t, http.StatusUnauthorized, status, key)
		require.Equal(t, "invalid_request_error", body["error"].(map[string]any)["type"])
	}
	for _, key := range []string{"sk_test_abc", "rk_test_abc"} {
		status, _ := h.call(testBlock, key, http.MethodGet, "/v1/webhook_endpoints", "")
		require.Equal(t, http.StatusOK, status, key)
	}
}

func TestRequestsWithoutABlockAreRefused(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Get(h.server.URL + "/v1/webhook_endpoints")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestMalformedParametersAreRefused(t *testing.T) {
	h := newHarness(t)
	for _, form := range []string{"a[=1", "[a]=1", "a[b]c=1", "a=1&a[b]=2", "%zz=1"} {
		status, _ := h.api(http.MethodPost, "/v1/customers", form)
		require.Equal(t, http.StatusBadRequest, status, form)
	}
}

func TestParseParamsBuildsNestedValuesAndLists(t *testing.T) {
	params, err := parseParams("limit=5&created[gte]=10",
		[]byte("line_items[1][price]=b&line_items[0][price]=a&line_items[0][quantity]=1&metadata[k]=v&limit=7"))
	require.NoError(t, err)
	require.Equal(t, "7", params["limit"])
	require.Equal(t, map[string]any{"gte": "10"}, params["created"])
	require.Equal(t, []any{map[string]any{"price": "a", "quantity": "1"}, map[string]any{"price": "b"}},
		params["line_items"])
	require.Equal(t, map[string]string{"k": "v"}, stringMap(params["metadata"].(map[string]any)))

	gaps, err := parseParams("", []byte("list[0]=a&list[2]=c&odd[01]=x"))
	require.NoError(t, err)
	require.Equal(t, []any{"a", "c"}, gaps["list"])
	require.Equal(t, map[string]any{"01": "x"}, gaps["odd"])

	_, err = parseParams("", []byte("a[b]=1&a=2"))
	require.Error(t, err)
	require.Equal(t, 3, intParam(map[string]any{"n": "x"}, "n", 3, 10))
	require.Equal(t, 3, intParam(map[string]any{"n": "0"}, "n", 3, 10))
	require.Equal(t, 10, intParam(map[string]any{"n": "50"}, "n", 3, 10))
	value, given := boolParam(map[string]any{}, "b")
	require.False(t, value)
	require.False(t, given)
}

func TestWebhookEndpointsAreCreatedListedUpdatedAndDeleted(t *testing.T) {
	h := newHarness(t)
	status, created := h.api(http.MethodPost, "/v1/webhook_endpoints",
		"url=https%3A%2F%2Fportal%2Fhook&enabled_events[0]=invoice.paid&description=Portal&metadata[owner]=portal&api_version=2024-06-20")
	require.Equal(t, http.StatusOK, status, created)
	id := created["id"].(string)
	require.True(t, strings.HasPrefix(created["secret"].(string), "whsec_"))
	require.Equal(t, "enabled", created["status"])

	status, listed := h.api(http.MethodGet, "/v1/webhook_endpoints?limit=100", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, listed["data"], 1)
	require.NotContains(t, listed["data"].([]any)[0], "secret")

	status, updated := h.api(http.MethodPost, "/v1/webhook_endpoints/"+id,
		"url=https%3A%2F%2Fother%2Fhook&enabled_events[0]=*&disabled=true&description=Moved")
	require.Equal(t, http.StatusOK, status, updated)
	require.Equal(t, "https://other/hook", updated["url"])
	require.Equal(t, "disabled", updated["status"])
	require.Equal(t, []any{"*"}, updated["enabled_events"])

	for _, form := range []string{"url=ftp%3A%2F%2Fx", "enabled_events[0]="} {
		status, _ = h.api(http.MethodPost, "/v1/webhook_endpoints/"+id, form)
		require.Equal(t, http.StatusBadRequest, status, form)
	}
	status, _ = h.api(http.MethodPost, "/v1/webhook_endpoints/we_missing", "disabled=false")
	require.Equal(t, http.StatusNotFound, status)

	status, deleted := h.api(http.MethodDelete, "/v1/webhook_endpoints/"+id, "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, true, deleted["deleted"])
	status, gone := h.api(http.MethodDelete, "/v1/webhook_endpoints/"+id, "")
	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "resource_missing", gone["error"].(map[string]any)["code"])
}

func TestWebhookEndpointCreationIsValidatedAndLimited(t *testing.T) {
	h := newHarness(t)
	for _, form := range []string{"enabled_events[0]=a", "url=portal&enabled_events[0]=a", "url=https%3A%2F%2Fp"} {
		status, _ := h.api(http.MethodPost, "/v1/webhook_endpoints", form)
		require.Equal(t, http.StatusBadRequest, status, form)
	}
	for i := 0; i < maxWebhookEndpoints; i++ {
		status, _ := h.api(http.MethodPost, "/v1/webhook_endpoints",
			fmt.Sprintf("url=https%%3A%%2F%%2Fp%d&enabled_events[0]=a", i))
		require.Equal(t, http.StatusOK, status)
	}
	status, _ := h.api(http.MethodPost, "/v1/webhook_endpoints", "url=https%3A%2F%2Fp&enabled_events[0]=a")
	require.Equal(t, http.StatusBadRequest, status)
	status, listed := h.api(http.MethodGet, "/v1/webhook_endpoints?limit=3", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, listed["data"], 3)
	require.Equal(t, true, listed["has_more"])
}

func TestCheckoutSessionsAreValidated(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	for _, spec := range []PriceSpec{
		{ID: "price_eur", Currency: "eur", Interval: "month", UnitAmount: 1},
		{ID: "price_year", Currency: "usd", Interval: "year", UnitAmount: 1},
		{ID: "price_old", Currency: "usd", Interval: "month", UnitAmount: 1},
	} {
		require.NoError(t, h.service.AddPrice(testBlock, spec))
	}
	require.NoError(t, h.service.ArchivePrice(testBlock, "price_old"))
	customerID := h.customer()
	base := "mode=subscription&customer=" + customerID
	fee := "&line_items[0][price]=price_fee&line_items[0][quantity]=1"
	cases := []struct{ form, code string }{
		{"mode=payment&customer=" + customerID + fee, "parameter_invalid"},
		{"mode=subscription&customer=cus_missing" + fee, "resource_missing"},
		{base, "parameter_invalid"},
		{base + "&line_items[0][price]=price_none", "resource_missing"},
		{base + "&line_items[0][price]=price_fee", "parameter_invalid"},
		{base + "&line_items[0][price]=price_old&line_items[0][quantity]=1", "parameter_invalid"},
		{base + "&line_items[0][price]=price_calls&line_items[0][quantity]=1", "parameter_invalid"},
		{base + "&line_items[0][price]=price_fee&line_items[0][quantity]=0", "parameter_invalid"},
		{base + fee + "&line_items[1][price]=price_eur&line_items[1][quantity]=1", "parameter_invalid"},
		{base + fee + "&line_items[1][price]=price_year&line_items[1][quantity]=1", "parameter_invalid"},
	}
	for _, c := range cases {
		status, body := h.api(http.MethodPost, "/v1/checkout/sessions", c.form)
		require.Equal(t, http.StatusBadRequest, status, c.form)
		require.Equal(t, c.code, body["error"].(map[string]any)["code"], c.form)
	}
}

func TestCheckoutCompletesIntoAPaidSubscriptionAndSendsSignedEvents(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "checkout.session.completed", "invoice.paid", "customer.subscription.updated")
	moesif := newReceiver(t)
	h.listen(moesif, "customer.subscription.created", "invoice.paid")

	customerID := h.customer()
	status, sess := h.api(http.MethodPost, "/v1/checkout/sessions", "mode=subscription&customer="+customerID+
		"&client_reference_id=chk-9&line_items[0][price]=price_fee&line_items[0][quantity]=2&line_items[1][price]=price_calls"+
		"&subscription_data[metadata][wso2_checkout]=chk-9")
	require.Equal(t, http.StatusOK, status, sess)
	require.Equal(t, "open", sess["status"])
	require.True(t, strings.HasPrefix(sess["client_secret"].(string), sess["id"].(string)+"_secret_"))

	status, done := h.control(http.MethodPost, "/test/checkout-sessions/chk-9/complete", "")
	require.Equal(t, http.StatusOK, status, done)
	require.Len(t, done["deliveries"], 4)
	for _, d := range done["deliveries"].([]any) {
		require.Equal(t, float64(http.StatusOK), d.(map[string]any)["status"])
	}
	require.Equal(t, []string{"checkout.session.completed", "invoice.paid"}, portal.types())
	require.Equal(t, []string{"customer.subscription.created", "invoice.paid"}, moesif.types())
	for _, e := range append(portal.received(), moesif.received()...) {
		require.True(t, e.Valid, e.Type)
	}

	status, read := h.api(http.MethodGet, "/v1/checkout/sessions/"+sess["id"].(string), "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "complete", read["status"])
	require.Equal(t, "paid", read["payment_status"])
	require.Equal(t, done["subscription"], read["subscription"])

	status, sub := h.api(http.MethodGet, "/v1/subscriptions/"+done["subscription"].(string), "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "active", sub["status"])
	require.Equal(t, map[string]any{"wso2_checkout": "chk-9"}, sub["metadata"])
	start, end := int64(sub["current_period_start"].(float64)), int64(sub["current_period_end"].(float64))
	require.Equal(t, time.Unix(start, 0).AddDate(0, 1, 0).Unix(), end)

	status, inv := h.api(http.MethodGet, "/v1/invoices/"+done["invoice"].(string), "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "paid", inv["status"])
	require.Equal(t, float64(1800), inv["total"])
	require.Equal(t, float64(1800), inv["amount_paid"])
	require.Equal(t, "Ann", inv["customer_name"])
	require.Equal(t, "chk-9", inv["subscription_details"].(map[string]any)["metadata"].(map[string]any)["wso2_checkout"])
	lines := inv["lines"].(map[string]any)["data"].([]any)
	require.Len(t, lines, 2)
	require.Equal(t, float64(2), lines[0].(map[string]any)["quantity"])
	require.Equal(t, float64(0), lines[1].(map[string]any)["amount"])

	status, _ = h.control(http.MethodPost, "/test/checkout-sessions/chk-9/complete", "")
	require.Equal(t, http.StatusConflict, status)
	status, _ = h.control(http.MethodPost, "/test/checkout-sessions/cs_missing/complete", "")
	require.Equal(t, http.StatusNotFound, status)
}

func TestACheckoutWithNothingToPayNeedsNoPayment(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	customerID := h.customer()
	status, sess := h.api(http.MethodPost, "/v1/checkout/sessions",
		"mode=subscription&customer="+customerID+"&line_items[0][price]=price_calls")
	require.Equal(t, http.StatusOK, status)
	status, _ = h.control(http.MethodPost, "/test/checkout-sessions/"+sess["id"].(string)+"/complete", "")
	require.Equal(t, http.StatusOK, status)
	_, read := h.api(http.MethodGet, "/v1/checkout/sessions/"+sess["id"].(string), "")
	require.Equal(t, "no_payment_required", read["payment_status"])
}

func TestExpiringACheckoutSendsTheEventInTheBackground(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "checkout.session.expired")
	customerID := h.customer()
	_, sess := h.api(http.MethodPost, "/v1/checkout/sessions",
		"mode=subscription&customer="+customerID+"&line_items[0][price]=price_fee&line_items[0][quantity]=1")
	id := sess["id"].(string)

	status, expired := h.api(http.MethodPost, "/v1/checkout/sessions/"+id+"/expire", "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "expired", expired["status"])
	h.service.sending.Wait()
	require.Equal(t, []string{"checkout.session.expired"}, portal.types())

	status, again := h.api(http.MethodPost, "/v1/checkout/sessions/"+id+"/expire", "")
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, again["error"].(map[string]any)["message"], "open")
	status, _ = h.api(http.MethodPost, "/v1/checkout/sessions/cs_missing/expire", "")
	require.Equal(t, http.StatusNotFound, status)
	status, _ = h.api(http.MethodGet, "/v1/checkout/sessions/cs_missing", "")
	require.Equal(t, http.StatusNotFound, status)
	status, _ = h.control(http.MethodPost, "/test/checkout-sessions/"+id+"/complete", "")
	require.Equal(t, http.StatusConflict, status)
}

func TestAnAbandonedCheckoutExpiresAndSendsTheEvent(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "checkout.session.expired")
	customerID := h.customer()
	_, sess := h.api(http.MethodPost, "/v1/checkout/sessions", "mode=subscription&customer="+customerID+
		"&client_reference_id=chk-2&line_items[0][price]=price_fee&line_items[0][quantity]=1")

	status, expired := h.control(http.MethodPost, "/test/checkout-sessions/chk-2/expire", "")
	require.Equal(t, http.StatusOK, status, expired)
	require.Len(t, expired["deliveries"], 1)
	require.Equal(t, []string{"checkout.session.expired"}, portal.types())
	_, read := h.api(http.MethodGet, "/v1/checkout/sessions/"+sess["id"].(string), "")
	require.Equal(t, "expired", read["status"])

	status, _ = h.control(http.MethodPost, "/test/checkout-sessions/chk-2/expire", "")
	require.Equal(t, http.StatusConflict, status)
	status, _ = h.control(http.MethodPost, "/test/checkout-sessions/cs_missing/expire", "")
	require.Equal(t, http.StatusNotFound, status)
}

func TestEndingASubscriptionAtPeriodEndIssuesTheFinalUsageInvoice(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "customer.subscription.updated", "customer.subscription.deleted", "invoice.paid")
	_, _, subID := h.subscribe()
	portal.mu.Lock()
	portal.events = nil
	portal.mu.Unlock()

	status, updated := h.api(http.MethodPost, "/v1/subscriptions/"+subID, "cancel_at_period_end=true&metadata[note]=x")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, true, updated["cancel_at_period_end"])
	require.Equal(t, "cancellation_requested", updated["cancellation_details"].(map[string]any)["reason"])
	h.service.sending.Wait()
	require.Equal(t, []string{"customer.subscription.updated"}, portal.types())

	recordUsage(t, h.service, testBlock, subID, "price_calls", 30)
	recordUsage(t, h.service, testBlock, subID, "price_calls", 11)
	status, ended := h.control(http.MethodPost, "/test/subscriptions/"+subID+"/period-end", "")
	require.Equal(t, http.StatusOK, status, ended)
	require.Equal(t, []string{"customer.subscription.updated", "invoice.paid", "customer.subscription.deleted"}, portal.types())

	_, final := h.api(http.MethodGet, "/v1/invoices/"+ended["invoice"].(string), "")
	require.Equal(t, float64(21), final["total"])
	require.Len(t, final["lines"].(map[string]any)["data"], 1)
	_, sub := h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "canceled", sub["status"])
	require.NotNil(t, sub["ended_at"])

	status, _ = h.control(http.MethodPost, "/test/subscriptions/"+subID+"/period-end", "")
	require.Equal(t, http.StatusConflict, status)
	status, _ = h.api(http.MethodPost, "/v1/subscriptions/"+subID, "cancel_at_period_end=false")
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodPost, "/v1/subscriptions/sub_missing", "cancel_at_period_end=true")
	require.Equal(t, http.StatusNotFound, status)
	status, _ = h.control(http.MethodPost, "/test/subscriptions/sub_missing/period-end", "")
	require.Equal(t, http.StatusNotFound, status)
}

func TestAPeriodEndRenewsASubscriptionAndStartsItsUsageAgain(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	_, _, subID := h.subscribe()
	recordUsage(t, h.service, testBlock, subID, "price_calls", 100)

	status, renewed := h.control(http.MethodPost, "/test/subscriptions/"+subID+"/period-end", "")
	require.Equal(t, http.StatusOK, status)
	_, inv := h.api(http.MethodGet, "/v1/invoices/"+renewed["invoice"].(string), "")
	require.Equal(t, float64(950), inv["total"])

	state := h.state()
	sub := state["subscriptions"].([]any)[0].(map[string]any)
	require.Equal(t, "active", sub["status"])
	require.Empty(t, sub["usage"])
	require.Equal(t, renewed["invoice"], sub["latest_invoice"])
}

func TestCancellingASubscriptionNowInvoicesItsUsageWhenAsked(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "customer.subscription.deleted", "invoice.paid")
	_, _, subID := h.subscribe()
	recordUsage(t, h.service, testBlock, subID, "price_calls", 8)

	status, canceled := h.api(http.MethodDelete, "/v1/subscriptions/"+subID+"?invoice_now=true", "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "canceled", canceled["status"])
	h.service.sending.Wait()
	require.Equal(t, []string{"invoice.paid", "invoice.paid", "customer.subscription.deleted"}, portal.types())

	_, invoices := h.api(http.MethodGet, "/v1/invoices?subscription="+subID, "")
	data := invoices["data"].([]any)
	require.Len(t, data, 2)
	require.Equal(t, float64(4), data[0].(map[string]any)["total"])

	status, _ = h.api(http.MethodDelete, "/v1/subscriptions/"+subID, "")
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodDelete, "/v1/subscriptions/sub_missing", "")
	require.Equal(t, http.StatusNotFound, status)
	refuseUsage(t, h.service, testBlock, subID, "price_calls", 1)

	state := h.state()
	var cancel map[string]any
	for _, r := range state["requests"].([]any) {
		if r.(map[string]any)["method"] == http.MethodDelete {
			cancel = r.(map[string]any)
			break
		}
	}
	require.Equal(t, "invoice_now=true", cancel["query"])
}

func TestCancellingWithoutAFinalInvoiceIssuesNone(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	_, _, subID := h.subscribe()
	status, _ := h.api(http.MethodDelete, "/v1/subscriptions/"+subID, "")
	require.Equal(t, http.StatusOK, status)
	_, invoices := h.api(http.MethodGet, "/v1/invoices?subscription="+subID, "")
	require.Len(t, invoices["data"], 1)
}

func TestPaymentStatusChangesFollowStripe(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "invoice.payment_failed", "invoice.paid", "customer.subscription.updated", "customer.subscription.deleted")
	_, _, subID := h.subscribe()
	path := "/test/subscriptions/" + subID
	openInvoices := func() []map[string]any {
		_, list := h.api(http.MethodGet, "/v1/invoices?subscription="+subID+"&limit=100", "")
		var open []map[string]any
		for _, inv := range list["data"].([]any) {
			if inv.(map[string]any)["status"] == "open" {
				open = append(open, inv.(map[string]any))
			}
		}
		return open
	}

	status, body := h.control(http.MethodPost, path, `{"status":"past_due"}`)
	require.Equal(t, http.StatusOK, status, body)
	open := openInvoices()
	require.Len(t, open, 1)
	require.NotNil(t, open[0]["next_payment_attempt"])
	require.Equal(t, float64(1), open[0]["attempt_count"])

	status, _ = h.control(http.MethodPost, path, `{"status":"past_due","retriesEnded":true}`)
	require.Equal(t, http.StatusOK, status)
	open = openInvoices()
	require.Len(t, open, 1)
	require.Nil(t, open[0]["next_payment_attempt"])
	require.Equal(t, float64(2), open[0]["attempt_count"])
	_, sub := h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "past_due", sub["status"])

	status, _ = h.control(http.MethodPost, path, `{"status":"unpaid"}`)
	require.Equal(t, http.StatusOK, status)
	_, sub = h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "unpaid", sub["status"])

	status, _ = h.control(http.MethodPost, path, `{"status":"active"}`)
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, openInvoices())
	_, sub = h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "active", sub["status"])

	status, _ = h.control(http.MethodPost, path, `{"status":"canceled"}`)
	require.Equal(t, http.StatusOK, status)
	_, sub = h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "payment_failed", sub["cancellation_details"].(map[string]any)["reason"])
	require.Equal(t, []string{
		"invoice.paid",
		"invoice.payment_failed", "customer.subscription.updated",
		"invoice.payment_failed", "customer.subscription.updated",
		"invoice.payment_failed", "customer.subscription.updated",
		"invoice.paid", "customer.subscription.updated",
		"customer.subscription.deleted",
	}, portal.types())

	for body, want := range map[string]int{
		`{"status":"active"}`: http.StatusConflict,
		`{"status":"paused"}`: http.StatusConflict,
		`{"state":"active"}`:  http.StatusBadRequest,
		`[`:                   http.StatusBadRequest,
	} {
		status, _ = h.control(http.MethodPost, path, body)
		require.Equal(t, want, status, body)
	}
	status, _ = h.control(http.MethodPost, "/test/subscriptions/sub_missing", `{"status":"active"}`)
	require.Equal(t, http.StatusNotFound, status)
}

func TestAnUnknownStatusIsRefused(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	_, _, subID := h.subscribe()
	status, _ := h.control(http.MethodPost, "/test/subscriptions/"+subID, `{"status":"paused"}`)
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.control(http.MethodPost, "/test/subscriptions/"+subID, `{"status":"canceled","reason":"cancellation_requested"}`)
	require.Equal(t, http.StatusOK, status)
	_, sub := h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "cancellation_requested", sub["cancellation_details"].(map[string]any)["reason"])
}

func TestInvoicesAreListedNewestFirstAndPaged(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	customerID, _, subID := h.subscribe()
	june := time.Now().AddDate(0, -4, 0).Unix()
	july := time.Now().AddDate(0, -3, 0).Unix()
	for _, created := range []int64{june, july} {
		status, added := h.control(http.MethodPost, "/test/invoices",
			fmt.Sprintf(`{"subscription":%q,"status":"paid","created":%d,"usage":{"price_calls":10}}`, subID, created))
		require.Equal(t, http.StatusOK, status, added)
		require.Equal(t, float64(905), added["total"])
	}

	status, all := h.api(http.MethodGet, "/v1/invoices?customer="+customerID+"&limit=100", "")
	require.Equal(t, http.StatusOK, status)
	data := all["data"].([]any)
	require.Len(t, data, 3)
	require.Equal(t, float64(july), data[1].(map[string]any)["created"])
	require.Equal(t, float64(june), data[2].(map[string]any)["created"])

	query := fmt.Sprintf("/v1/invoices?created[gte]=%d&created[lt]=%d&limit=1", june, july+1)
	status, first := h.api(http.MethodGet, query, "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, first["data"], 1)
	require.Equal(t, true, first["has_more"])
	last := first["data"].([]any)[0].(map[string]any)["id"].(string)
	status, second := h.api(http.MethodGet, query+"&starting_after="+last, "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, second["data"], 1)
	require.Equal(t, false, second["has_more"])
	require.Equal(t, float64(june), second["data"].([]any)[0].(map[string]any)["created"])

	status, missingCursor := h.api(http.MethodGet, "/v1/invoices?starting_after=in_missing", "")
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "resource_missing", missingCursor["error"].(map[string]any)["code"])
	status, _ = h.api(http.MethodGet, "/v1/invoices?created[gte]=soon", "")
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodGet, "/v1/invoices/in_missing", "")
	require.Equal(t, http.StatusNotFound, status)
	status, none := h.api(http.MethodGet, "/v1/invoices?subscription=sub_other", "")
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, none["data"])

	for body, want := range map[string]int{
		`{"subscription":"sub_missing","status":"paid"}`:   http.StatusNotFound,
		`{"subscription":"` + subID + `","status":"void"}`: http.StatusBadRequest,
		`{"subscription":"` + subID + `","extra":1}`:       http.StatusBadRequest,
	} {
		status, _ = h.control(http.MethodPost, "/test/invoices", body)
		require.Equal(t, want, status, body)
	}
}

func TestInvoiceLinesBeyondTheEmbeddedOnesArePaged(t *testing.T) {
	h := newHarness(t)
	customerID := h.customer()
	form := "mode=subscription&customer=" + customerID
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("price_%02d", i)
		require.NoError(t, h.service.AddPrice(testBlock, PriceSpec{ID: id, Currency: "usd", Interval: "month", Metered: true, UnitAmount: 1}))
		form += fmt.Sprintf("&line_items[%d][price]=%s", i, id)
	}
	_, sess := h.api(http.MethodPost, "/v1/checkout/sessions", form)
	_, done := h.control(http.MethodPost, "/test/checkout-sessions/"+sess["id"].(string)+"/complete", "")
	invoiceID := done["invoice"].(string)

	_, inv := h.api(http.MethodGet, "/v1/invoices/"+invoiceID, "")
	embedded := inv["lines"].(map[string]any)
	require.Len(t, embedded["data"], embeddedLines)
	require.Equal(t, true, embedded["has_more"])
	lastEmbedded := embedded["data"].([]any)[embeddedLines-1].(map[string]any)["id"].(string)

	status, rest := h.api(http.MethodGet, "/v1/invoices/"+invoiceID+"/lines?limit=100&starting_after="+lastEmbedded, "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, rest["data"], 2)
	require.Equal(t, false, rest["has_more"])
	status, _ = h.api(http.MethodGet, "/v1/invoices/"+invoiceID+"/lines?starting_after=il_missing", "")
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodGet, "/v1/invoices/in_missing/lines", "")
	require.Equal(t, http.StatusNotFound, status)
}

func TestAPreviewShowsTheNextInvoiceAndItsLines(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	_, _, subID := h.subscribe()
	recordUsage(t, h.service, testBlock, subID, "price_calls", 7)

	status, preview := h.api(http.MethodPost, "/v1/invoices/create_preview", "subscription="+subID)
	require.Equal(t, http.StatusOK, status, preview)
	id := preview["id"].(string)
	require.True(t, strings.HasPrefix(id, "upcoming_in_"))
	require.Equal(t, "draft", preview["status"])
	require.Equal(t, float64(904), preview["total"])

	status, lines := h.api(http.MethodGet, "/v1/invoices/"+id+"/lines", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, lines["data"], 2)
	status, _ = h.api(http.MethodGet, "/v1/invoices/"+id, "")
	require.Equal(t, http.StatusNotFound, status)
	status, _ = h.api(http.MethodPost, "/v1/invoices/create_preview", "subscription=sub_missing")
	require.Equal(t, http.StatusBadRequest, status)

	for i := 0; i < maxRetainedPreviews; i++ {
		h.api(http.MethodPost, "/v1/invoices/create_preview", "subscription="+subID)
	}
	status, _ = h.api(http.MethodGet, "/v1/invoices/"+id+"/lines", "")
	require.Equal(t, http.StatusNotFound, status)

	h.api(http.MethodDelete, "/v1/subscriptions/"+subID, "")
	status, _ = h.api(http.MethodPost, "/v1/invoices/create_preview", "subscription="+subID)
	require.Equal(t, http.StatusBadRequest, status)
}

func TestRefundsArePaidOnceForAKnownPayment(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	_, _, subID := h.subscribe()
	_, list := h.api(http.MethodGet, "/v1/invoices?subscription="+subID, "")
	paymentIntent := list["data"].([]any)[0].(map[string]any)["payment_intent"].(string)

	status, refund := h.api(http.MethodPost, "/v1/refunds", "payment_intent="+paymentIntent)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, float64(900), refund["amount"])
	status, _ = h.api(http.MethodPost, "/v1/refunds", "payment_intent="+paymentIntent)
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodPost, "/v1/refunds", "payment_intent=pi_missing")
	require.Equal(t, http.StatusBadRequest, status)
	require.Len(t, h.state()["refunds"], 1)
}

func TestTheBillingPageNeedsAKnownCustomerAndConfiguration(t *testing.T) {
	h := newHarness(t)
	customerID := h.customer()
	status, _ := h.api(http.MethodPost, "/v1/billing_portal/configurations", "metadata[a]=b")
	require.Equal(t, http.StatusBadRequest, status)
	status, config := h.api(http.MethodPost, "/v1/billing_portal/configurations",
		"features[invoice_history][enabled]=true&metadata[wso2_api_portal]=billing-page")
	require.Equal(t, http.StatusOK, status)
	configID := config["id"].(string)

	status, listed := h.api(http.MethodGet, "/v1/billing_portal/configurations?active=true&limit=100", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, listed["data"], 1)
	_, inactive := h.api(http.MethodGet, "/v1/billing_portal/configurations?active=false", "")
	require.Empty(t, inactive["data"])

	status, sess := h.api(http.MethodPost, "/v1/billing_portal/sessions",
		"customer="+customerID+"&configuration="+configID+"&return_url=https%3A%2F%2Fportal%2Fback")
	require.Equal(t, http.StatusOK, status)
	require.True(t, strings.HasPrefix(sess["url"].(string), "https://billing.stripe.test/p/session/"))
	status, _ = h.api(http.MethodPost, "/v1/billing_portal/sessions", "customer="+customerID)
	require.Equal(t, http.StatusOK, status)
	status, _ = h.api(http.MethodPost, "/v1/billing_portal/sessions", "customer=cus_missing")
	require.Equal(t, http.StatusBadRequest, status)
	status, _ = h.api(http.MethodPost, "/v1/billing_portal/sessions", "customer="+customerID+"&configuration=bpc_missing")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestResentEventsCarryTheAskedSignature(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	h.listen(portal, "checkout.session.completed")
	h.subscribe()
	eventID := portal.received()[0].ID

	for _, body := range []string{
		fmt.Sprintf(`{"event":%q}`, eventID),
		`{"type":"checkout.session.completed","signature":"valid"}`,
		fmt.Sprintf(`{"event":%q,"signature":"invalid"}`, eventID),
		`{"type":"checkout.session.completed","signature":"stale"}`,
	} {
		status, resent := h.control(http.MethodPost, "/test/events/resend", body)
		require.Equal(t, http.StatusOK, status, resent)
		require.Equal(t, eventID, resent["event"])
		require.Len(t, resent["deliveries"], 1)
	}
	received := portal.received()
	require.Len(t, received, 5)
	for _, e := range received {
		require.Equal(t, eventID, e.ID)
	}
	require.True(t, received[1].Valid)
	require.True(t, received[2].Valid)
	require.False(t, received[3].Valid)
	require.True(t, received[4].Valid)
	require.Greater(t, received[4].Age, 5*time.Minute)

	for body, want := range map[string]int{
		`{"event":"evt_missing"}`:                   http.StatusNotFound,
		`{"type":"invoice.voided"}`:                 http.StatusNotFound,
		`{"type":"invoice.paid","signature":"odd"}`: http.StatusBadRequest,
		`{}`: http.StatusBadRequest,
		`{"event":"evt_1","type":"invoice.paid"}`: http.StatusBadRequest,
	} {
		status, _ := h.control(http.MethodPost, "/test/events/resend", body)
		require.Equal(t, want, status, body)
	}

	events := h.state()["events"].([]any)
	var deliveries []any
	for _, e := range events {
		if e.(map[string]any)["id"] == eventID {
			deliveries = e.(map[string]any)["deliveries"].([]any)
		}
	}
	require.Len(t, deliveries, 5)
}

func TestFailedDeliveriesAreRecorded(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	portal := newReceiver(t)
	portal.status = http.StatusInternalServerError
	h.listen(portal, "checkout.session.completed")
	status, _ := h.api(http.MethodPost, "/v1/webhook_endpoints", "url=http%3A%2F%2F127.0.0.1%3A1%2Fhook&enabled_events[0]=*")
	require.Equal(t, http.StatusOK, status)
	customerID := h.customer()
	_, sess := h.api(http.MethodPost, "/v1/checkout/sessions",
		"mode=subscription&customer="+customerID+"&line_items[0][price]=price_fee&line_items[0][quantity]=1")

	_, done := h.control(http.MethodPost, "/test/checkout-sessions/"+sess["id"].(string)+"/complete", "")
	deliveries := done["deliveries"].([]any)
	require.Len(t, deliveries, 4)
	require.Equal(t, float64(http.StatusInternalServerError), deliveries[1].(map[string]any)["status"])
	unreachable := deliveries[0].(map[string]any)
	require.Equal(t, float64(0), unreachable["status"])
	require.NotEmpty(t, unreachable["error"])
}

func TestFailuresFailMatchingCallsTheGivenNumberOfTimes(t *testing.T) {
	h := newHarness(t)
	status, added := h.control(http.MethodPost, "/test/fail",
		`{"method":"post","path":"/v1/webhook_endpoints","match":"moesif","status":503,"times":2}`)
	require.Equal(t, http.StatusCreated, status, added)
	require.Equal(t, float64(1), h.state()["failures"])

	moesifEndpoint := "url=https%3A%2F%2Fmoesif%2Fhook&enabled_events[0]=*"
	status, _ = h.api(http.MethodPost, "/v1/webhook_endpoints", "url=https%3A%2F%2Fportal%2Fhook&enabled_events[0]=*")
	require.Equal(t, http.StatusOK, status)
	for i := 0; i < 2; i++ {
		status, failed := h.api(http.MethodPost, "/v1/webhook_endpoints", moesifEndpoint)
		require.Equal(t, http.StatusServiceUnavailable, status)
		require.Equal(t, "api_error", failed["error"].(map[string]any)["type"])
	}
	status, _ = h.api(http.MethodPost, "/v1/webhook_endpoints", moesifEndpoint)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, float64(0), h.state()["failures"])

	status, _ = h.control(http.MethodPost, "/test/fail", `{"match":"limit=100","status":500}`)
	require.Equal(t, http.StatusCreated, status)
	status, _ = h.api(http.MethodGet, "/v1/webhook_endpoints?limit=100", "")
	require.Equal(t, http.StatusInternalServerError, status)
	status, _ = h.api(http.MethodGet, "/v1/webhook_endpoints?limit=100", "")
	require.Equal(t, http.StatusOK, status)

	for _, body := range []string{`{"status":500}`, `{"path":"/v1/x","status":200}`, `{"path":"/v1/x","status":500,"times":-1}`,
		`{"path":"/v1/x","status":500,"often":true}`, `{`} {
		status, _ = h.control(http.MethodPost, "/test/fail", body)
		require.Equal(t, http.StatusBadRequest, status, body)
	}
	status, _ = h.control(http.MethodGet, "/test/state", "not json")
	require.Equal(t, http.StatusBadRequest, status)
}

func TestPriceAmountsFollowTheirPricingModel(t *testing.T) {
	flat := PriceSpec{UnitAmount: 900}
	require.Equal(t, int64(1800), flat.amount(2))
	fraction := PriceSpec{UnitAmount: 0.8}
	require.Equal(t, int64(8), fraction.amount(10))
	blocks := PriceSpec{UnitAmount: 50, DivideBy: 1000}
	require.Equal(t, int64(0), blocks.amount(0))
	require.Equal(t, int64(50), blocks.amount(1))
	require.Equal(t, int64(100), blocks.amount(1001))
	tiers := []Tier{{UpTo: 10, UnitAmount: 5}, {UpTo: 20, UnitAmount: 3, FlatAmount: 100}, {UnitAmount: 1}}
	volume := PriceSpec{Tiers: tiers, TiersMode: "volume"}
	require.Equal(t, int64(50), volume.amount(10))
	require.Equal(t, int64(145), volume.amount(15))
	require.Equal(t, int64(30), volume.amount(30))
	graduated := PriceSpec{Tiers: tiers, TiersMode: "graduated"}
	require.Equal(t, int64(0), graduated.amount(0))
	require.Equal(t, int64(50), graduated.amount(10))
	require.Equal(t, int64(165), graduated.amount(15))
	require.Equal(t, int64(190), graduated.amount(30))
}

func TestPricesAreValidatedAndRendered(t *testing.T) {
	service := New()
	valid := PriceSpec{ID: "p", Currency: "USD", Interval: "month"}
	invalid := []PriceSpec{
		{Currency: "usd", Interval: "month"},
		{ID: "p", Currency: "us", Interval: "month"},
		{ID: "p", Currency: "usd", Interval: "week"},
		{ID: "p", Currency: "usd", Interval: "month", UnitAmount: -1},
		{ID: "p", Currency: "usd", Interval: "month", DivideBy: -1},
		{ID: "p", Currency: "usd", Interval: "month", Tiers: []Tier{{}}},
		{ID: "p", Currency: "usd", Interval: "month", TiersMode: "volume"},
		{ID: "p", Currency: "usd", Interval: "month", TiersMode: "volume", Tiers: []Tier{{UpTo: 5}}},
		{ID: "p", Currency: "usd", Interval: "month", TiersMode: "volume", Tiers: []Tier{{UpTo: 5}, {UpTo: 5}, {}}},
		{ID: "p", Currency: "usd", Interval: "month", TiersMode: "volume", Tiers: []Tier{{UpTo: 5, UnitAmount: -1}, {}}},
	}
	for _, spec := range invalid {
		require.Error(t, service.AddPrice(testBlock, spec), "%+v", spec)
	}
	require.Error(t, service.AddPrice("", valid))
	require.NoError(t, service.AddPrice(testBlock, valid))
	require.Equal(t, "usd", service.account(testBlock).prices["p"].Currency)
	require.Error(t, service.ArchivePrice("", "p"))
	require.Error(t, service.ArchivePrice(testBlock, "missing"))
	require.NoError(t, service.ArchivePrice(testBlock, "p"))
	require.Equal(t, false, service.account(testBlock).prices["p"].json()["active"])

	plain := (&PriceSpec{ID: "p", Currency: "usd", Interval: "year", UnitAmount: 900}).json()
	require.Equal(t, int64(900), plain["unit_amount"])
	require.Equal(t, "licensed", plain["recurring"].(map[string]any)["usage_type"])
	fractional := (&PriceSpec{ID: "p", Metered: true, UnitAmount: 0.5, DivideBy: 10}).json()
	require.Nil(t, fractional["unit_amount"])
	require.Equal(t, "0.5", fractional["unit_amount_decimal"])
	require.Equal(t, "metered", fractional["recurring"].(map[string]any)["usage_type"])
	require.NotNil(t, fractional["transform_quantity"])
	tiered := (&PriceSpec{ID: "p", TiersMode: "graduated", Tiers: []Tier{{UpTo: 5, UnitAmount: 1}, {UnitAmount: 0.5}}}).json()
	require.Equal(t, "tiered", tiered["billing_scheme"])
	require.Nil(t, tiered["tiers"].([]map[string]any)[1]["up_to"])
	require.Equal(t, map[string]any{"id": "price_x", "object": "price"}, priceJSON(map[string]*PriceSpec{}, "price_x"))
}

func TestUsageIsRecordedOnlyForAMeteredPriceOfALiveSubscription(t *testing.T) {
	h := newHarness(t)
	h.addPrices()
	_, _, subID := h.subscribe()
	refuseUsage(t, h.service, "", subID, "price_calls", 1)
	refuseUsage(t, h.service, testBlock, subID, "price_calls", -1)
	refuseUsage(t, h.service, testBlock, "sub_missing", "price_calls", 1)
	refuseUsage(t, h.service, testBlock, subID, "price_fee", 1)
	refuseUsage(t, h.service, testBlock, subID, "price_none", 1)
	recordUsage(t, h.service, testBlock, subID, "price_calls", 0)

	found, err := h.service.RecordUsage(testBlock, "chk-1", "price_calls", 2)
	require.NoError(t, err)
	require.Equal(t, subID, found)
	refuseUsage(t, h.service, testBlock, "", "price_calls", 1)
	status, _ := h.control(http.MethodPost, "/test/subscriptions/chk-1", `{"status":"past_due"}`)
	require.Equal(t, http.StatusOK, status)
	_, sub := h.api(http.MethodGet, "/v1/subscriptions/"+subID, "")
	require.Equal(t, "past_due", sub["status"])
	status, _ = h.control(http.MethodPost, "/test/invoices", `{"subscription":"chk-1","status":"paid"}`)
	require.Equal(t, http.StatusOK, status)
}

func TestBlocksHaveSeparateAccountsAndResetClearsOne(t *testing.T) {
	h := newHarness(t)
	var wg sync.WaitGroup
	for _, block := range []string{"block-a", "block-b"} {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(block string) {
				defer wg.Done()
				req, _ := http.NewRequest(http.MethodPost, h.server.URL+"/"+block+"/v1/customers", strings.NewReader("name=x"))
				req.Header.Set("Authorization", "Bearer "+testKey)
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					_ = resp.Body.Close()
				}
			}(block)
		}
	}
	wg.Wait()
	require.Len(t, h.state()["customers"], 10)
	require.Len(t, h.service.account("block-b").customers, 10)

	status, _ := h.control(http.MethodPost, "/test/reset", "")
	require.Equal(t, http.StatusNoContent, status)
	require.Empty(t, h.state()["customers"])
	require.Len(t, h.service.account("block-b").customers, 10)

	status, health := h.control(http.MethodGet, "/test/health", "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "stripe", health["service"])
}

func TestRetainedRecordsAreBounded(t *testing.T) {
	acct := newAccount()
	for i := 0; i < maxRetainedRequests+5; i++ {
		acct.record(http.MethodGet, "/v1/x", strconv.Itoa(i))
	}
	require.Len(t, acct.requests, maxRetainedRequests)
	require.Equal(t, "5", acct.requests[0].Query)

	for i := 0; i < maxRetainedEvents+5; i++ {
		acct.newEvents(time.Now(), eventSpec{"invoice.paid", map[string]any{"id": "in_" + strconv.Itoa(i)}})
	}
	require.Len(t, acct.events, maxRetainedEvents)
	require.Equal(t, "in_5", acct.events[0].ObjectID)
}

func TestOversizedRequestsAreRefused(t *testing.T) {
	h := newHarness(t)
	status, _ := h.api(http.MethodPost, "/v1/customers", "name="+strings.Repeat("x", maxRequestBodyBytes))
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
}
