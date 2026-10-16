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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMethod(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "lowercase is normalized", in: "get", want: "GET"},
		{name: "mixed case is normalized", in: "PaTcH", want: "PATCH"},
		{name: "surrounding space is trimmed", in: "  post ", want: "POST"},
		{name: "empty defaults to GET", in: "", want: "GET"},
		{name: "CONNECT is refused", in: "CONNECT", wantErr: true},
		{name: "TRACE is refused", in: "trace", wantErr: true},
		{name: "unknown verb is refused", in: "FROBNICATE", wantErr: true},
		{name: "smuggled request line is refused", in: "GET / HTTP/1.1", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeMethod(tc.in)
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrInvalidMethod)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSanitizeHeadersRefusesHeadersTheRelayOwns(t *testing.T) {
	// Every one of these would either let the caller contradict what the relay
	// resolved, smuggle a second request past the gateway, or attach ambient
	// browser credentials to a tenant-controlled destination.
	for _, name := range []string{
		"Host", "host", "Content-Length", "Transfer-Encoding", "Connection",
		"Upgrade", "TE", "Trailer", "Proxy-Authorization", "Expect",
		"Cookie", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-IP", "Forwarded",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := SanitizeHeaders([]Header{{Name: name, Value: "x"}})
			assert.ErrorIs(t, err, ErrForbiddenHeader)
		})
	}
}

func TestSanitizeHeadersAllowsAuthorizationFromTheConsoleForm(t *testing.T) {
	// The API under test may well require one, and the value comes from the
	// console's own header list. What must never travel is the *session's*
	// token, and that is guaranteed by this package never holding it.
	got, err := SanitizeHeaders([]Header{
		{Name: "Authorization", Value: "Bearer api-under-test-token"},
		{Name: "X-API-Key", Value: "test-key"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Bearer api-under-test-token", got.Get("Authorization"))
	assert.Equal(t, "test-key", got.Get("X-API-Key"))
}

func TestSanitizeHeadersRejectsInjection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		hdr   Header
		wantE error
	}{
		{"CR in value", Header{Name: "X-Test", Value: "a\rX-Evil: 1"}, ErrInvalidHeaderValue},
		{"LF in value", Header{Name: "X-Test", Value: "a\nX-Evil: 1"}, ErrInvalidHeaderValue},
		{"NUL in value", Header{Name: "X-Test", Value: "a\x00b"}, ErrInvalidHeaderValue},
		{"space in name", Header{Name: "X Test", Value: "v"}, ErrInvalidHeaderName},
		{"colon in name", Header{Name: "X-Test:", Value: "v"}, ErrInvalidHeaderName},
		{"newline in name", Header{Name: "X-Test\r\nX-Evil", Value: "v"}, ErrInvalidHeaderName},
		{"empty name", Header{Name: "", Value: "v"}, ErrInvalidHeaderName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SanitizeHeaders([]Header{tc.hdr})
			assert.ErrorIs(t, err, tc.wantE)
		})
	}
}

func TestSanitizeHeadersEnforcesCountAndSizeCeilings(t *testing.T) {
	many := make([]Header, maxHeaderCount+1)
	for i := range many {
		many[i] = Header{Name: "X-Test", Value: "v"}
	}
	_, err := SanitizeHeaders(many)
	assert.ErrorIs(t, err, ErrTooManyHeaders)

	// Under the count ceiling but far over the byte ceiling: the two bounds are
	// independent, and a JSON size limit alone would not catch this.
	big := []Header{{Name: "X-Test", Value: strings.Repeat("a", maxHeaderBytes+1)}}
	_, err = SanitizeHeaders(big)
	assert.ErrorIs(t, err, ErrHeadersTooLarge)
}

func TestSanitizeHeadersKeepsRepeatedNames(t *testing.T) {
	got, err := SanitizeHeaders([]Header{
		{Name: "Accept", Value: "application/json"},
		{Name: "Accept", Value: "text/plain"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"application/json", "text/plain"}, got.Values("Accept"))
}

func TestSanitizeQueryEncodesRatherThanTrusts(t *testing.T) {
	got, err := SanitizeQuery([]Header{
		{Name: "q", Value: "a&b=c"},
		{Name: "next", Value: "x#frag"},
	})
	require.NoError(t, err)
	// The ampersand and hash are encoded, so neither can introduce an extra
	// parameter or a fragment into the resolved URL.
	assert.Equal(t, "next=x%23frag&q=a%26b%3Dc", got)
}

func TestSanitizeQueryRejectsControlCharacters(t *testing.T) {
	_, err := SanitizeQuery([]Header{{Name: "q\r\n", Value: "v"}})
	assert.ErrorIs(t, err, ErrInvalidQueryName)

	_, err = SanitizeQuery([]Header{{Name: "q", Value: "v\x00"}})
	assert.ErrorIs(t, err, ErrInvalidQueryName)
}

func TestResolvePathContainsRequestsWithinTheAPI(t *testing.T) {
	const base = "/pizza/v1"

	for _, tc := range []struct {
		name        string
		in          string
		wantDecoded string
		wantErr     error
	}{
		{name: "plain path", in: "/order", wantDecoded: "/pizza/v1/order"},
		{name: "root", in: "/", wantDecoded: "/pizza/v1/"},
		{name: "empty defaults to root", in: "", wantDecoded: "/pizza/v1/"},
		{name: "nested path", in: "/order/42/items", wantDecoded: "/pizza/v1/order/42/items"},

		// Containment. Each of these resolves outside the one API the caller
		// was authorized for, and reaches a sibling route on the same gateway.
		{name: "dot-dot escapes the context", in: "/../admin", wantErr: ErrPathEscapesAPI},
		{name: "deep dot-dot escapes", in: "/a/../../admin", wantErr: ErrPathEscapesAPI},
		{name: "encoded dot-dot escapes", in: "/%2e%2e/admin", wantErr: ErrPathEscapesAPI},
		{name: "encoded slash traversal escapes", in: "/..%2fadmin", wantErr: ErrPathEscapesAPI},

		// Shapes that are not a relative path at all.
		{name: "absolute URL", in: "https://evil.example.com/x", wantErr: ErrInvalidPath},
		{name: "scheme-relative authority", in: "//evil.example.com/x", wantErr: ErrInvalidPath},
		{name: "backslash authority", in: `/\evil.example.com/x`, wantErr: ErrInvalidPath},
		{name: "no leading slash", in: "order", wantErr: ErrInvalidPath},
		{name: "CRLF injection", in: "/order\r\nX-Evil: 1", wantErr: ErrInvalidPath},
		{name: "NUL byte", in: "/order\x00", wantErr: ErrInvalidPath},

		// Query and fragment have their own envelope field; smuggling them
		// through the path would bypass SanitizeQuery entirely.
		{name: "query smuggled in path", in: "/order?admin=1", wantErr: ErrInvalidPath},
		{name: "fragment smuggled in path", in: "/order#x", wantErr: ErrInvalidPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, _, err := ResolvePath(base, tc.in)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantDecoded, decoded)
		})
	}
}

func TestResolvePathPreservesEncodedSegments(t *testing.T) {
	// A path parameter whose value contains a slash is legitimate, and must
	// reach the gateway still encoded — re-sending it decoded would turn one
	// segment into two and address a different route.
	decoded, escaped, err := ResolvePath("/pizza/v1", "/order/a%2Fb")
	require.NoError(t, err)
	assert.Equal(t, "/pizza/v1/order/a/b", decoded)
	assert.Equal(t, "/pizza/v1/order/a%2Fb", escaped)
}

func TestResolvePathWithRootContext(t *testing.T) {
	decoded, _, err := ResolvePath("", "/order")
	require.NoError(t, err)
	assert.Equal(t, "/order", decoded)

	_, _, err = ResolvePath("", "/../order")
	require.NoError(t, err, "a traversal that cannot escape the root is harmless")
}

func TestDecodeBody(t *testing.T) {
	b, err := Envelope{Body: "hello"}.DecodeBody()
	require.NoError(t, err)
	assert.Equal(t, "hello", string(b))

	b, err = Envelope{Body: "aGVsbG8=", BodyEncoding: EncodingBase64}.DecodeBody()
	require.NoError(t, err)
	assert.Equal(t, "hello", string(b))

	_, err = Envelope{Body: "x", BodyEncoding: "rot13"}.DecodeBody()
	assert.ErrorIs(t, err, ErrInvalidBodyEncoding)
}

func TestEncodeBodyFallsBackToBase64ForBinary(t *testing.T) {
	body, enc := EncodeBody([]byte("plain text"))
	assert.Equal(t, EncodingUTF8, enc)
	assert.Equal(t, "plain text", body)

	// A PNG header is not valid UTF-8; carrying it as a JSON string would
	// replace the invalid bytes and corrupt the response the tester sees.
	body, enc = EncodeBody([]byte{0x89, 0x50, 0x4e, 0x47, 0xff, 0xfe})
	assert.Equal(t, EncodingBase64, enc)
	assert.NotEmpty(t, body)
}
