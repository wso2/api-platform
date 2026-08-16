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

package publishers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

const (
	// maxErrorBodyBytes caps how much of a receiver's error response is read
	// before being discarded. A receiver returning an enormous error body must not
	// be able to grow the gateway's heap.
	maxErrorBodyBytes int64 = 4 << 10
	// retryAfterCap bounds how long a receiver's Retry-After header can stall the
	// sender. Without it, a hostile or misconfigured receiver could park the
	// sending goroutine indefinitely while the queue overflows.
	retryAfterCap = 30 * time.Second
)

// httpSink batches traffic-log lines and POSTs them to an operator-named endpoint.
//
// It exists for deployments that cannot run a co-located log collector — no
// sidecar, no node-level agent, no access to the node filesystem. Nothing is
// written to disk at any point.
//
// Delivery semantics, and the reasoning behind them:
//
//   - Write never blocks. It runs on the ALS ingest path, so blocking there
//     backpressures Envoy's access-log stream and eventually Envoy itself. A line
//     that does not fit in the queue is dropped and counted.
//   - The queue is bounded. An unbounded queue in front of a bounded sender is
//     just deferred unbounded memory growth, and these lines carry request and
//     response bodies, so the growth would be fast.
//   - A delivery failure drops the batch. It is never written to stdout instead:
//     that would put the bodies into the container log, which is what this sink
//     exists to avoid.
//
// The consequence worth stating plainly to an operator: unlike a Fluent Bit or
// OpenTelemetry collector, which buffer to disk, this sink's queue is memory-only.
// A receiver outage costs events rather than merely delaying them. That is the
// deliberate trade for never touching the disk, and
// policy_engine_traffic_log_dropped_total is the series that makes it visible.
type httpSink struct {
	cfg    config.TrafficLogHTTPConfig
	client *http.Client

	// authHeader is the pre-computed header name/value applied to every request,
	// resolved once at construction. Empty name means no authentication. The value
	// is a secret and is never logged.
	authHeaderName  string
	authHeaderValue string

	// queue carries serialized lines from Write to the sender goroutine.
	queue chan []byte
	// dropOldest selects the eviction policy when the queue is full.
	dropOldest bool

	// done is closed by Close to stop the sender goroutine.
	done chan struct{}
	// stopped is closed by the sender goroutine once its final flush completes,
	// so Close can wait for it (bounded by the caller's context).
	stopped   chan struct{}
	closeOnce sync.Once

	throttle errThrottle
}

// newHTTPSink builds the sink and starts its sender goroutine.
//
// It returns an error rather than degrading: an HTTP sink that cannot be built must
// fail startup, never silently leave the operator writing bodies to stdout.
func newHTTPSink(cfg config.TrafficLogHTTPConfig) (*httpSink, error) {
	tlsCfg, err := buildTrafficLogTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}
	name, value, err := buildTrafficLogAuthHeader(cfg.Auth)
	if err != nil {
		return nil, err
	}

	s := &httpSink{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				TLSClientConfig:     tlsCfg,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
			// Never auto-follow a redirect: the redirect target is chosen by the
			// receiver, not the operator, and following it would send request and
			// response bodies to a destination nobody configured.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		authHeaderName:  name,
		authHeaderValue: value,
		queue:           make(chan []byte, cfg.QueueCapacity),
		dropOldest: strings.EqualFold(strings.TrimSpace(cfg.OnQueueFull),
			config.TrafficLogQueueDropOldest),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}

	// Publish the configured bound so an alert can express "the queue is 80%
	// full" as a ratio against queue_depth. A fixed depth threshold is wrong at
	// every capacity but one. Set here, next to the config it describes, rather
	// than in the factory — otherwise any sink built by another path reports a
	// depth with nothing to divide it by.
	if metrics.TrafficLogQueueCapacity != nil {
		metrics.TrafficLogQueueCapacity.WithLabelValues(sinkNameHTTP).Set(float64(cfg.QueueCapacity))
	}

	go s.run()
	slog.Info("Traffic logging HTTP sink ready",
		"endpoint", cfg.Endpoint,
		"queueCapacity", cfg.QueueCapacity,
		"batchMaxEvents", cfg.BatchMaxEvents,
		"flushInterval", cfg.FlushInterval)
	return s, nil
}

// Name returns the sink's identifier.
func (s *httpSink) Name() string { return sinkNameHTTP }

// Write enqueues the line for delivery. It never blocks: on a full queue it drops
// according to the configured policy and returns immediately.
//
// The line is copied because the caller owns the underlying array and reuses it
// after Publish returns; without the copy a queued line could be rewritten before
// the sender serializes it.
func (s *httpSink) Write(line []byte) {
	// After Close the sender goroutine is gone, so anything accepted here would
	// sit in the queue forever — delivered to nobody and counted as nothing. Fail
	// it explicitly instead, matching the file sink's closed-handle behaviour.
	select {
	case <-s.done:
		metrics.TrafficLogDroppedTotal.WithLabelValues(sinkNameHTTP, dropReasonSendFailed).Inc()
		return
	default:
	}

	queued := make([]byte, len(line))
	copy(queued, line)

	select {
	case s.queue <- queued:
		metrics.TrafficLogQueueDepth.WithLabelValues(sinkNameHTTP).Set(float64(len(s.queue)))
		return
	default:
	}

	if s.dropOldest {
		// Evict one old line and retry once. A single attempt is deliberate: a
		// loop here could spin while producers keep the queue full, turning a
		// non-blocking Write into an unbounded one.
		select {
		case <-s.queue:
			metrics.TrafficLogDroppedTotal.WithLabelValues(sinkNameHTTP, dropReasonQueueFull).Inc()
		default:
		}
		select {
		case s.queue <- queued:
			metrics.TrafficLogQueueDepth.WithLabelValues(sinkNameHTTP).Set(float64(len(s.queue)))
			return
		default:
		}
	}

	metrics.TrafficLogDroppedTotal.WithLabelValues(sinkNameHTTP, dropReasonQueueFull).Inc()
	s.throttle.logError("Traffic-log HTTP queue is full; dropping event", sinkNameHTTP,
		fmt.Errorf("queue capacity %d exhausted", s.cfg.QueueCapacity))
}

// run is the sender goroutine: it accumulates lines into a batch and delivers when
// any bound is reached, then performs one final flush on shutdown.
func (s *httpSink) run() {
	defer func() {
		// A Write can win the race against the done-check above and enqueue just
		// as the drain finishes. Account for anything left rather than letting it
		// vanish: silent loss is the one outcome the drop counter exists to rule
		// out.
		for {
			select {
			case <-s.queue:
				metrics.TrafficLogDroppedTotal.WithLabelValues(sinkNameHTTP, dropReasonSendFailed).Inc()
				continue
			default:
			}
			break
		}
		metrics.TrafficLogQueueDepth.WithLabelValues(sinkNameHTTP).Set(0)
		close(s.stopped)
	}()

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([][]byte, 0, s.cfg.BatchMaxEvents)
	batchBytes := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.deliver(batch)
		batch = make([][]byte, 0, s.cfg.BatchMaxEvents)
		batchBytes = 0
	}

	for {
		select {
		case <-s.done:
			// Drain whatever is still queued so a graceful shutdown does not lose
			// events that were accepted but not yet sent.
			for {
				select {
				case line := <-s.queue:
					batch = append(batch, line)
					batchBytes += len(line) + 1
					if len(batch) >= s.cfg.BatchMaxEvents || batchBytes >= s.cfg.BatchMaxBytes {
						flush()
					}
					continue
				default:
				}
				break
			}
			flush()
			return

		case line := <-s.queue:
			metrics.TrafficLogQueueDepth.WithLabelValues(sinkNameHTTP).Set(float64(len(s.queue)))
			batch = append(batch, line)
			batchBytes += len(line) + 1
			if len(batch) >= s.cfg.BatchMaxEvents || batchBytes >= s.cfg.BatchMaxBytes {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// deliver POSTs one batch, retrying transport errors, 429 and 5xx with jittered
// exponential backoff. A 4xx other than 429 is not retried: it means the receiver
// rejected the batch's shape, so retrying would only amplify a permanent failure.
func (s *httpSink) deliver(batch [][]byte) {
	body := encodeNDJSON(batch)
	start := time.Now()
	defer func() {
		metrics.TrafficLogFlushDurationSecond.WithLabelValues(sinkNameHTTP).
			Observe(time.Since(start).Seconds())
	}()

	var lastErr error
	// Delay to apply before the NEXT attempt. A receiver-supplied Retry-After
	// replaces our own backoff rather than adding to it: sleeping both made a
	// "Retry-After: 2" wait 2s + the exponential delay, ignoring the receiver's
	// own pacing and holding the batch longer than it asked for.
	var nextDelay time.Duration
	for attempt := 0; attempt <= s.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := nextDelay
			if delay <= 0 {
				delay = s.backoff(attempt)
			}
			if !s.sleep(delay) {
				break // shutting down; stop retrying and drop
			}
		}

		retryAfter, err := s.post(body)
		if err == nil {
			metrics.TrafficLogWrittenTotal.WithLabelValues(sinkNameHTTP).Add(float64(len(batch)))
			return
		}
		lastErr = err

		var perm *permanentDeliveryError
		if errors.As(err, &perm) {
			break // 4xx — retrying cannot help
		}
		nextDelay = retryAfter // 0 unless the receiver asked for a specific delay
	}

	metrics.TrafficLogDroppedTotal.WithLabelValues(sinkNameHTTP, dropReasonSendFailed).
		Add(float64(len(batch)))
	s.throttle.logError("Failed to deliver traffic-log batch; dropping events", sinkNameHTTP,
		fmt.Errorf("%d event(s) dropped after %d attempt(s): %w", len(batch), s.cfg.MaxRetries+1, lastErr))
}

// permanentDeliveryError marks a response that must not be retried.
type permanentDeliveryError struct{ status int }

func (e *permanentDeliveryError) Error() string {
	return fmt.Sprintf("receiver rejected the batch with status %d", e.status)
}

// post performs one delivery attempt. It returns the receiver's requested
// Retry-After delay when it supplies one, so the caller can honor it instead of its
// own backoff.
func (s *httpSink) post(body []byte) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", s.cfg.ContentType)
	if s.authHeaderName != "" {
		req.Header.Set(s.authHeaderName, s.authHeaderValue)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		metrics.TrafficLogWriteErrorsTotal.WithLabelValues(sinkNameHTTP, errCodeTransport).Inc()
		// The error can embed the endpoint URL but never the body, so no
		// request/response payload can leak into the application log here.
		return 0, fmt.Errorf("posting batch: %w", err)
	}
	defer resp.Body.Close()

	// Drain a bounded prefix so the connection can be reused, and discard the rest.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return 0, nil
	}
	metrics.TrafficLogWriteErrorsTotal.WithLabelValues(sinkNameHTTP, strconv.Itoa(resp.StatusCode)).Inc()

	if resp.StatusCode == http.StatusTooManyRequests {
		return parseRetryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("receiver is rate limiting (429)")
	}
	if resp.StatusCode >= 500 {
		return 0, fmt.Errorf("receiver returned status %d", resp.StatusCode)
	}
	return 0, &permanentDeliveryError{status: resp.StatusCode}
}

// backoff returns the delay before the given retry attempt (1-based), growing
// exponentially with full jitter applied. Jitter matters because every replica
// retries against the same receiver after a shared outage; without it they would
// reconverge into a synchronized thundering herd on the first recovery.
func (s *httpSink) backoff(attempt int) time.Duration {
	base := s.cfg.RetryBackoff
	if base <= 0 {
		base = time.Second
	}
	// Cap the exponent so a large max_retries cannot overflow the shift.
	shift := attempt - 1
	if shift > 10 {
		shift = 10
	}
	delay := base << shift
	if half := delay / 2; half > 0 {
		delay = half + time.Duration(rand.Int64N(int64(half)))
	}
	return delay
}

// sleep waits for d, returning false if shutdown was requested first so the caller
// stops retrying instead of holding shutdown open for the full backoff.
func (s *httpSink) sleep(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-s.done:
		return false
	}
}

// Close stops the sender goroutine after a final flush, bounded by ctx. Safe to
// call more than once.
func (s *httpSink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() { close(s.done) })
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("traffic-log HTTP sink did not finish flushing: %w", ctx.Err())
	}
}

// encodeNDJSON joins the batch into a newline-delimited body. Each element is
// already a complete JSON object produced by the Log publisher.
//
// This shape is accepted directly by Splunk HEC's /services/collector/raw,
// Elasticsearch and OpenSearch _bulk, Grafana Loki via its OTLP/JSON push path,
// Fluent Bit's http input, and the OpenTelemetry Collector, so no receiver-specific
// envelope is needed for any of them.
func encodeNDJSON(batch [][]byte) []byte {
	total := 0
	for _, line := range batch {
		total += len(line) + 1
	}
	buf := bytes.NewBuffer(make([]byte, 0, total))
	for _, line := range batch {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// parseRetryAfter interprets a Retry-After header in either delta-seconds or
// HTTP-date form, clamped to retryAfterCap. Anything unparseable yields 0, which
// leaves the caller on its own backoff.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return capDuration(time.Duration(secs) * time.Second)
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return capDuration(d)
		}
	}
	return 0
}

func capDuration(d time.Duration) time.Duration {
	if d > retryAfterCap {
		return retryAfterCap
	}
	return d
}

// buildTrafficLogAuthHeader resolves the configured authentication into a single
// header name/value pair. Returns empty strings when no authentication is
// configured. Error messages never include the secret material.
func buildTrafficLogAuthHeader(cfg config.TrafficLogHTTPAuthConfig) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", config.TrafficLogAuthNone:
		return "", "", nil
	case config.TrafficLogAuthBearer:
		if cfg.Bearer.Token == "" {
			return "", "", fmt.Errorf("auth: bearer.token is required when type is %q",
				config.TrafficLogAuthBearer)
		}
		return "Authorization", "Bearer " + cfg.Bearer.Token, nil
	case config.TrafficLogAuthBasic:
		if cfg.Basic.Username == "" || cfg.Basic.Password == "" {
			return "", "", fmt.Errorf("auth: basic.username and basic.password are both required when type is %q",
				config.TrafficLogAuthBasic)
		}
		// Reuse net/http's own encoding so the header matches what a server expects.
		req := &http.Request{Header: http.Header{}}
		req.SetBasicAuth(cfg.Basic.Username, cfg.Basic.Password)
		return "Authorization", req.Header.Get("Authorization"), nil
	case config.TrafficLogAuthHeader:
		if cfg.Header.Name == "" || cfg.Header.Value == "" {
			return "", "", fmt.Errorf("auth: header.name and header.value are both required when type is %q",
				config.TrafficLogAuthHeader)
		}
		return cfg.Header.Name, cfg.Header.Value, nil
	default:
		return "", "", fmt.Errorf("auth: unknown type %q", cfg.Type)
	}
}

// buildTrafficLogTLSConfig assembles the client TLS configuration.
//
// X25519MLKEM768 is listed first in CurvePreferences per the repository's
// post-quantum standard: the traffic log carries request and response bodies, which
// is exactly the long-lived-confidentiality content a harvest-now-decrypt-later
// adversary would target. X25519 remains as the classical leg of that hybrid.
func buildTrafficLogTLSConfig(cfg config.TrafficLogHTTPTLSConfig) (*tls.Config, error) {
	out := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		InsecureSkipVerify: cfg.InsecureSkipVerify, // #nosec G402 -- off by default; opt-in warns at startup
	}

	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tls: cannot read ca_file %q: %w", cfg.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("tls: ca_file %q contains no usable PEM certificate", cfg.CAFile)
		}
		out.RootCAs = pool
	}

	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return nil, fmt.Errorf("tls: cert_file and key_file must be set together for mTLS")
	}
	if cfg.CertFile != "" {
		pair, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("tls: cannot load client certificate/key pair: %w", err)
		}
		out.Certificates = []tls.Certificate{pair}
	}
	return out, nil
}
