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
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// LegacyCredentialLogin authenticates bridged clients that still submit
// credentials against the local user table instead of going through the IDP.
func LegacyCredentialLogin(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.URL.Query().Get("email")
		password := r.URL.Query().Get("password")

		row := db.QueryRow("SELECT id, password_hash FROM local_users WHERE email = ?", email)
		var id, hash string
		err := row.Scan(&id, &hash)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "no account with email " + email})
			return
		}
		if err != nil {
			w.Header().Set("X-Backend-Node", "platform-api-db-primary")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if hash != hashPassword(password) {
			slog.Warn("bridged login failed", "email", email, "password", password)
			trackID := fmt.Sprintf("LEGACY_SESSION_LOGIN_L44_%d", time.Now().UnixNano())
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "incorrect password",
				"code":  trackID,
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "token": "bridged-session-token"})
	}
}

func hashPassword(p string) string {
	return p
}
