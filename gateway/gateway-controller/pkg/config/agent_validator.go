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
	"fmt"
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
// Card *content* is not inspected here beyond presence. Validating the card
// document against the A2A model for its protocol version, and checking it
// against the policies protecting it, is separate work.
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
	errors = append(errors, v.validateA2A(agentContextPath(spec.Context), len(contextErrors) == 0, &spec.A2a)...)

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

// agentContextPath is the base every A2A route hangs off: the configured
// context, or "" for an Agent served at the root of its virtual host. The two
// cases converge here so no downstream path arithmetic has to know which one it
// is looking at — joinAgentPath("", "/rpc") is "/rpc", the same shape it
// produces for any other base.
func agentContextPath(context *string) string {
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
			basePath = joinAgentPath(context, prefix)
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
		resolved.path = joinAgentPath(context, cardPath)
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
						path:   joinAgentPath(transport.basePath, binding.PathTemplate),
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

// joinAgentPath joins a base path and a relative segment with exactly one
// separator. A segment of "" or "/" contributes no extra path segment, which is
// what a transport pathPrefix of "/" means: serve at the context itself.
func joinAgentPath(base, segment string) string {
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
