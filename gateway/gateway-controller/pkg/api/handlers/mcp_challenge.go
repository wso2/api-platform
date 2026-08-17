/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

// MCPChallengeMiddleware attaches the WWW-Authenticate challenge MCP clients
// need in order to start, or restart, an OAuth flow.
//
// It takes two scope sets:
//   - 401 uses initialScopes
//   - 403 uses grantingScopes
//
// A 403 that already carries a challenge — the per-tool gate's, which names the
// precise scope for that one call — is left untouched.
func MCPChallengeMiddleware(resourceMetadataURL string, initialScopes, grantingScopes []string) func(http.Handler) http.Handler {
	build := func(errCode string, scopes []string) string {
		var params []string
		if errCode != "" {
			params = append(params, fmt.Sprintf("error=%q", errCode))
		}
		if resourceMetadataURL != "" {
			params = append(params, fmt.Sprintf("resource_metadata=%q", resourceMetadataURL))
		}
		if len(scopes) > 0 {
			params = append(params, fmt.Sprintf("scope=%q", strings.Join(scopes, " ")))
		}
		if len(params) == 0 {
			return ""
		}
		return "Bearer " + strings.Join(params, ", ")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&challengeWriter{
				ResponseWriter: w,
				unauthorized:   func() string { return build("", initialScopes) },
				forbidden:      func() string { return build("insufficient_scope", grantingScopes) },
			}, r)
		})
	}
}

// challengeWriter adds the challenge header immediately before the status is
// written, the last moment at which headers can still be set.
type challengeWriter struct {
	http.ResponseWriter
	unauthorized func() string
	forbidden    func() string
	written      bool
}

func (cw *challengeWriter) WriteHeader(status int) {
	if !cw.written {
		cw.written = true
		if cw.Header().Get("WWW-Authenticate") == "" {
			var v string
			switch status {
			case http.StatusUnauthorized:
				v = cw.unauthorized()
			case http.StatusForbidden:
				v = cw.forbidden()
			}
			if v != "" {
				cw.Header().Set("WWW-Authenticate", v)
			}
		}
	}
	cw.ResponseWriter.WriteHeader(status)
}

func (cw *challengeWriter) Write(b []byte) (int, error) {
	if !cw.written {
		cw.WriteHeader(http.StatusOK)
	}
	return cw.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer, so the SDK's
// SSE flushing keeps working through this wrapper.
func (cw *challengeWriter) Unwrap() http.ResponseWriter { return cw.ResponseWriter }

// Flush satisfies handlers that type-assert http.Flusher directly.
func (cw *challengeWriter) Flush() {
	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		if !cw.written {
			cw.WriteHeader(http.StatusOK)
		}
		f.Flush()
	}
}
