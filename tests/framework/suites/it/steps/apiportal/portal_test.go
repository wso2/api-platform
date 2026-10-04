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

package apiportal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha3"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopLevelJSONArrayLength(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		want    int
		wantErr string
	}{
		{name: "empty array", body: `{"labels":[]}`, field: "labels", want: 0},
		{name: "populated array", body: `{"labels":["one","two"]}`, field: "labels", want: 2},
		{name: "missing field", body: `{}`, field: "labels", wantErr: `is absent`},
		{name: "non array", body: `{"labels":null}`, field: "labels", wantErr: `is not an array`},
		{name: "invalid JSON", body: `{`, field: "labels", wantErr: `not a JSON object`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := topLevelJSONArrayLength([]byte(tt.body), tt.field)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDevportalAPIHandle(t *testing.T) {
	tests := []struct {
		name              string
		platformAPIHandle string
		want              string
	}{
		{name: "typical handle", platformAPIHandle: "devportal-api-ab12cd34", want: "dp-devportal-api-ab12cd34"},
		{name: "empty handle", platformAPIHandle: "", want: "dp-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, devportalAPIHandle(tt.platformAPIHandle))
		})
	}
}

func TestDevportalAPIHandleDistinctFromPlatformAPIHandle(t *testing.T) {
	handle := "shared-name"
	require.NotEqual(t, handle, devportalAPIHandle(handle))
}

func TestValidWebhookSignature(t *testing.T) {
	secret := "test-signing-secret"
	body := []byte(`{"id":"application-123","name":"signed"}`)
	timestamp := "1710000000"
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(append([]byte(timestamp+"."), body...))
	require.NoError(t, err)
	header := "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name   string
		secret string
		body   []byte
		header string
		want   bool
	}{
		{name: "valid raw payload", secret: secret, body: body, header: header, want: true},
		{name: "wrong secret", secret: "other-secret", body: body, header: header, want: false},
		{name: "altered raw payload", secret: secret, body: []byte(`{"id":"application-123","name":"changed"}`), header: header, want: false},
		{name: "malformed header", secret: secret, body: body, header: "v1=invalid", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, validWebhookSignature(tt.secret, tt.body, tt.header))
		})
	}
}

func TestDecryptWebhookField(t *testing.T) {
	secret := "test-webhook-secret"
	plaintext := "generated-api-key-value"
	key, err := hkdf.Key(sha3.New256, []byte(secret), nil, webhookFieldKeyInfo, 32)
	require.NoError(t, err)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	iv := []byte("012345678901")
	sealed := gcm.Seal(nil, iv, []byte(plaintext), nil)
	tagOffset := len(sealed) - gcm.Overhead()
	envelope := map[string]any{
		"iv":         base64.StdEncoding.EncodeToString(iv),
		"tag":        base64.StdEncoding.EncodeToString(sealed[tagOffset:]),
		"ciphertext": base64.StdEncoding.EncodeToString(sealed[:tagOffset]),
	}

	decrypted, err := decryptWebhookField(secret, envelope)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}
