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

// Package websocket is the RFC 6455 echo upstream used by WebSocketApi
// integration tests: it accepts a handshake on any path and echoes every text
// and binary frame back unchanged, answers pings, and closes cleanly on a
// close frame.
//
// RFC 6455 is implemented here directly rather than with a library, matching
// every other testbench service's stdlib-only backends — the subset an echo
// server needs is small, and it keeps the integration-test backends from
// dragging a new third-party dependency through the dependency-vetting gate.
//
// Unlike the capture service, this one is stateless and unpartitioned: its
// behaviour is a pure function of the handshake request, following the same
// convention backend.go's `?statusCode=` already establishes. A scenario asks
// for non-default behaviour via query parameters on the handshake URL itself:
//
//	rejectStatus  answer the handshake with this status instead of 101
//	subprotocol   echo this subprotocol back, only if the client offered it
//	pushInterval  push a server-originated frame this often (Go duration, e.g. "2s")
package websocket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3019

// websocketGUID is the fixed value RFC 6455 §4.2.2 mixes into the accept-key
// hash. It is part of the protocol, not a secret.
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Opcodes from RFC 6455 §5.2.
const (
	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA
)

// maxFrameSize bounds how much a single frame may carry, so a malformed or
// hostile length header cannot make this process allocate without limit.
const maxFrameSize = 1 << 20 // 1 MiB

// Service implements the stateless WebSocket echo testbench service.
type Service struct{}

// New returns a new websocket service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "websocket" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the handshake endpoint and a plain health check.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("/", s.handshake)
	return testbench.NormalizeMethod(mux)
}

func (s *Service) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

// handshake upgrades the connection and runs the echo loop, or answers a
// configured non-101 status without upgrading.
func (s *Service) handshake(w http.ResponseWriter, r *http.Request) {
	logger := testbench.ServiceLogger(r.Context())

	if raw := r.URL.Query().Get("rejectStatus"); raw != "" {
		code, err := strconv.Atoi(raw)
		if err != nil || code < 100 || code > 599 {
			code = http.StatusForbidden
		}
		logger.Info("websocket: refusing upgrade (rejectStatus)", "path", r.URL.Path, "status", code)
		w.WriteHeader(code)
		return
	}

	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "expected a websocket upgrade", http.StatusBadRequest)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	// http.NewResponseController, not a raw w.(http.Hijacker) assertion: the server wraps
	// every handler's ResponseWriter for access logging, and that wrapper only exposes
	// Hijack through the Unwrap() convention the controller knows to follow. A direct
	// assertion on the wrapper type fails even though the underlying connection supports it.
	conn, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		http.Error(w, "connection cannot be hijacked", http.StatusInternalServerError)
		return
	}
	defer func() { _ = conn.Close() }()

	// Clear the deadline the HTTP server set for reading headers: from here the
	// connection is a long-lived WebSocket, and the test decides its lifetime —
	// not the shared server's generic read/write timeouts, which stop applying
	// to a connection the moment it is hijacked.
	_ = conn.SetDeadline(time.Time{})

	subprotocol := ""
	if want := r.URL.Query().Get("subprotocol"); want != "" && offers(r.Header.Get("Sec-WebSocket-Protocol"), want) {
		subprotocol = want
	}
	if err := writeHandshake(rw, key, subprotocol); err != nil {
		logger.Error("websocket: handshake write failed", "error", err)
		return
	}
	logger.Info("websocket: connection opened", "path", r.URL.Path)

	if raw := r.URL.Query().Get("pushInterval"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			go pushPeriodically(rw, d)
		}
	}

	echoLoop(rw, r.URL.Path, logger)
}

// writeHandshake sends the 101 response, including the accept key derived from
// the client's key per RFC 6455 §4.2.2.
func writeHandshake(rw *bufio.ReadWriter, key, subprotocol string) error {
	sum := sha1.Sum([]byte(key + websocketGUID)) // #nosec G401 -- fixed by RFC 6455, not a security hash
	accept := base64.StdEncoding.EncodeToString(sum[:])

	var b strings.Builder
	b.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	b.WriteString("Upgrade: websocket\r\n")
	b.WriteString("Connection: Upgrade\r\n")
	b.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n")
	if subprotocol != "" {
		b.WriteString("Sec-WebSocket-Protocol: " + subprotocol + "\r\n")
	}
	b.WriteString("\r\n")

	if _, err := rw.WriteString(b.String()); err != nil {
		return err
	}
	return rw.Flush()
}

// offers reports whether the client's Sec-WebSocket-Protocol header listed want.
func offers(header, want string) bool {
	for _, candidate := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), want) {
			return true
		}
	}
	return false
}

// echoLoop reads frames until the peer closes or the connection breaks,
// echoing data frames and answering control frames.
func echoLoop(rw *bufio.ReadWriter, path string, logger *slog.Logger) {
	for {
		opcode, payload, err := readFrame(rw)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				logger.Debug("websocket: connection closed", "path", path, "error", err)
			}
			return
		}

		switch opcode {
		case opText, opBinary, opContinuation:
			if err := writeFrame(rw, opcode, payload); err != nil {
				logger.Debug("websocket: echo write failed", "error", err)
				return
			}
		case opPing:
			// A pong carries the ping's payload back, per RFC 6455 §5.5.3.
			if err := writeFrame(rw, opPong, payload); err != nil {
				return
			}
		case opPong:
			// Unsolicited pongs are legal and need no reply.
		case opClose:
			// Echo the close frame back so the peer sees an orderly shutdown
			// rather than an abnormal 1006.
			_ = writeFrame(rw, opClose, payload)
			return
		}
	}
}

// pushPeriodically sends a server-originated frame on an interval, so a test
// can prove a connection stays alive on server traffic alone.
func pushPeriodically(rw *bufio.ReadWriter, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		msg := fmt.Sprintf("server-push %d", time.Now().Unix())
		if err := writeFrame(rw, opText, []byte(msg)); err != nil {
			return
		}
	}
}

// readFrame reads one frame. Continuation handling is deliberately minimal: an
// echo server can return each fragment as it arrives, because echoing the
// fragments in order reproduces the original message.
func readFrame(rw *bufio.ReadWriter) (opcode byte, payload []byte, err error) {
	header := make([]byte, 2)
	if _, err = io.ReadFull(rw, header); err != nil {
		return 0, nil, err
	}

	opcode = header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err = io.ReadFull(rw, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err = io.ReadFull(rw, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	}

	if length > maxFrameSize {
		return 0, nil, fmt.Errorf("frame of %d bytes exceeds the %d byte limit", length, maxFrameSize)
	}

	var maskKey [4]byte
	if masked {
		if _, err = io.ReadFull(rw, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, length)
	if _, err = io.ReadFull(rw, payload); err != nil {
		return 0, nil, err
	}

	// Client-to-server frames are masked (RFC 6455 §5.3); unmask in place.
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}

// writeFrame writes one unmasked frame. Server-to-client frames are never
// masked, per RFC 6455 §5.1.
func writeFrame(rw *bufio.ReadWriter, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode} // FIN set: this echo never fragments its own output

	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 0xFFFF:
		header = append(header, 126, 0, 0)
		binary.BigEndian.PutUint16(header[2:], uint16(length))
	default:
		header = append(header, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[2:], uint64(length))
	}

	if _, err := rw.Write(header); err != nil {
		return err
	}
	if _, err := rw.Write(payload); err != nil {
		return err
	}
	return rw.Flush()
}
