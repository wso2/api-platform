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

// mock-websocket-backend is a WebSocket echo server for the gateway integration
// tests: it accepts an RFC 6455 handshake on any path and echoes every text and
// binary frame back unchanged, answering pings and closing cleanly on a close
// frame.
//
// RFC 6455 is implemented here directly rather than with a library because every
// other mock server in this directory is stdlib-only, and the subset an echo
// server needs is small. Keeping it that way means the integration-test backends
// never drag a new third-party dependency through the dependency-vetting gate.
//
// Behaviour knobs, all via environment variables, exist so tests can drive the
// gateway's own connection handling rather than only the happy path:
//
//	PORT                   listen port (default 8080)
//	REJECT_UPGRADE_STATUS  answer every handshake with this status instead of 101
//	SUBPROTOCOL            echo this subprotocol back in the handshake response
//	SERVER_SEND_INTERVAL   push a frame to the client this often (e.g. "2s")
package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

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

func main() {
	port := envOr("PORT", "8080")

	http.HandleFunc("/", handleWebSocket)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":" + port
	log.Printf("mock-websocket-backend listening on %s", addr)
	server := &http.Server{
		Addr: addr,
		// No ReadTimeout/WriteTimeout: a hijacked WebSocket connection is
		// long-lived by design, and those deadlines would cut it off. The
		// handshake itself is bounded by ReadHeaderTimeout instead.
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// A test can ask the backend to refuse the upgrade, so the gateway's
	// pass-through of a non-101 handshake response can be exercised.
	if status := os.Getenv("REJECT_UPGRADE_STATUS"); status != "" {
		code, err := strconv.Atoi(status)
		if err != nil || code < 100 || code > 599 {
			code = http.StatusForbidden
		}
		log.Printf("refusing upgrade for %s with %d (REJECT_UPGRADE_STATUS)", r.URL.Path, code)
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

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "connection cannot be hijacked", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		log.Printf("hijack failed: %v", err)
		return
	}
	defer conn.Close()

	// Clear the deadline the HTTP server set for reading headers: from here the
	// connection is a long-lived WebSocket, and the test decides its lifetime.
	_ = conn.SetDeadline(time.Time{})

	if err := writeHandshake(rw, key, r.Header.Get("Sec-WebSocket-Protocol")); err != nil {
		log.Printf("handshake write failed: %v", err)
		return
	}
	log.Printf("connection opened: path=%s", r.URL.Path)

	if interval := os.Getenv("SERVER_SEND_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil && d > 0 {
			go pushPeriodically(rw, d)
		}
	}

	echoLoop(rw, r.URL.Path)
}

// writeHandshake sends the 101 response, including the accept key derived from
// the client's key per RFC 6455 §4.2.2.
func writeHandshake(rw *bufio.ReadWriter, key, offeredSubprotocols string) error {
	sum := sha1.Sum([]byte(key + websocketGUID)) // #nosec G401 -- fixed by RFC 6455, not a security hash
	accept := base64.StdEncoding.EncodeToString(sum[:])

	var b strings.Builder
	b.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	b.WriteString("Upgrade: websocket\r\n")
	b.WriteString("Connection: Upgrade\r\n")
	b.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n")

	// Echo a subprotocol only when the test asked for a specific one and the
	// client actually offered it. Selecting one the client did not offer makes a
	// browser fail the connection.
	if want := os.Getenv("SUBPROTOCOL"); want != "" && offers(offeredSubprotocols, want) {
		b.WriteString("Sec-WebSocket-Protocol: " + want + "\r\n")
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

// echoLoop reads frames until the peer closes or the connection breaks, echoing
// data frames and answering control frames.
func echoLoop(rw *bufio.ReadWriter, path string) {
	for {
		opcode, payload, err := readFrame(rw)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("connection closed: path=%s err=%v", path, err)
			}
			return
		}

		switch opcode {
		case opText, opBinary, opContinuation:
			if err := writeFrame(rw, opcode, payload); err != nil {
				log.Printf("echo write failed: %v", err)
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

// pushPeriodically sends a server-originated frame on an interval, so a test can
// prove a connection stays alive on server traffic alone.
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
