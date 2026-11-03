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
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// recordingEventHub keeps what is published, so a test can inspect the row the
// SQL backend would persist.
type recordingEventHub struct {
	noopHub
	published []eventhub.Event
}

type noopHub struct{}

func (noopHub) Initialize() error                               { return nil }
func (noopHub) RegisterGateway(string) error                    { return nil }
func (noopHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (noopHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (noopHub) UnsubscribeAll(string) error                     { return nil }
func (noopHub) CleanUpEvents() error                            { return nil }
func (noopHub) Close() error                                    { return nil }

func (h *recordingEventHub) PublishEvent(_ string, e eventhub.Event) error {
	h.published = append(h.published, e)
	return nil
}

// The persisted action is derived from the event-type suffix alone, and an
// unrecognized suffix silently becomes UPDATE — which the action column's CHECK
// constraint would still accept. So each Agent event type is pinned both by
// name and by the action it derives.
func TestAgentEventTypesDeriveTheirAction(t *testing.T) {
	for _, tc := range []struct {
		eventType  string
		wantName   string
		wantAction string
	}{
		{EventTypeAgentDeployed, "agent.deployed", "CREATE"},
		{EventTypeAgentUndeployed, "agent.undeployed", "DELETE"},
		{EventTypeAgentDeleted, "agent.deleted", "DELETE"},
	} {
		assert.Equal(t, tc.wantName, tc.eventType)
		assert.Equal(t, tc.wantAction, actionForEventType(tc.eventType), tc.eventType)
	}
}

func TestBroadcastAgentEvents(t *testing.T) {
	performedAt := time.Date(2026, 9, 24, 10, 0, 0, 123_000_000, time.UTC)
	for _, tc := range []struct {
		name        string
		publish     func(*GatewayEventsService) error
		wantType    string
		wantAction  string
		wantPayload map[string]any
	}{
		{
			name: "deployed",
			publish: func(s *GatewayEventsService) error {
				return s.BroadcastAgentDeploymentEvent("gw-1", &model.AgentDeploymentEvent{
					ProxyId: "artifact-uuid", DeploymentID: "dep-1", PerformedAt: performedAt})
			},
			wantType:   "agent.deployed",
			wantAction: "CREATE",
			wantPayload: map[string]any{"proxyId": "artifact-uuid", "deploymentId": "dep-1",
				"performedAt": "2026-09-24T10:00:00.123Z"},
		},
		{
			name: "undeployed",
			publish: func(s *GatewayEventsService) error {
				return s.BroadcastAgentUndeploymentEvent("gw-1", &model.AgentUndeploymentEvent{
					ProxyId: "artifact-uuid", DeploymentID: "dep-1", PerformedAt: performedAt})
			},
			wantType:   "agent.undeployed",
			wantAction: "DELETE",
			wantPayload: map[string]any{"proxyId": "artifact-uuid", "deploymentId": "dep-1",
				"performedAt": "2026-09-24T10:00:00.123Z"},
		},
		{
			name: "deleted",
			publish: func(s *GatewayEventsService) error {
				return s.BroadcastAgentDeletionEvent("gw-1", &model.AgentDeletionEvent{ProxyId: "artifact-uuid"})
			},
			wantType:    "agent.deleted",
			wantAction:  "DELETE",
			wantPayload: map[string]any{"proxyId": "artifact-uuid"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := &recordingEventHub{}
			svc := NewGatewayEventsService(hub, nil, slog.Default())
			require.NoError(t, tc.publish(svc))
			require.Len(t, hub.published, 1)

			ev := hub.published[0]
			assert.Equal(t, "gw-1", ev.GatewayID)
			assert.Equal(t, eventTypePlatformGateway, ev.EventType)
			assert.Equal(t, tc.wantAction, ev.Action)

			var envelope struct {
				Type    string         `json:"type"`
				Payload map[string]any `json:"payload"`
				UserID  string         `json:"userId"`
			}
			require.NoError(t, json.Unmarshal([]byte(ev.EventData), &envelope))
			assert.Equal(t, tc.wantType, envelope.Type)
			assert.Equal(t, tc.wantPayload, envelope.Payload)
			assert.Empty(t, envelope.UserID, "deployment lifecycle events carry no actor")
		})
	}
}
