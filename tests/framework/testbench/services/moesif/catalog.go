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
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench/services/stripe"
)

const (
	stripeProvider   = "stripe"
	billingMeterSlug = "hourly_usage"
)

var (
	stripeKeyPattern = regexp.MustCompile(`^(sk|rk)_test_[A-Za-z0-9]+$`)
	decimalPattern   = regexp.MustCompile(`^\d+(\.\d+)?$`)
)

// zeroDecimal and threeDecimal are the currencies whose minor unit is not a hundredth.
var (
	zeroDecimal = map[string]bool{
		"bif": true, "clp": true, "djf": true, "gnf": true, "jpy": true, "kmf": true, "krw": true, "mga": true,
		"pyg": true, "rwf": true, "ugx": true, "vnd": true, "vuv": true, "xaf": true, "xof": true, "xpf": true,
	}
	threeDecimal = map[string]bool{"bhd": true, "jod": true, "kwd": true, "omr": true, "tnd": true}
)

type application struct {
	mu           sync.Mutex
	plans        map[string]*plan
	prices       map[string]*price
	meters       map[string]*meter
	partners     map[string]*partnerAccount
	disconnected map[string]bool
	records      []usageRecord
	stripeEvents []stripeEvent
	requests     []requestRecord
	failures     []*failure
	sequence     int64
}

func newApplication() *application {
	return &application{
		plans: map[string]*plan{}, prices: map[string]*price{}, meters: map[string]*meter{},
		partners: map[string]*partnerAccount{}, disconnected: map[string]bool{},
	}
}

type requestRecord struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query,omitempty"`
}

func (a *application) record(method, path, query string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, requestRecord{Method: method, Path: path, Query: query})
	if len(a.requests) > maxRetainedRequests {
		a.requests = a.requests[len(a.requests)-maxRetainedRequests:]
	}
}

// takeFailure returns the first failure matching the request and uses one of its times.
func (a *application) takeFailure(method, path, query string, body []byte) *failure {
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

func (a *application) next() int64 {
	a.sequence++
	return a.sequence
}

func (a *application) connected() bool {
	account := a.partners[stripeProvider]
	return account != nil && account.State == "enabled"
}

type plan struct {
	ID              string
	Name            string
	Status          string
	BillingType     string
	ReportingPeriod string
	Created         time.Time
	Order           int64
}

func (p *plan) json() map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "provider": stripeProvider, "status": p.Status, "billing_type": p.BillingType,
		"reporting_period": p.ReportingPeriod, "created_at": p.Created.UTC().Format(time.RFC3339),
	}
}

type price struct {
	ID      string
	PlanID  string
	Status  string
	Payload map[string]any
	Created time.Time
	Order   int64
}

func (p *price) json() map[string]any {
	body := map[string]any{}
	for key, value := range p.Payload {
		body[key] = value
	}
	body["id"], body["status"], body["created_at"] = p.ID, p.Status, p.Created.UTC().Format(time.RFC3339)
	return body
}

type meter struct {
	ID        string
	Name      string
	PriceID   string
	Signature string
	Payload   map[string]any
	Created   time.Time
	Order     int64
}

func (m *meter) json() map[string]any {
	body := map[string]any{}
	for key, value := range m.Payload {
		body[key] = value
	}
	body["_id"], body["created_at"] = m.ID, m.Created.UTC().Format(time.RFC3339)
	return body
}

type partnerAccount struct {
	Version        string
	APIVersion     string
	State          string
	CompanyIDField string
}

func (p *partnerAccount) json() map[string]any {
	return map[string]any{
		"partner_account": map[string]any{
			"version": p.Version, "api_version": p.APIVersion, "credentials": map[string]any{"state": p.State},
		},
		"billing_config": map[string]any{stripeProvider: map[string]any{"company_id_field": p.CompanyIDField}},
	}
}

func text(body map[string]any, key string) string {
	value, _ := body[key].(string)
	return value
}

func object(body map[string]any, key string) map[string]any {
	value, _ := body[key].(map[string]any)
	return value
}

// providerOf checks the provider query parameter, which every catalog call names.
func providerOf(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("provider") != stripeProvider {
		writeError(w, http.StatusBadRequest, "provider must be stripe.")
		return false
	}
	return true
}

func sortedByOrder[T any](values map[string]T, order func(T) int64) []T {
	out := make([]T, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return order(out[i]) < order(out[j]) })
	return out
}

func (s *Service) listPlans(_ string, app *application, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	if !providerOf(w, r) {
		return
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	plans := []map[string]any{}
	for _, p := range sortedByOrder(app.plans, func(p *plan) int64 { return p.Order }) {
		plans = append(plans, p.json())
	}
	writeJSON(w, http.StatusOK, plans)
}

func (s *Service) createPlan(_ string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any) {
	if !providerOf(w, r) {
		return
	}
	switch {
	case text(body, "name") == "":
		writeError(w, http.StatusBadRequest, "name is required.")
		return
	case text(body, "provider") != stripeProvider || text(body, "status") != "active":
		writeError(w, http.StatusBadRequest, "A plan is created active, for the stripe provider.")
		return
	case text(body, "billing_type") == "" || text(body, "reporting_period") == "":
		writeError(w, http.StatusBadRequest, "billing_type and reporting_period are required.")
		return
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	if !app.connected() {
		writeError(w, http.StatusBadRequest, "No billing provider is connected for stripe.")
		return
	}
	p := &plan{
		ID: newID("prod_"), Name: text(body, "name"), Status: "active", BillingType: text(body, "billing_type"),
		ReportingPeriod: text(body, "reporting_period"), Created: s.now(), Order: app.next(),
	}
	app.plans[p.ID] = p
	writeJSON(w, http.StatusCreated, p.json())
}

// archiveRequest checks an archive call's body: the provider and the archived status.
func archiveRequest(w http.ResponseWriter, r *http.Request, body map[string]any) bool {
	if !providerOf(w, r) {
		return false
	}
	if text(body, "provider") != stripeProvider || text(body, "status") != "archived" {
		writeError(w, http.StatusBadRequest, "Only the archived status can be set.")
		return false
	}
	return true
}

func (s *Service) archivePlan(_ string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any) {
	if !archiveRequest(w, r, body) {
		return
	}
	id := r.PathValue("id")
	app.mu.Lock()
	defer app.mu.Unlock()
	p := app.plans[id]
	if p == nil {
		writeError(w, http.StatusBadRequest, "resource_missing: No such product: '"+id+"'")
		return
	}
	p.Status = "archived"
	writeJSON(w, http.StatusOK, p.json())
}

// minorUnits converts a decimal amount in the currency's major unit to its minor unit.
func minorUnits(amount, currency string) (float64, error) {
	if !decimalPattern.MatchString(amount) {
		return 0, fmt.Errorf("%q is not a decimal amount", amount)
	}
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0, err
	}
	scale := 100.0
	switch {
	case zeroDecimal[currency]:
		scale = 1
	case threeDecimal[currency]:
		scale = 1000
	}
	return math.Round(value*scale*1e9) / 1e9, nil
}

// priceSpec reads a catalog price into the Stripe price Moesif creates for it.
func priceSpec(id string, body map[string]any) (stripe.PriceSpec, error) {
	currency := text(body, "currency")
	spec := stripe.PriceSpec{ID: id, Currency: currency, Product: text(body, "plan_id"), Nickname: text(body, "name")}
	switch {
	case spec.Nickname == "" || spec.Product == "":
		return spec, errors.New("name and plan_id are required")
	case text(body, "provider") != stripeProvider || text(body, "status") != "active":
		return spec, errors.New("a price is created active, for the stripe provider")
	case len(currency) != 3 || strings.ToLower(currency) != currency:
		return spec, errors.New("currency is a lower-case three-letter code")
	case body["period"] != float64(1):
		return spec, errors.New("period must be 1")
	case text(body, "usage_aggregator") != "sum":
		return spec, errors.New("usage_aggregator must be sum")
	}
	switch text(body, "period_units") {
	case "M":
		spec.Interval = "month"
	case "y":
		spec.Interval = "year"
	default:
		return spec, errors.New("period_units must be M or y")
	}
	model := text(body, "pricing_model")
	spec.Metered = model != "flat"
	var err error
	switch model {
	case "flat", "per_unit":
		if spec.UnitAmount, err = minorUnits(text(body, "price_in_decimal"), currency); err != nil {
			return spec, fmt.Errorf("price_in_decimal: %w", err)
		}
		if transform := object(body, "transform_quantity"); transform != nil {
			divide, _ := transform["divide_by"].(float64)
			if model != "per_unit" || divide < 2 || divide != math.Trunc(divide) || transform["round"] != "up" {
				return spec, errors.New("transform_quantity divides a per-unit price by a whole number above one, rounding up")
			}
			spec.DivideBy = int64(divide)
		}
	case "volume", "tiered":
		spec.TiersMode = "volume"
		if model == "tiered" {
			spec.TiersMode = "graduated"
		}
		if spec.Tiers, err = tiersOf(body["tiers"], currency); err != nil {
			return spec, err
		}
	default:
		return spec, errors.New("pricing_model must be flat, per_unit, volume or tiered")
	}
	return spec, nil
}

// tiersOf reads a catalog price's tiers: each ends at a whole number, the last at "inf", with a
// unit amount, a flat amount, or both.
func tiersOf(value any, currency string) ([]stripe.Tier, error) {
	raw, _ := value.([]any)
	if len(raw) == 0 {
		return nil, errors.New("a tiered price needs tiers")
	}
	tiers := make([]stripe.Tier, 0, len(raw))
	for i, entry := range raw {
		fields, _ := entry.(map[string]any)
		var tier stripe.Tier
		last := i == len(raw)-1
		switch upTo := fields["up_to"].(type) {
		case string:
			if upTo != "inf" || !last {
				return nil, errors.New(`only the last tier ends at "inf"`)
			}
		case float64:
			if last || upTo < 1 || upTo != math.Trunc(upTo) {
				return nil, errors.New("a tier ends at a whole number, and the last at \"inf\"")
			}
			tier.UpTo = int64(upTo)
		default:
			return nil, errors.New("every tier needs up_to")
		}
		unit, hasUnit := fields["unit_price_in_decimal"].(string)
		flat, hasFlat := fields["flat_price_in_decimal"].(string)
		if !hasUnit && !hasFlat {
			return nil, errors.New("a tier needs a unit or a flat amount")
		}
		var err error
		if hasUnit {
			if tier.UnitAmount, err = minorUnits(unit, currency); err != nil {
				return nil, fmt.Errorf("unit_price_in_decimal: %w", err)
			}
		}
		if hasFlat {
			if tier.FlatAmount, err = minorUnits(flat, currency); err != nil {
				return nil, fmt.Errorf("flat_price_in_decimal: %w", err)
			}
		}
		tiers = append(tiers, tier)
	}
	return tiers, nil
}

func (s *Service) createPrice(key string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any) {
	if !providerOf(w, r) {
		return
	}
	id := newID("price_")
	spec, err := priceSpec(id, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error()+".")
		return
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	if !app.connected() {
		writeError(w, http.StatusBadRequest, "No billing provider is connected for stripe.")
		return
	}
	if p := app.plans[spec.Product]; p == nil || p.Status != "active" {
		writeError(w, http.StatusBadRequest, "plan_id is not an active plan.")
		return
	}
	if err := s.billing.AddPrice(key, spec); err != nil {
		writeError(w, http.StatusBadRequest, "Stripe refused the price: "+err.Error())
		return
	}
	p := &price{ID: id, PlanID: spec.Product, Status: "active", Payload: body, Created: s.now(), Order: app.next()}
	app.prices[id] = p
	writeJSON(w, http.StatusCreated, p.json())
}

func (s *Service) archivePrice(key string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any) {
	if !archiveRequest(w, r, body) {
		return
	}
	id := r.PathValue("id")
	app.mu.Lock()
	defer app.mu.Unlock()
	p := app.prices[id]
	if p == nil {
		writeError(w, http.StatusBadRequest, "resource_missing: No such price: '"+id+"'")
		return
	}
	if err := s.billing.ArchivePrice(key, id); err != nil {
		writeError(w, http.StatusBadRequest, "Stripe refused to archive the price: "+err.Error())
		return
	}
	p.Status = "archived"
	writeJSON(w, http.StatusOK, p.json())
}

func (s *Service) deletePrice(key string, app *application, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	if !providerOf(w, r) {
		return
	}
	id := r.PathValue("id")
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.prices[id] == nil {
		writeError(w, http.StatusNotFound, "Price not found.")
		return
	}
	if err := s.billing.ArchivePrice(key, id); err != nil {
		writeError(w, http.StatusBadRequest, "Stripe refused to archive the price: "+err.Error())
		return
	}
	delete(app.prices, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) listMeters(_ string, app *application, w http.ResponseWriter, _ *http.Request, _ map[string]any) {
	app.mu.Lock()
	defer app.mu.Unlock()
	meters := []map[string]any{}
	for _, m := range sortedByOrder(app.meters, func(m *meter) int64 { return m.Order }) {
		meters = append(meters, m.json())
	}
	writeJSON(w, http.StatusOK, meters)
}

// meterPrice reads the Stripe price and product a billing meter reports to.
func meterPrice(body map[string]any) (priceID, productID string, err error) {
	plan := object(body, "billing_plan")
	stripeParams := object(object(plan, "params"), "stripe_params")
	product := object(stripeParams, "product")
	prices, _ := stripeParams["prices"].([]any)
	if text(plan, "provider_slug") != stripeProvider || len(prices) != 1 {
		return "", "", errors.New("billing_plan reports to one stripe price")
	}
	entry, _ := prices[0].(map[string]any)
	if text(entry, "price_id") == "" || text(product, "id") == "" || text(product, "usage_type") != "metered" {
		return "", "", errors.New("billing_plan names a metered product and its price")
	}
	return text(entry, "price_id"), text(product, "id"), nil
}

func (s *Service) createMeter(_ string, app *application, w http.ResponseWriter, _ *http.Request, body map[string]any) {
	switch {
	case text(body, "name") == "":
		writeError(w, http.StatusBadRequest, "name is required.")
		return
	case text(body, "slug") != billingMeterSlug:
		writeError(w, http.StatusBadRequest, "Invalid meter.")
		return
	case text(body, "status") != "active" || text(body, "url_query") == "":
		writeError(w, http.StatusBadRequest, "A meter is created active, with its url_query.")
		return
	}
	query := object(body, "es_query")
	sum := object(object(object(object(object(query, "aggs"), "seg"), "aggs"), "usage_value"), "sum")
	sig, ids, err := countSignature(object(query, "query"), sum)
	if err == nil && (ids.customer != "{{company_id}}" || ids.subscription != "{{subscription_id}}") {
		err = errors.New("es_query must count one company's subscription with {{company_id}} and {{subscription_id}}")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error()+".")
		return
	}
	priceID, productID, err := meterPrice(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error()+".")
		return
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	p := app.prices[priceID]
	if p == nil || p.Status != "active" || p.PlanID != productID || text(p.Payload, "pricing_model") == "flat" {
		writeError(w, http.StatusBadRequest, "billing_plan must name an active usage price of its product.")
		return
	}
	m := &meter{
		ID: newID(""), Name: text(body, "name"), PriceID: priceID, Signature: sig, Payload: body,
		Created: s.now(), Order: app.next(),
	}
	app.meters[m.ID] = m
	writeJSON(w, http.StatusCreated, m.json())
}

func (s *Service) deleteMeter(_ string, app *application, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	id := r.PathValue("id")
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.meters[id] == nil {
		writeError(w, http.StatusNotFound, "Meter not found.")
		return
	}
	delete(app.meters, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) getPartnerAccount(_ string, app *application, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	provider := r.PathValue("provider")
	app.mu.Lock()
	defer app.mu.Unlock()
	if account := app.partners[provider]; account != nil {
		writeJSON(w, http.StatusOK, account.json())
		return
	}
	if app.disconnected[provider] {
		writeError(w, http.StatusBadRequest, "Unable to get the Partner account")
		return
	}
	writeError(w, http.StatusNotFound, "Not found.")
}

func (s *Service) savePartnerAccount(_ string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any) {
	if r.PathValue("provider") != stripeProvider {
		writeError(w, http.StatusBadRequest, "Only stripe can be connected.")
		return
	}
	partner := object(body, "partner_account")
	credentials := object(partner, "credentials")
	account := &partnerAccount{
		Version: text(partner, "version"), APIVersion: text(partner, "api_version"), State: text(credentials, "state"),
		CompanyIDField: text(object(object(body, "billing_config"), stripeProvider), "company_id_field"),
	}
	switch {
	case account.Version == "" || account.APIVersion == "" || account.CompanyIDField == "":
		writeError(w, http.StatusBadRequest, "partner_account needs its version and api_version, and billing_config its company_id_field.")
		return
	case account.State != "enabled" && account.State != "disabled":
		writeError(w, http.StatusBadRequest, "credentials.state must be enabled or disabled.")
		return
	case account.State == "enabled" && !stripeKeyPattern.MatchString(text(credentials, "api_key")):
		writeError(w, http.StatusBadRequest, "Stripe refused the API key.")
		return
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	app.partners[stripeProvider] = account
	delete(app.disconnected, stripeProvider)
	writeJSON(w, http.StatusOK, account.json())
}

func (s *Service) deletePartnerAccount(_ string, app *application, w http.ResponseWriter, r *http.Request, _ map[string]any) {
	provider := r.PathValue("provider")
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.partners[provider] == nil {
		writeError(w, http.StatusNotFound, "Not found.")
		return
	}
	delete(app.partners, provider)
	app.disconnected[provider] = true
	writeJSON(w, http.StatusOK, map[string]any{})
}
