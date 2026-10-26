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
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

// retryDelay is when Stripe tries a failed payment again.
const retryDelay = 72 * time.Hour

// decode reads a control body into dst; an empty body leaves dst as it is. Unknown fields are
// refused, so a misspelled field fails the call instead of being ignored.
func decode(w http.ResponseWriter, body []byte, dst any) bool {
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "stripe: control request body: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// findSession finds a Checkout Session by its id or, for the newest one, by its client reference.
func (a *account) findSession(id string) *session {
	if sess := a.sessions[id]; sess != nil {
		return sess
	}
	var found *session
	for _, sess := range a.sessions {
		if sess.ClientReferenceID == id && (found == nil || sess.Order > found.Order) {
			found = sess
		}
	}
	return found
}

// findSubscription finds a subscription by its id or, for the newest one, by a value in its
// metadata, such as the checkout reference the portal puts there.
func (a *account) findSubscription(ref string) *subscription {
	if sub := a.subscriptions[ref]; sub != nil {
		return sub
	}
	var found *subscription
	for _, sub := range a.subscriptions {
		for _, value := range sub.Metadata {
			if value == ref && ref != "" && (found == nil || sub.Order > found.Order) {
				found = sub
			}
		}
	}
	return found
}

// completeCheckout pays an open Checkout Session as the customer would: the subscription starts
// with its first invoice paid, and the events are delivered before the call answers.
func (s *Service) completeCheckout(acct *account, w http.ResponseWriter, r *http.Request, _ []byte) {
	id := r.PathValue("id")
	acct.mu.Lock()
	sess := acct.findSession(id)
	if sess == nil {
		acct.mu.Unlock()
		http.Error(w, "stripe: no such checkout session: "+id, http.StatusNotFound)
		return
	}
	if sess.Status != "open" {
		acct.mu.Unlock()
		http.Error(w, "stripe: the checkout session is "+sess.Status, http.StatusConflict)
		return
	}
	now := s.now()
	sub := &subscription{
		ID: newID("sub"), Customer: sess.Customer, Status: "active", Usage: map[string]int64{},
		PeriodStart: now.Unix(), Metadata: copyMap(sess.SubscriptionMetadata), Created: now.Unix(), Order: acct.next(),
	}
	for _, item := range sess.LineItems {
		sub.Items = append(sub.Items, subscriptionItem{ID: newID("si"), Price: item.Price, Quantity: item.Quantity})
	}
	sub.PeriodEnd = periodEnd(now, sub.interval(acct.prices)).Unix()
	acct.subscriptions[sub.ID] = sub
	inv := acct.issueInvoice(sub, now, invoiceOptions{status: "paid", periodEnd: sub.PeriodEnd})
	sess.Status, sess.Subscription, sess.PaymentStatus = "complete", sub.ID, "paid"
	if inv.Total == 0 {
		sess.PaymentStatus = "no_payment_required"
	}
	sends := acct.newEvents(now,
		eventSpec{"customer.subscription.created", sub.json(acct.prices)},
		eventSpec{"checkout.session.completed", sess.json()},
		eventSpec{"invoice.paid", inv.json(acct.prices)})
	acct.mu.Unlock()
	deliveries := s.deliver(acct, sends)
	writeJSON(w, http.StatusOK, map[string]any{
		"checkoutSession": sess.ID, "subscription": sub.ID, "invoice": inv.ID, "deliveries": deliveries,
	})
}

// abandonCheckout ends an open Checkout Session as Stripe does when its customer leaves it unpaid,
// and delivers the event before the call answers.
func (s *Service) abandonCheckout(acct *account, w http.ResponseWriter, r *http.Request, _ []byte) {
	id := r.PathValue("id")
	acct.mu.Lock()
	sess := acct.findSession(id)
	if sess == nil {
		acct.mu.Unlock()
		http.Error(w, "stripe: no such checkout session: "+id, http.StatusNotFound)
		return
	}
	if sess.Status != "open" {
		acct.mu.Unlock()
		http.Error(w, "stripe: the checkout session is "+sess.Status, http.StatusConflict)
		return
	}
	sess.Status = "expired"
	sends := acct.newEvents(s.now(), eventSpec{"checkout.session.expired", sess.json()})
	acct.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"checkoutSession": sess.ID, "deliveries": s.deliver(acct, sends)})
}

type statusRequest struct {
	// Status is past_due, unpaid, active or canceled.
	Status string `json:"status"`
	// RetriesEnded, with past_due, leaves the failed invoice with no further attempt.
	RetriesEnded bool `json:"retriesEnded"`
	// Reason, with canceled, is the cancellation reason; payment_failed when left out.
	Reason string `json:"reason"`
}

// setSubscriptionStatus changes a subscription's payment status as Stripe would: a renewal payment
// fails, Stripe stops retrying, the customer pays, or Stripe ends the subscription.
func (s *Service) setSubscriptionStatus(acct *account, w http.ResponseWriter, r *http.Request, body []byte) {
	var req statusRequest
	if !decode(w, body, &req) {
		return
	}
	id := r.PathValue("id")
	acct.mu.Lock()
	sub := acct.findSubscription(id)
	if sub == nil {
		acct.mu.Unlock()
		http.Error(w, "stripe: no such subscription: "+id, http.StatusNotFound)
		return
	}
	if sub.Status == "canceled" {
		acct.mu.Unlock()
		http.Error(w, "stripe: the subscription has ended", http.StatusConflict)
		return
	}
	now := s.now()
	var specs []eventSpec
	switch req.Status {
	case "past_due", "unpaid":
		inv := acct.openInvoice(sub, now)
		inv.NextPaymentAttempt = now.Add(retryDelay).Unix()
		if req.Status == "unpaid" || req.RetriesEnded {
			inv.NextPaymentAttempt = 0
		}
		sub.Status = req.Status
		specs = append(specs, eventSpec{"invoice.payment_failed", inv.json(acct.prices)})
	case "active":
		for _, inv := range acct.invoices {
			if inv.Subscription == sub.ID && inv.Status == "open" {
				inv.pay()
				specs = append(specs, eventSpec{"invoice.paid", inv.json(acct.prices)})
			}
		}
		sub.Status = "active"
	case "canceled":
		reason := req.Reason
		if reason == "" {
			reason = "payment_failed"
		}
		sub.cancel(now, reason)
	default:
		acct.mu.Unlock()
		http.Error(w, "stripe: status is past_due, unpaid, active or canceled", http.StatusBadRequest)
		return
	}
	kind := "customer.subscription.updated"
	if sub.Status == "canceled" {
		kind = "customer.subscription.deleted"
	}
	specs = append(specs, eventSpec{kind, sub.json(acct.prices)})
	sends := acct.newEvents(now, specs...)
	acct.mu.Unlock()
	deliveries := s.deliver(acct, sends)
	writeJSON(w, http.StatusOK, map[string]any{"subscription": sub.ID, "deliveries": deliveries})
}

// openInvoice is the subscription's unpaid invoice, issued now when it has none.
func (a *account) openInvoice(sub *subscription, now time.Time) *invoice {
	for _, inv := range a.invoices {
		if inv.Subscription == sub.ID && inv.Status == "open" {
			inv.AttemptCount++
			return inv
		}
	}
	return a.issueInvoice(sub, now, invoiceOptions{status: "open", periodEnd: sub.PeriodEnd})
}

// endPeriod ends a subscription's period now. A subscription set to end at the period's end ends,
// with a final invoice for its usage; any other renews, with a paid invoice for the next period's
// fee and the past period's usage.
func (s *Service) endPeriod(acct *account, w http.ResponseWriter, r *http.Request, _ []byte) {
	id := r.PathValue("id")
	acct.mu.Lock()
	sub := acct.findSubscription(id)
	if sub == nil {
		acct.mu.Unlock()
		http.Error(w, "stripe: no such subscription: "+id, http.StatusNotFound)
		return
	}
	if sub.Status == "canceled" {
		acct.mu.Unlock()
		http.Error(w, "stripe: the subscription has ended", http.StatusConflict)
		return
	}
	now := s.now()
	var specs []eventSpec
	var issued *invoice
	if sub.CancelAtPeriodEnd {
		if sub.metered(acct.prices) {
			issued = acct.issueInvoice(sub, now, invoiceOptions{status: "paid", usageOnly: true, periodEnd: now.Unix()})
			specs = append(specs, eventSpec{"invoice.paid", issued.json(acct.prices)})
		}
		sub.cancel(now, "cancellation_requested")
		specs = append(specs, eventSpec{"customer.subscription.deleted", sub.json(acct.prices)})
	} else {
		issued = acct.issueInvoice(sub, now, invoiceOptions{status: "paid", periodEnd: now.Unix()})
		sub.Usage = map[string]int64{}
		sub.PeriodStart, sub.PeriodEnd = now.Unix(), periodEnd(now, sub.interval(acct.prices)).Unix()
		specs = append(specs, eventSpec{"invoice.paid", issued.json(acct.prices)},
			eventSpec{"customer.subscription.updated", sub.json(acct.prices)})
	}
	sends := acct.newEvents(now, specs...)
	acct.mu.Unlock()
	deliveries := s.deliver(acct, sends)
	result := map[string]any{"subscription": sub.ID, "deliveries": deliveries}
	if issued != nil {
		result["invoice"] = issued.ID
	}
	writeJSON(w, http.StatusOK, result)
}

type invoiceRequest struct {
	Subscription string           `json:"subscription"`
	Status       string           `json:"status"`
	Created      int64            `json:"created"`
	Usage        map[string]int64 `json:"usage"`
}

// addInvoice adds an invoice Stripe issued earlier for a subscription, such as one from a past
// month, without sending events.
func (s *Service) addInvoice(acct *account, w http.ResponseWriter, _ *http.Request, body []byte) {
	var req invoiceRequest
	if !decode(w, body, &req) {
		return
	}
	if req.Status != "paid" && req.Status != "open" {
		http.Error(w, "stripe: an added invoice is paid or open", http.StatusBadRequest)
		return
	}
	acct.mu.Lock()
	defer acct.mu.Unlock()
	sub := acct.findSubscription(req.Subscription)
	if sub == nil {
		http.Error(w, "stripe: no such subscription: "+req.Subscription, http.StatusNotFound)
		return
	}
	created := s.now()
	if req.Created != 0 {
		created = time.Unix(req.Created, 0)
	}
	usage := req.Usage
	if usage == nil {
		usage = map[string]int64{}
	}
	inv := acct.issueInvoice(sub, created, invoiceOptions{status: req.Status, periodEnd: created.Unix(), usage: usage})
	inv.PeriodStart = created.Unix()
	writeJSON(w, http.StatusOK, map[string]any{"invoice": inv.ID, "total": inv.Total})
}

type resendRequest struct {
	// Event is the event's id; Type, when no id is given, picks the newest event of that type.
	Event string `json:"event"`
	Type  string `json:"type"`
	// Signature is valid, invalid or stale; valid when left out.
	Signature string `json:"signature"`
}

// resendEvent delivers an event again to the endpoints that receive it now, with a valid, invalid
// or stale signature.
func (s *Service) resendEvent(acct *account, w http.ResponseWriter, _ *http.Request, body []byte) {
	req := resendRequest{Signature: signatureValid}
	if !decode(w, body, &req) {
		return
	}
	if req.Signature != signatureValid && req.Signature != signatureInvalid && req.Signature != signatureStale {
		http.Error(w, "stripe: signature is valid, invalid or stale", http.StatusBadRequest)
		return
	}
	if (req.Event == "") == (req.Type == "") {
		http.Error(w, "stripe: name the event or its type", http.StatusBadRequest)
		return
	}
	acct.mu.Lock()
	var found *event
	for _, e := range acct.events {
		if (req.Event != "" && e.ID == req.Event) || (req.Event == "" && e.Type == req.Type) {
			found = e
		}
	}
	if found == nil {
		acct.mu.Unlock()
		http.Error(w, "stripe: no such event", http.StatusNotFound)
		return
	}
	sends := acct.sendsOf(found, req.Signature)
	acct.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"event": found.ID, "deliveries": s.deliver(acct, sends)})
}

// addFailure makes matching Stripe calls fail with a status, a number of times.
func (s *Service) addFailure(acct *account, w http.ResponseWriter, _ *http.Request, body []byte) {
	var f failure
	if !decode(w, body, &f) {
		return
	}
	if err := f.validate(); err != nil {
		http.Error(w, "stripe: "+err.Error(), http.StatusBadRequest)
		return
	}
	acct.mu.Lock()
	acct.failures = append(acct.failures, &f)
	acct.mu.Unlock()
	writeJSON(w, http.StatusCreated, f)
}

func byOrder[T any](values map[string]T, order func(T) int64) []T {
	out := make([]T, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return order(out[i]) < order(out[j]) })
	return out
}

// state reports everything the account holds, oldest first, and the calls it was sent.
func (s *Service) state(acct *account, w http.ResponseWriter, _ *http.Request, _ []byte) {
	acct.mu.Lock()
	defer acct.mu.Unlock()
	customers := []map[string]any{}
	for _, c := range byOrder(acct.customers, func(c *customer) int64 { return c.Order }) {
		customers = append(customers, c.json())
	}
	priceIDs := make([]string, 0, len(acct.prices))
	for id := range acct.prices {
		priceIDs = append(priceIDs, id)
	}
	sort.Strings(priceIDs)
	prices := []map[string]any{}
	for _, id := range priceIDs {
		prices = append(prices, acct.prices[id].json())
	}
	sessions := []map[string]any{}
	for _, sess := range byOrder(acct.sessions, func(sess *session) int64 { return sess.Order }) {
		sessions = append(sessions, sess.json())
	}
	subscriptions := []map[string]any{}
	for _, sub := range byOrder(acct.subscriptions, func(sub *subscription) int64 { return sub.Order }) {
		body := sub.json(acct.prices)
		body["usage"] = sub.Usage
		subscriptions = append(subscriptions, body)
	}
	invoices := []map[string]any{}
	for _, inv := range acct.invoices {
		invoices = append(invoices, inv.json(acct.prices))
	}
	endpoints := []map[string]any{}
	for _, e := range acct.endpoints {
		endpoints = append(endpoints, e.json())
	}
	configs := []map[string]any{}
	for _, c := range acct.configs {
		configs = append(configs, c.json())
	}
	events := []map[string]any{}
	for _, e := range acct.events {
		events = append(events, e.json())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"customers": customers, "prices": prices, "checkoutSessions": sessions, "subscriptions": subscriptions,
		"invoices": invoices, "webhookEndpoints": endpoints, "billingPortalConfigurations": configs,
		"refunds": append([]map[string]any{}, acct.refunds...), "events": events,
		"requests": append([]requestRecord{}, acct.requests...), "failures": len(acct.failures),
	})
}
