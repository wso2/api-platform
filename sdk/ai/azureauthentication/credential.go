/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package azureauthentication

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azpolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// rejectionCacheFor is how long a refused token request is answered again without a new one, give
// or take a quarter for jitter. It keeps a misconfigured identity from sending Entra ID or the
// managed identity endpoint a token request for every API request.
const rejectionCacheFor = 2 * time.Second

var (
	// oauthErrorCodePattern matches an OAuth error code such as invalid_client.
	oauthErrorCodePattern = regexp.MustCompile(`^[a-z_]{1,64}$`)

	// responseBodyPattern finds where MSAL quotes a response body it couldn't decode: "raw message
	// was:" for a token request and "json message bytes were:" for another. That body may carry the
	// token it was meant to deliver, under any spelling the decoder accepts as the token field's
	// name: escaped, or in another case. So everything from there on is withheld, however the body
	// is encoded.
	responseBodyPattern = regexp.MustCompile(`(?:raw message was|json message bytes were):`)

	// tokenFieldPattern and jwtPattern find tokens in the rest of the SDK's error text, should it
	// quote a body some other way. tokenFieldPattern finds a token field's name in any case, however
	// many times its quotes are escaped.
	tokenFieldPattern = regexp.MustCompile(`(?i)\\*"(?:access_token|refresh_token|id_token)\\*"`)
	jwtPattern        = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)

	// errNoToken is a token response that succeeded without a token, which MSAL's managed identity
	// client passes on as a success.
	errNoToken = errors.New("the token response has no access token")

	// credentials holds the credentials shared by every policy in the process.
	credentials = newCredentialRegistry()
)

// Get returns the gateway-wide credential for id, building it on first use, so every caller with
// the same identity shares one token cache. It fails when id is invalid or the host can't provide
// the identity, such as Container Apps without IDENTITY_ENDPOINT.
func Get(id Identity) (*Credential, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return credentials.get(id)
}

// credentialRegistry holds one credential per identity, so every route, API and policy that uses an
// identity shares its token cache, across config pushes too.
type credentialRegistry struct {
	mu         sync.Mutex
	byIdentity map[Identity]*Credential
}

func newCredentialRegistry() *credentialRegistry {
	return &credentialRegistry{byIdentity: make(map[Identity]*Credential)}
}

// get returns the credential for id, building it the first time.
func (r *credentialRegistry) get(id Identity) (*Credential, error) {
	kind := kindOf(id.Type)
	if kind == nil {
		return nil, fmt.Errorf("unknown authentication type %q", id.Type)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.byIdentity[id]; ok {
		return c, nil
	}
	sdkCredential, err := kind.newCredential(id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", kind.unavailable, err)
	}
	c := &Credential{
		sdk:        sdkCredential,
		kind:       kind,
		slots:      make(map[string]chan struct{}),
		rejections: make(map[string]rejection),
	}
	r.byIdentity[id] = c
	return c, nil
}

// Credential is an identity's SDK credential, which caches and refreshes its tokens, and the token
// requests it was refused recently, by scope. It is safe for concurrent use.
type Credential struct {
	sdk  azcore.TokenCredential
	kind *authenticationKind

	mu         sync.Mutex
	slots      map[string]chan struct{} // see slot
	rejections map[string]rejection
}

// Service names the token service for logs, such as "Microsoft Entra ID".
func (c *Credential) Service() string { return c.kind.service }

// slot returns the admission slot a GetToken for scope waits for, one caller at a time. An SDK
// that fetches one token at a time per credential (oneFetchAtATime) does so behind a lock that
// doesn't watch the caller's context, so all its scopes share one slot and the wait honours the
// caller's timeout. Otherwise each scope has its own slot, and a cached token never waits for
// another scope's fetch.
func (c *Credential) slot(scope string) chan struct{} {
	key := scope
	if c.kind.oneFetchAtATime {
		key = ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.slots[key]
	if !ok {
		s = make(chan struct{}, 1)
		c.slots[key] = s
	}
	return s
}

type rejection struct {
	err   *TokenError
	until time.Time
}

// TokenError is a failed token request.
type TokenError struct {
	// Reason is a short description of the failure: the token service, and the HTTP status and
	// OAuth error code it answered with, never its error description, which can name the tenant,
	// the application and the resource. It is safe for a fault's description or the log.
	Reason string

	// Rejected reports that the service refused the request itself: a 4xx other than 408 and 429
	// (and other than IMDS's 404 and 410, which it answers while it starts or updates).
	Rejected bool

	// Cached reports that the failure was answered from the rejection cache, without a new
	// request, so a caller can log it once rather than for every request.
	Cached bool

	cause error // the SDK's error, which only Error gives, with tokens withheld
}

// Error is Reason followed by the SDK's error with any response body it quotes withheld, everything
// from its first token field on withheld, and any JWT before them redacted. It is for logs only: the
// SDK's error can name the tenant, the application and the resource.
func (e *TokenError) Error() string {
	if e.cause == nil {
		return e.Reason
	}
	return e.Reason + ": " + e.loggable()
}

// Unwrap returns the SDK's error, for errors.Is and errors.As. Its own text is not redacted: log
// the TokenError itself, never what Unwrap returns.
func (e *TokenError) Unwrap() error { return e.cause }

// loggable is the SDK's error text with the response body MSAL quotes withheld, everything after
// the first token field's name withheld, and any JWT before them replaced by [REDACTED].
// Withholding the rest, rather than the field's value, holds for a body cut off inside the token,
// which has no closing quote to end the value at.
func (e *TokenError) loggable() string {
	text := e.cause.Error()
	if loc := responseBodyPattern.FindStringIndex(text); loc != nil {
		text = text[:loc[1]] + " [REDACTED] (the response body is withheld)"
	}
	if loc := tokenFieldPattern.FindStringIndex(text); loc != nil {
		text = text[:loc[1]] + ": [REDACTED] (the rest of the error is withheld)"
	}
	return jwtPattern.ReplaceAllString(text, "[REDACTED]")
}

// Token returns an access token for scope, obtained within timeout, or DefaultTokenTimeout when
// timeout is not positive. scope must pass ValidateScope. A failure is a *TokenError; a request the
// service refused is answered from the rejection cache for about two seconds.
func (c *Credential) Token(ctx context.Context, scope string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultTokenTimeout
	}
	token, terr := c.token(ctx, scope, timeout)
	if terr != nil {
		return "", terr
	}
	return token, nil
}

func (c *Credential) token(ctx context.Context, scope string, timeout time.Duration) (string, *TokenError) {
	if err := c.recentRejection(scope); err != nil {
		return "", err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	slot := c.slot(scope)
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-fetchCtx.Done():
		return "", c.describe(fetchCtx, fetchCtx.Err())
	}
	// A caller that waited behind a refused request takes its answer instead of asking again.
	if err := c.recentRejection(scope); err != nil {
		return "", err
	}

	tok, err := c.sdk.GetToken(fetchCtx, azpolicy.TokenRequestOptions{Scopes: []string{scope}})
	if err == nil && tok.Token == "" {
		err = errNoToken
	}
	if err == nil {
		return tok.Token, nil
	}

	terr := c.describe(fetchCtx, err)
	if terr.Rejected {
		c.remember(scope, terr)
	}
	return "", terr
}

// describe gives a failed GetToken its reason. The SDK reports most failures as one error type
// with the response attached, so the reason comes from the status and the OAuth error code.
func (c *Credential) describe(fetchCtx context.Context, err error) *TokenError {
	var failed *azidentity.AuthenticationFailedError
	isAuthFailure := errors.As(err, &failed)
	service := c.kind.service
	switch {
	case errors.Is(err, errNoToken):
		return &TokenError{Reason: service + " returned an unusable response", cause: err}
	case fetchCtx.Err() != nil:
		return &TokenError{Reason: "timed out obtaining an access token from " + service, cause: err}
	case !isAuthFailure && c.kind.imds:
		// The managed identity credential reports every failure as an AuthenticationFailedError,
		// except IMDS refusing the request without a usable response: its 400 for the host's
		// default identity when the host has several, or a local proxy's 403.
		return &TokenError{Reason: service + " rejected the token request", cause: err, Rejected: true}
	case isAuthFailure && failed.RawResponse != nil:
		status, code := failed.RawResponse.StatusCode, oauthErrorCode(failed.RawResponse)
		if c.refuses(status) {
			detail := fmt.Sprintf("HTTP %d", status)
			if code != "" {
				detail += ", " + code
			}
			return &TokenError{Reason: fmt.Sprintf("%s rejected the token request (%s)", service, detail), cause: err, Rejected: true}
		}
		reason := fmt.Sprintf("%s returned HTTP %d", service, status)
		if code != "" {
			reason += " (" + code + ")"
		}
		return &TokenError{Reason: reason, cause: err}
	default:
		return &TokenError{Reason: "could not obtain an access token from " + service, cause: err}
	}
}

// refuses reports whether status means the service refused the request itself, rather than a
// failure that can clear on its own: a timeout, throttling, or IMDS starting up or updating (404
// and 410).
func (c *Credential) refuses(status int) bool {
	switch {
	case status < 400 || status >= 500 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests:
		return false
	case c.kind.imds && (status == http.StatusNotFound || status == http.StatusGone):
		return false
	default:
		return true
	}
}

// oauthErrorCode returns the OAuth error code in resp's body, such as invalid_client, or "" if the
// body has none. The Container Apps identity endpoint passes Entra ID's code on as errorCode. The
// description and message are left out: they can name the tenant, the application and the resource.
func oauthErrorCode(resp *http.Response) string {
	body, err := runtime.Payload(resp)
	if err != nil {
		return ""
	}
	var parsed struct {
		Error     string `json:"error"`
		ErrorCode string `json:"errorCode"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return ""
	}
	code := cmp.Or(parsed.Error, parsed.ErrorCode)
	if !oauthErrorCodePattern.MatchString(code) {
		return ""
	}
	return code
}

// recentRejection returns the cached rejection of scope, or nil if there is none or it expired.
func (c *Credential) recentRejection(scope string) *TokenError {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.rejections[scope]
	if !ok {
		return nil
	}
	if !time.Now().Before(r.until) {
		delete(c.rejections, scope)
		return nil
	}
	cached := *r.err
	cached.Cached = true
	return &cached
}

func (c *Credential) remember(scope string, err *TokenError) {
	jitter := rand.N(rejectionCacheFor/2 + 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rejections[scope] = rejection{err: err, until: time.Now().Add(rejectionCacheFor*3/4 + jitter)}
}
