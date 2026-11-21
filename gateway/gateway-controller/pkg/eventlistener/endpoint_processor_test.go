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

package eventlistener

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

type fakeEndpointSyncer struct {
	calls []string
	err   error
}

func (f *fakeEndpointSyncer) Sync(_ context.Context, correlationID string) error {
	f.calls = append(f.calls, correlationID)
	return f.err
}

func TestHandleEvent_Endpoint_ReconcilesFromTheDatabase(t *testing.T) {
	for _, action := range []string{"CREATE", "UPDATE", "DELETE"} {
		t.Run(action, func(t *testing.T) {
			syncer := &fakeEndpointSyncer{}
			listener := &EventListener{logger: newTestLogger()}
			listener.SetEndpointSyncer(syncer)

			listener.handleEvent(eventhub.Event{
				EventType: eventhub.EventTypeEndpoint, Action: action, EntityID: "endpoint-1", EventID: "corr-1",
			})

			assert.Equal(t, []string{"corr-1"}, syncer.calls)
		})
	}
}

func TestHandleEvent_Endpoint_UnknownActionDoesNothing(t *testing.T) {
	syncer := &fakeEndpointSyncer{}
	listener := &EventListener{logger: newTestLogger()}
	listener.SetEndpointSyncer(syncer)

	listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeEndpoint, Action: "RENAME", EntityID: "endpoint-1"})

	assert.Empty(t, syncer.calls)
}

func TestHandleEvent_Endpoint_FailuresAndMissingSyncerDoNotPanic(t *testing.T) {
	listener := &EventListener{logger: newTestLogger()}
	event := eventhub.Event{EventType: eventhub.EventTypeEndpoint, Action: "CREATE", EntityID: "endpoint-1"}

	// A replica without Endpoint support ignores the event.
	assert.NotPanics(t, func() { listener.handleEvent(event) })

	syncer := &fakeEndpointSyncer{err: errors.New("database unavailable")}
	listener.SetEndpointSyncer(syncer)
	assert.NotPanics(t, func() { listener.handleEvent(event) })
	assert.Len(t, syncer.calls, 1)
}

// replica is one controller replica's Endpoint state: its own in-memory store
// and policy-engine resources, over the database it shares with the others.
type replica struct {
	store     *storage.ConfigStore
	resources *lazyresourcexds.LazyResourceStateManager
	listener  *EventListener
}

func newReplica(db storage.Storage) *replica {
	logger := newTestLogger()
	lazyStore := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, logger), logger)
	store := storage.NewConfigStore()
	listener := &EventListener{logger: logger}
	listener.SetEndpointSyncer(utils.NewEndpointPublisher(db, store, nil, resources))
	return &replica{store: store, resources: resources, listener: listener}
}

func (r *replica) publishedEndpoints() []string {
	var names []string
	for name := range r.resources.GetResourcesByType(utils.LazyResourceTypeEndpoint) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sharedEndpoint(name string) *models.StoredEndpoint {
	endpoint := &models.StoredEndpoint{
		UUID: "uuid-" + name,
		Configuration: api.EndpointConfiguration{
			ApiVersion: api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.EndpointConfigurationKindEndpoint,
			Metadata:   api.Metadata{Name: name},
		},
	}
	endpoint.Configuration.Spec.Upstreams = append(endpoint.Configuration.Spec.Upstreams, struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{Url: "http://orders-" + name + ":9080"})
	return endpoint
}

// An Endpoint written through one replica becomes routable on another when the
// event arrives, and is withdrawn there when it is deleted.
func TestEndpointEvents_KeepAnotherReplicaInSync(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	other := newReplica(db)

	// Replica one writes tenant-a to the shared database and publishes the event.
	endpoint := sharedEndpoint("tenant-a")
	require.NoError(t, db.SaveEndpoint(endpoint))
	other.listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeEndpoint, Action: "CREATE", EntityID: endpoint.UUID, EventID: "corr-create"})

	_, inStore := other.store.GetEndpoint("tenant-a")
	assert.True(t, inStore, "the translator on the other replica must see the Endpoint")
	assert.Equal(t, []string{"tenant-a"}, other.publishedEndpoints())

	// An update made elsewhere follows.
	basePath := "/v2"
	endpoint.Configuration.Spec.BasePath = &basePath
	require.NoError(t, db.UpdateEndpoint(endpoint))
	other.listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeEndpoint, Action: "UPDATE", EntityID: endpoint.UUID, EventID: "corr-update"})

	resource, ok := other.resources.GetResourceByIDAndType("tenant-a", utils.LazyResourceTypeEndpoint)
	require.True(t, ok)
	assert.Equal(t, "/v2", resource.Resource["basePath"])

	// And a delete withdraws it.
	require.NoError(t, db.DeleteEndpoint(endpoint.UUID))
	other.listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeEndpoint, Action: "DELETE", EntityID: endpoint.UUID, EventID: "corr-delete"})

	_, inStore = other.store.GetEndpoint("tenant-a")
	assert.False(t, inStore)
	assert.Empty(t, other.publishedEndpoints())
}

// Events can be lost or arrive out of order; each one reconciles the whole set,
// so a single later event repairs a replica that missed earlier ones.
func TestEndpointEvents_OneEventCatchesUpOnMissedOnes(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	other := newReplica(db)

	for _, name := range []string{"tenant-a", "tenant-b", "tenant-c"} {
		require.NoError(t, db.SaveEndpoint(sharedEndpoint(name)))
	}
	require.NoError(t, db.DeleteEndpoint("uuid-tenant-b"))

	other.listener.handleEvent(eventhub.Event{EventType: eventhub.EventTypeEndpoint, Action: "CREATE", EntityID: "uuid-tenant-c"})

	assert.Equal(t, []string{"tenant-a", "tenant-c"}, other.publishedEndpoints())
}
