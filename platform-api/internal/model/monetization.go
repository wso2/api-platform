/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package model

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Types of a meter: REQUEST_COUNT counts each API request as one; TOKENS adds up a number in
// each AI API request's event, the one its event key names.
const (
	MeterTypeRequestCount = "REQUEST_COUNT"
	MeterTypeTokens       = "TOKENS"
)

// Operators of a price filter. Any of them works on any of a meter's properties: EQUALS, NOT_EQUALS,
// IN, NOT_IN and STARTS_WITH match text, the four comparisons a whole number.
const (
	FilterOperatorEquals             = "EQUALS"
	FilterOperatorNotEquals          = "NOT_EQUALS"
	FilterOperatorIn                 = "IN"
	FilterOperatorNotIn              = "NOT_IN"
	FilterOperatorStartsWith         = "STARTS_WITH"
	FilterOperatorGreaterThan        = "GREATER_THAN"
	FilterOperatorGreaterThanOrEqual = "GREATER_THAN_OR_EQUAL"
	FilterOperatorLessThan           = "LESS_THAN"
	FilterOperatorLessThanOrEqual    = "LESS_THAN_OR_EQUAL"
)

const (
	// maxMeterFilterFields is how many properties a meter lists at most.
	maxMeterFilterFields = 10
	// maxPropertyNameChars bounds the name of a meter's property.
	maxPropertyNameChars = 40
	// maxPriceFilters is how many filters a price has at most.
	maxPriceFilters = 20
	// maxComparisons is how many comparisons a price puts on one property at most: enough for a range
	// such as more than 100 and at most 500.
	maxComparisons = 2
	// maxCompareValue is the largest number a comparison takes.
	maxCompareValue = 1_000_000_000_000
)

var (
	meterTypes = []string{MeterTypeRequestCount, MeterTypeTokens}
	// filterOperators are the operators of a price filter, in the order messages list them.
	filterOperators = []string{FilterOperatorEquals, FilterOperatorNotEquals, FilterOperatorIn, FilterOperatorNotIn,
		FilterOperatorStartsWith, FilterOperatorGreaterThan, FilterOperatorGreaterThanOrEqual, FilterOperatorLessThan,
		FilterOperatorLessThanOrEqual}
	// compareOperators are the operators that compare a whole number; the others match text.
	compareOperators = []string{FilterOperatorGreaterThan, FilterOperatorGreaterThanOrEqual, FilterOperatorLessThan,
		FilterOperatorLessThanOrEqual}
)

// Meter is what a paid plan charges for. A meter only says what is counted; each plan price narrows
// it with its own filters. A meter made in the Management Portal is kept here and sent to an API
// Portal when a plan that charges for it is published there. A meter made in an API Portal is a copy,
// kept current from that portal's meter.* webhook events.
type Meter struct {
	UUID             string `json:"id" db:"uuid"`
	OrganizationUUID string `json:"organizationId" db:"organization_uuid"`
	Handle           string `json:"handle" db:"handle"`
	DisplayName      string `json:"displayName" db:"display_name"`
	Description      string `json:"description,omitempty" db:"description"`
	Type             string `json:"type" db:"type"`
	// TypeKey is the event field whose number a TOKENS meter adds up, such as
	// metadata.aiTokenUsage.promptTokens; nil for REQUEST_COUNT.
	TypeKey *string `json:"typeKey" db:"type_key"`
	// FilterFields are the properties the meter's prices may filter on, each a name and the event
	// field it reads; empty when they take no filters.
	FilterFields []MeterFilterField `json:"filterFields" db:"filter_fields"`
	// SourceUpdatedAt is when the meter last changed where it was made: the time of the request for
	// a meter made here, the event's occurred_at for a copy of an API Portal meter. An API Portal
	// event older than it is stale and ignored.
	SourceUpdatedAt time.Time `json:"sourceUpdatedAt" db:"source_updated_at"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
}

// MeterFilterField is a property a meter's prices may filter on: the name the administrator gives it
// (Field) and the event field it reads (EventKey).
type MeterFilterField struct {
	Field    string `json:"field"`
	EventKey string `json:"event_key"`
}

// Property returns the meter's property with the name, matched ignoring case.
func (m *Meter) Property(name string) (MeterFilterField, bool) {
	for _, f := range m.FilterFields {
		if sameName(f.Field, name) {
			return f, true
		}
	}
	return MeterFilterField{}, false
}

// maxEventKeyChars bounds an event key, as its column does.
const maxEventKeyChars = 255

// ValidateMeter checks a meter against the rules the API Portal applies:
//   - the type is REQUEST_COUNT or TOKENS;
//   - a TOKENS meter has a TypeKey (the event field it adds up), and a REQUEST_COUNT meter none;
//   - the TypeKey is 1 to 255 characters with no spaces;
//   - the filter fields follow validateFilterFields.
//
// The returned error's text is safe to show.
func ValidateMeter(m *Meter) error {
	if !slices.Contains(meterTypes, m.Type) {
		return fmt.Errorf("type must be one of %s", strings.Join(meterTypes, ", "))
	}
	switch {
	case m.Type == MeterTypeTokens && m.TypeKey == nil:
		return errors.New("A TOKENS meter needs a typeKey")
	case m.Type != MeterTypeTokens && m.TypeKey != nil:
		return errors.New("typeKey applies only to a TOKENS meter")
	case m.TypeKey != nil && !validEventKey(*m.TypeKey):
		return fmt.Errorf("typeKey must be 1 to %d characters with no spaces", maxEventKeyChars)
	}
	return validateFilterFields(m.FilterFields)
}

// validateFilterFields checks a meter's properties: at most 10, each with a name of 1 to 40 characters
// with no control characters and an event key of 1 to 255 characters with no spaces. Names are unique
// ignoring case, and event keys are unique. An event key may be the meter's TypeKey.
func validateFilterFields(fields []MeterFilterField) error {
	if len(fields) > maxMeterFilterFields {
		return fmt.Errorf("A meter has at most %d filter properties", maxMeterFilterFields)
	}
	for i, f := range fields {
		if !validPropertyName(f.Field) {
			return fmt.Errorf("A filter property's name must be 1 to %d characters", maxPropertyNameChars)
		}
		if slices.ContainsFunc(fields[:i], func(o MeterFilterField) bool { return sameName(o.Field, f.Field) }) {
			return fmt.Errorf("The meter has the property %s more than once", f.Field)
		}
		if !validEventKey(f.EventKey) {
			return fmt.Errorf("The event key of %s must be 1 to %d characters with no spaces", f.Field, maxEventKeyChars)
		}
		if duplicate := slices.IndexFunc(fields[:i], func(o MeterFilterField) bool { return o.EventKey == f.EventKey }); duplicate >= 0 {
			return fmt.Errorf("%s and %s read the same event key %s", f.Field, fields[duplicate].Field, f.EventKey)
		}
	}
	return nil
}

// validPropertyName reports whether a property's name, trimmed by the caller, is 1 to
// maxPropertyNameChars characters with no control characters.
func validPropertyName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxPropertyNameChars {
		return false
	}
	return !strings.ContainsFunc(name, unicode.IsControl)
}

// sameName reports whether two property names are the same, ignoring case.
func sameName(a, b string) bool {
	return strings.EqualFold(a, b)
}

// validEventKey reports whether key is 1 to maxEventKeyChars characters with no whitespace or
// control characters.
func validEventKey(key string) bool {
	if key == "" || utf8.RuneCountInString(key) > maxEventKeyChars {
		return false
	}
	return !strings.ContainsFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

// Pricing models of a usage price, which charges for the units a meter counts: PER_UNIT a price
// per unit (or per batch of units), VOLUME all the units at the price of the tier their total falls
// in, GRADUATED the units in each tier at that tier's price.
const (
	PricingModelPerUnit   = "PER_UNIT"
	PricingModelVolume    = "VOLUME"
	PricingModelGraduated = "GRADUATED"
)

// PricingModelFlat is the pricing model of the one stored price of a FLAT_RATE plan, which holds the
// amount charged every billing period.
const PricingModelFlat = "FLAT"

// Pricing types of a paid plan: a FLAT_RATE plan charges one amount every billing period; a
// USAGE_BASED plan charges its usage prices.
const (
	PricingTypeFlatRate   = "FLAT_RATE"
	PricingTypeUsageBased = "USAGE_BASED"
)

// Billing periods of a paid plan.
const (
	BillingPeriodMonth = "MONTH"
	BillingPeriodYear  = "YEAR"
)

const (
	// maxPlanPrices is how many usage prices a plan has at most: a payment provider subscription
	// holds at most 20 items (Stripe).
	maxPlanPrices        = 20
	maxPriceTiers        = 10
	maxPricingMeterChars = 40
	maxUnitQuantity      = 1_000_000_000
	maxFilterValues      = 20
	maxFilterValueChars  = 255
)

var (
	decimalAmountPattern = regexp.MustCompile(`^\d{1,12}(\.\d{1,12})?$`)
	// listOperators take 1 to maxFilterValues values; the other operators take exactly one.
	listOperators = []string{FilterOperatorIn, FilterOperatorNotIn}
)

// PriceTier is one tier of a VOLUME or GRADUATED price. UpTo is nil on the last tier. Amounts are
// decimal strings in the currency's main unit.
type PriceTier struct {
	UpTo       *int64  `json:"up_to"`
	UnitAmount *string `json:"unit_amount"`
	FlatAmount *string `json:"flat_amount"`
}

// PriceFilter narrows the requests a price counts: a request is counted only when it matches every
// filter of the price. Field names one of the meter's properties, as the meter spells it, and the
// filter reads that property's event key.
type PriceFilter struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// SubscriptionPlanPrice is a usage price of a paid plan: it charges for what the meter MeterHandle
// counts, narrowed by its Filters (none counts every request the meter counts), in one pricing
// model; the fields of other models are nil. A plan may price one meter several times, as long as
// no request is counted by two of those prices. A price has no name of its own: it is shown as the
// meter's name followed by its filters.
type SubscriptionPlanPrice struct {
	MeterHandle  string        `json:"meter_handle"`
	Filters      []PriceFilter `json:"filters"`
	PricingModel string        `json:"pricing_model"`
	UnitAmount   *string       `json:"unit_amount"`
	UnitQuantity *int64        `json:"unit_quantity"`
	Tiers        []PriceTier   `json:"tiers"`
}

// SubscriptionPlanPricing is the pricing of a paid subscription plan; a plan without pricing is
// free. A paid plan is either FLAT_RATE, charging FlatAmount every billing period, or USAGE_BASED,
// charging 1 to 20 usage prices. Pricing is set when the plan is created, through this REST API
// (the Management Portal) or in an API Portal (its subscription_plan.created webhook event), and
// never changes. The repository stores a FLAT_RATE plan's amount as the one entry of its prices, a
// PricingModelFlat price.
type SubscriptionPlanPricing struct {
	SubscriptionPlanUUID string `json:"subscriptionPlanId" db:"subscription_plan_uuid"`
	OrganizationUUID     string `json:"organizationId" db:"organization_uuid"`
	PricingType          string `json:"pricingType"`
	Currency             string `json:"currency" db:"currency"`
	BillingPeriod        string `json:"billingPeriod" db:"billing_period"`
	// FlatAmount is the amount a FLAT_RATE plan charges every billing period; nil for USAGE_BASED.
	FlatAmount *string `json:"flatAmount"`
	// Prices are the usage prices of a USAGE_BASED plan, in order; empty for FLAT_RATE.
	Prices []SubscriptionPlanPrice `json:"prices" db:"prices"`
	// CreatedAt is when the pricing was set: the time of the request for a plan created here, the
	// event's occurred_at for a plan created in an API Portal.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// ValidatePlanPricing checks a plan's pricing against the rules the API Portal applies; the API
// Portal decides, this check only refuses what it would refuse anyway. That the meters exist is left
// to the caller, and so is that each filter names a property of its meter. The filters are normalized
// in place: names and values trimmed, repeated values dropped with the order kept, and a comparison's
// number written without leading zeros. The returned error's text names the field and is safe to show.
func ValidatePlanPricing(p *SubscriptionPlanPricing) error {
	// Only the currencies the portals offer (offeredCurrencies): the API Portal refuses any other, so a
	// plan priced in one would fail when it is published. A code that is not a currency at all gets the
	// same answer.
	if !offeredCurrencies[p.Currency] {
		return errors.New("currency must be the code of a currency with two decimal places, such as USD or EUR. " +
			"Currencies with no or three decimal places cannot be used.")
	}
	if p.BillingPeriod != BillingPeriodMonth && p.BillingPeriod != BillingPeriodYear {
		return errors.New("billingPeriod must be one of MONTH, YEAR")
	}
	switch p.PricingType {
	case PricingTypeFlatRate:
		if len(p.Prices) > 0 {
			return errors.New("A flat rate plan has no usage prices")
		}
		if p.FlatAmount == nil {
			return errors.New("flatAmount is required")
		}
		if err := validateAmount("flatAmount", *p.FlatAmount); err != nil {
			return err
		}
		if !isPositiveAmount(*p.FlatAmount) {
			return errors.New("flatAmount must be more than 0. Create a free plan instead.")
		}
	case PricingTypeUsageBased:
		if p.FlatAmount != nil {
			return errors.New("A usage based plan has no flat amount")
		}
		if len(p.Prices) == 0 || len(p.Prices) > maxPlanPrices {
			return fmt.Errorf("A usage based plan needs 1 to %d prices", maxPlanPrices)
		}
		for i := range p.Prices {
			if err := validatePrice(&p.Prices[i], fmt.Sprintf("price %d", i+1)); err != nil {
				return err
			}
		}
		for i := range p.Prices {
			for j := i + 1; j < len(p.Prices); j++ {
				if p.Prices[i].MeterHandle == p.Prices[j].MeterHandle && filtersOverlap(p.Prices[i].Filters, p.Prices[j].Filters) {
					return fmt.Errorf("Prices %d and %d both count the same requests of the meter %s; give them different filters.",
						i+1, j+1, p.Prices[i].MeterHandle)
				}
			}
		}
	default:
		return errors.New("pricingType must be one of FLAT_RATE, USAGE_BASED")
	}
	return nil
}

// validatePrice checks one usage price on its own; where names it in messages ("price 2"). The checks
// and their wording are the API Portal's (planPricingValidation.js), in its order.
func validatePrice(p *SubscriptionPlanPrice, where string) error {
	if p.PricingModel != PricingModelPerUnit && p.PricingModel != PricingModelVolume && p.PricingModel != PricingModelGraduated {
		return fmt.Errorf("pricingModel of %s must be one of PER_UNIT, VOLUME, GRADUATED", where)
	}
	if extra := fieldsOfOtherModels(p); len(extra) > 0 {
		return fmt.Errorf("A %s price (%s) does not take %s", p.PricingModel, where, strings.Join(extra, ", "))
	}
	if p.MeterHandle == "" {
		return fmt.Errorf("meterId of %s is required", where)
	}
	if utf8.RuneCountInString(p.MeterHandle) > maxPricingMeterChars {
		return fmt.Errorf("meterId of %s must be at most %d characters", where, maxPricingMeterChars)
	}
	if err := validateFilters(p.Filters, where); err != nil {
		return err
	}
	if p.PricingModel != PricingModelPerUnit {
		return validateTiers(p.Tiers, where)
	}
	if p.UnitQuantity != nil && (*p.UnitQuantity < 1 || *p.UnitQuantity > maxUnitQuantity) {
		return fmt.Errorf("unitQuantity of %s must be a whole number from 1 to %d", where, maxUnitQuantity)
	}
	if p.UnitAmount == nil || *p.UnitAmount == "" {
		return fmt.Errorf("unitAmount of %s is required", where)
	}
	return validateAmount("unitAmount of "+where, *p.UnitAmount)
}

// fieldsOfOtherModels returns the fields a price gives that its pricing model does not take: tiers for
// PER_UNIT, unitAmount and unitQuantity for VOLUME and GRADUATED; an empty amount counts as not given.
func fieldsOfOtherModels(p *SubscriptionPlanPrice) []string {
	var extra []string
	if p.PricingModel == PricingModelPerUnit {
		if p.Tiers != nil {
			extra = append(extra, "tiers")
		}
		return extra
	}
	if p.UnitAmount != nil && *p.UnitAmount != "" {
		extra = append(extra, "unitAmount")
	}
	if p.UnitQuantity != nil {
		extra = append(extra, "unitQuantity")
	}
	return extra
}

// validateFilters checks the filters of a price and normalizes them in place: at most 20 filters, each
// naming a property (1 to 40 characters) with any operator and the values that operator takes. On one
// property, ignoring the case of its name, a price has either one filter that matches text (EQUALS,
// NOT_EQUALS, IN, NOT_IN, STARTS_WITH) or one or two comparisons, which must be able to all be true
// together. Messages start with where ("price 2: ...").
func validateFilters(filters []PriceFilter, where string) error {
	if len(filters) > maxPriceFilters {
		return fmt.Errorf("%s: a price has at most %d filters", where, maxPriceFilters)
	}
	for i := range filters {
		f := &filters[i]
		f.Field = strings.TrimSpace(f.Field)
		if !validPropertyName(f.Field) {
			return fmt.Errorf("field of filter %d of %s must be 1 to %d characters", i+1, where, maxPropertyNameChars)
		}
		if !slices.Contains(filterOperators, f.Operator) {
			return fmt.Errorf("The operator of filter %d of %s must be one of %s", i+1, where, strings.Join(filterOperators, ", "))
		}
		values, err := normalizeFilterValues(f.Field, f.Operator, f.Values)
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		f.Values = values
	}
	for _, name := range propertiesOf(filters) {
		on := filtersOn(filters, name)
		if slices.ContainsFunc(on, func(f PriceFilter) bool { return !isComparison(f) }) {
			if len(on) > 1 {
				return fmt.Errorf("%s filters on %s more than once", where, name)
			}
			continue
		}
		if len(on) > maxComparisons {
			return fmt.Errorf("%s: a price puts at most %d comparisons on %s", where, maxComparisons, name)
		}
		if compareRange(filters, name).empty() {
			return fmt.Errorf("%s: the comparisons on %s can't all be true", where, name)
		}
	}
	return nil
}

// normalizeFilterValues checks the values of a filter on the property name with the operator and
// returns them as stored: exactly one value for every operator but IN and NOT_IN, which take 1 to 20.
// A text value is trimmed and 1 to 255 characters, and repeated values are dropped with the order
// kept. A comparison's value is one whole number from 0 to 1000000000000, written in digits; it is
// stored without leading zeros.
func normalizeFilterValues(name, operator string, raw []string) ([]string, error) {
	if !slices.Contains(listOperators, operator) {
		if len(raw) != 1 {
			return nil, fmt.Errorf("Operator %s takes exactly one value", operator)
		}
	} else if len(raw) == 0 || len(raw) > maxFilterValues {
		return nil, fmt.Errorf("A filter on %s needs 1 to %d values", name, maxFilterValues)
	}
	if slices.Contains(compareOperators, operator) {
		n, ok := parseCompareValue(raw[0])
		if !ok {
			return nil, fmt.Errorf("A comparison on %s takes a whole number from 0 to %d", name, int64(maxCompareValue))
		}
		return []string{strconv.FormatInt(n, 10)}, nil
	}
	values := make([]string, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > maxFilterValueChars {
			return nil, fmt.Errorf("Filter values for %s must be 1 to %d characters", name, maxFilterValueChars)
		}
		if !slices.Contains(values, value) {
			values = append(values, value)
		}
	}
	return values, nil
}

// parseCompareValue reads a comparison's number: a whole number from 0 to maxCompareValue, written
// in digits only.
func parseCompareValue(value string) (int64, bool) {
	n, ok := wholeNumber(strings.TrimSpace(value))
	if !ok || n > maxCompareValue {
		return 0, false
	}
	return n, true
}

// wholeNumber reads a value written in digits only.
func wholeNumber(value string) (int64, bool) {
	if value == "" || strings.ContainsFunc(value, func(r rune) bool { return r < '0' || r > '9' }) {
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return math.MaxInt64, true
	}
	return n, true
}

// isComparison reports whether a filter compares a whole number rather than matching text.
func isComparison(f PriceFilter) bool {
	return slices.Contains(compareOperators, f.Operator)
}

// propertiesOf returns the properties the filters name, in order, each once (ignoring case).
func propertiesOf(filters []PriceFilter) []string {
	var names []string
	for _, f := range filters {
		if !slices.ContainsFunc(names, func(name string) bool { return sameName(name, f.Field) }) {
			names = append(names, f.Field)
		}
	}
	return names
}

// filtersOn returns the filters on the property name, matched ignoring case.
func filtersOn(filters []PriceFilter, name string) []PriceFilter {
	var on []PriceFilter
	for _, f := range filters {
		if sameName(f.Field, name) {
			on = append(on, f)
		}
	}
	return on
}

// numberRange is the whole numbers from low to high, both included; empty when low > high.
type numberRange struct {
	low, high int64
}

func (r numberRange) empty() bool { return r.low > r.high }

func (r numberRange) contains(n int64) bool { return r.low <= n && n <= r.high }

// compareRange returns the numbers that the filters' comparisons on the property name all accept:
// every number when there are none. The values have been checked by normalizeFilterValues.
func compareRange(filters []PriceFilter, name string) numberRange {
	r := numberRange{low: 0, high: math.MaxInt64}
	for _, f := range filtersOn(filters, name) {
		if !isComparison(f) || len(f.Values) != 1 {
			continue
		}
		n, ok := parseCompareValue(f.Values[0])
		if !ok {
			continue
		}
		switch f.Operator {
		case FilterOperatorGreaterThan:
			r.low = max(r.low, n+1)
		case FilterOperatorGreaterThanOrEqual:
			r.low = max(r.low, n)
		case FilterOperatorLessThan:
			r.high = min(r.high, n-1)
		case FilterOperatorLessThanOrEqual:
			r.high = min(r.high, n)
		}
	}
	return r
}

// validateTiers checks VOLUME and GRADUATED tiers: 1 to 10 tiers, each ending above the one
// before, only the last one open-ended, and each with a per-unit price, a flat fee, or both.
func validateTiers(tiers []PriceTier, where string) error {
	if len(tiers) == 0 || len(tiers) > maxPriceTiers {
		return fmt.Errorf("%s needs 1 to %d tiers", where, maxPriceTiers)
	}
	var previous int64
	for i, tier := range tiers {
		last := i == len(tiers)-1
		if last && tier.UpTo != nil {
			return fmt.Errorf("The last tier of %s must have no upper bound (upTo null)", where)
		}
		if !last {
			if tier.UpTo == nil || *tier.UpTo <= previous {
				return fmt.Errorf("Each tier of %s needs an upTo greater than the tier before it", where)
			}
			previous = *tier.UpTo
		}
		if tier.UnitAmount != nil {
			if err := validateAmount(fmt.Sprintf("unitAmount of tier %d of %s", i+1, where), *tier.UnitAmount); err != nil {
				return err
			}
		}
		if tier.FlatAmount != nil {
			if err := validateAmount(fmt.Sprintf("flatAmount of tier %d of %s", i+1, where), *tier.FlatAmount); err != nil {
				return err
			}
		}
		if tier.UnitAmount == nil && tier.FlatAmount == nil {
			return fmt.Errorf("Tier %d of %s needs a unitAmount, a flatAmount or both", i+1, where)
		}
	}
	return nil
}

func validateAmount(field, value string) error {
	if !decimalAmountPattern.MatchString(value) {
		return fmt.Errorf("%s must be a decimal amount such as 9.99", field)
	}
	return nil
}

// isPositiveAmount reports whether a decimal amount (already validated) is above zero.
func isPositiveAmount(value string) bool {
	return strings.Trim(strings.Replace(value, ".", "", 1), "0") != ""
}

// filtersOverlap reports whether one request could be counted by two prices on the same meter: for
// every property both prices filter on, their filters on it can match one value. A property only one
// price filters on matches any value for the other. The filters have been checked by validateFilters.
func filtersOverlap(a, b []PriceFilter) bool {
	for _, name := range propertiesOf(a) {
		onA, onB := filtersOn(a, name), filtersOn(b, name)
		if len(onB) == 0 {
			continue
		}
		comparesA, comparesB := isComparison(onA[0]), isComparison(onB[0])
		var overlap bool
		switch {
		case !comparesA && !comparesB:
			overlap = filterValuesOverlap(onA[0], onB[0])
		case comparesA && comparesB:
			ra, rb := compareRange(a, name), compareRange(b, name)
			overlap = !(numberRange{low: max(ra.low, rb.low), high: min(ra.high, rb.high)}).empty()
		case comparesA:
			overlap = rangeOverlapsText(compareRange(a, name), onB[0])
		default:
			overlap = rangeOverlapsText(compareRange(b, name), onA[0])
		}
		if !overlap {
			return false
		}
	}
	return true
}

// rangeOverlapsText reports whether a text filter matches some number of the range: for EQUALS and IN,
// one of its values is a whole number in the range; NOT_EQUALS, NOT_IN and STARTS_WITH always do.
func rangeOverlapsText(r numberRange, f PriceFilter) bool {
	if operatorKind(f.Operator) != matchesValues {
		return true
	}
	return slices.ContainsFunc(f.Values, func(v string) bool {
		n, ok := wholeNumber(v)
		return ok && r.contains(n)
	})
}

// Kinds of text filter operator, in the order filterValuesOverlap compares them.
const (
	matchesValues  = iota // EQUALS, IN
	matchesPrefix         // STARTS_WITH
	excludesValues        // NOT_EQUALS, NOT_IN
)

func operatorKind(operator string) int {
	switch operator {
	case FilterOperatorStartsWith:
		return matchesPrefix
	case FilterOperatorNotEquals, FilterOperatorNotIn:
		return excludesValues
	default:
		return matchesValues
	}
}

// filterValuesOverlap reports whether some value matches both text filters on one property:
//   - EQUALS/IN with EQUALS/IN: the lists share a value;
//   - EQUALS/IN with STARTS_WITH: a value of the list starts with the prefix;
//   - EQUALS/IN with NOT_EQUALS/NOT_IN: a value of the first list is not in the second;
//   - STARTS_WITH with STARTS_WITH: one prefix starts with the other;
//   - NOT_EQUALS/NOT_IN with NOT_EQUALS/NOT_IN or STARTS_WITH: always, since values outside the lists
//     remain.
func filterValuesOverlap(a, b PriceFilter) bool {
	if operatorKind(b.Operator) < operatorKind(a.Operator) {
		a, b = b, a
	}
	switch {
	case operatorKind(a.Operator) == matchesValues && operatorKind(b.Operator) == matchesValues:
		return slices.ContainsFunc(a.Values, func(v string) bool { return slices.Contains(b.Values, v) })
	case operatorKind(a.Operator) == matchesValues && operatorKind(b.Operator) == matchesPrefix:
		return slices.ContainsFunc(a.Values, func(v string) bool { return strings.HasPrefix(v, b.Values[0]) })
	case operatorKind(a.Operator) == matchesValues:
		return slices.ContainsFunc(a.Values, func(v string) bool { return !slices.Contains(b.Values, v) })
	case operatorKind(a.Operator) == matchesPrefix && operatorKind(b.Operator) == matchesPrefix:
		return strings.HasPrefix(a.Values[0], b.Values[0]) || strings.HasPrefix(b.Values[0], a.Values[0])
	default:
		return true
	}
}

// Billing statuses of a paid subscription, as the API Portal reports them.
const (
	BillingStatusActive    = "ACTIVE"
	BillingStatusPastDue   = "PAST_DUE"
	BillingStatusUnpaid    = "UNPAID"
	BillingStatusCancelled = "CANCELLED"
)

// ValidBillingStatuses lists the billing statuses a subscription can have.
var ValidBillingStatuses = map[string]bool{
	BillingStatusActive: true, BillingStatusPastDue: true, BillingStatusUnpaid: true, BillingStatusCancelled: true,
}

// SubscriptionBilling is the billing state of a paid subscription. UpdatedAt is when the API Portal
// recorded it, so a late event is not applied over a newer one.
type SubscriptionBilling struct {
	SubscriptionUUID string    `json:"-" db:"subscription_uuid"`
	OrganizationUUID string    `json:"-" db:"organization_uuid"`
	Status           string    `json:"status" db:"billing_status"`
	CustomerID       string    `json:"customerId" db:"billing_customer_id"`
	SubscriptionID   string    `json:"subscriptionId" db:"billing_subscription_id"`
	UpdatedAt        time.Time `json:"updatedAt" db:"billing_updated_at"`
}

// GatewayStatus is the subscription status gateways enforce: INACTIVE while a paid subscription's
// billing status stops access (UNPAID or CANCELLED), otherwise the subscription's own status. The
// subscription's stored status stays what the subscriber chose, so access returns to it once the
// payment is made, and a subscriber cannot resume an unpaid subscription themselves.
func GatewayStatus(status SubscriptionStatus, billing *SubscriptionBilling) string {
	if billing != nil && (billing.Status == BillingStatusUnpaid || billing.Status == BillingStatusCancelled) {
		return string(SubscriptionStatusInactive)
	}
	return string(status)
}
