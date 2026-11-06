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
	"strconv"
	"strings"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

func TestNewGlobalPropertyEvaluator_EmptyConfig(t *testing.T) {
	e := newGlobalPropertyEvaluator(nil, nil, 0)
	require.NotNil(t, e)
	assert.Nil(t, e.resolve(createBaseEvent()))
}

func TestGlobalPropertyEvaluator_LiteralPassthrough(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{"env": "prod"}, nil, 0)
	resolved := e.resolve(createBaseEvent())
	assert.Equal(t, "prod", resolved["env"])
}

func TestGlobalPropertyEvaluator_ResolvesRequestAndAPIContext(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"path":    "$ctx:request.path",
		"method":  "$ctx:request.method",
		"reqId":   "$ctx:request.id",
		"apiName": "$ctx:api.name",
		"apiKind": "$ctx:api.kind",
		"project": "$ctx:project.id",
	}, nil, 0)
	event := createBaseEvent()

	resolved := e.resolve(event)

	assert.Equal(t, "/resource", resolved["path"])
	assert.Equal(t, "GET", resolved["method"])
	assert.Equal(t, "corr-123", resolved["reqId"])
	assert.Equal(t, "test-api", resolved["apiName"])
	assert.Equal(t, "Rest", resolved["apiKind"])
	assert.Equal(t, "project-123", resolved["project"])
}

// request.pathTemplate is the matched operation template, not the concrete request path,
// and falls back to empty when the collector did not stamp it.
func TestGlobalPropertyEvaluator_ResolvesRequestPathTemplate(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"route": "$ctx:request.pathTemplate",
	}, nil, 0)

	event := createBaseEvent()
	event.Operation.APIResourceTemplate = "/basepath/auth/web"
	event.Properties = map[string]interface{}{dto.PropKeyOperationPath: "/basepath/auth/{channel}"}
	assert.Equal(t, "/basepath/auth/{channel}", e.resolve(event)["route"])

	assert.Equal(t, "", e.resolve(createBaseEvent())["route"])
}

// response.status and target.statusCode are available to global properties
// even though the log-message policy (which resolves in OnRequestHeaders,
// before any response exists) can never see them.
func TestGlobalPropertyEvaluator_ResolvesResponseAndTargetInfo(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"status":       "$ctx:response.status",
		"targetStatus": "$ctx:target.statusCode",
		"dest":         "$ctx:target.destination",
	}, nil, 0)
	event := createBaseEvent()
	event.ProxyResponseCode = 503
	event.Target = &dto.Target{
		TargetResponseCode: 502,
		Destination:        "backend.internal:8080/resource",
	}

	resolved := e.resolve(event)

	assert.Equal(t, float64(503), resolved["status"])
	assert.Equal(t, float64(502), resolved["targetStatus"])
	assert.Equal(t, "backend.internal:8080/resource", resolved["dest"])
}

// application.* reflects whatever an auth policy stamped into analytics
// metadata (e.g. api-key-auth), independent of any log-message policy.
func TestGlobalPropertyEvaluator_ResolvesApplicationInfo(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"appId":   "$ctx:application.id",
		"appName": "$ctx:application.name",
		"appKey":  "$ctx:application.keyType",
	}, nil, 0)
	event := createBaseEvent()
	event.Application = &dto.Application{
		ApplicationID:   "app-1",
		ApplicationName: "my-app",
		KeyType:         "PRODUCTION",
	}

	resolved := e.resolve(event)

	assert.Equal(t, "app-1", resolved["appId"])
	assert.Equal(t, "my-app", resolved["appName"])
	assert.Equal(t, "PRODUCTION", resolved["appKey"])
}

func TestGlobalPropertyEvaluator_ResolvesHeaders(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"tenant":   "$ctx:request.header['x-tenant-id']",
		"traceId":  "$ctx:response.header['x-trace-id']",
		"hasAuthz": "$ctx:'authorization' in request.header",
	}, nil, 0)
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-tenant-id":"acme","authorization":"Bearer x"}`
	event.Properties["responseHeaders"] = `{"x-trace-id":"trace-abc"}`

	resolved := e.resolve(event)

	assert.Equal(t, "acme", resolved["tenant"])
	assert.Equal(t, "trace-abc", resolved["traceId"])
	assert.Equal(t, true, resolved["hasAuthz"])
}

// A property expression reading a masked header (e.g. authorization) must see
// the same redacted "****" placeholder as the emitted requestHeaders/
// responseHeaders maps — otherwise traffic_logging.masked_headers would give a
// false sense that a credential is protected while a property expression leaks
// it verbatim.
func TestGlobalPropertyEvaluator_MasksSensitiveHeaders(t *testing.T) {
	mask := map[string]bool{"authorization": true}
	e := newGlobalPropertyEvaluator(map[string]string{
		"token":   "$ctx:request.header['authorization']",
		"traceId": "$ctx:response.header['x-trace-id']",
	}, mask, 0)
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"authorization":"Bearer secret-token"}`
	event.Properties["responseHeaders"] = `{"x-trace-id":"trace-abc"}`

	resolved := e.resolve(event)

	assert.Equal(t, maskedHeaderValue, resolved["token"], "masked header must not leak its raw value via a $ctx: property")
	assert.Equal(t, "trace-abc", resolved["traceId"], "unmasked header still resolves normally")
}

// metadata is backed by SharedContext.Metadata -- a generic bag ANY policy can write
// to (see gateway/system-policies/analytics's populateGenericMetadata) -- unlike
// auth.*, there is no fixed schema, so values of different underlying JSON types
// (string, bool, number) must all resolve correctly via cel.DynType.
func TestGlobalPropertyEvaluator_ResolvesGenericMetadata(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"appId":     "$ctx:metadata['applicationId']",
		"isTrial":   "$ctx:metadata['isTrial']",
		"maxTokens": "$ctx:metadata['maxTokens']",
	}, nil, 0)
	event := createBaseEvent()
	event.Properties[dto.PropKeyMetadata] = `{"applicationId":"app-42","isTrial":true,"maxTokens":100}`

	resolved := e.resolve(event)

	assert.Equal(t, "app-42", resolved["appId"])
	assert.Equal(t, true, resolved["isTrial"])
	assert.Equal(t, float64(100), resolved["maxTokens"])
}

// When no policy wrote anything to SharedContext.Metadata (or the request never
// reached the collector), metadata resolves to an empty map rather than being
// unbound -- consistent with the zero-value guarantee every other $ctx: namespace
// gives. Unguarded indexing into an empty/missing key still errors and is omitted,
// same behavior as auth.property, and for the same reason: map *index* is not
// covered by the scalar zero-value guarantee -- use `in` to guard it.
func TestGlobalPropertyEvaluator_GenericMetadataAbsentRequiresInGuard(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"unguarded": "$ctx:metadata['applicationId']",
		"guarded":   "$ctx:'applicationId' in metadata ? metadata['applicationId'] : ''",
	}, nil, 0)
	event := createBaseEvent() // no metadata property set at all

	resolved := e.resolve(event)

	_, hasUnguarded := resolved["unguarded"]
	assert.False(t, hasUnguarded, "unguarded index into absent metadata omits the property")
	assert.Equal(t, "", resolved["guarded"], "in-guarded index resolves to the fallback instead of erroring")
}

// auth.* is backed by analytics metadata the collector system policy stamps generically
// for any authenticated request (see gateway/system-policies/analytics's
// populateAuthAnalyticsMetadata), regardless of auth type — no auth policy modification
// was needed for this to work.
func TestGlobalPropertyEvaluator_ResolvesAuthContext_WhenAuthenticated(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"subject":       "$ctx:auth.subject",
		"authType":      "$ctx:auth.type",
		"issuer":        "$ctx:auth.issuer",
		"credentialId":  "$ctx:auth.credential_id",
		"tokenId":       "$ctx:auth.token_id",
		"audience":      "$ctx:auth.audience",
		"scopes":        "$ctx:auth.scopes",
		"tenant":        "$ctx:auth.property['tenant']",
		"authenticated": "$ctx:auth.authenticated",
		"authorized":    "$ctx:auth.authorized",
	}, nil, 0)
	event := createBaseEvent()
	event.Properties[dto.PropKeyAuthUserID] = "alice"
	event.Properties[dto.PropKeyAuthType] = "jwt"
	event.Properties[dto.PropKeyAuthIssuer] = "https://issuer.example.com"
	event.Properties[dto.PropKeyAuthCredentialID] = "client-123"
	event.Properties[dto.PropKeyAuthTokenID] = "jti-abc"
	event.Properties[dto.PropKeyAuthAudience] = "aud1,aud2"
	event.Properties[dto.PropKeyAuthScopes] = "admin read"
	event.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme"}`
	event.Properties[dto.PropKeyAuthAuthorized] = "true"

	resolved := e.resolve(event)

	assert.Equal(t, "alice", resolved["subject"])
	assert.Equal(t, "jwt", resolved["authType"])
	assert.Equal(t, "https://issuer.example.com", resolved["issuer"])
	assert.Equal(t, "client-123", resolved["credentialId"])
	assert.Equal(t, "jti-abc", resolved["tokenId"])
	assert.Equal(t, []interface{}{"aud1", "aud2"}, resolved["audience"])
	assert.Equal(t, []interface{}{"admin", "read"}, resolved["scopes"])
	assert.Equal(t, "acme", resolved["tenant"])
	assert.Equal(t, true, resolved["authenticated"])
	assert.Equal(t, true, resolved["authorized"])
}

// When the request is unauthenticated (or auth failed / was denied), every auth.*
// variable resolves to its zero value rather than erroring — so a conditional
// expression like this one is the recommended way to handle both cases in one property.
func TestGlobalPropertyEvaluator_AuthDefaultsToZeroValuesWhenUnauthenticated(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"subject":       "$ctx:auth.subject != '' ? auth.subject : 'anonymous'",
		"authenticated": "$ctx:auth.authenticated",
		"scopes":        "$ctx:auth.scopes",
	}, nil, 0)
	event := createBaseEvent() // no auth.* properties set — e.g. a denied/401 request

	resolved := e.resolve(event)

	assert.Equal(t, "anonymous", resolved["subject"])
	assert.Equal(t, false, resolved["authenticated"])
	assert.Equal(t, []interface{}{}, resolved["scopes"])
}

// Unlike scalar auth.* variables, indexing into auth.property (a map) for a
// claim that isn't present raises a "no such key" evaluation error rather than
// resolving to a zero value — so an unguarded auth.property['tenant'] omits the
// property entirely on any request whose token lacks that claim. Guarding the
// index with `in` (the pattern documented in config-template.toml) resolves
// safely in both cases instead.
func TestGlobalPropertyEvaluator_AuthPropertyIndexRequiresInGuard(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"unguarded": "$ctx:auth.property['tenant']",
		"guarded":   "$ctx:'tenant' in auth.property ? auth.property['tenant'] : ''",
	}, nil, 0)

	withClaim := createBaseEvent()
	withClaim.Properties[dto.PropKeyAuthProperties] = `{"tenant":"acme"}`
	resolved := e.resolve(withClaim)
	assert.Equal(t, "acme", resolved["unguarded"])
	assert.Equal(t, "acme", resolved["guarded"])

	withoutClaim := createBaseEvent() // no auth.property claims at all
	resolved = e.resolve(withoutClaim)
	_, hasUnguarded := resolved["unguarded"]
	assert.False(t, hasUnguarded, "unguarded map index omits the property when the claim is absent")
	assert.Equal(t, "", resolved["guarded"], "in-guarded index resolves to the fallback instead of erroring")
}

// A reference to a variable that genuinely doesn't exist in the CEL environment still
// fails to compile and is permanently omitted — the general mechanism auth.* used to
// exercise before the namespace existed.
func TestGlobalPropertyEvaluator_UndeclaredVariableOmitted(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"bogus": "$ctx:nonexistent.field",
		"ok":    "literal-value",
	}, nil, 0)
	resolved := e.resolve(createBaseEvent())

	_, hasBogus := resolved["bogus"]
	assert.False(t, hasBogus, "undeclared variable reference must fail to compile and be omitted")
	assert.Equal(t, "literal-value", resolved["ok"], "other properties still resolve")
}

// A malformed CEL expression (syntax error) is dropped at construction, not a
// panic, and does not prevent other properties from resolving.
func TestGlobalPropertyEvaluator_MalformedExpressionOmitted(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"broken": "$ctx:request.path +++ invalid",
		"ok":     "$ctx:request.method",
	}, nil, 0)
	event := createBaseEvent()

	resolved := e.resolve(event)

	_, hasBroken := resolved["broken"]
	assert.False(t, hasBroken)
	assert.Equal(t, "GET", resolved["ok"])
}

func TestGlobalPropertyEvaluator_NoPropertiesReturnsNilNotEmptyMap(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{}, nil, 0)
	assert.Nil(t, e.resolve(createBaseEvent()))
}

// Task 2 (CEL variable binding): a config that references no request.header/response.header
// expression must never pay for header parsing/lowercasing/masking at all -- not just "not use
// the result". referencedVars is what buildGlobalPropertyEvalCtx consults to decide this, so
// asserting it directly (and that the eval context omits the header keys entirely) locks in the
// optimization rather than merely its externally-observable output.
func TestGlobalPropertyEvaluator_NoHeaderReference_HeadersNeverParsed(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"apiName": "$ctx:api.name",
		"status":  "$ctx:response.status",
	}, nil, 0)

	assert.False(t, e.referencedVars["request.header"], "request.header must not be marked referenced")
	assert.False(t, e.referencedVars["response.header"], "response.header must not be marked referenced")

	event := createBaseEvent()
	// Present in the event but must be ignored entirely since nothing references it.
	event.Properties["requestHeaders"] = `{"x-tenant-id":"acme"}`
	event.Properties["responseHeaders"] = `{"x-trace-id":"trace-abc"}`

	resolved := e.resolve(event)
	assert.Equal(t, "test-api", resolved["apiName"])
	assert.Equal(t, float64(200), resolved["status"])

	ctx := buildGlobalPropertyEvalCtx(event, e.referencedVars, nil, 0)
	_, hasReqHeader := ctx["request.header"]
	_, hasRespHeader := ctx["response.header"]
	assert.False(t, hasReqHeader, "request.header key must be absent from the eval context, not just unused")
	assert.False(t, hasRespHeader, "response.header key must be absent from the eval context, not just unused")
}

// The converse of the test above: when an expression DOES reference request.header, headers
// are parsed, lower-cased and masked exactly as before -- referencedVars must include it.
func TestGlobalPropertyEvaluator_HeaderReference_MarksReferencedAndParses(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"tenant": "$ctx:request.header['x-tenant-id']",
	}, map[string]bool{"authorization": true}, 0)

	assert.True(t, e.referencedVars["request.header"])
	assert.False(t, e.referencedVars["response.header"], "response.header must stay unreferenced when only request.header is used")

	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"X-Tenant-Id":"acme","Authorization":"Bearer secret"}`

	ctx := buildGlobalPropertyEvalCtx(event, e.referencedVars, map[string]bool{"authorization": true}, 0)
	headers, ok := ctx["request.header"].(map[string]string)
	if assert.True(t, ok, "request.header must be a map[string]string in the eval context") {
		assert.Equal(t, "acme", headers["x-tenant-id"], "keys must be lower-cased")
		assert.Equal(t, maskedHeaderValue, headers["authorization"], "masked_headers must still be redacted")
	}
}

// A variable referenced by an expression whose backing event field is nil (never populated by
// this event) must still resolve to its declared zero value rather than being omitted or
// panicking on a nil pointer dereference.
func TestGlobalPropertyEvaluator_ReferencedVariableWithNilEventField_ResolvesToZeroValue(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"targetStatus": "$ctx:target.statusCode",
		"targetDest":   "$ctx:target.destination",
		"appId":        "$ctx:application.id",
	}, nil, 0)
	event := createBaseEvent()
	event.Target = nil
	event.Application = nil

	resolved := e.resolve(event)

	assert.Equal(t, float64(0), resolved["targetStatus"])
	assert.Equal(t, "", resolved["targetDest"])
	assert.Equal(t, "", resolved["appId"])
}

// Properties keep the starting values for a request that carries almost nothing: "" for text,
// 0 for numbers, false, [] and {}.
func TestGlobalPropertyEvaluator_AlmostEmptyEventGetsStartingValues(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"path":          "$ctx:request.path",
		"api":           "$ctx:api.name",
		"app":           "$ctx:application.id",
		"subject":       "$ctx:auth.subject",
		"client":        "$ctx:client.ip",
		"category":      "$ctx:error.category",
		"status":        "$ctx:response.status",
		"code":          "$ctx:error.errorCode",
		"authenticated": "$ctx:auth.authenticated",
		"authorized":    "$ctx:auth.authorized",
		"scopes":        "$ctx:auth.scopes",
		"claims":        "$ctx:auth.property",
		"metadata":      "$ctx:metadata",
		"headers":       "$ctx:request.header",
	}, nil, 0)

	resolved := e.resolve(&dto.Event{Properties: map[string]interface{}{}})

	assert.Equal(t, map[string]interface{}{
		"path": "", "api": "", "app": "", "subject": "", "client": "", "category": "",
		"status": float64(0), "code": float64(0),
		"authenticated": false, "authorized": false,
		"scopes":   []interface{}{},
		"claims":   map[string]interface{}{},
		"metadata": map[string]interface{}{},
		"headers":  map[string]interface{}{},
	}, resolved)
}

// For the same backend error, properties keep "" and 0 for the fields the error does not use.
func TestGlobalPropertyEvaluator_ErrorFieldsTheErrorDoesNotUseKeepStartingValues(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"code":     "$ctx:error.errorCode",
		"type":     "$ctx:error.type",
		"wso2":     "$ctx:error.wso2ErrorCode",
		"policy":   "$ctx:error.policy",
		"original": "$ctx:error.originalStatus",
		"jsonRpc":  "$ctx:error.jsonRpcCode",
		"name":     "$ctx:error.guardrail.name",
	}, nil, 0)

	assert.Equal(t, map[string]interface{}{
		"code": float64(503), "type": "upstream",
		"wso2": float64(0), "policy": "", "original": float64(0), "jsonRpc": float64(0), "name": "",
	}, e.resolve(backendErrorEvent()))
}

// Traffic log templates rely on two cel-go behaviours: evaluating a declared variable that is
// missing from the activation fails, and a variable name whose last part is also a CEL
// function name (type, timestamp) still compiles and evaluates as the variable.
func TestGlobalPropertyEnvAssumptions(t *testing.T) {
	env, err := cel.NewEnv(
		ext.Strings(),
		cel.Variable("error.type", cel.StringType),
		cel.Variable("request.timestamp", cel.StringType),
	)
	require.NoError(t, err)

	compile := func(t *testing.T, expr string) cel.Program {
		t.Helper()
		ast, issues := env.Compile(expr)
		require.NoError(t, issues.Err(), expr)
		program, err := env.Program(ast)
		require.NoError(t, err, expr)
		return program
	}

	t.Run("unbound variable fails evaluation", func(t *testing.T) {
		_, _, err := compile(t, "request.timestamp").Eval(map[string]interface{}{})
		assert.Error(t, err)
	})

	t.Run("names ending in a function name resolve to the variable", func(t *testing.T) {
		vars := map[string]interface{}{
			"error.type":        "authentication",
			"request.timestamp": "2026-10-30T10:15:30.123Z",
		}
		for expr, want := range vars {
			out, _, err := compile(t, expr).Eval(vars)
			require.NoError(t, err, expr)
			assert.Equal(t, want, out.Value(), expr)
		}
	})
}

// fullyPopulatedEvent returns an event that carries a non-zero value for every variable in
// globalPropertyVariables.
func fullyPopulatedEvent() *dto.Event {
	event := createBaseEvent()
	event.Target = &dto.Target{TargetResponseCode: 200, Destination: "backend.internal:8080/resource"}
	event.Application = &dto.Application{
		ApplicationID:    "app-1",
		ApplicationName:  "my-app",
		ApplicationOwner: "alice",
		KeyType:          "PRODUCTION",
	}
	event.ErrorType = "OTHER"
	event.Error = &dto.Error{
		ErrorCode:      446,
		ErrorMessage:   dto.OtherMediationError,
		Wso2ErrorCode:  906201,
		Type:           "guardrail",
		Direction:      "response",
		Summary:        "Response blocked",
		Policy:         "pii-guardrail",
		PolicyPhase:    "response_body",
		Source:         "gateway",
		OriginalStatus: 200,
		Guardrail:      &dto.ErrorGuardrail{Name: "pii", Action: "block", Reason: "email address found"},
		JSONRPCCode:    -32603,
	}
	event.Properties = map[string]interface{}{
		dto.PropKeyOperationPath:    "/resource/{id}",
		dto.PropKeyRequestHeaders:   `{"x-request-id":"req-1"}`,
		dto.PropKeyResponseHeaders:  `{"content-type":"application/json"}`,
		dto.PropKeyAuthUserID:       "alice",
		dto.PropKeyAuthType:         "jwt",
		dto.PropKeyAuthIssuer:       "https://idp.example.com",
		dto.PropKeyAuthCredentialID: "client-1",
		dto.PropKeyAuthTokenID:      "jti-1",
		dto.PropKeyAuthAudience:     "aud1,aud2",
		dto.PropKeyAuthScopes:       "read write",
		dto.PropKeyAuthProperties:   `{"tenant":"acme"}`,
		dto.PropKeyAuthAuthorized:   "true",
		dto.PropKeyMetadata:         `{"region":"eu"}`,
		dto.PropKeyRequestPayload:   `{"order":1}`,
		dto.PropKeyResponsePayload:  `{"id":7}`,
	}
	return event
}

// A variable added to globalPropertyVariables without a binding in buildGlobalPropertyEvalCtx
// would always read as its zero value. With every variable referenced and a fully populated
// event, each one must be bound to a value taken from the event.
func TestBuildGlobalPropertyEvalCtx_FillsEveryVariable(t *testing.T) {
	referenced := make(map[string]bool, len(globalPropertyVariables))
	for _, v := range globalPropertyVariables {
		referenced[v.name] = true
	}

	ctx := buildGlobalPropertyEvalCtx(fullyPopulatedEvent(), referenced, nil, 0)

	for _, v := range globalPropertyVariables {
		value, bound := ctx[v.name]
		if assert.True(t, bound, "%s is not bound", v.name) {
			assert.NotEqual(t, v.zero, value,
				"%s is still its zero value: bind it in buildGlobalPropertyEvalCtx and set it in fullyPopulatedEvent", v.name)
		}
	}
}

// Only these variables are left unbound when a request has no value for them; every other
// variable keeps its empty starting value, so existing traffic_logging.properties keep working.
func TestGlobalPropertyVariables_OptionalSet(t *testing.T) {
	var optional []string
	for _, v := range globalPropertyVariables {
		if v.zero == nil {
			optional = append(optional, v.name)
		}
	}
	assert.ElementsMatch(t, []string{
		"request.timestamp",
		"latencies.duration",
		"latencies.requestMediationLatency",
		"latencies.responseMediationLatency",
		"latencies.backendLatency",
		"request.body",
		"response.body",
	}, optional)
}

func TestGlobalProperties_RequestTimeClientErrorLatenciesAndBodies(t *testing.T) {
	event := fullyPopulatedEvent()
	event.RequestTimestamp = fixedRequestTime
	e := newGlobalPropertyEvaluator(map[string]string{
		"ts":             "$ctx:request.timestamp",
		"ip":             "$ctx:client.ip",
		"ua":             "$ctx:client.userAgent",
		"category":       "$ctx:error.category",
		"code":           "$ctx:error.errorCode",
		"message":        "$ctx:error.errorMessage",
		"wso2Code":       "$ctx:error.wso2ErrorCode",
		"type":           "$ctx:error.type",
		"direction":      "$ctx:error.direction",
		"summary":        "$ctx:error.summary",
		"policy":         "$ctx:error.policy",
		"phase":          "$ctx:error.policyPhase",
		"source":         "$ctx:error.source",
		"originalStatus": "$ctx:error.originalStatus",
		"jsonRpcCode":    "$ctx:error.jsonRpcCode",
		"guardrail":      "$ctx:error.guardrail.name",
		"action":         "$ctx:error.guardrail.action",
		"reason":         "$ctx:error.guardrail.reason",
		"duration":       "$ctx:latencies.duration",
		"reqMediation":   "$ctx:latencies.requestMediationLatency",
		"respMediation":  "$ctx:latencies.responseMediationLatency",
		"backend":        "$ctx:latencies.backendLatency",
		"durationMs":     "$ctx:convertUnit(latencies.duration, 'ms')",
		"reqBody":        "$ctx:request.body",
		"respBody":       "$ctx:response.body",
	}, nil, 5)

	assert.Equal(t, map[string]interface{}{
		"ts":             "2026-10-30T10:15:30.123Z",
		"ip":             "192.168.1.1",
		"ua":             "test-agent",
		"category":       "OTHER",
		"code":           float64(446),
		"message":        "MEDIATION_ERROR",
		"wso2Code":       float64(906201),
		"type":           "guardrail",
		"direction":      "response",
		"summary":        "Response blocked",
		"policy":         "pii-guardrail",
		"phase":          "response_body",
		"source":         "gateway",
		"originalStatus": float64(200),
		"jsonRpcCode":    float64(-32603),
		"guardrail":      "pii",
		"action":         "block",
		"reason":         "email address found",
		"duration":       float64(250000),
		"reqMediation":   float64(50000),
		"respMediation":  float64(30000),
		"backend":        float64(40000),
		"durationMs":     float64(250),
		"reqBody":        `{"ord`,
		"respBody":       `{"id"`,
	}, e.resolve(event))
}

// For a successful request with no timestamp, latencies or captured bodies, the optional
// variables have no value, so their properties are left out; the others are empty.
func TestGlobalProperties_NoValueAndEmptyValues(t *testing.T) {
	event := createBaseEvent()
	event.RequestTimestamp = time.Time{}
	event.TrafficLogLatencies = nil
	e := newGlobalPropertyEvaluator(map[string]string{
		"ts":        "$ctx:request.timestamp",
		"duration":  "$ctx:latencies.duration",
		"backendMs": "$ctx:convertUnit(latencies.backendLatency, 'ms')",
		"reqBody":   "$ctx:request.body",
		"respBody":  "$ctx:response.body",
		"category":  "$ctx:error.category",
		"code":      "$ctx:error.errorCode",
		"guardrail": "$ctx:error.guardrail.name",
	}, nil, 0)

	assert.Equal(t, map[string]interface{}{
		"category":  "",
		"code":      float64(0),
		"guardrail": "",
	}, e.resolve(event))
}

func TestGlobalProperties_ConvertUnitMistakesAreCompileErrors(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		message    string
	}{
		{"unsupported unit", "$ctx:convertUnit(latencies.duration, 'mins')", `convertUnit: unsupported unit "mins" (supported: ns, us, ms, s)`},
		{"unit not a constant", "$ctx:convertUnit(latencies.duration, request.header['unit'])", "convertUnit: the unit must be a constant: 'ns', 'us', 'ms' or 's'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			e := newGlobalPropertyEvaluator(map[string]string{"dur": tt.expression, "env": "prod"}, nil, 0)

			assert.Equal(t, map[string]interface{}{"env": "prod"}, e.resolve(createBaseEvent()), "only the bad property is left out")
			assert.Equal(t, 1, strings.Count(logs.String(), "level=ERROR"), logs.String())
			// The text handler quotes the error, escaping the quotes inside it.
			assert.Contains(t, logs.String(), strings.Trim(strconv.Quote(tt.message), `"`))
		})
	}
}

// A dynamic value that is not an int at run time is an evaluation error for that request,
// not a crash.
func TestGlobalProperties_ConvertUnitOfANonIntValue(t *testing.T) {
	event := createBaseEvent()
	event.Properties[dto.PropKeyMetadata] = `{"region":"eu"}`
	e := newGlobalPropertyEvaluator(map[string]string{"v": "$ctx:convertUnit(metadata.region, 'ms')", "env": "prod"}, nil, 0)

	assert.Equal(t, map[string]interface{}{"env": "prod"}, e.resolve(event))
}
