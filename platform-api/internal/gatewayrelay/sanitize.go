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

package gatewayrelay

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Errors from checking the request. The handler answers all of them with one
// generic rejection; which check failed is for the log, not the caller.
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

// Bounds on the request's lists, independent of its byte limit, which alone
// would allow tens of thousands of one-byte headers.
const (
	maxHeaderCount     = 50
	maxHeaderBytes     = 16 << 10 // 16 KiB across all names and values
	maxQueryParamCount = 100
)

// allowedMethods is the set the console offers. CONNECT and TRACE are left out:
// neither is an API call, and CONNECT would ask the relay to open a tunnel.
var allowedMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

// forbiddenHeaders may never be set on a relayed request: hop-by-hop headers
// (RFC 9110 §7.6.1) invite request smuggling, Host and Content-Length belong to
// the relay, and Cookie and the forwarding headers would carry ambient
// credentials or forge the client address. Authorization stays allowed for the
// API under test; the caller's own token never reaches this package.
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

// NormalizeMethod upper-cases the method before checking it, so a lowercase
// "get" cannot miss the uppercase allowlist. An empty method is GET.
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

// hasControlChars reports a C0 control or DEL, which would allow header or
// request splitting. Tab is legal in a field value.
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

// SanitizeHeaders validates the caller's headers. A refused header fails the
// whole request rather than being dropped, which would mislead the tester.
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

// SanitizeQuery validates and encodes the query parameters. Encoding names and
// values keeps a value from adding a parameter or a fragment.
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

// ResolvePath joins the caller's path onto the invoke URL's path (basePath, e.g.
// "/pizza/v1") and refuses a result outside it, so "/../admin" cannot reach the
// gateway's "/admin". The check runs on the decoded path, catching %2e%2e too.
//
// Callers set URL.Path to decoded and URL.RawPath to escaped, so an encoded
// slash inside a segment is sent as written.
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
	// A URL parser reads "//host/x" as an authority, not a path.
	if strings.HasPrefix(rawPath, "//") {
		return "", "", ErrInvalidPath
	}

	u, err := url.Parse(rawPath)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.User != nil {
		return "", "", ErrInvalidPath
	}
	// Query parameters have their own field; one in the path would bypass SanitizeQuery.
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
