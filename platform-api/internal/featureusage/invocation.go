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
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Invocation is the REST API call a feature occurrence happened in, as
// Moesif's Events API records it. It names the route, not the request: the
// URI is the route template without a query, and nothing from the caller —
// headers, body, address — is carried at all.
type Invocation struct {
	// Method is the HTTP verb, e.g. "POST".
	Method string
	// URI is an absolute http(s) URI whose path is the route template, e.g.
	// "https://platform-api.example.com/api/v0.9/rest-apis". No query, no
	// fragment, no user info.
	URI string
	// Status is the HTTP status the call answered with.
	Status int
	// RequestTime is when the call was received.
	RequestTime time.Time
	// ResponseTime is when it was answered. Zero means now.
	ResponseTime time.Time
}

const maxURILength = 2048

var (
	// hostPattern accepts a DNS name or IPv4 address with an optional port.
	// validHost adds bracketed IPv6; anything else in the Host header — it is
	// caller-supplied — is replaced by "unknown" rather than sent.
	hostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?(:[0-9]{1,5})?$`)

	allowedVerbs = map[string]bool{
		http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
		http.MethodPatch: true, http.MethodDelete: true,
	}
)

// InvocationFromRequest describes r, answered with status, as an Invocation.
// The path is the route pattern the request matched (so a path parameter
// appears as its template, e.g. {restApiId}, never as a value) and the query
// is dropped. started is when the handler began serving r.
func InvocationFromRequest(r *http.Request, status int, started time.Time) Invocation {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	}
	host := r.Host
	if !validHost(host) {
		host = "unknown"
	}
	return Invocation{
		Method:      r.Method,
		URI:         scheme + "://" + host + routePath(r),
		Status:      status,
		RequestTime: started,
	}
}

// validHost reports whether host is a DNS name or IPv4 address, or a bracketed
// IPv6 address ("[::1]", "[2001:db8::1]:9243"), each with an optional port.
func validHost(host string) bool {
	if !strings.HasPrefix(host, "[") {
		return hostPattern.MatchString(host)
	}
	end := strings.IndexByte(host, ']')
	if end < 0 {
		return false
	}
	addr, err := netip.ParseAddr(host[1:end])
	if err != nil || !addr.Is6() || addr.Zone() != "" {
		return false
	}
	rest := host[end+1:]
	if rest == "" {
		return true
	}
	if !strings.HasPrefix(rest, ":") {
		return false
	}
	port, err := strconv.Atoi(rest[1:])
	return err == nil && port >= 1 && port <= 65535 && rest[1:] == strconv.Itoa(port)
}

// routePath returns the path of the route pattern r matched, falling back to
// the request path for a request that did not go through a ServeMux.
func routePath(r *http.Request) string {
	pattern := r.Pattern
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:] // drop the "POST " method prefix
	}
	if i := strings.IndexByte(pattern, '/'); i > 0 {
		pattern = pattern[i:] // drop a host prefix
	}
	if strings.HasPrefix(pattern, "/") {
		return pattern
	}
	return r.URL.Path
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
