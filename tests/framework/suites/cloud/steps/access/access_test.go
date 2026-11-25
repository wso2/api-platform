/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package access

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// jwtFor builds an unsigned compact JWT carrying the given header and claims.
func jwtFor(t *testing.T, header, claims map[string]any) string {
	t.Helper()
	encodedHeader, err := encodeJWTSegment(header)
	require.NoError(t, err)
	encodedClaims, err := encodeJWTSegment(claims)
	require.NoError(t, err)
	return encodedHeader + "." + encodedClaims + ".c2ln"
}

func TestDecodeJWT(t *testing.T) {
	token := jwtFor(t, map[string]any{"alg": "RS256", "kid": "k1"}, map[string]any{"ouId": "org-1", "exp": 4102444800})
	header, claims, err := decodeJWT(token)
	require.NoError(t, err)
	require.Equal(t, "k1", header["kid"])
	require.Equal(t, "org-1", claims["ouId"])

	for _, malformed := range []string{"", "a.b", "a.b.c.d", "!!!.e30.c2ln", "e30.bm90LWpzb24.c2ln"} {
		_, _, err := decodeJWT(malformed)
		require.Error(t, err, malformed)
		if malformed != "" {
			require.NotContains(t, err.Error(), malformed, "an error must not repeat the token")
		}
	}
}

func TestOrganisationClaimPrefersOuIDThenOrganization(t *testing.T) {
	name, value, ok := organisationClaim(map[string]any{"organization": "o", "ouId": "id"})
	require.True(t, ok)
	require.Equal(t, "ouId", name)
	require.Equal(t, "id", value)

	name, value, ok = organisationClaim(map[string]any{"organization": "o"})
	require.True(t, ok)
	require.Equal(t, "organization", name)
	require.Equal(t, "o", value)

	for _, claims := range []map[string]any{{}, {"ouId": ""}, {"ouId": 7}, {"organization": " "}} {
		_, _, ok = organisationClaim(claims)
		require.False(t, ok, "%v", claims)
	}
}

func TestTokenExpiry(t *testing.T) {
	expiry, ok := tokenExpiry(map[string]any{"exp": float64(4102444800)})
	require.True(t, ok)
	require.Equal(t, int64(4102444800), expiry.Unix())

	_, ok = tokenExpiry(map[string]any{"exp": "soon"})
	require.False(t, ok)
	_, ok = tokenExpiry(map[string]any{})
	require.False(t, ok)
}

func TestForgeTokenKinds(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	realHeader := map[string]any{"alg": "RS256", "kid": "real-key"}
	realClaims := map[string]any{"ouId": "real-org", "ouHandle": "real", "exp": float64(now.Add(time.Hour).Unix())}

	resigned, err := forgeToken(forgeBadSignature, realHeader, realClaims, now)
	require.NoError(t, err)
	header, claims, err := decodeJWT(resigned)
	require.NoError(t, err)
	require.Equal(t, "RS256", header["alg"])
	require.Equal(t, "real-key", header["kid"])
	require.Equal(t, "real-org", claims["ouId"])
	require.Equal(t, float64(now.Add(time.Hour).Unix()), claims["exp"], "the claims are the real ones, so only the signature is wrong")
	require.Equal(t, "real-org", realClaims["ouId"], "the real claims are copied, not modified")
	signed, err := base64.RawURLEncoding.DecodeString(strings.Split(resigned, ".")[2])
	require.NoError(t, err)
	require.Len(t, signed, 256)

	unsigned, err := forgeToken(forgeUnsigned, nil, nil, now)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(unsigned, "."), "an unsigned token has an empty signature")
	header, claims, err = decodeJWT(unsigned)
	require.NoError(t, err)
	require.Equal(t, "none", header["alg"])
	require.NotEqual(t, "real-org", claims["ouId"])
	require.Equal(t, "https://issuer.invalid", claims["iss"])

	rsaToken, err := forgeToken(forgeUnknownKey, nil, nil, now)
	require.NoError(t, err)
	header, _, err = decodeJWT(rsaToken)
	require.NoError(t, err)
	require.Equal(t, "RS256", header["alg"])
	require.True(t, strings.HasPrefix(header["kid"].(string), "unknown-"))
	signature, err := base64.RawURLEncoding.DecodeString(strings.Split(rsaToken, ".")[2])
	require.NoError(t, err)
	require.Len(t, signature, 256)

	hmacToken, err := forgeToken(forgeUnknownSecret, nil, nil, now)
	require.NoError(t, err)
	header, _, err = decodeJWT(hmacToken)
	require.NoError(t, err)
	require.Equal(t, "HS256", header["alg"])
	signature, err = base64.RawURLEncoding.DecodeString(strings.Split(hmacToken, ".")[2])
	require.NoError(t, err)
	require.Len(t, signature, sha256.Size)

	again, err := forgeToken(forgeUnsigned, nil, nil, now)
	require.NoError(t, err)
	require.NotEqual(t, unsigned, again, "each forgery names a fresh organisation")

	_, err = forgeToken("with a typo", nil, nil, now)
	require.ErrorContains(t, err, "unsupported token forgery")
}

func TestAssertTokenIsLiveJWT(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Steps: &cloudbase.Steps{}}
	live := jwtFor(t, map[string]any{"alg": "RS256"}, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, live)
	require.NoError(t, steps.assertTokenIsLiveJWT(ctx))

	for name, token := range map[string]string{
		"expired":     jwtFor(t, map[string]any{"alg": "RS256"}, map[string]any{"exp": float64(time.Now().Add(-time.Minute).Unix())}),
		"no expiry":   jwtFor(t, map[string]any{"alg": "RS256"}, map[string]any{"ouId": "o"}),
		"not a token": "opaque",
	} {
		ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
		t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, token)
		err := steps.assertTokenIsLiveJWT(ctx)
		require.Error(t, err, name)
		require.NotContains(t, err.Error(), token)
	}
}

func TestAssertTokenCarriesOrganisation(t *testing.T) {
	steps := &Steps{Steps: &cloudbase.Steps{}}
	for name, tc := range map[string]struct {
		claims  map[string]any
		wantErr bool
	}{
		"ouId":         {claims: map[string]any{"ouId": "org-1", "ouHandle": "acme"}},
		"organization": {claims: map[string]any{"organization": "org-1"}},
		"none":         {claims: map[string]any{"sub": "user"}, wantErr: true},
	} {
		ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
		t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, jwtFor(t, map[string]any{"alg": "RS256"}, tc.claims))
		err := steps.assertTokenCarriesOrganisation(ctx)
		if tc.wantErr {
			require.Error(t, err, name)
		} else {
			require.NoError(t, err, name)
		}
	}
}

func TestSendWithoutAuthorizationOmitsTheHeaderAndPublishesTheResponse(t *testing.T) {
	var authorization []string
	var method, path string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		authorization, method, path = r.Header.Values("Authorization"), r.Method, r.URL.Path
		w.WriteHeader(http.StatusUnauthorized)
	})
	require.NoError(t, steps.sendWithoutAuthorization(ctx, "post", "/projects"))
	require.Empty(t, authorization)
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "/projects", path)
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusUnauthorized))
}

func TestSendCloudRequestRejectsUnsupportedInput(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, func(http.ResponseWriter, *http.Request) {
		t.Error("no request should be sent")
	})
	require.ErrorContains(t, steps.sendWithoutAuthorization(ctx, "TRACE", "/projects"), "unsupported HTTP method")
	require.ErrorContains(t, steps.sendWithoutAuthorization(ctx, "GET", "projects"), "must start with /")
}

func TestSendWithAuthorizationHeaderSendsTheValueVerbatim(t *testing.T) {
	var got string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	})
	require.NoError(t, steps.sendWithAuthorizationHeader(ctx, "GET", "/projects", "Basic dXNlcjpwYXNz"))
	require.Equal(t, "Basic dXNlcjpwYXNz", got)
}

func TestSendWithForgedTokenSendsABearerThatIsNotTheConsoleToken(t *testing.T) {
	real := jwtFor(t, map[string]any{"alg": "RS256", "kid": "k"}, map[string]any{"ouId": "real-org", "exp": float64(time.Now().Add(time.Hour).Unix())})
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, real)
	var sent string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	})
	for _, kind := range []string{forgeBadSignature, forgeUnsigned, forgeUnknownKey, forgeUnknownSecret} {
		require.NoError(t, steps.sendWithForgedToken(ctx, "GET", "/projects", kind), kind)
		require.True(t, strings.HasPrefix(sent, "Bearer "), kind)
		require.NotEqual(t, "Bearer "+real, sent, kind)
		_, _, err := decodeJWT(strings.TrimPrefix(sent, "Bearer "))
		require.NoError(t, err, kind)
	}
}

func TestSendWithForgedTokenNeedsTheConsoleTokenOnlyForTheBadSignatureKind(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	require.NoError(t, steps.sendWithForgedToken(ctx, "GET", "/projects", forgeUnknownKey))
	require.ErrorContains(t, steps.sendWithForgedToken(ctx, "GET", "/projects", forgeBadSignature), cloudbase.CloudConsoleAccessTokenEnv)
}

func TestSendWithExpiredTokenSkipsWithoutOne(t *testing.T) {
	t.Setenv(cloudConsoleExpiredTokenEnv, "")
	called := false
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusUnauthorized)
	})
	require.ErrorIs(t, steps.sendWithExpiredToken(ctx, "GET", "/projects"), godog.ErrSkip)
	require.False(t, called, "no request is sent without an expired token")
}

func TestSendWithExpiredTokenSendsTheTokenOnlyWhenItHasExpired(t *testing.T) {
	var sent string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	})
	header := map[string]any{"alg": "RS256", "kid": "k"}

	expired := jwtFor(t, header, map[string]any{"exp": float64(time.Now().Add(-time.Hour).Unix())})
	t.Setenv(cloudConsoleExpiredTokenEnv, expired)
	require.NoError(t, steps.sendWithExpiredToken(ctx, "GET", "/projects"))
	require.Equal(t, "Bearer "+expired, sent)

	sent = ""
	live := jwtFor(t, header, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())})
	t.Setenv(cloudConsoleExpiredTokenEnv, live)
	err := steps.sendWithExpiredToken(ctx, "GET", "/projects")
	require.ErrorContains(t, err, "has not expired")
	require.NotContains(t, err.Error(), live)
	require.Empty(t, sent, "a token that is still valid is never sent")

	t.Setenv(cloudConsoleExpiredTokenEnv, jwtFor(t, header, map[string]any{"ouId": "org"}))
	require.ErrorContains(t, steps.sendWithExpiredToken(ctx, "GET", "/projects"), "no exp claim")

	t.Setenv(cloudConsoleExpiredTokenEnv, "not-a-jwt")
	err = steps.sendWithExpiredToken(ctx, "GET", "/projects")
	require.ErrorContains(t, err, "not a readable JWT")
	require.NotContains(t, err.Error(), "not-a-jwt,")
	require.Empty(t, sent)
}

func TestForeignOrganisationHeaderIsIgnoredOrReported(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	honoured := false
	var seen []string
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("X-Org-Id"))
		if honoured && r.Header.Get("X-Org-Id") != "" {
			cloudbase.WriteJSON(w, http.StatusForbidden, map[string]string{"code": "forbidden"})
			return
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]string{"id": "default"})
	})
	require.NoError(t, steps.assertForeignOrganisationHeaderIgnored(ctx, "GET", "/projects/default", "X-Org-Id"))
	require.Len(t, seen, 2)
	require.Empty(t, seen[0])
	require.True(t, strings.HasPrefix(seen[1], "forged-"))
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusOK))

	honoured = true
	require.ErrorContains(t, steps.assertForeignOrganisationHeaderIgnored(ctx, "GET", "/projects/default", "X-Org-Id"), "changed the status")
}

func TestForeignOrganisationHeaderDetectsAChangedBody(t *testing.T) {
	t.Setenv(cloudbase.CloudConsoleAccessTokenEnv, "console-token")
	steps, ctx, _, _ := cloudSteps(t, func(w http.ResponseWriter, r *http.Request) {
		id := "default"
		if r.Header.Get("X-Org-Id") != "" {
			id = "someone-else"
		}
		cloudbase.WriteJSON(w, http.StatusOK, map[string]string{"id": id})
	})
	require.ErrorContains(t, steps.assertForeignOrganisationHeaderIgnored(ctx, "GET", "/projects/default", "X-Org-Id"), "changed the response body")
}

func TestAssertNoInternalDetail(t *testing.T) {
	steps, ctx, _, _ := cloudSteps(t, nil)
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 401, "Unauthorized")
	require.NoError(t, steps.assertNoInternalDetail(ctx))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 401, `{"code":"unauthorized","message":"Invalid or expired credentials."}`)
	require.NoError(t, steps.assertNoInternalDetail(ctx))

	for _, leak := range []string{
		"panic: runtime error", "goroutine 17 [running]", "main.go:42", "upstream connect error", "Stack Trace follows",
	} {
		cloudbase.PublishedResponse(t, steps.Steps, ctx, 401, leak)
		require.ErrorContains(t, steps.assertNoInternalDetail(ctx), "internal detail", leak)
	}
}

func TestResponseAssertions(t *testing.T) {
	steps, ctx, local, _ := cloudSteps(t, nil)
	_, err := httpx.Published(ctx)
	require.Error(t, err, "nothing has been published yet")
	require.Error(t, steps.assertCloudResponseClientError(ctx))

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 500, `{"code":"internal_error"}`)
	require.Error(t, steps.assertCloudResponseClientError(ctx))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 404, `{"code":"not_found","message":"gone"}`)
	require.NoError(t, steps.assertCloudResponseClientError(ctx))

	require.NoError(t, steps.assertCloudResponseContains(ctx, "contain", "gone"))
	require.Error(t, steps.assertCloudResponseContains(ctx, "contain", "present"))
	local.Set("gateway", "gw-1")
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 400, `{"message":"Gateway \"gw-1\" is not in environment"}`)
	require.NoError(t, steps.assertCloudResponseContains(ctx, "contain", `${CTX:gateway}`))

	cloudbase.PublishedResponse(t, steps.Steps, ctx, 409, `{"code":"gateway_set_incomplete","message":"m"}`)
	require.NoError(t, steps.assertCloudErrorCode(ctx, "gateway_set_incomplete"))
	require.Error(t, steps.assertCloudErrorCode(ctx, "build_limit_reached"))
	cloudbase.PublishedResponse(t, steps.Steps, ctx, 500, "not json")
	require.Error(t, steps.assertCloudErrorCode(ctx, "internal_error"))
}

func TestAssertCloudResponseStatusAndErrorCode(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("cloud-runner"))
	steps := &Steps{Steps: &cloudbase.Steps{}}
	require.Error(t, steps.AssertCloudResponseStatus(ctx, http.StatusOK))

	funnel := httpx.NewFunnel(httpx.NewClient(httpx.Options{}), 0, 0)
	require.NoError(t, funnel.Publish(ctx, &httpx.Response{
		StatusCode: http.StatusConflict, Body: []byte(`{"code":"PUBLICATION_STATE_CONFLICT"}`),
	}))
	require.NoError(t, steps.AssertCloudResponseStatus(ctx, http.StatusConflict))
	require.ErrorContains(t, steps.AssertCloudResponseStatus(ctx, http.StatusOK), "expected response status 200")
	require.NoError(t, steps.assertCloudErrorCode(ctx, "PUBLICATION_STATE_CONFLICT"))
	require.ErrorContains(t, steps.assertCloudErrorCode(ctx, "DRAFT_NOT_FOUND"), "expected error code")

	require.NoError(t, funnel.Publish(ctx, &httpx.Response{StatusCode: http.StatusUnauthorized, Body: []byte(`{}`)}))
	require.ErrorContains(t, steps.assertCloudErrorCode(ctx, "UNAUTHORIZED"), "reading the error code")
}

func TestResponseBodyAssertionChecksAbsenceToo(t *testing.T) {
	steps, ctx, _, _ := restAPISteps(t)
	cloudbase.PublishedResponse(t, steps.Steps, ctx, http.StatusBadRequest, `{"code":"DESTINATION_NOT_ALLOWED"}`)
	require.NoError(t, steps.assertCloudResponseContains(ctx, "not contain", "169.254"))
	require.Error(t, steps.assertCloudResponseContains(ctx, "not contain", "DESTINATION"))
}

// cloudSteps wires the area's steps to a stub cloud API.
func cloudSteps(t *testing.T, handler http.HandlerFunc) (*Steps, context.Context, *tcontext.Local, *cleanup.Registry) {
	t.Helper()
	base, ctx, local, registry := cloudbase.CloudSteps(t, handler)
	return &Steps{Steps: base}, ctx, local, registry
}

// restAPISteps wires the area's steps to a stub REST API.
func restAPISteps(t *testing.T) (*Steps, context.Context, *tcontext.Local, *cloudbase.RestAPIStub) {
	t.Helper()
	base, ctx, local, stub := cloudbase.RestAPISteps(t)
	return &Steps{Steps: base}, ctx, local, stub
}
