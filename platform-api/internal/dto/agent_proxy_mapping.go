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

package dto

import (
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Agent proxy request <-> model mapping.
//
// Every conversion between the wire shape and the persisted model lives here, so
// there is one place that decides what is column-backed, what goes into the
// configuration document, and what is redacted on the way out. Callers resolve
// identity (UUID, organization, project UUID, handle, actor) and gateway
// associations; those never come from the request body.

// AgentProxyConfigurationFromRequest maps the authoring fields of a request onto
// the persisted configuration document.
//
// Nothing column-backed is written into the document: no display name,
// description, resource version, ownership, audit, origin, data version — and no
// protocol discriminator, which is read from agent_proxies.protocol on load.
func AgentProxyConfigurationFromRequest(req *api.A2AAgentProxy) model.AgentProxyConfiguration {
	if req == nil {
		return model.AgentProxyConfiguration{}
	}
	return model.AgentProxyConfiguration{
		Context:    clonePtr(req.Context),
		Vhost:      clonePtr(req.Vhost),
		Upstream:   upstreamToModel(req.Upstream),
		Resilience: resilienceToModel(req.Resilience),
		A2A:        a2aConfigToModel(&req.A2a),
	}
}

// AgentProxyToResponse renders a persisted Agent proxy as its public A2A wire
// shape. projectHandle is the caller-resolved project handle — the public
// contract carries handles, never UUIDs. associatedGateways is likewise resolved
// by the caller; pass nil to omit the field.
//
// Upstream credential values are redacted here, not by schema annotation.
func AgentProxyToResponse(m *model.AgentProxy, projectHandle string, associatedGateways *[]api.AssociatedGateway) *api.A2AAgentProxy {
	if m == nil {
		return nil
	}
	kind := api.A2AAgentProxyKindAgentProxy
	handle := m.Handle
	readOnly := m.IsReadOnly()

	resp := &api.A2AAgentProxy{
		Kind:               &kind,
		Id:                 &handle,
		DisplayName:        m.Name,
		Description:        strPtrIfNotEmpty(m.Description),
		Version:            m.Version,
		ProjectId:          projectHandle,
		Context:            clonePtr(m.Configuration.Context),
		Vhost:              clonePtr(m.Configuration.Vhost),
		Upstream:           upstreamToAPIRedacted(&m.Configuration.Upstream),
		Resilience:         resilienceToAPI(m.Configuration.Resilience),
		Protocol:           api.A2AAgentProxyProtocol(m.Protocol),
		AssociatedGateways: associatedGateways,
		CreatedBy:          strPtrIfNotEmpty(m.CreatedBy),
		UpdatedBy:          strPtrIfNotEmpty(m.UpdatedBy),
		ReadOnly:           &readOnly,
	}
	if cfg := a2aConfigToAPI(m.Configuration.A2A); cfg != nil {
		resp.A2a = *cfg
	}
	if !m.CreatedAt.IsZero() {
		createdAt := m.CreatedAt
		resp.CreatedAt = &createdAt
	}
	if !m.UpdatedAt.IsZero() {
		updatedAt := m.UpdatedAt
		resp.UpdatedAt = &updatedAt
	}
	return resp
}

// AgentProxyToListItem renders the lightweight collection projection. Protocol
// comes from the column, as it does on every other read path.
func AgentProxyToListItem(m *model.AgentProxy, projectHandle string) api.AgentProxyListItem {
	if m == nil {
		return api.AgentProxyListItem{}
	}
	readOnly := m.IsReadOnly()
	item := api.AgentProxyListItem{
		Id:          m.Handle,
		DisplayName: m.Name,
		Description: strPtrIfNotEmpty(m.Description),
		Version:     m.Version,
		ProjectId:   projectHandle,
		Protocol:    api.AgentProxyListItemProtocol(m.Protocol),
		Context:     clonePtr(m.Configuration.Context),
		Vhost:       clonePtr(m.Configuration.Vhost),
		CreatedBy:   strPtrIfNotEmpty(m.CreatedBy),
		UpdatedBy:   strPtrIfNotEmpty(m.UpdatedBy),
		ReadOnly:    &readOnly,
	}
	if !m.CreatedAt.IsZero() {
		createdAt := m.CreatedAt
		item.CreatedAt = &createdAt
	}
	if !m.UpdatedAt.IsZero() {
		updatedAt := m.UpdatedAt
		item.UpdatedAt = &updatedAt
	}
	return item
}

// PreserveAgentProxyUpstreamAuth carries an existing upstream credential forward
// when a full-replacement update omits it. Responses redact credential values, so
// a client that reads an Agent proxy and PUTs it back sends an empty value; the
// stored secret must survive that round trip rather than being erased by it.
//
// Inheritance is deliberately narrow: a value is carried forward only when the
// incoming auth block describes the *same configuration* as the stored one.
// Changing any part of it — the type (including to "none", which removes auth) or
// the header the credential is sent in — makes this a new configuration, and a new
// configuration must carry its own credential. Otherwise a caller could retarget a
// stored secret at a different header by editing one field, and the service's
// "incomplete credentials on a changed auth configuration" check would be looking
// at an inherited value rather than the real state.
func PreserveAgentProxyUpstreamAuth(existing, updated *model.UpstreamConfig) *model.UpstreamConfig {
	if updated == nil {
		return existing
	}
	if existing == nil {
		return updated
	}
	preserveEndpointAuth(existing.Main, updated.Main)
	preserveEndpointAuth(existing.Sandbox, updated.Sandbox)
	return updated
}

func preserveEndpointAuth(existing, updated *model.UpstreamEndpoint) {
	if existing == nil || updated == nil {
		return
	}
	if existing.Auth == nil || updated.Auth == nil {
		return
	}
	if !authConfigurationUnchanged(existing.Auth, updated.Auth) {
		return
	}
	if updated.Auth.Value == "" {
		updated.Auth.Value = existing.Auth.Value
	}
}

// authConfigurationUnchanged reports whether an incoming auth block describes the
// same configuration as the stored one. Every field of the block participates
// except the credential value itself, which is the one field a round trip
// legitimately arrives without, because responses redact it and nothing else.
func authConfigurationUnchanged(existing, updated *model.UpstreamAuth) bool {
	return existing.Type == updated.Type && existing.Header == updated.Header
}

// --- request -> model -------------------------------------------------------

func upstreamToModel(in api.Upstream) model.UpstreamConfig {
	out := model.UpstreamConfig{Main: upstreamDefinitionToModel(&in.Main)}
	if in.Sandbox != nil {
		out.Sandbox = upstreamDefinitionToModel(in.Sandbox)
	}
	return out
}

func upstreamDefinitionToModel(in *api.UpstreamDefinition) *model.UpstreamEndpoint {
	if in == nil {
		return nil
	}
	out := &model.UpstreamEndpoint{}
	if in.Url != nil {
		out.URL = *in.Url
	}
	if in.Ref != nil {
		out.Ref = *in.Ref
	}
	if in.Auth != nil {
		auth := &model.UpstreamAuth{}
		if in.Auth.Type != nil {
			auth.Type = string(*in.Auth.Type)
		}
		if in.Auth.Header != nil {
			auth.Header = *in.Auth.Header
		}
		if in.Auth.Value != nil {
			auth.Value = *in.Auth.Value
		}
		out.Auth = auth
	}
	return out
}

func resilienceToModel(in *api.Resilience) *model.Resilience {
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

func a2aConfigToModel(in *api.A2AProtocolConfig) *model.A2AProtocolConfig {
	if in == nil {
		return nil
	}
	return &model.A2AProtocolConfig{
		ProtocolVersion:  string(in.ProtocolVersion),
		OperationConfigs: operationConfigsToModel(&in.OperationConfigs),
		AgentCard:        agentCardToModel(in.AgentCard),
	}
}

// operationConfigsToModel maps the operation configuration block, which carries
// the transports as well as the common and per-operation policy positions.
func operationConfigsToModel(in *api.A2AOperationConfigs) model.A2AOperationConfigs {
	out := model.A2AOperationConfigs{Policies: policiesToModel(in.Policies)}
	if in.Transports != nil {
		out.Transports = make([]model.A2ATransport, 0, len(in.Transports))
		for _, t := range in.Transports {
			out.Transports = append(out.Transports, model.A2ATransport{
				ProtocolBinding: string(t.ProtocolBinding),
				PathPrefix:      clonePtr(t.PathPrefix),
			})
		}
	}
	if in.Operations != nil {
		out.Operations = make([]model.A2AOperation, 0, len(*in.Operations))
		for _, op := range *in.Operations {
			out.Operations = append(out.Operations, model.A2AOperation{
				Name:       string(op.Name),
				Policies:   policiesToModel(op.Policies),
				Resilience: resilienceToModel(op.Resilience),
			})
		}
	}
	return out
}

func agentCardToModel(in *api.AgentCardConfig) *model.AgentCardConfig {
	if in == nil {
		return nil
	}
	out := &model.AgentCardConfig{}
	if in.Public != nil {
		out.Public = &model.PublicAgentCard{
			Mode:     cardModeToModel(in.Public.Mode),
			Path:     clonePtr(in.Public.Path),
			Policies: policiesToModel(in.Public.Policies),
			Content:  cardDocumentToModel(in.Public.Content),
		}
		// rewriteUrls is passthrough-only; a managed card never carries one, and a
		// nil pointer here is what keeps it out of the stored document entirely.
		out.Public.RewriteUrls = clonePtr(in.Public.RewriteUrls)
	}
	if in.Protected != nil {
		out.Protected = &model.ProtectedAgentCard{
			Mode:        derefString(cardModeToModel(in.Protected.Mode)),
			RewriteUrls: clonePtr(in.Protected.RewriteUrls),
			Content:     cardDocumentToModel(in.Protected.Content),
		}
	}
	return out
}

func policiesToModel(in *[]api.Policy) []model.Policy {
	if in == nil {
		return nil
	}
	out := make([]model.Policy, 0, len(*in))
	for _, p := range *in {
		policy := model.Policy{Name: p.Name, Version: p.Version}
		if p.ExecutionCondition != nil {
			cond := *p.ExecutionCondition
			policy.ExecutionCondition = &cond
		}
		if p.Params != nil {
			params := *p.Params
			policy.Params = &params
		}
		out = append(out, policy)
	}
	return out
}

// cardDocumentToModel keeps Agent Card content exactly as supplied. The document
// is free-form by contract, so nothing here inspects, normalizes or drops a key.
func cardDocumentToModel(in *api.AgentCardDocument) model.AgentCardDocument {
	if in == nil || *in == nil {
		return nil
	}
	return model.AgentCardDocument(*in)
}

// --- model -> response ------------------------------------------------------

// upstreamToAPIRedacted renders the stored upstream for a response with every
// credential value removed. The handle of a {{ secret "..." }} placeholder is a
// credential reference and is never echoed either.
func upstreamToAPIRedacted(in *model.UpstreamConfig) api.Upstream {
	out := api.Upstream{}
	if in == nil {
		return out
	}
	if main := upstreamDefinitionToAPIRedacted(in.Main); main != nil {
		out.Main = *main
	}
	out.Sandbox = upstreamDefinitionToAPIRedacted(in.Sandbox)
	return out
}

func upstreamDefinitionToAPIRedacted(in *model.UpstreamEndpoint) *api.UpstreamDefinition {
	if in == nil {
		return nil
	}
	out := &api.UpstreamDefinition{
		Url: strPtrIfNotEmpty(in.URL),
		Ref: strPtrIfNotEmpty(in.Ref),
	}
	if in.Auth != nil {
		auth := &api.UpstreamAuth{
			Header: strPtrIfNotEmpty(in.Auth.Header),
			Value:  nil, // never expose a stored credential or its secret handle
		}
		if in.Auth.Type != "" {
			t := api.UpstreamAuthType(in.Auth.Type)
			auth.Type = &t
		}
		out.Auth = auth
	}
	return out
}

func resilienceToAPI(in *model.Resilience) *api.Resilience {
	if in == nil {
		return nil
	}
	return &api.Resilience{
		Timeout:     strPtrIfNotEmpty(in.Timeout),
		IdleTimeout: strPtrIfNotEmpty(in.IdleTimeout),
	}
}

func a2aConfigToAPI(in *model.A2AProtocolConfig) *api.A2AProtocolConfig {
	if in == nil {
		return nil
	}
	return &api.A2AProtocolConfig{
		ProtocolVersion:  api.A2AProtocolConfigProtocolVersion(in.ProtocolVersion),
		OperationConfigs: operationConfigsToAPI(&in.OperationConfigs),
		AgentCard:        agentCardToAPI(in.AgentCard),
	}
}

// operationConfigsToAPI renders the stored operation configuration. Transports
// are always present on the wire — the contract requires them — so a stored
// document without any renders an empty array rather than a missing key.
func operationConfigsToAPI(in *model.A2AOperationConfigs) api.A2AOperationConfigs {
	out := api.A2AOperationConfigs{
		Transports: make([]api.A2ATransport, 0, len(in.Transports)),
		Policies:   policiesToAPI(in.Policies),
	}
	for _, t := range in.Transports {
		out.Transports = append(out.Transports, api.A2ATransport{
			ProtocolBinding: api.A2ATransportProtocolBinding(t.ProtocolBinding),
			PathPrefix:      clonePtr(t.PathPrefix),
		})
	}
	if in.Operations != nil {
		ops := make([]api.A2AOperation, 0, len(in.Operations))
		for _, op := range in.Operations {
			ops = append(ops, api.A2AOperation{
				Name:       api.A2AOperationName(op.Name),
				Policies:   policiesToAPI(op.Policies),
				Resilience: resilienceToAPI(op.Resilience),
			})
		}
		out.Operations = &ops
	}
	return out
}

func agentCardToAPI(in *model.AgentCardConfig) *api.AgentCardConfig {
	if in == nil {
		return nil
	}
	out := &api.AgentCardConfig{}
	if in.Public != nil {
		out.Public = &api.PublicAgentCard{
			Mode:        publicCardModeToAPI(in.Public.Mode),
			Path:        clonePtr(in.Public.Path),
			Policies:    policiesToAPI(in.Public.Policies),
			RewriteUrls: clonePtr(in.Public.RewriteUrls),
			Content:     cardDocumentToAPI(in.Public.Content),
		}
	}
	if in.Protected != nil {
		out.Protected = &api.ProtectedAgentCard{
			Mode:        protectedCardModeToAPI(in.Protected.Mode),
			RewriteUrls: clonePtr(in.Protected.RewriteUrls),
			Content:     cardDocumentToAPI(in.Protected.Content),
		}
	}
	return out
}

func policiesToAPI(in []model.Policy) *[]api.Policy {
	if in == nil {
		return nil
	}
	out := make([]api.Policy, 0, len(in))
	for _, p := range in {
		policy := api.Policy{Name: p.Name, Version: p.Version}
		if p.ExecutionCondition != nil {
			cond := *p.ExecutionCondition
			policy.ExecutionCondition = &cond
		}
		if p.Params != nil {
			params := *p.Params
			policy.Params = &params
		}
		out = append(out, policy)
	}
	return &out
}

func cardDocumentToAPI(in model.AgentCardDocument) *api.AgentCardDocument {
	if in == nil {
		return nil
	}
	doc := api.AgentCardDocument(in)
	return &doc
}

// --- card mode conversions --------------------------------------------------
//
// The generated public and protected card modes are distinct named string types,
// so these convert through the plain string the model stores. The model keeps a
// plain string because the persisted document must not be coupled to a generated
// type name.

// cardModeToModel converts any generated card-mode value to the model's plain
// string, preserving absence.
func cardModeToModel[T ~string](in *T) *string {
	if in == nil {
		return nil
	}
	v := string(*in)
	return &v
}

func publicCardModeToAPI(in *string) *api.PublicAgentCardMode {
	if in == nil {
		return nil
	}
	mode := api.PublicAgentCardMode(*in)
	return &mode
}

// protectedCardModeToAPI always returns a value: a stored protected block always
// carries an explicit mode, which is what the contract requires whenever the
// block is present.
func protectedCardModeToAPI(in string) *api.ProtectedAgentCardMode {
	mode := api.ProtectedAgentCardMode(in)
	return &mode
}

func derefString(in *string) string {
	if in == nil {
		return ""
	}
	return *in
}

// --- small helpers ----------------------------------------------------------

// clonePtr returns a pointer to a copy of *in, or nil when in is nil, so an
// omitted field stays omitted and callers can never alias the request's memory.
func clonePtr[T any](in *T) *T {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func strPtrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}
