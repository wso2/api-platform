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

package mcpserver

import (
	"fmt"
	"net/http"

	"github.com/wso2/api-platform/common/authenticators"
)

// Wrap is a middleware for the Platform API's chain, placed after CORS and
// before the sign-in. For /mcp it refuses a request from a browser page on
// another site with 403. When login discovery is on (idp mode) it also adds
// the discovery header to every 401. Other paths pass through.
func (s *Server) Wrap(next http.Handler) http.Handler {
	inner := next
	if s.discoveryEnabled() {
		inner = s.challenge(next)
	}
	guarded := http.NewCrossOriginProtection().Handler(inner)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authenticators.HasPathPrefix(r.URL.Path, []string{Path}) {
			guarded.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// challenge adds WWW-Authenticate to every 401, pointing at the discovery route.
func (s *Server) challenge(next http.Handler) http.Handler {
	value := fmt.Sprintf(`Bearer resource_metadata=%q`, s.publicURL(DiscoveryPath))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&challengeWriter{ResponseWriter: w, value: value}, r)
	})
}

type challengeWriter struct {
	http.ResponseWriter
	value string
}

func (w *challengeWriter) WriteHeader(code int) {
	if code == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", w.value)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *challengeWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
