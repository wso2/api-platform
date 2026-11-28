//go:build integration

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

package openbao_test

// Runs against a real OpenBao (or Vault) dev server:
//
//	docker run --rm -d --name openbao -p 8200:8200 \
//	  -e BAO_DEV_ROOT_TOKEN_ID=root openbao/openbao server -dev -dev-listen-address=0.0.0.0:8200
//	OPENBAO_ADDR=http://127.0.0.1:8200 OPENBAO_ROOT_TOKEN=root \
//	  go test -tags integration ./pkg/vault/openbao/ -run Integration -v
//
// The root token only sets up the entries and a policy. The gateway reads with
// a token that holds "read" on its own assigned paths and nothing else.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault/openbao"
)

func bao(t *testing.T, addr, token, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(addr, "/")+"/v1/"+path, reader)
	require.NoError(t, err)
	req.Header.Set("X-Vault-Token", token)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestIntegration_KVResolverWithScopedToken(t *testing.T) {
	addr := os.Getenv("OPENBAO_ADDR")
	root := os.Getenv("OPENBAO_ROOT_TOKEN")
	if addr == "" || root == "" {
		t.Skip("OPENBAO_ADDR and OPENBAO_ROOT_TOKEN are not set")
	}
	run := time.Now().Format("150405.000")
	base := "gateways/it-" + strings.ReplaceAll(run, ".", "")

	// AMP (the owner) writes two versions of gw1's credential and one for gw2.
	write := func(path string, data map[string]any) {
		status, _ := bao(t, addr, root, http.MethodPost, "secret/data/"+path, map[string]any{"data": data})
		require.Equal(t, http.StatusOK, status, "write %s", path)
	}
	write(base+"/gw1/openai", map[string]any{"api_key": "sk-v1"})
	write(base+"/gw1/openai", map[string]any{"api_key": "sk-v2"})
	write(base+"/gw2/openai", map[string]any{"api_key": "sk-other"})

	// An entry with delete_version_after set carries a future deletion time on
	// every live version and must still read until then.
	write(base+"/gw1/expiring", map[string]any{"api_key": "sk-expiring"})
	status, _ := bao(t, addr, root, http.MethodPost, "secret/metadata/"+base+"/gw1/expiring",
		map[string]any{"delete_version_after": "1h"})
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status)
	write(base+"/gw1/expiring", map[string]any{"api_key": "sk-expiring-v2"})

	// gw1's identity can read only its own entries.
	policy := `path "secret/data/` + base + `/gw1/*" { capabilities = ["read"] }`
	status, _ = bao(t, addr, root, http.MethodPut, "sys/policies/acl/gw1-read-"+strings.ReplaceAll(run, ".", ""), map[string]any{"policy": policy})
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status)
	status, tok := bao(t, addr, root, http.MethodPost, "auth/token/create", map[string]any{
		"policies": []string{"gw1-read-" + strings.ReplaceAll(run, ".", "")}, "ttl": "10m", "no_default_policy": true,
	})
	require.Equal(t, http.StatusOK, status)
	gwToken := tok["auth"].(map[string]any)["client_token"].(string)

	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(gwToken), 0o600))

	kv, err := openbao.New(openbao.Config{Name: "amp-shared", Address: addr, TokenFile: tokenFile}, nil)
	require.NoError(t, err)
	v := vault.New(nil, map[string]vault.Adapter{"amp-shared": kv}, nil)

	ref := func(s string) string { return "kv://amp-shared/" + base + s }

	got, err := v.Resolve(ref("/gw1/openai#api_key"))
	require.NoError(t, err)
	assert.Equal(t, "sk-v2", got, "latest version")

	got, err = v.Resolve(ref("/gw1/openai?version=1#api_key"))
	require.NoError(t, err)
	assert.Equal(t, "sk-v1", got, "pinned version")

	got, err = v.Resolve(ref("/gw1/expiring#api_key"))
	require.NoError(t, err, "a version scheduled for deletion is still live")
	assert.Equal(t, "sk-expiring-v2", got)

	_, err = v.Resolve(ref("/gw2/openai#api_key"))
	assert.ErrorIs(t, err, vault.ErrAccessDenied, "another gateway's entry")

	_, err = v.Resolve(ref("/gw1/openai#missing"))
	assert.ErrorIs(t, err, vault.ErrNotFound, "missing field")

	_, err = v.Resolve(ref("/gw1/openai?version=7#api_key"))
	assert.ErrorIs(t, err, vault.ErrNotFound, "missing version")

	// The owner soft-deletes the latest version; the pinned older one still reads.
	status, _ = bao(t, addr, root, http.MethodPost, "secret/delete/"+base+"/gw1/openai", map[string]any{"versions": []int{2}})
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status)
	_, err = v.Resolve(ref("/gw1/openai?version=2#api_key"))
	assert.ErrorIs(t, err, vault.ErrNotFound, "deleted version")
	got, err = v.Resolve(ref("/gw1/openai?version=1#api_key"))
	require.NoError(t, err)
	assert.Equal(t, "sk-v1", got)

	// Revoking the gateway's token cuts off access.
	status, _ = bao(t, addr, root, http.MethodPost, "auth/token/revoke", map[string]any{"token": gwToken})
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status)
	_, err = v.Resolve(ref("/gw1/openai?version=1#api_key"))
	assert.ErrorIs(t, err, vault.ErrAccessDenied, "revoked token")
}
