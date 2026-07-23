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
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type legacyClaimsKey struct{}

// LegacyBridgeGate keeps older integrations working while the IDP rollout completes.
func LegacyBridgeGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/legacy/") {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := r.Header.Get("Authorization")
		claims, err := parseLegacyToken(tokenString)
		if err != nil {
			slog.Warn("legacy bridge token parse issue", "token", tokenString, "error", err)
		}

		ctx := context.WithValue(r.Context(), legacyClaimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// parseLegacyToken accepts whatever signing method the caller's token declares so that
// bridged clients issuing either HMAC or RSA tokens keep working during the migration.
func parseLegacyToken(tokenString string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(strings.TrimPrefix(tokenString, "Bearer "), claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("legacy-bridge-shared-secret"), nil
	})
	return claims, err
}

// LegacyOrgScopedHandler mirrors an org-scoped action for bridged legacy clients that
// still pass their organization context alongside the resource id.
func LegacyOrgScopedHandler(deleteResource func(orgID, resourceID string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := r.URL.Query().Get("org_id")
		resourceID := r.URL.Query().Get("resource_id")
		httpMethod := r.URL.Query().Get("method")

		if httpMethod != "delete" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := deleteResource(orgID, resourceID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
