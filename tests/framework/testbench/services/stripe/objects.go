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
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tier is one tier of a tiered price. Amounts are in the currency's minor unit and may have a
// fraction.
type Tier struct {
	// UpTo is the last quantity of the tier; zero on the last tier, which has no end.
	UpTo int64
	// UnitAmount is charged per unit in the tier.
	UnitAmount float64
	// FlatAmount is charged once when the quantity reaches the tier.
	FlatAmount float64
}

// PriceSpec is a price in an account, as the Moesif catalog creates it in Stripe.
type PriceSpec struct {
	// ID is the price's id.
	ID string
	// Currency is an ISO 4217 code.
	Currency string
	// Interval is "month" or "year".
	Interval string
	// Metered is true for a price charged by reported usage, false for a fixed price.
	Metered bool
	// UnitAmount is charged per unit, in the currency's minor unit; it may have a fraction.
	UnitAmount float64
	// Tiers, when given, replace UnitAmount.
	Tiers []Tier
	// TiersMode is "volume" or "graduated" for a tiered price.
	TiersMode string
	// DivideBy, above one, charges per block of that many units, rounding up.
	DivideBy int64
	// Product names the catalog plan the price belongs to.
	Product string
	// Nickname is the price's name.
	Nickname string

	archived bool
}

func (p *PriceSpec) validate() error {
	switch {
	case p.ID == "":
		return errors.New("a price needs an id")
	case len(p.Currency) != 3:
		return errors.New("a price needs a three-letter currency")
	case p.Interval != "month" && p.Interval != "year":
		return errors.New(`a price's interval is "month" or "year"`)
	case p.UnitAmount < 0 || math.IsNaN(p.UnitAmount) || math.IsInf(p.UnitAmount, 0):
		return errors.New("a price's unit amount is a number that is not negative")
	case p.DivideBy < 0:
		return errors.New("a price's block size is not negative")
	case len(p.Tiers) > 0 && p.TiersMode != "volume" && p.TiersMode != "graduated":
		return errors.New(`a tiered price's mode is "volume" or "graduated"`)
	case len(p.Tiers) == 0 && p.TiersMode != "":
		return errors.New("a price with a tiers mode needs tiers")
	}
	var previous int64
	for i, tier := range p.Tiers {
		last := i == len(p.Tiers)-1
		switch {
		case last && tier.UpTo != 0:
			return errors.New("the last tier has no end")
		case !last && tier.UpTo <= previous:
			return errors.New("each tier ends after the one before it")
		case tier.UnitAmount < 0 || tier.FlatAmount < 0:
			return errors.New("a tier's amounts are not negative")
		}
		previous = tier.UpTo
	}
	return nil
}

// amount is what the price charges for a quantity, in the currency's minor unit.
func (p *PriceSpec) amount(quantity int64) int64 {
	if p.DivideBy > 1 {
		quantity = (quantity + p.DivideBy - 1) / p.DivideBy
	}
	if len(p.Tiers) == 0 {
		return int64(math.Round(float64(quantity) * p.UnitAmount))
	}
	if p.TiersMode == "volume" {
		for _, tier := range p.Tiers {
			if tier.UpTo == 0 || quantity <= tier.UpTo {
				return int64(math.Round(float64(quantity)*tier.UnitAmount + tier.FlatAmount))
			}
		}
	}
	var total float64
	var previous int64
	for _, tier := range p.Tiers {
		if quantity <= previous {
			break
		}
		upper := quantity
		if tier.UpTo != 0 && tier.UpTo < upper {
			upper = tier.UpTo
		}
		total += float64(upper-previous)*tier.UnitAmount + tier.FlatAmount
		previous = upper
	}
	return int64(math.Round(total))
}

func (p *PriceSpec) json() map[string]any {
	usage := "licensed"
	if p.Metered {
		usage = "metered"
	}
	body := map[string]any{
		"id": p.ID, "object": "price", "currency": p.Currency, "active": !p.archived, "billing_scheme": "per_unit",
		"product": nullable(p.Product), "nickname": nullable(p.Nickname),
		"recurring":           map[string]any{"interval": p.Interval, "interval_count": 1, "usage_type": usage},
		"unit_amount":         nil,
		"unit_amount_decimal": strconv.FormatFloat(p.UnitAmount, 'f', -1, 64),
	}
	if p.UnitAmount == math.Trunc(p.UnitAmount) {
		body["unit_amount"] = int64(p.UnitAmount)
	}
	if len(p.Tiers) > 0 {
		tiers := make([]map[string]any, 0, len(p.Tiers))
		for _, tier := range p.Tiers {
			tiers = append(tiers, map[string]any{
				"up_to": nullableCount(tier.UpTo), "unit_amount_decimal": strconv.FormatFloat(tier.UnitAmount, 'f', -1, 64),
				"flat_amount_decimal": strconv.FormatFloat(tier.FlatAmount, 'f', -1, 64),
			})
		}
		body["billing_scheme"], body["tiers_mode"], body["tiers"] = "tiered", p.TiersMode, tiers
		body["unit_amount"], body["unit_amount_decimal"] = nil, nil
	}
	if p.DivideBy > 1 {
		body["transform_quantity"] = map[string]any{"divide_by": p.DivideBy, "round": "up"}
	}
	return body
}

// AddPrice adds a price to the account of the partition key, replacing a price with the same id.
func (s *Service) AddPrice(key string, spec PriceSpec) error {
	if key == "" {
		return errors.New("stripe: a price needs a partition key")
	}
	if err := spec.validate(); err != nil {
		return fmt.Errorf("stripe: %w", err)
	}
	spec.Currency = strings.ToLower(spec.Currency)
	spec.Tiers = append([]Tier(nil), spec.Tiers...)
	spec.archived = false
	acct := s.account(key)
	acct.mu.Lock()
	defer acct.mu.Unlock()
	acct.prices[spec.ID] = &spec
	return nil
}

// ArchivePrice archives a price of the account of the partition key, so no new subscription can
// use it.
func (s *Service) ArchivePrice(key, priceID string) error {
	if key == "" {
		return errors.New("stripe: archiving a price needs a partition key")
	}
	acct := s.account(key)
	acct.mu.Lock()
	defer acct.mu.Unlock()
	price := acct.prices[priceID]
	if price == nil {
		return fmt.Errorf("stripe: no such price: %q", priceID)
	}
	price.archived = true
	return nil
}

// RecordUsage adds a quantity to the current period's usage of a subscription's metered price, as
// Moesif reports the usage its billing meters count. The subscription is found by its id or by a
// value in its metadata; its id is returned.
func (s *Service) RecordUsage(key, subscriptionRef, priceID string, quantity int64) (string, error) {
	if key == "" {
		return "", errors.New("stripe: usage needs a partition key")
	}
	if quantity < 0 {
		return "", errors.New("stripe: a usage quantity is not negative")
	}
	acct := s.account(key)
	acct.mu.Lock()
	defer acct.mu.Unlock()
	sub := acct.findSubscription(subscriptionRef)
	if sub == nil {
		return "", fmt.Errorf("stripe: no such subscription: %q", subscriptionRef)
	}
	if sub.Status == "canceled" {
		return "", fmt.Errorf("stripe: the subscription %q has ended", sub.ID)
	}
	for _, item := range sub.Items {
		if item.Price == priceID && acct.prices[priceID] != nil && acct.prices[priceID].Metered {
			sub.Usage[priceID] += quantity
			return sub.ID, nil
		}
	}
	return "", fmt.Errorf("stripe: the subscription %q has no metered price %q", sub.ID, priceID)
}

const maxRetainedPreviews = 64

type account struct {
	mu            sync.Mutex
	customers     map[string]*customer
	prices        map[string]*PriceSpec
	sessions      map[string]*session
	subscriptions map[string]*subscription
	invoices      []*invoice
	previews      map[string]*invoice
	previewOrder  []string
	endpoints     []*endpoint
	configs       []*portalConfig
	refunds       []map[string]any
	events        []*event
	requests      []requestRecord
	failures      []*failure
	sequence      int64
}

func newAccount() *account {
	return &account{
		customers:     map[string]*customer{},
		prices:        map[string]*PriceSpec{},
		sessions:      map[string]*session{},
		subscriptions: map[string]*subscription{},
		previews:      map[string]*invoice{},
	}
}

type requestRecord struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query,omitempty"`
}

func (a *account) record(method, path, query string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, requestRecord{Method: method, Path: path, Query: query})
	if len(a.requests) > maxRetainedRequests {
		a.requests = a.requests[len(a.requests)-maxRetainedRequests:]
	}
}

// takeFailure returns the first failure matching the request and uses one of its times.
func (a *account) takeFailure(method, path, query string, body []byte) *failure {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, f := range a.failures {
		if !f.matches(method, path, query, body) {
			continue
		}
		taken := *f
		f.Times--
		if f.Times <= 0 {
			a.failures = append(a.failures[:i], a.failures[i+1:]...)
		}
		return &taken
	}
	return nil
}

// next returns an increasing number, so objects created in the same second keep their order.
func (a *account) next() int64 {
	a.sequence++
	return a.sequence
}

func (a *account) keepPreview(preview *invoice) {
	a.previews[preview.ID] = preview
	a.previewOrder = append(a.previewOrder, preview.ID)
	if len(a.previewOrder) > maxRetainedPreviews {
		delete(a.previews, a.previewOrder[0])
		a.previewOrder = a.previewOrder[1:]
	}
}

type customer struct {
	ID       string
	Name     string
	Email    string
	Metadata map[string]string
	Created  int64
	Order    int64
}

func (c *customer) json() map[string]any {
	return map[string]any{
		"id": c.ID, "object": "customer", "name": nullable(c.Name), "email": nullable(c.Email),
		"metadata": c.Metadata, "created": c.Created,
	}
}

type lineItem struct {
	Price    string
	Quantity *int64
}

type session struct {
	ID                   string
	ClientSecret         string
	Status               string
	PaymentStatus        string
	Customer             string
	ClientReferenceID    string
	Metadata             map[string]string
	SubscriptionMetadata map[string]string
	LineItems            []lineItem
	Subscription         string
	ReturnURL            string
	Created              int64
	Order                int64
}

func (s *session) json() map[string]any {
	items := make([]map[string]any, 0, len(s.LineItems))
	for _, item := range s.LineItems {
		entry := map[string]any{"price": map[string]any{"id": item.Price}}
		if item.Quantity != nil {
			entry["quantity"] = *item.Quantity
		}
		items = append(items, entry)
	}
	return map[string]any{
		"id": s.ID, "object": "checkout.session", "mode": "subscription", "ui_mode": "embedded",
		"client_secret": s.ClientSecret, "status": s.Status, "payment_status": s.PaymentStatus,
		"customer": s.Customer, "client_reference_id": nullable(s.ClientReferenceID), "metadata": s.Metadata,
		"subscription": nullable(s.Subscription), "return_url": nullable(s.ReturnURL), "created": s.Created,
		"line_items": map[string]any{"object": "list", "data": items, "has_more": false},
	}
}

type subscriptionItem struct {
	ID       string
	Price    string
	Quantity *int64
}

type subscription struct {
	ID                 string
	Customer           string
	Status             string
	Items              []subscriptionItem
	Usage              map[string]int64
	PeriodStart        int64
	PeriodEnd          int64
	CancelAtPeriodEnd  bool
	CanceledAt         int64
	EndedAt            int64
	CancellationReason string
	Metadata           map[string]string
	LatestInvoice      string
	Created            int64
	Order              int64
}

func (s *subscription) json(prices map[string]*PriceSpec) map[string]any {
	items := make([]map[string]any, 0, len(s.Items))
	for _, item := range s.Items {
		entry := map[string]any{"id": item.ID, "object": "subscription_item", "price": priceJSON(prices, item.Price)}
		if item.Quantity != nil {
			entry["quantity"] = *item.Quantity
		}
		items = append(items, entry)
	}
	return map[string]any{
		"id": s.ID, "object": "subscription", "customer": s.Customer, "status": s.Status,
		"current_period_start": s.PeriodStart, "current_period_end": s.PeriodEnd,
		"cancel_at_period_end": s.CancelAtPeriodEnd, "cancel_at": nil,
		"canceled_at": nullableCount(s.CanceledAt), "ended_at": nullableCount(s.EndedAt),
		"cancellation_details": map[string]any{"reason": nullable(s.CancellationReason)},
		"metadata":             s.Metadata, "latest_invoice": nullable(s.LatestInvoice), "created": s.Created,
		"items": map[string]any{"object": "list", "data": items, "has_more": false},
	}
}

func (s *subscription) metered(prices map[string]*PriceSpec) bool {
	for _, item := range s.Items {
		if price := prices[item.Price]; price != nil && price.Metered {
			return true
		}
	}
	return false
}

func (s *subscription) interval(prices map[string]*PriceSpec) string {
	for _, item := range s.Items {
		if price := prices[item.Price]; price != nil {
			return price.Interval
		}
	}
	return "month"
}

func (s *subscription) cancel(now time.Time, reason string) {
	s.Status = "canceled"
	s.CanceledAt = now.Unix()
	s.EndedAt = now.Unix()
	s.CancellationReason = reason
}

type invoiceLine struct {
	ID          string
	Price       string
	Description string
	Quantity    int64
	Amount      int64
}

type invoice struct {
	ID                   string
	Number               string
	Customer             string
	CustomerName         string
	CustomerEmail        string
	Subscription         string
	SubscriptionMetadata map[string]string
	Currency             string
	Total                int64
	AmountPaid           int64
	Status               string
	AttemptCount         int64
	NextPaymentAttempt   int64
	PaymentIntent        string
	PeriodStart          int64
	PeriodEnd            int64
	Lines                []invoiceLine
	Created              int64
	Order                int64
}

// embeddedLines is how many lines an invoice carries; the rest are read page by page.
const embeddedLines = 10

func (inv *invoice) json(prices map[string]*PriceSpec) map[string]any {
	lines := make([]map[string]any, 0, embeddedLines)
	for i, line := range inv.Lines {
		if i == embeddedLines {
			break
		}
		lines = append(lines, line.json(prices, inv.Currency))
	}
	return map[string]any{
		"id": inv.ID, "object": "invoice", "number": nullable(inv.Number), "customer": inv.Customer,
		"customer_name": nullable(inv.CustomerName), "customer_email": nullable(inv.CustomerEmail),
		"subscription":         nullable(inv.Subscription),
		"subscription_details": map[string]any{"metadata": inv.SubscriptionMetadata},
		"currency":             inv.Currency, "total": inv.Total, "amount_due": inv.Total, "amount_paid": inv.AmountPaid,
		"status": inv.Status, "attempt_count": inv.AttemptCount,
		"next_payment_attempt": nullableCount(inv.NextPaymentAttempt), "payment_intent": nullable(inv.PaymentIntent),
		"period_start": inv.PeriodStart, "period_end": inv.PeriodEnd, "created": inv.Created,
		"hosted_invoice_url": "https://invoice.stripe.test/i/" + inv.ID,
		"invoice_pdf":        "https://invoice.stripe.test/i/" + inv.ID + "/pdf",
		"lines":              map[string]any{"object": "list", "data": lines, "has_more": len(inv.Lines) > embeddedLines},
	}
}

func (line invoiceLine) json(prices map[string]*PriceSpec, currency string) map[string]any {
	return map[string]any{
		"id": line.ID, "object": "line_item", "price": priceJSON(prices, line.Price),
		"description": line.Description, "quantity": line.Quantity, "amount": line.Amount, "currency": currency,
	}
}

func priceJSON(prices map[string]*PriceSpec, id string) map[string]any {
	if price := prices[id]; price != nil {
		return price.json()
	}
	return map[string]any{"id": id, "object": "price"}
}

// invoiceOptions says what kind of invoice to build for a subscription.
type invoiceOptions struct {
	// status is draft, open or paid.
	status string
	// usageOnly leaves out the fixed prices: the final invoice of a subscription ended early.
	usageOnly bool
	// periodEnd ends the period the invoice covers.
	periodEnd int64
	// usage replaces the subscription's recorded usage, by price.
	usage map[string]int64
}

// buildInvoice is an invoice for a subscription's prices: each fixed price at its quantity, each
// usage price at the usage recorded in the period.
func (a *account) buildInvoice(sub *subscription, opts invoiceOptions) *invoice {
	usage := sub.Usage
	if opts.usage != nil {
		usage = opts.usage
	}
	inv := &invoice{
		Customer: sub.Customer, Subscription: sub.ID, SubscriptionMetadata: copyMap(sub.Metadata),
		Status: opts.status, PeriodStart: sub.PeriodStart, PeriodEnd: opts.periodEnd,
	}
	if c := a.customers[sub.Customer]; c != nil {
		inv.CustomerName, inv.CustomerEmail = c.Name, c.Email
	}
	for _, item := range sub.Items {
		price := a.prices[item.Price]
		if price == nil || (opts.usageOnly && !price.Metered) {
			continue
		}
		quantity := usage[item.Price]
		if !price.Metered {
			quantity = 1
			if item.Quantity != nil {
				quantity = *item.Quantity
			}
		}
		name := price.Nickname
		if name == "" {
			name = price.Product
		}
		line := invoiceLine{
			ID: newID("il"), Price: price.ID, Quantity: quantity, Amount: price.amount(quantity),
			Description: strconv.FormatInt(quantity, 10) + " × " + name,
		}
		inv.Currency = price.Currency
		inv.Total += line.Amount
		inv.Lines = append(inv.Lines, line)
	}
	return inv
}

// issueInvoice builds a subscription's invoice and adds it to the account as its latest one. A
// paid invoice has a payment when it charges anything; an open one has had one failed attempt.
func (a *account) issueInvoice(sub *subscription, now time.Time, opts invoiceOptions) *invoice {
	inv := a.buildInvoice(sub, opts)
	inv.ID = newID("in")
	inv.Order = a.next()
	inv.Number = fmt.Sprintf("TEST-%04d", inv.Order)
	inv.Created = now.Unix()
	inv.AttemptCount = 1
	switch opts.status {
	case "paid":
		inv.pay()
	case "open":
		inv.NextPaymentAttempt = now.Add(72 * time.Hour).Unix()
	}
	a.invoices = append(a.invoices, inv)
	sub.LatestInvoice = inv.ID
	return inv
}

func (inv *invoice) pay() {
	inv.Status = "paid"
	inv.AmountPaid = inv.Total
	inv.NextPaymentAttempt = 0
	if inv.Total > 0 && inv.PaymentIntent == "" {
		inv.PaymentIntent = newID("pi")
	}
}

type endpoint struct {
	ID            string
	URL           string
	EnabledEvents []string
	Disabled      bool
	Secret        string
	Description   string
	Metadata      map[string]string
	APIVersion    string
	Created       int64
}

func (e *endpoint) json() map[string]any {
	status := "enabled"
	if e.Disabled {
		status = "disabled"
	}
	return map[string]any{
		"id": e.ID, "object": "webhook_endpoint", "url": e.URL, "enabled_events": e.EnabledEvents,
		"status": status, "description": nullable(e.Description), "metadata": e.Metadata,
		"api_version": nullable(e.APIVersion), "created": e.Created,
	}
}

func (e *endpoint) receives(eventType string) bool {
	if e.Disabled {
		return false
	}
	for _, enabled := range e.EnabledEvents {
		if enabled == "*" || enabled == eventType {
			return true
		}
	}
	return false
}

type portalConfig struct {
	ID       string
	Active   bool
	Metadata map[string]string
	Features map[string]any
}

func (c *portalConfig) json() map[string]any {
	return map[string]any{
		"id": c.ID, "object": "billing_portal.configuration", "active": c.Active,
		"metadata": c.Metadata, "features": c.Features,
	}
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableCount(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func copyMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

// periodEnd is one interval after start.
func periodEnd(start time.Time, interval string) time.Time {
	if interval == "year" {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}
