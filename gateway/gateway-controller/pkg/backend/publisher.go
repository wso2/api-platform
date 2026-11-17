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

// Package backend implements gateway-wide Backends: the resource lifecycle, its replica sync,
// and the record each Backend publishes to the policy engine.
package backend

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// LazyResourceType is the lazy resource type a Backend is published as. Must match the SDK's
// policy.LazyResourceTypeBackend, which policies and the kernel read.
const LazyResourceType = "Backend"

// LazyResources is the subset of the lazy-resource xDS manager the publisher uses.
type LazyResources interface {
	GetResourcesByType(resourceType string) map[string]*storage.LazyResource
	StoreResource(resource *storage.LazyResource, correlationID string) error
	RemoveResourceByIDAndType(id, resourceType, correlationID string) error
	// RefreshSnapshot pushes the whole lazy-resource store to the policy engine again.
	RefreshSnapshot() error
}

// Lister lists the gateway's Backends.
type Lister interface {
	ListBackends() []*models.StoredBackend
}

// Publisher keeps one lazy resource per Backend in the policy engine: {name, clusterName,
// basePath, labels}. It reconciles the whole set on every call, so it is safe to run at startup,
// after any change, and on every replica.
type Publisher struct {
	backends  Lister
	resources LazyResources

	// mu serialises Publish so the last one to run leaves the latest set published.
	mu sync.Mutex
	// stale is set when a push to the policy engine failed after the lazy-resource store had
	// already changed: the store then reads as published while the policy engine lacks the
	// change. The next Publish pushes the whole store again before reconciling.
	stale bool
}

// NewPublisher creates a publisher reading Backends from backends and publishing through
// resources.
func NewPublisher(backends Lister, resources LazyResources) *Publisher {
	return &Publisher{backends: backends, resources: resources}
}

// Publish makes the published records match the Backends. It stores before it removes, so no
// intermediate snapshot lacks a record for a Backend that still exists.
func (p *Publisher) Publish(correlationID string) error {
	if p == nil || p.resources == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stale {
		if err := p.resources.RefreshSnapshot(); err != nil {
			return fmt.Errorf("re-pushing backend records: %w", err)
		}
		p.stale = false
	}

	published := p.resources.GetResourcesByType(LazyResourceType)
	current := map[string]struct{}{}
	for _, backend := range p.backends.ListBackends() {
		resource := Resource(backend)
		current[resource.ID] = struct{}{}
		if existing, ok := published[resource.ID]; ok && reflect.DeepEqual(existing.Resource, resource.Resource) {
			continue
		}
		if err := p.resources.StoreResource(resource, correlationID); err != nil {
			p.stale = true
			return fmt.Errorf("publishing backend %q: %w", resource.ID, err)
		}
	}
	for id := range published {
		if _, ok := current[id]; ok {
			continue
		}
		if err := p.resources.RemoveResourceByIDAndType(id, LazyResourceType, correlationID); err != nil {
			p.stale = true
			return fmt.Errorf("removing backend %q: %w", id, err)
		}
	}
	return nil
}

// Stale reports whether a failed push left the policy engine behind the published records.
func (p *Publisher) Stale() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stale
}

// Resource builds the lazy resource published for one Backend. The keys match the SDK's
// policy.BackendResourceKey* constants.
func Resource(backend *models.StoredBackend) *storage.LazyResource {
	labels := map[string]interface{}{}
	for key, value := range backend.Labels() {
		labels[key] = value
	}
	return &storage.LazyResource{
		ID:           backend.GetHandle(),
		ResourceType: LazyResourceType,
		Resource: map[string]interface{}{
			"name":        backend.GetHandle(),
			"clusterName": backend.ClusterName(),
			"basePath":    backend.BasePath(),
			"labels":      labels,
		},
	}
}
