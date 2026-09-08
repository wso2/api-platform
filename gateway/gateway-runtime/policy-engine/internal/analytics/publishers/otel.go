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
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// Identifiers with more than one reader. Individual attribute names are written
// as literals at their single call site: sharing them with the tests would let a
// typo satisfy its own assertion.
const (
	// otelAttrNamespace prefixes every attribute OpenTelemetry does not define.
	otelAttrNamespace = "wso2"
	// otelScopeName is the InstrumentationScope downstream pipelines route on. It
	// must match the collector's scope filter; a mismatch fails silently.
	otelScopeName = otelAttrNamespace + ".analytics"
	// otelEventName is emitted both as the record's eventName and as an
	// event.name attribute — collectors before ~v0.117 drop the former.
	otelEventName = otelAttrNamespace + ".api.transaction"

	otelSeverityNumberInfo = 9
	otelSeverityTextInfo   = "INFO"

	// otelCloseFlushTimeout bounds the shutdown flush when the caller's context
	// carries no deadline.
	otelCloseFlushTimeout = 5 * time.Second
	// otelPublisherName is this publisher's value for the `publisher` metric
	// label. Kept local so metric call sites need no config import.
	otelPublisherName = "otel"
	// otelMaxResponseBytes caps how much of the endpoint's response is read. A
	// 2xx body carries partialSuccess and a failure body carries an error
	// message; neither may be allowed to grow the heap.
	otelMaxResponseBytes int64 = 4 << 10
)

// ns qualifies an attribute name with the WSO2 namespace.
func ns(name string) string { return otelAttrNamespace + "." + name }

// OTel exports analytics events to an OpenTelemetry collector as OTLP log
// records ("OTel Events": log records carrying an event.name), over OTLP/HTTP
// with a JSON body.
//
// The logs signal is used rather than traces or metrics because it is the only
// OTLP signal that represents one discrete transaction with per-consumer
// attributes intact: metrics aggregate the transaction away and impose a
// cardinality ceiling, and traces are sampled by design.
//
// The OTLP wire format is built directly rather than through the OpenTelemetry Go
// SDK, whose logs signal is still pre-1.0 (sdk/log v0.x). The OTLP/HTTP JSON
// encoding it would produce is stable, so only the transport would change.
//
// Publish never blocks and never performs I/O on the caller's goroutine: the ALS
// ingest path calls it, and blocking there backpressures Envoy's access-log
// stream. Records go to a bounded queue drained by one worker; a full queue drops
// and counts rather than growing.
type OTel struct {
	cfg    config.OTelPublisherConfig
	client *http.Client

	queue chan *otelLogRecord

	stop       chan struct{}
	workerDone chan struct{}
	closeOnce  sync.Once
	closeErr   error

	droppedMu sync.Mutex
	dropped   int
	// dropOldest is resolved once at construction rather than per record.
	dropOldest bool
	// gzip is resolved once at construction from cfg.Compression.
	gzip bool
	// retryAbortDepth is the queue depth at which a retrying batch gives up so
	// the worker can resume draining. 0 disables the check. See export.
	retryAbortDepth int
}

// NewOTel creates the OTLP-logs publisher and starts its exporting worker.
// cfg is assumed validated by config.validateOTelPublisherConfig; the TLS
// material is loaded again here, so this constructor fails closed rather than
// starting a publisher that can never reach its endpoint.
func NewOTel(cfg *config.OTelPublisherConfig) (*OTel, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	tlsCfg, err := buildOTelTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	o := &OTel{
		cfg: *cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				TLSClientConfig:     tlsCfg,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     90 * time.Second,
			},
			// Never auto-follow a redirect: the target is chosen by the endpoint,
			// not the operator, and following it would send analytics records to a
			// destination nobody configured.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		queue:      make(chan *otelLogRecord, cfg.QueueCapacity),
		stop:       make(chan struct{}),
		workerDone: make(chan struct{}),
		dropOldest: strings.EqualFold(strings.TrimSpace(cfg.OnQueueFull), config.QueueDropOldest),
		gzip: strings.EqualFold(strings.TrimSpace(cfg.Compression),
			config.OTelCompressionGzip),
		retryAbortDepth: cfg.EffectiveRetryAbortDepth(),
	}
	o.initMetrics()
	go o.run()

	if u, err := url.Parse(cfg.Endpoint); err == nil && u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		slog.Warn("OTel publisher is exporting analytics over plaintext HTTP to a non-loopback endpoint",
			"endpoint", cfg.Endpoint)
	}
	// Headers are deliberately omitted: they carry credentials.
	slog.Info("OTel analytics publisher started",
		"endpoint", cfg.Endpoint, "batchSize", cfg.BatchSize,
		"flushInterval", cfg.FlushInterval, "queueCapacity", cfg.QueueCapacity,
		"onQueueFull", cfg.OnQueueFull)
	return o, nil
}

// buildOTelTLSConfig assembles the client TLS configuration.
//
// X25519MLKEM768 leads CurvePreferences per the repository's post-quantum
// standard; the classical curves stay listed after it so an endpoint that does
// not yet offer the hybrid still completes a handshake.
func buildOTelTLSConfig(cfg config.OTelTLSConfig) (*tls.Config, error) {
	out := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384,
		},
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

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Publish converts the event to an OTLP log record and enqueues it.
//
// Never blocks: analytics is strictly downstream of request handling, so a full
// queue costs a record and never a request.
func (o *OTel) Publish(event *dto.Event) {
	if event == nil {
		return
	}
	record := o.buildRecord(event)

	select {
	case o.queue <- record:
		mAnalyticsQueueDepth(otelPublisherName, len(o.queue))
		return
	default:
	}

	if o.dropOldest {
		// Evict one old record and retry once. A single attempt is deliberate: a
		// loop could spin while producers keep the queue full, turning a
		// non-blocking Publish into an unbounded one.
		select {
		case <-o.queue:
			o.countQueueDrop()
		default:
		}
		select {
		case o.queue <- record:
			mAnalyticsQueueDepth(otelPublisherName, len(o.queue))
			return
		default:
		}
	}

	o.countQueueDrop()
}

// countQueueDrop records one record dropped for a full queue, warning on the
// first and then every hundredth so a sustained outage cannot flood the log.
func (o *OTel) countQueueDrop() {
	mAnalyticsDropped(otelPublisherName, dropReasonQueueFull, 1)
	count := o.countDrops(1)
	if count == 1 || count%100 == 0 {
		slog.Warn("OTel publisher queue full; dropping analytics event",
			"droppedTotal", count, "queueCapacity", o.cfg.QueueCapacity,
			"onQueueFull", o.cfg.OnQueueFull)
	}
}

// run drains the queue, exporting on a full batch or on the flush interval.
func (o *OTel) run() {
	defer func() {
		// The queue is not drained further after this point, so leaving the last
		// non-zero depth published would read as a permanently backed-up queue.
		mAnalyticsQueueDepth(otelPublisherName, 0)
		close(o.workerDone)
	}()

	ticker := time.NewTicker(o.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]*otelLogRecord, 0, o.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		o.export(batch)
		batch = batch[:0]
	}

	for {
		select {
		case record := <-o.queue:
			mAnalyticsQueueDepth(otelPublisherName, len(o.queue))
			batch = append(batch, record)
			if len(batch) >= o.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-o.stop:
			// Drain what is still queued so a rolling update does not lose it.
			for drained := true; drained; {
				select {
				case record := <-o.queue:
					mAnalyticsQueueDepth(otelPublisherName, len(o.queue))
					batch = append(batch, record)
					if len(batch) >= o.cfg.BatchSize {
						flush()
					}
				default:
					drained = false
				}
			}
			flush()
			return
		}
	}
}

// Close stops the worker and flushes what is buffered, satisfying Closer.
func (o *OTel) Close(ctx context.Context) error {
	o.closeOnce.Do(func() {
		close(o.stop)

		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, otelCloseFlushTimeout)
			defer cancel()
		}

		select {
		case <-o.workerDone:
		case <-ctx.Done():
			o.closeErr = fmt.Errorf("otel publisher shutdown timed out: %w", ctx.Err())
		}

		o.droppedMu.Lock()
		dropped := o.dropped
		o.droppedMu.Unlock()
		slog.Info("OTel analytics publisher stopped", "droppedTotal", dropped)
	})
	return o.closeErr
}

// export builds the OTLP payload for one batch and delivers it, retrying
// transport errors, 429 and 5xx with jittered exponential backoff. Any other 4xx
// means the endpoint rejected the payload's shape, so retrying would only amplify
// a permanent failure.
func (o *OTel) export(batch []*otelLogRecord) {
	records := make([]otelLogRecord, 0, len(batch))
	for _, r := range batch {
		records = append(records, *r)
	}

	resource := newOTelAttrs().
		str("service.name", o.cfg.ServiceName).
		str("service.version", o.cfg.ServiceVersion)
	for k, v := range o.cfg.ResourceAttributes {
		resource.str(k, v)
	}

	body, err := json.Marshal(otelExportRequest{
		ResourceLogs: []otelResourceLogs{{
			Resource: otelResource{Attributes: resource.list()},
			ScopeLogs: []otelScopeLogs{{
				Scope:      otelScope{Name: otelScopeName},
				LogRecords: records,
			}},
		}},
	})
	if err != nil {
		slog.Error("OTel publisher failed to marshal OTLP payload", "error", err, "records", len(records))
		o.dropRecords(dropReasonSerializeFailed, len(records))
		return
	}
	if o.gzip {
		compressed, err := gzipBytes(body)
		if err != nil {
			slog.Error("OTel publisher failed to compress OTLP payload", "error", err, "records", len(records))
			o.dropRecords(dropReasonSerializeFailed, len(records))
			return
		}
		body = compressed
	}

	// Covers every attempt and the waits between them: that total is what holds
	// the worker, and therefore what lets the queue fill behind it.
	start := time.Now()
	defer func() { mAnalyticsExportDuration(otelPublisherName, time.Since(start).Seconds()) }()

	var lastErr error
	// Delay before the NEXT attempt. A Retry-After replaces our own backoff
	// rather than adding to it, so the endpoint's own pacing is what applies.
	var nextDelay time.Duration
	for attempt := 0; attempt <= o.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Head-of-line check, before committing to another wait. One worker
			// exports, so nothing drains the queue while this batch retries. Past
			// the abort depth, retrying to save this batch costs more newer records
			// to queue-full than it rescues — so abandon it and resume draining.
			if depth := len(o.queue); o.retryAbortDepth > 0 && depth >= o.retryAbortDepth {
				o.dropRecords(dropReasonBackpressure, len(records))
				slog.Error("OTel publisher abandoning batch retries to resume draining; the endpoint "+
					"is reachable but too slow to keep up",
					"records", len(records), "attempts", attempt,
					"queueDepth", depth, "queueCapacity", o.cfg.QueueCapacity, "error", lastErr)
				return
			}
			if !o.sleep(nextDelay, attempt) {
				break // shutting down: stop retrying rather than hold shutdown open
			}
		}

		retryAfter, err := o.post(body, len(records))
		if err == nil {
			mAnalyticsPublished(otelPublisherName, len(records))
			return
		}
		lastErr = err

		var perm *otelPermanentExportError
		if errors.As(err, &perm) {
			break // 4xx other than 429 — retrying cannot help
		}
		nextDelay = retryAfter // 0 unless the endpoint asked for a specific delay
	}

	o.dropRecords(dropReasonSendFailed, len(records))
	slog.Error("OTel publisher failed to export analytics batch; dropping records",
		"records", len(records), "attempts", o.cfg.MaxRetries+1,
		"endpoint", o.cfg.Endpoint, "error", lastErr)
}

// otelPermanentExportError marks a response that must not be retried.
type otelPermanentExportError struct{ status int }

func (e *otelPermanentExportError) Error() string {
	return fmt.Sprintf("endpoint rejected the batch with status %d", e.status)
}

// post performs one export attempt, returning the endpoint's requested
// Retry-After when it supplies one so the caller can honor it over its own
// backoff.
func (o *OTel) post(body []byte, records int) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), o.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	for k, v := range o.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		mAnalyticsExportError(otelPublisherName, errCodeTransport, 1)
		// The error can embed the endpoint URL but never the payload, so no
		// request data can leak into the application log here.
		return 0, fmt.Errorf("posting batch: %w", err)
	}
	defer resp.Body.Close()

	// Read a bounded prefix: enough for the partialSuccess field or an error
	// message, and capped so a hostile endpoint cannot grow the heap.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, otelMaxResponseBytes))
	_, _ = io.Copy(io.Discard, resp.Body) // drain the rest so the connection is reusable

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		o.logPartialSuccess(respBody, records)
		return 0, nil
	}
	mAnalyticsExportError(otelPublisherName, strconv.Itoa(resp.StatusCode), 1)

	if resp.StatusCode == http.StatusTooManyRequests {
		return parseRetryAfter(resp.Header.Get("Retry-After")),
			fmt.Errorf("endpoint is rate limiting (429)")
	}
	if resp.StatusCode >= 500 {
		return 0, fmt.Errorf("endpoint returned status %d: %s", resp.StatusCode, otelResponseExcerpt(respBody))
	}
	return 0, &otelPermanentExportError{status: resp.StatusCode}
}

// logPartialSuccess reports records the endpoint accepted the request for but
// rejected. Without this a 200 carrying rejectedLogRecords looks like a clean
// export, and the records are silently gone.
func (o *OTel) logPartialSuccess(respBody []byte, records int) {
	if len(respBody) == 0 {
		return
	}
	var parsed otelExportResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return // a non-JSON 2xx body is not an error; nothing to report
	}
	// Proto3 JSON encodes int64 as a string, but some receivers emit a bare
	// number, so the field is json.Number to accept either.
	rejected, err := parsed.PartialSuccess.RejectedLogRecords.Int64()
	if err != nil || rejected <= 0 {
		return
	}
	o.dropRecords(dropReasonRejected, int(rejected))
	slog.Error("OTel endpoint accepted the export but rejected records",
		"rejected", rejected, "records", records,
		"endpointMessage", parsed.PartialSuccess.ErrorMessage)
}

// sleep waits out the backoff before a retry, returning false if shutdown was
// requested first. delay is the endpoint's Retry-After when it supplied one,
// otherwise the jittered exponential backoff for this attempt.
func (o *OTel) sleep(delay time.Duration, attempt int) bool {
	if delay <= 0 {
		delay = o.backoff(attempt)
	}
	if delay <= 0 {
		return true
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-o.stop:
		return false
	}
}

// backoff returns the delay before the given retry attempt (1-based), growing
// exponentially with full jitter. Jitter matters because every replica retries
// against the same endpoint after a shared outage; without it they reconverge
// into a synchronized herd on the first recovery.
func (o *OTel) backoff(attempt int) time.Duration {
	base := o.cfg.RetryBackoff
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

// dropRecords counts n records lost for the given reason, on both the local
// total and the labelled metric.
func (o *OTel) dropRecords(reason string, n int) {
	if n <= 0 {
		return
	}
	mAnalyticsDropped(otelPublisherName, reason, n)
	o.countDrops(n)
}

// countDrops adds n to the dropped total and returns the new total. It does not
// log: every caller has something more specific to say than "a record was lost".
func (o *OTel) countDrops(n int) int {
	if n <= 0 {
		return 0
	}
	o.droppedMu.Lock()
	o.dropped += n
	total := o.dropped
	o.droppedMu.Unlock()
	return total
}

// Metric helpers.
//
// Every analytics-publisher metric goes through these rather than touching the
// package vars directly. The vars are nil until metrics.Init() runs — main()
// calls it long before any publisher exists, but a constructor must not depend
// on that ordering, and guarding only in the constructor while the export path
// dereferences freely turns a startup panic into a first-request panic.

func mAnalyticsPublished(publisher string, n int) {
	if metrics.AnalyticsPublishedTotal != nil {
		metrics.AnalyticsPublishedTotal.WithLabelValues(publisher).Add(float64(n))
	}
}

func mAnalyticsDropped(publisher, reason string, n int) {
	if metrics.AnalyticsDroppedTotal != nil {
		metrics.AnalyticsDroppedTotal.WithLabelValues(publisher, reason).Add(float64(n))
	}
}

func mAnalyticsQueueDepth(publisher string, depth int) {
	if metrics.AnalyticsQueueDepth != nil {
		metrics.AnalyticsQueueDepth.WithLabelValues(publisher).Set(float64(depth))
	}
}

func mAnalyticsQueueCapacity(publisher string, capacity int) {
	if metrics.AnalyticsQueueCapacity != nil {
		metrics.AnalyticsQueueCapacity.WithLabelValues(publisher).Set(float64(capacity))
	}
}

func mAnalyticsExportDuration(publisher string, seconds float64) {
	if metrics.AnalyticsExportDurationSeconds != nil {
		metrics.AnalyticsExportDurationSeconds.WithLabelValues(publisher).Observe(seconds)
	}
}

func mAnalyticsExportError(publisher, code string, n int) {
	if metrics.AnalyticsExportErrorsTotal != nil {
		metrics.AnalyticsExportErrorsTotal.WithLabelValues(publisher, code).Add(float64(n))
	}
}

// initOTelMetrics materializes this publisher's counters at zero.
//
// A labelled Prometheus counter does not exist in the scrape until it is first
// incremented, so on a healthy gateway analytics_dropped_total is simply absent.
// That makes a dashboard panel read "No data" rather than 0, and leaves an
// operator unable to tell "nothing was dropped" from "the metrics path is
// broken" — an unacceptable ambiguity for the one series that makes silent
// analytics loss visible.
//
// Export-error codes are deliberately not pre-created: the label carries the
// HTTP status, which is unbounded, and materializing every possible status would
// be worse than the gap it closes.
func (o *OTel) initMetrics() {
	mAnalyticsPublished(otelPublisherName, 0)
	for _, reason := range []string{
		dropReasonQueueFull, dropReasonSendFailed, dropReasonBackpressure,
		dropReasonRejected, dropReasonSerializeFailed,
	} {
		mAnalyticsDropped(otelPublisherName, reason, 0)
	}
	mAnalyticsExportError(otelPublisherName, errCodeTransport, 0)
	mAnalyticsQueueCapacity(otelPublisherName, o.cfg.QueueCapacity)
	mAnalyticsQueueDepth(otelPublisherName, 0)
}

// otelResponseExcerpt renders a bounded, single-line excerpt of an endpoint's
// error body for the log. The body is the endpoint's own text, never ours.
func otelResponseExcerpt(body []byte) string {
	const maxExcerpt = 256
	excerpt := strings.TrimSpace(string(body))
	if len(excerpt) > maxExcerpt {
		excerpt = excerpt[:maxExcerpt] + "..."
	}
	return strings.ReplaceAll(excerpt, "\n", " ")
}

// gzipBytes compresses the payload for Content-Encoding: gzip.
func gzipBytes(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(body); err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// buildRecord maps the canonical analytics event onto one OTLP log record.
//
// Naming follows stable OpenTelemetry conventions where they exist, the GenAI and
// MCP conventions (both Development-stability, and now maintained in
// open-telemetry/semantic-conventions-genai) for AI fields, and the wso2.*
// namespace where OpenTelemetry defines nothing — API-product concepts and cost,
// chiefly. See gateway/spec/analytics-otel-attribute-mapping.md.
func (o *OTel) buildRecord(event *dto.Event) *otelLogRecord {
	attrs := newOTelAttrs()
	attrs.str("event.name", otelEventName)

	// HTTP. APIResourceTemplate already carries the full path including the API
	// context; the context is only a fallback when there is no template.
	route := ""
	if event.Operation != nil {
		route = event.Operation.APIResourceTemplate
		attrs.str("http.request.method", event.Operation.APIMethod)
		attrs.str("http.route", route)
	}
	if route == "" && event.API != nil {
		route = event.API.APIContext
	}
	attrs.str("url.path", route)
	attrs.i64("http.response.status_code", int64(event.ProxyResponseCode))
	attrs.str("client.address", event.UserIP)
	attrs.str("user_agent.original", event.UserAgentHeader)
	attrs.anyInt("http.request.body.size", event.Properties["requestSize"])
	attrs.anyInt("http.response.body.size", event.Properties["responseSize"])
	attrs.anyStr(ns("response.content_type"), event.Properties["responseContentType"])

	// API identity.
	if event.API != nil {
		attrs.str(ns("api.id"), event.API.APIID)
		attrs.str(ns("api.name"), event.API.APIName)
		attrs.str(ns("api.version"), event.API.APIVersion)
		attrs.str(ns("api.context"), event.API.APIContext)
		attrs.str(ns("api.type"), event.API.APIType)
		attrs.str(ns("api.subtype"), event.API.SubType)
		attrs.str(ns("project.id"), event.API.ProjectID)
		attrs.str(ns("organization.id"), event.API.OrganizationID)
		attrs.str(ns("environment.id"), event.API.EnvironmentID)
	}

	// Consumer identity.
	attrs.anyStr("user.id", event.Properties[dto.PropKeyAuthUserID])
	attrs.str("user.name", event.UserName)
	if event.Application != nil {
		attrs.str(ns("application.id"), event.Application.ApplicationID)
		attrs.str(ns("application.name"), event.Application.ApplicationName)
		attrs.str(ns("application.owner"), event.Application.ApplicationOwner)
		attrs.str(ns("application.key_type"), event.Application.KeyType)
	}
	if event.Subscription != nil {
		attrs.str(ns("subscription.id"), event.Subscription.BillingSubscriptionID)
		attrs.str(ns("subscription.customer.id"), event.Subscription.BillingCustomerID)
		attrs.str(ns("subscription.status"), event.Subscription.Status)
		attrs.str(ns("subscription.plan"), event.Subscription.PlanName)
	}
	if event.MetaInfo != nil {
		attrs.str(ns("correlation.id"), event.MetaInfo.CorrelationID)
		attrs.str(ns("gateway.type"), event.MetaInfo.GatewayType)
		attrs.str(ns("region.id"), event.MetaInfo.RegionID)
	}

	// Latency. OpenTelemetry has no log attribute for these.
	if event.Latencies != nil {
		attrs.i64(ns("latency.response_ms"), event.Latencies.ResponseLatency)
		attrs.i64(ns("latency.backend_ms"), event.Latencies.BackendLatency)
		attrs.i64(ns("latency.request_mediation_ms"), event.Latencies.RequestMediationLatency)
		attrs.i64(ns("latency.response_mediation_ms"), event.Latencies.ResponseMediationLatency)
		attrs.i64(ns("latency.duration_ms"), event.Latencies.Duration)
	}

	// Upstream outcome. Destination is authority+path, so only its authority
	// half is a server address.
	if event.Target != nil {
		attrs.str(ns("upstream.destination"), event.Target.Destination)
		if authority, _, found := strings.Cut(event.Target.Destination, "/"); found || authority != "" {
			if host, port, err := net.SplitHostPort(authority); err == nil {
				attrs.str("server.address", host)
				if p, convErr := strconv.Atoi(port); convErr == nil {
					attrs.i64("server.port", int64(p))
				}
			} else {
				attrs.str("server.address", authority)
			}
		}
		attrs.i64(ns("upstream.response.status_code"), int64(event.Target.TargetResponseCode))
		attrs.str(ns("upstream.response.detail"), event.Target.ResponseCodeDetail)
		attrs.b(ns("cache.hit"), event.Target.ResponseCacheHit)
	}

	// Faults. error.type is the one stable error attribute; the categories are
	// ours, derived in analytics.classifyFault from the Envoy response flags.
	attrs.str("error.type", event.ErrorType)
	attrs.str(ns("event.category"), string(event.EventCategory))
	attrs.str(ns("error.category"), string(event.FaultCategory))
	if event.Error != nil {
		attrs.i64(ns("error.code"), int64(event.Error.ErrorCode))
		attrs.str(ns("error.sub_category"), string(event.Error.ErrorMessage))
	}

	// Payloads, only present when body capture is enabled on the collector.
	attrs.anyStr(ns("request.body"), event.Properties[dto.PropKeyRequestPayload])
	attrs.anyStr(ns("response.body"), event.Properties[dto.PropKeyResponsePayload])

	o.appendAIAttributes(event, attrs, route)
	o.appendMCPAttributes(event, attrs)

	now := time.Now()
	requestTime := event.RequestTimestamp
	if requestTime.IsZero() {
		requestTime = now
	}

	body := "api.transaction"
	if event.Operation != nil && route != "" {
		body = event.Operation.APIMethod + " " + route
	}

	return &otelLogRecord{
		TimeUnixNano:         strconv.FormatInt(requestTime.UnixNano(), 10),
		ObservedTimeUnixNano: strconv.FormatInt(now.UnixNano(), 10),
		SeverityNumber:       otelSeverityNumberInfo,
		SeverityText:         otelSeverityTextInfo,
		EventName:            otelEventName,
		Body:                 otelAnyValue{StringValue: &body},
		Attributes:           attrs.list(),
	}
}

// appendAIAttributes maps the AI metadata the pipeline stashes in
// Event.Properties onto GenAI conventions. Cost has no GenAI equivalent.
func (o *OTel) appendAIAttributes(event *dto.Event, attrs *otelAttrs, route string) {
	if event.Properties == nil {
		return
	}

	if md, ok := event.Properties["aiMetadata"].(dto.AIMetadata); ok {
		attrs.str("gen_ai.provider.name", otelGenAIProviderName(md.VendorName))
		attrs.str(ns("gen_ai.provider.template_name"), md.VendorName)
		// aitoken:modelid is the response model when the provider returns one,
		// falling back to the request model.
		attrs.str("gen_ai.response.model", md.Model)
		attrs.str(ns("gen_ai.provider.api_version"), md.VendorVersion)
		switch cost := md.LLMCost.(type) {
		case float64:
			attrs.f64(ns("gen_ai.cost.total"), cost)
		case string:
			attrs.str(ns("gen_ai.cost.total"), cost)
		}
	}
	attrs.anyStr("gen_ai.request.model", event.Properties[constants.RequestModelPropertyKey])

	if usage, ok := event.Properties["aiTokenUsage"].(dto.AITokenUsage); ok {
		attrs.i64("gen_ai.usage.input_tokens", int64(usage.PromptToken))
		attrs.i64("gen_ai.usage.output_tokens", int64(usage.CompletionToken))
		attrs.i64(ns("gen_ai.usage.total_tokens"), int64(usage.TotalToken))
	}

	if op := otelGenAIOperationName(route); op != "" {
		attrs.str("gen_ai.operation.name", op)
	}

	attrs.anyBool(ns("gen_ai.egress"), event.Properties["isEgress"])
	attrs.anyBool(ns("guardrail.hit"), event.Properties[constants.GuardrailHitMetadataKey])
	attrs.anyStr(ns("guardrail.name"), event.Properties[constants.GuardrailNameMetadataKey])
}

// appendMCPAttributes flattens Properties["mcpAnalytics"] onto MCP conventions.
// The capability determines which attribute the capability name belongs on.
func (o *OTel) appendMCPAttributes(event *dto.Event, attrs *otelAttrs) {
	mcp, ok := event.Properties["mcpAnalytics"].(map[string]interface{})
	if !ok {
		return
	}

	attrs.anyStr("mcp.method.name", mcp["jsonRpcMethod"])
	attrs.anyStr("mcp.session.id", mcp["sessionId"])
	attrs.anyStr("jsonrpc.request.id", mcp["jsonRpcId"])

	// Tools and prompts are named (params.name); a resource is addressed by URI
	// (params.uri), which the analytics policy extracts into its own field.
	capabilityName, _ := mcp["capabilityName"].(string)
	switch capability, _ := mcp["capability"].(string); capability {
	case "TOOL":
		attrs.str("gen_ai.tool.name", capabilityName)
	case "RESOURCE":
		attrs.anyStr("mcp.resource.uri", mcp["resourceUri"])
	case "PROMPT":
		attrs.str("gen_ai.prompt.name", capabilityName)
	}

	// A JSON-RPC error code is a string in rpc.response.status_code.
	switch code := mcp["errorCode"].(type) {
	case int:
		attrs.str("rpc.response.status_code", strconv.Itoa(code))
	case float64:
		attrs.str("rpc.response.status_code", strconv.FormatInt(int64(code), 10))
	case string:
		attrs.str("rpc.response.status_code", code)
	}
	if isError, ok := mcp["isError"].(bool); ok && isError {
		attrs.strIfEmpty("error.type", "mcp_error")
	}

	attrs.anyStr("mcp.protocol.version", mcp["protocolVersion"])
	attrs.anyStr(ns("mcp.client.requested_protocol_version"), mcp["requestedProtocolVersion"])
	attrs.anyStr(ns("mcp.client.name"), mcp["name"])
	attrs.anyStr(ns("mcp.client.version"), mcp["version"])
}

// otelGenAIProviderName maps a WSO2 LLM provider template name onto the
// gen_ai.provider.name enum. An unrecognised provider yields "" so the enum
// attribute is left unset rather than carrying a non-member value; the raw name
// is always kept in wso2.gen_ai.provider.template_name.
func otelGenAIProviderName(templateName string) string {
	switch strings.ToLower(templateName) {
	case "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	case "awsbedrock":
		return "aws.bedrock"
	case "azure-openai":
		return "azure.ai.openai"
	case "azureai-foundry":
		return "azure.ai.inference"
	case "gemini":
		return "gcp.gemini"
	case "mistralai":
		return "mistral_ai"
	default:
		return ""
	}
}

// otelGenAIOperationName derives the gen_ai.operation.name enum value from the
// route. "" when the route is not a recognised inference endpoint.
func otelGenAIOperationName(route string) string {
	r := strings.ToLower(route)
	switch {
	case strings.Contains(r, "/chat/completions"), strings.Contains(r, "/messages"):
		return "chat"
	case strings.Contains(r, "/embeddings"):
		return "embeddings"
	case strings.Contains(r, "/generatecontent"):
		return "generate_content"
	case strings.Contains(r, "/completions"):
		return "text_completion"
	default:
		return ""
	}
}

// --- OTLP/HTTP JSON wire types -----------------------------------------------
//
// The proto3 JSON mapping of the OTLP protobufs: field names are lowerCamelCase
// and 64-bit integers are encoded as JSON strings.

type otelExportRequest struct {
	ResourceLogs []otelResourceLogs `json:"resourceLogs"`
}

type otelResourceLogs struct {
	Resource  otelResource    `json:"resource"`
	ScopeLogs []otelScopeLogs `json:"scopeLogs"`
}

type otelResource struct {
	Attributes []otelKeyValue `json:"attributes"`
}

type otelScopeLogs struct {
	Scope      otelScope       `json:"scope"`
	LogRecords []otelLogRecord `json:"logRecords"`
}

type otelScope struct {
	Name string `json:"name"`
}

type otelLogRecord struct {
	TimeUnixNano         string         `json:"timeUnixNano"`
	ObservedTimeUnixNano string         `json:"observedTimeUnixNano"`
	SeverityNumber       int            `json:"severityNumber"`
	SeverityText         string         `json:"severityText"`
	EventName            string         `json:"eventName,omitempty"`
	Body                 otelAnyValue   `json:"body"`
	Attributes           []otelKeyValue `json:"attributes"`
}

// otelExportResponse is the OTLP ExportLogsServiceResponse. A 2xx can still
// report records the endpoint refused, which is otherwise indistinguishable from
// a clean export.
type otelExportResponse struct {
	PartialSuccess otelPartialSuccess `json:"partialSuccess"`
}

type otelPartialSuccess struct {
	// json.Number because proto3 JSON encodes int64 as a string while some
	// receivers emit a bare number; either must parse.
	RejectedLogRecords json.Number `json:"rejectedLogRecords"`
	ErrorMessage       string      `json:"errorMessage"`
}

type otelKeyValue struct {
	Key   string       `json:"key"`
	Value otelAnyValue `json:"value"`
}

// otelAnyValue is the OTLP AnyValue union; exactly one field is set.
type otelAnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

// otelAttrs accumulates attributes, skipping empty ones so a record carries only
// what the event populated.
type otelAttrs struct {
	kvs []otelKeyValue
}

func newOTelAttrs() *otelAttrs {
	return &otelAttrs{kvs: make([]otelKeyValue, 0, 48)}
}

func (a *otelAttrs) str(key, value string) *otelAttrs {
	if value == "" {
		return a
	}
	v := value
	a.kvs = append(a.kvs, otelKeyValue{Key: key, Value: otelAnyValue{StringValue: &v}})
	return a
}

// strIfEmpty sets key only when it is not already present.
func (a *otelAttrs) strIfEmpty(key, value string) *otelAttrs {
	for _, kv := range a.kvs {
		if kv.Key == key {
			return a
		}
	}
	return a.str(key, value)
}

func (a *otelAttrs) i64(key string, value int64) *otelAttrs {
	if value == 0 {
		return a
	}
	v := strconv.FormatInt(value, 10)
	a.kvs = append(a.kvs, otelKeyValue{Key: key, Value: otelAnyValue{IntValue: &v}})
	return a
}

func (a *otelAttrs) f64(key string, value float64) *otelAttrs {
	if value == 0 {
		return a
	}
	v := value
	a.kvs = append(a.kvs, otelKeyValue{Key: key, Value: otelAnyValue{DoubleValue: &v}})
	return a
}

func (a *otelAttrs) b(key string, value bool) *otelAttrs {
	if !value {
		return a
	}
	v := value
	a.kvs = append(a.kvs, otelKeyValue{Key: key, Value: otelAnyValue{BoolValue: &v}})
	return a
}

// anyStr / anyInt / anyBool accept the interface{} values held in
// Event.Properties, whose concrete types vary by producer.
func (a *otelAttrs) anyStr(key string, value interface{}) *otelAttrs {
	if s, ok := value.(string); ok {
		return a.str(key, s)
	}
	return a
}

func (a *otelAttrs) anyInt(key string, value interface{}) *otelAttrs {
	switch v := value.(type) {
	case int:
		return a.i64(key, int64(v))
	case int32:
		return a.i64(key, int64(v))
	case int64:
		return a.i64(key, v)
	case uint32:
		return a.i64(key, int64(v))
	case uint64:
		return a.i64(key, int64(v))
	case float64:
		return a.i64(key, int64(v))
	}
	return a
}

func (a *otelAttrs) anyBool(key string, value interface{}) *otelAttrs {
	switch v := value.(type) {
	case bool:
		return a.b(key, v)
	case string:
		if parsed, err := strconv.ParseBool(v); err == nil {
			return a.b(key, parsed)
		}
	}
	return a
}

func (a *otelAttrs) list() []otelKeyValue {
	return a.kvs
}
