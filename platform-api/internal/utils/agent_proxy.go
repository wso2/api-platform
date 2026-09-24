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

package utils

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

type AgentProxyUtils struct{}

// BuildAgentProxyDeploymentYAML projects a persisted Agent proxy into the gateway's
// kind: Agent deployment artifact, without marshalling it.
//
// The input must be the validated persisted model, never a redacted GET response:
// the upstream credential is carried through as stored (normally a
// {{ secret "handle" }} placeholder the gateway resolves), and a redacted value
// would ship a blank credential.
//
// The builder is selected by the persisted protocol, never inferred from which
// configuration block happens to be present. The control-plane shape is mapped
// field by field rather than forwarded: transports move under operationConfigs,
// and nothing control-plane-only (protocol, ownership, audit, associated
// gateways) reaches the gateway spec. Omitted optional blocks stay omitted, and
// explicit false values (rewriteUrls) are preserved.
func (u *AgentProxyUtils) BuildAgentProxyDeploymentYAML(proxy *model.AgentProxy) (*model.AgentProxyDeploymentYAML, error) {
	if proxy == nil {
		return nil, fmt.Errorf("agent proxy is nil")
	}

	var a2a model.AgentDeploymentA2A
	switch proxy.Protocol {
	case model.AgentProxyProtocolA2A:
		if proxy.Configuration.A2A == nil {
			return nil, fmt.Errorf("agent proxy %q has protocol %q but no a2a configuration", proxy.Handle, proxy.Protocol)
		}
		built, err := buildAgentDeploymentA2A(proxy.Configuration.A2A)
		if err != nil {
			return nil, fmt.Errorf("agent proxy %q: %w", proxy.Handle, err)
		}
		a2a = built
	default:
		return nil, fmt.Errorf("agent proxy %q has unsupported protocol %q", proxy.Handle, proxy.Protocol)
	}

	// Considering upstream main only as the sandbox is not supported in the gateway side currently.
	main := proxy.Configuration.Upstream.Main
	if main == nil {
		return nil, fmt.Errorf("agent proxy %q has no main upstream", proxy.Handle)
	}
	upstream := model.AgentDeploymentUpstream{
		URL: main.URL,
		Ref: main.Ref,
	}
	if main.Auth != nil {
		upstream.Auth = &model.AgentDeploymentUpstreamAuth{
			Type:   main.Auth.Type,
			Header: main.Auth.Header,
			Value:  main.Auth.Value,
		}
	}

	deployment := &model.AgentProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.GatewayKindAgent,
		Metadata: model.DeploymentMetadata{
			Name: proxy.Handle,
		},
		Spec: model.AgentProxyDeploymentSpec{
			DisplayName: proxy.Name,
			Version:     proxy.Version,
			Context:     proxy.Configuration.Context,
			Vhost:       proxy.Configuration.Vhost,
			Upstream:    upstream,
			Resilience:  toAgentDeploymentResilience(proxy.Configuration.Resilience),
			A2A:         a2a,
		},
	}
	if proxy.ProjectUUID != "" {
		deployment.Metadata.Labels = map[string]string{
			"projectId": proxy.ProjectUUID,
		}
	}

	return deployment, nil
}

// GenerateAgentProxyDeploymentYAML creates the deployment YAML string.
func (u *AgentProxyUtils) GenerateAgentProxyDeploymentYAML(proxy *model.AgentProxy) (string, error) {
	d, err := u.BuildAgentProxyDeploymentYAML(proxy)
	if err != nil {
		return "", err
	}
	yamlBytes, err := yaml.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent proxy to YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// buildAgentDeploymentA2A maps the control-plane a2a block to the gateway's
// spec.a2a. The gateway requires operationConfigs.transports, so operationConfigs
// is always emitted — even when the control-plane operationConfigs was omitted.
func buildAgentDeploymentA2A(cfg *model.A2AProtocolConfig) (model.AgentDeploymentA2A, error) {
	if len(cfg.Transports) == 0 {
		return model.AgentDeploymentA2A{}, fmt.Errorf("a2a configuration has no transports")
	}

	transports := make([]model.AgentDeploymentTransport, 0, len(cfg.Transports))
	for _, t := range cfg.Transports {
		transport := model.AgentDeploymentTransport{ProtocolBinding: t.ProtocolBinding}
		if t.PathPrefix != nil {
			transport.PathPrefix = *t.PathPrefix
		}
		transports = append(transports, transport)
	}

	operationConfigs := model.AgentDeploymentOperationConfigs{Transports: transports}
	if cfg.OperationConfigs != nil {
		operationConfigs.Policies = toAgentDeploymentPolicies(cfg.OperationConfigs.Policies)
		if len(cfg.OperationConfigs.Operations) > 0 {
			operations := make([]model.AgentDeploymentOperation, 0, len(cfg.OperationConfigs.Operations))
			for _, op := range cfg.OperationConfigs.Operations {
				operations = append(operations, model.AgentDeploymentOperation{
					Name:       op.Name,
					Policies:   toAgentDeploymentPolicies(op.Policies),
					Resilience: toAgentDeploymentResilience(op.Resilience),
				})
			}
			operationConfigs.Operations = operations
		}
	}

	return model.AgentDeploymentA2A{
		ProtocolVersion:  cfg.ProtocolVersion,
		OperationConfigs: operationConfigs,
		AgentCard:        toAgentDeploymentAgentCard(cfg.AgentCard),
	}, nil
}

// toAgentDeploymentAgentCard maps the card block without resolving defaults: an
// omitted public or protected block, mode or path stays omitted, so the gateway
// applies its own defaults (public passthrough at the well-known path).
func toAgentDeploymentAgentCard(card *model.AgentCardConfig) *model.AgentDeploymentAgentCard {
	if card == nil {
		return nil
	}
	out := &model.AgentDeploymentAgentCard{}
	if pub := card.Public; pub != nil {
		public := &model.AgentDeploymentPublicCard{
			Policies:    toAgentDeploymentPolicies(pub.Policies),
			RewriteUrls: pub.RewriteUrls,
			Content:     pub.Content,
		}
		if pub.Mode != nil {
			public.Mode = *pub.Mode
		}
		if pub.Path != nil {
			public.Path = *pub.Path
		}
		out.Public = public
	}
	if prot := card.Protected; prot != nil {
		out.Protected = &model.AgentDeploymentProtectedCard{
			Mode:        prot.Mode,
			RewriteUrls: prot.RewriteUrls,
			Content:     prot.Content,
		}
	}
	return out
}

func toAgentDeploymentPolicies(policies []model.Policy) []model.AgentDeploymentPolicy {
	if len(policies) == 0 {
		return nil
	}
	out := make([]model.AgentDeploymentPolicy, 0, len(policies))
	for _, p := range policies {
		policy := model.AgentDeploymentPolicy{
			Name:    p.Name,
			Version: p.Version,
		}
		if p.Params != nil {
			policy.Params = *p.Params
		}
		if p.ExecutionCondition != nil {
			policy.ExecutionCondition = *p.ExecutionCondition
		}
		out = append(out, policy)
	}
	return out
}

func toAgentDeploymentResilience(r *model.Resilience) *model.AgentDeploymentResilience {
	if r == nil {
		return nil
	}
	return &model.AgentDeploymentResilience{
		Timeout:     r.Timeout,
		IdleTimeout: r.IdleTimeout,
	}
}
