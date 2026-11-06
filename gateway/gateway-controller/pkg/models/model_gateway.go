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

package models

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// Names that are part of the gateway's contract with operators and with the
// control plane, so they are spelled once, here.
const (
	// DefaultModelEndpointHandle is the handle of the model endpoint the gateway
	// provisions when controller.models.endpoint.provision is set. Reserved: no
	// other endpoint may take it, so a key issued against it keeps working.
	DefaultModelEndpointHandle = "wso2"

	// DefaultModelEndpointDisplayName and DefaultModelEndpointVersion are what the
	// provisioner writes when it creates the default endpoint. An operator may
	// change the display name afterwards; the version is kept.
	DefaultModelEndpointDisplayName = "WSO2 Model Endpoint"
	DefaultModelEndpointVersion     = "v1.0"

	// DefaultModelEndpointInboundTemplate is the inbound template of an endpoint
	// that names none.
	DefaultModelEndpointInboundTemplate = "openai"

	// LLMProviderExposureRoute and LLMProviderExposureNone are the values of
	// LLMProviderConfigData.exposure.
	LLMProviderExposureRoute = "route"
	LLMProviderExposureNone  = "none"
)

// ModelEndpointProviderName is the provider_name a model endpoint's chains carry
// outside any one provider's scope.
//
// Policies that key on provider_name (token-based-ratelimit,
// llm-cost-based-ratelimit) resolve it to a template through the
// ProviderTemplateMapping lazy resource, so the event listener publishes one
// mapping per endpoint, from this name to the endpoint's inbound template. One
// name per endpoint, not per format, so a policy that caches per provider name
// keeps each endpoint's state apart. Underscores keep it out of the metadata.name
// space, so no real provider can collide with it.
func ModelEndpointProviderName(handle string) string {
	return "__wso2_model_endpoint_" + handle + "__"
}

// contributingKinds are artifact kinds that never compile into a runtime config
// of their own. Their content reaches the data plane through another artifact's
// config instead.
//
// A Model is the case: it is served on the model endpoints it names, and the
// policy engine requires every chain a route resolves to — and every upstream it
// names — to sit under that route's own API id (the resolver's partition check,
// and upstream cluster naming). So each endpoint compiles the Models it serves
// into its own config, and a Model on its own has nothing to emit.
var contributingKinds = []string{KindModel}

// IsContributingKind reports whether a kind contributes to another artifact's
// runtime config rather than compiling into one of its own. The Envoy translator
// skips such kinds instead of treating them as kinds it has no transformer for,
// which would send them down the legacy translation path.
func IsContributingKind(kind string) bool {
	return slices.Contains(contributingKinds, kind)
}

// modelEndpointNamespace is the UUIDv5 namespace model endpoint UUIDs are derived
// in. Fixed forever: every replica, and every restart of every replica, must
// derive the same UUID for the same handle, which is what makes provisioning an
// idempotent upsert rather than a race between creators.
var modelEndpointNamespace = uuid.MustParse("6f2b5c7e-0d1a-5b8e-9c43-2a7d1e4f9b60")

// ModelEndpointUUID is the gateway-local UUID of the model endpoint with the
// given handle. Every endpoint's UUID is derived, not only the provisioned one's:
// the UUID is the endpoint's API id on the data plane, which keys its chains and
// its API keys, so replicas must agree on it without coordinating.
func ModelEndpointUUID(handle string) string {
	return uuid.NewSHA1(modelEndpointNamespace, []byte(KindModelEndpoint+"/"+handle)).String()
}

// EffectiveInboundTemplate is an endpoint's inbound template: the one it names,
// or the default.
func EffectiveInboundTemplate(spec api.ModelEndpointConfigData) string {
	if spec.InboundTemplate != nil && strings.TrimSpace(*spec.InboundTemplate) != "" {
		return strings.TrimSpace(*spec.InboundTemplate)
	}
	return DefaultModelEndpointInboundTemplate
}

// LLMProviderExposure returns a provider's effective exposure: `route` unless it
// says `none`. An absent value is `route`, so every provider authored before
// the field existed keeps its route.
func LLMProviderExposure(spec api.LLMProviderConfigData) string {
	if spec.Exposure != nil && string(*spec.Exposure) == LLMProviderExposureNone {
		return LLMProviderExposureNone
	}
	return LLMProviderExposureRoute
}

// IsLLMProviderExposed reports whether a provider publishes a route of its own.
func IsLLMProviderExposed(spec api.LLMProviderConfigData) bool {
	return LLMProviderExposure(spec) == LLMProviderExposureRoute
}

// IsControlPlaneSyncedKind reports whether a gateway-created artifact of this kind
// is pushed to the control plane. Models and model endpoints are not, until the
// control plane can import them: a control plane that cannot would store them as
// something else, or refuse the whole push.
func IsControlPlaneSyncedKind(kind string) bool {
	return kind != KindModel && kind != KindModelEndpoint
}

// ModelEndpointHandles returns the distinct endpoint handles a Model names, in
// the order it names them.
func ModelEndpointHandles(spec api.ModelConfigData) []string {
	seen := make(map[string]bool, len(spec.Endpoints))
	out := make([]string, 0, len(spec.Endpoints))
	for _, h := range spec.Endpoints {
		h = strings.TrimSpace(h)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// ModelNamesEndpoint reports whether a Model names the endpoint.
func ModelNamesEndpoint(spec api.ModelConfigData, handle string) bool {
	return slices.Contains(ModelEndpointHandles(spec), handle)
}

// ModelTargetProviderIDs returns the distinct provider handles a Model's
// targets name, in target order.
func ModelTargetProviderIDs(spec api.ModelConfigData) []string {
	seen := make(map[string]bool, len(spec.Targets))
	ids := make([]string, 0, len(spec.Targets))
	for _, t := range spec.Targets {
		if t.Provider == "" || seen[t.Provider] {
			continue
		}
		seen[t.Provider] = true
		ids = append(ids, t.Provider)
	}
	return ids
}

// ModelTargetAlias returns the name a routing policy uses for a target: its
// alias, or its name when it sets none. It is unique within a valid Model.
func ModelTargetAlias(t api.ModelTarget) string {
	if t.Alias != nil && *t.Alias != "" {
		return *t.Alias
	}
	return t.Name
}
