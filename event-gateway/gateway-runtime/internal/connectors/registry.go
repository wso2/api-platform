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

package connectors

import (
	"fmt"
	"sync"
)

// EntrypointFactory creates an entrypoint from per-channel config.
// Entrypoint-specific configuration is captured in the closure at registration time.
type EntrypointFactory func(cfg EntrypointConfig) (Entrypoint, error)

// EndpointFactory creates an endpoint.
// Connection-specific configuration is captured in the closure at registration time.
type EndpointFactory func() (Endpoint, error)

// Registry holds factories for creating entrypoints and endpoints by type name.
// New types are added by registering factories — no changes to the runtime or main required.
type Registry struct {
	mu          sync.RWMutex
	entrypoints map[string]EntrypointFactory
	endpoints   map[string]EndpointFactory
}

// NewRegistry creates an empty connector registry.
func NewRegistry() *Registry {
	return &Registry{
		entrypoints: make(map[string]EntrypointFactory),
		endpoints:   make(map[string]EndpointFactory),
	}
}

// RegisterEntrypoint registers an entrypoint factory by type name (e.g. "websub", "websocket").
func (r *Registry) RegisterEntrypoint(name string, factory EntrypointFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entrypoints[name] = factory
}

// RegisterEndpoint registers an endpoint factory by type name (e.g. "kafka").
func (r *Registry) RegisterEndpoint(name string, factory EndpointFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints[name] = factory
}

// CreateEntrypoint creates an entrypoint using the registered factory.
func (r *Registry) CreateEntrypoint(name string, cfg EntrypointConfig) (Entrypoint, error) {
	r.mu.RLock()
	factory, ok := r.entrypoints[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown entrypoint type: %s", name)
	}
	return factory(cfg)
}

// CreateEndpoint creates an endpoint using the registered factory.
func (r *Registry) CreateEndpoint(name string) (Endpoint, error) {
	r.mu.RLock()
	factory, ok := r.endpoints[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown endpoint type: %s", name)
	}
	return factory()
}
