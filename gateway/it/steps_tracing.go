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

package it

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// mockOTLPCollectorURL is the test API of the mock OTLP collector in
// docker-compose.test.tracing.override.yaml.
const mockOTLPCollectorURL = "http://localhost:8089"

// spanWait bounds how long a step waits for spans to arrive. Envoy batches its spans
// for several seconds before exporting them.
const spanWait = 30 * time.Second

// collectedSpan is a span as the mock collector reports it.
type collectedSpan struct {
	TraceID      string         `json:"traceId"`
	SpanID       string         `json:"spanId"`
	ParentSpanID string         `json:"parentSpanId"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Service      string         `json:"service"`
	StatusCode   string         `json:"statusCode"`
	Attributes   map[string]any `json:"attributes"`
	Links        []struct {
		TraceID string `json:"traceId"`
		SpanID  string `json:"spanId"`
	} `json:"links"`
}

// TracingSteps asserts on the spans the gateway exports. A scenario either starts its
// own trace, sending a traceparent with its requests, or adopts the trace of the first
// span it waits for.
type TracingSteps struct {
	client  *http.Client
	http    *steps.HTTPSteps
	traceID string
	spans   []collectedSpan
	// matched is the span the last attribute wait found, for a span whose name is not
	// known in advance.
	matched *collectedSpan
}

// RegisterTracingSteps registers the tracing step definitions.
func RegisterTracingSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	t := &TracingSteps{client: state.HTTPClient, http: httpSteps}
	ctx.Before(func(c context.Context, _ *godog.Scenario) (context.Context, error) {
		t.traceID, t.spans, t.matched = "", nil, nil
		return c, nil
	})

	ctx.Step(`^I reset the trace collector$`, t.resetCollector)
	ctx.Step(`^I send the following requests in a new trace$`, t.startTrace)
	ctx.Step(`^I wait for the span "([^"]*)"$`, t.waitForSpan)
	ctx.Step(`^the trace should be the one the requests sent$`, t.traceIsTheCallers)
	ctx.Step(`^the span "([^"]*)" should be a child of the span "([^"]*)"$`, t.childOf)
	ctx.Step(`^the span "([^"]*)" should be a child of a span from service "([^"]*)"$`, t.childOfService)
	ctx.Step(`^the span "([^"]*)" should be a "([A-Z]+)" span$`, t.hasKind)
	ctx.Step(`^the span "([^"]*)" should have status "([A-Z]+)"$`, t.hasStatus)
	ctx.Step(`^the span "([^"]*)" should have attribute "([^"]*)" with value "([^"]*)"$`, t.hasAttribute)
	ctx.Step(`^the span "([^"]*)" should have attribute "([^"]*)"$`, t.hasAttributeKey)
	ctx.Step(`^the span "([^"]*)" should not have attribute "([^"]*)"$`, t.lacksAttribute)
	ctx.Step(`^the span "([^"]*)" attribute "([^"]*)" should contain "([^"]*)"$`, t.attributeContains)
	ctx.Step(`^the span "([^"]*)" attribute "([^"]*)" should be greater than (\d+)$`, t.attributeGreaterThan)
	ctx.Step(`^some span "([^"]*)" should have attribute "([^"]*)" with value "([^"]*)"$`, t.someHasAttribute)
	ctx.Step(`^some span "([^"]*)" should have attribute "([^"]*)"$`, t.someHasAttributeKey)
	ctx.Step(`^I wait for (\d+) spans named "([^"]*)"$`, t.waitForSpans)
	ctx.Step(`^I wait for a span with attribute "([^"]*)" with value "([^"]*)"$`, t.waitForSpanWithAttribute)
	ctx.Step(`^that span should have attribute "([^"]*)" with value "([^"]*)"$`, t.matchedHasAttribute)
	ctx.Step(`^the trace should have (\d+) spans? named "([^"]*)"$`, t.countNamed)
	ctx.Step(`^no span in the trace should have attribute "([^"]*)"$`, t.noSpanHasAttribute)
	ctx.Step(`^an analytics log record should carry the trace$`, t.analyticsLogCarriesTrace)
	ctx.Step(`^I send a POST request to "([^"]*)" with a chat body of (\d+) bytes$`, t.sendLargeChat)
}

// collector calls the mock collector's test API and fails on a non-2xx reply, so a
// failed reset cannot leave earlier spans in the next scenario's way.
func (t *TracingSteps) collector(method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, mockOTLPCollectorURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}
	return resp, nil
}

func (t *TracingSteps) resetCollector() error {
	resp, err := t.collector(http.MethodPost, "/test/reset")
	if err != nil {
		return fmt.Errorf("reset the trace collector: %w", err)
	}
	resp.Body.Close()
	return nil
}

// startTrace sends a fresh, sampled traceparent with every request that follows.
func (t *TracingSteps) startTrace() error {
	traceID, spanID := make([]byte, 16), make([]byte, 8)
	_, _ = rand.Read(traceID)
	_, _ = rand.Read(spanID)
	t.traceID = hex.EncodeToString(traceID)
	t.http.SetHeader("traceparent", "00-"+t.traceID+"-"+hex.EncodeToString(spanID)+"-01")
	return nil
}

func (t *TracingSteps) fetch(traceID string) ([]collectedSpan, error) {
	resp, err := t.collector(http.MethodGet, "/test/spans?traceId="+traceID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var spans []collectedSpan
	return spans, json.NewDecoder(resp.Body).Decode(&spans)
}

// waitForSpan waits until a span of that name has arrived — in the scenario's trace
// when it has one, or else in any trace, whose id the scenario then adopts — and
// refreshes the scenario's view of its trace.
func (t *TracingSteps) waitForSpan(name string) error {
	deadline := time.Now().Add(spanWait)
	for {
		spans, err := t.fetch(t.traceID)
		if err == nil {
			for _, s := range spans {
				if s.Name == name {
					if t.traceID == "" {
						t.traceID = s.TraceID
						spans, err = t.fetch(t.traceID)
					}
					if err == nil {
						t.spans = spans
						return nil
					}
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no span %q arrived within %s (trace %q, %d spans seen: %s)", name, spanWait, t.traceID, len(spans), spanNames(spans))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func spanNames(spans []collectedSpan) string {
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

func (t *TracingSteps) span(name string) (collectedSpan, error) {
	for _, s := range t.spans {
		if s.Name == name {
			return s, nil
		}
	}
	return collectedSpan{}, fmt.Errorf("the trace has no span %q (spans: %s)", name, spanNames(t.spans))
}

func (t *TracingSteps) traceIsTheCallers() error {
	for _, s := range t.spans {
		if s.TraceID != t.traceID {
			return fmt.Errorf("span %q is in trace %s, not the caller's %s", s.Name, s.TraceID, t.traceID)
		}
	}
	return nil
}

func (t *TracingSteps) childOf(child, parent string) error {
	c, err := t.span(child)
	if err != nil {
		return err
	}
	for _, p := range t.spans {
		if p.Name == parent && p.SpanID == c.ParentSpanID {
			return nil
		}
	}
	return fmt.Errorf("span %q (parent %s) is not a child of a span %q", child, c.ParentSpanID, parent)
}

// childOfService waits for the parent to arrive: the router exports its spans in its own
// batches, seconds after the policy engine's.
func (t *TracingSteps) childOfService(child, service string) error {
	c, err := t.span(child)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(spanWait)
	for {
		for _, p := range t.spans {
			if p.SpanID == c.ParentSpanID {
				if p.Service != service {
					return fmt.Errorf("span %q's parent %q is from service %q, want %q", child, p.Name, p.Service, service)
				}
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("span %q's parent %s did not arrive in the trace (spans: %s)", child, c.ParentSpanID, spanNames(t.spans))
		}
		time.Sleep(500 * time.Millisecond)
		if spans, err := t.fetch(t.traceID); err == nil {
			t.spans = spans
		}
	}
}

func (t *TracingSteps) hasKind(name, kind string) error {
	s, err := t.span(name)
	if err != nil {
		return err
	}
	if s.Kind != kind {
		return fmt.Errorf("span %q is %s, want %s", name, s.Kind, kind)
	}
	return nil
}

func (t *TracingSteps) hasStatus(name, status string) error {
	s, err := t.span(name)
	if err != nil {
		return err
	}
	if s.StatusCode != status {
		return fmt.Errorf("span %q has status %s, want %s", name, s.StatusCode, status)
	}
	return nil
}

// attrText renders an attribute value the way a feature file spells it: integers
// without a decimal point, lists as JSON.
func attrText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		b, _ := json.Marshal(x)
		return string(b)
	}
	return fmt.Sprint(v)
}

func (t *TracingSteps) attribute(name, key string) (any, error) {
	s, err := t.span(name)
	if err != nil {
		return nil, err
	}
	v, ok := s.Attributes[key]
	if !ok {
		return nil, fmt.Errorf("span %q has no attribute %q (attributes: %v)", name, key, s.Attributes)
	}
	return v, nil
}

func (t *TracingSteps) hasAttribute(name, key, want string) error {
	v, err := t.attribute(name, key)
	if err != nil {
		return err
	}
	if got := attrText(v); got != want {
		return fmt.Errorf("span %q attribute %q is %q, want %q", name, key, got, want)
	}
	return nil
}

func (t *TracingSteps) hasAttributeKey(name, key string) error {
	_, err := t.attribute(name, key)
	return err
}

func (t *TracingSteps) lacksAttribute(name, key string) error {
	s, err := t.span(name)
	if err != nil {
		return err
	}
	if v, ok := s.Attributes[key]; ok {
		return fmt.Errorf("span %q must not have attribute %q, has %v", name, key, v)
	}
	return nil
}

func (t *TracingSteps) attributeContains(name, key, want string) error {
	v, err := t.attribute(name, key)
	if err != nil {
		return err
	}
	if !strings.Contains(attrText(v), want) {
		return fmt.Errorf("span %q attribute %q (%q) does not contain %q", name, key, attrText(v), want)
	}
	return nil
}

func (t *TracingSteps) attributeGreaterThan(name, key string, min int) error {
	v, err := t.attribute(name, key)
	if err != nil {
		return err
	}
	n, ok := v.(float64)
	if !ok || n <= float64(min) {
		return fmt.Errorf("span %q attribute %q is %v, want more than %d", name, key, v, min)
	}
	return nil
}

func (t *TracingSteps) someHasAttribute(name, key, want string) error {
	for _, s := range t.spans {
		if s.Name == name && attrText(s.Attributes[key]) == want {
			return nil
		}
	}
	return fmt.Errorf("no span %q has attribute %q = %q", name, key, want)
}

func (t *TracingSteps) someHasAttributeKey(name, key string) error {
	for _, s := range t.spans {
		if _, ok := s.Attributes[key]; ok && s.Name == name {
			return nil
		}
	}
	return fmt.Errorf("no span %q has attribute %q", name, key)
}

// waitForSpans waits, in the scenario's trace, until n spans of that name have arrived.
func (t *TracingSteps) waitForSpans(n int, name string) error {
	if err := t.waitForSpan(name); err != nil {
		return err
	}
	deadline := time.Now().Add(spanWait)
	for {
		if t.countNamed(n, name) == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return t.countNamed(n, name)
		}
		time.Sleep(500 * time.Millisecond)
		spans, err := t.fetch(t.traceID)
		if err == nil {
			t.spans = spans
		}
	}
}

// waitForSpanWithAttribute waits, in the scenario's trace, for a span carrying the
// attribute value, and remembers it for "that span" steps.
func (t *TracingSteps) waitForSpanWithAttribute(key, want string) error {
	deadline := time.Now().Add(spanWait)
	for {
		if spans, err := t.fetch(t.traceID); err == nil {
			t.spans = spans
			for i := range spans {
				if attrText(spans[i].Attributes[key]) == want {
					t.matched = &spans[i]
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no span with %s = %q arrived within %s (spans: %s)", key, want, spanWait, spanNames(t.spans))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (t *TracingSteps) matchedHasAttribute(key, want string) error {
	if t.matched == nil {
		return fmt.Errorf("no span was matched by an earlier step")
	}
	if got := attrText(t.matched.Attributes[key]); got != want {
		return fmt.Errorf("span %q attribute %q is %q, want %q", t.matched.Name, key, got, want)
	}
	return nil
}

func (t *TracingSteps) countNamed(want int, name string) error {
	n := 0
	for _, s := range t.spans {
		if s.Name == name {
			n++
		}
	}
	if n != want {
		return fmt.Errorf("the trace has %d spans named %q, want %d (spans: %s)", n, name, want, spanNames(t.spans))
	}
	return nil
}

func (t *TracingSteps) noSpanHasAttribute(key string) error {
	for _, s := range t.spans {
		if v, ok := s.Attributes[key]; ok {
			return fmt.Errorf("span %q has attribute %q = %v", s.Name, key, v)
		}
	}
	return nil
}

// analyticsLogCarriesTrace waits for the OTel analytics publisher's record of the
// request, joined to its trace by the record's own traceId.
func (t *TracingSteps) analyticsLogCarriesTrace() error {
	deadline := time.Now().Add(spanWait)
	for {
		resp, err := t.collector(http.MethodGet, "/test/logs?traceId="+t.traceID)
		if err == nil {
			var logs []json.RawMessage
			err = json.NewDecoder(resp.Body).Decode(&logs)
			resp.Body.Close()
			if err == nil && len(logs) > 0 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no analytics log record carried trace %s within %s", t.traceID, spanWait)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// sendLargeChat posts a chat request whose prompt pads the body to size bytes.
func (t *TracingSteps) sendLargeChat(url string, size int) error {
	prefix := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"`
	suffix := `"}]}`
	body := bytes.Repeat([]byte("a"), size-len(prefix)-len(suffix))
	return t.http.SendRequest(http.MethodPost, url, append(append([]byte(prefix), body...), suffix...))
}
