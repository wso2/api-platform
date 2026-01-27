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

// GetCompositeKey returns the composite key "displayName:version" for indexing
func (c *StoredConfig) GetCompositeKey() string {
	if c.Configuration.Kind == api.WebSubApi {
		asyncData, err := c.Configuration.Spec.AsWebhookAPIData()
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%s:%s", asyncData.DisplayName, asyncData.Version)
	}
	configData, err := c.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s:%s", configData.DisplayName, configData.Version)
}

// GetDisplayName returns the API display name
func (c *StoredConfig) GetDisplayName() string {
	if c.Configuration.Kind == api.WebSubApi {
		asyncData, err := c.Configuration.Spec.AsWebhookAPIData()
		if err != nil {
			return ""
		}
		return asyncData.DisplayName
	}
	configData, err := c.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		return ""
	}
	return configData.DisplayName
}

// GetHandle returns the API handle from metadata.name
func (c *StoredConfig) GetHandle() string {
	return c.Configuration.Metadata.Name
}

// GetVersion returns the API version
func (c *StoredConfig) GetVersion() string {
	if c.Configuration.Kind == api.WebSubApi {
		asyncData, err := c.Configuration.Spec.AsWebhookAPIData()
		if err != nil {
			return ""
		}
		return asyncData.Version
	}
	configData, err := c.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		return ""
	}
	return configData.Version
}

// GetContext returns the API context path
func (c *StoredConfig) GetContext() string {
	if c.Configuration.Kind == api.WebSubApi {
		asyncData, err := c.Configuration.Spec.AsWebhookAPIData()
		if err != nil {
			return ""
		}
		return asyncData.Context
	}
	configData, err := c.Configuration.Spec.AsAPIConfigData()
	if err != nil {
		return ""
	}
	return configData.Context
}

func (c *StoredConfig) GetPolicies() *[]api.Policy {
	if c.Configuration.Kind == api.RestApi {
		httpData, err := c.Configuration.Spec.AsAPIConfigData()
		if err != nil {
			return nil
		}
		return httpData.Policies
	} else {
		// TODO: enable when policies are supported for WebSubHub
	}
	return nil
}
