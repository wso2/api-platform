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
	"net/http"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/utils"

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
}

func NewAuthLoginHandler(cfg *config.Server) *AuthLoginHandler {
	return &AuthLoginHandler{cfg: cfg}
}

func (h *AuthLoginHandler) RegisterPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/portal/v0.9/auth/login", h.Login)
}

func (h *AuthLoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := r.ParseForm(); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(utils.CodeCommonValidationFailed, "username and password are required"))
		return
	}
	req.Username = r.PostForm.Get("username")
	req.Password = r.PostForm.Get("password")
	if req.Username == "" || req.Password == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(utils.CodeCommonValidationFailed, "username and password are required"))
		return
	}

	fileBasedAuth := &h.cfg.Auth.FileBased
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
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(utils.CodeCommonUnauthorized, "Invalid or expired credentials."))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(matched.PasswordHash), []byte(req.Password)); err != nil {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(utils.CodeCommonUnauthorized, "Invalid or expired credentials."))
		return
	}

	expiry := time.Now().Add(8 * time.Hour)
	claims := jwt.MapClaims{
		"sub":          matched.Username,
		"username":     matched.Username,
		"scope":        matched.Scopes,
		"organization": fileBasedAuth.Organization.UUID,
		"org_name":     fileBasedAuth.Organization.DisplayName,
		"org_handle":   fileBasedAuth.Organization.ID,
		"iss":          h.cfg.Auth.JWT.Issuer,
		"exp":          expiry.Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.Auth.JWT.SecretKey))
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(utils.CodeCommonInternalError, "Failed to issue token."))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, loginResponse{
		Token:     signed,
		ExpiresAt: expiry.Unix(),
	})
}
