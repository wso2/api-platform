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

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Subscription plan event types. Plans created or changed in the API Portal are copied here so
// that the gateways receive them; plans created through the REST API reach the gateways directly.
const (
	EventSubscriptionPlanCreated = "subscription_plan.created"
	EventSubscriptionPlanUpdated = "subscription_plan.updated"
	EventSubscriptionPlanDeleted = "subscription_plan.deleted"
)

// subscriptionPlanData is the data payload for subscription_plan.* events. handle is the handle
// the Platform API knows the plan by (the API Portal's ref_id for the plan); it is the only field
// on subscription_plan.deleted.
type subscriptionPlanData struct {
	Handle      string                  `json:"handle"`
	DisplayName string                  `json:"display_name"`
	Description string                  `json:"description"`
	Limits      []subscriptionPlanLimit `json:"limits"`
	// Pricing is the plan's pricing, or null for a free plan; it never changes.
	Pricing *planPricingData `json:"pricing"`
}

// subscriptionPlanLimit is one API Portal plan limit. limit_count -1 means unlimited.
type subscriptionPlanLimit struct {
	LimitType  string `json:"limit_type"`
	LimitCount int    `json:"limit_count"`
	TimeUnit   string `json:"time_unit"`
	TimeAmount int    `json:"time_amount"`
}

// requestLimit maps the API Portal limits onto the single request limit a plan carries here: the
// first finite REQUEST_COUNT limit with a supported time unit. Unlimited plans and other limit
// types map to no limit. The Platform API counts per one time unit, so a limit over several
// units ("1000 per 2 hours") is applied per unit, which is stricter, never looser.
func requestLimit(limits []subscriptionPlanLimit) (*int, string) {
	for _, l := range limits {
		if !strings.EqualFold(l.LimitType, constants.LimitTypeRequestCount) || l.LimitCount < 1 {
			continue
		}
		unit := strings.ToUpper(strings.TrimSpace(l.TimeUnit))
		if !constants.ValidThrottleLimitUnits[unit] {
			continue
		}
		count := l.LimitCount
		return &count, unit
	}
	return nil, ""
}

func (d *subscriptionPlanData) validateHandle() error {
	handle, err := requireHandle("handle", d.Handle, model.MaxSubscriptionPlanHandleChars)
	d.Handle = handle
	return err
}

// handleSubscriptionPlanUpserted reconciles a plan created or updated in the API Portal. Both
// events upsert and carry the plan's pricing, so a missed created event is recovered by the next
// update, pricing included.
func (r *Receiver) handleSubscriptionPlanUpserted(ctx context.Context, env *Envelope) error {
	var d subscriptionPlanData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validateHandle(); err != nil {
		return err
	}
	name := strings.TrimSpace(d.DisplayName)
	if name == "" {
		return fmt.Errorf("%w: data.display_name is required", ErrInvalidEnvelope)
	}
	var pricing *model.SubscriptionPlanPricing
	if d.Pricing != nil {
		var err error
		if pricing, err = d.Pricing.toPricing(env.OrgID, occurredAt(env)); err != nil {
			return err
		}
	}
	limitCount, limitUnit := requestLimit(d.Limits)
	_, err := r.plans.UpsertPlanFromWebhook(env.OrgID, d.Handle, name, limitCount, limitUnit, pricing, occurredAt(env))
	return err
}

// handleSubscriptionPlanDeleted follows a plan deleted in the API Portal: the plan is removed from
// what offers or uses it here and deleted. A plan that is not here answers 200.
func (r *Receiver) handleSubscriptionPlanDeleted(ctx context.Context, env *Envelope) error {
	var d subscriptionPlanData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validateHandle(); err != nil {
		return err
	}
	return r.plans.DeletePlanFromWebhook(env.OrgID, d.Handle, occurredAt(env))
}
