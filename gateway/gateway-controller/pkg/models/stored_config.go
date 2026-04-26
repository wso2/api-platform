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
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// ArtifactKind identifies the type of configuration stored in the database.
// These constants are decoupled from the OpenAPI-generated kind enums so that
// renaming a field in the spec does not silently break DB queries.
type ArtifactKind = string

const (
	KindRestApi     ArtifactKind = "RestApi"
	KindWebSubApi   ArtifactKind = "WebSubApi"
	KindMcp         ArtifactKind = "Mcp"
	KindLlmProxy    ArtifactKind = "LlmProxy"
	KindLlmProvider ArtifactKind = "LlmProvider"
)

// DesiredState represents the intended deployment state of an API configuration.
// It reflects what the user wants (deployed or undeployed), not the runtime status.
type DesiredState string

const (
	StateDeployed   DesiredState = "deployed"   // User wants this configuration active in Router
	StateUndeployed DesiredState = "undeployed" // User wants this configuration removed from Router
)

// ParseDesiredState normalises and validates a string into a DesiredState.
// Returns the matching state and true, or ("", false) for unrecognised values.
func ParseDesiredState(s string) (DesiredState, bool) {
	switch strings.ToLower(s) {
	case string(StateDeployed):
		return StateDeployed, true
	case string(StateUndeployed):
		return StateUndeployed, true
	default:
		return "", false
	}
}

// Origin identifies how an artifact was created.
type Origin string

const (
	OriginControlPlane Origin = "control_plane" // Deployed via platform-API WebSocket events
	OriginGatewayAPI   Origin = "gateway_api"   // Created directly via gateway REST API
)

// CPSyncStatus represents the sync state of a gateway-created artifact with the on-prem control plane (relevant to bottom up API deployments).
type CPSyncStatus string

const (
	CPSyncStatusPending CPSyncStatus = "pending" // Awaiting sync to control plane
	CPSyncStatusSuccess CPSyncStatus = "success" // Successfully synced to control plane
	CPSyncStatusFailed  CPSyncStatus = "failed"  // Sync failed after retries
)

// IsValidOrigin returns true if the origin value is a recognized enum value.
func IsValidOrigin(o Origin) bool {
	return o == OriginControlPlane || o == OriginGatewayAPI
}

// StoredConfig represents the configuration stored in the database and in-memory
type StoredConfig struct {
	UUID                string       `json:"uuid"`
	Kind                string       `json:"kind"`
	Handle              string       `json:"handle"`
	DisplayName         string       `json:"displayName"`
	Version             string       `json:"version"`
	Configuration       any          `json:"configuration"`
	SourceConfiguration any          `json:"source_configuration,omitempty"`
	DesiredState        DesiredState `json:"desiredState"`
	DeploymentID        string       `json:"deploymentId,omitempty"`
	Origin              Origin       `json:"origin"`
	CreatedAt           time.Time    `json:"createdAt"`
	UpdatedAt           time.Time    `json:"updatedAt"`
	DeployedAt          *time.Time   `json:"deployedAt,omitempty"`
	SensitiveValues     []string     `json:"-"`                      // not persisted — holds resolved secret values for redaction
	CPSyncStatus        CPSyncStatus `json:"cpSyncStatus,omitempty"` // pending, success, failed
	CPSyncInfo          string       `json:"cpSyncInfo,omitempty"`   // failure detail when CPSyncStatus=failed
	CPArtifactID        string       `json:"-"`                      // APIM/CP UUID for bottom-up synced artifacts; populated after successful sync
}

// GetCompositeKey returns the composite key "kind:displayName:version" for indexing
func (c *StoredConfig) GetCompositeKey() string {
	return fmt.Sprintf("%s:%s:%s", c.Kind, c.DisplayName, c.Version)
}

// GetContext returns the context path from SourceConfiguration with $version resolved.
func (c *StoredConfig) GetContext() (string, error) {
	switch sc := c.SourceConfiguration.(type) {
	case api.RestAPI:
		return strings.ReplaceAll(sc.Spec.Context, "$version", c.Version), nil
	case api.WebSubAPI:
		return strings.ReplaceAll(sc.Spec.Context, "$version", c.Version), nil
	case api.LLMProviderConfiguration:
		if sc.Spec.Context != nil {
			return strings.ReplaceAll(*sc.Spec.Context, "$version", c.Version), nil
		}
		return "", nil
	case api.LLMProxyConfiguration:
		if sc.Spec.Context != nil {
			return strings.ReplaceAll(*sc.Spec.Context, "$version", c.Version), nil
		}
		return "", nil
	case api.MCPProxyConfiguration:
		if sc.Spec.Context != nil {
			return strings.ReplaceAll(*sc.Spec.Context, "$version", c.Version), nil
		}
		return "", nil
	}
	return "", fmt.Errorf("unsupported source configuration type: %T", c.SourceConfiguration)
}

func (c *StoredConfig) GetPolicies() *[]api.Policy {
	if sc, ok := c.Configuration.(api.RestAPI); ok {
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

// GetAnnotations returns the annotations from the Configuration metadata, regardless of type.
func (c *StoredConfig) GetAnnotations() *map[string]string {
	switch cfg := c.Configuration.(type) {
	case api.RestAPI:
		return cfg.Metadata.Annotations
	case api.WebSubAPI:
		return cfg.Metadata.Annotations
	}
	return nil
}
