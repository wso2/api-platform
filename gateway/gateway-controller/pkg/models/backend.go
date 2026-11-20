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

package models

import (
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// BackendClusterPrefix prefixes the Envoy cluster of every global Backend. It is reserved:
// no API-owned cluster uses it. Must match the SDK's policy.BackendClusterPrefix.
const BackendClusterPrefix = "backend_"

// StoredBackend is a gateway-wide Backend as stored in the database and in memory.
type StoredBackend struct {
	UUID          string             `json:"uuid"`
	Configuration api.BackendRequest `json:"configuration"`
	Origin        Origin             `json:"origin"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

// GetHandle returns the Backend name (metadata.name).
func (e *StoredBackend) GetHandle() string {
	return e.Configuration.Metadata.Name
}

// ClusterName returns the Envoy cluster the Backend is served by.
func (e *StoredBackend) ClusterName() string {
	return BackendClusterPrefix + e.GetHandle()
}

// Labels returns the Backend's metadata.labels, never nil.
func (e *StoredBackend) Labels() map[string]string {
	if e.Configuration.Metadata.Labels == nil {
		return map[string]string{}
	}
	return *e.Configuration.Metadata.Labels
}

// BasePath returns spec.basePath, or "" when it is not set.
func (e *StoredBackend) BasePath() string {
	if e.Configuration.Spec.BasePath == nil {
		return ""
	}
	return *e.Configuration.Spec.BasePath
}

// UpstreamDefinition returns the Backend's spec in the shape of an upstreamDefinitions entry,
// so both are turned into clusters by the same code.
func (e *StoredBackend) UpstreamDefinition() api.UpstreamDefinition {
	return BackendUpstreamDefinition(e.Configuration)
}

// BackendUpstreamDefinition projects a Backend onto an upstreamDefinitions entry. The validator
// and the cluster translator both use it, so they always see the same settings.
func BackendUpstreamDefinition(backend api.BackendRequest) api.UpstreamDefinition {
	spec := backend.Spec
	return api.UpstreamDefinition{
		Name:      backend.Metadata.Name,
		BasePath:  spec.BasePath,
		Timeout:   spec.Timeout,
		Tls:       spec.Tls,
		Upstreams: spec.Endpoints,
	}
}
