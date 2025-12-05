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
	"fmt"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// ConfigStatus represents the lifecycle state of an API configuration
type ConfigStatus string

const (
	StatusPending  ConfigStatus = "pending"  // Submitted but not yet deployed
	StatusDeployed ConfigStatus = "deployed" // Active in Router
	StatusFailed   ConfigStatus = "failed"   // Deployment failed
)

// StoredConfig represents the configuration stored in the database and in-memory
type StoredConfig struct {
	ID                  string               `json:"id"`
	Kind                string               `json:"kind"`
	Configuration       api.APIConfiguration `json:"configuration"`
	SourceConfiguration any                  `json:"source_configuration,omitempty"`
	Status              ConfigStatus         `json:"status"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
	DeployedAt          *time.Time           `json:"deployed_at,omitempty"`
	DeployedVersion     int64                `json:"deployed_version"`
}

// GetCompositeKey returns the composite key "name:version" for indexing
func (c *StoredConfig) GetCompositeKey() string {
	return fmt.Sprintf("%s:%s", c.Configuration.Spec.Name, c.Configuration.Spec.Version)
}

// GetName returns the API name
func (c *StoredConfig) GetName() string {
	return c.Configuration.Spec.Name
}

// GetVersion returns the API version
func (c *StoredConfig) GetVersion() string {
	return c.Configuration.Spec.Version
}

// GetContext returns the API context path
func (c *StoredConfig) GetContext() string {
	return c.Configuration.Spec.Context
}
