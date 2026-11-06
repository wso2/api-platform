/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

type auditReceivedBatch struct {
	at     time.Time
	header http.Header
	lines  []string
}

// receiver records every batch POSTed to it; respond writes the response to
// the n-th request (1-based) and may block.
type auditReceiver struct {
	*httptest.Server
	mu      sync.Mutex
	batches []auditReceivedBatch
}

func newAuditReceiver(t *testing.T, useTLS bool, respond func(n int, w http.ResponseWriter)) *auditReceiver {
	t.Helper()
	r := &auditReceiver{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.batches = append(r.batches, auditReceivedBatch{time.Now(), req.Header.Clone(), strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")})
		n := len(r.batches)
		r.mu.Unlock()
		if respond != nil {
			respond(n, w)
		}
	})
	if useTLS {
		r.Server = httptest.NewTLSServer(handler)
	} else {
		r.Server = httptest.NewServer(handler)
	}
	t.Cleanup(r.Close)
	return r
}

func (r *auditReceiver) received() []auditReceivedBatch {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.batches)
}

func auditRespondWith(statuses ...int) func(int, http.ResponseWriter) {
	return func(n int, w http.ResponseWriter) {
		status := statuses[min(n, len(statuses))-1]
		if status == http.StatusFound {
			w.Header().Set("Location", "/elsewhere")
		}
		w.WriteHeader(status)
	}
}

func testAuditHTTPConfig(endpoint string) config.AuditHTTP {
	return config.AuditHTTP{
		Endpoint:       endpoint,
		ContentType:    "application/x-ndjson",
		BatchMaxEvents: 100,
		BatchMaxBytes:  1 << 20,
		FlushInterval:  10 * time.Millisecond,
		QueueCapacity:  100,
		RequestTimeout: 2 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   time.Millisecond,
	}
}

func newTestAuditHTTPSink(t *testing.T, cfg config.AuditHTTP) (*auditHTTPSink, *auditLogBuffer) {
	t.Helper()
	logger, logs := newAuditTestLogger()
	s, err := newAuditHTTPSink(cfg, logger)
	if err != nil {
		t.Fatalf("newAuditHTTPSink error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s, logs
}

func auditBatchLines(batches []auditReceivedBatch) [][]string {
	out := make([][]string, len(batches))
	for i, b := range batches {
		out[i] = b.lines
	}
	return out
}

func TestAuditHTTPSinkSendsBatchesWithItsHeaders(t *testing.T) {
	recv := newAuditReceiver(t, false, nil)
	cfg := testAuditHTTPConfig(recv.URL)
	cfg.BatchMaxEvents = 2
	cfg.FlushInterval = time.Hour
	cfg.Auth.Type = config.AuditAuthBearer
	cfg.Auth.Bearer.Token = "s3cret-t0k"
	s, logs := newTestAuditHTTPSink(t, cfg)

	for _, line := range []string{`{"id":"1"}`, `{"id":"2"}`, `{"id":"3"}`} {
		s.Write([]byte(line))
	}
	waitForAudit(t, "the first batch", func() bool { return len(recv.received()) == 1 })
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	got := recv.received()
	if want := [][]string{{`{"id":"1"}`, `{"id":"2"}`}, {`{"id":"3"}`}}; !slices.EqualFunc(auditBatchLines(got), want, slices.Equal) {
		t.Fatalf("batches = %q, want %q (a full batch, then the rest on Close)", auditBatchLines(got), want)
	}
	for _, b := range got {
		if ct := b.header.Get("Content-Type"); ct != "application/x-ndjson" {
			t.Errorf("Content-Type = %q", ct)
		}
		if auth := b.header.Get("Authorization"); auth != "Bearer s3cret-t0k" {
			t.Errorf("Authorization = %q", auth)
		}
	}
	if strings.Contains(logs.String(), "s3cret-t0k") {
		t.Fatalf("the token was logged:\n%s", logs)
	}
	if s.Name() != "http" {
		t.Fatalf("Name() = %q, want http", s.Name())
	}
}

func TestAuditHTTPSinkFlushesOnTheInterval(t *testing.T) {
	recv := newAuditReceiver(t, false, nil)
	s, _ := newTestAuditHTTPSink(t, testAuditHTTPConfig(recv.URL))

	s.Write([]byte(`{"id":"1"}`))

	waitForAudit(t, "the interval flush", func() bool { return len(recv.received()) == 1 })
}

func TestAuditHTTPSinkSplitsBatchesAtBatchMaxBytes(t *testing.T) {
	recv := newAuditReceiver(t, false, nil)
	cfg := testAuditHTTPConfig(recv.URL)
	cfg.FlushInterval = time.Hour
	cfg.BatchMaxBytes = 20
	s, _ := newTestAuditHTTPSink(t, cfg)

	big := strings.Repeat("d", 30)
	for _, line := range []string{"aaaaaaaa", "bbbbbbbb", "cccccccc", big, "eeeeeeee"} {
		s.Write([]byte(line))
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	// A line that would take the body past the limit starts a new batch, and a
	// line larger than the limit is sent alone.
	want := [][]string{{"aaaaaaaa", "bbbbbbbb"}, {"cccccccc"}, {big}, {"eeeeeeee"}}
	if got := auditBatchLines(recv.received()); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("batches = %q, want %q", got, want)
	}
}

func TestAuditHTTPSinkRetries(t *testing.T) {
	for _, tc := range []struct {
		name         string
		statuses     []int
		maxRetries   int
		wantAttempts int
		wantDropped  string // the logged reason when the batch is dropped
	}{
		{"5xx, then accepted", []int{503, 200}, 3, 2, ""},
		{"429, then accepted", []int{429, 200}, 3, 2, ""},
		{"4xx is not retried", []int{400}, 3, 1, "rejected the batch with status 400"},
		{"a redirect is not followed", []int{302}, 3, 1, "rejected the batch with status 302"},
		{"retries exhausted", []int{500}, 2, 3, "receiver returned status 500"},
		{"retries off", []int{500}, 0, 1, "receiver returned status 500"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recv := newAuditReceiver(t, false, auditRespondWith(tc.statuses...))
			cfg := testAuditHTTPConfig(recv.URL)
			cfg.MaxRetries = tc.maxRetries
			s, logs := newTestAuditHTTPSink(t, cfg)

			s.Write([]byte(`{"id":"1"}`))
			if tc.wantDropped != "" {
				waitForAudit(t, "the dropped batch", func() bool { return strings.Contains(logs.String(), "Failed to deliver audit records") })
				if !strings.Contains(logs.String(), tc.wantDropped) {
					t.Fatalf("drop reason not logged, want %q:\n%s", tc.wantDropped, logs)
				}
			} else {
				waitForAudit(t, "the accepted attempt", func() bool { return len(recv.received()) == tc.wantAttempts })
			}
			if err := s.Close(context.Background()); err != nil {
				t.Fatalf("Close error = %v", err)
			}

			if got := len(recv.received()); got != tc.wantAttempts {
				t.Fatalf("attempts = %d, want %d", got, tc.wantAttempts)
			}
			if tc.wantDropped == "" && strings.Contains(logs.String(), "Failed to deliver") {
				t.Fatalf("a delivered batch was logged as dropped:\n%s", logs)
			}
			if strings.Contains(logs.String(), "/elsewhere") || len(recv.received()) > tc.wantAttempts {
				t.Fatal("the redirect was followed")
			}
		})
	}
}

func TestAuditHTTPSinkWaitsForRetryAfter(t *testing.T) {
	recv := newAuditReceiver(t, false, func(n int, w http.ResponseWriter) {
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		}
	})
	s, _ := newTestAuditHTTPSink(t, testAuditHTTPConfig(recv.URL))

	s.Write([]byte(`{"id":"1"}`))
	waitForAudit(t, "the retry", func() bool { return len(recv.received()) == 2 })

	got := recv.received()
	if gap := got[1].at.Sub(got[0].at); gap < 900*time.Millisecond {
		t.Fatalf("retried after %s, want about the 1s Retry-After", gap)
	}
}

func TestAuditHTTPSinkRetriesAnUnreachableReceiver(t *testing.T) {
	recv := newAuditReceiver(t, false, nil)
	endpoint := recv.URL
	recv.Close()
	cfg := testAuditHTTPConfig(endpoint)
	cfg.MaxRetries = 1
	s, logs := newTestAuditHTTPSink(t, cfg)

	s.Write([]byte(`{"id":"1"}`))

	waitForAudit(t, "the dropped batch", func() bool { return strings.Contains(logs.String(), "Failed to deliver audit records") })
	if !strings.Contains(logs.String(), "posting batch") {
		t.Fatalf("the transport error was not logged:\n%s", logs)
	}
}

func TestAuditHTTPSinkStopsRetryingOnClose(t *testing.T) {
	recv := newAuditReceiver(t, false, auditRespondWith(500))
	cfg := testAuditHTTPConfig(recv.URL)
	cfg.RetryBackoff = time.Hour
	s, logs := newTestAuditHTTPSink(t, cfg)

	s.Write([]byte(`{"id":"1"}`))
	waitForAudit(t, "the first attempt", func() bool { return len(recv.received()) == 1 })
	start := time.Now()
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Close waited %s for the backoff", elapsed)
	}
	if !strings.Contains(logs.String(), "Failed to deliver audit records") {
		t.Fatalf("the abandoned batch was not logged:\n%s", logs)
	}
}

func TestAuditHTTPSinkDropsWhenTheQueueIsFull(t *testing.T) {
	release := make(chan struct{})
	recv := newAuditReceiver(t, false, func(n int, w http.ResponseWriter) {
		if n == 1 {
			<-release
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	cfg := testAuditHTTPConfig(recv.URL)
	cfg.QueueCapacity = 1
	cfg.BatchMaxEvents = 1
	s, logs := newTestAuditHTTPSink(t, cfg)

	s.Write([]byte(`{"id":"1"}`))
	waitForAudit(t, "the first batch to be in flight", func() bool { return len(recv.received()) == 1 })
	s.Write([]byte(`{"id":"2"}`)) // fills the queue
	s.Write([]byte(`{"id":"3"}`)) // dropped
	close(release)
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	if !strings.Contains(logs.String(), "Audit HTTP output queue is full; dropping the record") {
		t.Fatalf("the dropped record was not logged:\n%s", logs)
	}
	if want := [][]string{{`{"id":"1"}`}, {`{"id":"2"}`}}; !slices.EqualFunc(auditBatchLines(recv.received()), want, slices.Equal) {
		t.Fatalf("batches = %q, want %q", auditBatchLines(recv.received()), want)
	}
}

func TestAuditHTTPSinkDropsWritesAfterClose(t *testing.T) {
	recv := newAuditReceiver(t, false, nil)
	s, logs := newTestAuditHTTPSink(t, testAuditHTTPConfig(recv.URL))
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	s.Write([]byte(`{"id":"1"}`))

	if !strings.Contains(logs.String(), "Audit HTTP output is closed; dropping the record") {
		t.Fatalf("the dropped record was not logged:\n%s", logs)
	}
	if n := len(recv.received()); n != 0 {
		t.Fatalf("%d batches sent after Close", n)
	}
}

func TestAuditHTTPSinkCloseGivesUpWhenTheContextEnds(t *testing.T) {
	release := make(chan struct{})
	recv := newAuditReceiver(t, false, func(int, http.ResponseWriter) { <-release })
	s, _ := newTestAuditHTTPSink(t, testAuditHTTPConfig(recv.URL))
	s.Write([]byte(`{"id":"1"}`))
	waitForAudit(t, "the batch to be in flight", func() bool { return len(recv.received()) == 1 })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Close(ctx); err == nil || !strings.Contains(err.Error(), "did not finish flushing") {
		t.Fatalf("Close with a stuck delivery = %v, want a timeout error", err)
	}
	close(release)
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("second Close error = %v", err)
	}
}

func TestAuditHTTPSinkOverTLS(t *testing.T) {
	recv := newAuditReceiver(t, true, nil)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: recv.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("receiver verified with ca_file", func(t *testing.T) {
		cfg := testAuditHTTPConfig(recv.URL)
		cfg.TLS.CAFile = caFile
		s, logs := newTestAuditHTTPSink(t, cfg)

		s.Write([]byte(`{"id":"1"}`))

		waitForAudit(t, "the connected log", func() bool { return strings.Contains(logs.String(), "Audit HTTP output connected") })
		for _, want := range []string{`tlsVersion="TLS 1.3"`, "keyExchange=X25519MLKEM768"} {
			if !strings.Contains(logs.String(), want) {
				t.Errorf("connected log lacks %s:\n%s", want, logs)
			}
		}
	})

	t.Run("unknown receiver certificate", func(t *testing.T) {
		cfg := testAuditHTTPConfig(recv.URL)
		cfg.MaxRetries = 0
		s, logs := newTestAuditHTTPSink(t, cfg)

		s.Write([]byte(`{"id":"1"}`))

		waitForAudit(t, "the dropped batch", func() bool { return strings.Contains(logs.String(), "Failed to deliver audit records") })
		if !strings.Contains(logs.String(), "certificate") {
			t.Fatalf("the verification error was not logged:\n%s", logs)
		}
	})

	t.Run("insecure_skip_verify", func(t *testing.T) {
		before := len(recv.received())
		cfg := testAuditHTTPConfig(recv.URL)
		cfg.TLS.InsecureSkipVerify = true
		s, _ := newTestAuditHTTPSink(t, cfg)

		s.Write([]byte(`{"id":"1"}`))

		waitForAudit(t, "the delivered batch", func() bool { return len(recv.received()) == before+1 })
	})
}

func TestNewAuditHTTPSinkRejectsBadSettings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*config.AuditHTTP)
		wantErr string
	}{
		{"queue_capacity 0", func(c *config.AuditHTTP) { c.QueueCapacity = 0 }, "must be positive"},
		{"max_retries negative", func(c *config.AuditHTTP) { c.MaxRetries = -1 }, "must be positive"},
		{"endpoint without a host", func(c *config.AuditHTTP) { c.Endpoint = "https:///ingest" }, "not a valid absolute URL"},
		{"endpoint not a URL", func(c *config.AuditHTTP) { c.Endpoint = "https://logs.example.com/%zz" }, "not a valid absolute URL"},
		{"credential missing", func(c *config.AuditHTTP) { c.Auth.Type = config.AuditAuthBasic }, "auth: basic.username and basic.password"},
		{"ca_file missing", func(c *config.AuditHTTP) { c.TLS.CAFile = filepath.Join(t.TempDir(), "missing.pem") }, "tls:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testAuditHTTPConfig("https://logs.example.com/ingest")
			tc.mutate(&cfg)
			logger, _ := newAuditTestLogger()
			s, err := newAuditHTTPSink(cfg, logger)
			if err == nil {
				_ = s.Close(context.Background())
				t.Fatal("newAuditHTTPSink succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("newAuditHTTPSink error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseAuditRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		value    string
		min, max time.Duration
	}{
		{"", 0, 0},
		{"soon", 0, 0},
		{"0", 0, 0},
		{"-5", 0, 0},
		{" 3 ", 3 * time.Second, 3 * time.Second},
		{"99999999999999999", auditRetryAfterCap, auditRetryAfterCap},
		{time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat), 8 * time.Second, 10 * time.Second},
		{time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat), 0, 0},
		{time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), auditRetryAfterCap, auditRetryAfterCap},
	} {
		if got := parseAuditRetryAfter(tc.value); got < tc.min || got > tc.max {
			t.Errorf("parseAuditRetryAfter(%q) = %s, want between %s and %s", tc.value, got, tc.min, tc.max)
		}
	}
}

func TestAuditHTTPSinkBackoff(t *testing.T) {
	s := &auditHTTPSink{cfg: config.AuditHTTP{RetryBackoff: 100 * time.Millisecond}}
	for _, tc := range []struct {
		attempt  int
		min, max time.Duration
	}{
		{1, 50 * time.Millisecond, 100 * time.Millisecond},
		{3, 200 * time.Millisecond, 400 * time.Millisecond},
		// The exponent is capped, so a large attempt cannot overflow.
		{50, 51200 * time.Millisecond, 102400 * time.Millisecond},
	} {
		for range 20 {
			if got := s.backoff(tc.attempt); got < tc.min || got >= tc.max {
				t.Fatalf("backoff(%d) = %s, want in [%s, %s)", tc.attempt, got, tc.min, tc.max)
			}
		}
	}

	s.cfg.RetryBackoff = time.Nanosecond
	if got := s.backoff(1); got != time.Nanosecond {
		t.Fatalf("backoff(1) with a 1ns base = %s, want 1ns without jitter", got)
	}
}

func TestEncodeAuditNDJSON(t *testing.T) {
	got := encodeAuditNDJSON([][]byte{[]byte(`{"id":"1"}`), []byte(`{"id":"2"}`)})
	if want := "{\"id\":\"1\"}\n{\"id\":\"2\"}\n"; string(got) != want {
		t.Fatalf("encodeAuditNDJSON = %q, want %q", got, want)
	}
}
