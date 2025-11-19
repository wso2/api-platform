/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
)

// PolicyConfiguration represents the complete policy configuration for routes
type PolicyConfiguration struct {
	Routes   []RoutePolicy `json:"routes"`
	Metadata Metadata      `json:"metadata"`
}

// RoutePolicy represents policy configuration for a specific route
type RoutePolicy struct {
	RouteKey         string   `json:"route_key"`
	RequestPolicies  []Policy `json:"request_policies"`
	ResponsePolicies []Policy `json:"response_policies"`
}

// Policy represents a single policy (request or response)
type Policy struct {
	Name               string                 `json:"name"`
	Version            string                 `json:"version"`
	ExecutionCondition *string                `json:"executionCondition"`
	Config             map[string]interface{} `json:"config"`
}

// Metadata contains metadata about the policy configuration
type Metadata struct {
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	ResourceVersion int64  `json:"resource_version"`
	APIName         string `json:"api_name"`
	Version         string `json:"version"`
	Context         string `json:"context"`
}

// GetConfigAsJSON returns the policy config as a JSON string
func (p *Policy) GetConfigAsJSON() (string, error) {
	data, err := json.Marshal(p.Config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StoredPolicyConfig represents the stored policy configuration with versioning
type StoredPolicyConfig struct {
	ID            string              `json:"id"`
	Configuration PolicyConfiguration `json:"configuration"`
	Version       int64               `json:"version"`
}

// GetCompositeKey returns a composite key for indexing: "api_name:version:context"
func (s *StoredPolicyConfig) GetCompositeKey() string {
	return s.Configuration.Metadata.APIName + ":" +
		s.Configuration.Metadata.Version + ":" +
		s.Configuration.Metadata.Context
}

// GetAPIName returns the API name from metadata
func (s *StoredPolicyConfig) GetAPIName() string {
	return s.Configuration.Metadata.APIName
}

// GetAPIVersion returns the API version from metadata
func (s *StoredPolicyConfig) GetAPIVersion() string {
	return s.Configuration.Metadata.Version
}

// GetContext returns the context from metadata
func (s *StoredPolicyConfig) GetContext() string {
	return s.Configuration.Metadata.Context
}
