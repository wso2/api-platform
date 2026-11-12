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
	"net/http"
	"strconv"
	"time"
)

// The Azure Instance Metadata Service (IMDS) managed identity token endpoint.
const (
	// IMDSAPIVersion is the api-version the IMDS token endpoint accepts.
	IMDSAPIVersion = "2018-02-01"
	// ManagedIdentityTenant is the tid of every token issued through the IMDS endpoint.
	ManagedIdentityTenant = "managed-identity"
	// SystemAssigned is the appid of a token requested without client_id, and the suffix of the
	// system-assigned identity's request log key.
	SystemAssigned = "system-assigned"
	// ManagedIdentityLogPrefix prefixes a managed identity's request log key: mi-<client_id>, or
	// mi-system-assigned. The log is read from /<key>/test/requests.
	ManagedIdentityLogPrefix = "mi-"
)

// Resources that select how the IMDS endpoint answers. Any other resource gets a token.
const (
	// ResourceNotFoundOnce answers the identity's first request with 404, as IMDS does while it
	// starts, and issues tokens after that.
	ResourceNotFoundOnce = "api://not-found-once"
	// ResourceGoneOnce answers the identity's first request with 410, as IMDS does while it
	// updates, and issues tokens after that.
	ResourceGoneOnce = "api://gone-once"
	// ResourceThrottleOnce answers the identity's first request with 429, and issues tokens after
	// that.
	ResourceThrottleOnce = "api://throttle-once"
	// ResourceErrorOnce answers the identity's first request with 500, and issues tokens after that.
	ResourceErrorOnce = "api://error-once"
	// ResourceIdentityNotFound answers every request with 400 invalid_request "Identity not found",
	// as IMDS does for an identity not assigned to the host.
	ResourceIdentityNotFound = "api://identity-not-found"
)

// imdsRejection is an IMDS token request the endpoint refuses.
type imdsRejection struct {
	code        string
	description string
}

// imdsToken serves GET /metadata/identity/oauth2/token as IMDS does. The path partition is
// ignored: the request log is keyed by the identity, since the gateway's IMDS address is the same
// for every scenario.
func (s *Service) imdsToken(_ string, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := Request{
		ClientID:   q.Get("client_id"),
		Resource:   q.Get("resource"),
		APIVersion: q.Get("api-version"),
		Metadata:   r.Header.Get("Metadata"),
	}
	rejection := validateIMDS(r, req)

	key := ManagedIdentityLogPrefix + SystemAssigned
	if req.ClientID != "" {
		key = ManagedIdentityLogPrefix + req.ClientID
	}
	n, status, admitted := s.admit(key, req, func(n int) int {
		if rejection != nil {
			return http.StatusBadRequest
		}
		return imdsOutcome(req.Resource, n)
	})
	if !admitted {
		http.Error(w, "entra: tenant budget exhausted", http.StatusInsufficientStorage)
		return
	}

	switch status {
	case http.StatusOK:
		issueIMDS(w, n, req)
	case http.StatusBadRequest:
		if rejection == nil {
			rejection = &imdsRejection{"invalid_request", "Identity not found"}
		}
		writeJSON(w, status, map[string]string{"error": rejection.code, "error_description": rejection.description})
	default:
		w.WriteHeader(status)
	}
}

// validateIMDS checks a token request as IMDS does: the Metadata header, the api-version, a
// resource, and no X-Forwarded-For, which IMDS refuses so that a proxy cannot relay requests.
func validateIMDS(r *http.Request, req Request) *imdsRejection {
	switch {
	case req.Metadata != "true":
		return &imdsRejection{"bad_request_102", "Required metadata header not specified"}
	case req.APIVersion == "":
		return &imdsRejection{"invalid_request", "Required query variable 'api-version' is missing"}
	case req.APIVersion != IMDSAPIVersion:
		return &imdsRejection{"invalid_request", "The api-version is not supported"}
	case req.Resource == "":
		return &imdsRejection{"invalid_request", "Required query variable 'resource' is missing"}
	case r.Header.Get("X-Forwarded-For") != "":
		return &imdsRejection{"invalid_request", "Requests with an X-Forwarded-For header are not accepted"}
	default:
		return nil
	}
}

// imdsOutcome is the status the identity's n-th well-formed request for resource is answered with.
func imdsOutcome(resource string, n int) int {
	switch {
	case resource == ResourceIdentityNotFound:
		return http.StatusBadRequest
	case resource == ResourceNotFoundOnce && n == 1:
		return http.StatusNotFound
	case resource == ResourceGoneOnce && n == 1:
		return http.StatusGone
	case resource == ResourceThrottleOnce && n == 1:
		return http.StatusTooManyRequests
	case resource == ResourceErrorOnce && n == 1:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// issueIMDS writes a token response in IMDS's shape, with every number as a string.
func issueIMDS(w http.ResponseWriter, n int, req Request) {
	appID := req.ClientID
	if appID == "" {
		appID = SystemAssigned
	}
	claims, err := json.Marshal(Claims{Aud: req.Resource, Tid: ManagedIdentityTenant, AppID: appID, N: n})
	if err != nil {
		http.Error(w, "entra: encoding the token claims", http.StatusInternalServerError)
		return
	}
	now := time.Now().Unix()
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":   TokenPrefix + base64.RawURLEncoding.EncodeToString(claims),
		"client_id":      appID,
		"expires_in":     strconv.Itoa(ExpiresIn),
		"ext_expires_in": strconv.Itoa(ExpiresIn),
		"expires_on":     strconv.FormatInt(now+ExpiresIn, 10),
		"not_before":     strconv.FormatInt(now, 10),
		"resource":       req.Resource,
		"token_type":     "Bearer",
	})
}
