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
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

type billingSubsFake struct {
	subscriptionService
	created []*model.SubscriptionBilling
	applied []*model.SubscriptionBilling
	keys    []string
}

func (f *billingSubsFake) CreateSubscriptionWithBilling(_ context.Context, apiID, kind, orgUUID, subscriberID string, _, _ *string, _, _, _ string, billing *model.SubscriptionBilling) (*model.Subscription, error) {
	f.created = append(f.created, billing)
	f.keys = append(f.keys, orgUUID+"/"+apiID+"/"+subscriberID)
	return &model.Subscription{UUID: "sub-1"}, nil
}

func (f *billingSubsFake) ApplyBillingFromWebhook(_ context.Context, orgUUID, apiHandle, _, subscriberID string, billing *model.SubscriptionBilling) error {
	f.applied = append(f.applied, billing)
	f.keys = append(f.keys, orgUUID+"/"+apiHandle+"/"+subscriberID)
	return nil
}

func newBillingTestReceiver(t *testing.T) (*Receiver, *billingSubsFake, config.Webhook) {
	t.Helper()
	cfg := config.Webhook{Enabled: true, Secret: "billing-test-secret", SignatureTolerance: 5 * time.Minute,
		MaxBodySize: 1 << 20, SignatureHeader: "X-Api-Portal-Signature"}
	subs := &billingSubsFake{}
	r, err := NewReceiver(cfg, nil, subs, nil, nil, nil, planTestOrgs{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	return r, subs, cfg
}

const billingSubData = `"subscriber_id":"alice","subscription_plan":{"ref_id":"gold"},"api":{"ref_id":"weather","type":"RestApi"}`

func TestSubscriptionCreated_PaidSubscriptionCarriesItsBillingState(t *testing.T) {
	r, subs, cfg := newBillingTestReceiver(t)

	_, err := postMonetizationEvent(t, r, cfg, EventSubscriptionCreated, "2026-10-07T12:00:00Z",
		`{`+billingSubData+`,"status":"ACTIVE","billing":{"status":"active","customer_id":"cus_1","subscription_id":"sub_1"}}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	b := subs.created[0]
	if b == nil || b.Status != model.BillingStatusActive || b.CustomerID != "cus_1" || b.SubscriptionID != "sub_1" {
		t.Fatalf("billing = %+v", b)
	}
	if !b.UpdatedAt.Equal(time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("updatedAt = %v, want the event's occurred_at", b.UpdatedAt)
	}
}

func TestSubscriptionCreated_FreeSubscriptionHasNoBilling(t *testing.T) {
	r, subs, cfg := newBillingTestReceiver(t)
	if _, err := postMonetizationEvent(t, r, cfg, EventSubscriptionCreated, "2026-10-07T12:00:00Z", `{`+billingSubData+`}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subs.created) != 1 || subs.created[0] != nil {
		t.Fatalf("created = %+v, want one call with no billing", subs.created)
	}
}

func TestSubscriptionBillingUpdated_AppliesTheNewStatus(t *testing.T) {
	r, subs, cfg := newBillingTestReceiver(t)
	if _, err := postMonetizationEvent(t, r, cfg, EventSubscriptionBillingUpdated, "2026-10-07T13:00:00Z",
		`{`+billingSubData+`,"billing":{"status":"UNPAID","customer_id":"cus_1","subscription_id":"sub_1"}}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subs.applied) != 1 || subs.applied[0].Status != model.BillingStatusUnpaid || subs.keys[0] != planTestOrgUUID+"/weather/alice" {
		t.Fatalf("applied = %+v, keys = %v", subs.applied, subs.keys)
	}
}

func TestSubscriptionBillingEvents_RejectAnUnknownStatusOrMissingBilling(t *testing.T) {
	r, subs, cfg := newBillingTestReceiver(t)
	for _, tc := range []struct{ eventType, data string }{
		{EventSubscriptionBillingUpdated, `{` + billingSubData + `}`},
		{EventSubscriptionBillingUpdated, `{` + billingSubData + `,"billing":{"status":"OVERDUE","customer_id":"c","subscription_id":"s"}}`},
		{EventSubscriptionCreated, `{` + billingSubData + `,"billing":{"status":"ACTIVE","subscription_id":"s"}}`},
	} {
		_, err := postMonetizationEvent(t, r, cfg, tc.eventType, "2026-10-07T12:00:00Z", tc.data)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed {
			t.Errorf("%s %s: want a validation error, got %v", tc.eventType, tc.data, err)
		}
	}
	if len(subs.created)+len(subs.applied) != 0 {
		t.Errorf("invalid events must not reach the service")
	}
}
