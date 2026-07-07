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

package v1alpha1

// UpstreamDefinition is a reusable upstream configuration with an optional connect
// timeout and load-balancing targets. Referenced from an upstream via its `ref` field.
// Mirrors the management-API UpstreamDefinition schema.
type UpstreamDefinition struct {
	// Name Unique identifier for this upstream definition (referenced by upstream.ref).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9\-_]+$`
	Name string `json:"name"`

	// BasePath Base path prefix prepended to all requests routed through this upstream (e.g. /api/v2).
	// +optional
	BasePath *string `json:"basePath,omitempty"`

	// Timeout Optional timeout configuration for this upstream (connect timeout).
	// +optional
	Timeout *UpstreamTimeout `json:"timeout,omitempty"`

	// Upstreams List of backend targets with optional weights for load balancing.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Upstreams []UpstreamTarget `json:"upstreams"`
}

// UpstreamTimeout carries the per-upstream timeout configuration. Only the connect
// timeout is supported at the upstream-definition level.
type UpstreamTimeout struct {
	// Connect Connection-establishment timeout duration (e.g. "5s", "500ms"). "0s" disables.
	// +optional
	// +kubebuilder:validation:Pattern=`^\d+(\.\d+)?(ms|s|m|h)$`
	Connect *string `json:"connect,omitempty"`
}

// UpstreamTarget is a single backend target within an UpstreamDefinition.
type UpstreamTarget struct {
	// Url Backend URL (host and port only; path comes from the definition's basePath).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+$`
	Url string `json:"url"`

	// Weight Weight for load balancing (optional, default 100).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight *int `json:"weight,omitempty"`
}
