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

package publishers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/ext"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	hdrs "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/headers"
)

// globalPropertyCtxPrefix marks a traffic_logging.properties value as a
// CEL expression to evaluate, rather than a literal string, mirroring the
// log-message policy's "$ctx:" convention.
const globalPropertyCtxPrefix = "$ctx:"

// evaluatorSection names the config section an evaluator serves and what its
// entries are called, so its log messages point at the right place.
type evaluatorSection struct {
	name    string // e.g. "traffic_logging.properties"
	entry   string // e.g. "property"
	entries string // plural of entry
}

var (
	propertiesSection = evaluatorSection{name: "traffic_logging.properties", entry: "property", entries: "properties"}
	jsonFieldsSection = evaluatorSection{name: "traffic_logging.json_fields", entry: "field", entries: "fields"}
)

// globalPropertyEvaluator resolves traffic_logging.properties against a
// dto.Event, mirroring the log-message policy's `properties` CEL surface.
//
// The auth.* namespace here is backed by analytics metadata the collector system
// policy (gateway/system-policies/analytics) stamps generically — auth-type-agnostic,
// via SharedContext.AuthContext — for any authenticated request, regardless of which
// auth policy ran (jwt-auth, opaque-token-auth, api-key-auth, mcp-auth, etc.); no
// modification to any of those policies was needed. It is populated whenever an auth
// policy actually ran and succeeded; for an unauthenticated or denied request (no
// AuthContext, or authentication failed) every auth.* variable is bound to its zero
// value rather than left unbound, so referencing auth.* never errors — it just
// resolves to empty/false/zero, which callers can test for (e.g.
// `auth.subject != "" ? auth.subject : "anonymous"`).
//
// In exchange, global mode can see fields the per-API policy never can, because it
// resolves after the response completes rather than in OnRequestHeaders:
// response.status, response.header, and target.*.
type globalPropertyEvaluator struct {
	// literals are property values with no "$ctx:" prefix, emitted as-is.
	literals map[string]string
	// compiled are pre-compiled CEL programs for "$ctx:"-prefixed values,
	// keyed by property name. Compiled once at construction since the
	// expression set is fixed, static config — not per-request or per-API.
	compiled map[string]cel.Program
	// maskedHeaders holds lower-cased header names redacted in the request.header/
	// response.header CEL variables, mirroring traffic_logging.masked_headers so a
	// property expression cannot re-expose a header (e.g. authorization) that the
	// emitted requestHeaders/responseHeaders map already redacts.
	maskedHeaders map[string]bool
	// excludedRequestHeaders/excludedResponseHeaders hold lower-cased header names
	// removed from request.header/response.header, from the header entries of
	// exclude_fields.
	excludedRequestHeaders  map[string]bool
	excludedResponseHeaders map[string]bool
	// truncateBody, when set, declares request.body/response.body and cuts each
	// body to max_payload_size. Set only for json_fields (see withBodies).
	truncateBody func(string) string
	// section names the config section and its entries in log messages (see withSection).
	section evaluatorSection
	// excludedVariables/excludedMapKeys hold the "$ctx:" entries of exclude_fields. Set
	// only for json_fields: expressions using an excluded variable are dropped at
	// construction, and excluded keys are removed from map variables per request.
	excludedVariables map[string]bool
	excludedMapKeys   map[string]map[string]bool
	// referencedVars is the union, across every compiled expression, of the declared
	// variable names (see globalPropertyVariables) actually referenced. Computed once at
	// construction (the expression set is static config) and consulted by
	// buildGlobalPropertyEvalCtx on every request so it only does the work needed to
	// populate the variables at least one expression can see — most importantly,
	// skipping the request.header/response.header parse+lowercase+mask entirely when no
	// expression references either. Never nil (see newGlobalPropertyEvaluator).
	referencedVars map[string]bool
}

// newGlobalPropertyEvaluator compiles every "$ctx:" expression once. A
// property whose expression fails to compile (including any reference to an
// undeclared variable such as auth.*) is dropped with a logged error rather
// than failing engine startup, consistent with the log-message policy's
// "unresolvable reference -> property omitted" behavior — except the failure
// here is permanent (logged once) since it can never succeed on a later
// request the way a per-request nil AuthContext might resolve differently
// request to request.
func newGlobalPropertyEvaluator(properties map[string]string, maskedHeaders map[string]bool, opts ...evaluatorOption) *globalPropertyEvaluator {
	e := &globalPropertyEvaluator{
		literals:       make(map[string]string),
		compiled:       make(map[string]cel.Program),
		maskedHeaders:  maskedHeaders,
		referencedVars: make(map[string]bool),
		section:        propertiesSection,
	}
	for _, opt := range opts {
		opt(e)
	}
	if len(properties) == 0 {
		return e
	}

	var extraVariables []globalPropertyVariable
	if e.truncateBody != nil {
		extraVariables = bodyVariables
	}
	env, err := createGlobalPropertyEnv(extraVariables...)
	if err != nil {
		slog.Error(e.section.name+": failed to create CEL environment; all "+e.section.entries+" disabled", "error", err)
		return e
	}

	for name, expr := range properties {
		rest, isCtx := strings.CutPrefix(expr, globalPropertyCtxPrefix)
		if !isCtx {
			e.literals[name] = expr
			continue
		}
		ast, issues := env.Compile(rest)
		if issues != nil && issues.Err() != nil {
			slog.Error(e.section.name+": failed to compile expression; "+e.section.entry+" will be omitted from every line",
				e.section.entry, name, "expression", rest, "error", issues.Err())
			continue
		}
		refs := referencedGlobalPropertyVariables(ast)
		if excluded := e.excludedVariablesIn(refs); len(excluded) > 0 {
			slog.Warn(e.section.name+": expression uses a variable excluded by exclude_fields; "+e.section.entry+" will be omitted from every line",
				e.section.entry, name, "variables", excluded)
			continue
		}
		program, err := env.Program(ast)
		if err != nil {
			slog.Error(e.section.name+": failed to build CEL program; "+e.section.entry+" will be omitted from every line",
				e.section.entry, name, "expression", rest, "error", err)
			continue
		}
		e.compiled[name] = program
		// Record which declared variables this expression actually touches, so
		// buildGlobalPropertyEvalCtx can skip populating (and, for headers, parsing)
		// everything else. Err on the side of over-binding: referencedGlobalPropertyVariables
		// reads cel-go's own checked reference map, so it can only under-report if cel-go's
		// checker itself failed to resolve a reference — and an unresolved reference means
		// Compile would already have failed above, so this expression wouldn't be here.
		for varName := range refs {
			e.referencedVars[varName] = true
		}
	}
	return e
}

// excludedVariablesIn returns, sorted, the variables in refs that exclude_fields excludes.
func (e *globalPropertyEvaluator) excludedVariablesIn(refs map[string]bool) []string {
	var out []string
	for v := range refs {
		if e.excludedVariables[v] {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}

// evaluatorOption configures a globalPropertyEvaluator beyond its expressions and
// masked headers.
type evaluatorOption func(*globalPropertyEvaluator)

// withExcludedHeaders removes the given lower-cased header names from
// request.header and response.header.
func withExcludedHeaders(request, response map[string]bool) evaluatorOption {
	return func(e *globalPropertyEvaluator) {
		e.excludedRequestHeaders = request
		e.excludedResponseHeaders = response
	}
}

// withBodies declares request.body and response.body, each passed through truncate.
func withBodies(truncate func(string) string) evaluatorOption {
	return func(e *globalPropertyEvaluator) {
		e.truncateBody = truncate
	}
}

// withCtxExclusions applies the "$ctx:" entries of exclude_fields, so json_fields never
// logs data excluded there, whatever an expression asks for.
func withCtxExclusions(variables map[string]bool, mapKeys map[string]map[string]bool) evaluatorOption {
	return func(e *globalPropertyEvaluator) {
		e.excludedVariables = variables
		e.excludedMapKeys = mapKeys
	}
}

// withSection names the config section in log messages, in place of propertiesSection.
func withSection(section evaluatorSection) evaluatorOption {
	return func(e *globalPropertyEvaluator) {
		e.section = section
	}
}

// referencedGlobalPropertyVariables returns the set of declared variable names (see
// globalPropertyVariables) that a compiled expression's AST actually references, using
// cel-go's own checked reference map (populated by env.Compile, which parses AND
// type-checks — see cel.Ast.NativeRep().ReferenceMap()). Only identifier references carry
// a Name; a function-overload reference (e.g. the index/"in" operators CEL resolves for
// "request.header['x']" or "'x' in request.header") has an empty Name and is skipped, since
// it doesn't correspond to one of our declared variables.
func referencedGlobalPropertyVariables(compiledAst *cel.Ast) map[string]bool {
	refs := compiledAst.NativeRep().ReferenceMap()
	out := make(map[string]bool, len(refs))
	for _, r := range refs {
		if r.Name != "" {
			out[r.Name] = true
		}
	}
	return out
}

// globalPropertyVariable declares one "$ctx:" variable resolvable for global traffic-log
// properties: its CEL type (for env construction) and its Go zero value (for
// buildGlobalPropertyEvalCtx, when the corresponding event field is absent or the variable
// is otherwise unreferenced-but-needed). Declaring name/type/zero together in one slice —
// rather than inline cel.Variable(...) env options plus a separate map literal of zero
// values — is what lets referencedGlobalPropertyVariables and buildGlobalPropertyEvalCtx
// stay in lock-step with the declared variable set with no risk of the lists drifting apart.
type globalPropertyVariable struct {
	name string
	typ  *cel.Type
	zero interface{}
}

// globalPropertyVariables is every "$ctx:" variable resolvable for global traffic-log
// properties. auth.* names deliberately match the log-message policy's own $ctx:auth.*
// variable names (auth.credential_id, auth.token_id, auth.property, not camelCase) so an
// expression can move between per-API and global properties unchanged; application.* has no
// policy equivalent, so it instead matches the emitted TrafficLogApplication JSON field
// names (application.keyType).
//
// Zero values that are slices/maps (auth.audience, auth.property, metadata, the two header
// maps, ...) are never mutated in place anywhere in this file — every assignment in
// buildGlobalPropertyEvalCtx replaces the ctx entry wholesale rather than appending/writing
// into the existing value, and removeExcludedMapKeys skips empty maps — so the same empty
// instance declared here is safely reused (read-only) across every request and goroutine
// instead of allocating a fresh empty collection per call.
var globalPropertyVariables = []globalPropertyVariable{
	{"request.path", cel.StringType, ""},
	{"request.pathTemplate", cel.StringType, ""},
	{"request.method", cel.StringType, ""},
	{"request.id", cel.StringType, ""},
	{"request.timestamp", cel.StringType, ""},
	{"request.header", cel.MapType(cel.StringType, cel.StringType), map[string]string{}},

	{"response.status", cel.IntType, 0},
	{"response.header", cel.MapType(cel.StringType, cel.StringType), map[string]string{}},

	{"api.id", cel.StringType, ""},
	{"api.name", cel.StringType, ""},
	{"api.version", cel.StringType, ""},
	{"api.context", cel.StringType, ""},
	{"api.kind", cel.StringType, ""},

	{"project.id", cel.StringType, ""},

	{"target.statusCode", cel.IntType, 0},
	{"target.destination", cel.StringType, ""},

	// Named after the default traffic-log line's JSON fields.
	{"client.ip", cel.StringType, ""},
	{"client.userAgent", cel.StringType, ""},
	{"latencies.durationUs", cel.IntType, 0},
	{"latencies.requestMediationLatencyUs", cel.IntType, 0},
	{"latencies.responseMediationLatencyUs", cel.IntType, 0},
	{"latencies.backendLatencyUs", cel.IntType, 0},
	// The same latencies in decimal milliseconds.
	{"latencies.durationMs", cel.DoubleType, 0.0},
	{"latencies.requestMediationLatencyMs", cel.DoubleType, 0.0},
	{"latencies.responseMediationLatencyMs", cel.DoubleType, 0.0},
	{"latencies.backendLatencyMs", cel.DoubleType, 0.0},
	{"errorType", cel.StringType, ""},
	{"error.errorCode", cel.IntType, 0},
	{"error.errorMessage", cel.StringType, ""},
	{"error.wso2ErrorCode", cel.IntType, 0},
	{"error.type", cel.StringType, ""},
	{"error.direction", cel.StringType, ""},
	{"error.summary", cel.StringType, ""},
	{"error.policy", cel.StringType, ""},
	{"error.policyPhase", cel.StringType, ""},
	{"error.source", cel.StringType, ""},
	{"error.originalStatus", cel.IntType, 0},
	{"error.jsonRpcCode", cel.IntType, 0},

	{"application.id", cel.StringType, ""},
	{"application.name", cel.StringType, ""},
	{"application.owner", cel.StringType, ""},
	{"application.keyType", cel.StringType, ""},

	// Backed by analytics metadata the collector system policy stamps generically
	// for any authenticated request (see globalPropertyEvaluator doc comment).
	// Always bound (to a zero value when the request wasn't authenticated), so
	// referencing auth.* never errors.
	{"auth.subject", cel.StringType, ""},
	{"auth.type", cel.StringType, ""},
	{"auth.issuer", cel.StringType, ""},
	{"auth.credential_id", cel.StringType, ""},
	{"auth.token_id", cel.StringType, ""},
	{"auth.audience", cel.ListType(cel.StringType), []string{}},
	{"auth.scopes", cel.ListType(cel.StringType), []string{}},
	{"auth.property", cel.MapType(cel.StringType, cel.StringType), map[string]string{}},
	{"auth.authenticated", cel.BoolType, false},
	{"auth.authorized", cel.BoolType, false},

	// Backed by SharedContext.Metadata — a generic bag ANY policy (including
	// third-party/Python policies) can write to, unlike the strongly-typed auth.*
	// namespace above. There is no fixed schema, so values are cel.DynType rather
	// than StringType; a policy author writing to SharedContext.Metadata should
	// assume its value is visible here, since (unlike headers) there is no masking
	// config for this path. Always bound to an empty map when absent, so
	// referencing metadata never errors — same zero-value guarantee as auth.*.
	{"metadata", cel.MapType(cel.StringType, cel.DynType), map[string]interface{}{}},
}

// bodyVariables are declared only for an evaluator built withBodies (json_fields).
// properties adds to the default line, whose bodies stay governed by the
// traffic_logging.request_body/response_body toggles.
var bodyVariables = []globalPropertyVariable{
	{"request.body", cel.StringType, ""},
	{"response.body", cel.StringType, ""},
}

// isDeclaredVariable reports whether name is a "$ctx:" variable, including the body
// variables only json_fields can use.
func isDeclaredVariable(name string) bool {
	isNamed := func(v globalPropertyVariable) bool { return v.name == name }
	return slices.ContainsFunc(globalPropertyVariables, isNamed) || slices.ContainsFunc(bodyVariables, isNamed)
}

// createGlobalPropertyEnv declares every variable in globalPropertyVariables, plus
// extra, in the CEL environment used to compile "$ctx:" expressions.
func createGlobalPropertyEnv(extra ...globalPropertyVariable) (*cel.Env, error) {
	opts := make([]cel.EnvOption, 0, len(globalPropertyVariables)+len(extra)+1)
	opts = append(opts, ext.Strings())
	for _, v := range globalPropertyVariables {
		opts = append(opts, cel.Variable(v.name, v.typ))
	}
	for _, v := range extra {
		opts = append(opts, cel.Variable(v.name, v.typ))
	}
	return cel.NewEnv(opts...)
}

// resolve evaluates every configured property against event, returning nil
// when no properties are configured (so the caller can tell "no properties"
// from "properties present but all empty" the same way the per-API directive
// does). Literal values pass through unchanged; "$ctx:" expressions are
// evaluated and converted to a plain Go value. An expression that fails to
// evaluate at runtime is skipped for that line only, logged at debug level —
// this should be rare, since every declared variable is always bound to a
// zero value rather than left unbound.
func (e *globalPropertyEvaluator) resolve(event *dto.Event) map[string]interface{} {
	if len(e.literals) == 0 && len(e.compiled) == 0 {
		return nil
	}

	result := make(map[string]interface{}, len(e.literals)+len(e.compiled))
	for name, value := range e.literals {
		result[name] = value
	}
	if len(e.compiled) == 0 {
		return result
	}

	evalCtx := buildGlobalPropertyEvalCtx(event, e.referencedVars, e.maskedHeaders, e.excludedRequestHeaders, e.excludedResponseHeaders)
	e.bindBodies(evalCtx, event)
	e.removeExcludedMapKeys(evalCtx)
	for name, program := range e.compiled {
		out, _, err := program.Eval(evalCtx)
		if err != nil {
			slog.Debug(e.section.name+": expression evaluation failed; omitting "+e.section.entry, e.section.entry, name, "error", err)
			continue
		}
		goVal, err := globalPropertyCELToGoValue(out)
		if err != nil {
			slog.Debug(e.section.name+": failed to convert CEL result; omitting "+e.section.entry, e.section.entry, name, "error", err)
			continue
		}
		result[name] = goVal
	}
	return result
}

// buildGlobalPropertyEvalCtx builds the CEL evaluation context from a dto.Event, populating
// only the variables in referenced — the union, across every compiled expression, of the
// variables actually used (see globalPropertyEvaluator.referencedVars) — rather than the
// full declared set. Every populated variable is always bound (to its zero value when the
// corresponding event field is absent) so evaluation never fails merely because a nested
// pointer was nil. request.header/response.header are masked with the same maskedHeaders
// config applied to the emitted requestHeaders/responseHeaders maps (see filterAndMaskHeaders
// in log.go), so a property expression like "$ctx:request.header['authorization']" cannot
// bypass masking and leak a credential the operator explicitly asked to redact — that
// parse+lowercase+mask work (and the json.Unmarshal calls for auth.property/metadata) is
// itself skipped whenever the corresponding variable isn't referenced by any configured
// expression, which is the whole point of this function taking referenced at all.
// Header names in excludedRequestHeaders/excludedResponseHeaders are dropped.
func buildGlobalPropertyEvalCtx(event *dto.Event, referenced map[string]bool, maskedHeaders, excludedRequestHeaders, excludedResponseHeaders map[string]bool) map[string]interface{} {
	ctx := make(map[string]interface{}, len(referenced))
	for _, v := range globalPropertyVariables {
		if referenced[v.name] {
			ctx[v.name] = v.zero
		}
	}

	if event == nil {
		return ctx
	}

	if event.Operation != nil {
		if referenced["request.path"] {
			ctx["request.path"] = event.Operation.APIResourceTemplate
		}
		if referenced["request.method"] {
			ctx["request.method"] = event.Operation.APIMethod
		}
	}
	if referenced["request.pathTemplate"] {
		if tmpl, ok := event.Properties[dto.PropKeyOperationPath].(string); ok {
			ctx["request.pathTemplate"] = tmpl
		}
	}
	if referenced["request.id"] && event.MetaInfo != nil {
		ctx["request.id"] = event.MetaInfo.CorrelationID
	}
	if referenced["request.timestamp"] && !event.RequestTimestamp.IsZero() {
		ctx["request.timestamp"] = event.RequestTimestamp.UTC().Format(trafficLogTimestampFormat)
	}
	if referenced["request.header"] {
		if headers := hdrs.Flatten(event.Properties[dto.PropKeyRequestHeaders]); headers != nil {
			ctx["request.header"] = filterAndMaskHeaders(lowerCaseHeaderKeys(headers), maskedHeaders, excludedRequestHeaders)
		}
	}

	if referenced["response.status"] {
		ctx["response.status"] = event.ProxyResponseCode
	}
	if referenced["response.header"] {
		if headers := hdrs.Flatten(event.Properties[dto.PropKeyResponseHeaders]); headers != nil {
			ctx["response.header"] = filterAndMaskHeaders(lowerCaseHeaderKeys(headers), maskedHeaders, excludedResponseHeaders)
		}
	}

	if event.API != nil {
		if referenced["api.id"] {
			ctx["api.id"] = event.API.APIID
		}
		if referenced["api.name"] {
			ctx["api.name"] = event.API.APIName
		}
		if referenced["api.version"] {
			ctx["api.version"] = event.API.APIVersion
		}
		if referenced["api.context"] {
			ctx["api.context"] = event.API.APIContext
		}
		if referenced["api.kind"] {
			ctx["api.kind"] = event.API.APIType
		}
		if referenced["project.id"] {
			ctx["project.id"] = event.API.ProjectID
		}
	}

	if event.Target != nil {
		if referenced["target.statusCode"] {
			ctx["target.statusCode"] = event.Target.TargetResponseCode
		}
		if referenced["target.destination"] {
			ctx["target.destination"] = event.Target.Destination
		}
	}

	if referenced["client.ip"] {
		ctx["client.ip"] = event.UserIP
	}
	if referenced["client.userAgent"] {
		ctx["client.userAgent"] = event.UserAgentHeader
	}
	if l := event.TrafficLogLatencies; l != nil {
		setIfReferenced(ctx, referenced, "latencies.durationUs", l.DurationUs)
		setIfReferenced(ctx, referenced, "latencies.requestMediationLatencyUs", l.RequestMediationLatencyUs)
		setIfReferenced(ctx, referenced, "latencies.responseMediationLatencyUs", l.ResponseMediationLatencyUs)
		setIfReferenced(ctx, referenced, "latencies.backendLatencyUs", l.BackendLatencyUs)
		setIfReferenced(ctx, referenced, "latencies.durationMs", microsToMillis(l.DurationUs))
		setIfReferenced(ctx, referenced, "latencies.requestMediationLatencyMs", microsToMillis(l.RequestMediationLatencyUs))
		setIfReferenced(ctx, referenced, "latencies.responseMediationLatencyMs", microsToMillis(l.ResponseMediationLatencyUs))
		setIfReferenced(ctx, referenced, "latencies.backendLatencyMs", microsToMillis(l.BackendLatencyUs))
	}
	setIfReferenced(ctx, referenced, "errorType", event.ErrorType)
	if e := event.Error; e != nil {
		setIfReferenced(ctx, referenced, "error.errorCode", e.ErrorCode)
		setIfReferenced(ctx, referenced, "error.errorMessage", string(e.ErrorMessage))
		setIfReferenced(ctx, referenced, "error.wso2ErrorCode", e.Wso2ErrorCode)
		setIfReferenced(ctx, referenced, "error.type", e.Type)
		setIfReferenced(ctx, referenced, "error.direction", e.Direction)
		setIfReferenced(ctx, referenced, "error.summary", e.Summary)
		setIfReferenced(ctx, referenced, "error.policy", e.Policy)
		setIfReferenced(ctx, referenced, "error.policyPhase", e.PolicyPhase)
		setIfReferenced(ctx, referenced, "error.source", e.Source)
		setIfReferenced(ctx, referenced, "error.originalStatus", e.OriginalStatus)
		setIfReferenced(ctx, referenced, "error.jsonRpcCode", e.JSONRPCCode)
	}

	if a := event.Application; a != nil {
		if referenced["application.id"] {
			ctx["application.id"] = a.ApplicationID
		}
		if referenced["application.name"] {
			ctx["application.name"] = a.ApplicationName
		}
		if referenced["application.owner"] {
			ctx["application.owner"] = a.ApplicationOwner
		}
		if referenced["application.keyType"] {
			ctx["application.keyType"] = a.KeyType
		}
	}

	// Auth-context, backed by analytics metadata the collector system policy stamps
	// generically for any authenticated request (see globalPropertyEvaluator doc
	// comment). auth.subject presence is what auth.authenticated derives from, rather
	// than a separately stamped flag, since the collector only ever stamps these keys
	// together, gated on Authenticated && Subject != "" (see populateAuthAnalyticsMetadata
	// in gateway/system-policies/analytics/analytics.go). Both are derived from the same
	// lookup, so it's guarded by either being referenced.
	if referenced["auth.subject"] || referenced["auth.authenticated"] {
		if subject, ok := event.Properties[dto.PropKeyAuthUserID].(string); ok && subject != "" {
			if referenced["auth.subject"] {
				ctx["auth.subject"] = subject
			}
			if referenced["auth.authenticated"] {
				ctx["auth.authenticated"] = true
			}
		}
	}
	if referenced["auth.type"] {
		if authType, ok := event.Properties[dto.PropKeyAuthType].(string); ok {
			ctx["auth.type"] = authType
		}
	}
	if referenced["auth.issuer"] {
		if issuer, ok := event.Properties[dto.PropKeyAuthIssuer].(string); ok {
			ctx["auth.issuer"] = issuer
		}
	}
	if referenced["auth.credential_id"] {
		if credentialID, ok := event.Properties[dto.PropKeyAuthCredentialID].(string); ok {
			ctx["auth.credential_id"] = credentialID
		}
	}
	if referenced["auth.token_id"] {
		if tokenID, ok := event.Properties[dto.PropKeyAuthTokenID].(string); ok {
			ctx["auth.token_id"] = tokenID
		}
	}
	if referenced["auth.audience"] {
		if audience, ok := event.Properties[dto.PropKeyAuthAudience].(string); ok && audience != "" {
			ctx["auth.audience"] = strings.Split(audience, ",")
		}
	}
	if referenced["auth.scopes"] {
		if scopes, ok := event.Properties[dto.PropKeyAuthScopes].(string); ok && scopes != "" {
			ctx["auth.scopes"] = strings.Split(scopes, " ")
		}
	}
	if referenced["auth.property"] {
		if raw, ok := event.Properties[dto.PropKeyAuthProperties].(string); ok && raw != "" {
			var props map[string]string
			if err := json.Unmarshal([]byte(raw), &props); err == nil {
				ctx["auth.property"] = props
			} else {
				slog.Debug("traffic_logging: failed to parse auth properties metadata", "error", err)
			}
		}
	}
	if referenced["auth.authorized"] {
		if authorized, ok := event.Properties[dto.PropKeyAuthAuthorized].(string); ok {
			if parsed, err := strconv.ParseBool(authorized); err == nil {
				ctx["auth.authorized"] = parsed
			}
		}
	}

	// SharedContext.Metadata, JSON-encoded by populateGenericMetadata in
	// gateway/system-policies/analytics/analytics.go. Raw and unfiltered: any policy
	// author writing to SharedContext.Metadata should assume its value reaches here.
	if referenced["metadata"] {
		if raw, ok := event.Properties[dto.PropKeyMetadata].(string); ok && raw != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &meta); err == nil {
				ctx["metadata"] = meta
			} else {
				slog.Debug("traffic_logging: failed to parse generic metadata", "error", err)
			}
		}
	}

	return ctx
}

// bindBodies sets request.body/response.body for the expressions that use them: the
// body the collector captured, cut to max_payload_size, or "" when none was captured.
// It does nothing unless the evaluator was built withBodies.
func (e *globalPropertyEvaluator) bindBodies(ctx map[string]interface{}, event *dto.Event) {
	if e.truncateBody == nil {
		return
	}
	if e.referencedVars["request.body"] {
		ctx["request.body"] = e.truncateBody(capturedBody(event, dto.PropKeyRequestPayload))
	}
	if e.referencedVars["response.body"] {
		ctx["response.body"] = e.truncateBody(capturedBody(event, dto.PropKeyResponsePayload))
	}
}

// removeExcludedMapKeys deletes the keys excluded by "$ctx:<map variable>.<key>"
// entries from the auth.property and metadata maps in ctx. Those maps are parsed
// afresh for every request, so deleting from them never changes the event.
func (e *globalPropertyEvaluator) removeExcludedMapKeys(ctx map[string]interface{}) {
	for mapVar, keys := range e.excludedMapKeys {
		switch m := ctx[mapVar].(type) {
		case map[string]string:
			deleteKeys(m, keys)
		case map[string]interface{}:
			deleteKeys(m, keys)
		}
	}
}

// deleteKeys removes keys from m. An empty m is left untouched, since it may be the
// shared zero value declared in globalPropertyVariables.
func deleteKeys[V any](m map[string]V, keys map[string]bool) {
	if len(m) == 0 {
		return
	}
	for k := range keys {
		delete(m, k)
	}
}

// capturedBody returns the body stored under key in the event, or "" when there is none.
func capturedBody(event *dto.Event, key string) string {
	if event == nil {
		return ""
	}
	body, _ := event.Properties[key].(string)
	return body
}

// microsToMillis converts microseconds to decimal milliseconds, so sub-millisecond
// latencies are not rounded down to 0.
func microsToMillis(us int64) float64 {
	return float64(us) / 1000
}

// setIfReferenced sets ctx[name] only when an expression uses that variable.
func setIfReferenced(ctx map[string]interface{}, referenced map[string]bool, name string, value interface{}) {
	if referenced[name] {
		ctx[name] = value
	}
}

// lowerCaseHeaderKeys returns a copy of headers with all keys lower-cased.
// The collector's serializeHeaders already lower-cases captured header names,
// so this is defensive normalization — it keeps request.header['x']/
// response.header['x'] reliably lowercase-keyed even if a future capture path
// changes, matching the log-message policy's documented guarantee for its own
// request.header variable.
func lowerCaseHeaderKeys(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[strings.ToLower(k)] = v
	}
	return out
}

// globalPropertyCELToGoValue converts a CEL result to a plain Go value that
// encoding/json can marshal directly. Mirrors the log-message policy's
// identically-purposed helper (duplicated here rather than shared: the
// policy lives in dev-policies/log-message, a separate Go module wired in
// via build-manifest.yaml/build.yaml, not a dependency of the policy-engine
// binary).
func globalPropertyCELToGoValue(val ref.Val) (interface{}, error) {
	native, err := val.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		return nil, err
	}
	pbVal, ok := native.(*structpb.Value)
	if !ok {
		return nil, fmt.Errorf("unexpected CEL native conversion result type %T", native)
	}
	return pbVal.AsInterface(), nil
}
