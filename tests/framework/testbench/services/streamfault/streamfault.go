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

// Package streamfault serves responses that fail after their headers, the one failure shape
// a proxy cannot rescue. Once a 200 has reached the client it cannot be un-sent, so an
// upstream that commits and then dies mid-body looks to an ordinary response filter like an
// upstream that succeeded. A gateway grading its upstreams has to see the difference.
//
// The mode is chosen by a request header rather than by path or by stored state, so the
// service holds nothing between requests and a scenario drives the sequence it wants by
// sending different headers. That matters because a route reaches one upstream URL: varying
// the response per request is otherwise only possible with a counter, which would make
// concurrent scenarios on one testbench interfere.
package streamfault

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3027

// ModeHeader names the response shape to serve. An absent or unknown value serves ModeWhole,
// so a request that reaches this service by accident gets an ordinary complete response.
const ModeHeader = "x-stream-fault"

// DurationHeader bounds ModeDrip, in whole seconds. Absent or unparseable means 5.
const DurationHeader = "x-stream-fault-seconds"

// The response shapes, named by ModeHeader.
const (
	// ModeWhole is a complete SSE body. The control for ModeTruncate: a streamed response is
	// the shape most likely to be flagged as broken by accident, so "the target was
	// suspended" means nothing without a complete stream of the same shape that does not
	// suspend it.
	ModeWhole = "whole"
	// ModeTruncate commits a 200, sends one chunk, then resets the connection.
	ModeTruncate = "truncate"
	// ModeFail refuses before any body, which is the ordinary retriable failure.
	ModeFail = "fail"
	// ModeDrip sends headers at once and then streams slowly, so a route timeout cuts the
	// body rather than the handshake.
	ModeDrip = "drip"
	// ModePlain is an ordinary non-streaming 200. It exists because WHEN a success is graded
	// depends on its shape: a proxy cannot know a streamed response succeeded until the
	// stream ends, so it defers to the access log and the verdict arrives asynchronously. A
	// scenario that needs a success to have been counted before its next request sends one
	// of these, which the response phase can grade on the spot.
	ModePlain = "plain"
)

// truncateHold is how long ModeTruncate streams before resetting. The delay is load-bearing:
// without it the reset can reach the proxy before it has committed the response downstream,
// which turns the whole exchange into a clean error the proxy's response phase already
// grades correctly - and the scenario would pass while asserting nothing.
const truncateHold = 1500 * time.Millisecond

// dripInterval is how often ModeDrip writes, and maxDripSeconds bounds how long it runs so a
// scenario cannot hold a connection past the suite's own timeouts.
const (
	dripInterval   = 200 * time.Millisecond
	maxDripSeconds = 20
)

// Service implements the stream-fault testbench service.
type Service struct{}

// New returns a new stream-fault service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "stream-fault" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service keeps no request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler serves every mode on every path, so a route can point at any path it likes and
// name the shape it wants in a header.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	return mux
}

func (s *Service) serve(w http.ResponseWriter, r *http.Request) {
	switch r.Header.Get(ModeHeader) {
	case ModeTruncate:
		truncate(w, r)
	case ModeFail:
		fail(w, r)
	case ModeDrip:
		drip(w, r)
	case ModePlain:
		plain(w, r)
	default:
		whole(w, r)
	}
}

// whole writes a complete SSE body of exactly the shape truncate aborts.
func whole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "data: {\"served_by\":%q,\"complete\":true}\n\ndata: [DONE]\n\n", r.URL.Path)
}

// plain writes an ordinary JSON 200: fixed length, no chunking, no event-stream content type,
// so nothing about it is provisional.
func plain(w http.ResponseWriter, r *http.Request) {
	body := fmt.Sprintf("{\"served_by\":%q,\"complete\":true}", r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, body)
}

// fail refuses before any body, so the proxy may still retry the attempt elsewhere.
func fail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, "{\"error\":\"stream-fault: upstream %q refused\"}", r.URL.Path)
}

// drip sends headers at once and then streams until its duration runs out, so a route
// timeout lands mid-body rather than on the handshake.
func drip(w http.ResponseWriter, r *http.Request) {
	seconds := 5
	if raw := r.Header.Get(DurationHeader); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			seconds = parsed
		}
	}
	if seconds > maxDripSeconds {
		seconds = maxDripSeconds
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream-fault: drip requires a flushable response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "data: {\"served_by\":%q,\"dripping\":true}\n\n", r.URL.Path)
	flusher.Flush()

	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(dripInterval):
		}
		if _, err := fmt.Fprint(w, "data: {\"chunk\":true}\n\n"); err != nil {
			return
		}
		flusher.Flush()
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// truncate commits a 200 and one chunk, holds the stream open, then resets the connection.
// The response is written by hand because net/http always terminates a chunked body
// correctly, and an abrupt end is the whole point.
func truncate(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "stream-fault: truncate requires a hijackable connection", http.StatusInternalServerError)
		return
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		testbench.ServiceLogger(r.Context()).Error("stream-fault could not hijack", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	body := fmt.Sprintf("data: {\"served_by\":%q,\"truncated\":true}", r.URL.Path)
	fmt.Fprint(buf, "HTTP/1.1 200 OK\r\n")
	fmt.Fprint(buf, "Content-Type: text/event-stream\r\n")
	fmt.Fprint(buf, "Transfer-Encoding: chunked\r\n")
	fmt.Fprint(buf, "\r\n")
	fmt.Fprintf(buf, "%x\r\n%s\r\n", len(body), body)
	if flushErr := buf.Flush(); flushErr != nil {
		testbench.ServiceLogger(r.Context()).Error("stream-fault could not flush", "error", flushErr)
		return
	}

	select {
	case <-r.Context().Done():
	case <-time.After(truncateHold):
	}

	// No terminating zero-length chunk. SetLinger(0) makes the close a reset rather than a
	// graceful shutdown, which is what a provider process dying actually looks like.
	if tcp, isTCP := conn.(*net.TCPConn); isTCP {
		_ = tcp.SetLinger(0)
	}
}
