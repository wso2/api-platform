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

package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These assert the three properties the legacy scenarios rest on. A fixture that quietly stopped
// having one of them would not fail loudly in a feature - it would make a scenario pass for the
// wrong reason - so each is pinned here, where it is cheap.

func TestLegacyServiceRefusesServerDiscover(t *testing.T) {
	srv := httptest.NewServer(NewLegacy().Handler())
	defer srv.Close()

	body := post(t, srv.URL+"/testblock"+Path, nil,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{}}`)

	require.Contains(t, body, `"code":-32601`)
	require.Contains(t, body, `"id":1`, "the refusal answers the request that was sent")
}

func TestLegacyServiceMintsASession(t *testing.T) {
	srv := httptest.NewServer(NewLegacy().Handler())
	defer srv.Close()

	resp := postResponse(t, srv.URL+"/testblock"+Path, nil, initializeBody())
	defer resp.Body.Close()

	require.NotEmpty(t, resp.Header.Get("Mcp-Session-Id"),
		"a handshake-era server identifies the session it just opened")
}

func TestLegacyServiceRefusesAModernRequest(t *testing.T) {
	srv := httptest.NewServer(NewLegacy().Handler())
	defer srv.Close()

	// The envelope 2026-07-28 requires. A stateful server cannot serve that revision, and
	// refusing it is what makes this fixture stand in for a server that predates it.
	resp := postResponse(t, srv.URL+"/testblock"+Path, map[string]string{"MCP-Protocol-Version": "2026-07-28"},
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":`+
			`{"io.modelcontextprotocol/protocolVersion":"2026-07-28",`+
			`"io.modelcontextprotocol/clientInfo":{"name":"t","version":"1"},`+
			`"io.modelcontextprotocol/clientCapabilities":{}}}}`)
	defer resp.Body.Close()

	require.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// The sessions are what make this service stateful, and the shared testbench hosts it only
// because they are isolated per block. A session opened under one block must therefore be
// unusable under another - otherwise two runners could answer each other's handshakes.
func TestLegacyServiceIsolatesSessionsByPartition(t *testing.T) {
	srv := httptest.NewServer(NewLegacy().Handler())
	defer srv.Close()

	opened := postResponse(t, srv.URL+"/blockone"+Path, nil, initializeBody())
	defer opened.Body.Close()
	session := opened.Header.Get("Mcp-Session-Id")
	require.NotEmpty(t, session)

	withSession := map[string]string{"Mcp-Session-Id": session}
	body := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`

	elsewhere := postResponse(t, srv.URL+"/blocktwo"+Path, withSession, body)
	defer elsewhere.Body.Close()
	require.NotEqual(t, http.StatusOK, elsewhere.StatusCode,
		"another block's handler must not recognise this session")
}

func initializeBody() string {
	return `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18",` +
		`"capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
}

func postResponse(t *testing.T, url string, headers map[string]string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func post(t *testing.T, url string, headers map[string]string, body string) string {
	t.Helper()
	resp := postResponse(t, url, headers, body)
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(payload)
}
