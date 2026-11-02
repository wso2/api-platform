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

// Package gatewayrelay sends a test request to a REST API deployed on a
// gateway and returns the gateway's complete response as data. It backs
// POST /rest-apis/{restApiId}/gateways/{gatewayId}/invoke, the console's Test
// console, which cannot call a gateway from the browser because of CORS,
// mixed content, or a gateway the user's network cannot reach.
//
// The caller never supplies a URL: the handler resolves the target from the
// REST API and the gateway it is deployed to (see BuildTarget), and the egress
// policy decides whether the control plane may connect to it at all. The
// caller's own credentials never reach the gateway: Relay forwards only the
// sanitized headers of the request it is given.
package gatewayrelay

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Body encodings used in both directions. A body travels as a plain string when
// it is valid UTF-8, and as base64 otherwise, so binary data survives the JSON
// round trip.
const (
	EncodingUTF8   = "utf8"
	EncodingBase64 = "base64"
)

// Header is one header or query parameter. A list preserves repeated names.
type Header struct {
	Name  string
	Value string
}

// Request is the test request to send. Path is relative to the API's invoke
// URL, leading slash included.
type Request struct {
	Method       string
	Path         string
	Query        []Header
	Headers      []Header
	Body         string
	BodyEncoding string
}

// DecodeBody returns the request body, decoding base64 when specified.
// Unrecognized encodings return an error.
func (r Request) DecodeBody() ([]byte, error) {
	switch r.BodyEncoding {
	case "", EncodingUTF8:
		return []byte(r.Body), nil
	case EncodingBase64:
		return base64.StdEncoding.DecodeString(r.Body)
	default:
		return nil, ErrInvalidBodyEncoding
	}
}

// Relayed is the gateway's response as data. Status is the gateway's status; a
// successful relay is always answered with 200, so the console can tell a
// gateway error from a relay error.
type Relayed struct {
	Status     int
	StatusText string
	Headers    []Header
	Body       string
	// BodyEncoding is EncodingUTF8 or EncodingBase64; see Body.
	BodyEncoding string
	// Truncated reports that the body exceeded MaxResponseBytes and was clipped.
	Truncated bool
	// DurationMs is the gateway round trip alone, without target resolution.
	DurationMs int64
}

// EncodeBody renders response bytes for the JSON response, choosing base64 only
// when the payload is not valid UTF-8.
func EncodeBody(b []byte) (body, encoding string) {
	if utf8.Valid(b) {
		return string(b), EncodingUTF8
	}
	return base64.StdEncoding.EncodeToString(b), EncodingBase64
}

// Errors from BuildTarget. Each has its own client-facing response.
var (
	ErrGatewayNoEndpoint = errors.New("gateway has no usable endpoint")
	ErrUnsupportedScheme = errors.New("gateway endpoint scheme is not http or https")
)

// Target is a resolved invoke destination.
type Target struct {
	// URL is the API's invoke URL on the gateway: origin plus the API's
	// context, with no trailing slash. The relayed path is appended to it.
	URL *url.URL
	// GatewayID names the gateway in log lines.
	GatewayID string
}

// BuildTarget composes the invoke URL from the gateway's first usable endpoint
// and the API's context, the same way the console builds the URL it shows, so
// the URL shown and the URL dialed cannot drift apart.
func BuildTarget(gatewayID string, endpoints []string, apiContext string) (Target, error) {
	endpoint := ""
	for _, e := range endpoints {
		if strings.TrimSpace(e) != "" {
			endpoint = strings.TrimSpace(e)
			break
		}
	}
	if endpoint == "" {
		return Target{}, ErrGatewayNoEndpoint
	}
	// The console tolerates a bare host and assumes https; match it rather than
	// refuse a record the console renders.
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || u.Host == "" {
		return Target{}, ErrGatewayNoEndpoint
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Target{}, ErrUnsupportedScheme
	}

	ctxPath := strings.TrimSpace(apiContext)
	if !strings.HasPrefix(ctxPath, "/") {
		ctxPath = "/" + ctxPath
	}

	// Only scheme, host and path make up an invoke URL; a query, fragment or
	// userinfo on the endpoint is dropped rather than sent with every request.
	return Target{
		URL: &url.URL{
			Scheme: u.Scheme,
			Host:   u.Host,
			Path:   strings.TrimRight(u.Path, "/") + strings.TrimRight(ctxPath, "/"),
		},
		GatewayID: gatewayID,
	}, nil
}
