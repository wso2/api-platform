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

// ArtifactKind identifies the type of configuration stored in the database.
// These constants are decoupled from the OpenAPI-generated kind enums so that
// renaming a field in the spec does not silently break DB queries.
type ArtifactKind = string

const (
	KindRestApi    ArtifactKind = "RestApi"
	KindWebSubApi  ArtifactKind = "WebSubApi"
	KindMcp        ArtifactKind = "Mcp"
	KindLlmProxy   ArtifactKind = "LlmProxy"
	KindLlmProvider ArtifactKind = "LlmProvider"
)

// ConfigStatus represents the lifecycle state of an API configuration
type ConfigStatus string

const (
	StatusPending    ConfigStatus = "pending"    // Submitted but not yet deployed
	StatusDeployed   ConfigStatus = "deployed"   // Active in Router
	StatusFailed     ConfigStatus = "failed"     // Deployment failed
	StatusUndeployed ConfigStatus = "undeployed" // Removed from Router but config preserved
)

// StoredConfig represents the configuration stored in the database and in-memory
type StoredConfig struct {
	UUID                string               `json:"uuid"`
	Kind                string               `json:"kind"`
	Handle              string               `json:"handle"`
	DisplayName         string               `json:"displayName"`
	Version             string               `json:"version"`
	Configuration       any                  `json:"configuration"`
	SourceConfiguration any                  `json:"source_configuration,omitempty"`
	Status              ConfigStatus         `json:"status"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
	DeployedAt          *time.Time           `json:"deployedAt,omitempty"`
	DeployedVersion     int64                `json:"deployed_version"` // Runtime-only: xDS snapshot version, not persisted to DB
}

// GetCompositeKey returns the composite key "kind:displayName:version" for indexing
func (c *StoredConfig) GetCompositeKey() string {
	return fmt.Sprintf("%s:%s:%s", c.Kind, c.DisplayName, c.Version)
}

// GetContext returns the context path from SourceConfiguration.
func (c *StoredConfig) GetContext() (string, error) {
	switch sc := c.SourceConfiguration.(type) {
	case api.RestAPI:
		return sc.Spec.Context, nil
	case api.WebSubAPI:
		return sc.Spec.Context, nil
	case api.LLMProviderConfiguration:
		if sc.Spec.Context != nil {
			return *sc.Spec.Context, nil
		}
		return "", nil
	case api.LLMProxyConfiguration:
		if sc.Spec.Context != nil {
			return *sc.Spec.Context, nil
		}
		return "", nil
	case api.MCPProxyConfiguration:
		if sc.Spec.Context != nil {
			return *sc.Spec.Context, nil
		}
		return "", nil
	}
	return "", fmt.Errorf("unsupported source configuration type: %T", c.SourceConfiguration)
}

func (c *StoredConfig) GetPolicies() *[]api.Policy {
	if sc, ok := c.SourceConfiguration.(api.RestAPI); ok {
		return sc.Spec.Policies
	}
	// TODO: enable when policies are supported for WebSubHub
	return nil
}

// GetMetadata returns the metadata from the Configuration, regardless of type.
func (c *StoredConfig) GetMetadata() *api.Metadata {
	switch cfg := c.Configuration.(type) {
	case api.RestAPI:
		return &cfg.Metadata
	case api.WebSubAPI:
		return &cfg.Metadata
	}
	return nil
}

// GetLabels returns the labels from the Configuration metadata, regardless of type.
func (c *StoredConfig) GetLabels() *map[string]string {
	switch cfg := c.Configuration.(type) {
	case api.RestAPI:
		return cfg.Metadata.Labels
	case api.WebSubAPI:
		return cfg.Metadata.Labels
	}
	return nil
}
