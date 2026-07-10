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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

func TestNewGlobalPropertyEvaluator_EmptyConfig(t *testing.T) {
	e := newGlobalPropertyEvaluator(nil, nil)
	require.NotNil(t, e)
	assert.Nil(t, e.resolve(createBaseEvent()))
}

func TestGlobalPropertyEvaluator_LiteralPassthrough(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{"env": "prod"}, nil)
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
	}, nil)
	event := createBaseEvent()

	resolved := e.resolve(event)

	assert.Equal(t, "/resource", resolved["path"])
	assert.Equal(t, "GET", resolved["method"])
	assert.Equal(t, "corr-123", resolved["reqId"])
	assert.Equal(t, "test-api", resolved["apiName"])
	assert.Equal(t, "Rest", resolved["apiKind"])
	assert.Equal(t, "project-123", resolved["project"])
}

// response.status and target.statusCode are available to global properties
// even though the log-message policy (which resolves in OnRequestHeaders,
// before any response exists) can never see them.
func TestGlobalPropertyEvaluator_ResolvesResponseAndTargetInfo(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{
		"status":       "$ctx:response.status",
		"targetStatus": "$ctx:target.statusCode",
		"dest":         "$ctx:target.destination",
	}, nil)
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
	}, nil)
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
	}, nil)
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
	}, mask)
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"authorization":"Bearer secret-token"}`
	event.Properties["responseHeaders"] = `{"x-trace-id":"trace-abc"}`

	resolved := e.resolve(event)

	assert.Equal(t, maskedHeaderValue, resolved["token"], "masked header must not leak its raw value via a $ctx: property")
	assert.Equal(t, "trace-abc", resolved["traceId"], "unmasked header still resolves normally")
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
	}, nil)
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
	}, nil)
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
	}, nil)

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
	}, nil)
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
	}, nil)
	event := createBaseEvent()

	resolved := e.resolve(event)

	_, hasBroken := resolved["broken"]
	assert.False(t, hasBroken)
	assert.Equal(t, "GET", resolved["ok"])
}

func TestGlobalPropertyEvaluator_NoPropertiesReturnsNilNotEmptyMap(t *testing.T) {
	e := newGlobalPropertyEvaluator(map[string]string{}, nil)
	assert.Nil(t, e.resolve(createBaseEvent()))
}
