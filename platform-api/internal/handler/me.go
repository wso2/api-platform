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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/middleware"

	"github.com/wso2/go-httpkit/httputil"
)

// MeHandler serves the signed-in user's identity and effective permissions on the
// portal API surface (portal-api.yaml), alongside the portal login endpoint.
//
// It exists because a token issued in role mode carries roles, not scopes: a portal
// that derives its UI from the scope claim would see an empty list and hide
// everything the user is in fact allowed to do. The scopes reported here are
// resolved through middleware.EffectiveScopes — the same function the scope
// enforcer authorizes against — so a portal can never show an action the API would
// then reject.
//
// The route requires authentication but carries no scope requirement: it is absent
// from the main OpenAPI scope registry, and routes the registry does not know are
// passed through by the enforcer. A portal must be able to ask what the user may do
// before it knows what the user may do.
type MeHandler struct {
	validationMode string
	slogger        *slog.Logger
}

func NewMeHandler(validationMode string, slogger *slog.Logger) *MeHandler {
	return &MeHandler{validationMode: validationMode, slogger: slogger}
}

func (h *MeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/portal/v0.9/me", middleware.MapErrors(h.slogger, h.GetMe))
}

type meOrganization struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

type meResponse struct {
	UserID       string          `json:"userId"`
	Username     string          `json:"username,omitempty"`
	Email        string          `json:"email,omitempty"`
	Organization *meOrganization `json:"organization,omitempty"`
	Roles        []string        `json:"roles"`
	Scopes       []string        `json:"scopes"`
}

// GetMe returns the authenticated caller's identity, IDP roles, and the scopes
// those roles (or the token's scope claim) grant.
func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) error {
	// The route is authentication-required. If the validated token carries no
	// resolvable user identity, fail closed rather than reporting an identity
	// record with an empty userId.
	userID, ok := middleware.GetUserIDFromRequest(r)
	if !ok || userID == "" {
		return apperror.Unauthorized.New().
			WithLogMessage("user ID not found in token")
	}

	username, _ := middleware.GetUsernameFromRequest(r)
	email, _ := middleware.GetEmailFromRequest(r)

	roles, _ := middleware.GetRolesClaimFromRequest(r)
	scopes := middleware.EffectiveScopes(r, h.validationMode)

	resp := meResponse{
		UserID:   userID,
		Username: username,
		Email:    email,
		// Marshal as [] rather than null so clients can index without a nil check.
		Roles:  nonNil(roles),
		Scopes: nonNil(scopes),
	}

	if orgID, ok := middleware.GetOrganizationFromRequest(r); ok && orgID != "" {
		orgName, _ := middleware.GetOrgNameFromRequest(r)
		orgHandle, _ := middleware.GetOrgHandleFromRequest(r)
		resp.Organization = &meOrganization{ID: orgID, Name: orgName, Handle: orgHandle}
	}

	// Permissions are per-user and change with the token; never let a shared cache
	// serve one user's scope list to another.
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
