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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

const (
	// auditMaxErrorBodyBytes caps how much of a receiver's response is read, so
	// a huge error body cannot grow the heap.
	auditMaxErrorBodyBytes int64 = 4 << 10
	// auditRetryAfterCap bounds how long a receiver's Retry-After can stall the
	// sender while the queue fills.
	auditRetryAfterCap = 30 * time.Second
)

// auditHTTPSink batches audit lines and POSTs them as newline-delimited JSON to
// an operator-chosen endpoint, for deployments without a log collector.
//
//   - Write never blocks: it runs on the request path. A line that does not fit
//     in the queue is dropped and logged.
//   - The queue is bounded, so a receiver outage cannot grow memory without limit.
//   - A batch that cannot be delivered is dropped. The queue is memory-only, so
//     unlike a collector that buffers to disk, an outage longer than the queue and
//     retries can absorb loses records; the audit table still has them.
type auditHTTPSink struct {
	cfg      config.AuditHTTP
	client   *http.Client
	host     string
	logger   *slog.Logger
	throttle auditErrThrottle

	// authHeaderName/Value are resolved once; an empty name means none. The
	// value is a secret and is never logged.
	authHeaderName  string
	authHeaderValue string

	queue chan []byte
	// done is closed by Close to stop the sender; stopped is closed by the
	// sender once its final flush is done.
	done      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
	// tlsLogged logs the negotiated TLS version and key exchange once, so an
	// operator can see whether the connection uses the post-quantum hybrid.
	tlsLogged sync.Once
}

// newAuditHTTPSink builds the sink and starts its sender. It returns an error
// rather than degrading; config validation has already checked the same values.
func newAuditHTTPSink(cfg config.AuditHTTP, logger *slog.Logger) (*auditHTTPSink, error) {
	// Non-positive values would panic in time.NewTicker or make.
	if cfg.FlushInterval <= 0 || cfg.RequestTimeout <= 0 || cfg.QueueCapacity <= 0 ||
		cfg.BatchMaxEvents <= 0 || cfg.BatchMaxBytes <= 0 || cfg.MaxRetries < 0 {
		return nil, fmt.Errorf("batch, queue and timeout settings must be positive")
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("endpoint is not a valid absolute URL")
	}
	name, value, err := cfg.Auth.AuthHeader()
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	client, err := cfg.NewClient()
	if err != nil {
		return nil, fmt.Errorf("tls: %w", err)
	}

	s := &auditHTTPSink{
		cfg:             cfg,
		client:          client,
		host:            u.Host,
		logger:          logger,
		authHeaderName:  name,
		authHeaderValue: value,
		queue:           make(chan []byte, cfg.QueueCapacity),
		done:            make(chan struct{}),
		stopped:         make(chan struct{}),
	}
	go s.run()
	logger.Info("Audit HTTP output ready", "host", u.Host,
		"queueCapacity", cfg.QueueCapacity, "batchMaxEvents", cfg.BatchMaxEvents, "flushInterval", cfg.FlushInterval)
	return s, nil
}

func (s *auditHTTPSink) Name() string { return config.AuditOutputHTTP }

// Write queues the line for delivery and never blocks. The line is copied
// because the caller reuses its buffer.
func (s *auditHTTPSink) Write(line []byte) {
	select {
	case <-s.done:
		// The sender is gone, so a queued line would never be sent.
		s.throttle.logError(s.logger, "Audit HTTP output is closed; dropping the record",
			config.AuditOutputHTTP, fmt.Errorf("output closed"))
		return
	default:
	}

	queued := make([]byte, len(line))
	copy(queued, line)
	select {
	case s.queue <- queued:
	default:
		s.throttle.logError(s.logger, "Audit HTTP output queue is full; dropping the record",
			config.AuditOutputHTTP, fmt.Errorf("queue capacity %d exhausted", s.cfg.QueueCapacity))
	}
}

// run is the sender: it collects lines into a batch, delivers it when a bound is
// reached, and flushes what is left on shutdown.
func (s *auditHTTPSink) run() {
	defer func() {
		// A Write can win the race against Close and queue a line after the final
		// drain. Report it rather than losing it silently.
		left := len(s.queue)
		for range left {
			<-s.queue
		}
		if left > 0 {
			s.logger.Error("Audit HTTP output closed with records still queued; dropping them",
				"sink", config.AuditOutputHTTP, "records", left)
		}
		close(s.stopped)
	}()

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([][]byte, 0, s.cfg.BatchMaxEvents)
	batchBytes := 0
	flush := func() {
		if len(batch) > 0 {
			s.deliver(batch)
			batch = make([][]byte, 0, s.cfg.BatchMaxEvents)
			batchBytes = 0
		}
	}
	add := func(line []byte) {
		// Send what is batched first if this line would take the body past
		// batch_max_bytes, so a body exceeds it only when one line alone does.
		if batchBytes+len(line)+1 > s.cfg.BatchMaxBytes {
			flush()
		}
		batch = append(batch, line)
		batchBytes += len(line) + 1
		if len(batch) >= s.cfg.BatchMaxEvents || batchBytes >= s.cfg.BatchMaxBytes {
			flush()
		}
	}

	for {
		select {
		case <-s.done:
			// Drain what is queued so a graceful shutdown keeps accepted records.
			for {
				select {
				case line := <-s.queue:
					add(line)
					continue
				default:
				}
				break
			}
			flush()
			return
		case line := <-s.queue:
			add(line)
		case <-ticker.C:
			flush()
		}
	}
}

// deliver POSTs one batch, retrying transport errors, 429 and 5xx with jittered
// exponential backoff. Any other 4xx means the receiver rejected the batch, so
// retrying would not help.
func (s *auditHTTPSink) deliver(batch [][]byte) {
	body := encodeAuditNDJSON(batch)
	var lastErr error
	// A receiver's Retry-After replaces the computed backoff rather than adding to it.
	var nextDelay time.Duration
	for attempt := 0; attempt <= s.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := nextDelay
			if delay <= 0 {
				delay = s.backoff(attempt)
			}
			if !s.sleep(delay) {
				break // shutting down; stop retrying
			}
		}
		retryAfter, retryable, err := s.post(body)
		if err == nil {
			return
		}
		lastErr = err
		if !retryable {
			break
		}
		nextDelay = retryAfter
	}
	s.throttle.logError(s.logger, "Failed to deliver audit records; dropping them", config.AuditOutputHTTP,
		fmt.Errorf("%d record(s) dropped: %w", len(batch), lastErr))
}

// post makes one delivery attempt. It reports whether a failure is worth
// retrying, and the receiver's Retry-After delay when it gives one.
func (s *auditHTTPSink) post(body []byte) (time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, false, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if s.authHeaderName != "" {
		req.Header.Set(s.authHeaderName, s.authHeaderValue)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, true, fmt.Errorf("posting batch: %w", err)
	}
	defer resp.Body.Close()
	// Drain a bounded prefix so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, auditMaxErrorBodyBytes))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if resp.TLS != nil {
			s.tlsLogged.Do(func() {
				s.logger.Info("Audit HTTP output connected", "host", s.host,
					"tlsVersion", tls.VersionName(resp.TLS.Version), "keyExchange", resp.TLS.CurveID.String())
			})
		}
		return 0, false, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return parseAuditRetryAfter(resp.Header.Get("Retry-After")), true, fmt.Errorf("receiver is rate limiting (429)")
	case resp.StatusCode >= 500:
		return 0, true, fmt.Errorf("receiver returned status %d", resp.StatusCode)
	default:
		return 0, false, fmt.Errorf("receiver rejected the batch with status %d", resp.StatusCode)
	}
}

// backoff returns the delay before retry attempt n (1-based): exponential, with
// jitter so replicas retrying after a shared outage do not do so in lockstep.
func (s *auditHTTPSink) backoff(attempt int) time.Duration {
	// Cap the exponent so a large max_retries cannot overflow the shift.
	delay := s.cfg.RetryBackoff << min(attempt-1, 10)
	if half := delay / 2; half > 0 {
		delay = half + time.Duration(rand.Int64N(int64(half)))
	}
	return delay
}

// sleep waits for d, returning false if Close was called first so shutdown is
// not held for a full backoff.
func (s *auditHTTPSink) sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-s.done:
		return false
	}
}

// Close stops the sender after a final flush, bounded by ctx. Safe to call more
// than once.
func (s *auditHTTPSink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() { close(s.done) })
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("audit HTTP output did not finish flushing: %w", ctx.Err())
	}
}

// encodeAuditNDJSON joins the batch into a newline-delimited body, which Splunk
// HEC's raw endpoint, Elasticsearch/OpenSearch, Fluent Bit and Vector accept as-is.
func encodeAuditNDJSON(batch [][]byte) []byte {
	var buf bytes.Buffer
	for _, line := range batch {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// parseAuditRetryAfter reads a Retry-After header in delta-seconds or HTTP-date
// form, capped at auditRetryAfterCap. Anything unparseable gives 0, which leaves
// the caller on its own backoff.
func parseAuditRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		// Clamp before converting: a large value overflows time.Duration.
		return time.Duration(min(secs, int(auditRetryAfterCap/time.Second))) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return min(max(time.Until(t), 0), auditRetryAfterCap)
	}
	return 0
}
