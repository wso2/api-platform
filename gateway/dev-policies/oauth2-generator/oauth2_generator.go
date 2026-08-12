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

package oauth2generator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	xoauth2 "golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	// defaultTokenRequestTimeout bounds how long a single token-endpoint
	// HTTP call is allowed to take. Without this, golang.org/x/oauth2 falls
	// back to http.DefaultClient, which has no timeout at all, so a hung
	// IdP would block a token fetch indefinitely.
	defaultTokenRequestTimeout = 10 * time.Second

	// defaultTokenTTLFallback is applied when the token endpoint's response
	// omits expires_in. golang.org/x/oauth2 leaves Token.Expiry as the zero
	// value in that case, which Token.Valid() always treats as
	// already-expired - without this fallback, the token would never be
	// cached and every request would refetch it.
	defaultTokenTTLFallback = time.Hour

	// defaultHeaderName is the header the generated credential is injected
	// into when headerName is omitted.
	defaultHeaderName = "Authorization"

	// defaultValuePrefix is prepended (followed by a single space) to the
	// credential value when valuePrefix is omitted. An explicitly configured
	// empty string is honored as "no prefix" rather than falling back to
	// this - see getStringParamOrDefault.
	defaultValuePrefix = "Bearer"

	// defaultTokenRequestMaxRetries bounds how many additional attempts
	// resilientTokenSource makes after the initial token-endpoint call
	// fails with a transient error - see isRetryableTokenError.
	defaultTokenRequestMaxRetries = 2

	// retryBaseDelay/retryMaxDelay bound resilientTokenSource's backoff
	// between attempts - exponential from retryBaseDelay, capped at
	// retryMaxDelay, with up to 50% jitter to avoid every replica retrying
	// in lockstep against the same struggling identity provider.
	retryBaseDelay = 100 * time.Millisecond
	retryMaxDelay  = 2 * time.Second

	// defaultExpiryBuffer is how far ahead of a token's actual expiry it's
	// treated as no-longer-fresh, both by the caching layer (token_cache.go's
	// tokenFreshEnough) and by the token-endpoint source itself (via
	// xoauth2.ReuseTokenSourceWithExpiry below) - see expiryBuffer's field
	// comment. 30s comfortably covers a request's own in-flight time to the
	// backend, well beyond golang.org/x/oauth2's own hardcoded 10s default.
	defaultExpiryBuffer = 30 * time.Second
)

// defaultPurgeStatusCodes is applied when tokenPurgeStatusCodes is
// omitted. 401 is the standard signal (RFC 6750 Section 3) that a bearer
// token was rejected as invalid - as opposed to e.g. 403, which usually
// means insufficient scope for an otherwise-valid token and would gain
// nothing from a purge.
var defaultPurgeStatusCodes = []int{http.StatusUnauthorized}

const (
	// GrantTypeClientCredentials (RFC 6749 Section 4.4) is the standard
	// machine-to-machine grant and should be preferred whenever the
	// upstream identity provider supports it.
	GrantTypeClientCredentials = "client_credentials"

	// GrantTypePassword (RFC 6749 Section 4.3) bridges legacy IdPs that only
	// expose this grant - discouraged for new integrations since it requires
	// handling the resource owner's raw credentials directly.
	GrantTypePassword = "password"

	// AuthType is the AuthContext.AuthType for the token-endpoint path. The
	// specific grant is available separately via Properties["grantType"].
	AuthType = "oauth2"

	// AuthTypeStaticToken is the AuthContext.AuthType for the
	// directly-supplied-token path - no OAuth2 grant involved.
	AuthTypeStaticToken = "static-token"

	// ModeTokenEndpoint and ModeStaticToken identify which mutually exclusive
	// auth path a policy instance was configured for.
	ModeTokenEndpoint = "token-endpoint"
	ModeStaticToken   = "static-token"

	// ClientAuthMethodBasic (client_secret_basic) sends client ID/secret via HTTP
	// Basic auth - RFC 6749's preferred convention, and this policy's default.
	ClientAuthMethodBasic = "client_secret_basic"

	// ClientAuthMethodPost (client_secret_post) sends client ID/secret as form
	// fields instead of the Basic header.
	ClientAuthMethodPost = "client_secret_post"
)

// oauth2Params bundles all extracted, validated policy params. Passed as a
// single struct (rather than positional args) now that the set has grown
// with grantType-conditional fields (username/password) — positional args
// for six-plus mostly-string fields invite mixed-up-order bugs.
type oauth2Params struct {
	// bearerToken is the directly-supplied credential for the static-token
	// auth path. Non-empty here means the whole token-endpoint block below
	// is unused - see validateAndExtractParams for the mutual-exclusion
	// check.
	bearerToken string

	grantType        string
	tokenEndpoint    string
	clientID         string
	clientSecret     string
	clientAuthMethod string
	username         string
	password         string

	// For client_credentials, EndpointParams carries the whole map into the token
	// request body. For password, only "scope" has any effect (mapped to
	// Config.Scopes) - every other key is ignored - see buildTokenSource.
	tokenRequestParams map[string]string

	// Extra HTTP headers sent with the token-endpoint request, on top of
	// clientAuthMethod/the grant's own headers. "Authorization"/"Content-Type"
	// are dropped rather than honored - see headerInjectingRoundTripper.
	tokenRequestHeaders map[string]string

	// requestTimeout bounds the token-endpoint HTTP call - see
	// defaultTokenRequestTimeout.
	requestTimeout time.Duration

	// tokenTTLFallback is applied by the caching layer when the token
	// endpoint's response omits expires_in - see defaultTokenTTLFallback.
	tokenTTLFallback time.Duration

	// expiryBuffer is how far ahead of a token's actual expiry it's treated
	// as stale, so a request never gets sent upstream with a credential that
	// expires mid-flight or a hair before the backend even sees it. Applied
	// both to the caching layer's own local/Redis freshness check
	// (token_cache.go's tokenFreshEnough) and to the token-endpoint source's
	// own reuse behavior (see buildTokenSource's use of
	// xoauth2.ReuseTokenSourceWithExpiry) - both must agree on the same
	// threshold, or the cache layer would decide a token is stale while the
	// token source itself still considers its own cached copy fresh and
	// hands back the very token being avoided. Unused when bearerToken is
	// set - see defaultExpiryBuffer.
	expiryBuffer time.Duration

	// purgeStatusCodes are the upstream response status codes that purge the
	// cached token - see OnResponseHeaders and defaultPurgeStatusCodes.
	// Forced empty by GetPolicy when bearerToken is set - see its comment.
	purgeStatusCodes map[int]struct{}

	// headerName and valuePrefix control where and how the credential
	// (fetched or directly-supplied) is injected - see buildHeaderValue.
	// Apply identically to both auth paths.
	headerName  string
	valuePrefix string

	// proxyURL/tlsCaCertPath/tlsInsecureSkipVerify configure the
	// token-endpoint HTTP client's Transport - see
	// buildTokenEndpointTransport. Unused when bearerToken is set.
	proxyURL              string
	tlsCaCertPath         string
	tlsInsecureSkipVerify bool

	// tokenRequestMaxRetries bounds resilientTokenSource's retry of the
	// token-endpoint fetch - see defaultTokenRequestMaxRetries. Unused when
	// bearerToken is set.
	tokenRequestMaxRetries int
}

// mode reports which of the two mutually exclusive auth paths p represents -
// see ModeTokenEndpoint/ModeStaticToken.
func (p oauth2Params) mode() string {
	if p.bearerToken != "" {
		return ModeStaticToken
	}
	return ModeTokenEndpoint
}

// Policy generates an upstream credential - either fetched via an OAuth2 grant
// (client_credentials or password) or a directly-supplied static one - and
// injects it into a configurable request header before forwarding.
type Policy struct {
	// mode records which of the two mutually exclusive auth paths this
	// instance was configured for - see ModeTokenEndpoint/ModeStaticToken.
	mode string

	grantType        string
	tokenEndpoint    string
	clientID         string
	clientAuthMethod string

	// headerName and valuePrefix control where and how the credential is
	// injected - see buildHeaderValue.
	headerName  string
	valuePrefix string

	// tokenSource supplies the credential to inject: a *redisCachingTokenSource
	// (token_cache.go) for the token-endpoint path, or a *staticTokenSource that
	// always returns the configured token as-is.
	tokenSource tokenProvider

	// Test seam — production code calls tokenSource.Token() directly; unit
	// tests override this to avoid a real network call to a token endpoint,
	// mirroring the retrieveCredentialsFunc pattern used in the
	// aws-authentication policy.
	tokenFunc func() (*xoauth2.Token, error)

	// purgeStatusCodes are the upstream response status codes that purge the
	// cached token via tokenSource.Purge() - see OnResponseHeaders. Empty
	// (explicitly set to [], or always for the static-token path - see
	// GetPolicy) disables response-phase processing entirely - see Mode().
	purgeStatusCodes map[int]struct{}
}

// staticTokenSource always returns the same configured token, no endpoint call or
// caching involved. Expiry is left at its zero value deliberately - Token.expired()
// treats that as "never expires", right for a credential with no expiry of its own.
// Purge is a no-op: nothing cached to clear, no fresher token to fetch.
type staticTokenSource struct {
	bearerToken string
}

func (s *staticTokenSource) Token() (*xoauth2.Token, error) {
	return &xoauth2.Token{AccessToken: s.bearerToken, TokenType: "Bearer"}, nil
}

func (s *staticTokenSource) Purge() {}

// buildHeaderValue combines valuePrefix and the credential into the header
// value to inject - e.g. buildHeaderValue("Bearer", "abc") -> "Bearer abc".
// An empty prefix (explicitly configured - see getStringParamOrDefault)
// yields the raw credential with no scheme prefix.
func buildHeaderValue(prefix, token string) string {
	if prefix == "" {
		return token
	}
	return prefix + " " + token
}

// GetPolicy is the v1alpha2 factory entry point (loaded by v1alpha2 kernels).
// metadata is part of the v1alpha2 factory signature but unused here: the
// Redis cache key is derived entirely from params (see
// oauth2ConfigDiscriminator in token_cache.go).
func GetPolicy(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
	slog.Debug("OAuth2Generator: constructing policy from params")

	p, err := validateAndExtractParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	mode := p.mode()
	slog.Debug("OAuth2Generator: validated params", "mode", mode,
		"grantType", p.grantType, "tokenEndpoint", p.tokenEndpoint, "clientId", p.clientID,
		"clientAuthMethod", p.clientAuthMethod, "headerName", p.headerName)

	var tokenSource tokenProvider
	if mode == ModeStaticToken {
		// Nothing to fetch or cache - the configured token is injected as-is
		// on every request. Also force purgeStatusCodes off: there is no
		// fresher token to fetch on a rejection, and leaving it configured
		// would otherwise turn on response-header processing (see Mode())
		// for a purge that Purge() no-ops anyway.
		tokenSource = &staticTokenSource{bearerToken: p.bearerToken}
		p.purgeStatusCodes = map[int]struct{}{}
	} else {
		innerSource, err := buildTokenSource(p)
		if err != nil {
			return nil, err
		}
		tokenSource, err = newRedisCachingTokenSource(innerSource, extractCacheParams(params), p)
		if err != nil {
			return nil, err
		}
	}

	pol := &Policy{
		mode:             mode,
		grantType:        p.grantType,
		tokenEndpoint:    p.tokenEndpoint,
		clientID:         p.clientID,
		clientAuthMethod: p.clientAuthMethod,
		headerName:       p.headerName,
		valuePrefix:      p.valuePrefix,
		tokenSource:      tokenSource,
		purgeStatusCodes: p.purgeStatusCodes,
	}
	pol.tokenFunc = pol.tokenSource.Token

	slog.Debug("OAuth2Generator: policy initialized", "mode", pol.mode,
		"grantType", pol.grantType, "tokenEndpoint", pol.tokenEndpoint, "clientId", pol.clientID,
		"clientAuthMethod", pol.clientAuthMethod, "headerName", pol.headerName)

	return pol, nil
}

// buildTokenSource constructs the token source for the given grantType.
// This is the extension point for future grants: each grant gets its own
// case here, building whatever xoauth2.TokenSource fits that grant's flow.
func buildTokenSource(p oauth2Params) (xoauth2.TokenSource, error) {
	authStyle := authStyleFor(p.clientAuthMethod)

	// Both grants forward ctx down to internal.RetrieveToken, which resolves
	// the *http.Client via internal.ContextClient(ctx) - falling back to
	// http.DefaultClient (no timeout) if unset. Injecting a bounded client
	// here, once, covers both grants; the Transport itself is shared across
	// policy instances/rebuilds - see getOrCreateTokenEndpointTransport.
	transport, err := getOrCreateTokenEndpointTransport(p)
	if err != nil {
		return nil, fmt.Errorf("invalid token endpoint transport config: %w", err)
	}
	// Neither Config type exposes a hook for arbitrary extra headers (unlike
	// EndpointParams for body fields), so tokenRequestHeaders needs a
	// RoundTripper wrapper instead.
	var roundTripper http.RoundTripper = transport
	if len(p.tokenRequestHeaders) > 0 {
		roundTripper = &headerInjectingRoundTripper{headers: p.tokenRequestHeaders, next: transport}
	}
	httpClient := &http.Client{Timeout: p.requestTimeout, Transport: roundTripper}
	ctx := context.WithValue(context.Background(), xoauth2.HTTPClient, httpClient)

	switch p.grantType {
	case GrantTypeClientCredentials:
		cfg := &clientcredentials.Config{
			ClientID:     p.clientID,
			ClientSecret: p.clientSecret,
			TokenURL:     p.tokenEndpoint,
			AuthStyle:    authStyle,
			// EndpointParams carries tokenRequestParams (e.g. scope) verbatim into
			// the token request body - golang.org/x/oauth2/clientcredentials
			// exposes this hook directly, so client_credentials keeps using
			// the library's own request/response handling untouched.
			EndpointParams: toURLValues(p.tokenRequestParams),
		}
		// cfg.TokenSource(ctx) would wrap this in the library's own
		// ReuseTokenSource, hardcoded to a 10s early-expiry buffer - not
		// configurable, and not the same threshold the caching layer uses
		// (see expiryBuffer's field comment). cfg.Token(ctx) is the same
		// underlying fetch with no caching of its own, so wrapping it
		// ourselves in ReuseTokenSourceWithExpiry(p.expiryBuffer) gets the
		// identical single-flight/mutex behavior with a matching threshold.
		raw := tokenFetcherFunc(func() (*xoauth2.Token, error) { return cfg.Token(ctx) })
		return xoauth2.ReuseTokenSourceWithExpiry(nil, raw, p.expiryBuffer), nil

	case GrantTypePassword:
		cfg := &xoauth2.Config{
			ClientID:     p.clientID,
			ClientSecret: p.clientSecret,
			Endpoint: xoauth2.Endpoint{
				TokenURL:  p.tokenEndpoint,
				AuthStyle: authStyle,
			},
		}
		// PasswordCredentialsToken hardcodes grant_type/username/password plus
		// scope (if Config.Scopes is set) - scope is the only
		// tokenRequestParams entry this grant can honor; anything else
		// (audience, resource, ...) has no effect. Space-delimited per RFC
		// 6749 Section 3.3.
		if scope := p.tokenRequestParams["scope"]; scope != "" {
			cfg.Scopes = strings.Fields(scope)
		}
		src := &passwordTokenSource{
			ctx:      ctx,
			cfg:      cfg,
			username: p.username,
			password: p.password,
		}
		// TokenSource(ctx, initialToken) only refreshes via refresh_token,
		// which the password grant's response may not include, so
		// passwordTokenSource re-authenticates directly instead.
		// ReuseTokenSourceWithExpiry gives it the same caching/mutex-safety
		// clientcredentials gets internally, using the same configurable
		// expiryBuffer threshold rather than the library's hardcoded 10s
		// default.
		return xoauth2.ReuseTokenSourceWithExpiry(nil, src, p.expiryBuffer), nil

	default:
		// Unreachable - validateAndExtractParams already rejects any other
		// value - kept as an explicit guard for a future added grant.
		return nil, fmt.Errorf("unsupported grantType %q", p.grantType)
	}
}

// tokenFetcherFunc adapts a plain fetch function to xoauth2.TokenSource, so a
// Config's uncached single-shot Token(ctx) method (rather than its own
// TokenSource(ctx), which bundles in the library's hardcoded-10s
// ReuseTokenSource) can be wrapped in our own ReuseTokenSourceWithExpiry -
// see buildTokenSource's client_credentials case.
type tokenFetcherFunc func() (*xoauth2.Token, error)

func (f tokenFetcherFunc) Token() (*xoauth2.Token, error) { return f() }

// tokenEndpointTransportKey identifies a distinct token-endpoint HTTP
// client configuration - proxy and TLS settings, the only things that
// affect the *http.Transport itself (Timeout lives on *http.Client, not
// the Transport, so it doesn't need to be part of this key).
type tokenEndpointTransportKey struct {
	proxyURL              string
	tlsCaCertPath         string
	tlsInsecureSkipVerify bool
}

// tokenEndpointTransports is the process-wide registry of shared *http.Transport
// values for the token-endpoint HTTP client, keyed by proxy/TLS configuration -
// same connection-pool-fragmentation reason as sdk/core/utils/redisclient's own
// registry, which this policy's own Redis tier now delegates to instead of
// keeping a local copy (see token_cache.go's use of redisclient.Resolve).
var tokenEndpointTransports = struct {
	mu sync.Mutex
	m  map[tokenEndpointTransportKey]*http.Transport
}{m: make(map[tokenEndpointTransportKey]*http.Transport)}

// getOrCreateTokenEndpointTransport returns the process-wide shared
// Transport for this proxy/TLS configuration, building it on first use.
func getOrCreateTokenEndpointTransport(p oauth2Params) (*http.Transport, error) {
	key := tokenEndpointTransportKey{
		proxyURL:              p.proxyURL,
		tlsCaCertPath:         p.tlsCaCertPath,
		tlsInsecureSkipVerify: p.tlsInsecureSkipVerify,
	}

	tokenEndpointTransports.mu.Lock()
	defer tokenEndpointTransports.mu.Unlock()

	if t, ok := tokenEndpointTransports.m[key]; ok {
		return t, nil
	}

	t, err := buildTokenEndpointTransport(p)
	if err != nil {
		return nil, err
	}
	tokenEndpointTransports.m[key] = t
	return t, nil
}

// buildTokenEndpointTransport wires proxyURL and TLS settings into one Transport
// deliberately, setting Proxy explicitly alongside TLSClientConfig: an unset
// Transport.Proxy means "never proxy" (it does NOT inherit ProxyFromEnvironment) -
// exactly how jwt-auth and opaque-token-auth each lost proxy support the moment a
// custom TLS config was added, by setting only one of the two.
func buildTokenEndpointTransport(p oauth2Params) (*http.Transport, error) {
	proxyFunc := http.ProxyFromEnvironment
	if p.proxyURL != "" {
		proxyURL, err := url.Parse(p.proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxyURL: %w", err)
		}
		proxyFunc = http.ProxyURL(proxyURL)
	}

	transport := &http.Transport{Proxy: proxyFunc}

	if p.tlsCaCertPath != "" || p.tlsInsecureSkipVerify {
		tlsConfig := &tls.Config{InsecureSkipVerify: p.tlsInsecureSkipVerify} //nolint:gosec // opt-in, logged at extraction time
		if p.tlsCaCertPath != "" {
			pool, err := loadCACertPool(p.tlsCaCertPath)
			if err != nil {
				return nil, fmt.Errorf("tlsCaCertPath: %w", err)
			}
			tlsConfig.RootCAs = pool
		}
		transport.TLSClientConfig = tlsConfig
	}

	return transport, nil
}

// loadCACertPool reads a PEM-encoded CA certificate file and returns a pool
// containing it, for TLS connections that must trust a private/internal CA
// - used by buildTokenEndpointTransport above, its only caller.
func loadCACertPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert file %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("no valid PEM certificates found in %q", path)
	}
	return pool, nil
}

// headerInjectingRoundTripper adds tokenRequestHeaders to every request before
// delegating to next - the only interception point available, since neither Config
// type exposes a hook for arbitrary headers. "Authorization"/"Content-Type" are
// never set here even if present: both are already managed by clientAuthMethod and
// the grant's own request encoding, and overriding either would break the request.
type headerInjectingRoundTripper struct {
	headers map[string]string
	next    http.RoundTripper
}

func (h *headerInjectingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone rather than mutate: req is owned by the caller (golang.org/x/oauth2's
	// internal.RetrieveToken), which may reuse or inspect it after this call.
	req = req.Clone(req.Context())
	for k, v := range h.headers {
		if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Content-Type") {
			continue
		}
		req.Header.Set(k, v)
	}
	return h.next.RoundTrip(req)
}

// resilientTokenSource wraps a real, IDP-fetching xoauth2.TokenSource with bounded
// retry, for transient failures only - see isRetryableTokenError.
type resilientTokenSource struct {
	inner      xoauth2.TokenSource
	maxRetries int
}

func (r *resilientTokenSource) Token() (*xoauth2.Token, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBackoff(attempt))
		}
		tok, err := r.inner.Token()
		if err == nil {
			return tok, nil
		}
		lastErr = err
		if !isRetryableTokenError(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// isRetryableTokenError classifies a token-fetch error as worth retrying. An
// *xoauth2.RetrieveError is only retryable on 429/5xx - a 4xx like invalid_client is
// a rejected request retrying can't fix. Any other error (DNS, connection refused,
// TLS, deadline exceeded) never reached the IdP at all, so it's treated as transient.
func isRetryableTokenError(err error) bool {
	var retrieveErr *xoauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		if retrieveErr.Response == nil {
			return true
		}
		status := retrieveErr.Response.StatusCode
		return status == http.StatusTooManyRequests || status >= 500
	}
	return true
}

// retryBackoff returns the delay before retry attempt n (n >= 1):
// exponential from retryBaseDelay, capped at retryMaxDelay, plus up to 50%
// jitter so that many gateway-runtime replicas whose tokens expire around
// the same time don't all retry against a struggling identity provider in
// lockstep.
func retryBackoff(attempt int) time.Duration {
	backoff := retryBaseDelay * time.Duration(uint64(1)<<uint(attempt-1))
	if backoff > retryMaxDelay || backoff <= 0 {
		backoff = retryMaxDelay
	}
	jitter := time.Duration(rand.Int63n(int64(backoff)/2 + 1))
	return backoff + jitter
}

// authStyleFor maps clientAuthMethod to the xoauth2.AuthStyle both Config types
// consume identically: AuthStyleInHeader sends client_id/secret via HTTP Basic
// (client_secret_basic); AuthStyleInParams sends them as form fields
// (client_secret_post). validateAndExtractParams already rejects any other value,
// so the default case is unreachable.
func authStyleFor(method string) xoauth2.AuthStyle {
	switch method {
	case ClientAuthMethodPost:
		return xoauth2.AuthStyleInParams
	default:
		return xoauth2.AuthStyleInHeader
	}
}

// toURLValues converts a flat string map into url.Values, the shape
// golang.org/x/oauth2/clientcredentials.Config.EndpointParams expects.
// Returns nil (not an empty, non-nil map) when there's nothing to add, so
// EndpointParams stays unset rather than an empty-but-present value.
func toURLValues(m map[string]string) url.Values {
	if len(m) == 0 {
		return nil
	}
	v := make(url.Values, len(m))
	for key, val := range m {
		v.Set(key, val)
	}
	return v
}

// passwordTokenSource implements the Resource Owner Password Credentials grant
// (RFC 6749 Section 4.3). Token() fully re-authenticates on every call - intended
// to be wrapped in xoauth2.ReuseTokenSource, which only calls through when the
// cached token is missing or expired.
type passwordTokenSource struct {
	ctx      context.Context
	cfg      *xoauth2.Config
	username string
	password string
}

func (s *passwordTokenSource) Token() (*xoauth2.Token, error) {
	return s.cfg.PasswordCredentialsToken(s.ctx, s.username, s.password)
}

// Mode returns the processing mode. Injecting a header needs no body inspection, so
// this uses the lighter header-phase hook (unlike aws-authentication's SigV4 payload
// hashing). Response headers are processed only when purgeStatusCodes is non-empty
// (always false for the static-token path - see GetPolicy).
func (p *Policy) Mode() policy.ProcessingMode {
	responseHeaderMode := policy.HeaderModeSkip
	if len(p.purgeStatusCodes) > 0 {
		responseHeaderMode = policy.HeaderModeProcess
	}
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: responseHeaderMode,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// getStringParam safely extracts a string parameter, returning "" if absent
// or the wrong type. Leading/trailing whitespace is trimmed: credential
// values pasted from config files or secret stores frequently carry a stray
// trailing newline or space, which is invisible in logs but silently
// corrupts a client-secret comparison at the token endpoint.
func getStringParam(params map[string]interface{}, key string) string {
	if val, ok := params[key]; ok {
		if str, ok := val.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

// getStringParamOrDefault extracts a string parameter, falling back to def when the
// key is absent or the wrong type - but NOT when it's an explicitly empty string, so
// a param like valuePrefix can be intentionally cleared to "no prefix".
func getStringParamOrDefault(params map[string]interface{}, key, def string) string {
	val, ok := params[key]
	if !ok {
		return def
	}
	str, ok := val.(string)
	if !ok {
		return def
	}
	return strings.TrimSpace(str)
}

// getRequiredStringParam extracts a required, non-empty string parameter,
// trimmed per getStringParam.
func getRequiredStringParam(params map[string]interface{}, key string) (string, error) {
	val, ok := params[key]
	if !ok {
		return "", fmt.Errorf("'%s' parameter is required", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return "", fmt.Errorf("'%s' cannot be empty", key)
	}
	return str, nil
}

// getBoolParam extracts an optional boolean parameter, falling back to def
// if the key is absent or the wrong type - matching this policy's other
// optional fields' permissive, best-effort extraction style.
func getBoolParam(params map[string]interface{}, key string, def bool) bool {
	if val, ok := params[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return def
}

// getIntParam extracts an optional integer parameter, tolerating the
// several numeric shapes JSON/YAML decoding can produce (int, int64,
// float64) - falling back to def if the key is absent, the wrong type, or
// negative (retry counts and thresholds have no meaningful negative value).
func getIntParam(params map[string]interface{}, key string, def int) int {
	val, ok := params[key]
	if !ok {
		return def
	}
	var n int
	switch v := val.(type) {
	case int:
		n = v
	case int64:
		n = int(v)
	case float64:
		n = int(v)
	default:
		return def
	}
	if n < 0 {
		return def
	}
	return n
}

// getDurationParam extracts an optional Go-duration-formatted string
// parameter (e.g. "10s", "1h"), falling back to def if the key is absent,
// the wrong type, or unparsable - matching this policy's other optional
// fields' permissive, best-effort extraction style.
func getDurationParam(params map[string]interface{}, key string, def time.Duration) time.Duration {
	if val, ok := params[key]; ok {
		if str, ok := val.(string); ok {
			if d, err := time.ParseDuration(strings.TrimSpace(str)); err == nil {
				return d
			}
		}
	}
	return def
}

// getStringMapParam extracts an optional flat string-to-string map
// parameter (tokenRequestParams, tokenRequestHeaders) by key. Absent or
// wrong-shaped input just yields no entries rather than an error, matching
// how the other optional fields in this policy behave.
func getStringMapParam(params map[string]interface{}, key string) map[string]string {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out[k] = trimmed
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// getPurgeStatusCodesParam extracts "tokenPurgeStatusCodes" - the
// upstream response status codes that purge the cached token (see
// OnResponseHeaders). Absent or wrong-shaped input falls back to def
// (defaultPurgeStatusCodes), matching this policy's other optional fields.
// An explicit empty list ([]), unlike an absent key, is honored as-is - it
// disables response-phase purging entirely rather than falling back to the
// default, since that's the only way to opt out.
func getPurgeStatusCodesParam(params map[string]interface{}, key string, def []int) map[int]struct{} {
	codes := def
	if raw, ok := params[key]; ok {
		if arr, ok := raw.([]interface{}); ok {
			parsed := make([]int, 0, len(arr))
			for _, v := range arr {
				switch n := v.(type) {
				case int:
					parsed = append(parsed, n)
				case int64:
					parsed = append(parsed, int(n))
				case float64:
					parsed = append(parsed, int(n))
				case string:
					if code, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
						parsed = append(parsed, code)
					}
				}
			}
			codes = parsed
		}
	}
	set := make(map[int]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	return set
}

// validateAndExtractParams validates and extracts all policy params.
//
// bearerToken and tokenEndpoint/clientId/clientSecret are mutually exclusive auth
// paths - configuring both, or neither, is rejected here. When bearerToken is set,
// every token-endpoint-only field below is simply unused.
//
// grantType defaults to client_credentials. Fields specific to one grant
// (username/password) are validated conditionally on grantType, since JSON
// Schema's static `required` can't express "required only when grantType is X".
// clientAuthMethod defaults to client_secret_basic and applies to both grants.
func validateAndExtractParams(params map[string]interface{}) (oauth2Params, error) {
	var p oauth2Params

	p.bearerToken = getStringParam(params, "bearerToken")
	tokenEndpointRaw := getStringParam(params, "tokenEndpoint")
	clientIDRaw := getStringParam(params, "clientId")
	clientSecretRaw := getStringParam(params, "clientSecret")
	hasTokenEndpointFields := tokenEndpointRaw != "" || clientIDRaw != "" || clientSecretRaw != ""

	switch {
	case p.bearerToken != "" && hasTokenEndpointFields:
		return oauth2Params{}, fmt.Errorf(
			"'bearerToken' cannot be combined with 'tokenEndpoint'/'clientId'/'clientSecret' - configure exactly one authentication path")
	case p.bearerToken == "" && !hasTokenEndpointFields:
		return oauth2Params{}, fmt.Errorf(
			"either 'bearerToken' or 'tokenEndpoint'+'clientId'+'clientSecret' must be configured")
	}

	p.headerName = getStringParamOrDefault(params, "headerName", defaultHeaderName)
	if p.headerName == "" {
		// Defensive only: policy-definition.yaml's minLength: 1 should
		// already reject an explicit empty string before this runs.
		p.headerName = defaultHeaderName
	}
	p.valuePrefix = getStringParamOrDefault(params, "valuePrefix", defaultValuePrefix)

	if p.bearerToken != "" {
		// Static-token path: nothing else to validate or default.
		return p, nil
	}

	p.grantType = getStringParam(params, "grantType")
	if p.grantType == "" {
		p.grantType = GrantTypeClientCredentials
	}
	if p.grantType != GrantTypeClientCredentials && p.grantType != GrantTypePassword {
		return oauth2Params{}, fmt.Errorf("'grantType' must be one of %q, %q", GrantTypeClientCredentials, GrantTypePassword)
	}

	p.clientAuthMethod = getStringParam(params, "clientAuthMethod")
	if p.clientAuthMethod == "" {
		p.clientAuthMethod = ClientAuthMethodBasic
	}
	if p.clientAuthMethod != ClientAuthMethodBasic && p.clientAuthMethod != ClientAuthMethodPost {
		return oauth2Params{}, fmt.Errorf("'clientAuthMethod' must be one of %q, %q", ClientAuthMethodBasic, ClientAuthMethodPost)
	}

	var err error
	p.tokenEndpoint, err = getRequiredStringParam(params, "tokenEndpoint")
	if err != nil {
		return oauth2Params{}, err
	}
	p.clientID, err = getRequiredStringParam(params, "clientId")
	if err != nil {
		return oauth2Params{}, err
	}
	p.clientSecret, err = getRequiredStringParam(params, "clientSecret")
	if err != nil {
		return oauth2Params{}, err
	}
	p.tokenRequestParams = getStringMapParam(params, "tokenRequestParams")
	p.tokenRequestHeaders = getStringMapParam(params, "tokenRequestHeaders")
	p.requestTimeout = getDurationParam(params, "tokenRequestTimeout", defaultTokenRequestTimeout)
	p.tokenTTLFallback = getDurationParam(params, "defaultTokenTTL", defaultTokenTTLFallback)
	p.expiryBuffer = getDurationParam(params, "expiryBuffer", defaultExpiryBuffer)
	if p.expiryBuffer < 0 {
		// A negative buffer has no sane meaning here (see getIntParam's
		// identical treatment of negative retry counts) - fall back rather
		// than let it invert the freshness check.
		p.expiryBuffer = defaultExpiryBuffer
	}
	p.purgeStatusCodes = getPurgeStatusCodesParam(params, "tokenPurgeStatusCodes", defaultPurgeStatusCodes)

	p.proxyURL = getStringParam(params, "proxyURL")
	p.tlsCaCertPath = getStringParam(params, "tlsCaCertPath")
	p.tlsInsecureSkipVerify = getBoolParam(params, "tlsInsecureSkipVerify", false)
	if p.tlsInsecureSkipVerify {
		slog.Warn("OAuth2Generator: tlsInsecureSkipVerify is enabled - TLS certificate verification for the token endpoint is disabled; this must never be used against a real identity provider")
	}

	p.tokenRequestMaxRetries = getIntParam(params, "tokenRequestMaxRetries", defaultTokenRequestMaxRetries)

	if p.grantType == GrantTypePassword {
		p.username, err = getRequiredStringParam(params, "username")
		if err != nil {
			return oauth2Params{}, err
		}
		p.password, err = getRequiredStringParam(params, "password")
		if err != nil {
			return oauth2Params{}, err
		}
	}

	return p, nil
}

// OnRequestHeaders obtains (fetching/reusing a cached token, or reading the
// directly-supplied one) the credential and injects it into headerName
// (default Authorization, prefixed per valuePrefix, default "Bearer ")
// before the request is forwarded to the upstream backend.
func (p *Policy) OnRequestHeaders(ctx context.Context, reqCtx *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	slog.Debug("OAuth2Generator: authenticating outbound request", "method", reqCtx.Method, "path", reqCtx.Path,
		"mode", p.mode, "grantType", p.grantType, "tokenEndpoint", p.tokenEndpoint, "clientId", p.clientID)

	tok, err := p.retrieveToken()
	if err != nil {
		return p.authFailure(reqCtx.SharedContext, "failed to obtain upstream credential", err)
	}

	p.authSuccess(reqCtx.SharedContext)

	return policy.UpstreamRequestHeaderModifications{
		HeadersToSet: map[string]string{
			p.headerName: buildHeaderValue(p.valuePrefix, tok.AccessToken),
		},
	}
}

// OnResponseHeaders purges the cached token when the upstream responds with one of
// purgeStatusCodes (default: 401) - the token was rejected, e.g. revoked out-of-band.
// Doesn't retry the current request; purging only guarantees the next one fetches
// fresh. Only reached when purgeStatusCodes is non-empty (never for static-token).
func (p *Policy) OnResponseHeaders(ctx context.Context, respCtx *policy.ResponseHeaderContext, _ map[string]interface{}) policy.ResponseHeaderAction {
	if _, purge := p.purgeStatusCodes[respCtx.ResponseStatus]; purge {
		slog.Warn("OAuth2Generator: upstream rejected the cached token, purging it for the next request",
			"status", respCtx.ResponseStatus, "grantType", p.grantType, "tokenEndpoint", p.tokenEndpoint, "clientId", p.clientID)
		p.tokenSource.Purge()
	}
	return policy.DownstreamResponseHeaderModifications{}
}

// OnUpstreamAttemptRequestHeaders implements policy.UpstreamAttemptPolicy - it
// runs once per individual upstream dial attempt (including Envoy-native
// retries; see resilience.retry), not once per client request. On any
// attempt after the first, the previous attempt's response is assumed
// rejected (that's why Envoy retried at all, per the configured
// resilience.retry.statusCodes), so the cached token is purged before
// refetching, guaranteeing attempt 2+ gets a genuinely fresh token rather
// than resending the same one that was just rejected. Fails open on any
// fetch error: an empty action lets the retry proceed with whatever
// Authorization header it already had rather than blocking it - this
// mechanism only ever makes a retry more likely to succeed, never a new way
// for it to fail (see Global Constraints).
func (p *Policy) OnUpstreamAttemptRequestHeaders(ctx context.Context, actx *policy.UpstreamAttemptContext) policy.UpstreamAttemptAction {
	if actx.AttemptCount > 1 {
		p.tokenSource.Purge()
	}

	tok, err := p.retrieveToken()
	if err != nil {
		slog.WarnContext(ctx, "OAuth2Generator: failed to fetch token for upstream attempt, failing open (no header mutation)",
			"attempt", actx.AttemptCount, "grantType", p.grantType, "clientId", p.clientID, "error", err)
		return policy.UpstreamAttemptHeaderModifications{}
	}

	return policy.UpstreamAttemptHeaderModifications{
		HeadersToSet: map[string]string{
			p.headerName: buildHeaderValue(p.valuePrefix, tok.AccessToken),
		},
	}
}

// retrieveToken fetches the current (possibly cached/refreshed) access token
// from the token source built once in GetPolicy.
func (p *Policy) retrieveToken() (*xoauth2.Token, error) {
	fetch := p.tokenFunc
	if fetch == nil {
		fetch = p.tokenSource.Token
	}
	return fetch()
}

// authType returns the AuthContext.AuthType value for this instance's mode -
// see AuthType/AuthTypeStaticToken.
func (p *Policy) authType() string {
	if p.mode == ModeStaticToken {
		return AuthTypeStaticToken
	}
	return AuthType
}

// credentialID returns the AuthContext.CredentialID value for this
// instance's mode: clientID for the token-endpoint path, or a fixed
// placeholder for the static-token path, which has no client identity of its
// own to report.
func (p *Policy) credentialID() string {
	if p.mode == ModeStaticToken {
		return "static-token"
	}
	return p.clientID
}

// authProperties returns the AuthContext.Properties for this instance's
// mode. grantType/tokenEndpoint only apply to (and are only populated for)
// the token-endpoint path.
func (p *Policy) authProperties() map[string]string {
	props := map[string]string{"mode": p.mode}
	if p.mode == ModeTokenEndpoint {
		props["grantType"] = p.grantType
		props["tokenEndpoint"] = p.tokenEndpoint
	}
	return props
}

// authFailure builds a 502 Bad Gateway ImmediateResponse for gateway-side
// credential-acquisition failures. 502 (not 401) is deliberate: the caller's
// request was fine — it is the gateway's own upstream credentials or the
// token endpoint that failed, a gateway-to-backend problem rather than a
// client-auth rejection.
func (p *Policy) authFailure(shared *policy.SharedContext, reason string, cause error) policy.RequestHeaderAction {
	slog.Error("OAuth2Generator: credential acquisition failed", "reason", reason, "error", cause,
		"mode", p.mode, "grantType", p.grantType, "tokenEndpoint", p.tokenEndpoint, "clientId", p.clientID)

	shared.AuthContext = &policy.AuthContext{
		Authenticated: false,
		AuthType:      p.authType(),
		CredentialID:  p.credentialID(),
		Properties:    p.authProperties(),
		Previous:      shared.AuthContext,
	}

	body, _ := json.Marshal(map[string]string{
		"error":   "Bad Gateway",
		"message": "failed to authenticate request to upstream service",
	})
	return policy.ImmediateResponse{
		StatusCode: http.StatusBadGateway,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}

// authSuccess records a successful credential injection in the shared
// AuthContext, preserving any existing chain (e.g. an earlier inbound auth
// policy) via Previous.
func (p *Policy) authSuccess(shared *policy.SharedContext) {
	shared.AuthContext = &policy.AuthContext{
		Authenticated: true,
		AuthType:      p.authType(),
		CredentialID:  p.credentialID(),
		Properties:    p.authProperties(),
		Previous:      shared.AuthContext,
	}
}
