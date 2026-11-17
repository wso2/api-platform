/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

import (
	"regexp"
	"sync"
)

// Global Backends are gateway-wide backends (kind: Backend). The gateway controller turns
// each one into an Envoy cluster named BackendClusterPrefix+name and publishes a lazy
// resource of type LazyResourceTypeBackend describing it, so policies and the kernel can
// resolve a name per request without the API naming the Backend in its configuration.

const (
	// LazyResourceTypeBackend is the lazy resource type the gateway controller publishes for
	// every global Backend.
	LazyResourceTypeBackend = "Backend"

	// BackendClusterPrefix prefixes the Envoy cluster of every global Backend. It is
	// reserved: no API-owned cluster uses it.
	BackendClusterPrefix = "backend_"

	// Keys of the lazy resource map the controller publishes for a Backend.
	BackendResourceKeyName        = "name"
	BackendResourceKeyClusterName = "clusterName"
	BackendResourceKeyBasePath    = "basePath"
	BackendResourceKeyLabels      = "labels"

	backendNameMinLength = 3
	backendNameMaxLength = 40
)

var backendNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// IsValidBackendName reports whether name follows the Backend name rule: lowercase letters,
// digits and single hyphens between them, 3 to 40 characters. A value that fails it can never
// name a Backend, so callers can reject it before any lookup.
func IsValidBackendName(name string) bool {
	return len(name) >= backendNameMinLength && len(name) <= backendNameMaxLength &&
		backendNamePattern.MatchString(name)
}

// BackendInfo is what a policy or the kernel needs to know about one global Backend.
type BackendInfo struct {
	// Name is the Backend's metadata.name.
	Name string
	// ClusterName is the Envoy cluster that routes to it; always BackendClusterPrefix+Name.
	ClusterName string
	// BasePath is prefixed to the request path after the API context is stripped.
	BasePath string
	// Labels are the Backend's metadata.labels. Shared by every lookup: do not modify.
	Labels map[string]string
}

// MatchesLabels reports whether the Backend carries every key/value pair in matchLabels.
// An empty or nil matchLabels matches every Backend, as a Kubernetes label selector does.
func (e BackendInfo) MatchesLabels(matchLabels map[string]string) bool {
	for key, value := range matchLabels {
		if got, ok := e.Labels[key]; !ok || got != value {
			return false
		}
	}
	return true
}

// LookupBackend resolves a global Backend by name from the shared lazy resource store.
func LookupBackend(name string) (BackendInfo, bool) {
	return GetLazyResourceStoreInstance().LookupBackend(name)
}

// backendIndex caches parsed Backend records for one store version, so a per-request
// lookup costs a map read instead of re-parsing the published resource.
type backendIndex struct {
	mu      sync.RWMutex
	built   bool
	version uint64
	byName  map[string]BackendInfo
}

// LookupBackend resolves a global Backend by name. The parsed records are cached and rebuilt
// when the store's version changes.
func (lrs *LazyResourceStore) LookupBackend(name string) (BackendInfo, bool) {
	current := lrs.Version()
	idx := &lrs.backends

	idx.mu.RLock()
	if idx.built && idx.version == current {
		info, ok := idx.byName[name]
		idx.mu.RUnlock()
		return info, ok
	}
	idx.mu.RUnlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()
	if !idx.built || idx.version != current {
		resources, version := lrs.snapshotByType(LazyResourceTypeBackend)
		byName := make(map[string]BackendInfo, len(resources))
		for id, resource := range resources {
			if info, ok := parseBackendResource(id, resource); ok {
				byName[info.Name] = info
			}
		}
		idx.byName, idx.version, idx.built = byName, version, true
	}
	info, ok := idx.byName[name]
	return info, ok
}

// parseBackendResource turns a published Backend lazy resource into a BackendInfo. A record
// whose id is not a valid Backend name is skipped, so the cluster name built from it can never
// point anywhere other than a Backend cluster.
func parseBackendResource(id string, resource *LazyResource) (BackendInfo, bool) {
	if resource == nil || !IsValidBackendName(id) {
		return BackendInfo{}, false
	}
	info := BackendInfo{Name: id, ClusterName: BackendClusterPrefix + id}
	if basePath, ok := resource.Resource[BackendResourceKeyBasePath].(string); ok {
		info.BasePath = basePath
	}
	switch labels := resource.Resource[BackendResourceKeyLabels].(type) {
	case map[string]string:
		info.Labels = make(map[string]string, len(labels))
		for key, value := range labels {
			info.Labels[key] = value
		}
	case map[string]interface{}:
		info.Labels = make(map[string]string, len(labels))
		for key, value := range labels {
			if s, ok := value.(string); ok {
				info.Labels[key] = s
			}
		}
	}
	return info, true
}
