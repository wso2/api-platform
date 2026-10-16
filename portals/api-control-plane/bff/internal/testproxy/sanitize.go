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

package testproxy

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Errors the sanitizer and envelope decoding can produce. They are all mapped
// to one generic client-facing rejection by the handler — the distinction
// exists for the server-side log, not for the caller, who must not be told
// which specific check refused their request.
var (
	ErrInvalidMethod       = errors.New("method is not one of the methods the console offers")
	ErrInvalidPath         = errors.New("path is not a relative path within the API")
	ErrPathEscapesAPI      = errors.New("path resolves outside the API's own context")
	ErrInvalidHeaderName   = errors.New("header name is not a valid HTTP token")
	ErrInvalidHeaderValue  = errors.New("header value contains a control character")
	ErrForbiddenHeader     = errors.New("header is controlled by the relay and cannot be set by the caller")
	ErrTooManyHeaders      = errors.New("too many headers")
	ErrHeadersTooLarge     = errors.New("headers exceed the allowed total size")
	ErrTooManyQueryParams  = errors.New("too many query parameters")
	ErrInvalidQueryName    = errors.New("query parameter name contains a control character")
	ErrInvalidBodyEncoding = errors.New("body encoding must be utf8 or base64")
	ErrBodyTooLarge        = errors.New("request body exceeds the allowed size")
)

// Bounds on the envelope's own lists, independent of the JSON byte ceiling.
// A 2 MiB envelope could otherwise hold tens of thousands of one-byte headers.
const (
	maxHeaderCount     = 50
	maxHeaderBytes     = 16 << 10 // 16 KiB across all names and values
	maxQueryParamCount = 100
)

// allowedMethods is the closed set the console offers, matching the SPA's own
// HTTP_METHODS list. CONNECT and TRACE are absent deliberately: neither is a
// meaningful API call, and CONNECT would ask the relay to open a tunnel.
var allowedMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

// forbiddenHeaders are the headers a caller may never set on a relayed request.
//
// Three groups, all for different reasons:
//
//   - Hop-by-hop headers (RFC 9110 §7.6.1) describe this connection, not the
//     message, and forwarding them invites request smuggling between the relay
//     and the gateway.
//   - Headers the relay owns (Host, Content-Length) would contradict what the
//     resolved target and the actual body say.
//   - Cookie and the forwarding headers would let the browser attach ambient
//     credentials or forge the apparent client address. Nothing the browser
//     holds should travel to a tenant-controlled gateway implicitly; a tester
//     who wants a cookie sent can still say so via the Test console's own
//     header list under a name that is not Cookie.
//
// Authorization is NOT in this list: an API under test legitimately needs one,
// and the value comes from the console's own form. What must never appear is
// the *session's* token, and that is guaranteed structurally; this package
// never receives it.
var forbiddenHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"proxy-connection":    true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"host":                true,
	"content-length":      true,
	"expect":              true,
	"cookie":              true,
	"cookie2":             true,
	"forwarded":           true,
	"x-forwarded-for":     true,
	"x-forwarded-host":    true,
	"x-forwarded-proto":   true,
	"x-forwarded-port":    true,
	"x-real-ip":           true,
}

// NormalizeMethod upper-cases the caller's method before checking it against
// the allowlist. Normalizing at the point of extraction is what stops a
// lowercase "get" from missing an uppercase-keyed check and being treated as
// something other than what it is.
func NormalizeMethod(raw string) (string, error) {
	m := strings.ToUpper(strings.TrimSpace(raw))
	if m == "" {
		m = http.MethodGet
	}
	if !allowedMethods[m] {
		return "", ErrInvalidMethod
	}
	return m, nil
}

// isHTTPToken reports whether s is a valid RFC 9110 field name.
func isHTTPToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isTokenByte(s[i]) {
			return false
		}
	}
	return true
}

func isTokenByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

// hasControlChars reports whether s carries a byte that must never reach a
// header value or a URL: CR and LF (header/request splitting), NUL, and the
// rest of the C0 set apart from horizontal tab, which is legal in a field value.
func hasControlChars(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' {
			continue
		}
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// SanitizeHeaders validates the caller's header list and returns it as an
// http.Header. Anything refused fails the whole request rather than being
// silently dropped: a tester whose header was quietly discarded would be
// debugging the wrong thing.
func SanitizeHeaders(in []Header) (http.Header, error) {
	if len(in) > maxHeaderCount {
		return nil, ErrTooManyHeaders
	}
	total := 0
	out := make(http.Header, len(in))
	for _, h := range in {
		name := strings.TrimSpace(h.Name)
		if !isHTTPToken(name) {
			return nil, ErrInvalidHeaderName
		}
		if forbiddenHeaders[strings.ToLower(name)] {
			return nil, ErrForbiddenHeader
		}
		if hasControlChars(h.Value) {
			return nil, ErrInvalidHeaderValue
		}
		total += len(name) + len(h.Value)
		if total > maxHeaderBytes {
			return nil, ErrHeadersTooLarge
		}
		out.Add(name, h.Value)
	}
	return out, nil
}

// SanitizeQuery validates the caller's query parameters and encodes them.
// url.Values.Encode percent-encodes both names and values, so a value cannot
// smuggle an extra parameter or a fragment into the target URL.
func SanitizeQuery(in []Header) (string, error) {
	if len(in) > maxQueryParamCount {
		return "", ErrTooManyQueryParams
	}
	values := url.Values{}
	for _, q := range in {
		if q.Name == "" || hasControlChars(q.Name) || hasControlChars(q.Value) {
			return "", ErrInvalidQueryName
		}
		values.Add(q.Name, q.Value)
	}
	return values.Encode(), nil
}

// ResolvePath joins the caller's path onto the API's invoke path and proves the
// result still lies inside it.
//
// basePath is the invoke URL's own path (the gateway's base plus the API's
// context, e.g. "/pizza/v1"). The containment check is what stops a path of
// "/../admin" from resolving to the gateway's "/admin" — outside the one API
// the caller was authorized for. It is performed on the *decoded* path, so an
// encoded traversal (%2e%2e%2f) is caught too: url.Parse decodes it into
// u.Path before path.Clean ever sees it.
//
// Both forms are returned. Callers must assign escaped to URL.RawPath and
// decoded to URL.Path, so a legitimately percent-encoded path segment (a path
// parameter whose value contains a slash) is transmitted as the caller wrote
// it rather than being re-interpreted as a segment separator on the wire.
func ResolvePath(basePath, rawPath string) (decoded, escaped string, err error) {
	if rawPath == "" {
		rawPath = "/"
	}
	if hasControlChars(rawPath) || strings.Contains(rawPath, `\`) {
		return "", "", ErrInvalidPath
	}
	if !strings.HasPrefix(rawPath, "/") {
		return "", "", ErrInvalidPath
	}
	// "//host/x" and "/\host/x" are parsed as an authority by WHATWG URL
	// parsing, so neither is the relative path it looks like.
	if strings.HasPrefix(rawPath, "//") {
		return "", "", ErrInvalidPath
	}

	u, err := url.Parse(rawPath)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.User != nil {
		return "", "", ErrInvalidPath
	}
	// The envelope carries query parameters in their own field; a query or
	// fragment smuggled through the path field would bypass SanitizeQuery.
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", "", ErrInvalidPath
	}

	base := strings.TrimSuffix(basePath, "/")
	joined := path.Clean(base + u.Path)
	if joined != base && !strings.HasPrefix(joined, base+"/") {
		return "", "", ErrPathEscapesAPI
	}

	decoded = base + u.Path
	escaped = base + u.EscapedPath()
	if decoded == "" {
		decoded, escaped = "/", "/"
	}
	return decoded, escaped, nil
}
