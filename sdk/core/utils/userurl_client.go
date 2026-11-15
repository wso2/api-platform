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

package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ErrDestinationNotAllowed matches (errors.Is) a request or check refused by
// the gateway's network access control
// (controller.server.network_security.access_control). The error that carries
// it never says which rule refused the destination or what it resolved to.
var ErrDestinationNotAllowed = errors.New("destination is not allowed by the gateway's network access control")

// ErrUserURLClientUnavailable is returned when the policy engine has not
// installed a user-URL client factory (e.g. a unit test that did not call
// SetUserURLClientFactory).
var ErrUserURLClientUnavailable = errors.New("user-URL HTTP client is not available: the policy engine has not installed one")

// UserURLClientOptions customizes a client from NewUserURLHTTPClient. The zero
// value is valid: the engine's configured timeouts, TLS verification on, and
// no redirects followed.
type UserURLClientOptions struct {
	// Timeout bounds a whole request, response headers included. Zero keeps
	// the engine's configured timeout.
	Timeout time.Duration
	// FollowRedirects follows same-host redirects (each hop is checked
	// again). When false, a 3xx response is returned to the caller as is.
	FollowRedirects bool
	// InsecureSkipVerify disables certificate verification for this client,
	// for a policy whose parameters explicitly opt out of it.
	InsecureSkipVerify bool
	// RootCAs, when set, replaces the trusted CA pool.
	RootCAs *x509.CertPool
	// GetClientCertificate, when set, supplies a client certificate for mTLS.
	GetClientCertificate func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	// MaxConnsPerHost, when positive, caps this client's connections per host
	// (and its idle pool to the same number). Zero keeps the engine's pooling.
	MaxConnsPerHost int
}

// UserURLClientFactory builds a client for destinations taken from tenant or
// user input. Installed by the policy engine; see SetUserURLClientFactory.
type UserURLClientFactory func(UserURLClientOptions) (*http.Client, error)

// UserURLChecker validates a destination taken from tenant or user input
// without sending a request. Installed by the policy engine.
type UserURLChecker func(ctx context.Context, rawURL string) error

var (
	userURLMu      sync.RWMutex
	userURLFactory UserURLClientFactory
	userURLChecker UserURLChecker
)

// SetUserURLClientFactory installs the factory NewUserURLHTTPClient uses and
// the checker CheckUserURL uses. Called once by the policy engine at startup;
// not intended to be called by policies.
func SetUserURLClientFactory(factory UserURLClientFactory, checker UserURLChecker) {
	userURLMu.Lock()
	defer userURLMu.Unlock()
	userURLFactory = factory
	userURLChecker = checker
}

// NewUserURLHTTPClient returns a client for a destination that comes from
// tenant or user input: an API-level policy parameter (an interceptor
// endpoint, an OPA server, a token endpoint, a JWKS URI) or a URL found in a
// request or response payload. Every request it makes, and every redirect hop
// it follows, is subject to the gateway's network access control
// (controller.server.network_security.access_control); a refused destination
// fails with an error matching ErrDestinationNotAllowed.
//
// Build the client once (for example when the policy instance is created) and
// reuse it: each call builds a new connection pool.
//
// For a destination set by the operator in config.toml (a policy system
// parameter), use SharedHTTPClient instead; network access control does not
// apply to those.
func NewUserURLHTTPClient(opts UserURLClientOptions) (*http.Client, error) {
	userURLMu.RLock()
	factory := userURLFactory
	userURLMu.RUnlock()
	if factory == nil {
		return nil, ErrUserURLClientUnavailable
	}
	return factory(opts)
}

// CheckUserURL applies the gateway's network access control to a destination
// taken from tenant or user input without sending a request: its scheme, its
// hostname, and the addresses the hostname resolves to. It returns an error
// matching ErrDestinationNotAllowed when the destination is refused, another
// error when it cannot be checked (malformed, or the host does not resolve),
// and nil when it is permitted or access control is off.
//
// It suits a policy that judges a URL without fetching it. A policy that
// fetches the URL needs no separate check: NewUserURLHTTPClient's client
// enforces the same rules when it connects.
func CheckUserURL(ctx context.Context, rawURL string) error {
	userURLMu.RLock()
	checker := userURLChecker
	userURLMu.RUnlock()
	if checker == nil {
		return ErrUserURLClientUnavailable
	}
	return checker(ctx, rawURL)
}
