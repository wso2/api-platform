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
	"maps"
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

	out := l.shapeEvent(event)

	data, err := json.Marshal(out)
	if err != nil {
		slog.Error("Failed to marshal analytics event for log publisher", "error", err)
		return
	}

	// When an explicit field selection is configured it is authoritative over which
	// fields appear: project the serialized record down to (or removing) the named
	// top-level keys and properties.* paths.
	if fields := event.TrafficLog.Fields; fields != nil && len(fields.Names) > 0 {
		if projected, perr := applyFieldsProjection(data, fields); perr != nil {
			slog.Error("Failed to project traffic-log fields; emitting unprojected line", "error", perr)
		} else {
			data = projected
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := fmt.Fprintln(l.out, string(data)); err != nil {
		slog.Error("Failed to write analytics event to stdout", "error", err)
	}
}

// shapeEvent returns the event to serialize, applying the per-API traffic-log
// directive to a shallow copy with a cloned Properties map so the shared event
// observed by other publishers is left untouched. For each flow it drops the
// headers/payload the API did not request, removes per-flow excluded headers, and
// redacts globally masked headers. ALS-derived fields (latencies, status, timing)
// are always retained.
func (l *Log) shapeEvent(event *dto.Event) *dto.Event {
	dir := event.TrafficLog
	hasCustom := dir != nil && len(dir.Properties) > 0
	if event.Properties == nil && !hasCustom {
		return event
	}

	props := make(map[string]interface{}, len(event.Properties)+1)
	maps.Copy(props, event.Properties)

	// When an explicit field selection is set it is authoritative over presence, so
	// the per-flow headers/payload booleans are not used for gating here; only header
	// masking + per-flow excludeHeaders are applied, and the projection (in Publish)
	// decides which fields survive.
	gate := dir.Fields == nil || len(dir.Fields.Names) == 0
	l.applyFlow(props, dir.Request, dto.PropKeyRequestHeaders, dto.PropKeyRequestPayload, gate)
	l.applyFlow(props, dir.Response, dto.PropKeyResponseHeaders, dto.PropKeyResponsePayload, gate)

	// Attach the policy's resolved custom properties under a dedicated namespace, so
	// they never collide with reserved keys and are projectable as "properties.custom".
	if hasCustom {
		props["custom"] = dir.Properties
	}

	cp := *event
	cp.Properties = props
	return &cp
}

// applyFlow enforces one flow's presentation rules on the cloned properties.
// Header content, when present, is always cleaned (per-flow excludeHeaders + global
// masking). When gate is true (no authoritative field selection), a nil flow or a
// disabled headers/payload field also drops the corresponding property entirely.
func (l *Log) applyFlow(props map[string]interface{}, flow *dto.TrafficLogFlow, headersKey, payloadKey string, gate bool) {
	var exclude []string
	if flow != nil {
		exclude = flow.ExcludeHeaders
	}
	if raw, ok := props[headersKey].(string); ok {
		props[headersKey] = l.filterHeaders(raw, exclude)
	}
	if raw, ok := props[payloadKey].(string); ok {
		props[payloadKey] = l.truncatePayload(raw)
	}

	if gate {
		if flow == nil || !flow.Headers {
			delete(props, headersKey)
		}
		if flow == nil || !flow.Payload {
			delete(props, payloadKey)
		}
	}
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

// applyFieldsProjection restricts the serialized event JSON to the configured
// fields. Names are top-level keys (e.g. "latencies") or dotted property paths
// (e.g. "properties.requestHeaders"). Mode "exclude" drops the named keys; any
// other value (default "include") keeps only the named keys. Naming the whole
// "properties" key keeps all of its subkeys.
func applyFieldsProjection(data []byte, fields *dto.TrafficLogFields) ([]byte, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	topNames := make(map[string]bool)
	propNames := make(map[string]bool)
	propsReferenced := false
	for _, name := range fields.Names {
		if sub, ok := strings.CutPrefix(name, "properties."); ok {
			if sub != "" {
				propNames[sub] = true
				propsReferenced = true
			}
			continue
		}
		if name == "properties" {
			propsReferenced = true
		}
		if name != "" {
			topNames[name] = true
		}
	}

	props, _ := m["properties"].(map[string]interface{})

	if strings.EqualFold(fields.Mode, "exclude") {
		for name := range topNames {
			delete(m, name)
		}
		for sub := range propNames {
			delete(props, sub)
		}
		if props != nil && len(props) == 0 {
			delete(m, "properties")
		}
		return json.Marshal(m)
	}

	// include (default): keep only referenced top-level keys.
	for key := range m {
		keep := topNames[key] || (key == "properties" && propsReferenced)
		if !keep {
			delete(m, key)
		}
	}
	// When specific property subkeys were named (and not the whole "properties"),
	// keep only those subkeys.
	if props != nil && len(propNames) > 0 && !topNames["properties"] {
		for sub := range props {
			if !propNames[sub] {
				delete(props, sub)
			}
		}
		if len(props) == 0 {
			delete(m, "properties")
		}
	}
	return json.Marshal(m)
}

// filterHeaders parses a JSON header map, drops any per-flow excluded headers,
// and redacts the values of globally masked headers (both case-insensitive). The
// raw string is returned unchanged if it is empty or not valid JSON.
func (l *Log) filterHeaders(raw string, excludeHeaders []string) string {
	if raw == "" {
		return raw
	}
	var headers map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return raw
	}

	excluded := make(map[string]bool, len(excludeHeaders))
	for _, h := range excludeHeaders {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			excluded[h] = true
		}
	}

	for name := range headers {
		lower := strings.ToLower(name)
		if excluded[lower] {
			delete(headers, name)
			continue
		}
		if l.maskedHeaders[lower] {
			headers[name] = maskedHeaderValue
		}
	}

	out, err := json.Marshal(headers)
	if err != nil {
		return raw
	}
	return string(out)
}
