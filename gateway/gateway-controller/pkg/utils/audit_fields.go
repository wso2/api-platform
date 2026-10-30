/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	auditservice "github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/audit"
)

// ConfigAuditFields is the audit event data for a deployable artifact (REST API,
// agent, LLM provider/proxy, MCP proxy): create records it as-is, update records
// auditservice.AuditDiff of the snapshots taken before and after the change. Only
// these fields are recorded — never upstream URLs, credentials or policy params.
func ConfigAuditFields(cfg *models.StoredConfig) map[string]any {
	fields := map[string]any{
		"displayName":  cfg.DisplayName,
		"version":      cfg.Version,
		"desiredState": string(cfg.DesiredState),
	}
	switch src := cfg.SourceConfiguration.(type) {
	case api.RestAPI:
		policies := appendPolicyNames([]string{}, src.Spec.Policies, "")
		faultPolicies := appendPolicyNames([]string{}, src.Spec.FaultPolicies, "")
		for _, op := range src.Spec.Operations {
			target := op.EffectiveMethod() + " " + op.EffectivePath()
			policies = appendPolicyNames(policies, op.Policies, target)
			faultPolicies = appendPolicyNames(faultPolicies, op.FaultPolicies, target)
		}
		fields["policies"] = policies
		fields["faultPolicies"] = faultPolicies
	case api.LLMProviderConfiguration:
		fields["template"] = src.Spec.Template
		fields["policies"] = llmPolicyNames(src.Spec.GlobalPolicies, src.Spec.OperationPolicies, src.Spec.Policies)
		fields["faultPolicies"] = llmPolicyNames(src.Spec.GlobalFaultPolicies, src.Spec.OperationFaultPolicies, nil)
	case api.LLMProxyConfiguration:
		fields["policies"] = llmPolicyNames(src.Spec.GlobalPolicies, src.Spec.OperationPolicies, src.Spec.Policies)
		fields["faultPolicies"] = llmPolicyNames(src.Spec.GlobalFaultPolicies, src.Spec.OperationFaultPolicies, nil)
	case api.GraphQLAPI:
		fields["policies"] = appendPolicyNames([]string{}, src.Spec.Policies, "")
	case api.MCPProxyConfiguration:
		fields["policies"] = appendPolicyNames([]string{}, src.Spec.Policies, "")
		fields["faultPolicies"] = appendPolicyNames([]string{}, src.Spec.FaultPolicies, "")
	case api.AgentConfiguration:
		a2a := src.Spec.A2a
		policies := appendPolicyNames([]string{}, a2a.OperationConfigs.Policies, "")
		faultPolicies := appendPolicyNames([]string{}, a2a.OperationConfigs.FaultPolicies, "")
		if a2a.OperationConfigs.Operations != nil {
			for _, op := range *a2a.OperationConfigs.Operations {
				policies = appendPolicyNames(policies, op.Policies, string(op.Name))
				faultPolicies = appendPolicyNames(faultPolicies, op.FaultPolicies, string(op.Name))
			}
		}
		if a2a.AgentCard != nil && a2a.AgentCard.Public != nil {
			policies = appendPolicyNames(policies, a2a.AgentCard.Public.Policies, "public agent card")
			faultPolicies = appendPolicyNames(faultPolicies, a2a.AgentCard.Public.FaultPolicies, "public agent card")
		}
		fields["policies"] = policies
		fields["faultPolicies"] = faultPolicies
	}
	return fields
}

// llmPolicyNames lists LLM provider/proxy policies: global ones as "name version",
// operation ones as "name version (METHODS /path)" once per path they are attached to.
// The deprecated policies list is honoured like operationPolicies, so it is listed the same way.
func llmPolicyNames(global *[]api.Policy, operation *[]api.OperationPolicy, deprecated *[]api.LLMPolicy) []string {
	names := appendPolicyNames([]string{}, global, "")
	if operation != nil {
		for _, p := range *operation {
			for _, path := range p.Paths {
				names = append(names, policyName(p.Name, p.Version, pathTarget(path.Methods, path.Path)))
			}
		}
	}
	if deprecated != nil {
		for _, p := range *deprecated {
			for _, path := range p.Paths {
				names = append(names, policyName(p.Name, p.Version, pathTarget(path.Methods, path.Path)))
			}
		}
	}
	return names
}

// appendPolicyNames appends each policy as "name version", scoped to target when it is set.
func appendPolicyNames(names []string, policies *[]api.Policy, target string) []string {
	if policies == nil {
		return names
	}
	for _, p := range *policies {
		names = append(names, policyName(p.Name, p.Version, target))
	}
	return names
}

// policyName formats a policy for audit events: "name version", plus " (target)" when
// it applies to one operation rather than the whole artifact.
func policyName(name, version, target string) string {
	if target = strings.TrimSpace(target); target == "" {
		return name + " " + version
	}
	return name + " " + version + " (" + target + ")"
}

// pathTarget formats an LLM policy path as "METHODS /path".
func pathTarget[M ~string](methods []M, path string) string {
	ms := make([]string, len(methods))
	for i, m := range methods {
		ms[i] = string(m)
	}
	return strings.Join(ms, ",") + " " + path
}

// DeployAuditData is the event data for a create request: the field values for a new
// artifact, or the diff against the configuration it replaced when the request
// overwrote an existing one (previous is nil for a new artifact).
func DeployAuditData(previous, stored *models.StoredConfig) map[string]any {
	if previous != nil {
		return auditservice.AuditDiff(ConfigAuditFields(previous), ConfigAuditFields(stored))
	}
	return ConfigAuditFields(stored)
}
