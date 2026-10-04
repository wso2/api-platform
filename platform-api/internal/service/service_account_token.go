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

package service

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ErrNoServiceAccountSigningKey means no private key is available to sign SA tokens.
var ErrNoServiceAccountSigningKey = errors.New("no service-account signing key configured")

// ServiceAccountKeys is the SA issuer and its keys: the signing pair plus any
// retired public keys that still verify.
type ServiceAccountKeys struct {
	Issuer     string
	OwnIssuer  bool // true when [auth.service_account.jwt] is configured
	PrivateKey *rsa.PrivateKey
	Current    *rsa.PublicKey
	Retired    []*rsa.PublicKey
}

// LoadServiceAccountKeys resolves the SA key once at startup: its own pair when
// configured, else auth.jwt. Returns ErrNoServiceAccountSigningKey when auth.jwt
// has no private key (verify-only internal_token, or idp mode).
func LoadServiceAccountKeys(cfg *config.Server) (*ServiceAccountKeys, error) {
	sa := &cfg.Auth.ServiceAccount
	jwtCfg, own := &cfg.Auth.JWT, sa.HasOwnKey()
	if own {
		jwtCfg = &sa.JWT
	} else if jwtCfg.PrivateKeyFile == "" {
		return nil, ErrNoServiceAccountSigningKey
	}
	priv, err := jwtCfg.LoadPrivateKey()
	if err != nil {
		if own {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrNoServiceAccountSigningKey, err)
	}
	keys := &ServiceAccountKeys{Issuer: jwtCfg.Issuer, OwnIssuer: own, PrivateKey: priv, Current: &priv.PublicKey}
	if own {
		if keys.Retired, err = sa.LoadRetiredPublicKeys(); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// PublicKeys returns the current key first, then the retired ones.
func (k *ServiceAccountKeys) PublicKeys() []*rsa.PublicKey {
	return append([]*rsa.PublicKey{k.Current}, k.Retired...)
}

// SATokenSigner mints service-account access tokens.
type SATokenSigner struct {
	keys     *ServiceAccountKeys
	kid      string
	ttl      time.Duration
	audience string
	claims   config.ClaimMappings
}

// NewSATokenSigner builds a signer. The kid is computed once here.
func NewSATokenSigner(keys *ServiceAccountKeys, cfg *config.Server) *SATokenSigner {
	return &SATokenSigner{
		keys:     keys,
		kid:      utils.RSAThumbprint(keys.Current),
		ttl:      cfg.Auth.ServiceAccount.TokenTTL,
		audience: cfg.Auth.ServiceAccount.Audience,
		claims:   cfg.Auth.ClaimMappings,
	}
}

// SignedToken is a minted token and what the exchange logs about it.
type SignedToken struct {
	Token     string
	JTI       string
	ExpiresAt time.Time
}

// Sign mints the token through the same claim_mappings the login endpoint uses,
// plus azp, aud, jti, the account's token version and a kid header, which
// login tokens do not carry.
func (s *SATokenSigner) Sign(sa *model.ServiceAccount, org *model.Organization, scope string) (*SignedToken, error) {
	now := time.Now()
	exp := now.Add(s.ttl)
	jti := uuid.NewString()
	sub := sa.Subject(org.Handle)

	orgClaim := org.IdpOrganizationRefUUID
	if orgClaim == "" {
		orgClaim = org.ID
	}

	claims := jwt.MapClaims{
		"sub": sub,
		"iss": s.keys.Issuer,
		"aud": s.audience,
		"exp": exp.Unix(),
		"iat": now.Unix(),
		"jti": jti,
		// Read by a Phase 2 gateway key manager as its consumer key claim.
		"azp": sa.ClientID,
		// Checked against the account's revocation watermark on every request.
		constants.ServiceAccountTokenVersionClaim: sa.TokenVersion,
	}
	utils.SetClaim(claims, utils.ClaimKey(s.claims.Username, "username"), sub)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.Scope, "scope"), scope)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.Organization, "organization"), orgClaim)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.OrgName, "org_name"), org.Name)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.OrgHandle, "org_handle"), org.Handle)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.Roles, "roles"), sa.RoleList())

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.kid
	signed, err := token.SignedString(s.keys.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign service-account token: %w", err)
	}
	return &SignedToken{Token: signed, JTI: jti, ExpiresAt: exp}, nil
}

// TTL is the lifetime of the tokens this signer mints.
func (s *SATokenSigner) TTL() time.Duration { return s.ttl }
