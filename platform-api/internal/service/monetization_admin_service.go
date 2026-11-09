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
package service

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// MonetizationAdminService serves the meters and plan pricing REST API the Management Portal uses.
//   - A meter made here is kept here and checked with the API Portal's rules. It reaches an API
//     Portal only when a plan that charges for it is published there (PublicationService's plan
//     sync); a later change or delete made here is not sent to API Portals.
//   - A meter made in an API Portal is a copy, kept current by that portal's webhook events.
//   - Plan pricing is set when a plan is created (SubscriptionPlanService), never changes, and
//     reaches an API Portal with the plan.
type MonetizationAdminService struct {
	meterRepo   repository.MeterRepository
	pricingRepo repository.SubscriptionPlanPricingRepository
	planRepo    repository.SubscriptionPlanRepository
	slogger     *slog.Logger
}

// NewMonetizationAdminService creates the service.
func NewMonetizationAdminService(
	meterRepo repository.MeterRepository,
	pricingRepo repository.SubscriptionPlanPricingRepository,
	planRepo repository.SubscriptionPlanRepository,
	slogger *slog.Logger,
) *MonetizationAdminService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &MonetizationAdminService{meterRepo: meterRepo, pricingRepo: pricingRepo, planRepo: planRepo, slogger: slogger}
}

// ListMeters returns the organization's meters, those whose name or handle contains search
// (ignoring case) when it is not empty.
func (s *MonetizationAdminService) ListMeters(orgUUID, search string) ([]*model.Meter, error) {
	meters, err := s.meterRepo.ListByOrganization(orgUUID)
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to list meters for org %s", orgUUID))
	}
	if search = strings.ToLower(strings.TrimSpace(search)); search != "" {
		meters = slices.DeleteFunc(meters, func(m *model.Meter) bool {
			return !strings.Contains(strings.ToLower(m.DisplayName), search) && !strings.Contains(strings.ToLower(m.Handle), search)
		})
	}
	return meters, nil
}

// GetMeter returns the meter with the handle.
func (s *MonetizationAdminService) GetMeter(orgUUID, handle string) (*model.Meter, error) {
	meter, err := s.meterRepo.GetByHandle(orgUUID, handle)
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get meter %s for org %s", handle, orgUUID))
	}
	if meter == nil {
		return nil, apperror.MeterNotFound.New()
	}
	return meter, nil
}

// GetPlanPricing returns the pricing of the plan with the handle.
func (s *MonetizationAdminService) GetPlanPricing(orgUUID, planHandle string) (*model.SubscriptionPlanPricing, error) {
	plan, err := s.plan(orgUUID, planHandle)
	if err != nil {
		return nil, err
	}
	pricing, err := s.pricingRepo.GetByPlan(plan.UUID, orgUUID)
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get pricing of plan %s", planHandle))
	}
	if pricing == nil {
		return nil, apperror.SubscriptionPlanPricingNotFound.New()
	}
	return pricing, nil
}

// CreateMeter saves a new meter, checked with the API Portal's rules (model.ParseMeterRequest): the
// handle is the request's id, or is derived from the display name.
func (s *MonetizationAdminService) CreateMeter(orgUUID string, body []byte) (*model.Meter, error) {
	meter, err := model.ParseMeterRequest(body, true)
	if err != nil {
		return nil, apperror.ValidationFailed.New(err.Error())
	}
	meter.OrganizationUUID = orgUUID
	meter.SourceUpdatedAt = time.Now().UTC()
	if err := s.meterRepo.Create(meter); err != nil {
		if errors.Is(err, repository.ErrMeterExists) {
			return nil, apperror.MeterExists.New(meter.Handle)
		}
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to create meter %s for org %s", meter.Handle, orgUUID))
	}
	return meter, nil
}

// UpdateMeter replaces the meter's settings with the request's. A meter that a plan's pricing here
// uses cannot change at all, its event keys included.
func (s *MonetizationAdminService) UpdateMeter(orgUUID, handle string, body []byte) (*model.Meter, error) {
	existing, err := s.GetMeter(orgUUID, handle)
	if err != nil {
		return nil, err
	}
	plans, err := s.meterUsage(orgUUID, handle)
	if err != nil {
		return nil, err
	}
	if len(plans) > 0 {
		return nil, apperror.MeterInUse.New("The meter is in use, so it can't be updated.")
	}
	meter, err := model.ParseMeterRequest(body, false)
	if err != nil {
		return nil, apperror.ValidationFailed.New(err.Error())
	}
	if meter.Handle != "" && meter.Handle != existing.Handle {
		return nil, apperror.ValidationFailed.New("The id of a meter cannot be changed")
	}
	meter.UUID, meter.OrganizationUUID, meter.Handle = existing.UUID, orgUUID, existing.Handle
	meter.SourceUpdatedAt = time.Now().UTC()
	if err := s.meterRepo.Upsert(meter); err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to update meter %s for org %s", handle, orgUUID))
	}
	return meter, nil
}

// DeleteMeter deletes the meter. A meter that a plan's pricing here uses cannot be deleted.
func (s *MonetizationAdminService) DeleteMeter(orgUUID, handle string) error {
	if _, err := s.GetMeter(orgUUID, handle); err != nil {
		return err
	}
	plans, err := s.meterUsage(orgUUID, handle)
	if err != nil {
		return err
	}
	if len(plans) > 0 {
		return apperror.MeterInUse.New("The meter is in use, so it can't be deleted.")
	}
	if err := s.meterRepo.Delete(orgUUID, handle); err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to delete meter %s for org %s", handle, orgUUID))
	}
	return nil
}

// MeterUsage returns, by meter handle, the handles of the plans whose pricing here has a price on
// the meter, in handle order. A meter no plan prices is absent.
func (s *MonetizationAdminService) MeterUsage(orgUUID string) (map[string][]string, error) {
	pricings, err := s.pricingRepo.ListByOrganization(orgUUID)
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to list pricing of org %s", orgUUID))
	}
	planUUIDs := make(map[string][]string) // meter handle -> plan UUIDs, each once
	var all []string
	for _, pricing := range pricings {
		for _, price := range pricing.Prices {
			if !slices.Contains(planUUIDs[price.MeterHandle], pricing.SubscriptionPlanUUID) {
				planUUIDs[price.MeterHandle] = append(planUUIDs[price.MeterHandle], pricing.SubscriptionPlanUUID)
				all = append(all, pricing.SubscriptionPlanUUID)
			}
		}
	}
	usage := make(map[string][]string, len(planUUIDs))
	if len(all) == 0 {
		return usage, nil
	}
	handles, err := s.planRepo.GetHandlesByIDs(all, orgUUID)
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get the plans of org %s", orgUUID))
	}
	for meter, uuids := range planUUIDs {
		var plans []string
		for _, planUUID := range uuids {
			if plan, ok := handles[planUUID]; ok {
				plans = append(plans, plan)
			}
		}
		if len(plans) > 0 {
			slices.Sort(plans)
			usage[meter] = plans
		}
	}
	return usage, nil
}

// meterUsage returns the handles of the plans whose pricing here has a price on the meter, in
// handle order.
func (s *MonetizationAdminService) meterUsage(orgUUID, handle string) ([]string, error) {
	usage, err := s.MeterUsage(orgUUID)
	if err != nil {
		return nil, err
	}
	return usage[handle], nil
}

// planPricingRequest is the pricing of POST /subscription-plans: a FLAT_RATE plan's amount or a
// USAGE_BASED plan's usage prices. It is also the shape the API Portal takes, so a plan's pricing is
// sent to the portal in it when the plan is published there.
type planPricingRequest struct {
	PricingType   string             `json:"pricingType"`
	Currency      string             `json:"currency"`
	BillingPeriod string             `json:"billingPeriod"`
	FlatAmount    *string            `json:"flatAmount,omitempty"`
	Prices        []planPriceRequest `json:"prices,omitempty"`
}

// planPriceRequest is one usage price of a planPricingRequest. A price has no name: it is shown as
// its meter's name followed by its filters.
type planPriceRequest struct {
	MeterID      string               `json:"meterId"`
	Filters      []priceFilterRequest `json:"filters"`
	PricingModel string               `json:"pricingModel"`
	UnitAmount   *string              `json:"unitAmount,omitempty"`
	UnitQuantity *int64               `json:"unitQuantity,omitempty"`
	Tiers        []priceTierRequest   `json:"tiers,omitempty"`
}

// priceFilterRequest is one filter of a planPriceRequest: the name of a property of the price's meter,
// an operator and its values.
type priceFilterRequest struct {
	Field    string       `json:"field"`
	Operator string       `json:"operator"`
	Values   filterValues `json:"values"`
}

// filterValues are the values of a priceFilterRequest. A value may be a JSON string or a JSON number;
// a number is kept as written.
type filterValues []string

func (v *filterValues) UnmarshalJSON(data []byte) error {
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	if items == nil {
		*v = nil
		return nil
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		var text string
		if err := json.Unmarshal(item, &text); err == nil {
			values = append(values, text)
			continue
		}
		var number json.Number
		if err := json.Unmarshal(item, &number); err != nil {
			return err
		}
		values = append(values, number.String())
	}
	*v = values
	return nil
}

// priceTierRequest is one tier of a VOLUME or GRADUATED planPriceRequest; upTo is null on the last.
type priceTierRequest struct {
	UpTo       *int64  `json:"upTo"`
	UnitAmount *string `json:"unitAmount"`
	FlatAmount *string `json:"flatAmount"`
}

// parsePlanPricing checks a pricing request against the rules the API Portal applies, all but that
// the meters exist, and returns the pricing with the codes upper-cased and the filter values
// normalized. A field that is not part of the pricing is refused rather than dropped, so a mistyped
// field can't leave a plan priced other than meant (pricing can't change once the plan exists).
func parsePlanPricing(body []byte) (*model.SubscriptionPlanPricing, error) {
	if refusal := unknownPricingField(body, pricingRequestParts, ""); refusal != "" {
		return nil, apperror.ValidationFailed.New(refusal)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var req planPricingRequest
	if err := decoder.Decode(&req); err != nil {
		return nil, apperror.ValidationFailed.New("The request body is not a valid plan pricing")
	}
	pricing := &model.SubscriptionPlanPricing{
		PricingType:   strings.ToUpper(strings.TrimSpace(req.PricingType)),
		Currency:      strings.ToUpper(strings.TrimSpace(req.Currency)),
		BillingPeriod: strings.ToUpper(strings.TrimSpace(req.BillingPeriod)),
		FlatAmount:    req.FlatAmount,
		Prices:        make([]model.SubscriptionPlanPrice, 0, len(req.Prices)),
	}
	for _, p := range req.Prices {
		pricing.Prices = append(pricing.Prices, parsePlanPrice(p))
	}
	if err := model.ValidatePlanPricing(pricing); err != nil {
		return nil, apperror.ValidationFailed.New(err.Error())
	}
	return pricing, nil
}

// pricingRequestPart is one part of a pricing request (the pricing, a price, a filter or a tier) as
// unknownPricingField sees it: its name in messages, the fields it takes, the fields only a response
// carries, and the fields that hold a list of other parts.
type pricingRequestPart struct {
	name     string
	fields   []string
	readOnly []string
	lists    map[string]*pricingRequestPart
}

// pricingRequestParts describes a pricing request from its request types, so it keeps their fields.
// The read-only fields are those the API Portal's pricing carries in responses.
var pricingRequestParts = func() *pricingRequestPart {
	filter := &pricingRequestPart{name: "filter", fields: jsonFieldNames(priceFilterRequest{})}
	tier := &pricingRequestPart{name: "tier", fields: jsonFieldNames(priceTierRequest{})}
	price := &pricingRequestPart{name: "price", fields: jsonFieldNames(planPriceRequest{}),
		lists: map[string]*pricingRequestPart{"filters": filter, "tiers": tier}}
	return &pricingRequestPart{fields: jsonFieldNames(planPricingRequest{}),
		readOnly: []string{"createdBy", "updatedBy", "createdAt", "updatedAt"},
		lists:    map[string]*pricingRequestPart{"prices": price}}
}()

// unknownPricingField returns the refusal of the first field of a pricing request that its part does
// not take, named by its place as the API Portal names it: "colour is not a known field", "colour of
// filter 2 of price 1 is not a known field", "createdAt is read-only". Like the portal's schema check,
// it looks at all the fields of a part before the parts it holds. It returns "" when every field is
// known, and leaves a body that is not the JSON the pricing takes to the decoder.
func unknownPricingField(raw []byte, part *pricingRequestPart, where string) string {
	keys, values, ok := jsonObjectFields(raw)
	if !ok {
		return ""
	}
	named := func(key string) string {
		if where == "" {
			return key
		}
		return key + " of " + where
	}
	for _, key := range keys {
		if slices.Contains(part.readOnly, key) {
			return named(key) + " is read-only"
		}
		if !slices.Contains(part.fields, key) {
			return named(key) + " is not a known field"
		}
	}
	for _, key := range keys {
		inner := part.lists[key]
		var items []json.RawMessage
		if inner == nil || json.Unmarshal(values[key], &items) != nil {
			continue
		}
		for i, item := range items {
			at := fmt.Sprintf("%s %d", inner.name, i+1)
			if where != "" {
				at += " of " + where
			}
			if refusal := unknownPricingField(item, inner, at); refusal != "" {
				return refusal
			}
		}
	}
	return ""
}

// jsonObjectFields returns the names of a JSON object's fields in the order written, and their
// values; ok is false when raw is not a JSON object.
func jsonObjectFields(raw []byte) (keys []string, values map[string]json.RawMessage, ok bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, nil, false
	}
	values = map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, nil, false
		}
		key, _ := token.(string)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, nil, false
		}
		keys = append(keys, key)
		values[key] = value
	}
	return keys, values, true
}

// jsonFieldNames returns the JSON names of a struct's fields.
func jsonFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ","); name != "" && name != "-" {
			names = append(names, name)
		}
	}
	return names
}

// parsePlanPrice returns a requested price with the codes upper-cased and the meter handle and the
// filters' property names trimmed.
func parsePlanPrice(p planPriceRequest) model.SubscriptionPlanPrice {
	price := model.SubscriptionPlanPrice{
		MeterHandle:  strings.TrimSpace(p.MeterID),
		Filters:      make([]model.PriceFilter, 0, len(p.Filters)),
		PricingModel: strings.ToUpper(strings.TrimSpace(p.PricingModel)),
		UnitAmount:   p.UnitAmount,
		UnitQuantity: p.UnitQuantity,
	}
	for _, f := range p.Filters {
		price.Filters = append(price.Filters, model.PriceFilter{
			Field:    strings.TrimSpace(f.Field),
			Operator: strings.ToUpper(strings.TrimSpace(f.Operator)),
			Values:   f.Values,
		})
	}
	if p.Tiers != nil {
		price.Tiers = make([]model.PriceTier, 0, len(p.Tiers))
		for _, t := range p.Tiers {
			price.Tiers = append(price.Tiers, model.PriceTier{UpTo: t.UpTo, UnitAmount: t.UnitAmount, FlatAmount: t.FlatAmount})
		}
	}
	return price
}

// portalPricing is a plan's pricing as the API Portal takes it; a FLAT_RATE plan is sent without
// prices.
func portalPricing(p *model.SubscriptionPlanPricing) *planPricingRequest {
	req := &planPricingRequest{
		PricingType:   p.PricingType,
		Currency:      p.Currency,
		BillingPeriod: p.BillingPeriod,
		FlatAmount:    p.FlatAmount,
		Prices:        make([]planPriceRequest, 0, len(p.Prices)),
	}
	for _, price := range p.Prices {
		filters := make([]priceFilterRequest, 0, len(price.Filters))
		for _, f := range price.Filters {
			filters = append(filters, priceFilterRequest{Field: f.Field, Operator: f.Operator, Values: f.Values})
		}
		body := planPriceRequest{
			MeterID:      price.MeterHandle,
			Filters:      filters,
			PricingModel: price.PricingModel,
			UnitAmount:   price.UnitAmount,
			UnitQuantity: price.UnitQuantity,
		}
		for _, t := range price.Tiers {
			body.Tiers = append(body.Tiers, priceTierRequest{UpTo: t.UpTo, UnitAmount: t.UnitAmount, FlatAmount: t.FlatAmount})
		}
		req.Prices = append(req.Prices, body)
	}
	return req
}

// checkPricedMeters checks that each price of pricing counts with a meter the organization has and
// that each of its filters names a property of that meter, ignoring case. Each filter is given the
// property's name as the meter spells it.
func checkPricedMeters(meterRepo repository.MeterRepository, orgUUID string, pricing *model.SubscriptionPlanPricing) error {
	meters := make(map[string]*model.Meter, len(pricing.Prices))
	for i, price := range pricing.Prices {
		meter, ok := meters[price.MeterHandle]
		if !ok {
			var err error
			meter, err = meterRepo.GetByHandle(orgUUID, price.MeterHandle)
			if err != nil {
				return apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get meter %s", price.MeterHandle))
			}
			if meter == nil {
				return apperror.ValidationFailed.New(fmt.Sprintf("Unknown meter: %s", price.MeterHandle))
			}
			meters[price.MeterHandle] = meter
		}
		for k, f := range price.Filters {
			property, ok := meter.Property(f.Field)
			if !ok {
				return apperror.ValidationFailed.New(
					fmt.Sprintf("price %d: the meter %s has no property %s", i+1, price.MeterHandle, f.Field))
			}
			price.Filters[k].Field = property.Field
		}
	}
	return nil
}

// plan returns the organization's plan with the handle, or SubscriptionPlanNotFound.
func (s *MonetizationAdminService) plan(orgUUID, planHandle string) (*model.SubscriptionPlan, error) {
	plan, err := s.planRepo.GetByHandleAndOrg(planHandle, orgUUID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && plan == nil) {
		return nil, apperror.SubscriptionPlanNotFound.New()
	}
	if err != nil {
		return nil, apperror.Internal.Wrap(err).WithLogMessage(fmt.Sprintf("failed to get plan %s", planHandle))
	}
	return plan, nil
}
