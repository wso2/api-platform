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

package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/agentproto"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Agent proxy contract validation.
//
// Hand-written and service-layer: resources/openapi.yaml is codegen plus
// documentation, and nothing validates a request against it at runtime, so
// every constraint the schema states has to be restated here or it is not
// enforced at all. The two must agree — a body the schema accepts and this
// rejects (or the reverse) is a contract the publisher cannot rely on — so each
// check below names the schema constraint it mirrors.
//
// What is deliberately *not* here is the gateway's own validation: route
// collisions, reserved paths, policy-definition existence and parameter
// schemas, card-versus-policy security agreement, and the host of a card's
// supportedInterfaces[].url. Those need knowledge that only the gateway has,
// and they surface as a deployment failure, not as a create failure. The one
// exception is the pair of facts an Agent proxy could never deploy under any
// configuration — an unregistered A2A protocol version and an unknown canonical
// operation name — which are rejected here rather than stored permanently
// undeployable.
//
// Checks that need the organization (handle uniqueness, project and gateway
// resolution, secret-handle resolution, the read-only origin guard) stay on the
// service's own call path, where the organization is in hand; everything in
// this file is a function of the request body alone.

// Encoded-size ceiling for one Agent Card document, matching the gateway's
// maxAgentCardBytes. Measured in encoded JSON bytes, which is what crosses the
// wire and what the gateway counts — an object has no meaningful maxLength, so
// the schema documents this limit and the service enforces it.
const agentProxyMaxCardBytes = 1 << 20 // 1 MiB

// Length bounds, each mirroring a minLength/maxLength in the Agent proxy
// schemas. Lengths are counted in characters, as JSON Schema counts them.
const (
	agentProxyDisplayNameMaxLen = 128
	agentProxyDescriptionMaxLen = 1023
	agentProxyProjectIDMinLen   = 3
	agentProxyProjectIDMaxLen   = 63
	agentProxyContextMaxLen     = 200
	agentProxyVhostMaxLen       = 253
	agentProxyPathPrefixMaxLen  = 200
	agentProxyCardPathMinLen    = 2
	agentProxyCardPathMaxLen    = 200
	agentProxyMaxTransports     = 2
)

var (
	// agentProxyDisplayNamePattern mirrors A2AAgentProxy.displayName's pattern.
	agentProxyDisplayNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\-_\. ]+$`)

	// agentProxyVersionPattern mirrors A2AAgentProxy.version's pattern. This is
	// the free-text resource label, unrelated to a2a.protocolVersion.
	agentProxyVersionPattern = regexp.MustCompile(`^v\d+\.\d+$`)

	// agentProxyProjectIDPattern mirrors A2AAgentProxy.projectId's pattern. It
	// checks the shape of the handle only; whether it names a project of this
	// organization is resolved separately.
	agentProxyProjectIDPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

	// agentProxyVhostPattern mirrors A2AAgentProxy.vhost's pattern: an
	// RFC-compliant hostname whose left-most label may be a wildcard.
	agentProxyVhostPattern = regexp.MustCompile(`^(\*\.|[a-zA-Z0-9](?:[a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

	// agentProxyPathPrefixPattern mirrors A2ATransport.pathPrefix's pattern. The
	// bare "/" is permitted and inserts no extra segment.
	agentProxyPathPrefixPattern = regexp.MustCompile(`^/(?:[A-Za-z0-9._~!$&()*+,;=:@%-]+(?:/[A-Za-z0-9._~!$&()*+,;=:@%-]+)*)?$`)

	// agentCardPathPattern mirrors AgentCardPath's pattern: an absolute path
	// reference with at least one segment and no trailing slash.
	agentCardPathPattern = regexp.MustCompile(`^/[A-Za-z0-9._~!$&()*+,;=:@%-]+(?:/[A-Za-z0-9._~!$&()*+,;=:@%-]+)*$`)

	// agentProxyDurationPattern mirrors Resilience.timeout/idleTimeout.
	agentProxyDurationPattern = regexp.MustCompile(`^\d+(\.\d+)?(ms|s|m|h)$`)
)

// requiredManagedAgentCardKeys are the keys an A2A Agent Card must carry for
// the gateway to serve it.
//
// The check is presence only. The card document is free-form by contract and is
// stored byte-for-byte, so validating its interior would both re-serialize it
// and duplicate the gateway's own card validation — but a managed card missing
// one of these is not a card at all, and storing it produces a deployment that
// serves an unusable document.
var requiredManagedAgentCardKeys = []string{
	"name",
	"description",
	"version",
	"supportedInterfaces",
	"capabilities",
	"defaultInputModes",
	"defaultOutputModes",
	"skills",
}

// validateAgentProxyRequest checks a create or full-replacement body against the
// Agent proxy contract.
//
// Create and replace are validated identically: PUT is a full replacement that
// requires the same mandatory authoring fields as create, so a rule that holds
// for one holds for the other. The rules that differ between them are about the
// *stored* resource rather than the body — an immutable protocol, credential
// retention, the read-only origin guard — and live on the service's call path.
func validateAgentProxyRequest(req *api.A2AAgentProxy) error {
	if req == nil {
		return apperror.ValidationFailed.New("A request body is required.")
	}
	if err := validateAgentProxyIdentity(req); err != nil {
		return err
	}
	if err := validateAgentProxyRouting(req); err != nil {
		return err
	}
	if err := validateAgentProxyUpstreamShape(req.Upstream); err != nil {
		return err
	}
	if err := validateAgentProxyResilience(req.Resilience, "resilience"); err != nil {
		return err
	}
	if !model.IsSupportedAgentProxyProtocol(model.AgentProxyProtocol(req.Protocol)) {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The protocol %q is not supported. Supported protocols: %s.",
				string(req.Protocol), strings.Join(model.SupportedAgentProxyProtocols(), ", ")))
	}
	// Phase 1 registers one protocol, so the matching block is always the A2A
	// one. A second protocol adds its own branch here alongside its own typed
	// configuration; it does not widen this one.
	return validateA2AProtocolConfig(&req.A2a)
}

// validateAgentProxyIdentity checks the shared identity and metadata fields.
// Whether projectId names a project of the caller's organization is resolved
// elsewhere; this is the shape of the value.
func validateAgentProxyIdentity(req *api.A2AAgentProxy) error {
	// Presence, not emptiness. An absent id asks the server to derive one from
	// displayName; a *supplied* id is the caller's choice and is held to the
	// handle contract as given. Collapsing "" and "   " into "omitted" would
	// answer a body the schema rejects (minLength 3) with a 201 and a handle the
	// caller never asked for. The value is not trimmed either, since it is
	// stored, addressed and compared against the path exactly as supplied.
	if req.Id != nil {
		if err := utils.ValidateHandle(*req.Id); err != nil {
			return err
		}
	}

	if req.Kind != nil && *req.Kind != api.A2AAgentProxyKindAgentProxy {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The kind field must be %q.", string(api.A2AAgentProxyKindAgentProxy)))
	}

	if strings.TrimSpace(req.DisplayName) == "" {
		return apperror.ValidationFailed.New("The displayName field is required.")
	}
	if utf8.RuneCountInString(req.DisplayName) > agentProxyDisplayNameMaxLen {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The displayName must be at most %d characters.", agentProxyDisplayNameMaxLen))
	}
	if !agentProxyDisplayNamePattern.MatchString(req.DisplayName) {
		return apperror.ValidationFailed.New(
			"The displayName may contain only letters, digits, spaces, hyphens, underscores and dots.")
	}

	if description := utils.ValueOrEmpty(req.Description); utf8.RuneCountInString(description) > agentProxyDescriptionMaxLen {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The description must be at most %d characters.", agentProxyDescriptionMaxLen))
	}

	if strings.TrimSpace(req.Version) == "" {
		return apperror.ValidationFailed.New("The version field is required.")
	}
	if !agentProxyVersionPattern.MatchString(req.Version) {
		return apperror.ValidationFailed.New("The version must look like v1.0.")
	}

	if strings.TrimSpace(req.ProjectId) == "" {
		return apperror.ValidationFailed.New("The projectId field is required.")
	}
	if n := utf8.RuneCountInString(req.ProjectId); n < agentProxyProjectIDMinLen || n > agentProxyProjectIDMaxLen {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The projectId must be between %d and %d characters.",
				agentProxyProjectIDMinLen, agentProxyProjectIDMaxLen))
	}
	if !agentProxyProjectIDPattern.MatchString(req.ProjectId) {
		return apperror.ValidationFailed.New("The projectId must be lowercase alphanumeric with hyphens only.")
	}
	return nil
}

// validateAgentProxyRouting checks the routing fields an Agent proxy shares with
// every other kind.
//
// Both are optional and neither is defaulted here: an omitted context or vhost
// is stored omitted and resolved when the gateway artifact is built, so that a
// default can change without rewriting every stored configuration document.
func validateAgentProxyRouting(req *api.A2AAgentProxy) error {
	if req.Context != nil {
		context := *req.Context
		if context == "" {
			return apperror.ValidationFailed.New("The context must not be empty. Omit it to use the default.")
		}
		if utf8.RuneCountInString(context) > agentProxyContextMaxLen {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The context must be at most %d characters.", agentProxyContextMaxLen))
		}
		if err := utils.ValidateContext(context); err != nil {
			return apperror.ValidationFailed.New("The context must be a valid path (e.g. /weather).")
		}
	}

	if req.Vhost != nil {
		vhost := *req.Vhost
		if vhost == "" {
			return apperror.ValidationFailed.New("The vhost must not be empty. Omit it to use the gateway default.")
		}
		if utf8.RuneCountInString(vhost) > agentProxyVhostMaxLen {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The vhost must be at most %d characters.", agentProxyVhostMaxLen))
		}
		if !agentProxyVhostPattern.MatchString(vhost) {
			return apperror.ValidationFailed.New(
				"The vhost must be a valid hostname; a wildcard is allowed only in the left-most label (e.g. *.example.com).")
		}
		// The published pattern accepts a wildcard in any label — it repeats one
		// alternation for every label — while the contract it documents allows
		// one only in the left-most. The schema is documentation and this is the
		// enforcement point, so the stated rule is applied here rather than left
		// to a regex that cannot express it.
		if strings.Contains(strings.TrimPrefix(vhost, "*."), "*") {
			return apperror.ValidationFailed.New(
				"The vhost may use a wildcard only in its left-most label (e.g. *.example.com).")
		}
	}
	return nil
}

// validateAgentProxyUpstreamShape checks the upstream endpoints.
//
// Only the *shape* is checked: exactly one of url/ref, a syntactically valid
// URL, and a recognised auth type. Whether the credential is present is checked
// against the effective configuration after PUT credential retention — a
// response redacts the stored value, so a round-tripped body legitimately
// arrives without one and that check cannot run this early.
func validateAgentProxyUpstreamShape(upstream api.Upstream) error {
	if err := validateAgentProxyEndpoint(&upstream.Main, "main"); err != nil {
		return err
	}
	if upstream.Sandbox != nil {
		return validateAgentProxyEndpoint(upstream.Sandbox, "sandbox")
	}
	return nil
}

func validateAgentProxyEndpoint(endpoint *api.UpstreamDefinition, name string) error {
	// Exclusivity is decided on key presence, not on whether a value is
	// non-empty. UpstreamDefinition's oneOf branches are `required: [url]` and
	// `required: [ref]`, and JSON Schema's required is about which keys the
	// object carries — so {"url": "...", "ref": ""} matches both branches and
	// satisfies neither's exclusivity. Reading the values instead would let a
	// body the schema rejects through, and store both fields.
	hasURL := endpoint.Url != nil
	hasRef := endpoint.Ref != nil
	switch {
	case !hasURL && !hasRef:
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The upstream %s must specify either a url or a ref.", name))
	case hasURL && hasRef:
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The upstream %s must specify either a url or a ref, not both.", name))
	}

	if hasURL {
		// Validated as supplied, for the same reason as a2a.protocolVersion: this
		// exact string is what gets stored and dialled, so a url that is only
		// valid once trimmed must be rejected rather than quietly accepted and
		// persisted with its whitespace.
		if err := utils.ValidateURL(*endpoint.Url); err != nil {
			// The cause names the syntactic problem and nothing about the
			// deployment, so it is safe to show; the URL itself is the caller's
			// own input and is not echoed back.
			return apperror.ValidationFailed.Wrap(err,
				fmt.Sprintf("The upstream %s url is not a valid URL.", name))
		}
	}
	// The shared UpstreamDefinition schema puts no minLength on ref, so this is
	// the only place an empty one is caught — and a ref that names nothing
	// resolves to no upstream at all.
	if hasRef && strings.TrimSpace(*endpoint.Ref) == "" {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The upstream %s ref must not be empty.", name))
	}

	if endpoint.Auth != nil && endpoint.Auth.Type != nil && !endpoint.Auth.Type.Valid() {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The upstream %s auth type %q is not supported.", name, string(*endpoint.Auth.Type)))
	}
	return nil
}

// validateAgentProxyResilience checks a timeout block. field names the position
// in the request so an error points at the per-operation block rather than the
// Agent-wide one.
func validateAgentProxyResilience(resilience *api.Resilience, field string) error {
	if resilience == nil {
		return nil
	}
	if err := validateAgentProxyDuration(resilience.Timeout, field+".timeout"); err != nil {
		return err
	}
	return validateAgentProxyDuration(resilience.IdleTimeout, field+".idleTimeout")
}

func validateAgentProxyDuration(value *string, field string) error {
	if value == nil {
		return nil
	}
	if !agentProxyDurationPattern.MatchString(*value) {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must be a duration such as 30s, 500ms or 1m.", field))
	}
	return nil
}

// validateA2AProtocolConfig checks the typed a2a block.
func validateA2AProtocolConfig(cfg *api.A2AProtocolConfig) error {
	if cfg == nil {
		return apperror.ValidationFailed.New("The a2a configuration block is required when protocol is \"a2a\".")
	}

	// Checked exactly as supplied, because this is the value that is stored and
	// later handed to the gateway. Trimming here would accept " 1.0 ", which the
	// schema's enum does not, and then persist the original with its whitespace
	// intact — an Agent proxy that validated against one value and deployed
	// against another. A near-miss is a configuration error to report, not a
	// value to normalise.
	version := agentproto.ProtocolVersion(cfg.ProtocolVersion)
	if version == "" {
		return apperror.ValidationFailed.New("The a2a.protocolVersion field is required.")
	}
	// An unregistered version is refused rather than stored: the operation set
	// it names does not exist, so the Agent proxy could never deploy under any
	// gateway configuration.
	if !agentproto.IsSupportedVersion(version) {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The a2a.protocolVersion %q is not supported. Supported versions: %s.",
				string(version), strings.Join(supportedProtocolVersionStrings(), ", ")))
	}

	if err := validateA2AOperationConfigs(&cfg.OperationConfigs, version); err != nil {
		return err
	}
	return validateAgentCardConfig(cfg.AgentCard)
}

// validateA2ATransports checks the transport array under a2a.operationConfigs.
//
// Uniqueness is by protocolBinding, which the schema cannot express: its
// uniqueItems keyword compares whole elements, so two entries for the same
// binding with different path prefixes would pass it while describing two
// conflicting routes for one binding.
func validateA2ATransports(transports []api.A2ATransport) error {
	// An omitted a2a.operationConfigs block decodes to the same empty transport
	// list as an explicitly empty one, so this one message covers both: the
	// block is required precisely because it has to carry the transports.
	if len(transports) == 0 {
		return apperror.ValidationFailed.New("At least one a2a.operationConfigs.transports entry is required.")
	}
	if len(transports) > agentProxyMaxTransports {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("At most %d a2a.operationConfigs.transports entries are allowed.", agentProxyMaxTransports))
	}

	seen := make(map[api.A2ATransportProtocolBinding]struct{}, len(transports))
	for i, transport := range transports {
		if !transport.ProtocolBinding.Valid() {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.transports[%d].protocolBinding %q is not supported. Supported bindings: %s, %s.",
					i, string(transport.ProtocolBinding), string(api.JSONRPC), string(api.HTTPJSON)))
		}
		if _, duplicate := seen[transport.ProtocolBinding]; duplicate {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.transports entries must each use a different protocolBinding; %q appears more than once.",
					string(transport.ProtocolBinding)))
		}
		seen[transport.ProtocolBinding] = struct{}{}

		if transport.PathPrefix == nil {
			continue
		}
		prefix := *transport.PathPrefix
		if prefix == "" {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.transports[%d].pathPrefix must not be empty. Omit it to use the default \"/\".", i))
		}
		if utf8.RuneCountInString(prefix) > agentProxyPathPrefixMaxLen {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.transports[%d].pathPrefix must be at most %d characters.", i, agentProxyPathPrefixMaxLen))
		}
		if !agentProxyPathPrefixPattern.MatchString(prefix) {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.transports[%d].pathPrefix must be an absolute path such as /rpc.", i))
		}
	}
	return nil
}

// validateA2AOperationConfigs checks the required operation configuration block:
// its transports, the Agent-wide policy position and the per-operation additions.
//
// Operation names are checked against the selected protocol version's canonical
// set, not against a version-independent list: an operation belongs to a version
// and an unknown name attaches configuration to a chain that will never exist.
func validateA2AOperationConfigs(configs *api.A2AOperationConfigs, version agentproto.ProtocolVersion) error {
	if configs == nil {
		return apperror.ValidationFailed.New("The a2a.operationConfigs block is required.")
	}
	if err := validateA2ATransports(configs.Transports); err != nil {
		return err
	}
	if err := validateAgentProxyPolicies(configs.Policies, "a2a.operationConfigs.policies"); err != nil {
		return err
	}
	if configs.Operations == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(*configs.Operations))
	for i, operation := range *configs.Operations {
		name := string(operation.Name)
		if !agentproto.IsOperation(version, name) {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.operations[%d].name %q is not an A2A %s operation. Supported operations: %s.",
					i, name, string(version), strings.Join(supportedOperationStrings(version), ", ")))
		}
		if _, duplicate := seen[name]; duplicate {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.operationConfigs.operations entries must each configure a different operation; %q appears more than once.", name))
		}
		seen[name] = struct{}{}

		field := fmt.Sprintf("a2a.operationConfigs.operations[%d]", i)
		if err := validateAgentProxyPolicies(operation.Policies, field+".policies"); err != nil {
			return err
		}
		if err := validateAgentProxyResilience(operation.Resilience, field+".resilience"); err != nil {
			return err
		}
	}
	return nil
}

// validateAgentProxyPolicies checks a policy attachment list. Policy parameters
// are free-form by contract and are not inspected; whether the named policy
// exists on the target gateway is the gateway's to answer at deploy time.
func validateAgentProxyPolicies(policies *[]api.Policy, field string) error {
	if policies == nil {
		return nil
	}
	for i, policy := range *policies {
		if strings.TrimSpace(policy.Name) == "" {
			return apperror.ValidationFailed.New(fmt.Sprintf("The %s[%d].name field is required.", field, i))
		}
	}
	return validatePolicyVersions(policies)
}

// validateAgentCardConfig checks the public and protected Agent Card
// configuration.
//
// An omitted block is valid and means the default — public passthrough at the
// well-known path — so absence is never turned into an explicit configuration
// here. The gateway resolves the default when it builds its routes; storing it
// would make a later change to that default invisible to every Agent proxy
// created before it.
func validateAgentCardConfig(card *api.AgentCardConfig) error {
	if card == nil {
		return nil
	}
	if err := validatePublicAgentCard(card.Public); err != nil {
		return err
	}
	return validateProtectedAgentCard(card.Protected)
}

// validatePublicAgentCard checks the public card's disjoint managed and
// passthrough branches.
//
// An absent mode means passthrough, matching the absent-block default. The mode
// decides the branch — never the presence of content, which would make a
// managed card with forgotten content silently behave as passthrough.
func validatePublicAgentCard(card *api.PublicAgentCard) error {
	if card == nil {
		return nil
	}

	mode := model.AgentCardModePassthrough
	if card.Mode != nil {
		if !card.Mode.Valid() {
			return apperror.ValidationFailed.New(
				fmt.Sprintf("The a2a.agentCard.public.mode %q is not supported. Supported modes: %s, %s.",
					string(*card.Mode), model.AgentCardModeManaged, model.AgentCardModePassthrough))
		}
		mode = string(*card.Mode)
	}

	if card.Path != nil {
		if err := validateAgentCardPath(*card.Path, "a2a.agentCard.public.path"); err != nil {
			return err
		}
	}
	if err := validateAgentProxyPolicies(card.Policies, "a2a.agentCard.public.policies"); err != nil {
		return err
	}

	if mode == model.AgentCardModeManaged {
		if card.RewriteUrls != nil {
			return apperror.ValidationFailed.New(
				"The a2a.agentCard.public.rewriteUrls field applies to passthrough cards only; a managed card is served exactly as stored.")
		}
		return validateManagedAgentCardContent(card.Content, "a2a.agentCard.public.content")
	}

	if card.Content != nil {
		return apperror.ValidationFailed.New(
			"The a2a.agentCard.public.content field is not allowed in passthrough mode; the card is fetched from the upstream agent. Set mode to \"managed\" to serve stored content.")
	}
	return nil
}

// validateProtectedAgentCard checks the protected card.
//
// It carries no path and no policies — those are rejected as unknown properties
// when the request is decoded, because the protected card is an A2A operation
// rather than a discovery route. Its mode is always explicit: unlike the public
// card there is no absent-block default to fall back to, so a present block
// with no mode is a rejection rather than a passthrough.
func validateProtectedAgentCard(card *api.ProtectedAgentCard) error {
	if card == nil {
		return nil
	}
	if card.Mode == nil {
		return apperror.ValidationFailed.New("The a2a.agentCard.protected.mode field is required when a protected card is configured.")
	}
	if !card.Mode.Valid() {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The a2a.agentCard.protected.mode %q is not supported. Supported modes: %s, %s.",
				string(*card.Mode), model.AgentCardModeManaged, model.AgentCardModePassthrough))
	}

	if string(*card.Mode) == model.AgentCardModeManaged {
		if card.RewriteUrls != nil {
			return apperror.ValidationFailed.New(
				"The a2a.agentCard.protected.rewriteUrls field applies to passthrough cards only; a managed card is served exactly as stored.")
		}
		return validateManagedAgentCardContent(card.Content, "a2a.agentCard.protected.content")
	}

	if card.Content != nil {
		return apperror.ValidationFailed.New(
			"The a2a.agentCard.protected.content field is not allowed in passthrough mode; the card is fetched from the upstream agent. Set mode to \"managed\" to serve stored content.")
	}
	return nil
}

// validateAgentCardPath checks the public card's discovery path.
//
// It is an absolute path reference and nothing else: a query, a fragment, a
// template placeholder or a trailing slash each describe something the gateway
// cannot register as a route. The specific causes are named before the general
// pattern check so the error says which one it was.
func validateAgentCardPath(path, field string) error {
	if path == "" {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must not be empty. Omit it to use the default %s.", field, model.DefaultAgentCardPath))
	}
	if !strings.HasPrefix(path, "/") {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must be an absolute path starting with /.", field))
	}
	if strings.ContainsAny(path, "?#") {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must not contain a query string or a fragment.", field))
	}
	if strings.ContainsAny(path, "{}") {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must be a fixed path, not a template.", field))
	}
	if strings.HasSuffix(path, "/") {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must not end with a trailing slash.", field))
	}
	if n := utf8.RuneCountInString(path); n < agentProxyCardPathMinLen || n > agentProxyCardPathMaxLen {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must be between %d and %d characters.", field, agentProxyCardPathMinLen, agentProxyCardPathMaxLen))
	}
	// Anything that survives the checks above and still fails here is an
	// illegal character or an empty segment (a "//" in the middle of the path).
	if !agentCardPathPattern.MatchString(path) {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s must be an absolute path such as %s.", field, model.DefaultAgentCardPath))
	}
	return nil
}

// validateManagedAgentCardContent checks a managed card's stored document:
// present, within the encoded-size ceiling, and carrying the keys an A2A Agent
// Card cannot be read without.
//
// The document is measured, not rewritten. Managed content is stored and served
// byte-for-byte because re-serializing it would change the bytes a future
// signing implementation signs, so nothing here normalizes, reorders or drops a
// key — the encoding below exists only to count bytes.
func validateManagedAgentCardContent(content *api.AgentCardDocument, field string) error {
	if content == nil || *content == nil {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s field is required in managed mode.", field))
	}

	encoded, err := json.Marshal(*content)
	if err != nil {
		return apperror.ValidationFailed.Wrap(err,
			fmt.Sprintf("The %s could not be read as a JSON document.", field))
	}
	if len(encoded) > agentProxyMaxCardBytes {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s exceeds the maximum Agent Card size of %d bytes.", field, agentProxyMaxCardBytes))
	}

	var missing []string
	for _, key := range requiredManagedAgentCardKeys {
		if _, ok := (*content)[key]; !ok {
			missing = append(missing, strconv.Quote(key))
		}
	}
	if len(missing) > 0 {
		return apperror.ValidationFailed.New(
			fmt.Sprintf("The %s is missing required Agent Card fields: %s.", field, strings.Join(missing, ", ")))
	}
	return nil
}

// supportedProtocolVersionStrings renders the registered A2A wire versions for
// an error message, in the registry's own stable order.
func supportedProtocolVersionStrings() []string {
	versions := agentproto.Versions()
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		out = append(out, string(v))
	}
	return out
}

// supportedOperationStrings renders one version's canonical operations for an
// error message, in protocol order.
func supportedOperationStrings(version agentproto.ProtocolVersion) []string {
	operations, ok := agentproto.Operations(version)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(operations))
	for _, op := range operations {
		out = append(out, string(op))
	}
	return out
}
