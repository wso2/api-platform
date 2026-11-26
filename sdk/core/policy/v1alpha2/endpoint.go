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

// LazyResourceTypeEndpoint is the lazy resource type the gateway controller
// publishes one of per gateway-wide Endpoint.
const LazyResourceTypeEndpoint = "Endpoint"

// EndpointClusterPrefix prefixes the router cluster of every gateway-wide
// Endpoint. The prefix is reserved: no API cluster name starts with it.
const EndpointClusterPrefix = "endpoint_"

// Endpoint names follow the platform handle rule, so a name is safe to use in a
// cluster name unchanged and a request-supplied value can be rejected before
// any lookup.
const (
	EndpointNameMinLength = 3
	EndpointNameMaxLength = 40
)

var endpointNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// IsValidEndpointName reports whether name is a well-formed Endpoint name:
// lowercase alphanumeric segments joined by single hyphens, 3 to 40 characters.
func IsValidEndpointName(name string) bool {
	if len(name) < EndpointNameMinLength || len(name) > EndpointNameMaxLength {
		return false
	}
	return endpointNamePattern.MatchString(name)
}

// EndpointInfo describes one gateway-wide Endpoint as the policy engine knows it.
type EndpointInfo struct {
	// Name is the Endpoint's metadata.name.
	Name string
	// Labels is the Endpoint's metadata.labels. Never nil.
	Labels map[string]string
	// ClusterName is the router cluster that carries the Endpoint's traffic.
	ClusterName string
	// BasePath is prefixed to the request path sent to the Endpoint's upstreams.
	BasePath string
}

// MatchesLabels reports whether the Endpoint carries every label in selector
// with the same value. An empty selector matches every Endpoint.
func (e EndpointInfo) MatchesLabels(selector map[string]string) bool {
	for key, want := range selector {
		if got, ok := e.Labels[key]; !ok || got != want {
			return false
		}
	}
	return true
}

// endpointIndex is the set of Endpoints parsed from one version of a store.
type endpointIndex struct {
	version uint64
	byName  map[string]EndpointInfo
}

var (
	sharedEndpointIndexMu sync.RWMutex
	sharedEndpointIndex   *endpointIndex
)

// LookupEndpoint returns the gateway-wide Endpoint called name, and whether the
// gateway has one. It reads the shared lazy resource store through an index
// that is rebuilt only when the store changes, so a lookup per request is a map
// read.
func LookupEndpoint(name string) (EndpointInfo, bool) {
	store := GetLazyResourceStoreInstance()
	version := store.Version()

	sharedEndpointIndexMu.RLock()
	index := sharedEndpointIndex
	sharedEndpointIndexMu.RUnlock()

	if index == nil || index.version != version {
		index = buildEndpointIndex(store, version)
		sharedEndpointIndexMu.Lock()
		// A concurrent rebuild from a newer version wins.
		if sharedEndpointIndex == nil || sharedEndpointIndex.version <= version {
			sharedEndpointIndex = index
		}
		sharedEndpointIndexMu.Unlock()
	}

	info, ok := index.byName[name]
	return info, ok
}

// buildEndpointIndex parses every Endpoint resource in store. A resource that
// lacks a well-formed name or a cluster name is left out, so it is never
// routable.
func buildEndpointIndex(store *LazyResourceStore, version uint64) *endpointIndex {
	resources, _ := store.GetResourcesByType(LazyResourceTypeEndpoint)
	index := &endpointIndex{version: version, byName: make(map[string]EndpointInfo, len(resources))}
	for id, resource := range resources {
		if resource == nil || !IsValidEndpointName(id) {
			continue
		}
		clusterName, _ := resource.Resource["clusterName"].(string)
		if clusterName != EndpointClusterPrefix+id {
			continue
		}
		basePath, _ := resource.Resource["basePath"].(string)
		index.byName[id] = EndpointInfo{
			Name:        id,
			Labels:      endpointLabels(resource.Resource["labels"]),
			ClusterName: clusterName,
			BasePath:    basePath,
		}
	}
	return index
}

// endpointLabels reads a labels value as delivered over xDS, where a string
// map arrives as map[string]interface{}.
func endpointLabels(raw interface{}) map[string]string {
	labels := map[string]string{}
	switch typed := raw.(type) {
	case map[string]string:
		for key, value := range typed {
			labels[key] = value
		}
	case map[string]interface{}:
		for key, value := range typed {
			if s, ok := value.(string); ok {
				labels[key] = s
			}
		}
	}
	return labels
}
