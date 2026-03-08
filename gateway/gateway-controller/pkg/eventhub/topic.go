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

package eventhub

import (
	"errors"
	"sync"
)

// Sentinel errors for organization operations
var (
	ErrOrganizationNotFound      = errors.New("organization not found")
	ErrOrganizationAlreadyExists = errors.New("organization already exists")
)

// organization tracks an organization and its subscribers
type organization struct {
	id           string
	subscribers  []chan Event
	knownVersion string
	lastPolled   int64
}

// organizationRegistry manages organization registrations and subscribers
type organizationRegistry struct {
	mu    sync.RWMutex
	orgs  map[string]*organization
}

// newOrganizationRegistry creates a new organization registry
func newOrganizationRegistry() *organizationRegistry {
	return &organizationRegistry{
		orgs: make(map[string]*organization),
	}
}

// register adds a new organization to the registry
func (r *organizationRegistry) register(orgID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orgs[orgID]; exists {
		return ErrOrganizationAlreadyExists
	}

	r.orgs[orgID] = &organization{
		id:          orgID,
		subscribers: make([]chan Event, 0),
	}
	return nil
}

// get returns the organization for the given ID
func (r *organizationRegistry) get(orgID string) (*organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	org, exists := r.orgs[orgID]
	if !exists {
		return nil, ErrOrganizationNotFound
	}
	return org, nil
}

// addSubscriber adds a subscriber channel for an organization
func (r *organizationRegistry) addSubscriber(orgID string, ch chan Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	org, exists := r.orgs[orgID]
	if !exists {
		return ErrOrganizationNotFound
	}

	org.subscribers = append(org.subscribers, ch)
	return nil
}

// removeSubscriber removes a subscriber channel for an organization
func (r *organizationRegistry) removeSubscriber(orgID string, ch chan Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	org, exists := r.orgs[orgID]
	if !exists {
		return ErrOrganizationNotFound
	}

	for i, sub := range org.subscribers {
		if sub == ch {
			org.subscribers = append(org.subscribers[:i], org.subscribers[i+1:]...)
			close(ch)
			return nil
		}
	}
	return nil
}

// getAll returns all registered organizations
func (r *organizationRegistry) getAll() []*organization {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orgs := make([]*organization, 0, len(r.orgs))
	for _, org := range r.orgs {
		orgs = append(orgs, org)
	}
	return orgs
}
