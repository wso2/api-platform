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
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// billingReadSubRepo holds one subscription and counts the changes made to it.
type billingReadSubRepo struct {
	repository.SubscriptionRepository
	sub     *model.Subscription
	changes int
}

func (r *billingReadSubRepo) GetByID(subscriptionID, orgUUID string) (*model.Subscription, error) {
	copied := *r.sub
	return &copied, nil
}

func (r *billingReadSubRepo) Update(*model.Subscription) error {
	r.changes++
	return nil
}

func (r *billingReadSubRepo) UpdateToken(subscriptionID, orgUUID, newToken string) error {
	r.changes++
	return nil
}

type billingReadPlanRepo struct {
	repository.SubscriptionPlanRepository
}

func (billingReadPlanRepo) GetByHandleAndOrg(handle, orgUUID string) (*model.SubscriptionPlan, error) {
	return &model.SubscriptionPlan{UUID: "plan-uuid-" + handle}, nil
}


// billingReadRepo answers every read with err, or with no billing state when err is nil.
type billingReadRepo struct {
	err error
}

func (r billingReadRepo) Get(subscriptionUUID, orgUUID string) (*model.SubscriptionBilling, error) {
	return nil, r.err
}

func (r billingReadRepo) GetMany(subscriptionUUIDs []string, orgUUID string) (map[string]*model.SubscriptionBilling, error) {
	return nil, r.err
}

func (r billingReadRepo) Upsert(*model.SubscriptionBilling) error {
	return nil
}

func newBillingReadService(readErr error) (*SubscriptionService, *billingReadSubRepo) {
	subs := &billingReadSubRepo{sub: &model.Subscription{UUID: "sub-1", SubscriberID: "alice",
		ArtifactUUID: "api-1", Status: model.SubscriptionStatusActive}}
	svc := NewSubscriptionService(nil, nil, subs, billingReadPlanRepo{}, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	svc.SetBillingRepository(billingReadRepo{err: readErr})
	return svc, subs
}

// A change whose gateway event could not carry the subscription's billing state is not made:
// treated as free, an unpaid subscription would be sent to the gateways as active.
func TestSubscriptionChanges_FailWithoutChangingWhenTheBillingStateCannotBeRead(t *testing.T) {
	readErr := errors.New("database is unavailable")
	changes := map[string]func(*SubscriptionService) error{
		"status": func(s *SubscriptionService) error {
			_, err := s.UpdateSubscription(context.Background(), "sub-1", "org-1", "alice", "INACTIVE", "alice")
			return err
		},
		"plan": func(s *SubscriptionService) error {
			_, err := s.ChangePlan(context.Background(), "sub-1", "org-1", "alice", "gold")
			return err
		},
		"token": func(s *SubscriptionService) error {
			_, err := s.RegenerateToken(context.Background(), "sub-1", "org-1", "alice", "new-token")
			return err
		},
	}
	for name, change := range changes {
		svc, subs := newBillingReadService(readErr)
		if err := change(svc); !errors.Is(err, readErr) {
			t.Errorf("%s: err = %v, want the read error", name, err)
		}
		if subs.changes != 0 {
			t.Errorf("%s: %d change(s) made, want none", name, subs.changes)
		}

		svc, subs = newBillingReadService(nil)
		if err := change(svc); err != nil {
			t.Errorf("%s with the billing state read: %v", name, err)
		}
		if subs.changes != 1 {
			t.Errorf("%s with the billing state read: %d change(s) made, want 1", name, subs.changes)
		}
	}
}
