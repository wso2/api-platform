/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package testproxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Errors from target resolution. Like the sanitizer's, these exist to make the
// server-side log specific; the caller always sees the same generic rejection.
var (
	ErrAPINotFound       = errors.New("api not found for this caller")
	ErrGatewayNotFound   = errors.New("gateway is not one this api is deployed to")
	ErrGatewayNoEndpoint = errors.New("gateway has no usable endpoint")
	ErrGatewayOtherOrg   = errors.New("gateway belongs to a different organization")
	ErrUnsupportedScheme = errors.New("gateway endpoint scheme is not http or https")
)

// maxResolveBodyBytes bounds each Platform API response the resolver reads.
// Small: these are one API record and one gateway list.
const maxResolveBodyBytes = 1 << 20 // 1 MiB

// Target is a resolved, authorized invoke destination.
type Target struct {
	// URL is the API's invoke URL on the chosen gateway; origin plus the API's
	// context, with no trailing slash. The relayed path is appended to its path.
	URL *url.URL
	// GatewayID is echoed back for logging, so an audit line names the gateway
	// by handle rather than only by address.
	GatewayID string
}

// Resolver turns (caller, org, api, gateway) into an invoke URL by asking
// Platform API, with the caller's own session token.
//
// Using the caller's token is the point: Platform API is the authorization
// boundary, so a caller who may not read this API in this organization simply
// gets a 404 back and the invoke is refused. The BFF does not reimplement
// organization scoping, and cannot get it subtly wrong in a way Platform API
// would not.
type Resolver struct {
	client  *http.Client
	baseURL string // Platform API origin + management base path, no trailing slash

	ttl     time.Duration
	maxSize int
	mu      sync.Mutex
	entries map[string]resolveEntry
	nowFunc func() time.Time
}

type resolveEntry struct {
	target  Target
	expires time.Time
}

// NewResolver builds a Resolver. platformBaseURL is the Platform API origin and
// managementBasePath its REST mount path (e.g. "/api/v0.9"). client should be
// the BFF's existing upstream client, this talks to the Platform API, not to a
// gateway, so it deliberately does not use the relay's guarded client.
func NewResolver(client *http.Client, platformBaseURL, managementBasePath string, ttl time.Duration, maxSize int) *Resolver {
	return &Resolver{
		client:  client,
		baseURL: strings.TrimRight(platformBaseURL, "/") + "/" + strings.Trim(managementBasePath, "/"),
		ttl:     ttl,
		maxSize: maxSize,
		entries: make(map[string]resolveEntry),
		nowFunc: time.Now,
	}
}

// cacheKey binds a cached resolution to the specific caller that established it.
//
// The token digest is intentionally included in the key. Keying solely by
// (org, api, gateway) would allow a successful resolution for one caller to
// be reused by another caller who is not authorized to access the API,
// creating a confused-deputy vulnerability for the duration of the cache
// entry's TTL. The token is hashed rather than stored directly, ensuring that
// the cache does not retain credentials in readable form.
func cacheKey(token, orgHandle, restAPIID, gatewayID string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8]) + "|" + orgHandle + "|" + restAPIID + "|" + gatewayID
}

// Resolve returns the invoke target, consulting Platform API on a cache miss.
func (r *Resolver) Resolve(ctx context.Context, token, orgHandle, restAPIID, gatewayID string) (Target, error) {
	key := cacheKey(token, orgHandle, restAPIID, gatewayID)
	if t, ok := r.cached(key); ok {
		return t, nil
	}

	api, err := r.fetchAPI(ctx, token, orgHandle, restAPIID)
	if err != nil {
		return Target{}, err
	}
	gateways, err := r.fetchGateways(ctx, token, orgHandle, restAPIID)
	if err != nil {
		return Target{}, err
	}

	gw, ok := findDeployedGateway(gateways, gatewayID)
	if !ok {
		return Target{}, ErrGatewayNotFound
	}
	// Defense in depth behind Platform API's own org check: a gateway record
	// that names a different organization than the one this call was scoped to
	// must not be dialed, whatever the listing said.
	if gw.OrganizationID != "" && orgHandle != "" && gw.OrganizationID != orgHandle {
		return Target{}, ErrGatewayOtherOrg
	}

	target, err := buildTarget(gw, api.Context)
	if err != nil {
		return Target{}, err
	}
	r.store(key, target)
	return target, nil
}

func (r *Resolver) cached(key string) (Target, bool) {
	if r.ttl <= 0 {
		return Target{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[key]
	if !ok || r.nowFunc().After(e.expires) {
		delete(r.entries, key)
		return Target{}, false
	}
	return e.target, true
}

func (r *Resolver) store(key string, t Target) {
	if r.ttl <= 0 || r.maxSize <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) >= r.maxSize {
		r.evictExpiredLocked()
		// Still full of live entries: drop the map rather than let it grow
		// without bound. A cold cache costs two Platform API calls, which is
		// exactly what the uncached path already does.
		if len(r.entries) >= r.maxSize {
			r.entries = make(map[string]resolveEntry, r.maxSize)
		}
	}
	r.entries[key] = resolveEntry{target: t, expires: r.nowFunc().Add(r.ttl)}
}

func (r *Resolver) evictExpiredLocked() {
	now := r.nowFunc()
	for k, e := range r.entries {
		if now.After(e.expires) {
			delete(r.entries, k)
		}
	}
}

// restAPI is the subset of Platform API's REST API record this package reads.
type restAPI struct {
	Context string `json:"context"`
}

// gateway is the subset of one RESTAPIGatewayResponse entry this package reads.
type gateway struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organizationId"`
	Endpoints      []string `json:"endpoints"`
	IsDeployed     bool     `json:"isDeployed"`
	Deployment     *struct {
		Status string `json:"status"`
	} `json:"deployment"`
}

type gatewayList struct {
	List []gateway `json:"list"`
}

func (r *Resolver) fetchAPI(ctx context.Context, token, orgHandle, restAPIID string) (restAPI, error) {
	var out restAPI
	path := "/rest-apis/" + url.PathEscape(restAPIID)
	if err := r.get(ctx, token, orgHandle, path, &out); err != nil {
		return restAPI{}, err
	}
	return out, nil
}

func (r *Resolver) fetchGateways(ctx context.Context, token, orgHandle, restAPIID string) ([]gateway, error) {
	var out gatewayList
	path := "/rest-apis/" + url.PathEscape(restAPIID) + "/gateways"
	if err := r.get(ctx, token, orgHandle, path, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// get performs one authenticated Platform API read. A 401/403/404 all collapse
// to ErrAPINotFound: from this package's point of view they are the same
// outcome; the caller does not get to use this API, and distinguishing them
// in the response would tell a caller whether an API id exists.
func (r *Resolver) get(ctx context.Context, token, orgHandle, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if orgHandle != "" {
		req.Header.Set("X-Org-Id", orgHandle)
	}

	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("platform api request failed: %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
	case res.StatusCode == http.StatusUnauthorized,
		res.StatusCode == http.StatusForbidden,
		res.StatusCode == http.StatusNotFound:
		return ErrAPINotFound
	default:
		return fmt.Errorf("platform api returned status %d", res.StatusCode)
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, maxResolveBodyBytes)).Decode(out); err != nil {
		return fmt.Errorf("platform api returned an undecodable response: %w", err)
	}
	return nil
}

// findDeployedGateway picks the named gateway, but only if the API is actually
// deployed to it.
//
// Requiring deployment is a real authorization narrowing, not cosmetics: the
// console only offers deployed gateways, so accepting an undeployed one would
// let a caller aim the relay at any gateway merely *associated* with the API.
// Both signals from the listing are honoured, because the SPA treats either as
// deployed and the server must not refuse a gateway the UI just offered.
func findDeployedGateway(gateways []gateway, gatewayID string) (gateway, bool) {
	for _, gw := range gateways {
		if gw.ID != gatewayID {
			continue
		}
		if gw.IsDeployed || gw.Deployment != nil {
			return gw, true
		}
		return gateway{}, false
	}
	return gateway{}, false
}

// buildTarget composes the invoke URL from the gateway's endpoint and the API's
// context, mirroring the SPA's own buildInvokeUrl so the URL the console shows
// and the URL the relay dials cannot drift apart.
func buildTarget(gw gateway, apiContext string) (Target, error) {
	endpoint := ""
	for _, e := range gw.Endpoints {
		if strings.TrimSpace(e) != "" {
			endpoint = strings.TrimSpace(e)
			break
		}
	}
	if endpoint == "" {
		return Target{}, ErrGatewayNoEndpoint
	}
	// The spec types endpoints as full URL strings, but the SPA tolerates a
	// bare host and assumes https; match that rather than failing on a record
	// the console would happily have rendered.
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || u.Host == "" {
		return Target{}, ErrGatewayNoEndpoint
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Target{}, ErrUnsupportedScheme
	}

	ctxPath := strings.TrimSpace(apiContext)
	if ctxPath == "" {
		ctxPath = "/"
	}
	if !strings.HasPrefix(ctxPath, "/") {
		ctxPath = "/" + ctxPath
	}

	// Compose only the parts that matter; anything else on the endpoint (query,
	// fragment, userinfo) is not part of an invoke URL and is dropped rather
	// than carried into every relayed request.
	target := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   strings.TrimRight(u.Path, "/") + strings.TrimRight(ctxPath, "/"),
	}
	return Target{URL: target, GatewayID: gw.ID}, nil
}
