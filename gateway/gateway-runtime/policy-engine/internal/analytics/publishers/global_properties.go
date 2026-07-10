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
	"strconv"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// globalPropertyCtxPrefix marks a traffic_logging.properties value as a
// CEL expression to evaluate, rather than a literal string, mirroring the
// log-message policy's "$ctx:" convention.
const globalPropertyCtxPrefix = "$ctx:"

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
}

// newGlobalPropertyEvaluator compiles every "$ctx:" expression once. A
// property whose expression fails to compile (including any reference to an
// undeclared variable such as auth.*) is dropped with a logged error rather
// than failing engine startup, consistent with the log-message policy's
// "unresolvable reference -> property omitted" behavior — except the failure
// here is permanent (logged once) since it can never succeed on a later
// request the way a per-request nil AuthContext might resolve differently
// request to request.
func newGlobalPropertyEvaluator(properties map[string]string, maskedHeaders map[string]bool) *globalPropertyEvaluator {
	e := &globalPropertyEvaluator{
		literals:      make(map[string]string),
		compiled:      make(map[string]cel.Program),
		maskedHeaders: maskedHeaders,
	}
	if len(properties) == 0 {
		return e
	}

	env, err := createGlobalPropertyEnv()
	if err != nil {
		slog.Error("traffic_logging.properties: failed to create CEL environment; all properties disabled", "error", err)
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
			slog.Error("traffic_logging.properties: failed to compile expression; property will be omitted from every line",
				"property", name, "expression", rest, "error", issues.Err())
			continue
		}
		program, err := env.Program(ast)
		if err != nil {
			slog.Error("traffic_logging.properties: failed to build CEL program; property will be omitted from every line",
				"property", name, "expression", rest, "error", err)
			continue
		}
		e.compiled[name] = program
	}
	return e
}

// createGlobalPropertyEnv declares every "$ctx:" variable resolvable for global
// traffic-log properties. auth.* names deliberately match the log-message policy's
// own $ctx:auth.* variable names (auth.credential_id, auth.token_id, auth.property,
// not camelCase) so an expression can move between per-API and global properties
// unchanged; application.* has no policy equivalent, so it instead matches the
// emitted TrafficLogApplication JSON field names (application.keyType).
func createGlobalPropertyEnv() (*cel.Env, error) {
	return cel.NewEnv(
		ext.Strings(),

		cel.Variable("request.path", cel.StringType),
		cel.Variable("request.method", cel.StringType),
		cel.Variable("request.id", cel.StringType),
		cel.Variable("request.header", cel.MapType(cel.StringType, cel.StringType)),

		cel.Variable("response.status", cel.IntType),
		cel.Variable("response.header", cel.MapType(cel.StringType, cel.StringType)),

		cel.Variable("api.id", cel.StringType),
		cel.Variable("api.name", cel.StringType),
		cel.Variable("api.version", cel.StringType),
		cel.Variable("api.context", cel.StringType),
		cel.Variable("api.kind", cel.StringType),

		cel.Variable("project.id", cel.StringType),

		cel.Variable("target.statusCode", cel.IntType),
		cel.Variable("target.destination", cel.StringType),

		cel.Variable("application.id", cel.StringType),
		cel.Variable("application.name", cel.StringType),
		cel.Variable("application.owner", cel.StringType),
		cel.Variable("application.keyType", cel.StringType),

		// Backed by analytics metadata the collector system policy stamps generically
		// for any authenticated request (see globalPropertyEvaluator doc comment).
		// Always bound (to a zero value when the request wasn't authenticated), so
		// referencing auth.* never errors.
		cel.Variable("auth.subject", cel.StringType),
		cel.Variable("auth.type", cel.StringType),
		cel.Variable("auth.issuer", cel.StringType),
		cel.Variable("auth.credential_id", cel.StringType),
		cel.Variable("auth.token_id", cel.StringType),
		cel.Variable("auth.audience", cel.ListType(cel.StringType)),
		cel.Variable("auth.scopes", cel.ListType(cel.StringType)),
		cel.Variable("auth.property", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("auth.authenticated", cel.BoolType),
		cel.Variable("auth.authorized", cel.BoolType),
	)
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

	evalCtx := buildGlobalPropertyEvalCtx(event, e.maskedHeaders)
	for name, program := range e.compiled {
		out, _, err := program.Eval(evalCtx)
		if err != nil {
			slog.Debug("traffic_logging.properties: expression evaluation failed; omitting property", "property", name, "error", err)
			continue
		}
		goVal, err := globalPropertyCELToGoValue(out)
		if err != nil {
			slog.Debug("traffic_logging.properties: failed to convert CEL result; omitting property", "property", name, "error", err)
			continue
		}
		result[name] = goVal
	}
	return result
}

// buildGlobalPropertyEvalCtx builds the CEL evaluation context from a
// dto.Event. Every declared variable is always bound (to a zero value when
// the corresponding event field is absent) so evaluation never fails merely
// because a nested pointer was nil. request.header/response.header are masked
// with the same maskedHeaders config applied to the emitted requestHeaders/
// responseHeaders maps (see maskHeaders in log.go), so a property expression
// like "$ctx:request.header['authorization']" cannot bypass masking and leak a
// credential the operator explicitly asked to redact.
func buildGlobalPropertyEvalCtx(event *dto.Event, maskedHeaders map[string]bool) map[string]interface{} {
	ctx := map[string]interface{}{
		"request.path":        "",
		"request.method":      "",
		"request.id":          "",
		"request.header":      map[string]string{},
		"response.status":     0,
		"response.header":     map[string]string{},
		"api.id":              "",
		"api.name":            "",
		"api.version":         "",
		"api.context":         "",
		"api.kind":            "",
		"project.id":          "",
		"target.statusCode":   0,
		"target.destination":  "",
		"application.id":      "",
		"application.name":    "",
		"application.owner":   "",
		"application.keyType": "",
		"auth.subject":        "",
		"auth.type":           "",
		"auth.issuer":         "",
		"auth.credential_id":  "",
		"auth.token_id":       "",
		"auth.audience":       []string{},
		"auth.scopes":         []string{},
		"auth.property":       map[string]string{},
		"auth.authenticated":  false,
		"auth.authorized":     false,
	}

	if event == nil {
		return ctx
	}

	if event.Operation != nil {
		ctx["request.path"] = event.Operation.APIResourceTemplate
		ctx["request.method"] = event.Operation.APIMethod
	}
	if event.MetaInfo != nil {
		ctx["request.id"] = event.MetaInfo.CorrelationID
	}
	if raw, ok := event.Properties[dto.PropKeyRequestHeaders].(string); ok {
		if headers := parseHeadersFromString(raw); headers != nil {
			ctx["request.header"] = maskHeaders(lowerCaseHeaderKeys(headers), maskedHeaders)
		}
	}

	ctx["response.status"] = event.ProxyResponseCode
	if raw, ok := event.Properties[dto.PropKeyResponseHeaders].(string); ok {
		if headers := parseHeadersFromString(raw); headers != nil {
			ctx["response.header"] = maskHeaders(lowerCaseHeaderKeys(headers), maskedHeaders)
		}
	}

	if event.API != nil {
		ctx["api.id"] = event.API.APIID
		ctx["api.name"] = event.API.APIName
		ctx["api.version"] = event.API.APIVersion
		ctx["api.context"] = event.API.APIContext
		ctx["api.kind"] = event.API.APIType
		ctx["project.id"] = event.API.ProjectID
	}

	if event.Target != nil {
		ctx["target.statusCode"] = event.Target.TargetResponseCode
		ctx["target.destination"] = event.Target.Destination
	}

	if a := event.Application; a != nil {
		ctx["application.id"] = a.ApplicationID
		ctx["application.name"] = a.ApplicationName
		ctx["application.owner"] = a.ApplicationOwner
		ctx["application.keyType"] = a.KeyType
	}

	// Auth-context, backed by analytics metadata the collector system policy stamps
	// generically for any authenticated request (see globalPropertyEvaluator doc
	// comment). auth.subject presence is what auth.authenticated derives from, rather
	// than a separately stamped flag, since the collector only ever stamps these keys
	// together, gated on Authenticated && Subject != "" (see populateAuthAnalyticsMetadata
	// in gateway/system-policies/analytics/analytics.go).
	if subject, ok := event.Properties[dto.PropKeyAuthUserID].(string); ok && subject != "" {
		ctx["auth.subject"] = subject
		ctx["auth.authenticated"] = true
	}
	if authType, ok := event.Properties[dto.PropKeyAuthType].(string); ok {
		ctx["auth.type"] = authType
	}
	if issuer, ok := event.Properties[dto.PropKeyAuthIssuer].(string); ok {
		ctx["auth.issuer"] = issuer
	}
	if credentialID, ok := event.Properties[dto.PropKeyAuthCredentialID].(string); ok {
		ctx["auth.credential_id"] = credentialID
	}
	if tokenID, ok := event.Properties[dto.PropKeyAuthTokenID].(string); ok {
		ctx["auth.token_id"] = tokenID
	}
	if audience, ok := event.Properties[dto.PropKeyAuthAudience].(string); ok && audience != "" {
		ctx["auth.audience"] = strings.Split(audience, ",")
	}
	if scopes, ok := event.Properties[dto.PropKeyAuthScopes].(string); ok && scopes != "" {
		ctx["auth.scopes"] = strings.Split(scopes, " ")
	}
	if raw, ok := event.Properties[dto.PropKeyAuthProperties].(string); ok && raw != "" {
		var props map[string]string
		if err := json.Unmarshal([]byte(raw), &props); err == nil {
			ctx["auth.property"] = props
		} else {
			slog.Debug("traffic_logging.properties: failed to parse auth properties metadata", "error", err)
		}
	}
	if authorized, ok := event.Properties[dto.PropKeyAuthAuthorized].(string); ok {
		if parsed, err := strconv.ParseBool(authorized); err == nil {
			ctx["auth.authorized"] = parsed
		}
	}

	return ctx
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
