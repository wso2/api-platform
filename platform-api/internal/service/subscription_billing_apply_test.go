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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// billingApplyArtifacts knows one API, "weather"; any other handle is not found, and "broken"
// fails to be read.
type billingApplyArtifacts struct {
	repository.ArtifactRepository
}

func (billingApplyArtifacts) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	switch handle {
	case "weather":
		return &model.APIMetadata{ID: "api-1", Handle: handle, Kind: kind}, nil
	case "broken":
		return nil, errors.New("database is unavailable")
	}
	return nil, nil
}

// billingApplySubscriptions holds alice's subscription to the API.
type billingApplySubscriptions struct {
	repository.SubscriptionRepository
}

func (billingApplySubscriptions) ListByFilters(orgUUID string, apiUUID, subscriberID, applicationID, status *string,
	limit, offset int) ([]*model.Subscription, error) {
	if subscriberID == nil || *subscriberID != "alice" {
		return nil, nil
	}
	plan, app := "gold", "app-1"
	return []*model.Subscription{{UUID: "sub-1", ArtifactUUID: *apiUUID, SubscriberID: "alice", SubscriptionToken: "token-1",
		SubscriptionPlanID: &plan, ApplicationID: &app, Status: model.SubscriptionStatusActive}}, nil
}

// billingApplyStore keeps one billing state; readErr and writeErr fail its reads and writes.
type billingApplyStore struct {
	stored   *model.SubscriptionBilling
	readErr  error
	writeErr error
	writes   int
}

func (s *billingApplyStore) Get(subscriptionUUID, orgUUID string) (*model.SubscriptionBilling, error) {
	return s.stored, s.readErr
}

func (s *billingApplyStore) GetMany(subscriptionUUIDs []string, orgUUID string) (map[string]*model.SubscriptionBilling, error) {
	return nil, s.readErr
}

func (s *billingApplyStore) Upsert(billing *model.SubscriptionBilling) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.writes++
	copied := *billing
	s.stored = &copied
	return nil
}

// billingApplyGateways lists the gateways the API is deployed to, or fails to.
type billingApplyGateways struct {
	repository.APIRepository
	err error
}

func (g billingApplyGateways) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	if g.err != nil {
		return nil, g.err
	}
	return []*model.APIGatewayWithDetails{{ID: "gw-1"}, nil, {ID: ""}, {ID: "gw-2"}}, nil
}

func newBillingApplyService(store *billingApplyStore, gateways billingApplyGateways) (*SubscriptionService, *capturingEventHub) {
	hub := &capturingEventHub{}
	svc := NewSubscriptionService(gateways, billingApplyArtifacts{}, billingApplySubscriptions{}, nil, nil,
		NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger()), nil, newTestLogger())
	if store != nil {
		svc.SetBillingRepository(store)
	}
	return svc, hub
}

func unpaid(at time.Time) *model.SubscriptionBilling {
	return &model.SubscriptionBilling{Status: model.BillingStatusUnpaid, CustomerID: "cus_1", SubscriptionID: "sub_stripe_1",
		UpdatedAt: at}
}

// A billing report is refused when billing is not kept, the state is not one, or the subscription
// can't be found.
func TestApplyBillingFromWebhook_RefusesWhatItCannotApply(t *testing.T) {
	now := time.Now()
	withoutBilling, _ := newBillingApplyService(nil, billingApplyGateways{})
	if err := withoutBilling.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", unpaid(now)); !apperror.ValidationFailed.Is(err) {
		t.Errorf("without billing: err = %v, want a validation failure", err)
	}
	svc, _ := newBillingApplyService(&billingApplyStore{}, billingApplyGateways{})
	for name, billing := range map[string]*model.SubscriptionBilling{
		"no state":       nil,
		"unknown status": {Status: "OVERDUE", UpdatedAt: now},
	} {
		if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", billing); !apperror.ValidationFailed.Is(err) {
			t.Errorf("%s: err = %v, want a validation failure", name, err)
		}
	}
	if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "missing", "RestApi", "alice", unpaid(now)); !apperror.ArtifactNotFound.Is(err) {
		t.Errorf("unknown API: err = %v, want artifact not found", err)
	}
	if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "broken", "RestApi", "alice", unpaid(now)); err == nil {
		t.Error("an API that can't be read: want an error")
	}
	if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "bob", unpaid(now)); !apperror.SubscriptionNotFound.Is(err) {
		t.Errorf("no subscription: err = %v, want subscription not found", err)
	}
}

// A billing state is stored and sent to every gateway the API is deployed to; one older than the
// stored state changes nothing.
func TestApplyBillingFromWebhook_StoresTheStateAndTellsTheGateways(t *testing.T) {
	now := time.Now()
	store := &billingApplyStore{}
	svc, hub := newBillingApplyService(store, billingApplyGateways{})
	if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", unpaid(now)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if store.writes != 1 || store.stored.SubscriptionUUID != "sub-1" || store.stored.OrganizationUUID != "org-1" {
		t.Fatalf("stored %+v after %d write(s)", store.stored, store.writes)
	}
	if len(hub.published) != 2 {
		t.Fatalf("published %d event(s), want one for each of the 2 gateways", len(hub.published))
	}
	var envelope struct {
		Payload model.SubscriptionUpdatedEvent `json:"payload"`
	}
	if err := json.Unmarshal([]byte(hub.published[0].EventData), &envelope); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	event := envelope.Payload
	if event.SubscriptionId != "sub-1" || event.SubscriptionPlanId != "gold" || event.ApplicationId != "app-1" ||
		event.BillingCustomerId != "cus_1" || event.Status != "INACTIVE" {
		t.Errorf("event = %+v, want sub-1 on gold for app-1, billed to cus_1 and stopped", event)
	}

	if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", unpaid(now.Add(-time.Hour))); err != nil {
		t.Fatalf("a stale state: %v", err)
	}
	if store.writes != 1 || len(hub.published) != 2 {
		t.Errorf("a stale state changed something: %d write(s), %d event(s)", store.writes, len(hub.published))
	}
}

// The state is kept even when the gateways can't be listed; a failed read or write of the state
// fails the report.
func TestApplyBillingFromWebhook_Failures(t *testing.T) {
	now := time.Now()
	store := &billingApplyStore{}
	svc, hub := newBillingApplyService(store, billingApplyGateways{err: errors.New("gateways unavailable")})
	if err := svc.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", unpaid(now)); err != nil {
		t.Fatalf("apply with the gateways unavailable: %v", err)
	}
	if store.writes != 1 || len(hub.published) != 0 {
		t.Errorf("got %d write(s) and %d event(s), want the state stored and no event", store.writes, len(hub.published))
	}

	readErr := errors.New("billing read failed")
	failing, _ := newBillingApplyService(&billingApplyStore{readErr: readErr}, billingApplyGateways{})
	if err := failing.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", unpaid(now)); !errors.Is(err, readErr) {
		t.Errorf("read failure: err = %v, want it", err)
	}
	writeErr := errors.New("billing write failed")
	failing, _ = newBillingApplyService(&billingApplyStore{writeErr: writeErr}, billingApplyGateways{})
	if err := failing.ApplyBillingFromWebhook(context.Background(), "org-1", "weather", "RestApi", "alice", unpaid(now)); !errors.Is(err, writeErr) {
		t.Errorf("write failure: err = %v, want it", err)
	}
}
