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

// This file wires the outbound authentication path Platform-API uses when it
// calls a portal's admin REST endpoints. Two provider implementations sit
// behind one interface, and a small process-wide registry caches provider
// instances (and their token caches) per portal handle.
//
// Consumers (the publisher and any future portal-facing component) call
// APIPortalAuthRegistry.Get(portal).AuthorizationHeader(ctx) and get back a
// ready-to-use `Bearer <token>` string. Token refresh, caching, and mutex-
// guarded refresh are the provider's concern, not the caller's.

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

// AuthProvider is the outbound-auth surface exposed to any component that
// needs to call a portal's admin REST endpoints. The concrete provider
// (`local` or `oauth2`) is selected per portal by the registry; callers are
// unaware of which one they're holding.
type AuthProvider interface {
	// AuthorizationHeader returns a valid "Bearer <token>" header value for
	// the next outbound call. The provider handles all caching + refresh
	// internally — callers never see a stale token unless they invalidate
	// explicitly.
	AuthorizationHeader(ctx context.Context) (string, error)

	// InvalidateCache clears any cached token so the next call re-mints or
	// re-fetches. Callers invoke this on a portal-side 401 to recover from
	// an expired/revoked token the provider hasn't yet noticed.
	InvalidateCache()
}

// tokenCacheRefreshBuffer is the safety window subtracted from the STS-reported
// expiry so we refresh slightly before the token actually expires — otherwise
// a call that lands right at the expiry boundary would fail with a 401.
const tokenCacheRefreshBuffer = 30 * time.Second

// localTokenTTL is the lifetime of a self-minted JWT for the `local` flow.
// Kept short so any key/config change on disk is picked up quickly on the
// next refresh; minting is cheap (single RS256 sign).
const localTokenTTL = 5 * time.Minute

// --- LocalAuthProvider -----------------------------------------------------

// localAuthProvider mints platform-api-signed RS256 JWTs. The signing key is
// the same one AuthLoginHandler uses (Auth.JWT.PrivateKeyFile), so the
// devportal's `verifyBearerToken` in local mode — configured with the paired
// public key — accepts these tokens without any per-portal setup.
type localAuthProvider struct {
	jwtCfg *config.JWT

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

func newLocalAuthProvider(jwtCfg *config.JWT) *localAuthProvider {
	return &localAuthProvider{jwtCfg: jwtCfg}
}

func (p *localAuthProvider) AuthorizationHeader(_ context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != "" && time.Now().Before(p.expiresAt.Add(-tokenCacheRefreshBuffer)) {
		return p.cached, nil
	}

	priv, err := p.jwtCfg.LoadPrivateKey()
	if err != nil {
		return "", fmt.Errorf("api-portal local auth: load private key: %w", err)
	}
	now := time.Now()
	exp := now.Add(localTokenTTL)
	claims := jwt.MapClaims{
		"sub":   "platform-api-system",
		"iss":   "platform-api",
		"roles": []string{"platform-api-system"},
		"iat":   now.Unix(),
		"exp":   exp.Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(priv)
	if err != nil {
		return "", fmt.Errorf("api-portal local auth: sign token: %w", err)
	}
	p.cached = "Bearer " + signed
	p.expiresAt = exp
	return p.cached, nil
}

func (p *localAuthProvider) InvalidateCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cached = ""
	p.expiresAt = time.Time{}
}

// --- ClientCredentialsAuthProvider -----------------------------------------

// clientCredentialsTokenResponse is the RFC 6749 §5.1 successful token
// response shape. Providers may include additional fields; we only read the
// two we need.
type clientCredentialsTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// clientCredentialsAuthProvider fetches a JWT from an external STS using the
// OAuth 2.0 client_credentials grant. Holds the plaintext client secret in
// memory for the lifetime of the provider; the caller retrieves that value by
// decrypting the stored ciphertext via the vault before constructing this.
type clientCredentialsAuthProvider struct {
	tokenURL     string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

func newClientCredentialsAuthProvider(tokenURL, clientID, clientSecret string, hc *http.Client) *clientCredentialsAuthProvider {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &clientCredentialsAuthProvider{
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   hc,
	}
}

func (p *clientCredentialsAuthProvider) AuthorizationHeader(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != "" && time.Now().Before(p.expiresAt.Add(-tokenCacheRefreshBuffer)) {
		return p.cached, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("api-portal oauth2 auth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api-portal oauth2 auth: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("api-portal oauth2 auth: sts returned %d: %s", resp.StatusCode, string(body))
	}
	var tok clientCredentialsTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("api-portal oauth2 auth: decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", errors.New("api-portal oauth2 auth: sts response missing access_token")
	}
	// If expires_in is zero or missing, treat the token as short-lived so we
	// refresh soon rather than caching for an unknown duration.
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Minute
	}
	p.cached = "Bearer " + tok.AccessToken
	p.expiresAt = time.Now().Add(ttl)
	return p.cached, nil
}

func (p *clientCredentialsAuthProvider) InvalidateCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cached = ""
	p.expiresAt = time.Time{}
}

// --- APIPortalAuthRegistry --------------------------------------------------

// APIPortalAuthRegistry is the process-wide cache of AuthProvider instances
// keyed by portal handle. Callers (publisher, health-check, anything else
// talking to a portal admin REST) share these instances so their token
// caches are hot across concurrent requests. Invalidate is called by the
// service layer on Update/Delete so cached providers reflect config changes.
type APIPortalAuthRegistry struct {
	jwtCfg     *config.JWT
	secrets    vault.SecretVault
	httpClient *http.Client

	mu    sync.Mutex
	cache map[string]AuthProvider
}

// NewAPIPortalAuthRegistry constructs the registry. `hc` may be nil, in which
// case each oauth2 provider gets a default http.Client with a 15s timeout.
func NewAPIPortalAuthRegistry(jwtCfg *config.JWT, secretVault vault.SecretVault, hc *http.Client) *APIPortalAuthRegistry {
	return &APIPortalAuthRegistry{
		jwtCfg:     jwtCfg,
		secrets:    secretVault,
		httpClient: hc,
		cache:      make(map[string]AuthProvider),
	}
}

// Get returns the cached AuthProvider for a portal, constructing one from the
// stored row if none exists yet. Never returns a nil provider on success.
func (r *APIPortalAuthRegistry) Get(portal *model.APIPortal) (AuthProvider, error) {
	if portal == nil {
		return nil, errors.New("api-portal auth registry: portal is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.cache[portal.Handle]; ok {
		return p, nil
	}
	p, err := r.buildProvider(portal)
	if err != nil {
		return nil, err
	}
	r.cache[portal.Handle] = p
	return p, nil
}

// Invalidate evicts the cached provider for the given portal handle. Called by
// the service layer after a successful Update or Delete so the next Get
// picks up any config changes (or, for Delete, so we don't leak stale
// providers).
func (r *APIPortalAuthRegistry) Invalidate(portalHandle string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, portalHandle)
}

// buildProvider constructs the concrete provider from a stored portal row.
// For oauth2 the clientSecret is base64-decoded and decrypted via the vault
// here; the plaintext is then held in memory by the provider until the
// registry entry is invalidated.
func (r *APIPortalAuthRegistry) buildProvider(portal *model.APIPortal) (AuthProvider, error) {
	switch portal.AuthType {
	case constants.APIPortalAuthTypeLocal:
		if r.jwtCfg == nil {
			return nil, errors.New("api-portal auth registry: jwtCfg is nil; cannot mint local tokens")
		}
		return newLocalAuthProvider(r.jwtCfg), nil
	case constants.APIPortalAuthTypeOAuth2:
		if r.secrets == nil {
			return nil, errors.New("api-portal auth registry: secret vault is nil; cannot decrypt oauth2 client secret")
		}
		tokenURL, _ := portal.AuthConfig[constants.APIPortalAuthConfigKeySTSTokenURL].(string)
		clientID, _ := portal.AuthConfig[constants.APIPortalAuthConfigKeyClientID].(string)
		encoded, _ := portal.AuthConfig[constants.APIPortalAuthConfigKeyClientSecret].(string)
		if tokenURL == "" || clientID == "" || encoded == "" {
			return nil, fmt.Errorf(
				"api-portal auth registry: portal %q authConfig is missing required oauth2 fields",
				portal.Handle)
		}
		ciphertext, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf(
				"api-portal auth registry: portal %q clientSecret is not valid base64 ciphertext: %w",
				portal.Handle, err)
		}
		plaintext, err := r.secrets.Decrypt(context.Background(), ciphertext)
		if err != nil {
			return nil, fmt.Errorf(
				"api-portal auth registry: portal %q clientSecret decryption failed: %w",
				portal.Handle, err)
		}
		return newClientCredentialsAuthProvider(tokenURL, clientID, plaintext, r.httpClient), nil
	default:
		return nil, fmt.Errorf(
			"api-portal auth registry: portal %q has unsupported auth_type %q",
			portal.Handle, portal.AuthType)
	}
}
