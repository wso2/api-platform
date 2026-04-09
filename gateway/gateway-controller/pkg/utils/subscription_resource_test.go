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

package utils

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type recordingSubscriptionDB struct {
	*testMockDB
	calls       *[]string
	application *models.StoredApplication
	mappings    []*models.ApplicationAPIKeyMapping
}

func newRecordingSubscriptionDB(calls *[]string) *recordingSubscriptionDB {
	return &recordingSubscriptionDB{
		testMockDB: newTestMockDB(),
		calls:      calls,
	}
}

func (m *recordingSubscriptionDB) SaveSubscription(sub *models.Subscription) error {
	*m.calls = append(*m.calls, "save_subscription")
	return nil
}

func (m *recordingSubscriptionDB) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	*m.calls = append(*m.calls, "save_subscription_plan")
	return nil
}

func (m *recordingSubscriptionDB) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error {
	*m.calls = append(*m.calls, "replace_application_mappings")
	m.application = application
	m.mappings = mappings
	return nil
}

type recordingSubscriptionUpdater struct {
	calls int
}

func (m *recordingSubscriptionUpdater) UpdateSnapshot(context.Context) error {
	m.calls++
	return nil
}

type recordingSubscriptionEventHub struct {
	calls           *[]string
	publishedEvents []eventhub.Event
	gatewayIDs      []string
}

func (m *recordingSubscriptionEventHub) Initialize() error { return nil }
func (m *recordingSubscriptionEventHub) RegisterGateway(string) error {
	return nil
}
func (m *recordingSubscriptionEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	*m.calls = append(*m.calls, "publish_event")
	m.gatewayIDs = append(m.gatewayIDs, gatewayID)
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}
func (m *recordingSubscriptionEventHub) Subscribe(string) (<-chan eventhub.Event, error) {
	return nil, nil
}
func (m *recordingSubscriptionEventHub) Unsubscribe(string, <-chan eventhub.Event) error {
	return nil
}
func (m *recordingSubscriptionEventHub) UnsubscribeAll(string) error { return nil }
func (m *recordingSubscriptionEventHub) CleanUpEvents() error        { return nil }
func (m *recordingSubscriptionEventHub) Close() error                { return nil }

func newSubscriptionResourceTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSubscriptionResourceServiceSaveSubscription_PublishesAfterDBWrite(t *testing.T) {
	calls := []string{}
	db := newRecordingSubscriptionDB(&calls)
	updater := &recordingSubscriptionUpdater{}
	hub := &recordingSubscriptionEventHub{calls: &calls}
	service := NewSubscriptionResourceService(db, updater, hub, "gateway-1")

	sub := &models.Subscription{
		ID:                "sub-1",
		APIID:             "api-1",
		SubscriptionToken: "plain-token",
		Status:            models.SubscriptionStatusActive,
	}

	err := service.SaveSubscription(sub, "corr-sub-create", newSubscriptionResourceTestLogger())
	require.NoError(t, err)

	assert.Equal(t, []string{"save_subscription", "publish_event"}, calls)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, "gateway-1", hub.gatewayIDs[0])
	assert.Equal(t, eventhub.EventTypeSubscription, hub.publishedEvents[0].EventType)
	assert.Equal(t, "CREATE", hub.publishedEvents[0].Action)
	assert.Equal(t, "sub-1", hub.publishedEvents[0].EntityID)
	assert.Equal(t, "corr-sub-create", hub.publishedEvents[0].EventID)
	assert.Equal(t, eventhub.EmptyEventData, hub.publishedEvents[0].EventData)
	assert.Zero(t, updater.calls)
}

func TestSubscriptionResourceServiceReplaceApplicationMappings_PublishesWithoutLocalRefresh(t *testing.T) {
	calls := []string{}
	db := newRecordingSubscriptionDB(&calls)
	updater := &recordingSubscriptionUpdater{}
	hub := &recordingSubscriptionEventHub{calls: &calls}
	service := NewSubscriptionResourceService(db, updater, hub, "gateway-2")

	application := &models.StoredApplication{
		ApplicationID:   "app-123",
		ApplicationUUID: "app-uuid-123",
		ApplicationName: "Shopping App",
		ApplicationType: "genai",
	}
	mappings := []*models.ApplicationAPIKeyMapping{
		{ApplicationUUID: "app-uuid-123", APIKeyID: "key-1"},
		{ApplicationUUID: "app-uuid-123", APIKeyID: "key-2"},
	}

	err := service.ReplaceApplicationAPIKeyMappings(application, mappings, "corr-app-update", newSubscriptionResourceTestLogger())
	require.NoError(t, err)

	assert.Equal(t, []string{"replace_application_mappings", "publish_event"}, calls)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeApplication, hub.publishedEvents[0].EventType)
	assert.Equal(t, "UPDATE", hub.publishedEvents[0].Action)
	assert.Equal(t, "app-uuid-123", hub.publishedEvents[0].EntityID)
	assert.Equal(t, "corr-app-update", hub.publishedEvents[0].EventID)
	assert.Zero(t, updater.calls)
	require.NotNil(t, db.application)
	assert.Equal(t, "app-uuid-123", db.application.ApplicationUUID)
	require.Len(t, db.mappings, 2)
}
