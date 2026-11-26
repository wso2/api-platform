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
	"fmt"
	"reflect"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// LazyResourceTypeEndpoint is the lazy resource type of a gateway-wide
// Endpoint. It must match the policy engine SDK's
// policy.LazyResourceTypeEndpoint.
const LazyResourceTypeEndpoint = "Endpoint"

// EndpointStorage is the read-only subset of storage.Storage the publisher
// reconciles from.
type EndpointStorage interface {
	GetAllEndpoints() ([]*models.StoredEndpoint, error)
}

// EndpointSnapshotUpdater rebuilds the router's xDS snapshot, which carries one
// cluster per Endpoint.
type EndpointSnapshotUpdater interface {
	UpdateSnapshot(ctx context.Context, correlationID string) error
}

// EndpointPublisher brings this replica in line with the gateway-wide Endpoints
// in the database: the in-memory store the translator builds clusters from,
// the router's clusters (CDS), and the lazy resources of type "Endpoint" the
// policy engine routes by.
//
// The clusters and the lazy resources travel on separate xDS streams, so the
// order matters. The policy engine must never name a cluster the router does
// not have:
//
//   - an Endpoint that is gone has its lazy resource removed BEFORE its
//     cluster, so the policy engine stops selecting it first;
//   - a new or changed Endpoint has its lazy resource published AFTER the
//     cluster snapshot is set.
//
// What remains is the time the router takes to apply the snapshot and warm a
// STRICT_DNS cluster; a request in that window gets a 503 from the router.
type EndpointPublisher struct {
	db        EndpointStorage
	store     *storage.ConfigStore
	snapshots EndpointSnapshotUpdater
	resources *lazyresourcexds.LazyResourceStateManager

	// mu serialises Sync and Publish, so the last one to run always leaves the
	// latest committed set applied, in order.
	mu sync.Mutex
	// clustersStale is set when the in-memory store moved ahead of the router
	// snapshot because a snapshot update failed. The next Sync then rebuilds
	// the snapshot even though the store already matches the database.
	clustersStale bool
}

// NewEndpointPublisher creates a publisher. snapshots and resources may be nil
// (a controller running without a router or a policy engine); the steps that
// need them are then skipped.
func NewEndpointPublisher(db EndpointStorage, store *storage.ConfigStore, snapshots EndpointSnapshotUpdater,
	resources *lazyresourcexds.LazyResourceStateManager) *EndpointPublisher {
	return &EndpointPublisher{db: db, store: store, snapshots: snapshots, resources: resources}
}

// Sync makes this replica match the Endpoints in the database. It is a full
// reconcile, so it applies a create, an update or a delete alike, applies a
// change made on another replica, and is a no-op when nothing changed —
// which is what a replica sees when its own change comes back as an event.
func (p *EndpointPublisher) Sync(ctx context.Context, correlationID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	current, err := p.db.GetAllEndpoints()
	if err != nil {
		return fmt.Errorf("listing endpoints: %w", err)
	}
	wanted := make(map[string]*models.StoredEndpoint, len(current))
	for _, endpoint := range current {
		wanted[endpoint.GetHandle()] = endpoint
	}

	// Work out what the router's clusters must change for.
	clustersChanged := false
	var removed []string
	for _, existing := range p.store.GetAllEndpoints() {
		name := existing.GetHandle()
		next, ok := wanted[name]
		switch {
		case !ok || next.UUID != existing.UUID:
			// Gone, or deleted and recreated under the same name.
			removed = append(removed, name)
			clustersChanged = true
		case !reflect.DeepEqual(existing.Configuration.Spec, next.Configuration.Spec):
			clustersChanged = true
		}
	}
	for name := range wanted {
		if _, ok := p.store.GetEndpoint(name); !ok {
			clustersChanged = true
		}
	}

	// 1. Stop the policy engine selecting an Endpoint before its cluster goes.
	if p.resources != nil {
		for _, name := range removed {
			if _, published := p.resources.GetResourceByIDAndType(name, LazyResourceTypeEndpoint); !published {
				continue
			}
			if err := p.resources.RemoveResourceByIDAndType(name, LazyResourceTypeEndpoint, correlationID); err != nil {
				return fmt.Errorf("removing endpoint %q from the policy engine: %w", name, err)
			}
		}
	}

	// 2. Bring the in-memory store in line; the translator reads it.
	for _, name := range removed {
		p.store.DeleteEndpoint(name)
	}
	for _, endpoint := range current {
		if err := p.store.SetEndpoint(endpoint); err != nil {
			return fmt.Errorf("storing endpoint %q: %w", endpoint.GetHandle(), err)
		}
	}

	// 3. Clusters. A label-only change needs none.
	if (clustersChanged || p.clustersStale) && p.snapshots != nil {
		if err := p.snapshots.UpdateSnapshot(ctx, correlationID); err != nil {
			// The policy engine is not told about an Endpoint whose cluster
			// the router may not have; the next Sync retries.
			p.clustersStale = true
			return fmt.Errorf("updating the router snapshot for endpoints: %w", err)
		}
		p.clustersStale = false
	}

	// 4. Tell the policy engine, now that the clusters exist.
	return p.publishLocked(correlationID)
}

// Publish makes the published lazy resources match the Endpoints in the
// in-memory store. Call it at startup, after the first router snapshot.
func (p *EndpointPublisher) Publish(correlationID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.publishLocked(correlationID)
}

// publishLocked stores before it removes, so no intermediate lazy-resource
// snapshot lacks an Endpoint that still exists. p.mu must be held.
func (p *EndpointPublisher) publishLocked(correlationID string) error {
	if p.resources == nil {
		return nil
	}

	published := p.resources.GetResourcesByType(LazyResourceTypeEndpoint)
	current := make(map[string]struct{})
	for _, endpoint := range p.store.GetAllEndpoints() {
		name := endpoint.GetHandle()
		current[name] = struct{}{}
		resource := endpointLazyResource(endpoint)
		if existing, ok := published[name]; ok && reflect.DeepEqual(existing.Resource, resource.Resource) {
			continue
		}
		if err := p.resources.StoreResource(resource, correlationID); err != nil {
			return fmt.Errorf("publishing endpoint %q: %w", name, err)
		}
	}

	for name := range published {
		if _, ok := current[name]; ok {
			continue
		}
		if err := p.resources.RemoveResourceByIDAndType(name, LazyResourceTypeEndpoint, correlationID); err != nil {
			return fmt.Errorf("removing endpoint %q: %w", name, err)
		}
	}
	return nil
}

// endpointLazyResource builds the resource the policy engine holds for one
// Endpoint: only what it needs to route — the cluster to select, the base path
// to prefix and the labels a selector matches. The upstream URLs, TLS settings
// and timeouts stay with the router.
func endpointLazyResource(endpoint *models.StoredEndpoint) *storage.LazyResource {
	labels := make(map[string]interface{})
	for key, value := range endpoint.Labels() {
		labels[key] = value
	}
	return &storage.LazyResource{
		ID:           endpoint.GetHandle(),
		ResourceType: LazyResourceTypeEndpoint,
		Resource: map[string]interface{}{
			"name":        endpoint.GetHandle(),
			"clusterName": endpoint.ClusterName(),
			"basePath":    endpoint.BasePath(),
			"labels":      labels,
		},
	}
}
