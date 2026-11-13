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
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type fakeConfigUpdater struct {
	updated []models.StoredConfig
	err     error
}

func (f *fakeConfigUpdater) UpdateConfig(cfg *models.StoredConfig) error {
	if f.err != nil {
		return f.err
	}
	f.updated = append(f.updated, *cfg)
	return nil
}

type fakeEventPublisher struct {
	events []eventhub.Event
	err    error
}

func (f *fakeEventPublisher) PublishEvent(_ string, event eventhub.Event) error {
	f.events = append(f.events, event)
	return f.err
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestNewEnvRefusedUndeployer_PersistsUndeployAndNotifiesReplicas(t *testing.T) {
	cases := []struct {
		kind        string
		origin      models.Origin
		wantSync    models.CPSyncStatus
		wantEvent   eventhub.EventType
		wantPublish bool
	}{
		{models.KindRestApi, models.OriginGatewayAPI, models.CPSyncStatusPending, eventhub.EventTypeAPI, true},
		{models.KindRestApi, models.OriginControlPlane, "", eventhub.EventTypeAPI, true},
		{models.KindGraphQLApi, models.OriginGatewayAPI, models.CPSyncStatusPending, eventhub.EventTypeAPI, true},
		{models.KindLlmProvider, models.OriginControlPlane, "", eventhub.EventTypeLLMProvider, true},
		{models.KindLlmProxy, models.OriginControlPlane, "", eventhub.EventTypeLLMProxy, true},
		{models.KindMcp, models.OriginControlPlane, "", eventhub.EventTypeMCPProxy, true},
		{models.KindAgent, models.OriginControlPlane, "", eventhub.EventTypeAgent, true},
		{"UnknownKind", models.OriginGatewayAPI, models.CPSyncStatusPending, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+string(tc.origin), func(t *testing.T) {
			db := &fakeConfigUpdater{}
			hub := &fakeEventPublisher{}
			cfg := &models.StoredConfig{UUID: "id-1", Kind: tc.kind, Handle: "h", Origin: tc.origin, DesiredState: models.StateDeployed}

			require.NoError(t, NewEnvRefusedUndeployer(db, hub, "gw-1", discardLogger())(cfg))

			require.Len(t, db.updated, 1)
			assert.Equal(t, models.StateUndeployed, db.updated[0].DesiredState)
			assert.Equal(t, tc.wantSync, db.updated[0].CPSyncStatus)
			if !tc.wantPublish {
				assert.Empty(t, hub.events)
				return
			}
			require.Len(t, hub.events, 1)
			assert.Equal(t, tc.wantEvent, hub.events[0].EventType)
			assert.Equal(t, "UPDATE", hub.events[0].Action)
			assert.Equal(t, "id-1", hub.events[0].EntityID)
			assert.Equal(t, "gw-1", hub.events[0].GatewayID)
			assert.NotEmpty(t, hub.events[0].EventID)
		})
	}
}

func TestNewEnvRefusedUndeployer_PersistFailureSkipsEvent(t *testing.T) {
	db := &fakeConfigUpdater{err: errors.New("db down")}
	hub := &fakeEventPublisher{}
	cfg := &models.StoredConfig{UUID: "id-1", Kind: models.KindRestApi, Handle: "h"}

	err := NewEnvRefusedUndeployer(db, hub, "gw-1", discardLogger())(cfg)

	assert.ErrorContains(t, err, "db down")
	assert.Empty(t, hub.events)
}

func TestNewEnvRefusedUndeployer_PublishFailureIsNotFatalAndNilPublisherIsAllowed(t *testing.T) {
	cfg := &models.StoredConfig{UUID: "id-1", Kind: models.KindRestApi, Handle: "h"}
	failing := &fakeEventPublisher{err: errors.New("hub down")}
	assert.NoError(t, NewEnvRefusedUndeployer(&fakeConfigUpdater{}, failing, "gw-1", discardLogger())(cfg))

	db := &fakeConfigUpdater{}
	assert.NoError(t, NewEnvRefusedUndeployer(db, nil, "gw-1", nil)(cfg))
	assert.Len(t, db.updated, 1)
}

func TestUndeployEnvRefusedAtStartup(t *testing.T) {
	cfg := &models.StoredConfig{UUID: "id-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed}
	calls := 0
	UndeployEnvRefusedAtStartup(cfg, func(*models.StoredConfig) error { calls++; return nil }, discardLogger())
	assert.Equal(t, models.StateUndeployed, cfg.DesiredState)
	assert.Equal(t, 1, calls)

	// Already undeployed: nothing to do.
	UndeployEnvRefusedAtStartup(cfg, func(*models.StoredConfig) error { calls++; return nil }, discardLogger())
	assert.Equal(t, 1, calls)

	// A nil callback still undeploys in memory.
	other := &models.StoredConfig{UUID: "id-2", Kind: models.KindRestApi, DesiredState: models.StateDeployed}
	UndeployEnvRefusedAtStartup(other, nil, nil)
	assert.Equal(t, models.StateUndeployed, other.DesiredState)
}
