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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthHandler serves public auth discovery endpoints for the login UI.
type AuthHandler struct {
	portal  config.PortalAuth
	slogger *slog.Logger
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(portal config.PortalAuth, slogger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		portal:  portal,
		slogger: slogger,
	}
}

// OrgAuthConfigResponse is the JSON shape returned by GET /portal/api/v1/organizations/{orgHandle}/auth.
type OrgAuthConfigResponse struct {
	Issuer                string   `json:"issuer"`
	ClientID              string   `json:"client_id"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	LogoutURL             string   `json:"logout_url,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	PKCERequired          bool     `json:"pkce_required"`
	ResponseType          string   `json:"response_type"`
}

// oidcDiscoveryDoc holds the subset of fields we read from /.well-known/openid-configuration.
type oidcDiscoveryDoc struct {
	Issuer                        string   `json:"issuer"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	EndSessionEndpoint            string   `json:"end_session_endpoint"`
	ResponseTypesSupported        []string `json:"response_types_supported"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	ScopesSupported               []string `json:"scopes_supported"`
}

// GetOrgAuthDiscovery handles GET /portal/api/v1/organizations/:orgHandle/auth.
// Returns the IDP configuration the login UI needs to initiate an OIDC flow.
// This endpoint is public (no auth required) and cacheable for 5 minutes.
func (h *AuthHandler) GetOrgAuthDiscovery(c *gin.Context) {
	if h.portal.DiscoveryURL == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "IDP not configured for this organization"))
		return
	}

	discovery, err := fetchOIDCDiscovery(c.Request.Context(), h.portal.DiscoveryURL)
	if err != nil {
		h.slogger.Error("Failed to fetch OIDC discovery document", "url", h.portal.DiscoveryURL, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			fmt.Sprintf("Failed to fetch IDP configuration: %v", err)))
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, OrgAuthConfigResponse{
		Issuer:                discovery.Issuer,
		ClientID:              h.portal.ClientID,
		AuthorizationEndpoint: discovery.AuthorizationEndpoint,
		TokenEndpoint:         discovery.TokenEndpoint,
		LogoutURL:             discovery.EndSessionEndpoint,
		Scopes:                intersectScopes(h.portal.Scopes, discovery.ScopesSupported),
		PKCERequired:          len(discovery.CodeChallengeMethodsSupported) > 0,
		ResponseType:          preferredResponseType(discovery.ResponseTypesSupported),
	})
}

// RegisterPublicRoutes registers the auth discovery endpoint before auth middleware.
func (h *AuthHandler) RegisterPublicRoutes(r *gin.Engine) {
	r.GET("/portal/api/v1/organizations/:orgHandle/auth", h.GetOrgAuthDiscovery)
}

// intersectScopes returns the configured portal scopes filtered to those the IDP
// advertises as supported. If the IDP returns an empty scopes_supported list (some
// IDPs omit it), all configured scopes are returned as-is.
func intersectScopes(configured []string, idpSupported []string) []string {
	if len(idpSupported) == 0 {
		return configured
	}
	supported := make(map[string]bool, len(idpSupported))
	for _, s := range idpSupported {
		supported[s] = true
	}
	result := make([]string, 0, len(configured))
	for _, s := range configured {
		if supported[s] {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return configured
	}
	return result
}

// preferredResponseType picks "code" from the IDP's supported list, falling back to
// the first entry, or "code" as a safe default when the list is empty.
func preferredResponseType(supported []string) string {
	for _, rt := range supported {
		if rt == "code" {
			return rt
		}
	}
	if len(supported) > 0 {
		return supported[0]
	}
	return "code"
}

// fetchOIDCDiscovery retrieves and parses the OIDC discovery document at discoveryURL.
// TLS verification is skipped to support local IDPs with self-signed certificates.
func fetchOIDCDiscovery(ctx context.Context, discoveryURL string) (*oidcDiscoveryDoc, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}

	var doc oidcDiscoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsing discovery document: %w", err)
	}
	return &doc, nil
}
