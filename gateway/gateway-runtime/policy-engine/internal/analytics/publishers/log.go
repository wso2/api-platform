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
// (when send_request_body/send_response_body are enabled) payloads attached by
// the analytics engine, so this publisher only serializes it.
type Log struct {
	// maskedHeaders holds lower-cased header names whose values are redacted in
	// the requestHeaders/responseHeaders properties before logging.
	maskedHeaders map[string]bool
	// maxPayloadSize caps the number of request/response payload bytes written to
	// the log line (0 = no limit). Truncation is output-side only.
	maxPayloadSize int
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
		maskedHeaders:  masked,
		maxPayloadSize: logCfg.MaxPayloadSize,
		out:            os.Stdout,
	}
}

// Publish writes the event to stdout as JSON. Traffic logging is per-API: only
// events carrying a traffic-log directive (stamped by the log-message policy in
// access-log mode on APIs that opted in) are emitted; all others are skipped.
func (l *Log) Publish(event *dto.Event) {
	if event == nil || event.TrafficLog == nil {
		return
	}

	dir := event.TrafficLog
	tl := l.toTrafficLogEvent(event, dir)

	data, err := json.Marshal(tl)
	if err != nil {
		slog.Error("Failed to marshal traffic-log event", "error", err)
		return
	}

	if fields := dir.Fields; fields != nil && (len(fields.Only) > 0 || len(fields.Exclude) > 0) {
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

// applyFieldsProjection mutates m in place to restrict it to the configured
// fields. Names are top-level keys (e.g. "latencies", "requestHeaders") or
// dotted sub-key paths within map fields (e.g. "requestHeaders.authorization",
// "labels.env"). Only keeps exactly the named fields; Exclude drops the named
// fields and keeps everything else. If both are set, Only takes precedence.
// Top-level values are kept as raw JSON bytes; only the specific nested
// objects referenced by a dotted path are decoded and re-encoded.
func applyFieldsProjection(m map[string]json.RawMessage, fields *dto.TrafficLogFields) {
	if len(fields.Only) > 0 {
		directKeys := make(map[string]bool)
		subKeys := make(map[string][]string) // topKey → sub-keys to keep
		for _, name := range fields.Only {
			if top, sub, found := strings.Cut(name, "."); found {
				subKeys[top] = append(subKeys[top], sub)
			} else {
				directKeys[name] = true
			}
		}
		for key := range m {
			if !directKeys[key] && subKeys[key] == nil {
				delete(m, key)
			}
		}
		for top, subs := range subKeys {
			if directKeys[top] {
				continue // whole key kept; don't filter sub-keys
			}
			keep := make(map[string]bool, len(subs))
			for _, s := range subs {
				keep[s] = true
			}
			filterNestedKeys(m, top, func(k string) bool { return keep[k] })
		}
		return
	}
	for _, name := range fields.Exclude {
		if top, sub, found := strings.Cut(name, "."); found {
			filterNestedKeys(m, top, func(k string) bool { return k != sub })
		} else {
			delete(m, name)
		}
	}
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
// Returns a new map; the input is not modified. To drop a header entirely rather
// than redacting its value, use the per-flow excludeHeaders directive (see
// dropHeaders) or a dotted fields.exclude path (e.g. "requestHeaders.authorization").
func (l *Log) maskHeaders(headers map[string]string, mask map[string]bool) map[string]string {
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

// dropHeaders deletes, in place, any header whose name (case-insensitive) appears
// in exclude. It is a no-op when exclude is empty. Unlike masking (which redacts
// the value) this removes the key entirely, matching the log-message policy's
// per-flow excludeHeaders semantics. Callers pass the freshly-built, non-shared
// map returned by maskHeaders, so in-place mutation never affects other publishers.
func dropHeaders(headers map[string]string, exclude []string) {
	if len(exclude) == 0 {
		return
	}
	drop := make(map[string]bool, len(exclude))
	for _, h := range exclude {
		drop[strings.ToLower(strings.TrimSpace(h))] = true
	}
	for name := range headers {
		if drop[strings.ToLower(name)] {
			delete(headers, name)
		}
	}
}
