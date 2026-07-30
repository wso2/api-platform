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

package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/go-httpkit/httputil"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// AuthLoginHandler issues JWT tokens for locally-configured users (file-based auth mode).
type AuthLoginHandler struct {
	cfg *config.Server
	// roleScopeMap is the role-to-scope mapping from auth.authorization.role_mappings,
	// used to expand each user's role into the scopes its token carries. In file
	// mode it is always populated: config validation requires the mapping file, and
	// startup checks every user's role against it.
	roleScopeMap map[string][]string
	slogger      *slog.Logger
}

func NewAuthLoginHandler(cfg *config.Server, roleScopeMap map[string][]string) *AuthLoginHandler {
	return &AuthLoginHandler{cfg: cfg, roleScopeMap: roleScopeMap, slogger: slog.Default()}
}

func (h *AuthLoginHandler) RegisterPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/portal/v0.9/auth/login", middleware.MapErrors(h.slogger, h.Login))
}

func (h *AuthLoginHandler) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := r.ParseForm(); err != nil {
		return apperror.ValidationFailed.New("username and password are required")
	}
	req.Username = r.PostForm.Get("username")
	req.Password = r.PostForm.Get("password")
	if req.Username == "" || req.Password == "" {
		return apperror.ValidationFailed.New("username and password are required")
	}

	fileBasedAuth := &h.cfg.Auth.File
	var matched *config.FileBasedUser
	for i := range fileBasedAuth.Users {
		if fileBasedAuth.Users[i].Username == req.Username {
			matched = &fileBasedAuth.Users[i]
			break
		}
	}

	// Use a constant-time compare even on the "user not found" path to prevent
	// timing-based username enumeration.
	if matched == nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$notarealhashjustpadding000000000000000000000000000"), []byte(req.Password))
		return apperror.Unauthorized.New().WithLogMessage("login failed: user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(matched.PasswordHash), []byte(req.Password)); err != nil {
		return apperror.Unauthorized.New().WithLogMessage("login failed: password mismatch")
	}

	// Claim names come from auth.claim_mappings — the same mapping IDP mode
	// reads incoming claims by — so a token this endpoint signs is readable by
	// validateLocalJWT (and by any other consumer configured against the same
	// mapping) without the two ever drifting apart. Mapped names are used as
	// flat claim keys here; a dot-separated nested path (meant for reading
	// externally-issued tokens) is not meaningful to sign against, so
	// validateFileModeClaimMappings rejects one at startup in this mode.
	cm := h.cfg.Auth.ClaimMappings
	expiry := time.Now().Add(h.cfg.Auth.JWT.TokenTTL)
	claims := jwt.MapClaims{
		"sub":                                     matched.Username,
		claimKey(cm.Username, "username"):         matched.Username,
		claimKey(cm.Scope, "scope"):               h.effectiveScopes(matched),
		claimKey(cm.Organization, "organization"): fileBasedAuth.Organization.UUID,
		claimKey(cm.OrgName, "org_name"):          fileBasedAuth.Organization.DisplayName,
		claimKey(cm.OrgHandle, "org_handle"):      fileBasedAuth.Organization.ID,
		"iss":                                     h.cfg.Auth.JWT.Issuer,
		"exp":                                     expiry.Unix(),
		"iat":                                     time.Now().Unix(),
	}
	// The role travels in the token as well as the scopes it expanded to, so a
	// consumer configured for role-based authorization reads the same identity
	// this endpoint authorized — the claim is a list, matching the shape IDPs
	// emit and the shape the roles claim is read back in. Config validation
	// guarantees the role is set, so this is unconditional.
	claims[claimKey(cm.Roles, "roles")] = []string{matched.Role}

	// Sign asymmetrically with RS256 using the configured RSA private key,
	// read fresh from its mounted file. Config validation (validateJWTConfig)
	// guarantees the key file is readable and matches the verification public
	// key, so a load error here is an internal fault.
	privateKey, err := h.cfg.Auth.JWT.LoadPrivateKey()
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage("failed to load JWT signing key")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage("failed to issue token")
	}

	httputil.WriteJSON(w, http.StatusOK, loginResponse{
		Token:     signed,
		ExpiresAt: expiry.Unix(),
	})
	return nil
}

// effectiveScopes returns the space-separated scope claim for a user: the scopes
// its role grants, per the mapping file. The role is the user's whole grant —
// there is no per-user scope list to drift out of sync with it — so widening or
// narrowing what a user may do is an edit to the role's entry in that one file.
//
// Authorization is still enforced against this scope claim; expanding the role at
// issue time is what lets a role-shaped configuration be checked by the scope-mode
// enforcer, rather than requiring authorization to run in role mode. Duplicates
// are dropped so a role that lists a scope twice doesn't repeat it in the claim.
func (h *AuthLoginHandler) effectiveScopes(user *config.FileBasedUser) string {
	fromRole := h.roleScopeMap[user.Role]
	scopes := make([]string, 0, len(fromRole))
	seen := make(map[string]struct{}, len(fromRole))
	for _, s := range fromRole {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		scopes = append(scopes, s)
	}
	return strings.Join(scopes, " ")
}

// claimKey returns name, falling back to def when the operator has left the
// corresponding auth.claim_mappings field unset.
func claimKey(name, def string) string {
	if name == "" {
		return def
	}
	return name
}
