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

// The gateway release at which each capability first appeared. Each constant
// is named after the capability, not the release, so a kind file reads as
// "below the release that added X" and several capabilities may legitimately
// share one release. Verified against the gateway source at tags
// gateway/v1.0.0, gateway/v1.1.0, gateway/v1.2.0 and gateway/v2026.09.24.
const (
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

	// MinUpstreamAuthTypeNoneOtherVersion is the first release whose upstream
	// auth validators accept type "none" and "other"; older releases accept
	// only "api-key".
	MinUpstreamAuthTypeNoneOtherVersion = "1.2.0"

	// MinWebBrokerKindGatewayVersion is the first release with the WebBrokerApi
	// artifact kind.
	MinWebBrokerKindGatewayVersion = "1.2.0"

	// MinMCPSpecVersionListGatewayVersion is the first release that understands
	// the plural spec.specVersions list on an MCP proxy; older releases know
	// only the singular spec.specVersion.
	MinMCPSpecVersionListGatewayVersion = "2026.09.24"

	// MinAgentKindGatewayVersion is the first release with the Agent artifact
	// kind (gateway-controller commit 5f7cbc443).
	MinAgentKindGatewayVersion = "2026.09.24"
)

// AtLeast reports whether a gateway that reported rawVersion is at least the
// release min. A blank or non-semver rawVersion is a current build (an
// unregistered gateway, or a dev/e2e tag such as "it-e2e") and satisfies every
// minimum: down-conversion is lossy, so it only applies when a gateway
// positively reports an older release. This is the one rule every predicate
// in the translator shares.
func AtLeast(rawVersion, min string) bool {
	v, ok := Parse(rawVersion)
	if !ok {
		return true
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
