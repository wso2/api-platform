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
	KindRestApi             ArtifactKind = "RestApi"
	KindWebSubApi           ArtifactKind = "WebSubApi"
	KindWebBrokerApi        ArtifactKind = "WebBrokerApi"
	KindMcp                 ArtifactKind = "Mcp"
	KindLlmProxy            ArtifactKind = "LlmProxy"
	KindLlmProvider         ArtifactKind = "LlmProvider"
	KindLlmProviderTemplate ArtifactKind = "LlmProviderTemplate"
	KindAgent               ArtifactKind = "Agent"
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
	DataVersion         string       `json:"dataVersion"`
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

	// Agent holds the persisted state that only an Agent artifact has. Nil for
	// every other kind, and for an Agent that has none of it.
	//
	// Every field above applies to any artifact; this one does not, so it is
	// kept behind a kind-scoped type rather than spreading Agent-shaped columns
	// across the shared struct. A later Agent-only column extends AgentArtifact
	// and leaves StoredConfig untouched.
	Agent *AgentArtifact `json:"agent,omitempty"`
}

// AgentArtifact is the per-artifact state the gateway produces for an Agent,
// as opposed to the Agent resource the user authored, which lives in
// Configuration.
//
// It is persisted alongside the configuration in the same row and the same
// transaction, so a signature can never be stored without the card bytes it was
// computed over.
type AgentArtifact struct {
	// SignedPublicCard and SignedProtectedCard hold the signed Agent Card
	// documents an Agent serves. Both are nil when the card is unsigned or
	// proxied from the upstream.
	//
	// They are an output, not an input: the card in Configuration is what the
	// user wrote, these are what the gateway signed at deploy time. They are
	// persisted rather than recomputed because signing happens only on deploy —
	// re-signing at startup would, for the randomized algorithms, change the
	// served bytes and ETag with no user-visible cause.
	//
	// The two card representations are validated, stored, and signed
	// independently, so an Agent may have either, both, or neither.
	//
	// Nothing populates them yet: card signing is unimplemented and rejected at
	// deploy time, so a managed protected card ships unsigned in Configuration
	// like the public one does.
	SignedPublicCard    *string `json:"signedPublicCard,omitempty"`
	SignedProtectedCard *string `json:"signedProtectedCard,omitempty"`
}

// SignedPublicCard returns the signed public Agent Card, or nil when this
// artifact is not an Agent or its card is unsigned. It saves every caller a
// two-step nil check.
func (c *StoredConfig) SignedPublicCard() *string {
	if c.Agent == nil {
		return nil
	}
	return c.Agent.SignedPublicCard
}

// SignedProtectedCard returns the signed protected Agent Card, or nil when this
// artifact is not an Agent or has no protected card.
func (c *StoredConfig) SignedProtectedCard() *string {
	if c.Agent == nil {
		return nil
	}
	return c.Agent.SignedProtectedCard
}

// GetCompositeKey returns the composite key "kind:displayName:version" for indexing
func (c *StoredConfig) GetCompositeKey() string {
	return fmt.Sprintf("%s:%s:%s", c.Kind, c.DisplayName, c.Version)
}

// GetApiVersion returns the YAML apiVersion string (e.g.
// "gateway.api-platform.wso2.com/v1") of the artifact. It prefers
// SourceConfiguration (the user-authored artifact, e.g. an LLMProxyConfiguration)
// and falls back to Configuration (which for LLM kinds is the derived RestAPI).
// Returns "" if neither carries a recognised typed configuration.
func (c *StoredConfig) GetApiVersion() string {
	if v := apiVersionOf(c.SourceConfiguration); v != "" {
		return v
	}
	return apiVersionOf(c.Configuration)
}

// apiVersionOf extracts the apiVersion from a typed configuration value.
func apiVersionOf(cfg any) string {
	switch sc := cfg.(type) {
	case api.RestAPI:
		return string(sc.ApiVersion)
	case api.LLMProviderConfiguration:
		return string(sc.ApiVersion)
	case api.LLMProxyConfiguration:
		return string(sc.ApiVersion)
	case api.MCPProxyConfiguration:
		return string(sc.ApiVersion)
	case api.AgentConfiguration:
		return string(sc.ApiVersion)
	}
	return ""
}

// GetContext returns the context path from SourceConfiguration with $version resolved.
func (c *StoredConfig) GetContext() (string, error) {
	switch sc := c.SourceConfiguration.(type) {
	case api.RestAPI:
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
	case api.AgentConfiguration:
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
	// Agent is deliberately absent: an Agent has no single spec-level policy
	// list. Its A2A operation chains and its Agent Card chain are built
	// separately by the Agent transformer from spec.a2a.operationConfigs.policies
	// and spec.a2a.agentCard.public.policies, so answering either one here would
	// misreport the other.
	// TODO: enable when policies are supported for WebSubHub
	return nil
}

// GetMetadata returns the metadata from the Configuration, regardless of type.
func (c *StoredConfig) GetMetadata() *api.Metadata {
	switch cfg := c.Configuration.(type) {
	case api.RestAPI:
		return &cfg.Metadata
	case api.AgentConfiguration:
		return &cfg.Metadata
	}
	return nil
}

// GetLabels returns the labels from the Configuration metadata, regardless of type.
func (c *StoredConfig) GetLabels() *map[string]string {
	switch cfg := c.Configuration.(type) {
	case api.RestAPI:
		return cfg.Metadata.Labels
	case api.AgentConfiguration:
		return cfg.Metadata.Labels
	}
	return nil
}

// GetAnnotations returns the annotations from the Configuration metadata, regardless of type.
func (c *StoredConfig) GetAnnotations() *map[string]string {
	switch cfg := c.Configuration.(type) {
	case api.RestAPI:
		return cfg.Metadata.Annotations
	case api.AgentConfiguration:
		return cfg.Metadata.Annotations
	}
	return nil
}
