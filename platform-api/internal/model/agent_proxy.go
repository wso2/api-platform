/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package model

import (
	"sort"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// AgentProxyProtocol identifies the communication protocol family an Agent proxy
// speaks. It is the persisted discriminator (agent_proxies.protocol) and is
// deliberately distinct from agentproto.ProtocolVersion, which selects the A2A
// *wire* version inside the A2A configuration block.
type AgentProxyProtocol string

// AgentProxyProtocolA2A is the canonical, lower-case stored value for A2A — the
// only protocol registered in Phase 1. A future protocol adds a sibling value
// here plus its own typed configuration block; it does not add a second
// discriminator inside AgentProxyConfiguration.
const AgentProxyProtocolA2A AgentProxyProtocol = "a2a"

// supportedAgentProxyProtocols is the closed set of values agent_proxies.protocol
// may hold. The column is a plain VARCHAR rather than a database enum so adding a
// protocol needs no DDL change on a shipped table.
var supportedAgentProxyProtocols = map[AgentProxyProtocol]struct{}{
	AgentProxyProtocolA2A: {},
}

// IsSupportedAgentProxyProtocol reports whether p is a protocol this platform
// version can store and deploy.
func IsSupportedAgentProxyProtocol(p AgentProxyProtocol) bool {
	_, ok := supportedAgentProxyProtocols[p]
	return ok
}

// SupportedAgentProxyProtocols returns the registered protocol values in a
// stable order, for error messages that have to name what is accepted. It is the
// one place that list comes from, so a newly registered protocol cannot be
// announced by one code path and omitted by another.
func SupportedAgentProxyProtocols() []string {
	out := make([]string, 0, len(supportedAgentProxyProtocols))
	for p := range supportedAgentProxyProtocols {
		out = append(out, string(p))
	}
	sort.Strings(out)
	return out
}

// Agent Card serving modes. Both the public and the protected card use them.
const (
	AgentCardModeManaged     = "managed"
	AgentCardModePassthrough = "passthrough"
)

// DefaultAgentCardPath is the A2A discovery path used when the public card
// configuration does not name one. A configured path replaces this route rather
// than aliasing it.
const DefaultAgentCardPath = "/.well-known/agent-card.json"

// AgentProxy is one editable Agent proxy, owned by exactly one organization and
// one project. Identity, ownership, protocol selection, provenance and audit
// metadata live in their own columns; everything else the author writes lives in
// Configuration, which is stored as one atomically replaced JSON document.
//
// Only Configuration is serialized into agent_proxies.configuration. Nothing
// column-backed is duplicated inside it — in particular the protocol
// discriminator, which is read from the column on every load.
type AgentProxy struct {
	UUID             string             `json:"uuid" db:"-"`
	Handle           string             `json:"id" db:"-"`
	OrganizationUUID string             `json:"organizationId" db:"-"`
	ProjectUUID      string             `json:"projectId" db:"-"`
	Name             string             `json:"displayName" db:"-"`
	Description      string             `json:"description,omitempty" db:"-"`
	Protocol         AgentProxyProtocol `json:"protocol" db:"-"`
	CreatedBy        string             `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy        string             `json:"updatedBy,omitempty" db:"updated_by"`
	Version          string             `json:"version" db:"-"`
	CreatedAt        time.Time          `json:"createdAt" db:"-"`
	UpdatedAt        time.Time          `json:"updatedAt" db:"-"`

	// Configuration is the only field written to the JSON column.
	Configuration AgentProxyConfiguration `json:"configuration" db:"-"`

	Origin      string `json:"origin,omitempty" db:"origin"`
	DataVersion string `json:"dataVersion,omitempty" db:"data_version"`

	// AssociatedGateways is loaded from / written to artifact_gateway_mappings and
	// is never serialized into the configuration document. ReplaceAssociatedGateways
	// opts an update into full replacement of that set.
	AssociatedGateways        []AssociatedGatewayMapping `json:"-" db:"-"`
	ReplaceAssociatedGateways bool                       `json:"-" db:"-"`
}

// IsReadOnly reports whether the Agent proxy was imported from a data-plane
// gateway and is therefore not editable in the control plane. readOnly is derived
// from origin, never stored a second time.
func (a *AgentProxy) IsReadOnly() bool {
	return a != nil && a.Origin == constants.OriginDP
}

// AgentProxyConfiguration is the authoring document persisted in
// agent_proxies.configuration. It holds routing, upstream, Agent-wide resilience
// and exactly one typed protocol block — and nothing that has its own column: no
// name, description, version, ownership, audit, origin, data version, and no
// protocol discriminator.
type AgentProxyConfiguration struct {
	Context    *string            `json:"context,omitempty"`
	Vhost      *string            `json:"vhost,omitempty"`
	Upstream   UpstreamConfig     `json:"upstream"`
	Resilience *Resilience        `json:"resilience,omitempty"`
	A2A        *A2AProtocolConfig `json:"a2a,omitempty"`
}

// Resilience carries the timeout settings applied to a request chain. An Agent-wide
// value is the default; a per-operation value overrides it.
type Resilience struct {
	Timeout     string `json:"timeout,omitempty"`
	IdleTimeout string `json:"idleTimeout,omitempty"`
}

// A2AProtocolConfig is the typed configuration required when Protocol is
// AgentProxyProtocolA2A. The pointer on AgentProxyConfiguration preserves block
// presence, so a missing block is distinguishable from an empty one.
//
// Its layout matches the gateway's spec.a2a: OperationConfigs is required and is
// a value, not a pointer, because it carries the required transports.
type A2AProtocolConfig struct {
	ProtocolVersion  string              `json:"protocolVersion"`
	OperationConfigs A2AOperationConfigs `json:"operationConfigs"`
	AgentCard        *AgentCardConfig    `json:"agentCard,omitempty"`
}

// A2AOperationConfigs holds the transports the Agent proxy is served on, the
// Agent-wide policy position for A2A and any per-operation additions. Operations
// is not an allowlist: an operation that is not listed still receives Policies.
type A2AOperationConfigs struct {
	Transports []A2ATransport `json:"transports"`
	Policies   []Policy       `json:"policies,omitempty"`
	Operations []A2AOperation `json:"operations,omitempty"`
}

// A2ATransport is one A2A protocol binding and the path prefix it is served on,
// relative to the Agent proxy's context.
type A2ATransport struct {
	ProtocolBinding string  `json:"protocolBinding"`
	PathPrefix      *string `json:"pathPrefix,omitempty"`
}

// A2AOperation is per-operation configuration for one canonical A2A operation.
// Its policies are appended after the common ones; its resilience overrides the
// Agent-wide default.
type A2AOperation struct {
	Name       string      `json:"name"`
	Policies   []Policy    `json:"policies,omitempty"`
	Resilience *Resilience `json:"resilience,omitempty"`
}

// AgentCardConfig holds the public and protected Agent Card configuration. An
// omitted block stays omitted: defaults are resolved when the artifact is built,
// never materialized into stored configuration.
type AgentCardConfig struct {
	Public    *PublicAgentCard    `json:"public,omitempty"`
	Protected *ProtectedAgentCard `json:"protected,omitempty"`
}

// AgentCardDocument is a complete A2A Agent Card, carried as the author supplied
// it. Its structure is the author's to write, so extension fields are preserved.
type AgentCardDocument map[string]interface{}

// PublicAgentCard is the public Agent Card configuration. Managed and passthrough
// are disjoint: managed requires Content and forbids RewriteUrls; passthrough
// forbids Content and may omit Mode. The service enforces that disjointness —
// this struct carries both branches so an omitted field stays omitted.
type PublicAgentCard struct {
	Mode        *string           `json:"mode,omitempty"`
	Path        *string           `json:"path,omitempty"`
	Policies    []Policy          `json:"policies,omitempty"`
	RewriteUrls *bool             `json:"rewriteUrls,omitempty"`
	Content     AgentCardDocument `json:"content,omitempty"`
}

// ProtectedAgentCard is the protected (extended) Agent Card configuration. It is
// an A2A operation rather than a discovery route, so it carries no path and no
// policies, and its mode is always explicit when the block is present.
type ProtectedAgentCard struct {
	Mode        string            `json:"mode"`
	RewriteUrls *bool             `json:"rewriteUrls,omitempty"`
	Content     AgentCardDocument `json:"content,omitempty"`
}

// EffectivePublicCardMode resolves the public Agent Card mode without probing for
// the presence of content: an absent card block, an absent public block and an
// absent mode all mean passthrough.
func (c *A2AProtocolConfig) EffectivePublicCardMode() string {
	if c == nil || c.AgentCard == nil || c.AgentCard.Public == nil || c.AgentCard.Public.Mode == nil {
		return AgentCardModePassthrough
	}
	return *c.AgentCard.Public.Mode
}

// EffectivePublicCardPath resolves the path the public Agent Card is served on.
func (c *A2AProtocolConfig) EffectivePublicCardPath() string {
	if c == nil || c.AgentCard == nil || c.AgentCard.Public == nil ||
		c.AgentCard.Public.Path == nil || *c.AgentCard.Public.Path == "" {
		return DefaultAgentCardPath
	}
	return *c.AgentCard.Public.Path
}

// ---------------------------------------------------------------------------
// Gateway artifact (kind: Agent)
// ---------------------------------------------------------------------------

// AgentProxyDeploymentYAML is the gateway-facing deployment artifact for an Agent
// proxy. The control-plane kind is AgentProxy; the gateway artifact kind stays
// Agent, and the mapping happens here at the boundary rather than by storing the
// gateway's vocabulary.
//
// These types are deliberately separate from the persisted model: they carry
// explicit yaml tags so the emitted document matches the field names the gateway
// controller's Agent validator accepts, and they omit every control-plane-only
// field (protocol, ownership, audit, associations).
type AgentProxyDeploymentYAML struct {
	ApiVersion string                   `yaml:"apiVersion"`
	Kind       string                   `yaml:"kind"`
	Metadata   DeploymentMetadata       `yaml:"metadata"`
	Spec       AgentProxyDeploymentSpec `yaml:"spec"`
}

// GetApiVersion returns the artifact's CRD apiVersion.
//
// Both accessors are required by the version translator's artifact interface; a
// deployment type missing either is silently skipped during version translation
// rather than failing loudly, so the compile-time assertion in the tests pins them.
func (d *AgentProxyDeploymentYAML) GetApiVersion() string { return d.ApiVersion }

// SetApiVersion sets the artifact's CRD apiVersion.
func (d *AgentProxyDeploymentYAML) SetApiVersion(v string) { d.ApiVersion = v }

// AgentProxyDeploymentSpec is the gateway artifact's spec block.
type AgentProxyDeploymentSpec struct {
	DisplayName string                     `yaml:"displayName"`
	Version     string                     `yaml:"version"`
	Context     *string                    `yaml:"context,omitempty"`
	Vhost       *string                    `yaml:"vhost,omitempty"`
	Upstream    AgentDeploymentUpstream    `yaml:"upstream"`
	Resilience  *AgentDeploymentResilience `yaml:"resilience,omitempty"`
	A2A         AgentDeploymentA2A         `yaml:"a2a"`
}

// AgentDeploymentUpstream is the gateway's upstream block. The gateway accepts a
// single endpoint, so only the main upstream crosses the boundary.
type AgentDeploymentUpstream struct {
	URL  string                       `yaml:"url,omitempty"`
	Ref  string                       `yaml:"ref,omitempty"`
	Auth *AgentDeploymentUpstreamAuth `yaml:"auth,omitempty"`
}

// AgentDeploymentUpstreamAuth carries upstream credentials. Value is normally a
// {{ secret "handle" }} placeholder resolved through the secret store.
type AgentDeploymentUpstreamAuth struct {
	Type   string `yaml:"type"`
	Header string `yaml:"header,omitempty"`
	Value  string `yaml:"value,omitempty"`
}

// AgentDeploymentResilience is the gateway's timeout block.
type AgentDeploymentResilience struct {
	Timeout     string `yaml:"timeout,omitempty"`
	IdleTimeout string `yaml:"idleTimeout,omitempty"`
}

// AgentDeploymentA2A is the gateway's spec.a2a block. It has the same layout as
// the control plane's a2a block — transports live under operationConfigs in both.
type AgentDeploymentA2A struct {
	ProtocolVersion  string                          `yaml:"protocolVersion"`
	OperationConfigs AgentDeploymentOperationConfigs `yaml:"operationConfigs"`
	AgentCard        *AgentDeploymentAgentCard       `yaml:"agentCard,omitempty"`
}

// AgentDeploymentOperationConfigs carries the required transports plus the common
// and per-operation policy positions.
type AgentDeploymentOperationConfigs struct {
	Transports []AgentDeploymentTransport `yaml:"transports"`
	Policies   []AgentDeploymentPolicy    `yaml:"policies,omitempty"`
	Operations []AgentDeploymentOperation `yaml:"operations,omitempty"`
}

// AgentDeploymentTransport is one gateway-facing A2A binding and its path prefix.
type AgentDeploymentTransport struct {
	ProtocolBinding string `yaml:"protocolBinding"`
	PathPrefix      string `yaml:"pathPrefix,omitempty"`
}

// AgentDeploymentOperation is per-operation gateway configuration.
type AgentDeploymentOperation struct {
	Name       string                     `yaml:"name"`
	Policies   []AgentDeploymentPolicy    `yaml:"policies,omitempty"`
	Resilience *AgentDeploymentResilience `yaml:"resilience,omitempty"`
}

// AgentDeploymentPolicy is a policy attachment in the gateway artifact. It spells
// out its own yaml tags rather than reusing model.Policy, whose fields carry json
// tags only and would otherwise be emitted lower-cased (executioncondition).
type AgentDeploymentPolicy struct {
	Name               string                 `yaml:"name"`
	Version            string                 `yaml:"version"`
	Params             map[string]interface{} `yaml:"params,omitempty"`
	ExecutionCondition string                 `yaml:"executionCondition,omitempty"`
}

// AgentDeploymentAgentCard is the gateway's spec.a2a.agentCard block. An omitted
// public or protected block is preserved as omitted.
type AgentDeploymentAgentCard struct {
	Public    *AgentDeploymentPublicCard    `yaml:"public,omitempty"`
	Protected *AgentDeploymentProtectedCard `yaml:"protected,omitempty"`
}

// AgentDeploymentPublicCard is the gateway's public Agent Card configuration.
type AgentDeploymentPublicCard struct {
	Mode        string                  `yaml:"mode,omitempty"`
	Path        string                  `yaml:"path,omitempty"`
	Policies    []AgentDeploymentPolicy `yaml:"policies,omitempty"`
	RewriteUrls *bool                   `yaml:"rewriteUrls,omitempty"`
	Content     map[string]interface{}  `yaml:"content,omitempty"`
}

// AgentDeploymentProtectedCard is the gateway's protected Agent Card configuration.
// It has no path and no policies: it is an A2A operation, not a discovery route.
type AgentDeploymentProtectedCard struct {
	Mode        string                 `yaml:"mode"`
	RewriteUrls *bool                  `yaml:"rewriteUrls,omitempty"`
	Content     map[string]interface{} `yaml:"content,omitempty"`
}
