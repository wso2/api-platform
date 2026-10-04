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
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/api-platform/httpkit/httputil"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ServiceAccountHandler serves the service-account management API, the token
// endpoint, introspection and the JWKS.
type ServiceAccountHandler struct {
	svc         *service.ServiceAccountService
	identity    *service.IdentityService
	keyMap      *middleware.IssuerKeyMap
	revocations middleware.RevocationChecker
	jwks        api.JWKSResponse
	slogger     *slog.Logger
}

// NewServiceAccountHandler creates the handler. publicKeys are the SA keys the
// JWKS publishes: the current one first, then any retired ones.
func NewServiceAccountHandler(svc *service.ServiceAccountService, identity *service.IdentityService,
	keyMap *middleware.IssuerKeyMap, revocations middleware.RevocationChecker, publicKeys []*rsa.PublicKey,
	slogger *slog.Logger) *ServiceAccountHandler {
	jwks := api.JWKSResponse{Keys: make([]api.JWK, 0, len(publicKeys))}
	for _, pub := range publicKeys {
		k := utils.NewRSAPublicJWK(pub)
		jwks.Keys = append(jwks.Keys, api.JWK{Kty: k.Kty, Kid: k.Kid, Use: &k.Use, Alg: &k.Alg, N: k.N, E: k.E})
	}
	return &ServiceAccountHandler{svc: svc, identity: identity, keyMap: keyMap, revocations: revocations, jwks: jwks, slogger: slogger}
}

func (h *ServiceAccountHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/service-accounts"
	// A literal segment beats a wildcard in ServeMux, so /token and /jwks.json
	// win over /{serviceAccountId} regardless of registration order.
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.humansOnly(h.List)))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.humansOnly(h.Create)))
	mux.HandleFunc("GET "+base+"/{serviceAccountId}", middleware.MapErrors(h.slogger, h.humansOnly(h.Get)))
	mux.HandleFunc("PUT "+base+"/{serviceAccountId}", middleware.MapErrors(h.slogger, h.humansOnly(h.Update)))
	mux.HandleFunc("DELETE "+base+"/{serviceAccountId}", middleware.MapErrors(h.slogger, h.humansOnly(h.Delete)))
	mux.HandleFunc("POST "+base+"/{serviceAccountId}/regenerate-secret", middleware.MapErrors(h.slogger, h.humansOnly(h.RegenerateSecret)))
	mux.HandleFunc("POST "+base+"/token", middleware.MapErrors(h.slogger, h.Token))
	mux.HandleFunc("POST "+base+"/introspect", middleware.MapErrors(h.slogger, h.Introspect))
	mux.HandleFunc("GET "+base+"/jwks.json", middleware.MapErrors(h.slogger, h.JWKS))
}

// humansOnly refuses SA tokens at the management endpoints outright, so a
// service account can never create or change one.
func (h *ServiceAccountHandler) humansOnly(next func(http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		if middleware.IsServiceAccountRequest(r) {
			return apperror.Forbidden.New().WithLogMessage("service-account token refused at service-account management")
		}
		return next(w, r)
	}
}

func (h *ServiceAccountHandler) List(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	limit, offset := parsePagination(r)
	resp, err := h.svc.List(orgID, limit, offset)
	if err != nil {
		return serviceError(err, "failed to list service accounts")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ServiceAccountHandler) Create(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	var req api.ServiceAccountCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}
	actor, err := resolveActorErr(r, h.identity, "create service account")
	if err != nil {
		return err
	}
	resp, err := h.svc.Create(orgID, actor, &req)
	if err != nil {
		return serviceError(err, "failed to create service account")
	}
	setLocation(w, "service-accounts", resp.ServiceAccount.Id)
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

func (h *ServiceAccountHandler) Get(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	resp, err := h.svc.Get(orgID, r.PathValue("serviceAccountId"))
	if err != nil {
		return serviceError(err, "failed to get service account")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ServiceAccountHandler) Update(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	var req api.ServiceAccountUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}
	actor, err := resolveActorErr(r, h.identity, "update service account")
	if err != nil {
		return err
	}
	resp, err := h.svc.Update(orgID, r.PathValue("serviceAccountId"), actor, &req)
	if err != nil {
		return serviceError(err, "failed to update service account")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ServiceAccountHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	actor, err := resolveActorErr(r, h.identity, "delete service account")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(orgID, r.PathValue("serviceAccountId"), actor); err != nil {
		return serviceError(err, "failed to delete service account")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *ServiceAccountHandler) RegenerateSecret(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	actor, err := resolveActorErr(r, h.identity, "regenerate service account secret")
	if err != nil {
		return err
	}
	resp, err := h.svc.RegenerateSecret(orgID, r.PathValue("serviceAccountId"), actor)
	if err != nil {
		return serviceError(err, "failed to regenerate service account secret")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// Token is the OAuth2 client credentials grant, form-encoded.
func (h *ServiceAccountHandler) Token(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return apperror.ValidationFailed.New("the request body must be form-encoded")
	}
	if r.PostForm.Get("grant_type") != string(api.ClientCredentials) {
		return apperror.ValidationFailed.New("grant_type must be client_credentials")
	}
	resp, err := h.svc.Exchange(service.ExchangeRequest{
		ClientID:     r.PostForm.Get("client_id"),
		ClientSecret: r.PostForm.Get("client_secret"),
		ClientIP:     clientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		return serviceError(err, "failed to issue service-account token")
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// Introspect is RFC 7662. Every inactive cause returns exactly {"active":false},
// so the endpoint cannot tell a caller why a token failed.
func (h *ServiceAccountHandler) Introspect(w http.ResponseWriter, r *http.Request) error {
	inactive := api.IntrospectionResponse{Active: false}
	if err := r.ParseForm(); err != nil {
		httputil.WriteJSON(w, http.StatusOK, inactive)
		return nil
	}
	claims, err := h.keyMap.VerifyServiceAccountToken(r.PostForm.Get("token"))
	if err != nil {
		h.slogger.Debug("introspection: inactive token", "reason", err.Error())
		httputil.WriteJSON(w, http.StatusOK, inactive)
		return nil
	}
	sub, _ := claims["sub"].(string)
	accountUUID, ok := model.AccountUUIDFromSubject(sub)
	version, hasVersion := middleware.ServiceAccountTokenVersion(claims)
	if !ok || !hasVersion || h.revocations.IsRevoked(accountUUID, version) {
		httputil.WriteJSON(w, http.StatusOK, inactive)
		return nil
	}
	httputil.WriteJSON(w, http.StatusOK, activeIntrospection(claims))
	return nil
}

// JWKS publishes the SA verification keys. Public: a public key is public.
func (h *ServiceAccountHandler) JWKS(w http.ResponseWriter, r *http.Request) error {
	httputil.WriteJSON(w, http.StatusOK, h.jwks)
	return nil
}

func activeIntrospection(claims jwt.MapClaims) api.IntrospectionResponse {
	str := func(k string) *string {
		if v, ok := claims[k].(string); ok && v != "" {
			return &v
		}
		return nil
	}
	num := func(k string) *int {
		if v, ok := claims[k].(float64); ok {
			n := int(v)
			return &n
		}
		return nil
	}
	tokenType := "Bearer"
	resp := api.IntrospectionResponse{
		Active: true, Sub: str("sub"), Iss: str("iss"), Jti: str("jti"), ClientId: str("azp"),
		Scope: str("scope"), Exp: num("exp"), Iat: num("iat"), TokenType: &tokenType,
	}
	if aud, err := claims.GetAudience(); err == nil && len(aud) > 0 {
		resp.Aud = &aud[0]
	}
	return resp
}

// clientIP is RemoteAddr without the port. Behind a load balancer this is the
// balancer's address; trusting a forwarded header is a separate decision.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
