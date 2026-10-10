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

package gwversion

// The LTS gateway release at which each capability first appeared. Each
// constant is named after the capability, not the release, so a kind file
// reads as "below the release that added X" and several capabilities may
// legitimately share one release. Verified against the gateway source at tags
// gateway/v1.0.0, gateway/v1.1.0 and gateway/v1.2.0.
const (
	// NoLTSRelease is the minimum of a capability that no LTS release has
	// yet: every gateway reporting an LTS version gets the adaptation (or the
	// refusal). It is not a version and never parses.
	NoLTSRelease = "no-lts-release"

	// MinGatewayV1Version is the first release whose CRD apiVersion is
	// "gateway.api-platform.wso2.com/v1"; older gateways accept only v1alpha1.
	// Every artifact kind flips apiVersion together at this boundary.
	MinGatewayV1Version = "1.2.0"

	// MinSecretSyncGatewayVersion is the first release that pulls secret values
	// from the control plane (controlplane/sync_secrets.go) and so can resolve a
	// {{ secret "handle" }} placeholder itself. 1.1.0 renders the template but
	// resolves only gateway-local secrets; 1.0.0 has no template engine at all.
	MinSecretSyncGatewayVersion = "1.2.0"

	// MinVerbatimMCPUpstreamPathVersion is the first release that forwards
	// "<context>/mcp" to exactly the configured MCP upstream path. Older
	// releases append the "/mcp" operation path to the upstream path.
	MinVerbatimMCPUpstreamPathVersion = "1.2.0"

	// MinLLMUpstreamAuthTypeNoneOtherVersion is the first release whose LLM
	// provider and proxy validators accept upstream auth type "none" and
	// "other" (config/llm_validator.go); older releases accept only "api-key".
	MinLLMUpstreamAuthTypeNoneOtherVersion = "1.2.0"

	// MinMCPUpstreamAuthTypeNoneOtherVersion: no LTS release's MCP validator
	// accepts auth type "none" or "other" without a header and value. 1.2.0
	// lists both values in its MCP schema but still requires header and value
	// for every type, so "none" fails there; every LTS release applies any
	// type like api-key.
	MinMCPUpstreamAuthTypeNoneOtherVersion = NoLTSRelease

	// MinWebBrokerKindGatewayVersion is the first release with the WebBrokerApi
	// artifact kind.
	MinWebBrokerKindGatewayVersion = "1.2.0"

	// MinMCPSpecVersionListGatewayVersion: no LTS release understands the
	// plural spec.specVersions list on an MCP proxy; they know only the
	// singular spec.specVersion.
	MinMCPSpecVersionListGatewayVersion = NoLTSRelease

	// MinAgentKindGatewayVersion: no LTS release has the Agent artifact kind.
	MinAgentKindGatewayVersion = NoLTSRelease
)

// AtLeast reports whether a gateway that reported rawVersion has the
// capability whose first LTS release is min. A blank or non-semver rawVersion is a current build (an
// unregistered gateway, or a dev/e2e tag such as "it-e2e") and satisfies every
// minimum: down-conversion is lossy, so it only applies when a gateway
// positively reports an older release. This is the one rule every predicate
// in the translator shares.
//
// An STS (date-named) release is not compared either: the channels do not
// share a version line, so it is treated as a current build, as before the
// translator existed (https://github.com/wso2/api-platform/issues/3681).
func AtLeast(rawVersion, min string) bool {
	v, ok := Parse(rawVersion)
	if !ok || !v.IsLTS() {
		return true
	}
	if min == NoLTSRelease {
		return false
	}
	return v.AtLeast(ParseVersion(min))
}

// Below is the negation of AtLeast: true only when the gateway positively
// reports a release older than min.
func Below(rawVersion, min string) bool {
	return !AtLeast(rawVersion, min)
}

// SupportsSecretSync reports whether the gateway resolves {{ secret }}
// placeholders itself, from values it pulls off the control plane.
func SupportsSecretSync(rawVersion string) bool {
	return AtLeast(rawVersion, MinSecretSyncGatewayVersion)
}

// RequiresInlineSecrets reports whether the control plane must replace every
// {{ secret }} placeholder with the plaintext value before the gateway sees
// the artifact. It is the delivery-time counterpart of SupportsSecretSync.
func RequiresInlineSecrets(rawVersion string) bool {
	return !SupportsSecretSync(rawVersion)
}

// Gateways names the gateways below min for a warning, e.g. "gateways below
// 1.2.0" or, for a capability no LTS release has, "LTS gateways".
func Gateways(min string) string {
	if min == NoLTSRelease {
		return "LTS gateways"
	}
	return "gateways below " + min
}

// Requirement says what a gateway needs for a capability whose first LTS
// release is min, for an error message.
func Requirement(min string) string {
	if min == NoLTSRelease {
		return "no LTS gateway release supports them yet"
	}
	return "gateway version " + min + " or newer is required"
}
