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

package featureusage

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Invocation is the REST API call a feature occurrence happened in, as
// Moesif's Events API records it. It names the route, not the request: the
// URI is the route template under a fixed host, without a query, and nothing
// from the caller — headers (Host included), body, address — is carried.
type Invocation struct {
	// Method is the HTTP verb, e.g. "POST".
	Method string
	// URI is an absolute http(s) URI whose path is the route template, e.g.
	// "https://platform-api/api/v0.9/rest-apis". No query, no fragment, no
	// user info.
	URI string
	// Status is the HTTP status the call answered with.
	Status int
	// RequestTime is when the call was received.
	RequestTime time.Time
	// ResponseTime is when it was answered. Zero means now.
	ResponseTime time.Time
}

const maxURILength = 2048

// reportedOrigin is the scheme and host of every reported URI. Moesif needs an
// absolute URI, but the request's own Host header is set by whoever sends the
// request, so it is never copied; the environment metadata is what tells
// deployments apart.
const reportedOrigin = "https://platform-api"

var allowedVerbs = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true,
}

// normalize checks inv and returns it ready to send: an upper-case verb, a
// URI with only what Moesif needs, a real HTTP status, and UTC times.
func (inv Invocation) normalize(now time.Time) (Invocation, error) {
	inv.Method = strings.ToUpper(strings.TrimSpace(inv.Method))
	if !allowedVerbs[inv.Method] {
		return Invocation{}, errors.New("the invocation has an unsupported method")
	}
	if len(inv.URI) > maxURILength {
		return Invocation{}, errors.New("the invocation URI is too long")
	}
	u, err := url.Parse(inv.URI)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/") {
		return Invocation{}, errors.New("the invocation URI is not an absolute http(s) URI without a query")
	}
	// Successes and failures are both reported; the status says which.
	if inv.Status < 100 || inv.Status > 599 {
		return Invocation{}, errors.New("the invocation status is not an HTTP status")
	}
	if inv.RequestTime.IsZero() {
		return Invocation{}, errors.New("the invocation has no request time")
	}
	if inv.ResponseTime.IsZero() {
		inv.ResponseTime = now
	}
	if inv.ResponseTime.Before(inv.RequestTime) {
		inv.ResponseTime = inv.RequestTime
	}
	inv.RequestTime = inv.RequestTime.UTC()
	inv.ResponseTime = inv.ResponseTime.UTC()
	return inv, nil
}
