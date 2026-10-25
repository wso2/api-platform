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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/httpkit/httputil"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ServiceAccountHandler serves the service-account management API, the token
// endpoint and the JWKS.
type ServiceAccountHandler struct {
	svc      *service.ServiceAccountService
	identity *service.IdentityService
	// The JWKS is built once: the keys are read once, at startup.
	jwks             []byte
	jwksETag         string
	jwksCacheControl string
	slogger          *slog.Logger
}

// NewServiceAccountHandler creates the handler. keys are the SA keys the JWKS
// publishes, in config order; maxAge is its Cache-Control max-age.
func NewServiceAccountHandler(svc *service.ServiceAccountService, identity *service.IdentityService,
	keys []service.ServiceAccountKey, maxAge time.Duration, slogger *slog.Logger) *ServiceAccountHandler {
	set := api.JWKSResponse{Keys: make([]api.JWK, 0, len(keys))}
	for _, key := range keys {
		k := utils.NewRSAPublicJWK(key.Public)
		set.Keys = append(set.Keys, api.JWK{Kty: k.Kty, Kid: key.Kid, Use: &k.Use, Alg: &k.Alg, N: k.N, E: k.E})
	}
	body, _ := json.Marshal(set) // cannot fail: strings only
	sum := sha256.Sum256(body)
	return &ServiceAccountHandler{svc: svc, identity: identity, slogger: slogger,
		jwks:             body,
		jwksETag:         `"` + hex.EncodeToString(sum[:16]) + `"`,
		jwksCacheControl: "public, max-age=" + strconv.Itoa(int(maxAge.Seconds())),
	}
}

func (h *ServiceAccountHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/service-accounts"
	// A literal segment beats a wildcard in ServeMux, so /token wins over
	// /{serviceAccountId} regardless of registration order.
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.humansOnly(h.List)))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.humansOnly(h.Create)))
	mux.HandleFunc("GET "+base+"/{serviceAccountId}", middleware.MapErrors(h.slogger, h.humansOnly(h.Get)))
	mux.HandleFunc("PUT "+base+"/{serviceAccountId}", middleware.MapErrors(h.slogger, h.humansOnly(h.Update)))
	mux.HandleFunc("DELETE "+base+"/{serviceAccountId}", middleware.MapErrors(h.slogger, h.humansOnly(h.Delete)))
	mux.HandleFunc("POST "+base+"/{serviceAccountId}/regenerate-secret", middleware.MapErrors(h.slogger, h.humansOnly(h.RegenerateSecret)))
	mux.HandleFunc("POST "+base+"/token", middleware.MapErrors(h.slogger, h.Token))
	mux.HandleFunc("GET "+constants.JWKSPath, middleware.MapErrors(h.slogger, h.JWKS))
	// Not specific to service accounts, so it lives at /roles. It is registered
	// here, so it is off when service accounts are.
	mux.HandleFunc("GET "+constants.APIBasePath+"/roles", middleware.MapErrors(h.slogger, h.humansOnly(h.Roles)))
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
	opts := parseListOptions(r)
	resp, err := h.svc.List(orgID, opts.Search, opts.Limit, opts.Offset)
	if err != nil {
		return serviceError(err, "failed to list service accounts")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *ServiceAccountHandler) Roles(w http.ResponseWriter, r *http.Request) error {
	httputil.WriteJSON(w, http.StatusOK, h.svc.Roles())
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

// Token is the OAuth2 client credentials grant, form-encoded. The client may
// authenticate with HTTP Basic or with form fields (RFC 6749 section 2.3.1).
func (h *ServiceAccountHandler) Token(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return apperror.ValidationFailed.New("the request body must be form-encoded")
	}
	if r.PostForm.Get("grant_type") != string(api.ClientCredentials) {
		return apperror.ValidationFailed.New("grant_type must be client_credentials")
	}
	clientID, clientSecret, err := clientCredentials(r)
	if err != nil {
		return err
	}
	resp, err := h.svc.Exchange(service.ExchangeRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        r.PostForm.Get("scope"),
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

// JWKS publishes the SA verification keys. Public: a public key is public.
// Answers like the gateway's JWKS: ETag, a 304 on If-None-Match, Cache-Control.
// HEAD reaches here too, and net/http drops the body.
func (h *ServiceAccountHandler) JWKS(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("ETag", h.jwksETag)
	w.Header().Set("Cache-Control", h.jwksCacheControl)
	if ifNoneMatch(r.Header.Values("If-None-Match"), h.jwksETag) {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	w.Header().Set("Content-Type", "application/jwk-set+json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.jwks)
	return nil
}

// ifNoneMatch reports whether If-None-Match selects etag. The comparison is weak
// (RFC 9110 section 13.1.2): a "W/" prefix is ignored and "*" matches.
func ifNoneMatch(values []string, etag string) bool {
	for _, v := range values {
		for _, c := range strings.Split(v, ",") {
			c = strings.TrimSpace(c)
			if c == "*" || (c != "" && strings.TrimPrefix(c, "W/") == etag) {
				return true
			}
		}
	}
	return false
}

// clientCredentials reads the client ID and secret from HTTP Basic or from the
// form. Basic values are form-urlencoded first (RFC 6749 section 2.3.1). Using
// both methods at once is refused (section 2.3).
func clientCredentials(r *http.Request) (string, string, error) {
	user, pass, basic := r.BasicAuth()
	if !basic {
		return r.PostForm.Get("client_id"), r.PostForm.Get("client_secret"), nil
	}
	if r.PostForm.Has("client_id") || r.PostForm.Has("client_secret") {
		return "", "", apperror.ValidationFailed.New("use one client authentication method: HTTP Basic or form fields, not both")
	}
	id, err1 := url.QueryUnescape(user)
	secret, err2 := url.QueryUnescape(pass)
	if err1 != nil || err2 != nil {
		return "", "", apperror.Unauthorized.New().WithLogMessage("malformed HTTP Basic client credentials")
	}
	return id, secret, nil
}

// clientIP is RemoteAddr without the port. Behind a load balancer this is the
// balancer's address; trusting a forwarded header is a separate decision.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
