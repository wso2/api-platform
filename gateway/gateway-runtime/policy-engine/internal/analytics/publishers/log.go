/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// maskedHeaderValue is the placeholder written in place of a masked header value.
const maskedHeaderValue = "****"

// Log is an analytics publisher that writes each enriched analytics event to
// stdout as a single JSON line. It is intended for log-scraping pipelines
// (Fluent Bit, Loki, ELK, etc.) and as a lightweight alternative to a SaaS
// analytics backend. The event already carries the rich metadata, headers and
// (when request_body/response_body are enabled) payloads attached by
// the analytics engine, so this publisher only serializes it.
type Log struct {
	// maskedHeaders holds lower-cased header names whose values are redacted in
	// the requestHeaders/responseHeaders properties before logging.
	maskedHeaders map[string]bool
	// maxPayloadSize caps the number of request/response payload bytes written to
	// the log line (0 = no limit). Truncation is output-side only.
	maxPayloadSize int
	// globalDir is the directive built from [traffic_logging] config, used for
	// every request. Nil when traffic logging is disabled, in which case Publish
	// is a no-op. Its Properties field is always left nil: properties are
	// request-time values (see globalProperties) and are attached to a
	// per-request copy of this directive in resolveGlobalDirective, never
	// mutated in place here.
	globalDir *dto.TrafficLogDirective
	// globalProperties resolves traffic_logging.properties per request. Nil (via
	// a zero-value evaluator) is never stored; buildGlobalPropertyEvaluator
	// always returns a usable, possibly-empty evaluator whose resolve() returns
	// nil when nothing is configured.
	globalProperties *globalPropertyEvaluator
	// mu serializes writes to stdout so concurrent ALS streams do not interleave.
	mu sync.Mutex
	// out is the destination writer; defaults to os.Stdout (overridable in tests).
	out *os.File
}

// NewLog creates a new stdout traffic-logging publisher.
func NewLog(logCfg *config.TrafficLoggingConfig) *Log {
	if logCfg == nil {
		logCfg = &config.TrafficLoggingConfig{}
	}

	masked := make(map[string]bool, len(logCfg.MaskedHeaders))
	for _, h := range logCfg.MaskedHeaders {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			masked[h] = true
		}
	}

	return &Log{
		maskedHeaders:    masked,
		maxPayloadSize:   logCfg.MaxPayloadSize,
		globalDir:        buildGlobalDirective(*logCfg),
		globalProperties: newGlobalPropertyEvaluator(logCfg.Properties, masked),
		out:              os.Stdout,
	}
}

// buildGlobalDirective converts the traffic-logging config into a
// TrafficLogDirective used by every Publish call. Returns nil when traffic
// logging is disabled.
func buildGlobalDirective(cfg config.TrafficLoggingConfig) *dto.TrafficLogDirective {
	if !cfg.Enabled {
		return nil
	}

	dir := &dto.TrafficLogDirective{
		Request: &dto.TrafficLogFlow{
			Headers: cfg.RequestHeaders,
			Payload: cfg.RequestBody,
		},
		Response: &dto.TrafficLogFlow{
			Headers: cfg.ResponseHeaders,
			Payload: cfg.ResponseBody,
		},
	}

	if len(cfg.ExcludeFields) > 0 {
		dir.Fields = &dto.TrafficLogFields{Exclude: cfg.ExcludeFields}
	}

	return dir
}

// resolveGlobalDirective returns the directive to use for a request
// (l.globalDir is guaranteed non-nil by the caller). When global properties are
// configured, it returns a shallow copy of l.globalDir carrying this request's
// resolved Properties, so concurrent requests never race on a shared, mutated
// globalDir.Properties field. The Request/Response/Fields pointers are shared
// read-only state and safe to alias across the copy.
func (l *Log) resolveGlobalDirective(event *dto.Event) *dto.TrafficLogDirective {
	resolved := l.globalProperties.resolve(event)
	if len(resolved) == 0 {
		return l.globalDir
	}
	dirCopy := *l.globalDir
	dirCopy.Properties = resolved
	return &dirCopy
}

// Publish writes the event to stdout as JSON, using the directive built from
// [traffic_logging] config. If traffic logging is disabled, the event is skipped.
func (l *Log) Publish(event *dto.Event) {
	if event == nil || l.globalDir == nil {
		return
	}

	dir := l.resolveGlobalDirective(event)
	tl := l.toTrafficLogEvent(event, dir)

	data, err := json.Marshal(tl)
	if err != nil {
		slog.Error("Failed to marshal traffic-log event", "error", err)
		return
	}

	if fields := dir.Fields; fields != nil && len(fields.Exclude) > 0 {
		// Shallow-decode only the top level; untouched fields stay as raw JSON
		// bytes and are never deep-decoded or re-encoded.
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			slog.Error("Failed to unmarshal for field projection; emitting as-is", "error", err)
		} else {
			applyFieldsProjection(m, fields)
			if projected, merr := json.Marshal(m); merr == nil {
				data = projected
			} else {
				slog.Error("Failed to remarshal after field projection; emitting as-is", "error", merr)
			}
		}
	}

	l.write(data)
}

func (l *Log) write(data []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := fmt.Fprintln(l.out, string(data)); err != nil {
		slog.Error("Failed to write analytics event to stdout", "error", err)
	}
}

// parseHeadersFromString converts the JSON-encoded header value stored in
// event.Properties (a map[string]string or map[string][]string serialized by the
// ext_proc layer) into a map[string]string so it embeds as a plain JSON object
// in the log line. Other publishers (e.g. Moesif) read the raw string directly;
// the Log publisher calls this only on the local TrafficLogEvent it builds, so
// the shared event is never modified. Multi-value headers are flattened to their
// first value. Returns nil on empty input or parse failure.
func parseHeadersFromString(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var single map[string]string
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		return single
	}
	// Fallback: multi-value wire format — flatten to first value.
	var multi map[string][]string
	if err := json.Unmarshal([]byte(raw), &multi); err == nil {
		out := make(map[string]string, len(multi))
		for k, vs := range multi {
			if len(vs) > 0 {
				out[k] = vs[0]
			}
		}
		return out
	}
	return nil
}

// truncatePayload returns up to maxPayloadSize bytes of the payload (0 = no
// limit). Truncation is on a byte boundary, matching the previous capture-time
// behavior.
func (l *Log) truncatePayload(s string) string {
	if l.maxPayloadSize <= 0 || len(s) <= l.maxPayloadSize {
		return s
	}
	return s[:l.maxPayloadSize]
}

// applyFieldsProjection mutates m in place, dropping the configured fields and
// keeping everything else. Names are top-level keys (e.g. "latencies",
// "requestHeaders") or dotted paths of arbitrary depth into nested JSON objects
// (e.g. "requestHeaders.authorization", "properties.claims.internal_debug" —
// the latter reaching into a nested object produced by a traffic_logging.properties
// $ctx: expression that evaluates to a CEL map wholesale, e.g.
// `claims = "$ctx:auth.property"`, rather than one flattened property per claim).
// A path segment that doesn't correspond to a JSON object at that point (e.g. one
// dot too many into a string leaf like requestBody, or a header value, which is
// always a string) is a no-op for that entry rather than an error. Sub-keys
// immediately under requestHeaders/responseHeaders are matched case-insensitively
// (consistent with maskedHeaders and HTTP header-name semantics) since the
// upstream may return a header in any casing (e.g. "Set-Cookie"); every other
// path segment, at any depth, matches case-sensitively.
func applyFieldsProjection(m map[string]json.RawMessage, fields *dto.TrafficLogFields) {
	for _, name := range fields.Exclude {
		deleteNestedPath(m, strings.Split(name, "."))
	}
}

// deleteNestedPath removes the key at the dotted path parts from m, recursively
// decoding and re-encoding each intermediate level. A parent level is deleted
// entirely once its last child is removed, so an emptied nested object doesn't
// linger in the output as "{}".
func deleteNestedPath(m map[string]json.RawMessage, parts []string) {
	top := parts[0]
	if len(parts) == 1 {
		delete(m, top)
		return
	}

	// Case-insensitive header-name matching only applies one level below
	// requestHeaders/responseHeaders. A deeper path under a header (e.g.
	// "requestHeaders.foo.bar") falls through to the generic path below, where
	// the json.Unmarshal of a header's string value into an object fails and
	// the entry is silently a no-op — the same graceful behavior as any other
	// path that reaches into a non-object leaf.
	if len(parts) == 2 && isHeaderField(top) {
		filterNestedKeys(m, top, func(k string) bool { return !strings.EqualFold(k, parts[1]) })
		return
	}

	raw, ok := m[top]
	if !ok {
		return
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return // not a JSON object at this level -- no-op, not an error
	}
	deleteNestedPath(nested, parts[1:])
	if len(nested) == 0 {
		delete(m, top)
		return
	}
	filtered, err := json.Marshal(nested)
	if err != nil {
		return
	}
	m[top] = filtered
}

// isHeaderField reports whether top is one of the header map fields, whose
// immediate sub-keys are matched case-insensitively.
func isHeaderField(top string) bool {
	return top == "requestHeaders" || top == "responseHeaders"
}

// filterNestedKeys decodes the JSON object stored at m[top], keeps only the
// sub-keys for which keep returns true, and re-encodes the result back into
// m[top]. Deletes m[top] entirely if no sub-keys survive, or if m[top] is
// absent or not a JSON object.
func filterNestedKeys(m map[string]json.RawMessage, top string, keep func(string) bool) {
	raw, ok := m[top]
	if !ok {
		return
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return
	}
	for k := range nested {
		if !keep(k) {
			delete(nested, k)
		}
	}
	if len(nested) == 0 {
		delete(m, top)
		return
	}
	filtered, err := json.Marshal(nested)
	if err != nil {
		return
	}
	m[top] = filtered
}

// maskHeaders redacts header values whose names appear in mask (case-insensitive).
// Returns a new map; the input is not modified. A dotted fields.exclude path
// (e.g. "requestHeaders.Authorization") can drop a header entirely instead of
// redacting it; like mask, that comparison is also case-insensitive (see
// isHeaderField, used by deleteNestedPath), so any casing Envoy delivers matches.
func maskHeaders(headers map[string]string, mask map[string]bool) map[string]string {
	result := make(map[string]string, len(headers))
	for name, value := range headers {
		if mask[strings.ToLower(name)] {
			result[name] = maskedHeaderValue
		} else {
			result[name] = value
		}
	}
	return result
}
