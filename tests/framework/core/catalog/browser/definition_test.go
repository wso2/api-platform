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

package browser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPlaywrightVersionFromModule(t *testing.T) {
	for module, want := range map[string]string{
		"v0.6201.1": "1.62.1",
		"v0.6201.0": "1.62.1",
		"v0.6100.0": "1.61.0",
		"v0.5700.2": "1.57.0",
	} {
		got, err := playwrightVersionFromModule(module)
		require.NoError(t, err, module)
		require.Equal(t, want, got, module)
	}
	_, err := playwrightVersionFromModule("v1.2.3")
	require.Error(t, err, "a version that does not encode a playwright minor+patch must be rejected")
}

// The triad pin: the module in this build must encode exactly PlaywrightVersion, or the
// browser image and the connecting client drift apart — the binding's most-reported failure.
func TestPlaywrightVersionMatchesModule(t *testing.T) {
	require.NoError(t, PlaywrightVersionMatchesModule())
}

func TestPlaywrightImageCarriesThePin(t *testing.T) {
	require.Equal(t, "mcr.microsoft.com/playwright:v"+PlaywrightVersion+"-noble", imgPlaywright())
}

// Headless runs, CI's among them, must get exactly the run-server component.
func TestBrowserIsHeadlessUnlessAskedOtherwise(t *testing.T) {
	for _, v := range []string{"", "0", "false", "not-a-bool"} {
		t.Setenv(EnvHeaded, v)
		def := Browser()
		require.Equal(t, []string{
			"npx", "-y", "playwright@" + PlaywrightVersion,
			"run-server", "--port", "3000", "--host", "0.0.0.0",
		}, def.Cmd, v)
		require.Len(t, def.Endpoints, 1, v)
		require.Equal(t, "ws", def.Endpoints[0].Name, v)
		require.Empty(t, def.Files, v)
	}
}

func TestHeadedBrowserRunsTheProxyInPlaceOfRunServer(t *testing.T) {
	for _, v := range []string{"1", "true"} {
		t.Setenv(EnvHeaded, v)
		def := Browser()
		require.Equal(t, []string{"node", "/opt/headed-proxy.js"}, def.Cmd, v)
		require.Len(t, def.Endpoints, 1, v)
		require.Equal(t, ProxyEndpoint, def.Endpoints[0].Name, v)
		require.Equal(t, proxyPort, def.Endpoints[0].Port, v)
		require.Len(t, def.Files, 1, v)
		require.Equal(t, headedProxy, def.Files[0].Content, v)
		require.Equal(t, svcBrowser, def.Alias, "the proxy must answer at the browser's alias")
	}
}

func TestSlowMo(t *testing.T) {
	t.Setenv(EnvSlowMo, "")
	ms, err := SlowMo()
	require.NoError(t, err)
	require.Zero(t, ms)

	t.Setenv(EnvSlowMo, "250")
	ms, err = SlowMo()
	require.NoError(t, err)
	require.Equal(t, 250.0, ms)

	for _, bad := range []string{"fast", "-1"} {
		t.Setenv(EnvSlowMo, bad)
		_, err = SlowMo()
		require.Error(t, err, bad)
	}
}

// TestHeadedProxyForwardsHTTPAndTunnelsConnect runs the embedded script under the host's
// node: a plain request is forwarded, and CONNECT (HTTPS, WebSockets) is tunnelled.
func TestHeadedProxyForwardsHTTPAndTunnelsConnect(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s %s", r.Method, r.URL.RequestURI())
	}))
	t.Cleanup(backend.Close)

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, node, "-e", string(headedProxy))
	cmd.Env = append(cmd.Environ(), "PROXY_PORT="+strconv.Itoa(port))
	require.NoError(t, cmd.Start())
	proxyAddr := "127.0.0.1:" + strconv.Itoa(port)
	require.Eventually(t, func() bool {
		c, err := net.Dial("tcp", proxyAddr)
		if err == nil {
			_ = c.Close()
		}
		return err == nil
	}, 10*time.Second, 50*time.Millisecond, "the proxy never listened")

	proxyURL, err := url.Parse("http://" + proxyAddr)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	resp, err := client.Get(backend.URL + "/pets?id=42")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "GET /pets?id=42", string(body))

	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	backendHost := strings.TrimPrefix(backend.URL, "http://")
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", backendHost, backendHost)
	require.NoError(t, err)
	r := bufio.NewReader(conn)
	tunnel, err := http.ReadResponse(r, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, tunnel.StatusCode)
	_, err = fmt.Fprintf(conn, "GET /tunnelled HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", backendHost)
	require.NoError(t, err)
	inner, err := http.ReadResponse(r, nil)
	require.NoError(t, err)
	innerBody, err := io.ReadAll(inner.Body)
	require.NoError(t, err)
	require.Equal(t, "GET /tunnelled", string(innerBody))
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}
