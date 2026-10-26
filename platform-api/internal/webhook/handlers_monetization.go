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

package webhook

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Meter event types. The API Portal owns its meters; these events keep the copies here current. A
// plan's pricing comes with the plan, on its subscription_plan.* events.
const (
	EventMeterCreated        = "meter.created"
	EventMeterUpdated        = "meter.updated"
	EventMeterDeleted        = "meter.deleted"
	maxMeterHandleLength     = 40
	maxMeterDescriptionChars = 1023
)

// meterData is the data payload of meter.* events; handle is the only field on meter.deleted.
// type_key is the event field a TOKENS meter adds up, null for a REQUEST_COUNT meter.
// filter_fields are the fields the meter's prices may filter on, each with the event field it reads.
// The keys are the meter's own, as stored in the API Portal, and are copied as they are.
type meterData struct {
	Handle       string                   `json:"handle"`
	DisplayName  string                   `json:"display_name"`
	Description  string                   `json:"description"`
	Type         string                   `json:"type"`
	TypeKey      *string                  `json:"type_key"`
	FilterFields []model.MeterFilterField `json:"filter_fields"`
}

// planPricingData is the pricing on subscription_plan.* events: a FLAT_RATE plan's amount or a
// USAGE_BASED plan's prices, in the form they are stored in (model.SubscriptionPlanPrice); a Token
// condition carries its key as event_key.
type planPricingData struct {
	PricingType   string                        `json:"pricing_type"`
	Currency      string                        `json:"currency"`
	BillingPeriod string                        `json:"billing_period"`
	FlatAmount    *string                       `json:"flat_amount"`
	Prices        []model.SubscriptionPlanPrice `json:"prices"`
}

// occurredAt is when the API Portal made the change. The portal writes it from its own clock;
// an absent or unreadable value is taken as now, so the change is applied rather than lost.
func occurredAt(env *Envelope) time.Time {
	value := strings.TrimSpace(env.OccurredAt)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

func requireHandle(field, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: data.%s is required", ErrInvalidEnvelope, field)
	}
	if utf8.RuneCountInString(value) > maxLength {
		return "", fmt.Errorf("%w: data.%s must be at most %d characters", ErrInvalidEnvelope, field, maxLength)
	}
	return value, nil
}

// handleMeterUpserted copies a meter created or updated in the API Portal. Both events upsert, so
// a missed created event is recovered by the next update.
func (r *Receiver) handleMeterUpserted(ctx context.Context, env *Envelope) error {
	var d meterData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	handle, err := requireHandle("handle", d.Handle, maxMeterHandleLength)
	if err != nil {
		return err
	}
	// Lengths are in characters, as the API Portal and the database columns count them, not bytes:
	// a name in a non-Latin script takes two to four bytes a character.
	name := strings.TrimSpace(d.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > 255 {
		return fmt.Errorf("%w: data.display_name is required and must be at most 255 characters", ErrInvalidEnvelope)
	}
	if utf8.RuneCountInString(d.Description) > maxMeterDescriptionChars {
		return fmt.Errorf("%w: data.description must be at most %d characters", ErrInvalidEnvelope, maxMeterDescriptionChars)
	}
	var typeKey *string
	if d.TypeKey != nil {
		key := strings.TrimSpace(*d.TypeKey)
		typeKey = &key
	}
	filterFields := make([]model.MeterFilterField, 0, len(d.FilterFields))
	for _, f := range d.FilterFields {
		filterFields = append(filterFields, model.MeterFilterField{
			Field:    strings.ToUpper(strings.TrimSpace(f.Field)),
			EventKey: strings.TrimSpace(f.EventKey),
		})
	}
	meter := &model.Meter{
		OrganizationUUID: env.OrgID,
		Handle:           handle,
		DisplayName:      name,
		Description:      d.Description,
		Type:             strings.ToUpper(strings.TrimSpace(d.Type)),
		TypeKey:          typeKey,
		FilterFields:     filterFields,
		SourceUpdatedAt:  occurredAt(env),
	}
	if err := model.ValidateMeter(meter); err != nil {
		return fmt.Errorf("%w: data: %s", ErrInvalidEnvelope, err.Error())
	}
	return r.monetization.UpsertMeterFromWebhook(meter)
}

// handleMeterDeleted deletes the copy of a meter deleted in the API Portal.
func (r *Receiver) handleMeterDeleted(ctx context.Context, env *Envelope) error {
	var d meterData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	handle, err := requireHandle("handle", d.Handle, maxMeterHandleLength)
	if err != nil {
		return err
	}
	return r.monetization.DeleteMeterFromWebhook(env.OrgID, handle, occurredAt(env))
}

// toPricing reads the pricing of an API Portal plan, made at occurredAt. The pricing
// must follow the rules the API Portal applies to it (model.ValidatePlanPricing).
func (d *planPricingData) toPricing(orgID string, occurredAt time.Time) (*model.SubscriptionPlanPricing, error) {
	prices := d.Prices
	if prices == nil {
		prices = []model.SubscriptionPlanPrice{}
	}
	for i := range prices {
		prices[i].MeterHandle = strings.TrimSpace(prices[i].MeterHandle)
		prices[i].PricingModel = strings.ToUpper(strings.TrimSpace(prices[i].PricingModel))
		if prices[i].Filters == nil {
			prices[i].Filters = []model.PriceFilter{}
		}
		for j := range prices[i].Filters {
			prices[i].Filters[j].Field = strings.ToUpper(strings.TrimSpace(prices[i].Filters[j].Field))
			prices[i].Filters[j].Operator = strings.ToUpper(strings.TrimSpace(prices[i].Filters[j].Operator))
		}
	}
	pricing := &model.SubscriptionPlanPricing{
		OrganizationUUID: orgID,
		PricingType:      strings.ToUpper(strings.TrimSpace(d.PricingType)),
		Currency:         strings.ToUpper(strings.TrimSpace(d.Currency)),
		BillingPeriod:    strings.ToUpper(strings.TrimSpace(d.BillingPeriod)),
		FlatAmount:       d.FlatAmount,
		Prices:           prices,
		CreatedAt:        occurredAt,
	}
	if err := model.ValidatePlanPricing(pricing); err != nil {
		return nil, fmt.Errorf("%w: data.pricing: %s", ErrInvalidEnvelope, err.Error())
	}
	return pricing, nil
}
