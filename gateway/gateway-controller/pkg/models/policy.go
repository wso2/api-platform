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
	policyenginev1 "github.com/policy-engine/sdk/policyengine/v1"
)

// StoredPolicyConfig represents the stored policy configuration with versioning.
// This is gateway-controller specific and wraps the SDK Configuration type.
type StoredPolicyConfig struct {
	// ID is the unique identifier for this policy configuration
	ID string `json:"id"`

	// Configuration is the actual policy configuration (from SDK)
	Configuration policyenginev1.Configuration `json:"configuration"`

	// Version is the version number for this stored configuration
	Version int64 `json:"version"`
}

// CompositeKey returns a composite key for indexing: "api_name:version:context"
func (s *StoredPolicyConfig) CompositeKey() string {
	return s.Configuration.Metadata.APIName + ":" +
		s.Configuration.Metadata.Version + ":" +
		s.Configuration.Metadata.Context
}

// APIName returns the API name from metadata
func (s *StoredPolicyConfig) APIName() string {
	return s.Configuration.Metadata.APIName
}

// APIVersion returns the API version from metadata
func (s *StoredPolicyConfig) APIVersion() string {
	return s.Configuration.Metadata.Version
}

// Context returns the context from metadata
func (s *StoredPolicyConfig) Context() string {
	return s.Configuration.Metadata.Context
}
