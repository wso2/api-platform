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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Subscription event types.
const (
	EventSubscriptionCreated          = "subscription.created"
	EventSubscriptionUpdated          = "subscription.updated"           // status change (ACTIVE ↔ INACTIVE)
	EventSubscriptionPlanChanged      = "subscription.plan_changed"      // plan switched; token unchanged
	EventSubscriptionTokenRegenerated = "subscription.token_regenerated" // token rotated (encrypted)
	EventSubscriptionDeleted          = "subscription.deleted"
)

// subscriptionPlanRef identifies a subscription plan. ref_id is the Platform API subscription plan
// handle (unique per organization), which the service layer resolves to the plan's UUID.
type subscriptionPlanRef struct {
	RefID    string `json:"ref_id"`
	PlanName string `json:"plan_name"`
	Status   string `json:"status"`
}

// subscriptionData is the data payload for subscription.* events.
//
// subscriber_id is required: a Platform API subscription belongs to a subscriber (the subscriptions
// table enforces NOT NULL). application_id is optional.
//
// token carries the subscription token issued by the Developer Portal — the value shown to the user
// and presented to the gateway for validation. It arrives as an encrypted envelope (listed in the
// envelope's encrypted_fields as "token"); once decrypted it is persisted as-is so the same token
// validates end to end. When absent, the Platform API generates its own.
type subscriptionData struct {
	SubscriberID     string              `json:"subscriber_id"`
	ApplicationID    string              `json:"application_id"`
	Token            *EncryptedKey       `json:"token"`
	SubscriptionPlan subscriptionPlanRef `json:"subscription_plan"`
	API              apiRef              `json:"api"`
	Status           string              `json:"status"`
}

func (d *subscriptionData) validate() error {
	if err := d.API.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(d.SubscriberID) == "" {
		return fmt.Errorf("%w: data.subscriber_id is required", ErrInvalidEnvelope)
	}
	return nil
}

func (d *subscriptionData) applicationIDPtr() *string {
	if strings.TrimSpace(d.ApplicationID) == "" {
		return nil
	}
	v := strings.TrimSpace(d.ApplicationID)
	return &v
}

// findSubscription locates an existing subscription by API (handle + kind) and subscriber within
// the org. kind scopes the API resolution to the artifact table backing that kind, so a handle
// shared across kinds resolves unambiguously. Returns (nil, nil) when none exists.
func (r *Receiver) findSubscription(orgID, apiRefID, kind, subscriberID string) (*model.Subscription, error) {
	return r.subs.FindByArtifactKindAndSubscriber(orgID, apiRefID, kind, subscriberID)
}

// handleSubscriptionCreated reconciles a subscription created in the Developer Portal. The existing
// service generates the subscription's token, persists it, and broadcasts to deployed gateways.
func (r *Receiver) handleSubscriptionCreated(ctx context.Context, env *Envelope) error {
	var d subscriptionData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}

	// Import the Developer Portal token (encrypted under data.token) so the value the user sees
	// validates at the gateway. When absent, the service generates its own.
	var token string
	if d.Token != nil {
		decrypted, err := r.decryptor.Decrypt(d.Token)
		if err != nil {
			return err
		}
		token = decrypted
	}

	// Developer Portal sync events have no interactive platform user, so the audit
	// actor is left empty (there is no internal-UUID identity to attribute).
	_, err := r.subs.CreateSubscription(d.API.RefID, d.API.kind(), env.OrgID, d.SubscriberID, d.applicationIDPtr(), &d.SubscriptionPlan.RefID, token, d.Status, "")
	if err != nil {
		// Domain-level idempotency: a duplicate delivery whose subscription already exists is success.
		if apperror.SubscriptionExists.Is(err) {
			return nil
		}
		return err
	}
	return nil
}

// handleSubscriptionUpdated applies a subscription status change (subscription.updated). The event
// carries the new status; the plan and token are unchanged, and no encrypted fields are present.
func (r *Receiver) handleSubscriptionUpdated(ctx context.Context, env *Envelope) error {
	var d subscriptionData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(d.Status) == "" {
		return fmt.Errorf("%w: data.status is required for %s", ErrInvalidEnvelope, env.EventType)
	}

	sub, err := r.findSubscription(env.OrgID, d.API.RefID, d.API.kind(), d.SubscriberID)
	if err != nil {
		return err
	}
	if sub == nil {
		return apperror.SubscriptionNotFound.New()
	}

	_, err = r.subs.UpdateSubscription(sub.UUID, env.OrgID, d.SubscriberID, d.Status, "")
	return err
}

// handleSubscriptionPlanChanged switches the plan of an existing subscription (subscription.plan_changed).
// data.subscription_plan.ref_id is the new plan; data.previous_plan (if present) is informational and
// ignored. The subscription's token is not rotated by this event.
func (r *Receiver) handleSubscriptionPlanChanged(ctx context.Context, env *Envelope) error {
	var d subscriptionData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}

	sub, err := r.findSubscription(env.OrgID, d.API.RefID, d.API.kind(), d.SubscriberID)
	if err != nil {
		return err
	}
	if sub == nil {
		return apperror.SubscriptionNotFound.New()
	}

	_, err = r.subs.ChangePlan(sub.UUID, env.OrgID, d.SubscriberID, d.SubscriptionPlan.RefID)
	return err
}

// handleSubscriptionTokenRegenerated rotates an existing subscription's token
// (subscription.token_regenerated). The new token arrives encrypted under data.token (listed in
// encrypted_fields as "token"); we decrypt it and persist it, invalidating the old token.
func (r *Receiver) handleSubscriptionTokenRegenerated(ctx context.Context, env *Envelope) error {
	var d subscriptionData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}
	if d.Token == nil {
		return fmt.Errorf("%w: data.token (encrypted) is required for %s", ErrInvalidEnvelope, env.EventType)
	}

	token, err := r.decryptor.Decrypt(d.Token)
	if err != nil {
		return err
	}

	sub, err := r.findSubscription(env.OrgID, d.API.RefID, d.API.kind(), d.SubscriberID)
	if err != nil {
		return err
	}
	if sub == nil {
		return apperror.SubscriptionNotFound.New()
	}

	_, err = r.subs.RegenerateToken(sub.UUID, env.OrgID, d.SubscriberID, token)
	return err
}

// handleSubscriptionDeleted removes a subscription deleted in the Developer Portal.
func (r *Receiver) handleSubscriptionDeleted(ctx context.Context, env *Envelope) error {
	var d subscriptionData
	if err := env.decodeData(&d); err != nil {
		return err
	}
	if err := d.validate(); err != nil {
		return err
	}

	sub, err := r.findSubscription(env.OrgID, d.API.RefID, d.API.kind(), d.SubscriberID)
	if err != nil {
		return err
	}
	if sub == nil {
		// Domain-level idempotency: already gone is success.
		return nil
	}

	if err := r.subs.DeleteSubscription(sub.UUID, env.OrgID, d.SubscriberID, ""); err != nil {
		if apperror.SubscriptionNotFound.Is(err) {
			return nil
		}
		return err
	}
	return nil
}
