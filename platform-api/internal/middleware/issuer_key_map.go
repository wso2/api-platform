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

package middleware

import (
	"crypto/rsa"
	"fmt"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// IssuerKind says how tokens from one issuer are verified.
type IssuerKind int

const (
	// IssuerKindLocal is auth.jwt: login tokens, and SA tokens on the default key.
	IssuerKindLocal IssuerKind = iota
	// IssuerKindIDP is verified by the IdP's JWKS, outside this map.
	IssuerKindIDP
	// IssuerKindServiceAccount is a key that signs only SA tokens.
	IssuerKindServiceAccount
)

// IssuerKeys registers one issuer. Current may be nil for a verify-nothing
// entry (skip_validation with no key); Retired keys verify but never sign.
type IssuerKeys struct {
	Issuer  string
	Kind    IssuerKind
	Current *rsa.PublicKey
	Retired []*rsa.PublicKey
}

type issuerEntry struct {
	kind    IssuerKind
	current *rsa.PublicKey
	byKID   map[string]*rsa.PublicKey
}

// IssuerKeyMap routes a token to the one key registered for its iss. A token
// is never tried against a key registered for another issuer, so adding an
// entry cannot widen what an existing token can do.
type IssuerKeyMap struct {
	entries    map[string]*issuerEntry
	audience   string // required on SA tokens only
	saDisabled bool
}

// NewIssuerKeyMap fails when two entries claim the same issuer.
func NewIssuerKeyMap(saAudience string, keys ...IssuerKeys) (*IssuerKeyMap, error) {
	m := &IssuerKeyMap{entries: map[string]*issuerEntry{}, audience: saAudience}
	for _, k := range keys {
		if _, dup := m.entries[k.Issuer]; dup {
			return nil, fmt.Errorf("issuer key map: issuer %q is registered twice", k.Issuer)
		}
		e := &issuerEntry{kind: k.Kind, current: k.Current, byKID: map[string]*rsa.PublicKey{}}
		for _, pub := range append([]*rsa.PublicKey{k.Current}, k.Retired...) {
			if pub != nil {
				e.byKID[utils.RSAThumbprint(pub)] = pub
			}
		}
		m.entries[k.Issuer] = e
	}
	return m, nil
}

// DisableServiceAccounts makes verify refuse every SA token. Without it, a
// token minted on the shared key before the feature was turned off would still
// verify, with no revocation check.
func (m *IssuerKeyMap) DisableServiceAccounts() { m.saDisabled = true }

// IsLocal reports whether iss is verified by this map rather than by an IdP.
func (m *IssuerKeyMap) IsLocal(iss string) bool {
	e, ok := m.entries[iss]
	return ok && e.kind != IssuerKindIDP
}

// IsServiceAccountToken: the issuer is the SA kind, or it is the local issuer
// and sub carries the reserved prefix. The second arm is the default config,
// where SA and login tokens share auth.jwt's issuer.
func (m *IssuerKeyMap) IsServiceAccountToken(iss, sub string) bool {
	e, ok := m.entries[iss]
	return ok && isServiceAccountToken(e, sub)
}

func isServiceAccountToken(e *issuerEntry, sub string) bool {
	switch e.kind {
	case IssuerKindServiceAccount:
		return true
	case IssuerKindLocal:
		return strings.HasPrefix(sub, constants.ServiceAccountSubPrefix)
	}
	return false
}

// keyFor is the jwt.Keyfunc. SA tokens must name a known kid. Login tokens
// carry none today, and a token minted elsewhere may carry a kid we never
// issued, so for those an unknown kid falls back to the current key.
func (m *IssuerKeyMap) keyFor(t *jwt.Token) (interface{}, error) {
	if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("unexpected or forbidden signing method: %v", t.Header["alg"])
	}
	claims, _ := t.Claims.(jwt.MapClaims)
	iss, _ := claims["iss"].(string)
	sub, _ := claims["sub"].(string)
	e, ok := m.entries[iss]
	if !ok || e.kind == IssuerKindIDP {
		return nil, fmt.Errorf("invalid token issuer")
	}
	kid, _ := t.Header["kid"].(string)
	if pub := e.byKID[kid]; kid != "" && pub != nil {
		return pub, nil
	}
	if isServiceAccountToken(e, sub) {
		return nil, fmt.Errorf("service-account token has a missing or unknown kid")
	}
	if e.current == nil {
		return nil, fmt.Errorf("no verification key for issuer")
	}
	return e.current, nil
}

// verify checks signature, expiry and issuer, and aud on SA tokens.
func (m *IssuerKeyMap) verify(tokenString string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, m.keyFor,
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	iss, _ := claims["iss"].(string)
	sub, _ := claims["sub"].(string)
	if m.IsServiceAccountToken(iss, sub) {
		if m.saDisabled {
			return nil, fmt.Errorf("service accounts are disabled")
		}
		if err := m.checkServiceAccountClaims(claims); err != nil {
			return nil, err
		}
	}
	return claims, nil
}

// VerifyServiceAccountToken verifies a token and requires it to be an SA token.
func (m *IssuerKeyMap) VerifyServiceAccountToken(tokenString string) (jwt.MapClaims, error) {
	claims, err := m.verify(tokenString)
	if err != nil {
		return nil, err
	}
	iss, _ := claims["iss"].(string)
	sub, _ := claims["sub"].(string)
	if !m.IsServiceAccountToken(iss, sub) {
		return nil, fmt.Errorf("not a service-account token")
	}
	return claims, nil
}

// checkServiceAccountClaims requires aud, iat, exp and the token version. Login
// tokens carry no aud and no version, so this runs on SA tokens only.
func (m *IssuerKeyMap) checkServiceAccountClaims(claims jwt.MapClaims) error {
	aud, err := claims.GetAudience()
	if err != nil || !slices.Contains(aud, m.audience) {
		return fmt.Errorf("service-account token has the wrong audience")
	}
	if iat, err := claims.GetIssuedAt(); err != nil || iat == nil {
		return fmt.Errorf("service-account token has no iat")
	}
	// The parser checks exp only when present; without it the token never expires.
	if exp, err := claims.GetExpirationTime(); err != nil || exp == nil {
		return fmt.Errorf("service-account token has no exp")
	}
	if _, ok := ServiceAccountTokenVersion(claims); !ok {
		return fmt.Errorf("service-account token has no token version")
	}
	return nil
}
