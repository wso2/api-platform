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

package entra

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ClientAssertionType is the only client_assertion_type the token endpoint accepts.
const ClientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"

// TokenPrefix starts every access token the service issues. The rest of the token is the
// base64url (unpadded) JSON encoding of its Claims.
const TokenPrefix = "testbench-entra."

// ExpiresIn is the lifetime, in seconds, of every issued token.
const ExpiresIn = 3599

// refreshSoonSeconds is the refresh_in a PrefixRefreshSoon tenant's first token carries.
const refreshSoonSeconds = 2

// Claims are the fields an issued token carries.
type Claims struct {
	// Aud is the requested scope without its "/.default" suffix.
	Aud string `json:"aud"`
	// Tid is the tenant the token was requested for.
	Tid string `json:"tid"`
	// AppID is the requesting client_id.
	AppID string `json:"appid"`
	// N is the token request's 1-based position for the tenant.
	N int `json:"n"`
}

// formRejection is a token request the endpoint refuses as malformed.
type formRejection struct {
	code        string
	description string
	aadsts      int
}

// token serves POST /oauth2/v2.0/token: the client credentials grant with a client assertion.
func (s *Service) token(tenant string, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	var req Request
	var rejection *formRejection
	if err := r.ParseForm(); err != nil {
		rejection = &formRejection{"invalid_request", "AADSTS900144: the request body is not a readable form", 900144}
	} else {
		f := r.PostForm
		req = Request{
			ClientID:            f.Get("client_id"),
			Scope:               f.Get("scope"),
			GrantType:           f.Get("grant_type"),
			ClientAssertionType: f.Get("client_assertion_type"),
			ClientAssertion:     f.Get("client_assertion"),
		}
		rejection = validate(req)
	}

	n, status, admitted := s.admit(tenant, req, func(n int) int {
		if rejection != nil {
			return http.StatusBadRequest
		}
		return outcome(tenant, n)
	})
	if !admitted {
		http.Error(w, "entra: tenant budget exhausted", http.StatusInsufficientStorage)
		return
	}

	switch status {
	case http.StatusBadRequest:
		writeOAuthError(w, status, tenant, n, rejection.code, rejection.description, rejection.aadsts)
	case http.StatusUnauthorized:
		writeOAuthError(w, status, tenant, n, "invalid_client",
			"AADSTS700213: No matching federated identity record found for presented assertion subject.", 700213)
	case http.StatusTooManyRequests:
		w.Header().Set("Retry-After", "1")
		writeOAuthError(w, status, tenant, n, "temporarily_unavailable", "The request was throttled.", 0)
	case http.StatusInternalServerError:
		w.WriteHeader(status)
	case http.StatusServiceUnavailable:
		writeOAuthError(w, status, tenant, n, "temporarily_unavailable", "The service is unavailable.", 0)
	default:
		if strings.HasPrefix(tenant, PrefixSlow) && !wait(r.Context(), s.slowDelay) {
			return
		}
		issue(w, tenant, n, req)
	}
}

// validate checks the client credentials grant with a client assertion, as Entra ID does.
func validate(req Request) *formRejection {
	switch {
	case req.GrantType != "client_credentials":
		return &formRejection{"unsupported_grant_type", "AADSTS70003: The app requested an unsupported grant type.", 70003}
	case req.ClientAssertionType != ClientAssertionType || req.ClientAssertion == "":
		return &formRejection{"invalid_request", "AADSTS50027: A client assertion of type jwt-bearer is required.", 50027}
	case req.ClientID == "":
		return &formRejection{"invalid_request", "AADSTS900144: The request body must contain the parameter client_id.", 900144}
	case !strings.HasSuffix(req.Scope, "/.default") || len(req.Scope) == len("/.default"):
		return &formRejection{"invalid_scope", "AADSTS1002012: The scope of a client credentials request must end in /.default.", 1002012}
	default:
		return nil
	}
}

// outcome is the status the tenant's n-th well-formed token request is answered with.
func outcome(tenant string, n int) int {
	switch {
	case strings.HasPrefix(tenant, PrefixInvalidClient):
		return http.StatusUnauthorized
	case strings.HasPrefix(tenant, PrefixRejectOnce) && n == 1:
		return http.StatusUnauthorized
	case strings.HasPrefix(tenant, PrefixThrottleOnce) && n == 1:
		return http.StatusTooManyRequests
	case strings.HasPrefix(tenant, PrefixErrorOnce) && n == 1:
		return http.StatusInternalServerError
	case strings.HasPrefix(tenant, PrefixUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusOK
	}
}

func issue(w http.ResponseWriter, tenant string, n int, req Request) {
	claims, err := json.Marshal(Claims{Aud: strings.TrimSuffix(req.Scope, "/.default"), Tid: tenant, AppID: req.ClientID, N: n})
	if err != nil {
		http.Error(w, "entra: encoding the token claims", http.StatusInternalServerError)
		return
	}
	body := map[string]any{
		"token_type":     "Bearer",
		"expires_in":     ExpiresIn,
		"ext_expires_in": ExpiresIn,
		"access_token":   TokenPrefix + base64.RawURLEncoding.EncodeToString(claims),
	}
	if strings.HasPrefix(tenant, PrefixRefreshSoon) && n == 1 {
		body["refresh_in"] = refreshSoonSeconds
	}
	writeJSON(w, http.StatusOK, body)
}

// writeOAuthError writes an Entra ID error body. aadsts is omitted from error_codes when zero.
func writeOAuthError(w http.ResponseWriter, status int, tenant string, n int, code, description string, aadsts int) {
	body := map[string]any{
		"error":             code,
		"error_description": description + " (testbench)",
		"trace_id":          fmt.Sprintf("testbench-trace-%s-%d", tenant, n),
		"correlation_id":    fmt.Sprintf("testbench-correlation-%s-%d", tenant, n),
	}
	if aadsts != 0 {
		body["error_codes"] = []int{aadsts}
	}
	writeJSON(w, status, body)
}

// parseToken decodes a token the service issued.
func parseToken(token string) (Claims, bool) {
	var c Claims
	encoded, ok := strings.CutPrefix(token, TokenPrefix)
	if !ok {
		return c, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Aud == "" {
		return Claims{}, false
	}
	return c, true
}
