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
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

const maxRetainedStripeEvents = 1024

type stripeEvent struct {
	Application string `json:"application"`
	ID          string `json:"id"`
	Type        string `json:"type"`
}

// receiveStripeEvent is Moesif's endpoint for Stripe events: it keeps each event's application,
// id and type.
func (s *Service) receiveStripeEvent(key string, w http.ResponseWriter, r *http.Request) {
	raw, ok := readBody(w, r)
	if !ok {
		return
	}
	var envelope struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	application := r.URL.Query().Get("moesif_application_id")
	if json.Unmarshal(raw, &envelope) != nil || envelope.ID == "" || envelope.Type == "" || application == "" {
		writeError(w, http.StatusBadRequest, "A Stripe event and its moesif_application_id are required.")
		return
	}
	app := s.application(key)
	app.mu.Lock()
	app.stripeEvents = append(app.stripeEvents, stripeEvent{Application: application, ID: envelope.ID, Type: envelope.Type})
	if len(app.stripeEvents) > maxRetainedStripeEvents {
		app.stripeEvents = app.stripeEvents[len(app.stripeEvents)-maxRetainedStripeEvents:]
	}
	app.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{})
}

// decode reads a control body into dst. Unknown fields are refused, so a misspelled field fails
// the call instead of being ignored.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	raw, ok := readBody(w, r)
	if !ok {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "moesif: control request body: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

type usageRequest struct {
	// Subscription is the Stripe subscription the calls are billed to: its id or a value in its
	// metadata, such as the portal's checkout reference.
	Subscription string `json:"subscription"`
	// Customer is the Stripe customer; any customer matches when left out.
	Customer string `json:"customer"`
	// Meter is the billing meter's id or name; the only meter when left out.
	Meter string `json:"meter"`
	// Value is the number the meter counted.
	Value int64 `json:"value"`
	// Time is when the calls were made; now when left out.
	Time *time.Time `json:"time"`
}

// findMeter is the meter with the id or name, or the only meter when ref is empty.
func (a *application) findMeter(ref string) *meter {
	if ref == "" {
		if len(a.meters) != 1 {
			return nil
		}
		for _, m := range a.meters {
			return m
		}
	}
	if m := a.meters[ref]; m != nil {
		return m
	}
	var found *meter
	for _, m := range a.meters {
		if m.Name == ref {
			if found != nil {
				return nil
			}
			found = m
		}
	}
	return found
}

// recordUsage records calls a billing meter counted for a subscription, as the gateway's events
// would, and reports them to Stripe against the meter's price.
func (s *Service) recordUsage(key string, w http.ResponseWriter, r *http.Request) {
	var req usageRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Subscription == "" || req.Value < 0 {
		http.Error(w, "moesif: usage needs a subscription and a value that is not negative", http.StatusBadRequest)
		return
	}
	at := s.now()
	if req.Time != nil {
		at = *req.Time
	}
	app := s.application(key)
	app.mu.Lock()
	defer app.mu.Unlock()
	m := app.findMeter(req.Meter)
	if m == nil {
		http.Error(w, "moesif: no single billing meter matches "+req.Meter, http.StatusNotFound)
		return
	}
	subscriptionID, err := s.billing.RecordUsage(key, req.Subscription, m.PriceID, req.Value)
	if err != nil {
		http.Error(w, "moesif: Stripe refused the usage: "+err.Error(), http.StatusConflict)
		return
	}
	app.records = append(app.records, usageRecord{
		Subscription: subscriptionID, Customer: req.Customer, Meter: m.ID, Time: at, Value: req.Value,
		signature: m.Signature,
	})
	if len(app.records) > maxRetainedRecords {
		app.records = app.records[len(app.records)-maxRetainedRecords:]
	}
	writeJSON(w, http.StatusCreated, map[string]any{"meter": m.ID, "price": m.PriceID, "subscription": subscriptionID})
}

// addFailure makes matching Moesif calls fail with a status, a number of times.
func (s *Service) addFailure(key string, w http.ResponseWriter, r *http.Request) {
	var f failure
	if !decode(w, r, &f) {
		return
	}
	if err := f.validate(); err != nil {
		http.Error(w, "moesif: "+err.Error(), http.StatusBadRequest)
		return
	}
	app := s.application(key)
	app.mu.Lock()
	app.failures = append(app.failures, &f)
	app.mu.Unlock()
	writeJSON(w, http.StatusCreated, f)
}

// state reports everything the application holds, oldest first, and the calls it was sent.
func (s *Service) state(key string, w http.ResponseWriter, _ *http.Request) {
	app := s.application(key)
	app.mu.Lock()
	defer app.mu.Unlock()
	plans := []map[string]any{}
	for _, p := range sortedByOrder(app.plans, func(p *plan) int64 { return p.Order }) {
		plans = append(plans, p.json())
	}
	prices := []map[string]any{}
	for _, p := range sortedByOrder(app.prices, func(p *price) int64 { return p.Order }) {
		prices = append(prices, p.json())
	}
	meters := []map[string]any{}
	for _, m := range sortedByOrder(app.meters, func(m *meter) int64 { return m.Order }) {
		meters = append(meters, m.json())
	}
	partners := map[string]any{}
	for provider, account := range app.partners {
		partners[provider] = account.json()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plans": plans, "prices": prices, "meters": meters, "partnerAccounts": partners,
		"usage":        append([]usageRecord{}, app.records...),
		"stripeEvents": append([]stripeEvent{}, app.stripeEvents...),
		"requests":     append([]requestRecord{}, app.requests...), "failures": len(app.failures),
	})
}
