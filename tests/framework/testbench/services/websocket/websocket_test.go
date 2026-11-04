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

package websocket

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testClient is a minimal RFC 6455 client used only to exercise the service
// under test — it is deliberately not the production client library.
type testClient struct {
	conn net.Conn
	rw   *bufio.ReadWriter
}

func dialAndHandshake(t *testing.T, server *httptest.Server, path string) *testClient {
	t.Helper()
	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	keyBytes := make([]byte, 16)
	_, err = rand.Read(keyBytes)
	require.NoError(t, err)
	key := base64.StdEncoding.EncodeToString(keyBytes)

	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, err = conn.Write([]byte(req))
	require.NoError(t, err)

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	resp, err := http.ReadResponse(rw.Reader, nil)
	require.NoError(t, err)
	require.Equal(t, 101, resp.StatusCode, "expected a successful upgrade")

	return &testClient{conn: conn, rw: rw}
}

func dialOnly(t *testing.T, server *httptest.Server, path string) *http.Response {
	t.Helper()
	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, err = conn.Write([]byte(req))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	return resp
}

// writeClientFrame writes a masked frame, as every real client must (RFC 6455 §5.3).
func (c *testClient) writeClientFrame(t *testing.T, opcode byte, payload []byte) {
	t.Helper()
	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length <= 0xFFFF:
		header = append(header, 0x80|126, 0, 0)
		binary.BigEndian.PutUint16(header[2:], uint16(length))
	}
	var mask [4]byte
	_, err := rand.Read(mask[:])
	require.NoError(t, err)
	header = append(header, mask[:]...)

	masked := make([]byte, length)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}

	_, err = c.rw.Write(header)
	require.NoError(t, err)
	_, err = c.rw.Write(masked)
	require.NoError(t, err)
	require.NoError(t, c.rw.Flush())
}

// readServerFrame reads one unmasked server frame.
func (c *testClient) readServerFrame(t *testing.T) (opcode byte, payload []byte) {
	t.Helper()
	header := make([]byte, 2)
	_, err := readFull(c.rw, header)
	require.NoError(t, err)
	opcode = header[0] & 0x0F
	length := uint64(header[1] & 0x7F)
	if length == 126 {
		ext := make([]byte, 2)
		_, err = readFull(c.rw, ext)
		require.NoError(t, err)
		length = uint64(binary.BigEndian.Uint16(ext))
	}
	payload = make([]byte, length)
	_, err = readFull(c.rw, payload)
	require.NoError(t, err)
	return opcode, payload
}

func readFull(rw *bufio.ReadWriter, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := rw.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func TestHandshake_EchoesTextFrame(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	client := dialAndHandshake(t, server, "/some/channel")
	client.writeClientFrame(t, opText, []byte("hello"))

	opcode, payload := client.readServerFrame(t)
	require.Equal(t, byte(opText), opcode)
	require.Equal(t, "hello", string(payload))
}

// Echoes and server pushes share one connection writer; every frame must still
// arrive whole rather than interleaved with another writer's bytes.
func TestHandshake_PushAndEchoFramesDoNotInterleave(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	client := dialAndHandshake(t, server, "/rooms?pushInterval=1ms")
	_ = client.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// One echo at a time while pushes arrive every millisecond, so the server's
	// echo and push writers keep contending for the same connection.
	for i := 0; i < 200; i++ {
		client.writeClientFrame(t, opText, []byte("echo-payload"))
		for {
			opcode, payload := client.readServerFrame(t)
			require.Equal(t, byte(opText), opcode)
			text := string(payload)
			if text == "echo-payload" {
				break
			}
			require.True(t, strings.HasPrefix(text, "server-push "), "corrupted frame payload %q", text)
		}
	}
}

func TestHandshake_AnswersPingWithPong(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	client := dialAndHandshake(t, server, "/rooms")
	client.writeClientFrame(t, opPing, []byte("ping-data"))

	opcode, payload := client.readServerFrame(t)
	require.Equal(t, byte(opPong), opcode)
	require.Equal(t, "ping-data", string(payload))
}

func TestHandshake_EchoesCloseFrame(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	client := dialAndHandshake(t, server, "/rooms")
	client.writeClientFrame(t, opClose, nil)

	opcode, _ := client.readServerFrame(t)
	require.Equal(t, byte(opClose), opcode)
}

func TestHandshake_RejectStatusSkipsUpgrade(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	resp := dialOnly(t, server, "/rooms?rejectStatus=403")
	require.Equal(t, 403, resp.StatusCode)
}

func TestHandshake_SubprotocolEchoedOnlyWhenOffered(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	req := "GET /rooms?subprotocol=chat.v1 HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Sec-WebSocket-Protocol: chat.v1, chat.v2\r\n\r\n"
	_, err = conn.Write([]byte(req))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.Equal(t, 101, resp.StatusCode)
	require.Equal(t, "chat.v1", resp.Header.Get("Sec-WebSocket-Protocol"))
}

func TestHandshake_SubprotocolNotOfferedIsNotEchoed(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	resp := dialOnly(t, server, "/rooms?subprotocol=chat.v1")
	require.Equal(t, 101, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Sec-WebSocket-Protocol"))
}

func TestHealth(t *testing.T) {
	server := httptest.NewServer(New().Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
}
