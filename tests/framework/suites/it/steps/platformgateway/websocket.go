/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package platformgateway

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// keyWSConnection holds the open WebSocket connection for the scenario, so a handshake step
// and the frame steps that follow it can share one socket.
const keyWSConnection = "websocketConnection"

// wsFrameTimeout bounds every frame read: a frame that never arrives must fail the step rather
// than hang the scenario.
const wsFrameTimeout = 20 * time.Second

// WebSocket opcodes (RFC 6455 §5.2), duplicated from the testbench's own copy: the two are
// independent implementations of the same wire protocol, not a shared library, so there is
// nothing to import between a test client and the server it dials.
const (
	wsOpText  = 0x1
	wsOpClose = 0x8
	wsOpPing  = 0x9
	wsOpPong  = 0xA
)

// wsConnection is the live socket a handshake step opened, read by the frame steps that follow.
type wsConnection struct {
	conn net.Conn
	rw   *bufio.ReadWriter
}

func (g *Gateway) registerWebSocketSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I open a WebSocket connection to "([^"]*)"$`, g.openWebSocketConnection)
	sc.Step(`^I open a WebSocket connection to "([^"]*)" with subprotocol "([^"]*)"$`,
		g.openWebSocketConnectionWithSubprotocol)
	sc.Step(`^I open a WebSocket connection to "([^"]*)" until status (\d+)$`,
		g.openWebSocketConnectionUntilStatus)
	sc.Step(`^I open a WebSocket connection to "([^"]*)" with subprotocol "([^"]*)" until status (\d+)$`,
		g.openWebSocketConnectionWithSubprotocolUntilStatus)
	sc.Step(`^I send a WebSocket text frame "([^"]*)"$`, g.sendWebSocketTextFrame)
	sc.Step(`^I should receive the WebSocket text frame "([^"]*)"$`, g.receiveWebSocketTextFrame)
	sc.Step(`^I send a WebSocket ping$`, g.sendWebSocketPing)
	sc.Step(`^I should receive a WebSocket pong$`, g.receiveWebSocketPong)
	sc.Step(`^I close the WebSocket connection$`, g.closeWebSocketConnection)

	// A scenario that fails between opening a connection and closing it must not leak the
	// socket into the next scenario on this runner.
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if v, ok := tcontext.Get(ctx, keyWSConnection); ok {
			if c, ok := v.(*wsConnection); ok {
				_ = c.conn.Close()
			}
		}
		return ctx, nil
	})
}

// openWebSocketConnection performs the RFC 6455 handshake against a data-plane path and, on a
// successful upgrade, keeps the connection open for the frame steps that follow.
//
// This is a deliberate, narrow exception to the funnel rule: a WebSocket upgrade is not an
// ordinary HTTP exchange the funnel's client can express, so the handshake is dialed directly —
// mirroring registerRawHTTPSteps' own raw-socket exception — and its result is published into
// the same response slot so the generic status-code assertion still applies to it.
func (g *Gateway) openWebSocketConnection(ctx context.Context, path string) error {
	return g.openWebSocket(ctx, path, "")
}

func (g *Gateway) openWebSocketConnectionWithSubprotocol(ctx context.Context, path, subprotocol string) error {
	return g.openWebSocket(ctx, path, subprotocol)
}

// openWebSocketConnectionUntilStatus retries the handshake until it answers with want, mirroring
// Base.sendUntilStatus: a handshake fired immediately after a deploy/update races the same
// store-to-xDS-to-Envoy propagation gap an ordinary data-plane request does, and a channel route
// is no different from any other route in that respect.
func (g *Gateway) openWebSocketConnectionUntilStatus(ctx context.Context, path string, want int) error {
	return g.openWebSocketUntilStatus(ctx, path, "", want)
}

func (g *Gateway) openWebSocketConnectionWithSubprotocolUntilStatus(ctx context.Context, path, subprotocol string, want int) error {
	return g.openWebSocketUntilStatus(ctx, path, subprotocol, want)
}

func (g *Gateway) openWebSocketUntilStatus(ctx context.Context, path, subprotocol string, want int) error {
	return stepscommon.AwaitResponse(ctx,
		func(ctx context.Context) (*httpx.Response, error) {
			if err := g.openWebSocket(ctx, path, subprotocol); err != nil {
				return nil, retry.Transient(err)
			}
			response, _ := httpx.Published(ctx)
			return response, nil
		},
		func(r *httpx.Response) bool { return r != nil && r.StatusCode == want },
		fmt.Sprintf("waiting for a WebSocket handshake to %q to return %d", path, want))
}

func (g *Gateway) openWebSocket(ctx context.Context, path, subprotocol string) error {
	resolvedPath, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resolvedPath, "/") {
		resolvedPath = "/" + resolvedPath
	}

	base, err := g.topo.URL("platform-gateway", "http")
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse gateway endpoint %q: %w", base, err)
	}
	if endpoint.Host == "" {
		return fmt.Errorf("gateway endpoint %q has no host", base)
	}

	started := time.Now()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		return fmt.Errorf("connect to gateway endpoint %q: %w", endpoint.Host, err)
	}
	_ = conn.SetDeadline(time.Now().Add(wsFrameTimeout))

	host := g.base.RequestHost(ctx)
	if host == "" {
		host = endpoint.Hostname()
	}

	key, err := newWebSocketKey()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("generate Sec-WebSocket-Key: %w", err)
	}

	var req strings.Builder
	req.WriteString("GET " + resolvedPath + " HTTP/1.1\r\n")
	req.WriteString("Host: " + host + "\r\n")
	req.WriteString("Upgrade: websocket\r\n")
	req.WriteString("Connection: Upgrade\r\n")
	req.WriteString("Sec-WebSocket-Key: " + key + "\r\n")
	req.WriteString("Sec-WebSocket-Version: 13\r\n")
	for name, value := range g.scenarioHeaders(ctx) {
		req.WriteString(name + ": " + value + "\r\n")
	}
	if subprotocol != "" {
		req.WriteString("Sec-WebSocket-Protocol: " + subprotocol + "\r\n")
	}
	req.WriteString("\r\n")

	if _, err := conn.Write([]byte(req.String())); err != nil {
		_ = conn.Close()
		return fmt.Errorf("send handshake request: %w", err)
	}

	bufReader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(bufReader, nil)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("read handshake response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	response := &httpx.Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Method:     "GET",
		URL:        base + resolvedPath,
		Elapsed:    time.Since(started),
	}
	if err := g.funnel.Publish(ctx, response); err != nil {
		_ = conn.Close()
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		// Not an upgrade: nothing further to keep open, and the generic status-code
		// assertion is what the scenario uses to check this outcome.
		_ = conn.Close()
		return nil
	}

	_ = conn.SetDeadline(time.Time{})
	rw := &wsConnection{conn: conn, rw: bufio.NewReadWriter(bufReader, bufio.NewWriter(conn))}
	return tcontext.Set(ctx, keyWSConnection, rw)
}

func newWebSocketKey() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func openConnection(ctx context.Context) (*wsConnection, error) {
	v, ok := tcontext.Get(ctx, keyWSConnection)
	if !ok {
		return nil, fmt.Errorf("no open WebSocket connection for this scenario — open one first")
	}
	c, ok := v.(*wsConnection)
	if !ok || c == nil {
		return nil, fmt.Errorf("no open WebSocket connection for this scenario — open one first")
	}
	return c, nil
}

func (g *Gateway) sendWebSocketTextFrame(ctx context.Context, text string) error {
	c, err := openConnection(ctx)
	if err != nil {
		return err
	}
	return writeMaskedFrame(c.rw, wsOpText, []byte(text))
}

func (g *Gateway) sendWebSocketPing(ctx context.Context) error {
	c, err := openConnection(ctx)
	if err != nil {
		return err
	}
	return writeMaskedFrame(c.rw, wsOpPing, nil)
}

func (g *Gateway) receiveWebSocketTextFrame(ctx context.Context, want string) error {
	c, err := openConnection(ctx)
	if err != nil {
		return err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(wsFrameTimeout))
	defer func() { _ = c.conn.SetReadDeadline(time.Time{}) }()

	opcode, payload, err := readFrame(c.rw)
	if err != nil {
		return fmt.Errorf("read WebSocket frame: %w", err)
	}
	if opcode != wsOpText {
		return fmt.Errorf("expected a text frame (opcode %#x), got opcode %#x", wsOpText, opcode)
	}
	if got := string(payload); got != want {
		return fmt.Errorf("expected text frame %q, got %q", want, got)
	}
	return nil
}

func (g *Gateway) receiveWebSocketPong(ctx context.Context) error {
	c, err := openConnection(ctx)
	if err != nil {
		return err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(wsFrameTimeout))
	defer func() { _ = c.conn.SetReadDeadline(time.Time{}) }()

	opcode, _, err := readFrame(c.rw)
	if err != nil {
		return fmt.Errorf("read WebSocket frame: %w", err)
	}
	if opcode != wsOpPong {
		return fmt.Errorf("expected a pong frame (opcode %#x), got opcode %#x", wsOpPong, opcode)
	}
	return nil
}

func (g *Gateway) closeWebSocketConnection(ctx context.Context) error {
	c, err := openConnection(ctx)
	if err != nil {
		return err
	}
	_ = writeMaskedFrame(c.rw, wsOpClose, nil)
	tcontext.Remove(ctx, keyWSConnection)
	return c.conn.Close()
}

// writeMaskedFrame writes one unmasked-payload frame masked per RFC 6455 §5.3 — every frame a
// client sends must be masked, unlike the server's own unmasked replies.
func writeMaskedFrame(rw *bufio.ReadWriter, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length <= 0xFFFF:
		header = append(header, 0x80|126, 0, 0)
		binary.BigEndian.PutUint16(header[2:], uint16(length))
	default:
		header = append(header, 0x80|127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[2:], uint64(length))
	}

	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return fmt.Errorf("generate frame mask: %w", err)
	}
	header = append(header, mask[:]...)

	masked := make([]byte, length)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}

	if _, err := rw.Write(header); err != nil {
		return err
	}
	if _, err := rw.Write(masked); err != nil {
		return err
	}
	return rw.Flush()
}

// readFrame reads one unmasked server-to-client frame (RFC 6455 §5.1).
func readFrame(rw *bufio.ReadWriter) (opcode byte, payload []byte, err error) {
	header := make([]byte, 2)
	if _, err = readFull(rw, header); err != nil {
		return 0, nil, err
	}
	opcode = header[0] & 0x0F
	length := uint64(header[1] & 0x7F)

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err = readFull(rw, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err = readFull(rw, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	}

	payload = make([]byte, length)
	if _, err = readFull(rw, payload); err != nil {
		return 0, nil, err
	}
	return opcode, payload, nil
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
