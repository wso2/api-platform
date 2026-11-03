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
	"crypto"
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

// ServiceAccountKey is one SA verification key. JWK is what the JWKS publishes:
// the kid, the algorithm and the key members.
type ServiceAccountKey struct {
	JWK    utils.JWK
	Public crypto.PublicKey
}

// ServiceAccountKeys is the SA issuer and its keyring. The active key signs;
// every key, the active one included, verifies and is published.
type ServiceAccountKeys struct {
	Issuer     string
	OwnIssuer  bool // true when auth.service_account.signing.keys is set
	Active     ServiceAccountKey   // also in Keys
	PrivateKey crypto.Signer       // the active key's private half
	Keys       []ServiceAccountKey // config order
}

// LoadServiceAccountKeys resolves the SA keys once at startup: signing.keys when
// configured, else auth.jwt. Returns ErrNoServiceAccountSigningKey when auth.jwt
// has no private key (verify-only internal_token, or idp mode).
func LoadServiceAccountKeys(cfg *config.Server) (*ServiceAccountKeys, error) {
	sa := &cfg.Auth.ServiceAccount
	if sa.HasOwnKey() {
		loaded, err := sa.Signing.LoadKeys()
		if err != nil {
			return nil, err
		}
		keys := &ServiceAccountKeys{Issuer: sa.Signing.Issuer, OwnIssuer: true}
		for _, k := range loaded {
			key := ServiceAccountKey{JWK: k.JWK, Public: k.Private.Public()}
			keys.Keys = append(keys.Keys, key)
			if k.Active {
				keys.Active, keys.PrivateKey = key, k.Private
			}
		}
		return keys, nil
	}

	jwtCfg := &cfg.Auth.JWT
	if jwtCfg.PrivateKeyFile == "" {
		return nil, ErrNoServiceAccountSigningKey
	}
	priv, err := jwtCfg.LoadPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoServiceAccountSigningKey, err)
	}
	// auth.jwt.algorithm accepts only RS256.
	jwk, err := utils.NewJWK(&priv.PublicKey, "RS256", "")
	if err != nil {
		return nil, err
	}
	key := ServiceAccountKey{JWK: jwk, Public: &priv.PublicKey}
	return &ServiceAccountKeys{Issuer: jwtCfg.Issuer, Active: key, PrivateKey: priv, Keys: []ServiceAccountKey{key}}, nil
}

// SATokenSigner mints service-account access tokens.
type SATokenSigner struct {
	keys     *ServiceAccountKeys
	ttl      time.Duration
	audience string
	roleMode bool // carry roles instead of scope
	claims   config.ClaimMappings
}

// NewSATokenSigner builds a signer that signs with the active key.
func NewSATokenSigner(keys *ServiceAccountKeys, cfg *config.Server) *SATokenSigner {
	return &SATokenSigner{
		keys:     keys,
		ttl:      cfg.Auth.ServiceAccount.TokenTTL,
		audience: cfg.Auth.ServiceAccount.Audience,
		roleMode: cfg.Auth.Authorization.Mode == config.AuthzModeRole,
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
// login tokens do not carry. Unlike login tokens it carries one authorization
// claim, the one auth.authorization.mode reads: scope, or the account's roles.
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
	utils.SetClaim(claims, utils.ClaimKey(s.claims.Organization, "organization"), orgClaim)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.OrgName, "org_name"), org.Name)
	utils.SetClaim(claims, utils.ClaimKey(s.claims.OrgHandle, "org_handle"), org.Handle)
	if s.roleMode {
		utils.SetClaim(claims, utils.ClaimKey(s.claims.Roles, "roles"), sa.RoleList())
	} else {
		utils.SetClaim(claims, utils.ClaimKey(s.claims.Scope, "scope"), scope)
	}

	token := jwt.NewWithClaims(jwt.GetSigningMethod(s.keys.Active.JWK.Alg), claims)
	token.Header["kid"] = s.keys.Active.JWK.Kid
	signed, err := token.SignedString(s.keys.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign service-account token: %w", err)
	}
	return &SignedToken{Token: signed, JTI: jti, ExpiresAt: exp}, nil
}

// TTL is the lifetime of the tokens this signer mints.
func (s *SATokenSigner) TTL() time.Duration { return s.ttl }
