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
	"regexp"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// EndpointClusterPrefix prefixes the Envoy cluster of every gateway-wide
// Endpoint. The prefix is reserved: every API cluster is named "upstream_..."
// (or "cluster_..." on the legacy path), so an Endpoint cluster can never
// collide with one. It must match the policy engine's
// policy.EndpointClusterPrefix; the two are separate constants because the
// controller and the runtime agree on a wire value.
const EndpointClusterPrefix = "endpoint_"

// Endpoint names follow the platform-api handle rule, so a gateway-created
// Endpoint imports into the control plane unchanged, a name never needs
// sanitising inside a cluster name, and a request-supplied value can be
// rejected before any lookup.
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

// EndpointClusterName returns the Envoy cluster name of the Endpoint called
// name. It is stable for the life of the Endpoint, so changing an Endpoint's
// upstreams never changes what a route or the policy engine refers to.
func EndpointClusterName(name string) string {
	return EndpointClusterPrefix + name
}

// StoredEndpoint is a gateway-wide Endpoint as stored in the database and held
// in memory.
type StoredEndpoint struct {
	UUID          string                    `json:"uuid"`
	Configuration api.EndpointConfiguration `json:"configuration"`
	CreatedAt     time.Time                 `json:"createdAt"`
	UpdatedAt     time.Time                 `json:"updatedAt"`
}

// GetHandle returns the Endpoint's name (metadata.name).
func (e *StoredEndpoint) GetHandle() string {
	return e.Configuration.Metadata.Name
}

// ClusterName returns the Envoy cluster that carries this Endpoint's traffic.
func (e *StoredEndpoint) ClusterName() string {
	return EndpointClusterName(e.GetHandle())
}

// Labels returns the Endpoint's metadata.labels. Never nil.
func (e *StoredEndpoint) Labels() map[string]string {
	if e.Configuration.Metadata.Labels == nil {
		return map[string]string{}
	}
	labels := make(map[string]string, len(*e.Configuration.Metadata.Labels))
	for key, value := range *e.Configuration.Metadata.Labels {
		labels[key] = value
	}
	return labels
}

// BasePath returns the path prefixed to requests sent to this Endpoint, "/"
// when the Endpoint sets none.
func (e *StoredEndpoint) BasePath() string {
	if bp := e.Configuration.Spec.BasePath; bp != nil && *bp != "" {
		return *bp
	}
	return "/"
}

// UpstreamDefinition returns the Endpoint's backend as an upstream definition
// named after the Endpoint. An Endpoint's spec has the fields of an
// upstreamDefinitions entry, and this is what lets the two share one
// validator, one converter and one cluster builder.
func (e *StoredEndpoint) UpstreamDefinition() api.UpstreamDefinition {
	return EndpointUpstreamDefinition(e.Configuration.Metadata.Name, e.Configuration.Spec)
}

// EndpointUpstreamDefinition returns an Endpoint spec as an upstream definition
// called name.
func EndpointUpstreamDefinition(name string, spec api.EndpointConfigurationData) api.UpstreamDefinition {
	return api.UpstreamDefinition{
		Name:      name,
		BasePath:  spec.BasePath,
		Timeout:   spec.Timeout,
		Tls:       spec.Tls,
		Upstreams: spec.Upstreams,
	}
}
