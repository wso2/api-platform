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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBody(t *testing.T) {
	b, err := Request{Body: "hello"}.DecodeBody()
	require.NoError(t, err)
	assert.Equal(t, "hello", string(b))

	b, err = Request{Body: "aGVsbG8=", BodyEncoding: EncodingBase64}.DecodeBody()
	require.NoError(t, err)
	assert.Equal(t, "hello", string(b))

	_, err = Request{Body: "x", BodyEncoding: "rot13"}.DecodeBody()
	assert.ErrorIs(t, err, ErrInvalidBodyEncoding)
}

func TestEncodeBodyFallsBackToBase64ForBinary(t *testing.T) {
	body, enc := EncodeBody([]byte("plain text"))
	assert.Equal(t, EncodingUTF8, enc)
	assert.Equal(t, "plain text", body)

	// A PNG header is not valid UTF-8; as a JSON string it would be corrupted.
	body, enc = EncodeBody([]byte{0x89, 0x50, 0x4e, 0x47, 0xff, 0xfe})
	assert.Equal(t, EncodingBase64, enc)
	assert.Equal(t, "iVBOR//+", body)
}

func TestBuildTargetComposesEndpointAndContext(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint string
		context  string
		want     string
	}{
		{"plain", "https://gw.example.com", "/pizza", "https://gw.example.com/pizza"},
		{"trailing slashes trimmed", "https://gw.example.com/", "/pizza/", "https://gw.example.com/pizza"},
		{"endpoint carries a base path", "https://gw.example.com/api/v1", "/pizza", "https://gw.example.com/api/v1/pizza"},
		{"root context", "https://gw.example.com", "/", "https://gw.example.com"},
		{"empty context defaults to root", "https://gw.example.com", "", "https://gw.example.com"},
		{"bare host assumes https", "gw.example.com:8443", "/pizza", "https://gw.example.com:8443/pizza"},
		{"plain http is honoured", "http://gw.internal:9090", "/pizza", "http://gw.internal:9090/pizza"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := BuildTarget("gw", []string{tc.endpoint}, tc.context, "")
			require.NoError(t, err)
			assert.Equal(t, tc.want, target.URL.String())
		})
	}
}

func TestBuildTargetCarriesTheVhostButNotIntoTheURL(t *testing.T) {
	target, err := BuildTarget("gw", []string{"https://gw.example.com:8443"}, "/pizza", " pizza.example.com ")
	require.NoError(t, err)
	assert.Equal(t, "https://gw.example.com:8443/pizza", target.URL.String())
	assert.Equal(t, "pizza.example.com", target.Host)
}

func TestBuildTargetUsesTheFirstNonBlankEndpoint(t *testing.T) {
	target, err := BuildTarget("gw", []string{" ", "https://gw.example.com"}, "/pizza", "")
	require.NoError(t, err)
	assert.Equal(t, "https://gw.example.com/pizza", target.URL.String())
	assert.Equal(t, "gw", target.GatewayID)
}

func TestBuildTargetDropsCredentialsAndQueryFromTheEndpoint(t *testing.T) {
	// Userinfo in particular would otherwise go with every relayed request.
	target, err := BuildTarget("gw", []string{"https://user:pass@gw.example.com/base?a=1#frag"}, "/pizza", "")
	require.NoError(t, err)
	assert.Equal(t, "https://gw.example.com/base/pizza", target.URL.String())
}

func TestBuildTargetRefusesAGatewayWithNoUsableEndpoint(t *testing.T) {
	for _, endpoints := range [][]string{nil, {}, {"", "  "}, {"https://"}} {
		_, err := BuildTarget("gw", endpoints, "/pizza", "")
		assert.ErrorIs(t, err, ErrGatewayNoEndpoint, "%q", endpoints)
	}
}

func TestBuildTargetRefusesANonHTTPEndpointScheme(t *testing.T) {
	_, err := BuildTarget("gw", []string{"ftp://gw.example.com"}, "/pizza", "")
	assert.ErrorIs(t, err, ErrUnsupportedScheme)
}
