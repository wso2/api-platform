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

package config

import (
	"regexp"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// AgentValidator validates Agent (A2A) configurations.
//
// What it checks today is the identity of the artifact — the fields the
// artifacts row is keyed on and the fields without which nothing downstream can
// run — plus two fail-closed rejections for features the gateway cannot yet
// honour. The structural A2A rules (transport path arithmetic, route collision
// between transports and the card route, the canonical-operation enum, card path
// constraints) are a separate, larger body of work and are not here yet; nothing
// generates Agent routes until they are, so an Agent that passes this validator
// is stored but not yet reachable.
type AgentValidator struct {
	// versionRegex matches the spec.version pattern.
	versionRegex *regexp.Regexp
	// urlFriendlyNameRegex matches URL-safe characters for display names.
	urlFriendlyNameRegex *regexp.Regexp
	// policyValidator validates policies referenced in the Agent configuration.
	// Optional: policy validation for Agent chains lands with the structural
	// rules, so this is currently held and not yet consulted.
	policyValidator *PolicyValidator
}

// NewAgentValidator creates an Agent configuration validator.
func NewAgentValidator() *AgentValidator {
	return &AgentValidator{
		versionRegex:         regexp.MustCompile(`^v\d+\.\d+$`),
		urlFriendlyNameRegex: regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`),
	}
}

// WithPolicyValidator sets the policy validator and returns the receiver for chaining.
func (v *AgentValidator) WithPolicyValidator(pv *PolicyValidator) *AgentValidator {
	v.policyValidator = pv
	return v
}

// Validate performs validation on an Agent configuration. Any other type is
// rejected rather than passed: a validator that silently accepts what it cannot
// inspect is worse than no validator.
func (v *AgentValidator) Validate(config any) []ValidationError {
	switch cfg := config.(type) {
	case *api.AgentConfiguration:
		if cfg == nil {
			return []ValidationError{{Field: "config", Message: "Agent configuration is nil"}}
		}
		return v.validateAgentConfiguration(cfg)
	case api.AgentConfiguration:
		return v.validateAgentConfiguration(&cfg)
	default:
		return []ValidationError{
			{
				Field:   "config",
				Message: "Unsupported configuration type for AgentValidator (expected Agent)",
			},
		}
	}
}

func (v *AgentValidator) validateAgentConfiguration(cfg *api.AgentConfiguration) []ValidationError {
	var errors []ValidationError

	if cfg.Kind != api.AgentConfigurationKindAgent {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported configuration kind (only 'Agent' is supported)",
		})
	}

	errors = append(errors, ValidateMetadata(&cfg.Metadata)...)
	errors = append(errors, v.validateSpec(&cfg.Spec)...)

	return errors
}

func (v *AgentValidator) validateSpec(spec *api.AgentConfigData) []ValidationError {
	var errors []ValidationError

	switch {
	case spec.DisplayName == "":
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "Agent displayName is required",
		})
	case len(spec.DisplayName) > 100:
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "Agent displayName must be 1-100 characters",
		})
	case !v.urlFriendlyNameRegex.MatchString(spec.DisplayName):
		errors = append(errors, ValidationError{
			Field:   "spec.displayName",
			Message: "Agent displayName must be URL-friendly (only letters, numbers, spaces, hyphens, underscores, and dots allowed)",
		})
	}

	switch {
	case spec.Version == "":
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "Version is required",
		})
	case !v.versionRegex.MatchString(spec.Version):
		errors = append(errors, ValidationError{
			Field:   "spec.version",
			Message: "Version must match vMAJOR.MINOR (e.g. v1.0)",
		})
	}

	if spec.Context == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context is required",
		})
	} else {
		errors = append(errors, validateNotReservedHealthPath("spec.context", spec.Context)...)
	}

	// An Agent forwards A2A operation traffic to exactly one upstream, and in
	// passthrough card mode also fetches the Agent Card from it, so a URL is
	// required rather than optional-with-a-ref like the generic upstream shape.
	if spec.Upstream.Url == nil || *spec.Upstream.Url == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream.url",
			Message: "Upstream url is required",
		})
	}

	errors = append(errors, v.validateAgentCard(&spec.A2a.AgentCard)...)

	return errors
}

// validateAgentCard enforces the two card features the gateway cannot honour
// yet. Both are rejected rather than ignored: accepting them would store an
// Agent whose served card does not match what the user asked for, and the
// mismatch is only visible to a client reading the card.
func (v *AgentValidator) validateAgentCard(card *api.A2AAgentCard) []ValidationError {
	var errors []ValidationError

	if card.Public.Signing != nil && card.Public.Signing.Enabled {
		errors = append(errors, ValidationError{
			Field:   "spec.a2a.agentCard.public.signing.enabled",
			Message: "Agent Card signing is not supported yet; set enabled: false or omit the signing block",
		})
	}

	if card.Protected != nil {
		errors = append(errors, ValidationError{
			Field:   "spec.a2a.agentCard.protected",
			Message: "The protected (extended) Agent Card is not supported yet; remove the protected block. GetExtendedAgentCard is proxied to the upstream",
		})
	}

	return errors
}
