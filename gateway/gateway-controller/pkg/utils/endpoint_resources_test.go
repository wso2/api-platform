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
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// testEndpoint builds a stored Endpoint with one target.
func testEndpoint(name, url string) *models.StoredEndpoint {
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
	}{Url: url})
	return endpoint
}

// fakeEndpointRows stands in for the endpoints table.
type fakeEndpointRows struct {
	rows    map[string]*models.StoredEndpoint
	listErr error
}

func (f *fakeEndpointRows) GetAllEndpoints() ([]*models.StoredEndpoint, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]*models.StoredEndpoint, 0, len(f.rows))
	for _, row := range f.rows {
		out = append(out, row)
	}
	return out, nil
}

func (f *fakeEndpointRows) put(endpoints ...*models.StoredEndpoint) {
	if f.rows == nil {
		f.rows = map[string]*models.StoredEndpoint{}
	}
	for _, endpoint := range endpoints {
		f.rows[endpoint.GetHandle()] = endpoint
	}
}

// recordingSnapshots records, at each router snapshot update, which Endpoints
// the translator would build clusters for and which the policy engine holds.
type recordingSnapshots struct {
	store     *storage.ConfigStore
	resources *lazyresourcexds.LazyResourceStateManager
	err       error

	calls []snapshotCall
}

type snapshotCall struct {
	clusters  []string // Endpoints in the in-memory store
	published []string // Endpoints the policy engine holds
}

func (r *recordingSnapshots) UpdateSnapshot(context.Context, string) error {
	call := snapshotCall{}
	for _, endpoint := range r.store.GetAllEndpoints() {
		call.clusters = append(call.clusters, endpoint.GetHandle())
	}
	call.published = publishedEndpoints(r.resources)
	r.calls = append(r.calls, call)
	return r.err
}

func publishedEndpoints(resources *lazyresourcexds.LazyResourceStateManager) []string {
	var names []string
	for name := range resources.GetResourcesByType(LazyResourceTypeEndpoint) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type endpointPublisherFixture struct {
	publisher *EndpointPublisher
	rows      *fakeEndpointRows
	store     *storage.ConfigStore
	resources *lazyresourcexds.LazyResourceStateManager
	snapshots *recordingSnapshots
}

func newEndpointPublisherFixture(t *testing.T) *endpointPublisherFixture {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	lazyStore := storage.NewLazyResourceStore(logger)
	resources := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, logger), logger)
	store := storage.NewConfigStore()
	rows := &fakeEndpointRows{}
	snapshots := &recordingSnapshots{store: store, resources: resources}
	return &endpointPublisherFixture{
		publisher: NewEndpointPublisher(rows, store, snapshots, resources),
		rows:      rows,
		store:     store,
		resources: resources,
		snapshots: snapshots,
	}
}

func TestEndpointPublisher_CreatePublishesAfterTheCluster(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))

	require.NoError(t, f.publisher.Sync(context.Background(), "corr-1"))

	require.Len(t, f.snapshots.calls, 1)
	// When the cluster snapshot was built the Endpoint was in the store, and
	// the policy engine had not heard of it yet.
	assert.Equal(t, []string{"tenant-a"}, f.snapshots.calls[0].clusters)
	assert.Empty(t, f.snapshots.calls[0].published,
		"the policy engine must not learn of an Endpoint before its cluster exists")
	assert.Equal(t, []string{"tenant-a"}, publishedEndpoints(f.resources))
}

func TestEndpointPublisher_DeleteUnpublishesBeforeTheCluster(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"), testEndpoint("tenant-b", "http://orders-b:9080"))
	require.NoError(t, f.publisher.Sync(context.Background(), ""))
	f.snapshots.calls = nil

	delete(f.rows.rows, "tenant-a")
	require.NoError(t, f.publisher.Sync(context.Background(), "corr-2"))

	require.Len(t, f.snapshots.calls, 1)
	assert.Equal(t, []string{"tenant-b"}, f.snapshots.calls[0].clusters)
	assert.Equal(t, []string{"tenant-b"}, f.snapshots.calls[0].published,
		"the policy engine must stop selecting an Endpoint before its cluster is removed")
	assert.Equal(t, []string{"tenant-b"}, publishedEndpoints(f.resources))
	_, ok := f.store.GetEndpoint("tenant-a")
	assert.False(t, ok)
}

func TestEndpointPublisher_LazyResourceCarriesOnlyWhatRoutingNeeds(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	endpoint := testEndpoint("tenant-a", "https://orders-a:9443")
	basePath := "/v1"
	labels := map[string]string{"team": "orders"}
	endpoint.Configuration.Spec.BasePath = &basePath
	endpoint.Configuration.Metadata.Labels = &labels
	endpoint.Configuration.Spec.Tls = &map[string]interface{}{"identity": "partner-identity"}
	f.rows.put(endpoint)

	require.NoError(t, f.publisher.Sync(context.Background(), ""))

	resource, ok := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
	require.True(t, ok)
	assert.Equal(t, map[string]interface{}{
		"name":        "tenant-a",
		"clusterName": "endpoint_tenant-a",
		"basePath":    "/v1",
		"labels":      map[string]interface{}{"team": "orders"},
	}, resource.Resource, "no upstream URL, TLS setting or timeout is sent to the policy engine")
}

// A backend change replaces the cluster and leaves the policy engine's copy
// alone; a base path or label change does the reverse.
func TestEndpointPublisher_ChangesTouchOnlyWhatTheyAffect(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))
	require.NoError(t, f.publisher.Sync(context.Background(), ""))
	lazyBefore, _ := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
	f.snapshots.calls = nil

	t.Run("an upstream URL change updates only the cluster", func(t *testing.T) {
		f.rows.put(testEndpoint("tenant-a", "http://orders-moved:9090"))
		require.NoError(t, f.publisher.Sync(context.Background(), ""))

		assert.Len(t, f.snapshots.calls, 1)
		lazyAfter, _ := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
		assert.Same(t, lazyBefore, lazyAfter, "the lazy resource must not be republished")
		stored, _ := f.store.GetEndpoint("tenant-a")
		assert.Equal(t, "http://orders-moved:9090", stored.Configuration.Spec.Upstreams[0].Url)
	})

	t.Run("a label change updates only the lazy resource", func(t *testing.T) {
		f.snapshots.calls = nil
		relabelled := testEndpoint("tenant-a", "http://orders-moved:9090")
		labels := map[string]string{"team": "billing"}
		relabelled.Configuration.Metadata.Labels = &labels
		f.rows.put(relabelled)
		require.NoError(t, f.publisher.Sync(context.Background(), ""))

		assert.Empty(t, f.snapshots.calls, "a label change needs no cluster update")
		resource, _ := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
		assert.Equal(t, map[string]interface{}{"team": "billing"}, resource.Resource["labels"])
	})

	t.Run("a base path change reaches both", func(t *testing.T) {
		f.snapshots.calls = nil
		moved := testEndpoint("tenant-a", "http://orders-moved:9090")
		basePath := "/v2"
		moved.Configuration.Spec.BasePath = &basePath
		f.rows.put(moved)
		require.NoError(t, f.publisher.Sync(context.Background(), ""))

		resource, _ := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
		assert.Equal(t, "/v2", resource.Resource["basePath"])
	})
}

// The replica that made a change has already applied it when its own event
// comes back; the second Sync must find nothing to do.
func TestEndpointPublisher_SyncIsIdempotent(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))
	require.NoError(t, f.publisher.Sync(context.Background(), ""))
	lazyBefore, _ := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
	f.snapshots.calls = nil

	require.NoError(t, f.publisher.Sync(context.Background(), ""))

	assert.Empty(t, f.snapshots.calls)
	lazyAfter, _ := f.resources.GetResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint)
	assert.Same(t, lazyBefore, lazyAfter)
}

// Deleted and recreated under the same name between two Syncs: the cluster
// must be rebuilt for the new Endpoint.
func TestEndpointPublisher_RecreatedEndpointIsReplaced(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))
	require.NoError(t, f.publisher.Sync(context.Background(), ""))
	f.snapshots.calls = nil

	recreated := testEndpoint("tenant-a", "http://orders-a:9080")
	recreated.UUID = "uuid-recreated"
	f.rows.put(recreated)
	require.NoError(t, f.publisher.Sync(context.Background(), ""))

	assert.Len(t, f.snapshots.calls, 1)
	stored, ok := f.store.GetEndpoint("tenant-a")
	require.True(t, ok)
	assert.Equal(t, "uuid-recreated", stored.UUID)
	assert.Equal(t, []string{"tenant-a"}, publishedEndpoints(f.resources))
}

// If the cluster snapshot fails, the policy engine is not told about the new
// Endpoint, and the next Sync retries the snapshot even though the store
// already matches the database.
func TestEndpointPublisher_FailedSnapshotIsRetried(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))
	f.snapshots.err = errors.New("snapshot failed")

	require.Error(t, f.publisher.Sync(context.Background(), ""))
	assert.Empty(t, publishedEndpoints(f.resources),
		"an Endpoint whose cluster may not exist must not be published")

	f.snapshots.err = nil
	f.snapshots.calls = nil
	require.NoError(t, f.publisher.Sync(context.Background(), ""))
	assert.Len(t, f.snapshots.calls, 1, "the snapshot must be retried")
	assert.Equal(t, []string{"tenant-a"}, publishedEndpoints(f.resources))
}

func TestEndpointPublisher_DatabaseErrorChangesNothing(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	f.rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))
	require.NoError(t, f.publisher.Sync(context.Background(), ""))
	f.snapshots.calls = nil

	f.rows.listErr = errors.New("database unavailable")
	require.Error(t, f.publisher.Sync(context.Background(), ""))

	assert.Empty(t, f.snapshots.calls)
	assert.Equal(t, []string{"tenant-a"}, publishedEndpoints(f.resources),
		"a failed read must not be mistaken for an empty set")
	_, ok := f.store.GetEndpoint("tenant-a")
	assert.True(t, ok)
}

// At startup the store is loaded and the first snapshot built before Publish.
func TestEndpointPublisher_PublishMatchesTheStore(t *testing.T) {
	f := newEndpointPublisherFixture(t)
	require.NoError(t, f.store.SetEndpoint(testEndpoint("tenant-a", "http://orders-a:9080")))
	require.NoError(t, f.store.SetEndpoint(testEndpoint("tenant-b", "http://orders-b:9080")))

	require.NoError(t, f.publisher.Publish(""))
	assert.Equal(t, []string{"tenant-a", "tenant-b"}, publishedEndpoints(f.resources))
	assert.Empty(t, f.snapshots.calls, "Publish never rebuilds clusters")

	f.store.DeleteEndpoint("tenant-a")
	require.NoError(t, f.publisher.Publish(""))
	assert.Equal(t, []string{"tenant-b"}, publishedEndpoints(f.resources))
}

func TestEndpointPublisher_WorksWithoutARouterOrPolicyEngine(t *testing.T) {
	rows := &fakeEndpointRows{}
	rows.put(testEndpoint("tenant-a", "http://orders-a:9080"))
	store := storage.NewConfigStore()
	publisher := NewEndpointPublisher(rows, store, nil, nil)

	require.NoError(t, publisher.Sync(context.Background(), ""))
	require.NoError(t, publisher.Publish(""))
	_, ok := store.GetEndpoint("tenant-a")
	assert.True(t, ok)
}
