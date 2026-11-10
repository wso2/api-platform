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

package platformgateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// Scenario values a trace publishes for "${CTX:...}" references.
const (
	keyTraceID     = "traceId"
	keyTraceparent = "traceparent"
)

// spanWait bounds a wait for spans. The router batches its spans for seconds before exporting.
const spanWait = 30 * time.Second

// collectedSpan is a span as the otel-collector component reports it.
type collectedSpan struct {
	TraceID      string         `json:"traceId"`
	SpanID       string         `json:"spanId"`
	ParentSpanID string         `json:"parentSpanId"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Service      string         `json:"service"`
	Attributes   map[string]any `json:"attributes"`
}

// registerTracingSteps wires the steps that read a scenario's trace back from the block's
// otel-collector. A scenario starts its own trace and sends its traceparent, so every assertion
// reads only the spans of its own requests, whatever other runners export at the same time.
func (g *Gateway) registerTracingSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I start a new trace$`, startTrace)
	sc.Step(`^the trace should have (at least )?(\d+) spans? named "([^"]*)"$`, g.traceSpanCount)
	sc.Step(`^the span "([^"]*)" should be a "([A-Z]+)" span$`, g.traceSpanKind)
	sc.Step(`^the span "([^"]*)" should be a child of (the span|a span from service) "([^"]*)"$`, g.traceSpanParent)
	sc.Step(`^the span "([^"]*)" should (have|not have) attribute "([^"]*)"(?: with value "([^"]*)")?$`,
		g.traceSpanAttribute)
	sc.Step(`^the span "([^"]*)" attribute "([^"]*)" should (contain|be greater than) "([^"]*)"$`,
		g.traceSpanAttributeCompare)
	sc.Step(`^a span with attribute "([^"]*)" with value "([^"]*)" should have attribute "([^"]*)" with value "([^"]*)"$`,
		g.traceSpanWithAttribute)
	sc.Step(`^no span in the trace should have attribute "([^"]*)"$`, g.traceNoSpanAttribute)
	sc.Step(`^the otel-collector should have an analytics log record for the trace$`, g.traceAnalyticsLogRecord)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with a chat body of (\d+) bytes$`, g.sendChatOfSize)
}

// startTrace publishes a fresh, sampled W3C trace context as ${CTX:traceparent}, and its trace
// id as ${CTX:traceId}, for the scenario's requests to carry.
func startTrace(ctx context.Context) error {
	traceID, spanID := make([]byte, 16), make([]byte, 8)
	if _, err := rand.Read(traceID); err != nil {
		return err
	}
	if _, err := rand.Read(spanID); err != nil {
		return err
	}
	id := hex.EncodeToString(traceID)
	if err := tcontext.Set(ctx, keyTraceID, id); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyTraceparent, "00-"+id+"-"+hex.EncodeToString(spanID)+"-01")
}

func scenarioTraceID(ctx context.Context) (string, error) {
	if v, ok := tcontext.Get(ctx, keyTraceID); ok {
		if id, ok := v.(string); ok && id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("the scenario has no trace: start one with \"I start a new trace\"")
}

// collectorRecords reads the records of the scenario's trace from one otel-collector listing.
func (g *Gateway) collectorRecords(ctx context.Context, listing string, out any) error {
	id, err := scenarioTraceID(ctx)
	if err != nil {
		return err
	}
	url, err := g.serviceURL(ctx, "otel-collector", listing+"?traceId="+id)
	if err != nil {
		return err
	}
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{Method: http.MethodGet, URL: url}, 0, 0)
	if err != nil {
		return retry.Transient(err)
	}
	if !resp.Succeeded() {
		return retry.Transient(fmt.Errorf("the otel-collector %s returned %s", listing, resp.Describe()))
	}
	return json.Unmarshal(resp.Body, out)
}

func (g *Gateway) traceSpans(ctx context.Context) ([]collectedSpan, error) {
	var spans []collectedSpan
	return spans, g.collectorRecords(ctx, "/test/spans", &spans)
}

// awaitSpans polls the scenario's trace until accept holds, and fails naming what was awaited
// and the spans that did arrive.
func (g *Gateway) awaitSpans(ctx context.Context, what string, accept func([]collectedSpan) bool) error {
	spans, err := retry.Until(ctx, retry.Options{Timeout: spanWait, Interval: 500 * time.Millisecond, Fast: true},
		g.traceSpans, accept)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if !accept(spans) {
		return fmt.Errorf("%s: not seen within %s (spans: %s)", what, spanWait, describeSpans(spans))
	}
	return nil
}

func expandAll(ctx context.Context, values ...*string) error {
	for _, v := range values {
		resolved, err := stepscommon.Expand(ctx, *v)
		if err != nil {
			return err
		}
		*v = resolved
	}
	return nil
}

func (g *Gateway) traceSpanCount(ctx context.Context, atLeast string, want int, name string) error {
	if err := expandAll(ctx, &name); err != nil {
		return err
	}
	return g.awaitSpans(ctx, fmt.Sprintf("%s%d spans named %q", atLeast, want, name), func(spans []collectedSpan) bool {
		n := len(spansNamed(spans, name))
		return n == want || (atLeast != "" && n > want)
	})
}

func (g *Gateway) traceSpanKind(ctx context.Context, name, kind string) error {
	if err := expandAll(ctx, &name); err != nil {
		return err
	}
	return g.awaitSpans(ctx, fmt.Sprintf("a %s span %q", kind, name), func(spans []collectedSpan) bool {
		for _, s := range spansNamed(spans, name) {
			if s.Kind == kind {
				return true
			}
		}
		return false
	})
}

func (g *Gateway) traceSpanParent(ctx context.Context, name, relation, parent string) error {
	if err := expandAll(ctx, &name, &parent); err != nil {
		return err
	}
	byService := relation == "a span from service"
	return g.awaitSpans(ctx, fmt.Sprintf("span %q as a child of %s %q", name, relation, parent),
		func(spans []collectedSpan) bool { return hasChildOf(spans, name, parent, byService) })
}

// hasChildOf reports whether a span named child has, in the same trace, a parent span named
// parent, or from the service named parent when byService is set.
func hasChildOf(spans []collectedSpan, child, parent string, byService bool) bool {
	for _, c := range spansNamed(spans, child) {
		for _, p := range spans {
			if p.SpanID != c.ParentSpanID {
				continue
			}
			if (byService && p.Service == parent) || (!byService && p.Name == parent) {
				return true
			}
		}
	}
	return false
}

func (g *Gateway) traceSpanAttribute(ctx context.Context, name, have, key, value string) error {
	if err := expandAll(ctx, &name, &key, &value); err != nil {
		return err
	}
	matches := func(s collectedSpan) bool {
		v, ok := s.Attributes[key]
		return ok && (value == "" || attrText(v) == value)
	}
	if have == "have" {
		return g.awaitSpans(ctx, fmt.Sprintf("span %q with attribute %q %q", name, key, value),
			func(spans []collectedSpan) bool { return slices.ContainsFunc(spansNamed(spans, name), matches) })
	}
	// An absence is asserted once the span has arrived, on every span of that name.
	var named []collectedSpan
	if err := g.awaitSpans(ctx, fmt.Sprintf("span %q", name), func(spans []collectedSpan) bool {
		named = spansNamed(spans, name)
		return len(named) > 0
	}); err != nil {
		return err
	}
	for _, s := range named {
		if matches(s) {
			return fmt.Errorf("span %q must not have attribute %q, has %v", name, key, s.Attributes[key])
		}
	}
	return nil
}

func (g *Gateway) traceSpanAttributeCompare(ctx context.Context, name, key, op, value string) error {
	if err := expandAll(ctx, &name, &key, &value); err != nil {
		return err
	}
	var matches func(any) bool
	switch op {
	case "contain":
		matches = func(v any) bool { return strings.Contains(attrText(v), value) }
	default:
		limit, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("%q is not a number: %w", value, err)
		}
		matches = func(v any) bool { n, ok := v.(float64); return ok && n > limit }
	}
	return g.awaitSpans(ctx, fmt.Sprintf("span %q whose attribute %q should %s %q", name, key, op, value),
		func(spans []collectedSpan) bool {
			return slices.ContainsFunc(spansNamed(spans, name), func(s collectedSpan) bool {
				v, ok := s.Attributes[key]
				return ok && matches(v)
			})
		})
}

// traceSpanWithAttribute asserts on a span found by an attribute rather than its name, for a
// call whose span name depends on what the gateway could read of the request.
func (g *Gateway) traceSpanWithAttribute(ctx context.Context, key, value, otherKey, otherValue string) error {
	if err := expandAll(ctx, &key, &value, &otherKey, &otherValue); err != nil {
		return err
	}
	return g.awaitSpans(ctx, fmt.Sprintf("a span with %s %q and %s %q", key, value, otherKey, otherValue),
		func(spans []collectedSpan) bool {
			return slices.ContainsFunc(spans, func(s collectedSpan) bool {
				return attrText(s.Attributes[key]) == value && attrText(s.Attributes[otherKey]) == otherValue
			})
		})
}

// traceNoSpanAttribute reads the trace once, so it belongs after a step that waited for the
// trace's spans to arrive.
func (g *Gateway) traceNoSpanAttribute(ctx context.Context, key string) error {
	if err := expandAll(ctx, &key); err != nil {
		return err
	}
	spans, err := g.traceSpans(ctx)
	if err != nil {
		return err
	}
	for _, s := range spans {
		if v, ok := s.Attributes[key]; ok {
			return fmt.Errorf("span %q has attribute %q = %v", s.Name, key, v)
		}
	}
	return nil
}

// traceAnalyticsLogRecord waits for an analytics log record joined to the scenario's trace by
// the record's own trace id.
func (g *Gateway) traceAnalyticsLogRecord(ctx context.Context) error {
	records, err := retry.Until(ctx, retry.Options{Timeout: spanWait, Interval: 500 * time.Millisecond, Fast: true},
		func(ctx context.Context) ([]json.RawMessage, error) {
			var records []json.RawMessage
			return records, g.collectorRecords(ctx, "/test/logs", &records)
		}, func(records []json.RawMessage) bool { return len(records) > 0 })
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("no analytics log record carried the trace within %s", spanWait)
	}
	return nil
}

// sendChatOfSize sends a chat completion whose prompt pads the body to size bytes.
func (g *Gateway) sendChatOfSize(ctx context.Context, method, path string, size int) error {
	if err := expandAll(ctx, &path); err != nil {
		return err
	}
	url, err := g.gatewayURL(path)
	if err != nil {
		return err
	}
	body, err := chatBodyOfSize(size)
	if err != nil {
		return err
	}
	headers := mergeCanonicalHeaders(map[string]string{"Content-Type": "application/json"}, g.scenarioHeaders(ctx))
	return g.invokeWith(ctx, method, url, headers, body)
}

func chatBodyOfSize(size int) ([]byte, error) {
	const prefix, suffix = `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"`, `"}]}`
	padding := size - len(prefix) - len(suffix)
	if padding < 0 {
		return nil, fmt.Errorf("a chat body needs at least %d bytes, %d requested", len(prefix)+len(suffix), size)
	}
	return append(append([]byte(prefix), bytes.Repeat([]byte("a"), padding)...), suffix...), nil
}

func spansNamed(spans []collectedSpan, name string) []collectedSpan {
	var out []collectedSpan
	for _, s := range spans {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

// attrText renders an attribute value the way a feature spells it: integers without a decimal
// point, lists as JSON.
func attrText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		encoded, _ := json.Marshal(x)
		return string(encoded)
	}
	return fmt.Sprint(v)
}

func describeSpans(spans []collectedSpan) string {
	parts := make([]string, 0, len(spans))
	for _, s := range spans {
		parts = append(parts, fmt.Sprintf("%s[%s] %v", s.Name, s.Service, s.Attributes))
	}
	return strings.Join(parts, "; ")
}
