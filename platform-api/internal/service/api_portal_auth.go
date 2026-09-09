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

// Placeholder for the shared-key outbound authentication path.
//
// The CRUD surface for /api-portals now stores an encrypted shared key on each
// row (model.APIPortal.InternalAuthKey). The provider that decrypts that value
// and returns `Authorization: SharedKey <raw>` for every publish call is a
// self-contained subsystem tracked as follow-up work, this file keeps only the
// interface and the registry hook the service and its callers already speak
// against, so the branch stays compilable while the implementation lands
// separately.
//
// See the design doc: Projects/DevPortal Publishing/Platform-API-Devportal-Design/
// SharedKey-Auth-Design.md (§Platform-API side).

package service

import (
	"context"
	"errors"
	"sync"
)

// AuthProvider is the outbound-auth surface exposed to any component that
// needs to call a portal's admin REST endpoints.
type AuthProvider interface {
	// AuthorizationHeader returns the "SharedKey <raw>" header value for the
	// next outbound call. Concrete implementations decrypt the row's stored
	// key (OSS) or resolve it out of a secret backend (cloud) and cache the
	// plaintext in memory.
	AuthorizationHeader(ctx context.Context) (string, error)

	// InvalidateCache clears any cached plaintext so the next call re-reads
	// from source. Callers invoke this after the caller-visible row updates.
	InvalidateCache()
}

// APIPortalAuthRegistry keeps at most one AuthProvider per portal handle. The
// service's Update / Delete paths call Invalidate so the next outbound call
// picks up whatever the row now says. Concrete provider construction is the
// caller's problem, this type only manages the cache.
type APIPortalAuthRegistry struct {
	mu        sync.Mutex
	providers map[string]AuthProvider
}

// NewAPIPortalAuthRegistry constructs an empty registry.
func NewAPIPortalAuthRegistry() *APIPortalAuthRegistry {
	return &APIPortalAuthRegistry{providers: map[string]AuthProvider{}}
}

// Invalidate drops the cached provider for a portal handle. No-op when the
// handle has no cached entry (idempotent, safe to call from Delete paths).
func (r *APIPortalAuthRegistry) Invalidate(portalHandle string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, portalHandle)
}

// errSharedKeyProviderNotImplemented is returned by Get until the SharedKey
// provider implementation lands. Removed once the follow-up PR wires the real
// provider through here.
var errSharedKeyProviderNotImplemented = errors.New("shared-key AuthProvider not yet implemented; see SharedKey-Auth-Design.md")

// Get returns a cached AuthProvider for the portal handle, or an error when
// none is configured. The provider construction path is deferred to the
// follow-up SharedKey work, callers today only need Invalidate to be safe.
func (r *APIPortalAuthRegistry) Get(portalHandle string) (AuthProvider, error) {
	if r == nil {
		return nil, errSharedKeyProviderNotImplemented
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.providers[portalHandle]; ok {
		return p, nil
	}
	return nil, errSharedKeyProviderNotImplemented
}
