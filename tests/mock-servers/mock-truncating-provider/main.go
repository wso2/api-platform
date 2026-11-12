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

// Mock provider that commits a 200 and then dies mid-body.
//
// This is the one failure shape the gateway cannot rescue and, before FR-030n/o/p,
// could not even grade: the response headers have already reached the client, so
// Envoy will not retry (envoyproxy/envoy#5876, closed by design), and the policy's
// response phase sees a perfectly ordinary 200. A provider truncating every
// response therefore looked like a provider succeeding at every response.
//
// Routes (the gateway appends the operation path, so each is matched by prefix):
//
//	/truncate  200, headers, one chunk, then RST  -- the subject
//	/whole     200, a complete SSE body           -- the control, same shape, no fault
//	/mixed     alternates truncation and 503      -- two failure causes on one target
//	/reset     clears /mixed's counter            -- keeps a scenario re-runnable
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// mixedCalls counts calls to /mixed so it can alternate between a truncation and
// an outright failure. A scenario calls /reset first, so a re-run starts clean.
var mixedCalls atomic.Int64

func main() {
	http.HandleFunc("/", route)
	log.Println("mock-truncating-provider listening on :9080")
	srv := &http.Server{
		Addr:              ":9080",
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Fatal(srv.ListenAndServe())
}

func route(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/reset"):
		mixedCalls.Store(0)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"reset":true}`)
	case strings.HasPrefix(r.URL.Path, "/truncate"):
		truncate(w, r)
	case strings.HasPrefix(r.URL.Path, "/whole"):
		whole(w, r)
	case strings.HasPrefix(r.URL.Path, "/mixed"):
		// Call 1 truncates, call 2 fails outright. The point is that these reach
		// the policy by different routes -- the truncation through the access-log
		// verdict, the 503 through the ordinary response phase -- and must land on
		// the same target's counter rather than cancelling each other out.
		if mixedCalls.Add(1) == 1 {
			truncate(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error":"mock-truncating-provider: upstream %s refused"}`, r.URL.Path)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"no mock route for %s"}`, r.URL.Path)
	}
}

// whole is the control: an SSE response of exactly the shape truncate aborts,
// delivered complete. Without it, "the target was suspended" cannot be told apart
// from "any streaming response suspends its target".
func whole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "data: {\"served_by\":\"%s\"}\n\ndata: [DONE]\n\n", r.URL.Path)
}

func truncate(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "mock requires a hijackable connection", http.StatusInternalServerError)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		log.Printf("hijack failed: %v", err)
		return
	}
	defer conn.Close()

	body := fmt.Sprintf("data: {\"served_by\":\"%s\",\"truncated\":true}", r.URL.Path)
	fmt.Fprint(buf, "HTTP/1.1 200 OK\r\n")
	fmt.Fprint(buf, "Content-Type: text/event-stream\r\n")
	fmt.Fprint(buf, "Transfer-Encoding: chunked\r\n")
	fmt.Fprint(buf, "\r\n")
	fmt.Fprintf(buf, "%x\r\n%s\r\n", len(body), body)
	if err := buf.Flush(); err != nil {
		log.Printf("flush failed: %v", err)
		return
	}

	// The delay is load-bearing. Without it Envoy sees the reset before it has
	// committed the response downstream and turns the whole exchange into a clean
	// 502 -- which the response phase already grades correctly, so the test would
	// pass while asserting nothing (T030k0). The bug exists only once the 200 has
	// reached the client, so the mock has to stream first.
	time.Sleep(1500 * time.Millisecond)

	// No terminating "0\r\n\r\n". SetLinger(0) makes the close an RST rather than a
	// graceful FIN, which is what a provider process dying actually looks like.
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
}
