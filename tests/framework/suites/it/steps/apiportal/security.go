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

package apiportal

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// preLoginSessionKey holds the Cookie header the login page issued, so a later login can be sent
// in the same browser session and the session id compared across it.
const preLoginSessionKey = "apiPortalPreLoginSessionCookie"

// sessionCookieName is the portal's session cookie (portals/api-portal/src/utils/sessionCookies.js).
const sessionCookieName = "connect.sid"

// registerSecuritySteps binds the steps that exercise the portal's security controls: CSRF on
// session writes, session cookie attributes and rotation, and bearer tokens the portal must refuse.
func (s *Steps) registerSecuritySteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send an API Portal session "([^"]*)" request to "([^"]*)" as "([^"]*)" with (no|an invalid) CSRF token and JSON values:$`,
		s.sendSessionRequestWithBadCSRF)
	sc.Step(`^I open the API Portal login page$`, s.openLoginPage)
	sc.Step(`^I submit an API Portal file login for actor "([^"]*)" in the login page's session$`,
		s.submitFileLoginInPreLoginSession)
	sc.Step(`^the API Portal session cookie should differ from the login page's session cookie$`,
		s.sessionCookieRotated)
	sc.Step(`^the API Portal response cookie "([^"]*)" should (have|not have) attribute "([^"]*)"$`,
		s.responseCookieAttribute)
	sc.Step(`^I send an API Portal "([^"]*)" request to "([^"]*)" with a "([^"]*)" bearer token of actor "([^"]*)"$`,
		s.sendWithForgedToken)
	sc.Step(`^I send an unauthenticated API Portal "([^"]*)" request to "([^"]*)" with header "([^"]*)" set to "([^"]*)"$`,
		s.sendUnauthenticatedWithHeader)
}

// sendSessionRequestWithBadCSRF signs the role in and sends a session-authenticated write whose
// CSRF token is either left out or replaced by a well-formed value the session never issued.
func (s *Steps) sendSessionRequestWithBadCSRF(ctx context.Context, method, path, role, mode string, table *godog.Table) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(path, "/api-portal/") {
		return fmt.Errorf("API Portal session path %q must be a full /api-portal/... path", path)
	}
	values, err := tableJSONValues(ctx, table)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encoding API Portal session request body: %w", err)
	}
	base, cookie, csrf, err := s.csrfSession(ctx, role)
	if err != nil {
		return err
	}
	headers := map[string]string{"Cookie": cookie}
	if mode == "an invalid" {
		headers["X-CSRF-Token"] = forgedCSRFToken(csrf)
	}
	_, err = s.portalFunnel().Send(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: portalPublicURL(base, path), Headers: headers,
		Body: payload, ContentType: "application/json",
	})
	if err != nil {
		return fmt.Errorf("API Portal session %s %s: %w", method, path, err)
	}
	return nil
}

// forgedCSRFToken returns a token of the issued token's shape that differs from it in every
// position, so the portal sees a plausible but wrong token.
func forgedCSRFToken(issued string) string {
	forged := []byte(issued)
	for i, c := range forged {
		if c == 'A' {
			forged[i] = 'B'
		} else {
			forged[i] = 'A'
		}
	}
	if len(forged) == 0 {
		return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	return string(forged)
}

// openLoginPage opens the default view's login page without a session, as a browser does before
// signing in, and keeps the cookies it sets for the login that follows.
func (s *Steps) openLoginPage(ctx context.Context) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	base, err := s.portalBase()
	if err != nil {
		return err
	}
	response, err := s.portalFunnel().Send(ctx, httpx.Request{
		Method: http.MethodGet, URL: base + "/api-portal/" + orgHandle + "/views/default/login",
	})
	if err != nil {
		return fmt.Errorf("API Portal login page: %w", err)
	}
	cookie := mergeCookieHeader("", response.Headers.Values("Set-Cookie"))
	if err := tcontext.Set(ctx, preLoginSessionKey, cookie); err != nil {
		return err
	}
	return nil
}

// submitFileLoginInPreLoginSession posts the login form with the cookies the login page set, so the
// portal sees a login inside an existing session. The resulting cookies replace the stored session.
func (s *Steps) submitFileLoginInPreLoginSession(ctx context.Context, role string) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	credentials, err := portalActor(role)
	if err != nil {
		return err
	}
	preLogin, err := tcontext.ResolveString(ctx, preLoginSessionKey)
	if err != nil {
		return fmt.Errorf("open the API Portal login page first: %w", err)
	}
	if cookieValue(preLogin, sessionCookieName) == "" {
		return fmt.Errorf("the API Portal login page set no %s cookie", sessionCookieName)
	}
	base, err := s.portalBase()
	if err != nil {
		return err
	}
	form := url.Values{"username": {credentials.Username}, "password": {credentials.Password}}.Encode()
	response, err := s.portalFunnel().Send(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + "/api-portal/" + orgHandle + "/views/default/login",
		Headers:     map[string]string{"Cookie": preLogin},
		Body:        []byte(form),
		ContentType: "application/x-www-form-urlencoded",
	})
	if err != nil {
		return fmt.Errorf("API Portal file login: %w", err)
	}
	if err := tcontext.Set(ctx, "apiPortalSessionCookie",
		mergeCookieHeader(preLogin, response.Headers.Values("Set-Cookie"))); err != nil {
		return err
	}
	return nil
}

// sessionCookieRotated asserts the session id after login is not the one the login page issued
// (session fixation). Only the comparison result is reported, never the ids.
func (s *Steps) sessionCookieRotated(ctx context.Context) error {
	preLogin, err := tcontext.ResolveString(ctx, preLoginSessionKey)
	if err != nil {
		return err
	}
	current, err := tcontext.ResolveString(ctx, "apiPortalSessionCookie")
	if err != nil {
		return err
	}
	before, after := cookieValue(preLogin, sessionCookieName), cookieValue(current, sessionCookieName)
	if before == "" || after == "" {
		return fmt.Errorf("API Portal %s cookie missing before or after login", sessionCookieName)
	}
	if before == after {
		return fmt.Errorf("API Portal kept the pre-login %s session id after login", sessionCookieName)
	}
	return nil
}

// cookieValue returns a cookie's value from a Cookie header.
func cookieValue(header, name string) string {
	for _, pair := range strings.Split(header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok && key == name {
			return value
		}
	}
	return ""
}

// responseCookieAttribute asserts an attribute of a cookie the published response sets. The
// attribute is a name ("HttpOnly", "SameSite") or name=value ("Path=/api-portal"). Names match
// case-insensitively, as do values other than Path, which browsers compare case-sensitively.
func (s *Steps) responseCookieAttribute(ctx context.Context, name, want, attribute string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	for _, raw := range response.Headers.Values("Set-Cookie") {
		cookieName := strings.TrimSpace(strings.SplitN(raw, "=", 2)[0])
		if cookieName != name {
			continue
		}
		has := cookieHasAttribute(raw, attribute)
		if want == "have" && !has {
			return fmt.Errorf("API Portal cookie %q has no %q attribute", name, attribute)
		}
		if want == "not have" && has {
			return fmt.Errorf("API Portal cookie %q has the %q attribute", name, attribute)
		}
		return nil
	}
	return fmt.Errorf("API Portal response did not set cookie %q", name)
}

// cookieHasAttribute reports whether a Set-Cookie value carries the attribute.
func cookieHasAttribute(setCookie, attribute string) bool {
	wantName, wantValue, withValue := strings.Cut(strings.TrimSpace(attribute), "=")
	parts := strings.Split(setCookie, ";")
	for _, part := range parts[1:] {
		name, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		if !strings.EqualFold(strings.TrimSpace(name), wantName) {
			continue
		}
		if !withValue || cookieAttributeValueMatches(wantName, strings.TrimSpace(value), strings.TrimSpace(wantValue)) {
			return true
		}
	}
	return false
}

// cookieAttributeValueMatches compares an attribute value by the rules for that attribute. RFC 6265
// path-matching is case-sensitive, so a Path differing only in case is a different path.
func cookieAttributeValueMatches(attribute, got, want string) bool {
	if strings.EqualFold(attribute, "Path") {
		return got == want
	}
	return strings.EqualFold(got, want)
}

// sendUnauthenticatedWithHeader sends a request with no credentials and one extra header.
func (s *Steps) sendUnauthenticatedWithHeader(ctx context.Context, method, path, name, value string) error {
	return s.sendPortalWithHeaders(ctx, method, path, "", nil, map[string]string{name: value})
}

// sendWithForgedToken sends a request whose bearer token is derived from the actor's real
// platform-api token. The variants:
//
//	valid              the real token, unchanged
//	re-signed          the real claims re-signed with the control plane's own key (the forging control)
//	expired            the real claims with exp in the past, signed with the control plane's key
//	tampered           the real token with one payload claim changed and the signature kept
//	alg-none           the real claims with alg "none" and no signature
//	hs256-public-key   the real claims signed with HS256 using the control plane's public key as secret
//	foreign-key        the real claims signed with RS256 by a key generated for this request
//
// The tokens are never logged.
func (s *Steps) sendWithForgedToken(ctx context.Context, method, path, variant, role string) error {
	_, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	forged, err := forgeToken(token, strings.ToLower(strings.TrimSpace(variant)), time.Now())
	if err != nil {
		return err
	}
	// The real token is replaced by the forged one; only the forged one reaches the portal.
	return s.sendPortalWithHeaders(ctx, method, path, "", nil, map[string]string{"Authorization": "Bearer " + forged})
}

// forgeToken derives the named variant from a real token, returning an error for a token that is
// not a compact JWS with JSON object header and claims, or for an unknown variant.
func forgeToken(token, variant string, now time.Time) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("the platform-api token is not a compact JWS")
	}
	var header, claims map[string]any
	if err := decodeSegment(parts[0], &header); err != nil {
		return "", fmt.Errorf("decoding the token header: %w", err)
	}
	if header == nil {
		return "", fmt.Errorf("the token header must be a JSON object")
	}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return "", fmt.Errorf("decoding the token claims: %w", err)
	}
	if claims == nil {
		return "", fmt.Errorf("the token claims must be a JSON object")
	}
	cp := shared.ControlPlaneCrypto()
	switch variant {
	case "valid":
		return token, nil
	case "re-signed":
		return signRS256(header, claims, cp["keys/jwt_private.pem"])
	case "expired":
		claims["iat"] = now.Add(-2 * time.Hour).Unix()
		claims["nbf"] = now.Add(-2 * time.Hour).Unix()
		claims["exp"] = now.Add(-1 * time.Hour).Unix()
		return signRS256(header, claims, cp["keys/jwt_private.pem"])
	case "tampered":
		claims["sub"] = fmt.Sprint(claims["sub"]) + "-tampered"
		payload, err := encodeSegment(claims)
		if err != nil {
			return "", err
		}
		return parts[0] + "." + payload + "." + parts[2], nil
	case "alg-none":
		header["alg"] = "none"
		signingInput, err := signingInput(header, claims)
		if err != nil {
			return "", err
		}
		return signingInput + ".", nil
	case "hs256-public-key":
		header["alg"] = "HS256"
		signingInput, err := signingInput(header, claims)
		if err != nil {
			return "", err
		}
		mac := hmac.New(sha256.New, cp["keys/jwt_public.pem"])
		mac.Write([]byte(signingInput))
		return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
	case "foreign-key":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", fmt.Errorf("generating a foreign signing key: %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return "", err
		}
		return signRS256(header, claims, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	default:
		return "", fmt.Errorf("unknown API Portal token variant %q", variant)
	}
}

// decodeSegment decodes one base64url JWS segment holding JSON into out.
func decodeSegment(segment string, out any) error {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// encodeSegment encodes a value as one base64url JSON JWS segment.
func encodeSegment(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// signingInput returns the header.claims text a JWS signature covers.
func signingInput(header, claims map[string]any) (string, error) {
	h, err := encodeSegment(header)
	if err != nil {
		return "", err
	}
	c, err := encodeSegment(claims)
	if err != nil {
		return "", err
	}
	return h + "." + c, nil
}

// signRS256 signs the header and claims with an RSA private key in PKCS#8 PEM.
func signRS256(header, claims map[string]any, privatePEM []byte) (string, error) {
	block, _ := pem.Decode(privatePEM)
	if block == nil {
		return "", fmt.Errorf("the signing key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing the signing key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("the signing key is not an RSA key")
	}
	header["alg"] = "RS256"
	input, err := signingInput(header, claims)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing the token: %w", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
