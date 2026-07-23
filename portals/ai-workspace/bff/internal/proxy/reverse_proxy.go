/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package proxy reverse-proxies SPA API calls to the Platform API, replacing the
// nginx /api-proxy location. It strips the inbound browser Cookie and injects the
// bearer token (read by the handler straight from the HttpOnly session cookie)
// as the upstream Authorization header.
package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// tokenCtxKey is used to pass the resolved bearer token from the handler (which
// has already looked up the session) into the proxy's Rewrite hook.
type tokenCtxKey struct{}

// WithToken returns a request carrying the bearer token to inject upstream.
func WithToken(r *http.Request, token string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), tokenCtxKey{}, token))
}

func tokenFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(tokenCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// ReverseProxy builds an httputil.ReverseProxy targeting the Platform API.
// prefix (e.g. "/proxy") is stripped from the path before forwarding.
func ReverseProxy(target *url.URL, prefix string, transport http.RoundTripper) *httputil.ReverseProxy {
	rp := &httputil.ReverseProxy{
		Transport: transport,
		// FlushInterval > 0 streams responses (SSE / streamed LLM output) instead
		// of buffering, matching nginx's long proxy_read_timeout behaviour.
		FlushInterval: 200 * time.Millisecond,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host

			// Strip the same-origin proxy prefix.
			p := strings.TrimPrefix(pr.In.URL.Path, prefix)
			if p == "" {
				p = "/"
			}
			pr.Out.URL.Path = singleJoin(target.Path, p)
			pr.Out.URL.RawPath = ""

			// The browser session cookie must never reach the Platform API.
			pr.Out.Header.Del("Cookie")

			// Inject the bearer token resolved from the session.
			if tok := tokenFromContext(pr.In); tok != "" {
				pr.Out.Header.Set("Authorization", "Bearer "+tok)
			} else {
				pr.Out.Header.Del("Authorization")
			}

			pr.SetXForwarded()
		},
	}
	return rp
}

func singleJoin(a, b string) string {
	switch {
	case a == "":
		return b
	case strings.HasSuffix(a, "/") && strings.HasPrefix(b, "/"):
		return a + b[1:]
	case !strings.HasSuffix(a, "/") && !strings.HasPrefix(b, "/"):
		return a + "/" + b
	}
	return a + b
}
