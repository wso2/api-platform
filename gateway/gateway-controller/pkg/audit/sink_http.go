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

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/httpkit/httpclient"
)

const (
	// A huge error body must not grow the heap.
	maxResponseBodyBytes int64 = 4 << 10
	// A receiver's Retry-After must not stall the sender for long.
	retryAfterCap = 30 * time.Second
)

// httpSink never blocks Write: lines go to a bounded in-memory queue, and a
// batch that cannot be delivered is dropped (the audit table still has it).
type httpSink struct {
	cfg      config.AuditHTTPConfig
	client   *http.Client
	host     string
	logger   *slog.Logger
	throttle errThrottle

	// authValue is a secret and is never logged.
	authName  string
	authValue string

	queue     chan []byte
	done      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
	// closeMu orders every enqueue before Close stops the sender, so the final
	// drain sees every accepted line.
	closeMu sync.RWMutex
	// The negotiated key exchange is logged once to show whether PQC is in use.
	tlsLogged sync.Once
}

func newHTTPSink(cfg config.AuditHTTPConfig, logger *slog.Logger) (*httpSink, error) {
	// Non-positive values would panic in time.NewTicker or make.
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
	client, err := newHTTPClient(cfg, u)
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

// Redirects are never followed: the receiver, not the operator, would choose the target.
func newHTTPClient(cfg config.AuditHTTPConfig, endpoint *url.URL) (*http.Client, error) {
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

func (s *httpSink) Write(line []byte) {
	s.closeMu.RLock()
	defer s.closeMu.RUnlock()
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

func (s *httpSink) run() {
	defer close(s.stopped)

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

// deliver retries transport errors, 429 and 5xx; any other status will not change on retry.
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

// Jitter keeps replicas from retrying in lockstep after a shared outage.
func (s *httpSink) backoff(attempt int) time.Duration {
	// Cap the exponent so a large max_retries cannot overflow the shift.
	delay := s.cfg.RetryBackoff << min(attempt-1, 10)
	if half := delay / 2; half > 0 {
		delay = half + time.Duration(rand.Int64N(int64(half)))
	}
	return delay
}

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

func (s *httpSink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		s.closeMu.Lock()
		close(s.done)
		s.closeMu.Unlock()
	})
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("audit HTTP output did not finish flushing: %w", ctx.Err())
	}
}

func encodeNDJSON(batch [][]byte) []byte {
	var buf bytes.Buffer
	for _, line := range batch {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// parseRetryAfter returns 0 for anything unparseable, leaving the caller on its own backoff.
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
