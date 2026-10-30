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
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	maxWebhookEndpoints = 16
	defaultListLimit    = 10
	maxListLimit        = 100
)

func badRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, "invalid_request_error", "parameter_invalid", message)
}

// missingParam answers a request naming an object the account does not have, as Stripe does for a
// parameter: 400 resource_missing.
func missingParam(w http.ResponseWriter, kind, id string) {
	writeError(w, http.StatusBadRequest, "invalid_request_error", "resource_missing",
		"No such "+kind+": '"+id+"'")
}

func list(url string, data []map[string]any, hasMore bool) map[string]any {
	return map[string]any{"object": "list", "url": url, "data": data, "has_more": hasMore}
}

func textList(values []any) ([]string, bool) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

func (s *Service) listWebhookEndpoints(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	limit := intParam(form, "limit", defaultListLimit, maxListLimit)
	acct.mu.Lock()
	defer acct.mu.Unlock()
	data := make([]map[string]any, 0, len(acct.endpoints))
	for i := len(acct.endpoints) - 1; i >= 0 && len(data) < limit; i-- {
		data = append(data, acct.endpoints[i].json())
	}
	writeJSON(w, http.StatusOK, list("/v1/webhook_endpoints", data, len(acct.endpoints) > limit))
}

func (s *Service) createWebhookEndpoint(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	url := stringParam(form, "url")
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		badRequest(w, "url must be an http or https address.")
		return
	}
	events, ok := textList(listParam(form, "enabled_events"))
	if !ok || len(events) == 0 {
		badRequest(w, "enabled_events must list at least one event type.")
		return
	}
	acct.mu.Lock()
	defer acct.mu.Unlock()
	if len(acct.endpoints) >= maxWebhookEndpoints {
		badRequest(w, "You have reached the maximum of 16 test webhook endpoints.")
		return
	}
	e := &endpoint{
		ID: newID("we"), URL: url, EnabledEvents: events, Secret: "whsec_" + strings.TrimPrefix(newID("s"), "s_test_"),
		Description: stringParam(form, "description"), Metadata: stringMap(objectParam(form, "metadata")),
		APIVersion: stringParam(form, "api_version"), Created: s.now().Unix(),
	}
	acct.endpoints = append(acct.endpoints, e)
	body := e.json()
	body["secret"] = e.Secret
	writeJSON(w, http.StatusOK, body)
}

func (a *account) endpoint(id string) *endpoint {
	for _, e := range a.endpoints {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (s *Service) updateWebhookEndpoint(acct *account, w http.ResponseWriter, r *http.Request, form map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	e := acct.endpoint(id)
	if e == nil {
		missing(w, "webhook endpoint", id)
		return
	}
	if url, given := form["url"]; given {
		text, _ := url.(string)
		if !strings.HasPrefix(text, "http://") && !strings.HasPrefix(text, "https://") {
			badRequest(w, "url must be an http or https address.")
			return
		}
		e.URL = text
	}
	if _, given := form["enabled_events"]; given {
		events, ok := textList(listParam(form, "enabled_events"))
		if !ok || len(events) == 0 {
			badRequest(w, "enabled_events must list at least one event type.")
			return
		}
		e.EnabledEvents = events
	}
	if disabled, given := boolParam(form, "disabled"); given {
		e.Disabled = disabled
	}
	if description, given := form["description"].(string); given {
		e.Description = description
	}
	writeJSON(w, http.StatusOK, e.json())
}

func (s *Service) deleteWebhookEndpoint(acct *account, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	for i, e := range acct.endpoints {
		if e.ID == id {
			acct.endpoints = append(acct.endpoints[:i], acct.endpoints[i+1:]...)
			writeJSON(w, http.StatusOK, map[string]any{"id": id, "object": "webhook_endpoint", "deleted": true})
			return
		}
	}
	missing(w, "webhook endpoint", id)
}

func (s *Service) createCustomer(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	c := &customer{
		ID: newID("cus"), Name: stringParam(form, "name"), Email: stringParam(form, "email"),
		Metadata: stringMap(objectParam(form, "metadata")), Created: s.now().Unix(),
	}
	acct.mu.Lock()
	c.Order = acct.next()
	acct.customers[c.ID] = c
	acct.mu.Unlock()
	writeJSON(w, http.StatusOK, c.json())
}

// checkoutItems reads a Checkout Session's line items: every price must be in the account, in one
// currency and interval, and a fixed price needs a quantity while a usage price has none.
func checkoutItems(acct *account, w http.ResponseWriter, form map[string]any) ([]lineItem, bool) {
	raw := listParam(form, "line_items")
	if len(raw) == 0 {
		badRequest(w, "line_items must list at least one price.")
		return nil, false
	}
	items := make([]lineItem, 0, len(raw))
	var first *PriceSpec
	for _, entry := range raw {
		fields, _ := entry.(map[string]any)
		priceID := stringParam(fields, "price")
		price := acct.prices[priceID]
		if price == nil {
			missingParam(w, "price", priceID)
			return nil, false
		}
		if price.archived {
			badRequest(w, "The price specified is inactive. This field only accepts active prices.")
			return nil, false
		}
		if first == nil {
			first = price
		} else if price.Currency != first.Currency || price.Interval != first.Interval {
			badRequest(w, "All prices of a subscription must have the same currency and interval.")
			return nil, false
		}
		item := lineItem{Price: priceID}
		quantityText, hasQuantity := fields["quantity"].(string)
		switch {
		case price.Metered && hasQuantity:
			badRequest(w, "Quantity should not be specified where usage_type is metered.")
			return nil, false
		case !price.Metered && !hasQuantity:
			badRequest(w, "Quantity is required for a price that is not metered.")
			return nil, false
		case hasQuantity:
			quantity, err := strconv.ParseInt(quantityText, 10, 64)
			if err != nil || quantity < 1 {
				badRequest(w, "Quantity must be a positive whole number.")
				return nil, false
			}
			item.Quantity = &quantity
		}
		items = append(items, item)
	}
	return items, true
}

func (s *Service) createCheckoutSession(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	if stringParam(form, "mode") != "subscription" {
		badRequest(w, "mode must be subscription.")
		return
	}
	acct.mu.Lock()
	defer acct.mu.Unlock()
	customerID := stringParam(form, "customer")
	if acct.customers[customerID] == nil {
		missingParam(w, "customer", customerID)
		return
	}
	items, ok := checkoutItems(acct, w, form)
	if !ok {
		return
	}
	id := newID("cs")
	sess := &session{
		ID: id, ClientSecret: id + "_secret_" + strings.TrimPrefix(newID("s"), "s_test_"),
		Status: "open", PaymentStatus: "unpaid", Customer: customerID,
		ClientReferenceID: stringParam(form, "client_reference_id"), Metadata: stringMap(objectParam(form, "metadata")),
		SubscriptionMetadata: stringMap(objectParam(objectParam(form, "subscription_data"), "metadata")),
		LineItems:            items, ReturnURL: stringParam(form, "return_url"), Created: s.now().Unix(),
		Order: acct.next(),
	}
	acct.sessions[id] = sess
	writeJSON(w, http.StatusOK, sess.json())
}

func (s *Service) getCheckoutSession(acct *account, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	sess := acct.sessions[id]
	if sess == nil {
		missing(w, "checkout.session", id)
		return
	}
	writeJSON(w, http.StatusOK, sess.json())
}

func (s *Service) expireCheckoutSession(acct *account, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	sess := acct.sessions[id]
	if sess == nil {
		acct.mu.Unlock()
		missing(w, "checkout.session", id)
		return
	}
	if sess.Status != "open" {
		acct.mu.Unlock()
		badRequest(w, "Only Checkout Sessions with a status of open can be expired.")
		return
	}
	sess.Status = "expired"
	body := sess.json()
	sends := acct.newEvents(s.now(), eventSpec{"checkout.session.expired", body})
	acct.mu.Unlock()
	s.sendLater(acct, sends)
	writeJSON(w, http.StatusOK, body)
}

func (s *Service) getSubscription(acct *account, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	sub := acct.subscriptions[id]
	if sub == nil {
		missing(w, "subscription", id)
		return
	}
	writeJSON(w, http.StatusOK, sub.json(acct.prices))
}

func (s *Service) updateSubscription(acct *account, w http.ResponseWriter, r *http.Request, form map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	sub := acct.subscriptions[id]
	if sub == nil {
		acct.mu.Unlock()
		missing(w, "subscription", id)
		return
	}
	if sub.Status == "canceled" {
		acct.mu.Unlock()
		badRequest(w, "A canceled subscription can only update its cancellation_details and metadata.")
		return
	}
	if cancel, given := boolParam(form, "cancel_at_period_end"); given {
		sub.CancelAtPeriodEnd = cancel
		sub.CancellationReason = ""
		if cancel {
			sub.CancellationReason = "cancellation_requested"
		}
	}
	for key, value := range stringMap(objectParam(form, "metadata")) {
		if sub.Metadata == nil {
			sub.Metadata = map[string]string{}
		}
		sub.Metadata[key] = value
	}
	body := sub.json(acct.prices)
	sends := acct.newEvents(s.now(), eventSpec{"customer.subscription.updated", body})
	acct.mu.Unlock()
	s.sendLater(acct, sends)
	writeJSON(w, http.StatusOK, body)
}

func (s *Service) cancelSubscription(acct *account, w http.ResponseWriter, r *http.Request, form map[string]any) {
	id := r.PathValue("id")
	invoiceNow, _ := boolParam(form, "invoice_now")
	acct.mu.Lock()
	sub := acct.subscriptions[id]
	if sub == nil {
		acct.mu.Unlock()
		missing(w, "subscription", id)
		return
	}
	if sub.Status == "canceled" {
		acct.mu.Unlock()
		badRequest(w, "This subscription has already been canceled.")
		return
	}
	now := s.now()
	var specs []eventSpec
	if invoiceNow && sub.metered(acct.prices) {
		final := acct.issueInvoice(sub, now, invoiceOptions{status: "paid", usageOnly: true, periodEnd: now.Unix()})
		specs = append(specs, eventSpec{"invoice.paid", final.json(acct.prices)})
	}
	sub.cancel(now, "cancellation_requested")
	body := sub.json(acct.prices)
	specs = append(specs, eventSpec{"customer.subscription.deleted", body})
	sends := acct.newEvents(now, specs...)
	acct.mu.Unlock()
	s.sendLater(acct, sends)
	writeJSON(w, http.StatusOK, body)
}

// invoicesQuery reads the invoice list filters.
type invoicesQuery struct {
	subscription  string
	customer      string
	createdGte    int64
	createdLt     int64
	startingAfter string
	limit         int
}

func readInvoicesQuery(form map[string]any) (invoicesQuery, bool) {
	q := invoicesQuery{
		subscription: stringParam(form, "subscription"), customer: stringParam(form, "customer"),
		startingAfter: stringParam(form, "starting_after"), limit: intParam(form, "limit", defaultListLimit, maxListLimit),
	}
	created := objectParam(form, "created")
	for key, target := range map[string]*int64{"gte": &q.createdGte, "lt": &q.createdLt} {
		text, given := created[key].(string)
		if !given {
			continue
		}
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return q, false
		}
		*target = value
	}
	return q, true
}

func (q invoicesQuery) matches(inv *invoice) bool {
	switch {
	case q.subscription != "" && inv.Subscription != q.subscription:
		return false
	case q.customer != "" && inv.Customer != q.customer:
		return false
	case q.createdGte != 0 && inv.Created < q.createdGte:
		return false
	case q.createdLt != 0 && inv.Created >= q.createdLt:
		return false
	}
	return true
}

// newestFirst orders invoices by creation time, newest first; invoices created in the same second
// keep the reverse of the order they were added in.
func newestFirst(invoices []*invoice) []*invoice {
	sorted := append([]*invoice(nil), invoices...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Created != sorted[j].Created {
			return sorted[i].Created > sorted[j].Created
		}
		return sorted[i].Order > sorted[j].Order
	})
	return sorted
}

func (s *Service) listInvoices(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	q, ok := readInvoicesQuery(form)
	if !ok {
		badRequest(w, "created must hold whole numbers.")
		return
	}
	acct.mu.Lock()
	defer acct.mu.Unlock()
	if q.startingAfter != "" && acct.invoice(q.startingAfter) == nil {
		missingParam(w, "invoice", q.startingAfter)
		return
	}
	matching := make([]*invoice, 0, len(acct.invoices))
	for _, inv := range newestFirst(acct.invoices) {
		if q.matches(inv) {
			matching = append(matching, inv)
		}
	}
	if q.startingAfter != "" {
		start := len(matching)
		for i, inv := range matching {
			if inv.ID == q.startingAfter {
				start = i + 1
				break
			}
		}
		matching = matching[start:]
	}
	page := matching
	if len(page) > q.limit {
		page = page[:q.limit]
	}
	data := make([]map[string]any, 0, len(page))
	for _, inv := range page {
		data = append(data, inv.json(acct.prices))
	}
	writeJSON(w, http.StatusOK, list("/v1/invoices", data, len(matching) > q.limit))
}

func (a *account) invoice(id string) *invoice {
	for _, inv := range a.invoices {
		if inv.ID == id {
			return inv
		}
	}
	if preview := a.previews[id]; preview != nil {
		return preview
	}
	return nil
}

func (s *Service) getInvoice(acct *account, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	id := r.PathValue("id")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	inv := acct.invoice(id)
	if inv == nil || acct.previews[id] != nil {
		missing(w, "invoice", id)
		return
	}
	writeJSON(w, http.StatusOK, inv.json(acct.prices))
}

func (s *Service) listInvoiceLines(acct *account, w http.ResponseWriter, r *http.Request, form map[string]any) {
	id := r.PathValue("id")
	limit := intParam(form, "limit", defaultListLimit, maxListLimit)
	startingAfter := stringParam(form, "starting_after")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	inv := acct.invoice(id)
	if inv == nil {
		missing(w, "invoice", id)
		return
	}
	lines := inv.Lines
	if startingAfter != "" {
		start := -1
		for i, line := range lines {
			if line.ID == startingAfter {
				start = i + 1
				break
			}
		}
		if start < 0 {
			missingParam(w, "line item", startingAfter)
			return
		}
		lines = lines[start:]
	}
	page := lines
	if len(page) > limit {
		page = page[:limit]
	}
	data := make([]map[string]any, 0, len(page))
	for _, line := range page {
		data = append(data, line.json(acct.prices, inv.Currency))
	}
	writeJSON(w, http.StatusOK, list("/v1/invoices/"+id+"/lines", data, len(lines) > limit))
}

func (s *Service) previewInvoice(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	id := stringParam(form, "subscription")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	sub := acct.subscriptions[id]
	if sub == nil {
		missingParam(w, "subscription", id)
		return
	}
	if sub.Status == "canceled" {
		badRequest(w, "No upcoming invoices for a canceled subscription.")
		return
	}
	preview := acct.buildInvoice(sub, invoiceOptions{status: "draft", periodEnd: sub.PeriodEnd})
	preview.ID = "upcoming_in_" + strings.TrimPrefix(newID("x"), "x_test_")
	preview.Created = s.now().Unix()
	acct.keepPreview(preview)
	writeJSON(w, http.StatusOK, preview.json(acct.prices))
}

func (s *Service) createRefund(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	paymentIntent := stringParam(form, "payment_intent")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	var paid *invoice
	for _, inv := range acct.invoices {
		if paymentIntent != "" && inv.PaymentIntent == paymentIntent {
			paid = inv
			break
		}
	}
	if paid == nil {
		missingParam(w, "payment_intent", paymentIntent)
		return
	}
	for _, refund := range acct.refunds {
		if refund["payment_intent"] == paymentIntent {
			badRequest(w, "Charge has already been refunded.")
			return
		}
	}
	refund := map[string]any{
		"id": newID("re"), "object": "refund", "payment_intent": paymentIntent, "amount": paid.AmountPaid,
		"currency": paid.Currency, "status": "succeeded", "created": s.now().Unix(),
	}
	acct.refunds = append(acct.refunds, refund)
	writeJSON(w, http.StatusOK, refund)
}

func (s *Service) listPortalConfigurations(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	limit := intParam(form, "limit", defaultListLimit, maxListLimit)
	active, filtered := boolParam(form, "active")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	matching := make([]map[string]any, 0, len(acct.configs))
	for _, c := range acct.configs {
		if !filtered || c.Active == active {
			matching = append(matching, c.json())
		}
	}
	page := matching
	if len(page) > limit {
		page = page[:limit]
	}
	writeJSON(w, http.StatusOK, list("/v1/billing_portal/configurations", page, len(matching) > limit))
}

func (s *Service) createPortalConfiguration(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	features := objectParam(form, "features")
	if len(features) == 0 {
		badRequest(w, "features must say what customers can do on the billing page.")
		return
	}
	c := &portalConfig{
		ID: newID("bpc"), Active: true, Metadata: stringMap(objectParam(form, "metadata")), Features: features,
	}
	acct.mu.Lock()
	acct.configs = append(acct.configs, c)
	acct.mu.Unlock()
	writeJSON(w, http.StatusOK, c.json())
}

func (s *Service) createPortalSession(acct *account, w http.ResponseWriter, _ *http.Request, form map[string]any) {
	customerID := stringParam(form, "customer")
	configID := stringParam(form, "configuration")
	acct.mu.Lock()
	defer acct.mu.Unlock()
	if acct.customers[customerID] == nil {
		missingParam(w, "customer", customerID)
		return
	}
	if configID != "" {
		found := false
		for _, c := range acct.configs {
			found = found || (c.ID == configID && c.Active)
		}
		if !found {
			missingParam(w, "configuration", configID)
			return
		}
	}
	id := newID("bps")
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "object": "billing_portal.session", "customer": customerID, "configuration": nullable(configID),
		"return_url": nullable(stringParam(form, "return_url")), "url": "https://billing.stripe.test/p/session/" + id,
		"created": s.now().Unix(),
	})
}
