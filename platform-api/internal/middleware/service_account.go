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
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	gohttpkit "github.com/wso2/api-platform/httpkit/middleware"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const keyServiceAccountUUID contextKey = "service_account_uuid"

// RevocationChecker reports whether a token carrying tokenVersion has been revoked.
type RevocationChecker interface {
	IsRevoked(accountUUID string, tokenVersion int64) bool
}

// ServiceAccountTokenVersion reads the token-version claim. It must be a
// positive whole number; anything else reads as absent.
func ServiceAccountTokenVersion(claims jwt.MapClaims) (int64, bool) {
	v, ok := claims[constants.ServiceAccountTokenVersionClaim].(float64)
	if !ok || v < 1 || v != math.Trunc(v) || v > math.MaxInt64/2 {
		return 0, false
	}
	return int64(v), true
}

// IssuerRoutingMiddleware sends a token whose iss is a local issuer to local
// verification, and everything else to the IdP chain unchanged. It is the
// only change to IdP-mode authentication: without it, no locally signed token
// is accepted there at all.
func IssuerRoutingMiddleware(keyMap *IssuerKeyMap, local AuthConfig, idpChain ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		idp := gohttpkit.Chain(idpChain...)(next)
		localHandler := LocalJWTAuthMiddleware(local)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
				claims := jwt.MapClaims{}
				if _, _, err := jwt.NewParser().ParseUnverified(tok, claims); err == nil {
					if iss, _ := claims["iss"].(string); keyMap.IsLocal(iss) {
						localHandler.ServeHTTP(w, r)
						return
					}
				}
			}
			idp.ServeHTTP(w, r)
		})
	}
}

// ServiceAccountRevocationMiddleware rejects an SA token whose token version is
// below its account's watermark. It runs after authentication and is separate from it,
// so it keeps working wherever the signature is checked. Human tokens never
// reach the cache.
func ServiceAccountRevocationMiddleware(keyMap *IssuerKeyMap, revocations RevocationChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaimsFromRequest(r)
			if !ok || claims == nil || !keyMap.IsServiceAccountToken(claims.Issuer, claims.Subject) {
				next.ServeHTTP(w, r)
				return
			}
			accountUUID, ok := model.AccountUUIDFromSubject(claims.Subject)
			if !ok || claims.ServiceAccountTokenVersion == nil {
				writeAuthError(w, "service-account token has a malformed subject or no token version")
				return
			}
			if revocations.IsRevoked(accountUUID, *claims.ServiceAccountTokenVersion) {
				writeAuthError(w, "service-account token was revoked")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyServiceAccountUUID, accountUUID)))
		})
	}
}

// IsServiceAccountRequest reports whether r was authenticated by an SA token.
func IsServiceAccountRequest(r *http.Request) bool {
	_, ok := getStringFromCtx(r, keyServiceAccountUUID)
	return ok
}

// WithServiceAccount marks r as SA-authenticated. For tests.
func WithServiceAccount(r *http.Request, accountUUID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), keyServiceAccountUUID, accountUUID))
}
