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
	"net/url"
	"strconv"
	"time"
)

// The Azure Instance Metadata Service (IMDS) managed identity token endpoint.
const (
	// IMDSAPIVersion is the api-version the IMDS token endpoint accepts.
	IMDSAPIVersion = "2018-02-01"
	// ManagedIdentityTenant is the tid of every token issued through the IMDS endpoint.
	ManagedIdentityTenant = "managed-identity"
	// SystemAssigned is the appid of a token requested without client_id, and the identity under
	// which the block logs those requests: /<block>/test/managed-identity/system-assigned.
	SystemAssigned = "system-assigned"
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

// imdsToken serves GET /<block>/metadata/identity/oauth2/token as IMDS does. Each block's gateway
// sends its IMDS requests under its own block's partition, so the counters that pick a fail-once
// answer, and the request logs, belong to the block and the identity together.
func (s *Service) imdsToken(block string, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := Request{
		ClientID:   q.Get("client_id"),
		Resource:   q.Get("resource"),
		APIVersion: q.Get("api-version"),
		Metadata:   r.Header.Get("Metadata"),
	}
	rejection := validateIMDS(r, q, req)

	identity := req.ClientID
	if identity == "" {
		identity = SystemAssigned
	}
	n, status, admitted := s.admit(imdsLogKey(block, identity), req, rejection == nil, func(eligible int) int {
		return imdsOutcome(req.Resource, eligible)
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

// imdsRecorded reports GET /<block>/test/managed-identity/<identity> as {"count": n, "requests":
// [...]}: the block's IMDS requests for a client ID, or for SystemAssigned.
func (s *Service) imdsRecorded(block string, w http.ResponseWriter, r *http.Request) {
	s.writeLog(w, imdsLogKey(block, r.PathValue("identity")))
}

// imdsLogKey is the state key of a block's identity. A tenant is a single path segment, so a key
// with a "/" never meets a tenant's.
func imdsLogKey(block, identity string) string {
	return block + "/" + identity
}

// validateIMDS checks a token request as IMDS does: the Metadata header, no repeated query
// parameter, the api-version, a resource, and no X-Forwarded-For, which IMDS refuses so that a
// proxy cannot relay requests.
func validateIMDS(r *http.Request, q url.Values, req Request) *imdsRejection {
	name, repeated := repeatedParameter(q)
	switch {
	case req.Metadata != "true":
		return &imdsRejection{"bad_request_102", "Required metadata header not specified"}
	case repeated:
		return &imdsRejection{"invalid_request", "The query parameter '" + name + "' is duplicated"}
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

// imdsOutcome is the status the identity's n-th well-formed request for resource is answered with;
// malformed requests are not counted.
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
