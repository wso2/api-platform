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
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
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

// claimsTable builds a two-column table from name/value pairs.
func claimsTable(t *testing.T, pairs ...string) *godog.Table {
	t.Helper()
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	var rows []row
	for i := 0; i+1 < len(pairs); i += 2 {
		rows = append(rows, row{Cells: []cell{{pairs[i]}, {pairs[i+1]}}})
	}
	raw, err := json.Marshal(map[string]any{"rows": rows})
	require.NoError(t, err)
	var table godog.Table
	require.NoError(t, json.Unmarshal(raw, &table))
	return &table
}

func TestClaimsFromTableDecodesListsMapsAndQuotedStrings(t *testing.T) {
	claims, err := claimsFromTable(context.Background(), claimsTable(t,
		"sub", "ivy",
		"org_id", `["acme"]`,
		"org_map", `{"acme": {"id": "1"}}`,
		"padded", `"default "`,
		"org_name", "Acme Corp",
	))
	require.NoError(t, err)
	require.Equal(t, "ivy", claims["sub"])
	require.Equal(t, []any{"acme"}, claims["org_id"])
	require.Equal(t, map[string]any{"acme": map[string]any{"id": "1"}}, claims["org_map"])
	require.Equal(t, "default ", claims["padded"], "a quoted value keeps the space Gherkin would trim")
	require.Equal(t, "Acme Corp", claims["org_name"])
}

func TestClaimsFromTableRejectsMalformedTables(t *testing.T) {
	cases := map[string]*godog.Table{
		"nil table":    nil,
		"missing sub":  claimsTable(t, "org_id", "acme"),
		"empty sub":    claimsTable(t, "sub", ""),
		"empty name":   claimsTable(t, "sub", "u", " ", "value"),
		"duplicate":    claimsTable(t, "sub", "u", "sub", "v"),
		"invalid JSON": claimsTable(t, "sub", "u", "roles", "[unclosed"),
		"wrong columns": func() *godog.Table {
			table := claimsTable(t, "sub", "u")
			table.Rows[0].Cells = table.Rows[0].Cells[:1]
			return table
		}(),
	}
	for name, table := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := claimsFromTable(context.Background(), table)
			require.Error(t, err)
		})
	}
}

func TestPortalDatabaseBindsPlaceholdersForItsEngine(t *testing.T) {
	query := "SELECT uuid FROM organizations WHERE idp_ref_id = ? AND portal_id = ?"
	require.Equal(t, "SELECT uuid FROM organizations WHERE idp_ref_id = $1 AND portal_id = $2",
		(&portalDatabase{engine: components.Postgres}).bind(query))
	require.Equal(t, "SELECT uuid FROM organizations WHERE idp_ref_id = @p1 AND portal_id = @p2",
		(&portalDatabase{engine: components.SQLServer}).bind(query))
	require.Equal(t, query, (&portalDatabase{engine: components.SQLite}).bind(query))
	require.Equal(t, "SELECT 1", (&portalDatabase{engine: components.Postgres}).bind("SELECT 1"))
}

func TestBrowserRedirectsAreRoutedToTheirTarget(t *testing.T) {
	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		return u
	}
	require.True(t, isIdentityProvider(parse("https://testbench:3014/oauth2/authorize?org=a")))
	require.False(t, isIdentityProvider(parse("/api-portal/default/views/default")))
	require.False(t, isIdentityProvider(parse("http://api-portal-multi-organization:9543/api-portal/default/callback")))
	require.False(t, isIdentityProvider(parse("https://testbench.evil.example/oauth2/authorize")))

	require.Equal(t, "/api-portal/default/callback?code=c&state=s",
		portalTarget(parse("http://api-portal-multi-organization:9543/api-portal/default/callback?code=c&state=s")))
	require.Equal(t, "/api-portal/a%2Fb/views/default", portalTarget(parse("/api-portal/a%2Fb/views/default")))

	for status, want := range map[int]bool{
		http.StatusFound: true, http.StatusSeeOther: true, http.StatusMovedPermanently: true,
		http.StatusTemporaryRedirect: true, http.StatusOK: false, http.StatusNotModified: false,
	} {
		require.Equal(t, want, isRedirect(status), status)
	}
}

func withScenarioPartition(t *testing.T, partition string) context.Context {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.NoError(t, tcontext.Set(ctx, "apiPortalRunnerPartition", partition))
	return ctx
}

func TestBrowserStateIsScopedToTheScenario(t *testing.T) {
	s := &Steps{}
	ctx := withScenarioPartition(t, "scenario-one")
	require.NoError(t, s.setBrowserValue(ctx, browserCookieKey, "sam", "sid=1"))
	require.Equal(t, "sid=1", s.browserValue(ctx, browserCookieKey, "sam"))
	require.Empty(t, s.browserValue(ctx, browserCookieKey, "ada"), "each browser has its own state")
	require.Empty(t, s.browserValue(ctx, browserSessionKey, "sam"), "each kind of state has its own key")

	require.NoError(t, tcontext.Set(ctx, "apiPortalRunnerPartition", "scenario-two"))
	require.Empty(t, s.browserValue(ctx, browserCookieKey, "sam"), "a later scenario starts with a fresh browser")
}

func TestSinkPartitionsAreScenarioScopedPathSegments(t *testing.T) {
	s := &Steps{}
	ctx := withScenarioPartition(t, "api-portal-testbench-abc-1")
	partition, err := s.sinkPartition(ctx, "here")
	require.NoError(t, err)
	require.Equal(t, "api-portal-testbench-abc-1-here", partition)
	for _, bad := range []string{"", " ", "a/b", "a?b", "a#b"} {
		_, err := s.sinkPartition(ctx, bad)
		require.Error(t, err, bad)
	}
}

func TestRESTAPIMetadataIsPublishedJSON(t *testing.T) {
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(restAPIMetadata("mt-api-1")), &metadata))
	require.Equal(t, "mt-api-1", metadata["id"])
	require.Equal(t, "PUBLISHED", metadata["status"])
	require.Equal(t, "REST", metadata["type"])
}

func TestTheIdentityProviderClientVerifiesTheProviderCertificate(t *testing.T) {
	pair := shared.IdentityProviderTLS()
	certificate, err := tls.X509KeyPair(pair.CertPEM, pair.PrivateKeyPEM)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	defer server.Close()

	client, err := newIdentityProviderClient()
	require.NoError(t, err)
	response, err := client.Do(context.Background(), httpx.Request{URL: server.URL}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.StatusCode)

	other := httptest.NewTLSServer(http.NotFoundHandler())
	defer other.Close()
	_, err = client.Do(context.Background(), httpx.Request{URL: other.URL}, 0, 0)
	require.Error(t, err, "a certificate other than the provider's must not be trusted")
}
