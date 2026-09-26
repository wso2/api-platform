/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/wso2/api-platform/platform-api/internal/agentproto"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Bottom-up (DP->CP) import of gateway-authored Agents.
//
// The gateway pushes its own artifact — kind Agent, spec.a2a — and the control
// plane stores it as kind AgentProxy with protocol a2a. This file is the inverse
// of utils.BuildAgentProxyDeploymentYAML: spec.a2a and the control plane's a2a
// block share a layout (transports under operationConfigs in both), and every
// field maps onto its namesake. The gateway's spec is never stored as
// control-plane configuration.
//
// Like every other importer, this one is lenient: it stores what maps onto the
// control-plane model and drops the rest. The deployment record the orchestrator
// writes keeps the pushed configuration verbatim, so what the gateway actually
// runs is never lost — only the read-only working copy is narrower. Fields the
// importer knows about but cannot store (upstreamDefinitions, a manual host
// rewrite, policy-backed upstream auth, enabled card signing) are named in a
// warning log so the gap is visible to an operator; fields it does not know at all
// are ignored, as utils.DecodeSpec ignores them for every kind.
//
// An import is refused only when there is nothing the control plane can store:
// a spec that is not an Agent at all, no a2a block (agent_proxies.protocol is
// required and A2A is the only protocol), an unregistered A2A protocol version
// (an Agent proxy on an operation set that does not exist), a handle wider than
// the agent_proxies.handle column, or a handle reserved for a static route under
// /agent-proxies/ (see reservedAgentProxyHandles). Nothing is ever fabricated to
// fill a gap.
//
// A refused import fails that one artifact: the reason is returned to the gateway,
// which records it as the artifact's cp_sync_status, and the rest of the batch
// proceeds.

// AgentCardCacheInvalidator drops the control plane's cached display fetch for one
// Agent proxy. It is satisfied by *AgentProxyService; the importer depends on the
// interface so an import that rewrites an Agent proxy's upstream or card mode
// invalidates the cache exactly as an authored update does.
type AgentCardCacheInvalidator interface {
	InvalidateAgentCard(orgUUID, proxyUUID string)
}

// agentProxyImporter imports gateway Agent artifacts (project-scoped).
type agentProxyImporter struct {
	agentProxyRepo repository.AgentProxyRepository
	cardCache      AgentCardCacheInvalidator
	slogger        *slog.Logger
}

func newAgentProxyImporter(agentProxyRepo repository.AgentProxyRepository, cardCache AgentCardCacheInvalidator,
	slogger *slog.Logger) *agentProxyImporter {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &agentProxyImporter{agentProxyRepo: agentProxyRepo, cardCache: cardCache, slogger: slogger}
}

// Kind is the incoming gateway artifact kind, not the stored one: dispatch is on
// what the gateway sends. The artifact is persisted as constants.AgentProxy.
func (i *agentProxyImporter) Kind() string          { return constants.GatewayKindAgent }
func (i *agentProxyImporter) RequiresProject() bool { return true }

func (i *agentProxyImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	handle := utils.ImportHandle(ctx.Configuration)

	// Mapped before anything is written, whatever the metadata mode: a push with
	// nothing the control plane can store is refused even when it would only have
	// added a deployment record.
	imported, dropped, err := mapGatewayAgentToAgentProxy(ctx.Configuration)
	if err != nil {
		return nil, err
	}
	if len(dropped) > 0 {
		// Field paths only — never values, which include upstream credentials.
		i.slogger.Warn("Gateway Agent carries configuration the control plane cannot represent; it is not stored in the working copy",
			"handle", handle, "dpId", ctx.DPID, "gatewayId", ctx.GatewayID, "droppedFields", dropped)
	}

	if ctx.Existing == nil {
		proxy := &model.AgentProxy{
			Handle:           handle,
			OrganizationUUID: ctx.OrgID,
			ProjectUUID:      ctx.ProjectID,
			Name:             imported.Name,
			Protocol:         imported.Protocol,
			Version:          imported.Version,
			Origin:           constants.OriginDP,
			DataVersion:      imported.DataVersion,
			Configuration:    imported.Configuration,
		}
		if err := i.agentProxyRepo.Create(proxy); err != nil {
			return nil, fmt.Errorf("failed to create agent proxy from gateway import: %w", err)
		}
		return &ImportResult{ID: proxy.UUID, DeployedVersion: proxy.Version, Deployable: true}, nil
	}

	existing, err := i.agentProxyRepo.GetByUUID(ctx.ID, ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing agent proxy: %w", err)
	}
	if existing == nil {
		// The orchestrator matched an AgentProxy artifact row by handle, so the kind
		// row must exist. Its absence is an inconsistency to report, not a cue to
		// record a deployment against a half-deleted artifact.
		return nil, fmt.Errorf("artifact %s is of kind %s but has no agent proxy row", ctx.ID, constants.AgentProxy)
	}
	// The protocol is fixed at creation. A gateway push cannot change it any more
	// than an authored update can.
	if existing.Protocol != imported.Protocol {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf(
			"The gateway Agent %q cannot be imported: the existing Agent proxy speaks %q and its protocol cannot be changed.",
			handle, string(existing.Protocol)))
	}

	switch ctx.MetadataMode {
	case utils.SkipWorkingCopy:
		// Stale, out-of-order push: a newer deployment already defines the working
		// copy. The orchestrator still records this gateway's deployment.
		return &ImportResult{ID: existing.UUID, DeployedVersion: imported.Version, Deployable: true}, nil
	case utils.WriteGatewaySpecificOnly:
		// Control-plane-owned: the gateway defines none of the working copy. An Agent
		// proxy has no gateway-specific fields outside the spec it would overwrite.
		return &ImportResult{ID: existing.UUID, DeployedVersion: imported.Version, Deployable: true}, nil
	case utils.WriteFullMetadata:
		if ctx.ProjectID != existing.ProjectUUID {
			// An artifact stays in the project it was created in: the repository's
			// update never rewrites project_uuid, for an import no more than for an
			// authored update. The move is logged rather than failing the push, so the
			// rest of the working copy still tracks the gateway.
			i.slogger.Warn("Gateway-imported Agent proxy names a different project; keeping the stored project",
				"handle", handle, "agentProxyUUID", existing.UUID, "storedProjectUUID", existing.ProjectUUID,
				"pushedProjectHandle", ctx.ProjectHandle)
		}
		existing.Name = imported.Name
		existing.Version = imported.Version
		existing.DataVersion = imported.DataVersion
		existing.Configuration = imported.Configuration
		// Gateway associations are the orchestrator's (ensureGatewayAssociation), so
		// the stored set is left as it is rather than replaced by an empty one.
		existing.ReplaceAssociatedGateways = false
		if err := i.agentProxyRepo.Update(existing); err != nil {
			return nil, fmt.Errorf("failed to update agent proxy from gateway import: %w", err)
		}
		// The upstream and card mode may have changed; a cached display fetch would
		// describe the Agent proxy as it was.
		if i.cardCache != nil {
			i.cardCache.InvalidateAgentCard(ctx.OrgID, existing.UUID)
		}
		return &ImportResult{ID: existing.UUID, DeployedVersion: existing.Version, Deployable: true}, nil
	default:
		return nil, fmt.Errorf("unknown metadata write mode %d", ctx.MetadataMode)
	}
}

// importedAgentProxy is the column-backed and configuration-document halves of a
// gateway Agent, mapped into the control plane's own vocabulary.
type importedAgentProxy struct {
	Name          string
	Version       string
	Protocol      model.AgentProxyProtocol
	DataVersion   string
	Configuration model.AgentProxyConfiguration
}

// mapGatewayAgentToAgentProxy decodes a pushed gateway Agent and maps it onto the
// control-plane model. dropped names every known gateway field that was present
// with a meaning the model cannot hold, in spec-path form, for the caller to log.
func mapGatewayAgentToAgentProxy(cfg dto.ArtifactImportConfig) (*importedAgentProxy, []string, error) {
	handle := utils.ImportHandle(cfg)
	refuse := func(cause error) error { return agentImportRefusal(handle, cause) }

	// agent_proxies.handle is VARCHAR(40), and the gateway allows far longer
	// handles. SQLite would store an over-long one silently while PostgreSQL and SQL
	// Server reject the insert, so the limit is checked here, the same on every
	// engine, rather than left to the database.
	if handle == "" {
		return nil, nil, refuse(errors.New("metadata.name is required"))
	}
	if n := utf8.RuneCountInString(handle); n > agentProxyImportMaxHandleLen {
		return nil, nil, refuse(fmt.Errorf("the handle is %d characters; the control plane stores at most %d", n, agentProxyImportMaxHandleLen))
	}
	// A reserved handle belongs to a static sibling route under /agent-proxies/.
	// Authored creates refuse it, and an import must not be the way around that:
	// see reservedAgentProxyHandles for why the segment is kept free.
	if _, reserved := reservedAgentProxyHandles[handle]; reserved {
		return nil, nil, refuse(fmt.Errorf("the handle %q is reserved by the control plane", handle))
	}

	var spec gatewayAgentSpec
	if len(cfg.Spec) == 0 {
		return nil, nil, refuse(errors.New("the artifact has no spec"))
	}
	if err := utils.DecodeSpec(cfg.Spec, &spec); err != nil {
		return nil, nil, refuse(err)
	}

	var dropped []string
	protocol, a2a, err := mapGatewayAgentProtocol(&spec, &dropped)
	if err != nil {
		return nil, nil, refuse(err)
	}

	imported := &importedAgentProxy{
		Name:     spec.DisplayName,
		Version:  spec.Version,
		Protocol: protocol,
		// The gateway kind is translated to the control-plane kind before it is
		// versioned, so an Agent is versioned by the AgentProxy entry rather than by
		// the unknown-kind default.
		DataVersion: string(gatewaytranslator.ComputeDataVersionForGatewayKind(cfg.Kind, cfg.APIVersion)),
		Configuration: model.AgentProxyConfiguration{
			Context:    clonePointer(spec.Context),
			Vhost:      clonePointer(spec.Vhost),
			Upstream:   mapGatewayAgentUpstream(spec.Upstream, spec.UpstreamDefinitions, &dropped),
			Resilience: mapGatewayAgentResilience(spec.Resilience),
			A2A:        a2a,
		},
	}
	return imported, dropped, nil
}

// agentProxyImportMaxHandleLen is the width of agent_proxies.handle.
const agentProxyImportMaxHandleLen = 40

// agentImportRefusal renders a refused import as one validation failure naming the
// Agent and the reason. The text is returned to the gateway, which records it as the
// artifact's cp_sync_info. Nothing in a reason carries a credential: the reasons
// name fields and versions only.
func agentImportRefusal(handle string, cause error) error {
	return apperror.ValidationFailed.New(
		fmt.Sprintf("The gateway Agent %q cannot be imported into the control plane: %s", handle, cause.Error()))
}

// mapGatewayAgentProtocol selects the protocol from the protocol block the spec
// actually carries. Phase 1 registers A2A only, so an Agent without spec.a2a has
// no protocol the control plane can store it under.
func mapGatewayAgentProtocol(spec *gatewayAgentSpec, dropped *[]string) (model.AgentProxyProtocol, *model.A2AProtocolConfig, error) {
	if spec.A2A == nil {
		return "", nil, errors.New("spec.a2a is required: the control plane stores Agents by protocol and A2A is the only protocol it supports")
	}
	// An unregistered version names an operation set that does not exist, so no
	// reader of the stored Agent proxy — the card path, the operation tables — could
	// make sense of it. This is the one A2A setting that is refused rather than
	// stored as sent.
	if !agentproto.IsSupportedVersion(agentproto.ProtocolVersion(spec.A2A.ProtocolVersion)) {
		return "", nil, fmt.Errorf("spec.a2a.protocolVersion %q is not supported; supported versions: %s",
			spec.A2A.ProtocolVersion, strings.Join(supportedProtocolVersionStrings(), ", "))
	}
	return model.AgentProxyProtocolA2A, mapGatewayA2A(spec.A2A, dropped), nil
}

// mapGatewayA2A is the inverse of utils.buildAgentDeploymentA2A. The gateway's
// spec.a2a and the control plane's a2a block share a layout — transports live
// under operationConfigs in both — so each field maps onto its namesake.
//
// A list's presence is preserved as sent: an absent policies or operations list
// stays nil and an explicitly empty one stays empty.
func mapGatewayA2A(in *gatewayA2AConfig, dropped *[]string) *model.A2AProtocolConfig {
	out := &model.A2AProtocolConfig{
		ProtocolVersion: in.ProtocolVersion,
		AgentCard:       mapGatewayAgentCard(in.AgentCard, dropped),
	}
	if in.OperationConfigs == nil {
		// The gateway requires this block, so its absence is not expected; the
		// Agent proxy is stored with no transports rather than refused.
		return out
	}

	cfgs := model.A2AOperationConfigs{
		Transports: make([]model.A2ATransport, 0, len(in.OperationConfigs.Transports)),
		Policies:   mapGatewayPolicies(in.OperationConfigs.Policies),
	}
	for _, t := range in.OperationConfigs.Transports {
		cfgs.Transports = append(cfgs.Transports, model.A2ATransport{
			ProtocolBinding: t.ProtocolBinding,
			PathPrefix:      clonePointer(t.PathPrefix),
		})
	}
	if in.OperationConfigs.Operations != nil {
		cfgs.Operations = make([]model.A2AOperation, 0, len(*in.OperationConfigs.Operations))
		for _, op := range *in.OperationConfigs.Operations {
			cfgs.Operations = append(cfgs.Operations, model.A2AOperation{
				Name:       op.Name,
				Policies:   mapGatewayPolicies(op.Policies),
				Resilience: mapGatewayAgentResilience(op.Resilience),
			})
		}
	}
	out.OperationConfigs = cfgs
	return out
}

// mapGatewayAgentCard carries the card block across without resolving a single
// default: an omitted public or protected block, mode, path or rewriteUrls stays
// omitted, and an explicit false stays false. Content is kept as the gateway holds
// it; nothing is added to a managed card that lacks a required key.
func mapGatewayAgentCard(in *gatewayAgentCard, dropped *[]string) *model.AgentCardConfig {
	if in == nil {
		return nil
	}
	out := &model.AgentCardConfig{}
	if pub := in.Public; pub != nil {
		noteEnabledCardSigning(pub.Signing, "public", dropped)
		out.Public = &model.PublicAgentCard{
			Mode:        clonePointer(pub.Mode),
			Path:        clonePointer(pub.Path),
			Policies:    mapGatewayPolicies(pub.Policies),
			RewriteUrls: clonePointer(pub.RewriteUrls),
			Content:     mapGatewayCardDocument(pub.Content),
		}
	}
	if prot := in.Protected; prot != nil {
		noteEnabledCardSigning(prot.Signing, "protected", dropped)
		out.Protected = &model.ProtectedAgentCard{
			Mode:        prot.Mode,
			RewriteUrls: clonePointer(prot.RewriteUrls),
			Content:     mapGatewayCardDocument(prot.Content),
		}
	}
	return out
}

// noteEnabledCardSigning records an enabled signing block as dropped. The control
// plane has no signing field yet (CP-D19). A disabled block means exactly what an
// absent one does, so it is not a loss worth reporting.
func noteEnabledCardSigning(signing *gatewayCardSigning, card string, dropped *[]string) {
	if signing != nil && signing.Enabled {
		*dropped = append(*dropped, "spec.a2a.agentCard."+card+".signing")
	}
}

func mapGatewayCardDocument(in *map[string]interface{}) model.AgentCardDocument {
	if in == nil || *in == nil {
		return nil
	}
	return model.AgentCardDocument(*in)
}

// mapGatewayAgentUpstream converts the gateway's single upstream into the control
// plane's main endpoint — the inverse of the builder, which only ever sends main.
//
// A ref is kept, but the upstreamDefinitions it resolves through have no place in
// the control-plane model and are dropped, as are a manual host rewrite and the
// policy-backed auth fields (policyName/policyParams/policyVersion). The auth type
// is stored as sent, including gateway-only types such as oauth2.
func mapGatewayAgentUpstream(in gatewayAgentUpstream, definitions *[]json.RawMessage, dropped *[]string) model.UpstreamConfig {
	if definitions != nil && len(*definitions) > 0 {
		*dropped = append(*dropped, "spec.upstreamDefinitions")
	}
	// auto is the gateway default, so it is not a loss.
	if in.HostRewrite != nil && *in.HostRewrite != gatewayHostRewriteAuto {
		*dropped = append(*dropped, "spec.upstream.hostRewrite")
	}

	endpoint := &model.UpstreamEndpoint{}
	if in.URL != nil {
		endpoint.URL = *in.URL
	}
	if in.Ref != nil {
		endpoint.Ref = *in.Ref
	}
	if in.Auth != nil {
		endpoint.Auth = mapGatewayAgentUpstreamAuth(in.Auth, dropped)
	}
	return model.UpstreamConfig{Main: endpoint}
}

func mapGatewayAgentUpstreamAuth(in *gatewayAgentUpstreamAuth, dropped *[]string) *model.UpstreamAuth {
	if in.PolicyName != nil {
		*dropped = append(*dropped, "spec.upstream.auth.policyName")
	}
	if in.PolicyParams != nil {
		*dropped = append(*dropped, "spec.upstream.auth.policyParams")
	}
	if in.PolicyVersion != nil {
		*dropped = append(*dropped, "spec.upstream.auth.policyVersion")
	}
	out := &model.UpstreamAuth{Type: in.Type}
	if in.Header != nil {
		out.Header = *in.Header
	}
	if in.Value != nil {
		out.Value = *in.Value
	}
	return out
}

func mapGatewayAgentResilience(in *gatewayResilience) *model.Resilience {
	if in == nil {
		return nil
	}
	out := &model.Resilience{}
	if in.Timeout != nil {
		out.Timeout = *in.Timeout
	}
	if in.IdleTimeout != nil {
		out.IdleTimeout = *in.IdleTimeout
	}
	return out
}

// mapGatewayPolicies preserves list presence: an absent list stays nil and an
// explicitly empty one stays empty, matching the forward mapping's view of both.
func mapGatewayPolicies(in *[]gatewayPolicy) []model.Policy {
	if in == nil {
		return nil
	}
	out := make([]model.Policy, 0, len(*in))
	for _, p := range *in {
		out = append(out, model.Policy{
			Name:               p.Name,
			Version:            p.Version,
			Params:             clonePointer(p.Params),
			ExecutionCondition: clonePointer(p.ExecutionCondition),
		})
	}
	return out
}

func clonePointer[T any](in *T) *T {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

// ---------------------------------------------------------------------------
// Gateway Agent spec, as pushed
// ---------------------------------------------------------------------------
//
// These mirror the gateway management API's AgentConfigData (kind: Agent) and are
// used for decoding a push only. They are deliberately not the deployment types in
// model: those carry yaml tags for emitting an artifact and model only what the
// control plane sends, whereas these also name the gateway fields the control plane
// cannot store, so that dropping one can be reported rather than happening unseen.

const gatewayHostRewriteAuto = "auto"

type gatewayAgentSpec struct {
	DisplayName string               `json:"displayName"`
	Version     string               `json:"version"`
	Context     *string              `json:"context,omitempty"`
	Vhost       *string              `json:"vhost,omitempty"`
	Upstream    gatewayAgentUpstream `json:"upstream"`
	// UpstreamDefinitions is decoded only so a non-empty list can be reported as
	// dropped; its entries are never read.
	UpstreamDefinitions *[]json.RawMessage `json:"upstreamDefinitions,omitempty"`
	Resilience          *gatewayResilience `json:"resilience,omitempty"`
	// DeploymentState is gateway lifecycle state. The push reports it separately
	// as the artifact's status, which is what the control plane records.
	DeploymentState *string           `json:"deploymentState,omitempty"`
	A2A             *gatewayA2AConfig `json:"a2a,omitempty"`
}

type gatewayAgentUpstream struct {
	URL         *string                   `json:"url,omitempty"`
	Ref         *string                   `json:"ref,omitempty"`
	HostRewrite *string                   `json:"hostRewrite,omitempty"`
	Auth        *gatewayAgentUpstreamAuth `json:"auth,omitempty"`
}

type gatewayAgentUpstreamAuth struct {
	Type          string                  `json:"type"`
	Header        *string                 `json:"header,omitempty"`
	Value         *string                 `json:"value,omitempty"`
	PolicyName    *string                 `json:"policyName,omitempty"`
	PolicyParams  *map[string]interface{} `json:"policyParams,omitempty"`
	PolicyVersion *string                 `json:"policyVersion,omitempty"`
}

type gatewayResilience struct {
	Timeout     *string `json:"timeout,omitempty"`
	IdleTimeout *string `json:"idleTimeout,omitempty"`
}

type gatewayA2AConfig struct {
	ProtocolVersion  string                      `json:"protocolVersion"`
	OperationConfigs *gatewayA2AOperationConfigs `json:"operationConfigs,omitempty"`
	AgentCard        *gatewayAgentCard           `json:"agentCard,omitempty"`
}

type gatewayA2AOperationConfigs struct {
	Transports []gatewayA2ATransport  `json:"transports"`
	Policies   *[]gatewayPolicy       `json:"policies,omitempty"`
	Operations *[]gatewayA2AOperation `json:"operations,omitempty"`
}

type gatewayA2ATransport struct {
	ProtocolBinding string  `json:"protocolBinding"`
	PathPrefix      *string `json:"pathPrefix,omitempty"`
}

type gatewayA2AOperation struct {
	Name       string             `json:"name"`
	Policies   *[]gatewayPolicy   `json:"policies,omitempty"`
	Resilience *gatewayResilience `json:"resilience,omitempty"`
}

type gatewayPolicy struct {
	Name               string                  `json:"name"`
	Version            string                  `json:"version"`
	Params             *map[string]interface{} `json:"params,omitempty"`
	ExecutionCondition *string                 `json:"executionCondition,omitempty"`
}

type gatewayAgentCard struct {
	Public    *gatewayPublicAgentCard    `json:"public,omitempty"`
	Protected *gatewayProtectedAgentCard `json:"protected,omitempty"`
}

type gatewayPublicAgentCard struct {
	Mode        *string                 `json:"mode,omitempty"`
	Path        *string                 `json:"path,omitempty"`
	Policies    *[]gatewayPolicy        `json:"policies,omitempty"`
	RewriteUrls *bool                   `json:"rewriteUrls,omitempty"`
	Content     *map[string]interface{} `json:"content,omitempty"`
	Signing     *gatewayCardSigning     `json:"signing,omitempty"`
}

type gatewayProtectedAgentCard struct {
	Mode        string                  `json:"mode"`
	RewriteUrls *bool                   `json:"rewriteUrls,omitempty"`
	Content     *map[string]interface{} `json:"content,omitempty"`
	Signing     *gatewayCardSigning     `json:"signing,omitempty"`
}

type gatewayCardSigning struct {
	Enabled bool `json:"enabled"`
}
