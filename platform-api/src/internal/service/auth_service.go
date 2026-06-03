/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"fmt"
	"log/slog"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

// TokenResponse is the body returned by the token exchange endpoint.
type TokenResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"tokenType"`
	ExpiresIn int    `json:"expiresIn"`
}

// MembershipStore is the persistence interface used by AuthService.
// Implemented by UserOrgMembershipRepo.
type MembershipStore interface {
	// HasMembership checks whether the user has any membership record for the given org.
	HasMembership(userID, orgUUID string) (bool, error)
	// GetOrganizationsByUserID returns all organizations the user belongs to per the DB.
	GetOrganizationsByUserID(userID string) ([]*model.Organization, error)
}

// AuthService issues org-scoped platform JWTs.
//
// Token exchange is only available when enabled=true (AUTH_JWT_ENABLED=true and
// AUTH_JWT_SECRET_KEY is set). When disabled, ExchangeToken returns
// ErrTokenExchangeDisabled.
//
// Org list resolution in ExchangeToken (in priority order):
//  1. JWT `organizations` claim — preferred, no DB query.
//  2. JWT `organization` claim  — single-org fallback for older tokens.
//  3. DB membership table       — used when no org claims are present in the
//     incoming token (e.g. first login before IDP claims are populated).
//
// Membership access check follows the same priority: if the requested orgID is
// found in the resolved list, access is granted. If the list came from JWT claims
// and the orgID is still missing, a DB check is performed as a freshness fallback
// (covers the gap between org creation and the user's next JWT refresh).
type AuthService struct {
	store       MembershipStore
	secretKey   string
	issuer      string
	tokenExpiry time.Duration
	enabled     bool
	slogger     *slog.Logger
}

func NewAuthService(
	store MembershipStore,
	secretKey string,
	issuer string,
	tokenExpirySeconds int,
	enabled bool,
	slogger *slog.Logger,
) *AuthService {
	expiry := time.Duration(tokenExpirySeconds) * time.Second
	if expiry <= 0 {
		expiry = time.Hour
	}
	return &AuthService{
		store:       store,
		secretKey:   secretKey,
		issuer:      issuer,
		tokenExpiry: expiry,
		enabled:     enabled,
		slogger:     slogger,
	}
}

// ExchangeToken validates that the user has access to orgID and issues a signed
// platform JWT scoped to that org, carrying claims forwarded from the incoming token.
func (s *AuthService) ExchangeToken(
	userID, email, username, firstName, lastName, orgID, scope string,
	jwtOrgIDs []string,
	jwtOrgID string,
) (*TokenResponse, error) {
	if !s.enabled {
		return nil, constants.ErrTokenExchangeDisabled
	}

	// --- Resolve the full org list for this user ---
	//
	// Priority: JWT organizations claim → JWT organization claim → DB lookup.
	// Track whether we fetched from DB so we can skip the redundant HasMembership
	// fallback in the access check below.
	orgIDs := jwtOrgIDs
	if len(orgIDs) == 0 && jwtOrgID != "" {
		orgIDs = []string{jwtOrgID}
	}

	fromDB := false
	if len(orgIDs) == 0 && s.store != nil {
		dbOrgs, err := s.store.GetOrganizationsByUserID(userID)
		if err != nil {
			s.slogger.Warn("failed to fetch org list from DB for token exchange", "userID", userID, "error", err)
		} else {
			for _, o := range dbOrgs {
				orgIDs = append(orgIDs, o.ID)
			}
			fromDB = true
		}
	}

	// --- Validate access to the requested org ---
	hasAccess := containsOrg(orgIDs, orgID)
	if !hasAccess && !fromDB && s.store != nil {
		// JWT claims were present but may be stale (org created after last login).
		// Fall back to DB as a freshness check.
		ok, err := s.store.HasMembership(userID, orgID)
		if err != nil {
			s.slogger.Warn("membership DB fallback check failed", "orgID", orgID, "userID", userID, "error", err)
		}
		hasAccess = ok
	}
	if !hasAccess {
		return nil, constants.ErrOrganizationNotFound
	}

	// Ensure the requested org is always present in the issued token's org list,
	// even when the DB or IDP claims haven't caught up yet.
	if !containsOrg(orgIDs, orgID) {
		orgIDs = append(orgIDs, orgID)
	}

	jti, _ := utils.GenerateUUID()
	now := time.Now()
	claims := middleware.CustomClaims{
		Organization:  orgID,
		Organizations: orgIDs,
		Username:      username,
		Email:         email,
		Scope:         scope,
		FirstName:     firstName,
		LastName:      lastName,
		Audience:      orgID,
		JTI:           jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		return nil, fmt.Errorf("sign platform token: %w", err)
	}

	s.slogger.Info("Platform token issued", "userID", userID, "orgID", orgID, "expiresIn", int(s.tokenExpiry.Seconds()))

	return &TokenResponse{
		Token:     signed,
		TokenType: "Bearer",
		ExpiresIn: int(s.tokenExpiry.Seconds()),
	}, nil
}

func containsOrg(orgIDs []string, target string) bool {
	for _, id := range orgIDs {
		if id == target {
			return true
		}
	}
	return false
}
