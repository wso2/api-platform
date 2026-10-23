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

// Fields a price filter matches. API_TYPE is the kind of API a request went to (such as RestApi or
// LlmProvider), so that one plan can price each kind on its own; its values are free text, matched
// as written. TOKEN reads a token count from the event, at its event key; unlike the fixed fields,
// a meter may list it several times, once per key.
const (
	FilterFieldHTTPStatusCode = "HTTP_STATUS_CODE"
	FilterFieldHTTPMethod     = "HTTP_METHOD"
	FilterFieldResource       = "RESOURCE"
	FilterFieldAPIType        = "API_TYPE"
	FilterFieldToken          = "TOKEN"
)

// Operators of a price filter. The comparisons are for TOKEN only, which also takes EQUALS.
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
	// maxMeterFilterFields is how many filter fields a meter lists at most.
	maxMeterFilterFields = 10
	// maxPriceFilters is how many filters a price has at most.
	maxPriceFilters = 20
	// maxTokenConditions is how many conditions a price puts on one Token key at most.
	maxTokenConditions = 2
	// maxTokenValue is the largest number a Token condition compares with.
	maxTokenValue = 1_000_000_000_000
)

var (
	meterTypes = []string{MeterTypeRequestCount, MeterTypeTokens}
	// fixedFilterFields are the fields a meter lists, and a price filters on, at most once.
	fixedFilterFields = []string{FilterFieldHTTPStatusCode, FilterFieldHTTPMethod, FilterFieldResource, FilterFieldAPIType}
	filterFields      = append(slices.Clone(fixedFilterFields), FilterFieldToken)
	matchOperators    = []string{FilterOperatorEquals, FilterOperatorNotEquals, FilterOperatorIn, FilterOperatorNotIn}
	// filterFieldLabels are the names the UIs show for the filter fields.
	filterFieldLabels = map[string]string{
		FilterFieldHTTPStatusCode: "Status code",
		FilterFieldHTTPMethod:     "HTTP method",
		FilterFieldResource:       "Resource",
		FilterFieldAPIType:        "API type",
		FilterFieldToken:          "Token",
	}
	// filterOperators are the operators each filter field takes.
	filterOperators = map[string][]string{
		FilterFieldHTTPStatusCode: matchOperators,
		FilterFieldHTTPMethod:     matchOperators,
		FilterFieldResource:       {FilterOperatorEquals, FilterOperatorNotEquals, FilterOperatorStartsWith},
		FilterFieldAPIType:        matchOperators,
		FilterFieldToken: {FilterOperatorGreaterThan, FilterOperatorGreaterThanOrEqual, FilterOperatorLessThan,
			FilterOperatorLessThanOrEqual, FilterOperatorEquals},
	}
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
	// FilterFields are the fields the meter's prices may filter on, each with the event field it
	// reads; empty when they take no filters.
	FilterFields []MeterFilterField `json:"filterFields" db:"filter_fields"`
	// SourceUpdatedAt is when the meter last changed where it was made: the time of the request for
	// a meter made here, the event's occurred_at for a copy of an API Portal meter. An API Portal
	// event older than it is stale and ignored.
	SourceUpdatedAt time.Time `json:"sourceUpdatedAt" db:"source_updated_at"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}

// MeterFilterField is a field a meter's prices may filter on, with the event field it reads (by
// default metadata.apiType for API_TYPE; a TOKEN field has no default). The JSON form is
// the one the API Portal's meter events carry, and the one stored.
type MeterFilterField struct {
	Field    string `json:"field"`
	EventKey string `json:"event_key"`
}

// AllowsFilterOn reports whether the meter's prices may filter on the fixed field.
func (m *Meter) AllowsFilterOn(field string) bool {
	return slices.ContainsFunc(m.FilterFields, func(f MeterFilterField) bool { return f.Field == field })
}

// HasTokenKey reports whether the meter's prices may put Token conditions on the key: the meter's
// TypeKey, or the key of one of its TOKEN filter fields.
func (m *Meter) HasTokenKey(key string) bool {
	if m.TypeKey != nil && *m.TypeKey == key {
		return true
	}
	return slices.ContainsFunc(m.FilterFields, func(f MeterFilterField) bool { return f.Field == FilterFieldToken && f.EventKey == key })
}

// maxEventKeyChars bounds an event key, as its column does.
const maxEventKeyChars = 255

// ValidateMeter checks a meter against the rules the API Portal applies:
//   - the type is REQUEST_COUNT or TOKENS;
//   - a TOKENS meter has a TypeKey (the event field it adds up), and a REQUEST_COUNT meter none;
//   - each filter field is one a price can filter on, with its event key: a fixed field listed once,
//     a TOKEN field once per key and never with the TypeKey's key;
//   - at most 10 filter fields;
//   - every event key is 1 to 255 characters with no spaces.
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
	if len(m.FilterFields) > maxMeterFilterFields {
		return fmt.Errorf("A meter has at most %d filter fields", maxMeterFilterFields)
	}
	seen := make(map[string]bool, len(m.FilterFields))
	tokenKeys := make(map[string]bool)
	for _, f := range m.FilterFields {
		if _, ok := filterOperators[f.Field]; !ok {
			return fmt.Errorf("filterFields: unknown field %s", f.Field)
		}
		if f.Field == FilterFieldToken {
			if err := validateTokenField(m, f, tokenKeys); err != nil {
				return err
			}
			continue
		}
		if seen[f.Field] {
			return fmt.Errorf("filterFields: %s is listed twice", f.Field)
		}
		seen[f.Field] = true
		if !validEventKey(f.EventKey) {
			return fmt.Errorf("The event key of %s must be 1 to %d characters with no spaces", filterFieldLabels[f.Field], maxEventKeyChars)
		}
	}
	return nil
}

// validateTokenField checks a TOKEN filter field: it has a key, not one listed before (seen) and
// not the meter's TypeKey.
func validateTokenField(m *Meter, f MeterFilterField, seen map[string]bool) error {
	switch {
	case f.EventKey == "":
		return errors.New("A Token field needs an eventKey")
	case !validEventKey(f.EventKey):
		return fmt.Errorf("The event key of %s must be 1 to %d characters with no spaces", filterFieldLabels[f.Field], maxEventKeyChars)
	case m.TypeKey != nil && *m.TypeKey == f.EventKey:
		return fmt.Errorf("The key %s is already the meter's Token key", f.EventKey)
	case seen[f.EventKey]:
		return fmt.Errorf("The meter lists the Token key %s more than once", f.EventKey)
	}
	seen[f.EventKey] = true
	return nil
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
	// One status code, from 100 to 599. Classes of codes ("2xx") are not accepted.
	statusCodePattern = regexp.MustCompile(`^[1-5]\d\d$`)
	httpMethods       = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	// singleValueOperators take exactly one value; IN and NOT_IN take 1 to maxFilterValues.
	singleValueOperators = map[string]bool{
		FilterOperatorEquals: true, FilterOperatorNotEquals: true, FilterOperatorStartsWith: true,
	}
)

// PriceTier is one tier of a VOLUME or GRADUATED price. UpTo is nil on the last tier. Amounts are
// decimal strings in the currency's main unit.
type PriceTier struct {
	UpTo       *int64  `json:"up_to"`
	UnitAmount *string `json:"unit_amount"`
	FlatAmount *string `json:"flat_amount"`
}

// PriceFilter narrows the requests a price counts: a request is counted only when it matches every
// filter of the price. A TOKEN filter is a condition on the token count at EventKey, one of the
// meter's Token keys, compared with one whole number; other filters have no EventKey. The JSON form
// is the one the API Portal's subscription plan events carry for pricing, and the one stored.
type PriceFilter struct {
	Field    string   `json:"field"`
	EventKey string   `json:"event_key,omitempty"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// SubscriptionPlanPrice is a usage price of a paid plan: it charges for what the meter MeterHandle
// counts, narrowed by its Filters (none counts every request the meter counts), in one pricing
// model; the fields of other models are nil. A plan may price one meter several times, as long as
// no request is counted by two of those prices. A price has no name of its own: it is shown as the
// meter's name followed by its filters. The JSON form is the one the API Portal's subscription plan
// events carry for pricing, and the one stored.
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
// never changes.
type SubscriptionPlanPricing struct {
	SubscriptionPlanUUID string `json:"subscriptionPlanId" db:"subscription_plan_uuid"`
	OrganizationUUID     string `json:"organizationId" db:"organization_uuid"`
	PricingType          string `json:"pricingType" db:"pricing_type"`
	Currency             string `json:"currency" db:"currency"`
	BillingPeriod        string `json:"billingPeriod" db:"billing_period"`
	// FlatAmount is the amount a FLAT_RATE plan charges every billing period; nil for USAGE_BASED.
	FlatAmount *string `json:"flatAmount" db:"flat_amount"`
	// Prices are the usage prices of a USAGE_BASED plan, in order; empty for FLAT_RATE.
	Prices []SubscriptionPlanPrice `json:"prices" db:"prices"`
	// SourceUpdatedAt is when the pricing was set: the time of the request for a plan created here,
	// the event's occurred_at for a plan created in an API Portal.
	SourceUpdatedAt time.Time `json:"sourceUpdatedAt" db:"source_updated_at"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}

// ValidatePlanPricing checks a plan's pricing against the rules the API Portal applies; the API
// Portal decides, this check only refuses what it would refuse anyway. That the meters exist is left
// to the caller. The filter values are normalized in place: trimmed, an HTTP method upper-cased,
// repeated values dropped with the order kept. The returned error's text names the field and is safe
// to show.
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

// validateFilters checks the filters of a price and normalizes their values in place: at most 20
// filters; one filter per fixed field; HTTP_STATUS_CODE, HTTP_METHOD and API_TYPE take EQUALS,
// NOT_EQUALS, IN or NOT_IN, and RESOURCE takes EQUALS, NOT_EQUALS or STARTS_WITH; exactly one value
// for EQUALS, NOT_EQUALS and STARTS_WITH, 1 to 20 for IN and NOT_IN. Token conditions are checked by
// validateTokenConditions. Messages start with where ("price 2: ...").
func validateFilters(filters []PriceFilter, where string) error {
	if len(filters) > maxPriceFilters {
		return fmt.Errorf("%s: a price has at most %d filters", where, maxPriceFilters)
	}
	seen := make(map[string]bool, len(filters))
	for i := range filters {
		f := &filters[i]
		operators, ok := filterOperators[f.Field]
		if !ok {
			return fmt.Errorf("field of filter %d of %s must be one of %s", i+1, where, strings.Join(filterFields, ", "))
		}
		f.EventKey = strings.TrimSpace(f.EventKey)
		if f.Field == FilterFieldToken {
			if err := validateTokenCondition(f, operators, where); err != nil {
				return err
			}
			continue
		}
		if f.EventKey != "" {
			return fmt.Errorf("%s: only a Token filter takes an eventKey", where)
		}
		if seen[f.Field] {
			return fmt.Errorf("%s filters on %s more than once", where, f.Field)
		}
		seen[f.Field] = true
		if !slices.Contains(operators, f.Operator) {
			return fmt.Errorf("The %s operator of %s must be one of %s", f.Field, where, strings.Join(operators, ", "))
		}
		if singleValueOperators[f.Operator] {
			if len(f.Values) != 1 {
				return fmt.Errorf("%s: Operator %s takes exactly one value", where, f.Operator)
			}
		} else if len(f.Values) == 0 || len(f.Values) > maxFilterValues {
			return fmt.Errorf("%s: A filter on %s needs 1 to %d values", where, f.Field, maxFilterValues)
		}
		values := make([]string, 0, len(f.Values))
		for _, raw := range f.Values {
			value, err := normalizeFilterValue(f.Field, raw)
			if err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			if f.Operator == FilterOperatorStartsWith {
				value = normalizePath(value)
			}
			if !slices.Contains(values, value) {
				values = append(values, value)
			}
		}
		f.Values = values
	}
	return validateTokenConditions(filters, where)
}

// validateTokenCondition checks one Token condition and normalizes its value: it names a key, takes
// a comparison operator (or EQUALS) and exactly one whole number from 0 to 1000000000000. Whether the
// meter has the key is left to the caller, who knows the meter.
func validateTokenCondition(f *PriceFilter, operators []string, where string) error {
	if f.EventKey == "" {
		return fmt.Errorf("%s: a Token filter needs an eventKey", where)
	}
	if !slices.Contains(operators, f.Operator) {
		return fmt.Errorf("The %s operator of %s must be one of %s", f.Field, where, strings.Join(operators, ", "))
	}
	if len(f.Values) != 1 {
		return fmt.Errorf("%s: the Token value must be a whole number from 0 to %d", where, int64(maxTokenValue))
	}
	n, ok := parseTokenValue(f.Values[0])
	if !ok {
		return fmt.Errorf("%s: the Token value must be a whole number from 0 to %d", where, int64(maxTokenValue))
	}
	f.Values = []string{strconv.FormatInt(n, 10)}
	return nil
}

// parseTokenValue reads a whole number from 0 to maxTokenValue, written with digits only.
func parseTokenValue(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsFunc(value, func(r rune) bool { return r < '0' || r > '9' }) {
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n > maxTokenValue {
		return 0, false
	}
	return n, true
}

// validateTokenConditions checks that a price puts at most two conditions on each Token key, and
// that the conditions on a key can all be true together.
func validateTokenConditions(filters []PriceFilter, where string) error {
	for _, key := range tokenKeysOf(filters) {
		conditions := 0
		for _, f := range filters {
			if f.Field == FilterFieldToken && f.EventKey == key {
				conditions++
			}
		}
		if conditions > maxTokenConditions {
			return fmt.Errorf("%s: a price puts at most %d Token conditions on %s", where, maxTokenConditions, key)
		}
		if tokenRangeOf(filters, key).empty() {
			return fmt.Errorf("%s: the Token conditions on %s can't all be true", where, key)
		}
	}
	return nil
}

// tokenKeysOf returns the Token keys the filters put conditions on, in order, each once.
func tokenKeysOf(filters []PriceFilter) []string {
	var keys []string
	for _, f := range filters {
		if f.Field == FilterFieldToken && !slices.Contains(keys, f.EventKey) {
			keys = append(keys, f.EventKey)
		}
	}
	return keys
}

// tokenRange is the whole numbers from low to high, both included; empty when low > high.
type tokenRange struct {
	low, high int64
}

func (r tokenRange) empty() bool { return r.low > r.high }

// tokenRangeOf returns the token counts at key that the filters' Token conditions all accept: every
// count when there are none. The values have been checked by validateTokenCondition.
func tokenRangeOf(filters []PriceFilter, key string) tokenRange {
	r := tokenRange{low: 0, high: math.MaxInt64}
	for _, f := range filters {
		if f.Field != FilterFieldToken || f.EventKey != key || len(f.Values) != 1 {
			continue
		}
		n, ok := parseTokenValue(f.Values[0])
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
		case FilterOperatorEquals:
			r.low, r.high = max(r.low, n), min(r.high, n)
		}
	}
	return r
}

// normalizeFilterValue checks one value of a filter on the field and returns it trimmed, an HTTP
// method in upper case.
func normalizeFilterValue(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxFilterValueChars {
		return "", fmt.Errorf("Filter values for %s must be 1 to %d characters", field, maxFilterValueChars)
	}
	switch field {
	case FilterFieldHTTPStatusCode:
		if !statusCodePattern.MatchString(value) {
			return "", fmt.Errorf("\"%s\" is not a status code such as 200 or 404", value)
		}
	case FilterFieldHTTPMethod:
		value = strings.ToUpper(value)
		if !slices.Contains(httpMethods, value) {
			return "", fmt.Errorf("HTTP method must be one of %s", strings.Join(httpMethods, ", "))
		}
	case FilterFieldResource:
		if !strings.HasPrefix(value, "/") {
			return "", errors.New("A resource must start with /")
		}
	}
	return value, nil
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
// every fixed field either price filters on, the two filters can match the same value, and for every
// Token key either price has conditions on, the two ranges of token counts share a number. A field
// or key only one price filters on matches any value, or number, for the other.
func filtersOverlap(a, b []PriceFilter) bool {
	for _, field := range fixedFilterFields {
		fa, inA := filterOn(a, field)
		fb, inB := filterOn(b, field)
		if inA && inB && !filterValuesOverlap(field, fa, fb) {
			return false
		}
	}
	for _, key := range tokenKeysOf(append(slices.Clone(a), b...)) {
		ra, rb := tokenRangeOf(a, key), tokenRangeOf(b, key)
		if (tokenRange{low: max(ra.low, rb.low), high: min(ra.high, rb.high)}).empty() {
			return false
		}
	}
	return true
}

func filterOn(filters []PriceFilter, field string) (PriceFilter, bool) {
	for _, f := range filters {
		if f.Field == field {
			return f, true
		}
	}
	return PriceFilter{}, false
}

// normalizePath returns a STARTS_WITH path as stored: without a trailing "/", so "/orders/" is
// "/orders"; "/" stays "/".
func normalizePath(path string) string {
	if trimmed := strings.TrimRight(path, "/"); trimmed != "" {
		return trimmed
	}
	return "/"
}

// underPath reports whether a resource is the path or under it, by whole path segments, as a router
// matches a path prefix: "/orders" covers "/orders" and "/orders/{id}", not "/orders-archive"; "/"
// covers every resource. This is what a STARTS_WITH filter counts (the API Portal's underPath).
func underPath(resource, path string) bool {
	base := normalizePath(path)
	return base == "/" || resource == base || strings.HasPrefix(resource, base+"/")
}

// Kinds of filter operator, in the order filterValuesOverlap compares them.
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

// filterValuesOverlap reports whether some value of the field matches both filters:
//   - EQUALS/IN with EQUALS/IN: the lists share a value;
//   - EQUALS/IN with STARTS_WITH: a value of the list is the path or under it (underPath);
//   - EQUALS/IN with NOT_EQUALS/NOT_IN: a value of the first list is not in the second;
//   - STARTS_WITH with STARTS_WITH: one path is the other or under it;
//   - NOT_EQUALS/NOT_IN with NOT_EQUALS/NOT_IN or STARTS_WITH: always, since values outside the lists
//     remain, except for an HTTP method, which has only seven values: there the two lists must
//     together leave one out.
func filterValuesOverlap(field string, a, b PriceFilter) bool {
	if operatorKind(b.Operator) < operatorKind(a.Operator) {
		a, b = b, a
	}
	switch {
	case operatorKind(a.Operator) == matchesValues && operatorKind(b.Operator) == matchesValues:
		return slices.ContainsFunc(a.Values, func(v string) bool { return slices.Contains(b.Values, v) })
	case operatorKind(a.Operator) == matchesValues && operatorKind(b.Operator) == matchesPrefix:
		return slices.ContainsFunc(a.Values, func(v string) bool { return underPath(v, b.Values[0]) })
	case operatorKind(a.Operator) == matchesValues:
		return slices.ContainsFunc(a.Values, func(v string) bool { return !slices.Contains(b.Values, v) })
	case operatorKind(a.Operator) == matchesPrefix && operatorKind(b.Operator) == matchesPrefix:
		return underPath(a.Values[0], b.Values[0]) || underPath(b.Values[0], a.Values[0])
	case field != FilterFieldHTTPMethod:
		return true
	default:
		return slices.ContainsFunc(httpMethods, func(m string) bool {
			return !slices.Contains(a.Values, m) && !slices.Contains(b.Values, m)
		})
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
