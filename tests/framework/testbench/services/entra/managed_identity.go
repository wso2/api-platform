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
	"cmp"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// The managed identity endpoint, in the shape Azure Container Apps and App Service serve it at the
// IDENTITY_ENDPOINT they set. The Azure Identity SDK sends a managed identity's token requests
// there whenever IDENTITY_ENDPOINT and IDENTITY_HEADER are set.
const (
	// IdentityAPIVersion is the api-version the managed identity endpoint accepts, the one the
	// Azure Identity SDK sends.
	IdentityAPIVersion = "2019-08-01"
	// IdentityHeaderName is the request header that must carry IdentityHeader.
	IdentityHeaderName = "X-IDENTITY-HEADER"
	// IdentityHeader is the secret the endpoint requires in IdentityHeaderName: the gateway
	// runtime's IDENTITY_HEADER. A request without it is answered with 403 and an empty body.
	IdentityHeader = "testbench-identity-header"
	// ManagedIdentityTenant is the tid of every token issued through the managed identity endpoint.
	ManagedIdentityTenant = "managed-identity"
	// SystemAssigned is the appid and client_id of a token requested without client_id, and the
	// identity under which the block logs those requests: /<block>/test/managed-identity/system-assigned.
	SystemAssigned = "system-assigned"
)

// Resources that select how the managed identity endpoint answers. Any other resource gets a
// token.
const (
	// ResourceNotFoundOnce answers the identity's first request with 404 and issues tokens after
	// that.
	ResourceNotFoundOnce = "api://not-found-once"
	// ResourceThrottleOnce answers the identity's first request with 429 and issues tokens after
	// that.
	ResourceThrottleOnce = "api://throttle-once"
	// ResourceErrorOnce answers the identity's first request with 500 and issues tokens after that.
	ResourceErrorOnce = "api://error-once"
	// ResourceIdentityNotFound answers every request with Container Apps' 400 for an identity the
	// app doesn't have: a client_id that isn't assigned, or no client_id without a system-assigned
	// identity.
	ResourceIdentityNotFound = "api://identity-not-found"
	// ResourceUnknown answers every request with 400 and Entra ID's refusal of an unknown resource
	// (AADSTS500011), which Container Apps passes on with the code in errorCode.
	ResourceUnknown = "api://unknown-resource"
)

// identityRejection is a managed identity token request the endpoint refuses as malformed: a status
// and Container Apps' message, or no message for an answer with an empty body.
type identityRejection struct {
	status  int
	message string
}

// identityToken serves GET /<block>/msi/token as the Container Apps identity endpoint does. Each
// block's gateway sends its managed identity requests under its own block's partition, so the
// counters that pick a fail-once answer, and the request logs, belong to the block and the identity
// together.
func (s *Service) identityToken(block string, w http.ResponseWriter, r *http.Request) {
	// ParseQuery keeps the pairs it could read even when it fails, so a malformed request is
	// still logged with what it carried.
	q, queryErr := url.ParseQuery(r.URL.RawQuery)
	req := Request{
		ClientID:       q.Get("client_id"),
		Resource:       q.Get("resource"),
		APIVersion:     q.Get("api-version"),
		IdentityHeader: r.Header.Get(IdentityHeaderName),
	}
	rejection := validateIdentityRequest(r, q, queryErr, req)
	rejected := 0
	if rejection != nil {
		rejected = rejection.status
	}

	a, admitted := s.admit(identityLogKey(block, cmp.Or(req.ClientID, SystemAssigned)), req, rejected, func(eligible int) int {
		return identityOutcome(req.Resource, eligible)
	})
	if !admitted {
		http.Error(w, "entra: partition budget exhausted", http.StatusInsufficientStorage)
		return
	}

	switch {
	case rejection != nil && rejection.message == "":
		w.WriteHeader(rejection.status)
	case rejection != nil:
		writeIdentityError(w, rejection.status, rejection.message)
	case a.status == http.StatusOK:
		issueIdentityToken(w, a.n, req)
	case req.Resource == ResourceUnknown:
		writeEntraRefusal(w, req.Resource)
	case req.Resource == ResourceIdentityNotFound && req.ClientID == "":
		writeIdentityError(w, a.status, "Unable to load the proper Managed Identity.")
	case req.Resource == ResourceIdentityNotFound:
		writeIdentityError(w, a.status, "No User Assigned or Delegated Managed Identity found for specified ClientId/ResourceId/PrincipalId.")
	default:
		w.WriteHeader(a.status)
	}
}

// identityRecorded reports GET /<block>/test/managed-identity/<identity> as {"count": n,
// "requests": [...]}: the block's managed identity requests for a client ID, or for SystemAssigned.
func (s *Service) identityRecorded(block string, w http.ResponseWriter, r *http.Request) {
	s.writeLog(w, identityLogKey(block, r.PathValue("identity")))
}

// identityLogKey is the state key of a block's identity. A tenant is a single path segment, so a
// key with a "/" never meets a tenant's.
func identityLogKey(block, identity string) string {
	return block + "/" + identity
}

// validateIdentityRequest checks a token request as the managed identity endpoint does: exactly one
// identity header carrying IdentityHeader (otherwise 403 with an empty body), then a query string
// that parses cleanly (queryErr is ParseQuery's error), no repeated query parameter, the
// api-version and a resource (otherwise 400).
func validateIdentityRequest(r *http.Request, q url.Values, queryErr error, req Request) *identityRejection {
	name, repeated := repeatedParameter(q)
	switch {
	case len(r.Header.Values(IdentityHeaderName)) != 1 || req.IdentityHeader != IdentityHeader:
		return &identityRejection{status: http.StatusForbidden}
	case queryErr != nil:
		return &identityRejection{http.StatusBadRequest, "The query string is malformed."}
	case repeated:
		return &identityRejection{http.StatusBadRequest, "The query parameter '" + name + "' is duplicated."}
	case req.APIVersion == "":
		return &identityRejection{http.StatusBadRequest, "The api-version query parameter is required."}
	case req.APIVersion != IdentityAPIVersion:
		return &identityRejection{http.StatusBadRequest, "The api-version '" + req.APIVersion + "' is not supported."}
	case req.Resource == "":
		return &identityRejection{http.StatusBadRequest, "The resource query parameter is required."}
	default:
		return nil
	}
}

// identityOutcome is the status the identity's n-th well-formed request for resource is answered
// with; malformed requests are not counted.
func identityOutcome(resource string, n int) int {
	switch {
	case resource == ResourceIdentityNotFound, resource == ResourceUnknown:
		return http.StatusBadRequest
	case resource == ResourceNotFoundOnce && n == 1:
		return http.StatusNotFound
	case resource == ResourceThrottleOnce && n == 1:
		return http.StatusTooManyRequests
	case resource == ResourceErrorOnce && n == 1:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// issueIdentityToken writes a token response in Container Apps' shape: every field a string, and
// expires_on in Unix seconds.
func issueIdentityToken(w http.ResponseWriter, n int, req Request) {
	appID := cmp.Or(req.ClientID, SystemAssigned)
	claims, err := json.Marshal(Claims{Aud: req.Resource, Tid: ManagedIdentityTenant, AppID: appID, N: n})
	if err != nil {
		http.Error(w, "entra: encoding the token claims", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"access_token": TokenPrefix + base64.RawURLEncoding.EncodeToString(claims),
		"expires_on":   strconv.FormatInt(time.Now().Unix()+ExpiresIn, 10),
		"resource":     req.Resource,
		"token_type":   "Bearer",
		"client_id":    appID,
	})
}

// writeIdentityError writes an error the managed identity endpoint answers itself, in Container
// Apps' shape, with a fresh GUID as its correlationId.
func writeIdentityError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"statusCode": status, "message": message, "correlationId": newGUID()})
}

// writeEntraRefusal writes Entra ID's refusal of an unknown resource as Container Apps passes it on:
// Entra ID's message in exceptionMessage and its code in errorCode.
func writeEntraRefusal(w http.ResponseWriter, resource string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"exceptionMessage": "AADSTS500011: The resource principal named " + resource +
			" was not found in the tenant named " + ManagedIdentityTenant + ". (testbench)",
		"errorCode":      "invalid_resource",
		"innerException": nil,
		"statusCode":     http.StatusBadRequest,
		"message":        nil,
		"correlationId":  newGUID(),
	})
}
