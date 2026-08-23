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
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/wso2/api-platform/common/agentproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// DefaultAgentCardPath is the public Agent Card path an Agent serves when it
// does not declare one of its own. It is the location A2A clients probe during
// discovery, so it is a default rather than a requirement: a custom path
// replaces this route instead of adding an alias.
const DefaultAgentCardPath = "/.well-known/agent-card.json"

// agentCardRouteMethod is the HTTP method of the public Agent Card route. Card
// discovery is a plain read, so the card participates in route collision
// detection as a GET and cannot collide with a POST-only operation route.
const agentCardRouteMethod = "GET"

// maxAgentContextLength matches the ceiling the other kinds' validators apply to
// spec.context.
const maxAgentContextLength = 200

// pathParamPattern matches a single {param} placeholder inside one path
// segment. It deliberately does not span "/" or nest: an A2A operation template
// places at most one placeholder per segment, sometimes followed by a literal
// suffix ("/tasks/{id}:cancel").
var pathParamPattern = regexp.MustCompile(`\{[^{}/]*\}`)

// AgentValidator validates Agent (A2A) configurations.
//
// Beyond the identity fields every artifact needs, this validator owns the
// structural half of the A2A contract: the path arithmetic that turns
// spec.context, transport prefixes, and the Agent Card path into concrete
// routes, and the rejection of any two of those routes that would land on the
// same place. Getting that wrong does not produce an error at runtime — it
// produces a route that never matches, or one that quietly shadows another, so
// the only chance to catch it is here.
//
// It also fails closed on two card features the gateway cannot honour yet
// (signing, and the protected/extended card), because storing an Agent whose
// served card differs from what was asked for is a mismatch only a client
// reading the card would ever notice.
//
// For a managed card it additionally enforces the part of the card↔gateway
// contract that concerns routing: the interfaces the card advertises must be
// exactly the transports the gateway exposes, at exactly the paths it serves
// them on. Nothing here rewrites the card — a disagreement is a deployment
// error, because the alternative is a gateway that publishes a discovery
// document pointing somewhere it does not route.
//
// Card *content* is otherwise not inspected. Validating the card document
// against the A2A model for its protocol version, and checking its declared
// security against the policies enforcing it, is separate work.
//
// Validate mutates its argument when a policy validator is attached: policy
// params that arrived as rendered template strings are coerced to their
// schema-declared types before being validated against that schema, and the
// caller persists the coerced values (see AgentService.renderAndValidate).
type AgentValidator struct {
	// versionRegex matches the spec.version pattern.
	versionRegex *regexp.Regexp
	// urlFriendlyNameRegex matches URL-safe characters for display names.
	urlFriendlyNameRegex *regexp.Regexp
	// policyValidator validates (and coerces) the policies referenced by the
	// Agent's three policy scopes. Optional: a validator built without one
	// skips policy checks entirely rather than rejecting every policy as
	// unknown.
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

	if v.policyValidator != nil {
		// Coerce before validating: text/template renders every value to a
		// string, so an integer param supplied as {{ env "LIMIT" }} arrives as
		// "100" and would fail its own schema without this.
		v.policyValidator.CoerceAgentPolicies(cfg)
		errors = append(errors, v.policyValidator.ValidateAgentPolicies(cfg)...)
	}

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

	contextErrors := v.validateContext(spec.Context)
	errors = append(errors, contextErrors...)

	// An Agent forwards A2A operation traffic to exactly one upstream, and in
	// passthrough card mode also fetches the Agent Card from it, so a URL is
	// required rather than optional-with-a-ref like the generic upstream shape.
	if spec.Upstream.Url == nil || *spec.Upstream.Url == "" {
		errors = append(errors, ValidationError{
			Field:   "spec.upstream.url",
			Message: "Upstream url is required",
		})
	}

	errors = append(errors, validateResilienceTimeouts("spec.resilience", spec.Resilience)...)

	// Route arithmetic is only meaningful once the context it hangs off is
	// sound; reporting collisions derived from a malformed context would bury
	// the one error worth fixing under a pile of consequences. An absent
	// context is sound — it resolves to the virtual host root.
	errors = append(errors, v.validateA2A(AgentContextPath(spec.Context), len(contextErrors) == 0, &spec.A2a)...)

	return errors
}

// validateContext applies the same context rules as the other kinds, plus the
// reserved-namespace check. It is spelled out here rather than shared with
// APIValidator.ValidateContext because that one is a method on a validator this
// package would otherwise have to construct just to call it — the same reason
// MCPValidator carries its own copy.
//
// The context is optional. An Agent without one is served at the root of its
// virtual host, which is where an A2A client probes for the Agent Card during
// cold discovery — nesting every route under a context is a choice, not a
// requirement. An explicitly empty string is still rejected: that is a
// malformed value, not an omission.
func (v *AgentValidator) validateContext(context *string) []ValidationError {
	if context == nil {
		return nil
	}
	if *context == "" {
		return []ValidationError{{
			Field:   "spec.context",
			Message: "Context must not be empty; omit the field to serve the agent at the root of its virtual host",
		}}
	}

	var errors []ValidationError
	if !strings.HasPrefix(*context, "/") {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context must start with /",
		})
	}
	if strings.HasSuffix(*context, "/") && *context != "/" {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: "Context cannot end with / (except for root context)",
		})
	}
	if len(*context) > maxAgentContextLength {
		errors = append(errors, ValidationError{
			Field:   "spec.context",
			Message: fmt.Sprintf("Context must be 1-%d characters", maxAgentContextLength),
		})
	}
	errors = append(errors, validateNotReservedHealthPath("spec.context", *context)...)
	return errors
}

// AgentContextPath is the base every A2A route hangs off: the configured
// context, or "" for an Agent served at the root of its virtual host. The two
// cases converge here so no downstream path arithmetic has to know which one it
// is looking at — JoinAgentPath("", "/rpc") is "/rpc", the same shape it
// produces for any other base.
func AgentContextPath(context *string) string {
	if context == nil {
		return ""
	}
	return *context
}

// resolvedTransport is one entry of spec.a2a.operationConfigs.transports after
// its path prefix has been joined onto the Agent's context.
type resolvedTransport struct {
	index    int
	binding  api.A2AProtocolBinding
	basePath string
	// usable reports whether basePath was derived from a well-formed prefix.
	// An unusable transport still occupies its slot for duplicate-binding
	// purposes but contributes no routes.
	usable bool
}

// resolvedCard is the public Agent Card route after path resolution.
type resolvedCard struct {
	path   string
	usable bool
}

func (v *AgentValidator) validateA2A(context string, contextUsable bool, a2a *api.A2AConfig) []ValidationError {
	var errors []ValidationError

	version := agentproto.ProtocolVersion(a2a.ProtocolVersion)
	versionUsable := false
	switch {
	case a2a.ProtocolVersion == "":
		errors = append(errors, ValidationError{
			Field:   "spec.a2a.protocolVersion",
			Message: "A2A protocolVersion is required",
		})
	case !agentproto.IsSupportedVersion(version):
		// Rejected rather than defaulted. An Agent that fell back to a
		// different version would enforce an operation set its own Agent Card
		// does not advertise.
		errors = append(errors, ValidationError{
			Field: "spec.a2a.protocolVersion",
			Message: fmt.Sprintf("Unsupported A2A protocol version '%s' (supported: %s)",
				a2a.ProtocolVersion, supportedProtocolVersions()),
		})
	default:
		versionUsable = true
	}

	transports, transportErrors := v.validateTransports(context, contextUsable, a2a.OperationConfigs.Transports)
	errors = append(errors, transportErrors...)

	errors = append(errors, v.validateOperationConfigs(version, versionUsable, a2a.OperationConfigs.Operations)...)

	card, cardErrors := v.validateAgentCard(context, contextUsable, &a2a.AgentCard)
	errors = append(errors, cardErrors...)

	if contextUsable && versionUsable {
		errors = append(errors, validateAgentRouteCollisions(version, transports, card)...)
	}

	errors = append(errors, v.validateManagedCardConsistency(
		a2a.ProtocolVersion, versionUsable,
		transports, len(transportErrors) == 0,
		&a2a.AgentCard.Public)...)

	return errors
}

// validateTransports checks the declared protocol bindings and resolves each
// one's gateway-facing base path.
func (v *AgentValidator) validateTransports(context string, contextUsable bool, transports []api.A2ATransport) ([]resolvedTransport, []ValidationError) {
	var errors []ValidationError

	if len(transports) == 0 {
		return nil, []ValidationError{{
			Field:   "spec.a2a.operationConfigs.transports",
			Message: "At least one transport is required",
		}}
	}
	if len(transports) > 2 {
		// One entry per protocol binding, and A2A 1.0 defines two.
		errors = append(errors, ValidationError{
			Field:   "spec.a2a.operationConfigs.transports",
			Message: "At most 2 transports are allowed (one per protocol binding)",
		})
	}

	resolved := make([]resolvedTransport, 0, len(transports))
	seenBinding := make(map[api.A2AProtocolBinding]int, len(transports))

	for i, transport := range transports {
		bindingField := fmt.Sprintf("spec.a2a.operationConfigs.transports[%d].protocolBinding", i)
		prefixField := fmt.Sprintf("spec.a2a.operationConfigs.transports[%d].pathPrefix", i)

		bindingOK := true
		switch transport.ProtocolBinding {
		case api.JSONRPC, api.HTTPJSON:
		default:
			bindingOK = false
			errors = append(errors, ValidationError{
				Field: bindingField,
				Message: fmt.Sprintf("Unsupported protocolBinding '%s' (expected '%s' or '%s')",
					transport.ProtocolBinding, api.JSONRPC, api.HTTPJSON),
			})
		}
		if bindingOK {
			if first, dup := seenBinding[transport.ProtocolBinding]; dup {
				errors = append(errors, ValidationError{
					Field: bindingField,
					Message: fmt.Sprintf("Duplicate protocolBinding '%s' (already declared by transports[%d])",
						transport.ProtocolBinding, first),
				})
				bindingOK = false
			} else {
				seenBinding[transport.ProtocolBinding] = i
			}
		}

		prefix := "/"
		prefixOK := true
		if transport.PathPrefix != nil {
			prefix = *transport.PathPrefix
			if prefixErrors := validateAgentPathValue(prefixField, "pathPrefix", prefix); len(prefixErrors) > 0 {
				errors = append(errors, prefixErrors...)
				prefixOK = false
			}
		}

		usable := bindingOK && prefixOK && contextUsable
		basePath := ""
		if prefixOK {
			// Two transports may share a base path. It is not ambiguous: the
			// JSONRPC endpoint is the base path itself and carries its
			// operation in the body, while every HTTP+JSON binding template is
			// non-empty and so lands strictly below it — JSONRPC at "/" with
			// HTTP+JSON at "/" is the idiomatic A2A layout, not a conflict.
			// Anything that genuinely does overlap is caught downstream on the
			// generated routes, which is where an overlap actually exists.
			basePath = JoinAgentPath(context, prefix)
			if contextUsable {
				errors = append(errors, validateNotReservedHealthPath(prefixField, basePath)...)
			}
		}

		resolved = append(resolved, resolvedTransport{
			index:    i,
			binding:  transport.ProtocolBinding,
			basePath: basePath,
			usable:   usable,
		})
	}

	return resolved, errors
}

// validateOperationConfigs checks per-operation overrides. The list is not an
// allowlist — an unlisted operation is still exposed — so the only rules are
// that each entry names a real operation of the selected protocol version, and
// that no operation is configured twice.
func (v *AgentValidator) validateOperationConfigs(version agentproto.ProtocolVersion, versionUsable bool, operations *[]api.A2AOperationConfig) []ValidationError {
	if operations == nil {
		return nil
	}

	var errors []ValidationError
	seen := make(map[api.A2AOperationName]int, len(*operations))

	for i, op := range *operations {
		nameField := fmt.Sprintf("spec.a2a.operationConfigs.operations[%d].name", i)

		switch {
		case op.Name == "":
			errors = append(errors, ValidationError{
				Field:   nameField,
				Message: "Operation name is required",
			})
		case !versionUsable:
			// The name can only be checked against a registered version's
			// operation set; the version error already says why.
		case !agentproto.IsOperation(version, string(op.Name)):
			errors = append(errors, ValidationError{
				Field: nameField,
				Message: fmt.Sprintf("'%s' is not an A2A %s operation (expected one of: %s)",
					op.Name, version, operationNameList(version)),
			})
		}

		if op.Name != "" {
			if first, dup := seen[op.Name]; dup {
				errors = append(errors, ValidationError{
					Field: nameField,
					Message: fmt.Sprintf("Duplicate operation '%s' (already configured by operations[%d])",
						op.Name, first),
				})
			} else {
				seen[op.Name] = i
			}
		}

		errors = append(errors, validateResilienceTimeouts(
			fmt.Sprintf("spec.a2a.operationConfigs.operations[%d].resilience", i), op.Resilience)...)
	}

	return errors
}

// validateAgentCard enforces the mode-specific card rules and resolves the card
// route's path. Two features are rejected outright rather than ignored:
// accepting them would store an Agent whose served card does not match what the
// user asked for.
func (v *AgentValidator) validateAgentCard(context string, contextUsable bool, card *api.A2AAgentCard) (resolvedCard, []ValidationError) {
	var errors []ValidationError

	if card.Protected != nil {
		errors = append(errors, ValidationError{
			Field:   "spec.a2a.agentCard.protected",
			Message: "The protected (extended) Agent Card is not supported yet; remove the protected block. GetExtendedAgentCard is proxied to the upstream",
		})
	}

	public := &card.Public
	modeOK := true
	switch public.Mode {
	case api.A2APublicAgentCardModeManaged:
		if public.Content == nil || len(*public.Content) == 0 {
			errors = append(errors, ValidationError{
				Field:   "spec.a2a.agentCard.public.content",
				Message: "A managed public Agent Card requires content",
			})
		}
	case api.A2APublicAgentCardModePassthrough:
		// The gateway neither parses nor rewrites a proxied card, so anything
		// that would require it to produce one is a contradiction, not a
		// harmless extra.
		if public.Content != nil {
			errors = append(errors, ValidationError{
				Field:   "spec.a2a.agentCard.public.content",
				Message: "A passthrough public Agent Card is served from the upstream; remove content or set mode: managed",
			})
		}
		if public.Signing != nil {
			errors = append(errors, ValidationError{
				Field:   "spec.a2a.agentCard.public.signing",
				Message: "A passthrough public Agent Card cannot be signed by the gateway; remove signing or set mode: managed",
			})
		}
	default:
		modeOK = false
		errors = append(errors, ValidationError{
			Field: "spec.a2a.agentCard.public.mode",
			Message: fmt.Sprintf("Unsupported Agent Card mode '%s' (expected '%s' or '%s')",
				public.Mode, api.A2APublicAgentCardModeManaged, api.A2APublicAgentCardModePassthrough),
		})
	}

	errors = append(errors, validateCardSigning(public.Signing)...)

	cardPath := DefaultAgentCardPath
	pathOK := true
	if public.Path != nil {
		cardPath = *public.Path
		if pathErrors := validateAgentPathValue("spec.a2a.agentCard.public.path", "Agent Card path", cardPath); len(pathErrors) > 0 {
			errors = append(errors, pathErrors...)
			pathOK = false
		}
	}

	resolved := resolvedCard{usable: modeOK && pathOK && contextUsable}
	if pathOK {
		resolved.path = JoinAgentPath(context, cardPath)
		if contextUsable {
			errors = append(errors, validateNotReservedHealthPath("spec.a2a.agentCard.public.path", resolved.path)...)
		}
	}

	return resolved, errors
}

// validateCardSigning fails closed on signing, which no release has implemented
// yet.
//
// `enabled` is the whole of the Agent-facing contract. There is deliberately no
// algorithm, `kid`, or profile selector here: those are administrator-owned and
// resolved from gateway TOML at signing time, so an operator can rotate the
// active key — including to a key using a different algorithm — without editing
// any Agent. Rotation still requires each Agent to be redeployed before its
// stored card carries a signature from the new key, because signatures are
// recomputed only on deploy; until then the card keeps verifying against the
// retired key, which stays published in the JWKS while any stored card
// references it. A per-Agent algorithm could only ever have restated the
// operator's choice or contradicted it.
func validateCardSigning(signing *api.A2ACardSigning) []ValidationError {
	if signing == nil || !signing.Enabled {
		return nil
	}

	return []ValidationError{{
		Field:   "spec.a2a.agentCard.public.signing.enabled",
		Message: "Agent Card signing is not supported yet; set enabled: false or omit the signing block",
	}}
}

// Agent Card field names. The card is carried as a free-form document rather
// than a typed struct, so every field this validator reads is named here once
// instead of being spelled as a literal at each use. They are the protobuf JSON
// names from the vendored A2A definition (message AgentCard / AgentInterface),
// which is what an A2A client reads and therefore what an author copies.
const (
	cardFieldSupportedInterfaces = "supportedInterfaces"
	cardFieldSignatures          = "signatures"
	cardInterfaceFieldURL        = "url"
	cardInterfaceFieldBinding    = "protocolBinding"
	cardInterfaceFieldVersion    = "protocolVersion"
	cardInterfaceFieldTenant     = "tenant"
)

// cardContentField is the configuration path of the managed card document, the
// prefix every card-content error is reported under.
const cardContentField = "spec.a2a.agentCard.public.content"

// maxAgentCardBytes is the ceiling on a managed Agent Card's JSON encoding.
//
// 1 MiB is the largest single object Kubernetes stores by default — the
// documented ConfigMap/Secret data limit, which falls out of etcd's default
// 1.5 MiB max request size. An Agent carrying a bigger card than this could not
// be applied as a custom resource in the first place, so capping here puts the
// gateway's rejection at the same boundary the platform already enforces
// instead of inventing a tighter one of its own. It is not a tuning knob: an
// operator lowering it would only move the failure earlier, and raising it
// would let through an artifact the cluster cannot hold.
//
// This bounds one card. It does not bound a node's policy-xDS snapshot, which
// is state-of-the-world — one message carries every policy chain for the node —
// and which neither pkg/policyxds/server.go nor the engine's xDS client
// currently limits explicitly, so the gRPC-Go default of 4 MiB on the receiving
// side applies. Four maximal cards would exceed it. Setting
// MaxRecvMsgSize/MaxSendMsgSize on both sides is the fix for that, and it is a
// separate, pre-existing gap; a per-card cap cannot substitute for it.
const maxAgentCardBytes = 1024 * 1024

// validateManagedCardConsistency checks a managed Agent Card against the
// gateway configuration that will serve it.
//
// Only the parts of the card the gateway can contradict are checked here: the
// interfaces it advertises, whose bindings and URLs must be the ones the
// gateway actually routes, and the two fields the gateway itself owns — the
// signature block and the document's size on the wire. Everything else in the
// card is the author's to write.
//
// A passthrough card is opaque to the gateway — it is fetched from the upstream
// and proxied unparsed — so none of this can be checked for one. That gap is
// real and has to stay visible in deployment status rather than being papered
// over here.
func (v *AgentValidator) validateManagedCardConsistency(
	protocolVersion api.A2AConfigProtocolVersion, versionUsable bool,
	transports []resolvedTransport, transportsUsable bool,
	public *api.A2APublicAgentCard,
) []ValidationError {
	if public.Mode != api.A2APublicAgentCardModeManaged || public.Content == nil {
		return nil
	}
	content := map[string]interface{}(*public.Content)
	if len(content) == 0 {
		// The empty-content rejection belongs to the mode check, which has
		// already reported it.
		return nil
	}

	var errors []ValidationError
	errors = append(errors, validateCardSize(content)...)
	errors = append(errors, validateCardNotPreSigned(content)...)
	errors = append(errors, validateCardInterfaces(protocolVersion, versionUsable, transports, transportsUsable, content)...)
	return errors
}

// validateCardSize caps the serialized card at maxAgentCardBytes.
//
// A card past the ceiling is rejected at deploy time so the failure names the
// artifact that caused it. Everything downstream of here — the stored row, the
// custom resource, the policy-xDS snapshot the card rides in — fails on size in
// a way that reports the container rather than the content, and in the
// snapshot's case takes every other artifact on the node with it.
//
// Size is measured over the JSON encoding because that is the form that travels;
// the document is still stored and served exactly as supplied.
func validateCardSize(content map[string]interface{}) []ValidationError {
	encoded, err := json.Marshal(content)
	if err != nil {
		return []ValidationError{{
			Field:   cardContentField,
			Message: "Agent Card content cannot be encoded as JSON",
		}}
	}
	if len(encoded) > maxAgentCardBytes {
		return []ValidationError{{
			Field: cardContentField,
			Message: fmt.Sprintf("Agent Card content is %d bytes, which exceeds the maximum of %d bytes",
				len(encoded), maxAgentCardBytes),
		}}
	}
	return nil
}

// validateCardNotPreSigned rejects a card that arrives carrying a signatures
// field.
//
// The field is the gateway's to write: it signs the card it serves, over the
// content the author supplied. A signature already in the document was computed
// by someone else over a different document — at best it is stale the moment
// the gateway stores it, and a client that verifies it would reject the card.
// Rejecting the field's presence, rather than only a non-empty value, keeps the
// contract in one direction: the author writes card content, the gateway writes
// signatures.
func validateCardNotPreSigned(content map[string]interface{}) []ValidationError {
	if _, present := content[cardFieldSignatures]; !present {
		return nil
	}
	return []ValidationError{{
		Field: cardContentField + "." + cardFieldSignatures,
		Message: "Agent Card content must not carry signatures; the gateway computes them for the card it serves. " +
			"Remove the field",
	}}
}

// validateCardInterfaces checks the card's advertised interfaces against the
// transports the gateway exposes.
//
// The card is what a client reads to decide where to send an A2A request, and
// the gateway routes what its transports say — so a disagreement between the
// two is not a cosmetic inconsistency, it is a client sending requests to a path
// that 404s, or using a binding nothing is listening for. The gateway will not
// rewrite the card to agree with itself: silently correcting an author's
// discovery document would hide the mistake and, once signing lands, change the
// bytes under the signature.
func validateCardInterfaces(
	protocolVersion api.A2AConfigProtocolVersion, versionUsable bool,
	transports []resolvedTransport, transportsUsable bool,
	content map[string]interface{},
) []ValidationError {
	interfacesField := cardContentField + "." + cardFieldSupportedInterfaces

	raw, present := content[cardFieldSupportedInterfaces]
	if !present {
		return []ValidationError{{
			Field:   interfacesField,
			Message: "A managed Agent Card must advertise supportedInterfaces for the configured transports",
		}}
	}
	entries, ok := cardArray(raw)
	if !ok {
		return []ValidationError{{
			Field:   interfacesField,
			Message: "Agent Card supportedInterfaces must be a list of interfaces",
		}}
	}
	if len(entries) == 0 {
		return []ValidationError{{
			Field:   interfacesField,
			Message: "Agent Card supportedInterfaces must not be empty",
		}}
	}

	// The transports are what every per-interface check compares against, so
	// nothing below can run against a transport list that did not resolve —
	// the errors would describe consequences of an error already reported.
	usableTransports := transportsUsable
	for _, transport := range transports {
		if !transport.usable {
			usableTransports = false
		}
	}

	var errors []ValidationError
	// Keyed by binding, which is unique across a transport list that resolved —
	// a duplicated binding makes its transport unusable, and usableTransports is
	// then false, so nothing reads this map.
	basePaths := make(map[api.A2AProtocolBinding]string, len(transports))
	for _, transport := range transports {
		basePaths[transport.binding] = transport.basePath
	}

	advertised := make(map[api.A2AProtocolBinding]int, len(entries))
	for i, entry := range entries {
		field := fmt.Sprintf("%s[%d]", interfacesField, i)

		iface, ok := cardObject(entry)
		if !ok {
			errors = append(errors, ValidationError{
				Field:   field,
				Message: "Agent Card supportedInterfaces entry must be an object",
			})
			continue
		}

		errors = append(errors, validateCardInterfaceVersion(field, protocolVersion, versionUsable, iface)...)
		errors = append(errors, validateCardInterfaceTenant(field, iface)...)

		binding, bindingOK, bindingErrors := validateCardInterfaceBinding(field, usableTransports, basePaths, iface)
		errors = append(errors, bindingErrors...)
		if bindingOK {
			if first, dup := advertised[binding]; dup {
				errors = append(errors, ValidationError{
					Field: field + "." + cardInterfaceFieldBinding,
					Message: fmt.Sprintf("Duplicate protocolBinding '%s' (already advertised by supportedInterfaces[%d]); "+
						"the gateway serves one path per binding", binding, first),
				})
			} else {
				advertised[binding] = i
			}
		}

		expectedPath, pathKnown := "", false
		if bindingOK && usableTransports {
			expectedPath, pathKnown = basePaths[binding]
		}
		errors = append(errors, validateCardInterfaceURL(field, expectedPath, pathKnown, iface)...)
	}

	// The match has to hold in both directions. An unadvertised transport is
	// the quieter half: the route exists and works, but no client discovers it,
	// so the transport looks broken rather than undeclared.
	if usableTransports {
		for _, transport := range transports {
			if _, ok := advertised[transport.binding]; ok {
				continue
			}
			errors = append(errors, ValidationError{
				Field: interfacesField,
				Message: fmt.Sprintf("No Agent Card interface advertises protocolBinding '%s', which is exposed by "+
					"spec.a2a.operationConfigs.transports[%d]", transport.binding, transport.index),
			})
		}
	}

	return errors
}

// validateCardInterfaceBinding resolves one interface's protocolBinding.
//
// It returns the binding alongside whether it names a configured transport, so
// the caller can go on to the duplicate and path checks that only make sense
// once the interface is known to correspond to something the gateway serves.
func validateCardInterfaceBinding(
	field string, usableTransports bool,
	basePaths map[api.A2AProtocolBinding]string,
	iface map[string]interface{},
) (api.A2AProtocolBinding, bool, []ValidationError) {
	bindingField := field + "." + cardInterfaceFieldBinding

	raw, present := iface[cardInterfaceFieldBinding]
	if !present {
		return "", false, []ValidationError{{
			Field:   bindingField,
			Message: "Agent Card interface must declare protocolBinding",
		}}
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", false, []ValidationError{{
			Field:   bindingField,
			Message: "Agent Card interface protocolBinding must be a non-empty string",
		}}
	}

	binding := api.A2AProtocolBinding(value)
	if usableTransports {
		if _, configured := basePaths[binding]; !configured {
			return binding, false, []ValidationError{{
				Field: bindingField,
				Message: fmt.Sprintf("Agent Card advertises protocolBinding '%s', which is not exposed by "+
					"spec.a2a.operationConfigs.transports", binding),
			}}
		}
	}
	return binding, true, nil
}

// validateCardInterfaceVersion requires each interface to advertise the Agent's
// own protocol version. An Agent exposes exactly one version and converts
// nothing, so an interface claiming another one tells clients to speak a
// protocol the gateway will route but the operation set does not define.
func validateCardInterfaceVersion(
	field string, protocolVersion api.A2AConfigProtocolVersion, versionUsable bool,
	iface map[string]interface{},
) []ValidationError {
	versionField := field + "." + cardInterfaceFieldVersion

	raw, present := iface[cardInterfaceFieldVersion]
	if !present {
		return []ValidationError{{
			Field:   versionField,
			Message: "Agent Card interface must declare protocolVersion",
		}}
	}
	value, ok := raw.(string)
	if !ok {
		return []ValidationError{{
			Field:   versionField,
			Message: "Agent Card interface protocolVersion must be a string",
		}}
	}
	if !versionUsable {
		// spec.a2a.protocolVersion is missing or unsupported and has been
		// reported as such; comparing against it would add an error whose fix
		// is to correct the card, when the card may well be the correct half.
		return nil
	}
	if value != string(protocolVersion) {
		return []ValidationError{{
			Field: versionField,
			Message: fmt.Sprintf("Agent Card interface advertises A2A protocol version '%s' but the agent exposes '%s'",
				value, protocolVersion),
		}}
	}
	return nil
}

// validateCardInterfaceTenant rejects the tenant routing hint.
//
// A tenant tells clients to include an opaque routing identifier so a server
// can demultiplex several agents behind one endpoint. The gateway routes an
// Agent by its own context and transport paths and never reads that field, so
// advertising one would have clients decorating every request with a value
// nothing acts on — and, where an upstream does act on it, would hand routing
// authority to a document the gateway publishes but does not enforce.
//
// An explicitly empty tenant is accepted: it is the protobuf default and means
// the same thing as omitting the field.
func validateCardInterfaceTenant(field string, iface map[string]interface{}) []ValidationError {
	raw, present := iface[cardInterfaceFieldTenant]
	if !present {
		return nil
	}
	if value, ok := raw.(string); ok && value == "" {
		return nil
	}
	return []ValidationError{{
		Field: field + "." + cardInterfaceFieldTenant,
		Message: "Agent Card interface must not declare tenant; the gateway does not serve tenant-scoped A2A routes, " +
			"so clients would send the value to an endpoint that ignores it",
	}}
}

// validateCardInterfaceURL checks the URL clients will actually dial.
//
// The path is compared against the transport's effective gateway base path,
// which is the whole point of the check: a card advertising the wrong path
// sends every client to a route that does not exist, and no request ever
// reaches the gateway to fail in a way anyone can see.
//
// The host is deliberately *not* compared. There is nothing to compare it
// against yet — the gateway has no configured public base URL — so a card
// naming the wrong host passes here. That is a known gap, closed when that
// configuration exists; it is not a silent acceptance of an arbitrary host, it
// is a check that cannot be written yet.
func validateCardInterfaceURL(field, expectedPath string, pathKnown bool, iface map[string]interface{}) []ValidationError {
	urlField := field + "." + cardInterfaceFieldURL

	raw, present := iface[cardInterfaceFieldURL]
	if !present {
		return []ValidationError{{
			Field:   urlField,
			Message: "Agent Card interface must declare url",
		}}
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return []ValidationError{{
			Field:   urlField,
			Message: "Agent Card interface url must be a non-empty string",
		}}
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.Scheme == "" || parsed.Host == "" {
		return []ValidationError{{
			Field:   urlField,
			Message: "Agent Card interface url must be an absolute URL with a scheme and host",
		}}
	}

	var errors []ValidationError
	if parsed.Scheme != "https" {
		errors = append(errors, ValidationError{
			Field:   urlField,
			Message: fmt.Sprintf("Agent Card interface url must use https, got '%s'", parsed.Scheme),
		})
	}
	if parsed.User != nil {
		// Userinfo in a published discovery document is a credential clients
		// would copy into every request; it is also the classic way to make a
		// URL read as one host while resolving to another.
		errors = append(errors, ValidationError{
			Field:   urlField,
			Message: "Agent Card interface url must not contain userinfo",
		})
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		errors = append(errors, ValidationError{
			Field:   urlField,
			Message: "Agent Card interface url must not contain a query string",
		})
	}
	if parsed.Fragment != "" {
		errors = append(errors, ValidationError{
			Field:   urlField,
			Message: "Agent Card interface url must not contain a fragment",
		})
	}

	if pathKnown {
		// An empty path and "/" are the same origin-root location; the route
		// arithmetic produces the latter, so a card written the former way is
		// advertising the right place.
		actualPath := parsed.Path
		if actualPath == "" {
			actualPath = "/"
		}
		if actualPath != expectedPath {
			errors = append(errors, ValidationError{
				Field: urlField,
				Message: fmt.Sprintf("Agent Card interface url path is '%s' but the gateway serves this transport at '%s'",
					actualPath, expectedPath),
			})
		}
	}

	return errors
}

// cardObject reads a nested object out of an Agent Card.
//
// A card is a free-form document, and its nested mappings come back typed by
// whichever decoder produced them: yaml.v3 reuses the enclosing named map type
// for nested mappings, while encoding/json produces a plain map. The two ingress
// paths are both real — YAML from the management API, JSON from storage — so a
// walker that accepted only one shape would validate on one path and skip the
// checks entirely on the other.
func cardObject(value interface{}) (map[string]interface{}, bool) {
	switch typed := value.(type) {
	case api.A2AAgentCardDocument:
		return typed, true
	case map[string]interface{}:
		return typed, true
	default:
		return nil, false
	}
}

// cardArray reads a nested list out of an Agent Card. Both decoders produce
// []interface{} for a sequence, but this exists alongside cardObject so the two
// reads look the same at the call site.
func cardArray(value interface{}) ([]interface{}, bool) {
	typed, ok := value.([]interface{})
	return typed, ok
}

// agentRoute is one gateway-facing route an Agent generates.
type agentRoute struct {
	method string
	// path is relative to the Agent's vhost and may carry {param} placeholders
	// in the segments an A2A operation template defines.
	path string
	// field is the configuration field that produced this route, so a collision
	// can be reported against something the author can edit.
	field string
	// source names the route in prose, for the other half of that message.
	source string
}

// routeKey is the identity two routes collide on.
//
// Method and path, and nothing else. Header matches are deliberately absent:
// the policy-chain partition key downstream is (artifact, vhost, operation), so
// two routes distinguished only by a header would resolve to one chain and one
// of them would silently run the other's policies. Adding a discriminator here
// without adding it to that key is how that bug gets introduced, so this stays
// the whole of route identity.
func (r agentRoute) routeKey() string {
	return r.method + " " + r.path
}

// validateAgentRouteCollisions builds every route the Agent will generate and
// rejects any two that cannot coexist.
//
// This is the check with no runtime counterpart. A collision does not fail a
// request: Envoy picks one route, and the other operation is simply never
// reachable, with no error anywhere to explain it.
func validateAgentRouteCollisions(version agentproto.ProtocolVersion, transports []resolvedTransport, card resolvedCard) []ValidationError {
	routes := buildAgentRoutes(version, transports, card)

	var errors []ValidationError
	seen := make(map[string]agentRoute, len(routes))
	for _, route := range routes {
		key := route.routeKey()
		if previous, dup := seen[key]; dup {
			errors = append(errors, ValidationError{
				Field: route.field,
				Message: fmt.Sprintf("Route collision: %s %s is generated by both %s and %s",
					route.method, route.path, previous.source, route.source),
			})
			continue
		}
		seen[key] = route
	}

	// A literal path that a templated operation route already matches is
	// unreachable for the same reason a duplicate is, but it does not show up
	// as an equal key: "/tasks/agent-card.json" is not the string
	// "/tasks/{id}", yet Envoy hands the request to whichever route it matched
	// first.
	for _, literal := range routes {
		if strings.Contains(literal.path, "{") {
			continue
		}
		for _, template := range routes {
			if !strings.Contains(template.path, "{") || template.method != literal.method {
				continue
			}
			if templateMatchesPath(template.path, literal.path) {
				errors = append(errors, ValidationError{
					Field: literal.field,
					Message: fmt.Sprintf("Route collision: %s %s (%s) is already matched by %s %s (%s)",
						literal.method, literal.path, literal.source,
						template.method, template.path, template.source),
				})
			}
		}
	}

	return errors
}

// buildAgentRoutes enumerates the Agent's routes in a deterministic order:
// transports in declaration order, each HTTP+JSON transport's operations in
// protocol order, and the Agent Card last so a collision involving it is
// reported against the card path — the field an author can actually change.
func buildAgentRoutes(version agentproto.ProtocolVersion, transports []resolvedTransport, card resolvedCard) []agentRoute {
	var routes []agentRoute

	for _, transport := range transports {
		if !transport.usable {
			continue
		}
		field := fmt.Sprintf("spec.a2a.operationConfigs.transports[%d].pathPrefix", transport.index)

		switch transport.binding {
		case api.JSONRPC:
			// One endpoint for every operation: the method is carried in the
			// JSON-RPC envelope, not the path.
			routes = append(routes, agentRoute{
				method: "POST",
				path:   transport.basePath,
				field:  field,
				source: fmt.Sprintf("the %s transport endpoint", api.JSONRPC),
			})
		case api.HTTPJSON:
			operations, ok := agentproto.Operations(version)
			if !ok {
				continue
			}
			for _, operation := range operations {
				bindings, ok := agentproto.HTTPJSONBindings(version, operation)
				if !ok {
					continue
				}
				for _, binding := range bindings {
					routes = append(routes, agentRoute{
						method: binding.Method,
						path:   JoinAgentPath(transport.basePath, binding.PathTemplate),
						field:  field,
						source: fmt.Sprintf("the %s route for %s", api.HTTPJSON, operation),
					})
				}
			}
		}
	}

	if card.usable {
		routes = append(routes, agentRoute{
			method: agentCardRouteMethod,
			path:   card.path,
			field:  "spec.a2a.agentCard.public.path",
			source: "the public Agent Card route",
		})
	}

	return routes
}

// JoinAgentPath joins a base path and a relative segment with exactly one
// separator. A segment of "" or "/" contributes no extra path segment, which is
// what a transport pathPrefix of "/" means: serve at the context itself.
//
// Exported because the Agent transformer composes the very same paths when it
// generates routes. The collision check above is only meaningful if the routes
// it reasons about are byte-identical to the ones that ship, so both sides do
// the arithmetic with this one function rather than each spelling it out.
func JoinAgentPath(base, segment string) string {
	trimmedBase := strings.TrimSuffix(base, "/")
	trimmedSegment := strings.Trim(segment, "/")
	if trimmedSegment == "" {
		if trimmedBase == "" {
			return "/"
		}
		return trimmedBase
	}
	return trimmedBase + "/" + trimmedSegment
}

// templateMatchesPath reports whether a route template would match a concrete
// path. Only ever called with a literal second argument: the templated routes
// all come from the protocol's own binding table, which is internally
// consistent by construction, so template-against-template is not a case that
// can arise from configuration.
func templateMatchesPath(template, literal string) bool {
	templateSegments := strings.Split(template, "/")
	literalSegments := strings.Split(literal, "/")
	if len(templateSegments) != len(literalSegments) {
		return false
	}
	for i := range templateSegments {
		if !segmentMatchesPath(templateSegments[i], literalSegments[i]) {
			return false
		}
	}
	return true
}

// segmentMatchesPath matches one template segment against one literal segment.
// A placeholder stands for a non-empty run of anything but "/", so
// "{id}:cancel" matches "abc:cancel" but not "abc:subscribe".
func segmentMatchesPath(templateSegment, literalSegment string) bool {
	if !strings.Contains(templateSegment, "{") {
		return templateSegment == literalSegment
	}

	var pattern strings.Builder
	pattern.WriteString("^")
	cursor := 0
	for _, placeholder := range pathParamPattern.FindAllStringIndex(templateSegment, -1) {
		pattern.WriteString(regexp.QuoteMeta(templateSegment[cursor:placeholder[0]]))
		pattern.WriteString("[^/]+")
		cursor = placeholder[1]
	}
	pattern.WriteString(regexp.QuoteMeta(templateSegment[cursor:]))
	pattern.WriteString("$")

	matcher, err := regexp.Compile(pattern.String())
	if err != nil {
		return false
	}
	return matcher.MatchString(literalSegment)
}

// validateAgentPathValue checks a path the author supplied — a transport prefix
// or the Agent Card path — for the properties the route arithmetic assumes:
// absolute, canonical, and literal.
//
// label names the field in prose so the same rules can be reported against
// either one.
func validateAgentPathValue(field, label, value string) []ValidationError {
	if value == "" {
		return []ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("%s must not be empty; use '/' to serve at the context itself", label),
		}}
	}

	var errors []ValidationError
	if !strings.HasPrefix(value, "/") {
		errors = append(errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must start with /", label),
		})
	}
	if value != "/" && strings.HasSuffix(value, "/") {
		errors = append(errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must not end with / (except for the root value '/')", label),
		})
	}
	if strings.ContainsAny(value, "?#") {
		errors = append(errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be a path only, with no query string or fragment", label),
		})
	}
	// Route templates are the protocol's to define; an author-supplied path is
	// matched literally, so a brace here would end up in the URL rather than
	// capturing anything.
	if strings.ContainsAny(value, "{}") {
		errors = append(errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must not contain path parameter placeholders", label),
		})
	}
	if strings.ContainsFunc(value, unicode.IsSpace) {
		errors = append(errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must not contain whitespace", label),
		})
	}
	// Envoy normalizes the request path before matching, so a value that is not
	// already canonical would be compared against something else at runtime —
	// which is exactly how a route ends up unreachable with nothing to explain
	// it. Only checked once the value is absolute, since path.Clean would
	// otherwise report a difference the leading-slash error already covers.
	if strings.HasPrefix(value, "/") && path.Clean(value) != value {
		errors = append(errors, ValidationError{
			Field: field,
			Message: fmt.Sprintf("%s must be canonical (no '.' or '..' segments and no repeated slashes); did you mean '%s'?",
				label, path.Clean(value)),
		})
	}

	return errors
}

// supportedProtocolVersions renders the registered A2A versions for an error
// message, so a rejection tells the author what they could have written.
func supportedProtocolVersions() string {
	versions := agentproto.Versions()
	rendered := make([]string, 0, len(versions))
	for _, version := range versions {
		rendered = append(rendered, string(version))
	}
	return strings.Join(rendered, ", ")
}

// operationNameList renders a protocol version's operation set for an error
// message. The set is closed and small, so naming all of it beats naming none.
func operationNameList(version agentproto.ProtocolVersion) string {
	operations, ok := agentproto.Operations(version)
	if !ok {
		return ""
	}
	rendered := make([]string, 0, len(operations))
	for _, operation := range operations {
		rendered = append(rendered, string(operation))
	}
	return strings.Join(rendered, ", ")
}
