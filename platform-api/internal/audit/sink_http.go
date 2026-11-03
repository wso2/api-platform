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

package audit

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

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/platform-api/config"
)

const (
	// maxResponseBodyBytes caps how much of a receiver's response is read, so a
	// huge error body cannot grow the heap.
	maxResponseBodyBytes int64 = 4 << 10
	// retryAfterCap bounds how long a receiver's Retry-After can stall the
	// sender while the queue fills.
	retryAfterCap = 30 * time.Second
)

// httpSink batches audit lines and POSTs them as newline-delimited JSON to an
// operator-chosen endpoint, for deployments without a log collector.
//
//   - Write never blocks: it runs on the request path. A line that does not fit
//     in the queue is dropped and logged.
//   - The queue is bounded, so a receiver outage cannot grow memory without limit.
//   - A batch that cannot be delivered is dropped. The queue is memory-only, so an
//     outage longer than the queue and retries can absorb loses records from this
//     output; the audit table still has them.
type httpSink struct {
	cfg      config.AuditHTTP
	client   *http.Client
	host     string
	logger   *slog.Logger
	throttle errThrottle

	// authName/authValue are resolved once; an empty name means none. The value
	// is a secret and is never logged.
	authName  string
	authValue string

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

// newHTTPSink builds the sink and starts its sender.
func newHTTPSink(cfg config.AuditHTTP, logger *slog.Logger) (*httpSink, error) {
	// Config validation rejects these; checked again because a non-positive value
	// would panic in time.NewTicker or make.
	if cfg.FlushInterval <= 0 || cfg.RequestTimeout <= 0 || cfg.QueueCapacity <= 0 ||
		cfg.BatchMaxEvents <= 0 || cfg.BatchMaxBytes <= 0 || cfg.MaxRetries < 0 {
		return nil, fmt.Errorf("batch, queue and timeout settings must be positive")
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("endpoint is not a valid absolute URL")
	}
	authName, authValue, err := cfg.Auth.AuthHeader()
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	client, err := newClient(cfg, u)
	if err != nil {
		return nil, fmt.Errorf("tls: %w", err)
	}

	s := &httpSink{
		cfg:       cfg,
		client:    client,
		host:      u.Host,
		logger:    logger,
		authName:  authName,
		authValue: authValue,
		queue:     make(chan []byte, cfg.QueueCapacity),
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
	go s.run()
	logger.Info("Audit HTTP output ready", "host", u.Host,
		"queueCapacity", cfg.QueueCapacity, "batchMaxEvents", cfg.BatchMaxEvents, "flushInterval", cfg.FlushInterval)
	return s, nil
}

// newClient builds the client with httpkit, reading the TLS files for an
// https:// endpoint (the tls block is ignored for http://). X25519MLKEM768 is
// preferred, with classical curves after it for receivers that lack it.
// Redirects are never followed: the target is chosen by the receiver, not the
// operator.
func newClient(cfg config.AuditHTTP, endpoint *url.URL) (*http.Client, error) {
	c := httpclient.DefaultConfig()
	c.Timeouts.Overall = cfg.RequestTimeout
	// httpkit's 10s response-header default would otherwise cut a longer one short.
	c.Timeouts.ResponseHeader = cfg.RequestTimeout
	c.Pooling.MaxIdleConnsPerHost = 2
	if endpoint.Scheme == "https" {
		c.TLS.MinVersion = "TLS1_2"
		c.TLS.MaxVersion = "TLS1_3"
		c.TLS.CurvePreferences = "X25519MLKEM768,X25519,P-256,P-384"
		c.TLS.RootCAFile = cfg.TLS.CAFile
		c.TLS.ClientCertFile = cfg.TLS.CertFile
		c.TLS.ClientKeyFile = cfg.TLS.KeyFile
		c.TLS.InsecureSkipVerify = cfg.TLS.InsecureSkipVerify // #nosec G402 -- off by default; warned at startup
		c.TLS.InsecureSkipVerifyAcknowledged = cfg.TLS.InsecureSkipVerify
	}
	client, err := httpclient.New(c)
	if err != nil {
		return nil, err
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client, nil
}

func (s *httpSink) Name() string { return config.AuditOutputHTTP }

// Write queues the line for delivery and never blocks. The line is copied
// because the caller reuses its buffer.
func (s *httpSink) Write(line []byte) {
	select {
	case <-s.done:
		// The sender is gone, so a queued line would never be sent.
		s.throttle.logError(s.logger, "Audit HTTP output is closed; dropping the record", s.Name(),
			fmt.Errorf("output closed"))
		return
	default:
	}

	queued := make([]byte, len(line))
	copy(queued, line)
	select {
	case s.queue <- queued:
	default:
		s.throttle.logError(s.logger, "Audit HTTP output queue is full; dropping the record", s.Name(),
			fmt.Errorf("queue capacity %d exhausted", s.cfg.QueueCapacity))
	}
}

// run is the sender: it collects lines into a batch, delivers it when a bound is
// reached, and flushes what is left on shutdown.
func (s *httpSink) run() {
	defer func() {
		// A Write can win the race against Close and queue a line after the final
		// drain. Report it rather than losing it silently.
		left := len(s.queue)
		for range left {
			<-s.queue
		}
		if left > 0 {
			s.logger.Error("Audit HTTP output closed with records still queued; dropping them",
				"sink", s.Name(), "records", left)
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
func (s *httpSink) deliver(batch [][]byte) {
	body := encodeNDJSON(batch)
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
	s.throttle.logError(s.logger, "Failed to deliver audit records; dropping them", s.Name(),
		fmt.Errorf("%d record(s) dropped: %w", len(batch), lastErr))
}

// post makes one delivery attempt. It reports whether a failure is worth
// retrying, and the receiver's Retry-After delay when it gives one.
func (s *httpSink) post(body []byte) (time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, false, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if s.authName != "" {
		req.Header.Set(s.authName, s.authValue)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, true, fmt.Errorf("posting batch: %w", err)
	}
	defer resp.Body.Close()
	// Drain a bounded prefix so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyBytes))

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
		return parseRetryAfter(resp.Header.Get("Retry-After")), true, fmt.Errorf("receiver is rate limiting (429)")
	case resp.StatusCode >= 500:
		return 0, true, fmt.Errorf("receiver returned status %d", resp.StatusCode)
	default:
		return 0, false, fmt.Errorf("receiver rejected the batch with status %d", resp.StatusCode)
	}
}

// backoff returns the delay before retry attempt n (1-based): exponential, with
// jitter so replicas retrying after a shared outage do not do so in lockstep.
func (s *httpSink) backoff(attempt int) time.Duration {
	// Cap the exponent so a large max_retries cannot overflow the shift.
	delay := s.cfg.RetryBackoff << min(attempt-1, 10)
	if half := delay / 2; half > 0 {
		delay = half + time.Duration(rand.Int64N(int64(half)))
	}
	return delay
}

// sleep waits for d, returning false if Close was called first so shutdown is
// not held for a full backoff.
func (s *httpSink) sleep(d time.Duration) bool {
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
func (s *httpSink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() { close(s.done) })
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("audit HTTP output did not finish flushing: %w", ctx.Err())
	}
}

// encodeNDJSON joins the batch into a newline-delimited body, which Splunk HEC's
// raw endpoint, Elasticsearch/OpenSearch, Fluent Bit and Vector accept as-is.
func encodeNDJSON(batch [][]byte) []byte {
	var buf bytes.Buffer
	for _, line := range batch {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// parseRetryAfter reads a Retry-After header in delta-seconds or HTTP-date form,
// capped at retryAfterCap. Anything unparseable gives 0, which leaves the caller
// on its own backoff.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		// Clamp before converting: a large value overflows time.Duration.
		return time.Duration(min(secs, int(retryAfterCap/time.Second))) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return min(max(time.Until(t), 0), retryAfterCap)
	}
	return 0
}
