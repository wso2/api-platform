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
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// Agent (A2A) data-plane steps the generic request steps cannot express: a server-sent event
// stream read incrementally with each event's arrival time, and the A2A dimension block of an
// analytics event.
//
// A buffered read answers what a stream said but not when each event arrived, and for A2A the
// second question is the point: an SSE-framed response that only becomes readable once the
// task finished is a buffered response, and every assertion about its content still passes.

// keyA2AStream holds the scenario's most recently captured event stream.
const keyA2AStream = "a2aStream"

// a2aStreamTimeout bounds one captured stream. It is a stuck-test backstop rather than an
// assertion: a stream that should end and does not is reported by the assertion that follows,
// with the events it did deliver.
const a2aStreamTimeout = 60 * time.Second

// a2aStreamEvent is one SSE data payload and when it became readable, measured from the moment
// the response headers arrived so connection setup is not counted as stream latency.
type a2aStreamEvent struct {
	Data   string
	Offset time.Duration
}

// a2aStream is one captured event stream.
type a2aStream struct {
	Events   []a2aStreamEvent
	Duration time.Duration
}

func (s *a2aStream) summary() string {
	if s == nil || len(s.Events) == 0 {
		return "stream delivered no events"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d event(s) over %s:", len(s.Events), s.Duration.Round(time.Millisecond))
	for i, event := range s.Events {
		data := event.Data
		if len(data) > 200 {
			data = data[:200] + "..."
		}
		fmt.Fprintf(&b, "\n  [%d] +%s %s", i, event.Offset.Round(time.Millisecond), data)
	}
	return b.String()
}

// a2aSSEData returns the payload of an SSE data line, or false for framing: comments, event, id
// and retry fields, and the blank lines that separate events.
func a2aSSEData(line string) (string, bool) {
	rest, ok := strings.CutPrefix(line, "data:")
	if !ok {
		return "", false
	}
	data := strings.TrimSpace(rest)
	return data, data != ""
}

func (g *Gateway) registerAgentSteps(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		tcontext.Remove(ctx, keyA2AStream)
		return ctx, nil
	})

	sc.Step(`^I (?:open an A2A stream|read (\d+) events? from an A2A stream opened) with a "([A-Z]+)" request to "([^"]*)"$`,
		func(ctx context.Context, count, method, path string) error {
			return g.openA2AStream(ctx, count, method, path, nil)
		})
	sc.Step(`^I (?:open an A2A stream|read (\d+) events? from an A2A stream opened) with a "([A-Z]+)" request to "([^"]*)" with body:$`,
		g.openA2AStream)
	sc.Step(`^the A2A stream should have received at least (\d+) events?$`, g.a2aStreamAtLeast)
	sc.Step(`^the A2A stream's first event should arrive before its last event$`, g.a2aStreamFirstBeforeLast)
	sc.Step(`^the A2A stream's last event should contain "([^"]*)"$`, g.a2aStreamLastContains)

	sc.Step(`^the latest analytics event for path "([^"]*)" should (have|not have|carry only|have a non-empty) A2A field "([^"]*)"(?: with value "([^"]*)")?$`,
		g.analyticsA2AField)
}

// openA2AStream performs one request through the data-plane funnel and reads the response as an
// event stream, recording when each event became readable.
//
// count is empty to read until the stream closes, or a positive number of events after which
// the stream is abandoned. The bounded form exists for SubscribeToTask, which attaches to a task
// that is deliberately still running: reading that stream to its end would wait out the agent's
// whole hold. The published response carries the stream's raw lines as its body, so status,
// header and body assertions apply to it.
func (g *Gateway) openA2AStream(ctx context.Context, count, method, path string, body *godog.DocString) error {
	maxEvents := 0
	if count != "" {
		parsed, err := strconv.Atoi(count)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("the number of events to read must be a positive integer, got %q", count)
		}
		maxEvents = parsed
	}
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	url, err := g.gatewayURL(resolved)
	if err != nil {
		return err
	}
	headers := g.scenarioHeaders(ctx)
	headers["Accept"] = "text/event-stream"
	var payload []byte
	if body != nil {
		content, expandErr := stepscommon.Expand(ctx, body.Content)
		if expandErr != nil {
			return expandErr
		}
		payload = []byte(content)
		if headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	}

	stream := &a2aStream{Events: []a2aStreamEvent{}}
	streamCtx, cancel := context.WithTimeout(ctx, a2aStreamTimeout)
	defer cancel()
	started := time.Now()
	_, err = g.funnel.Stream(streamCtx, httpx.Request{
		Method: strings.ToUpper(method), URL: url, Headers: headers, Body: payload, Host: g.requestHost(ctx),
	}, func(line string, since time.Duration) bool {
		if data, ok := a2aSSEData(line); ok {
			stream.Events = append(stream.Events, a2aStreamEvent{Data: data, Offset: since})
		}
		return maxEvents == 0 || len(stream.Events) < maxEvents
	})
	stream.Duration = time.Since(started)
	if err != nil {
		return fmt.Errorf("reading the A2A stream from %s: %w (%s)", url, err, stream.summary())
	}
	return tcontext.Set(ctx, keyA2AStream, stream)
}

func a2aStreamOf(ctx context.Context) (*a2aStream, error) {
	value, ok := tcontext.Get(ctx, keyA2AStream)
	if !ok {
		return nil, fmt.Errorf("no A2A stream has been opened in this scenario")
	}
	stream, ok := value.(*a2aStream)
	if !ok || stream == nil {
		return nil, fmt.Errorf("the captured A2A stream is %T, not a stream", value)
	}
	return stream, nil
}

func (g *Gateway) a2aStreamAtLeast(ctx context.Context, want int) error {
	stream, err := a2aStreamOf(ctx)
	if err != nil {
		return err
	}
	if len(stream.Events) < want {
		return fmt.Errorf("expected at least %d stream event(s), got %d: %s", want, len(stream.Events), stream.summary())
	}
	return nil
}

// a2aStreamFirstBeforeLast is the assertion that separates a stream from a buffered response in
// SSE framing: a response released whole delivers every event at the same instant, while a
// real stream's first event is readable while the task is still running.
func (g *Gateway) a2aStreamFirstBeforeLast(ctx context.Context) error {
	stream, err := a2aStreamOf(ctx)
	if err != nil {
		return err
	}
	if len(stream.Events) < 2 {
		return fmt.Errorf("need at least 2 events to compare arrival times, got %d: %s", len(stream.Events), stream.summary())
	}
	first, last := stream.Events[0].Offset, stream.Events[len(stream.Events)-1].Offset
	if first >= last {
		return fmt.Errorf("first event did not arrive before the last: first=%s last=%s; the response was "+
			"delivered as one buffered unit rather than streamed: %s", first, last, stream.summary())
	}
	return nil
}

func (g *Gateway) a2aStreamLastContains(ctx context.Context, want string) error {
	stream, err := a2aStreamOf(ctx)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	if len(stream.Events) == 0 {
		return fmt.Errorf("the stream delivered no events")
	}
	if last := stream.Events[len(stream.Events)-1].Data; !strings.Contains(last, resolved) {
		return fmt.Errorf("the last stream event did not contain %q: %s", resolved, stream.summary())
	}
	return nil
}

// analyticsA2AField asserts one dimension of the A2A block on the latest analytics event for a
// request path.
//
// The path is what selects the event: one operation invoked over two bindings produces two
// events, so "the latest event" alone is ambiguous. Within the block a dimension is named by its
// published path, such as "response.task_state", so a field that moved between the request and
// response sub-blocks fails rather than being found at the wrong level.
func (g *Gateway) analyticsA2AField(ctx context.Context, path, mode, field, want string) error {
	if mode == "have" && want == "" {
		return fmt.Errorf(`asserting A2A field %q requires a value: use 'with value "..."'`, field)
	}
	if mode != "have" && want != "" {
		return fmt.Errorf("asserting that the event should %s A2A field %q takes no value", mode, field)
	}
	resolvedPath, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	resolvedWant, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	event, err := g.latestAnalyticsEvent(ctx, resolvedPath)
	if err != nil {
		return err
	}
	block, err := a2aAnalyticsBlock(event)
	if err != nil {
		return fmt.Errorf("analytics event for %q: %w", resolvedPath, err)
	}
	return a2aAnalyticsAssert(block, mode, field, resolvedWant)
}

// a2aAnalyticsBlock returns the event's A2A block. The two metadata shapes that preceded the
// block are reported rather than accepted as a fallback: an event carrying either would mean
// the publisher writes the same dimensions twice.
func a2aAnalyticsBlock(event *analyticsEvent) (map[string]any, error) {
	for _, retired := range []string{"a2aAnalytics", "agentAnalytics"} {
		if _, present := event.Metadata[retired]; present {
			return nil, fmt.Errorf("the event carries the retired metadata key %q, which the a2a block replaced", retired)
		}
	}
	if event.A2A == nil {
		return nil, fmt.Errorf("the event carries no a2a block (metadata keys: %s)", sortedAnyKeys(event.Metadata))
	}
	return event.A2A, nil
}

// a2aAnalyticsRequired are the dimensions every A2A event carries, with the catch-all value they
// hold when the event is not an operation, such as a card fetch or a CORS preflight.
var a2aAnalyticsRequired = map[string]string{"operation": "Unknown", "transport": "UNKNOWN"}

// a2aAnalyticsIdentity are the callee agent's identity dimensions, set on every Agent event.
// Their values are generated per deployment, so they are asserted non-empty rather than pinned.
var a2aAnalyticsIdentity = []string{"agent_id", "agent_name"}

func a2aAnalyticsAssert(block map[string]any, mode, field, want string) error {
	switch mode {
	case "have":
		value, err := a2aAnalyticsField(block, field)
		if err != nil {
			return err
		}
		if got := fmt.Sprintf("%v", value); got != want {
			return fmt.Errorf("expected A2A analytics field %q to be %q, got %q", field, want, got)
		}
	case "not have":
		if value, err := a2aAnalyticsField(block, field); err == nil {
			return fmt.Errorf("expected no A2A analytics field %q, but it is present with value %v", field, value)
		}
	case "have a non-empty":
		value, err := a2aAnalyticsField(block, field)
		if err != nil {
			return err
		}
		if value == nil || fmt.Sprintf("%v", value) == "" {
			return fmt.Errorf("A2A analytics field %q is present but empty", field)
		}
	case "carry only":
		return a2aAnalyticsCarriesOnly(block, field)
	default:
		return fmt.Errorf("unsupported A2A analytics assertion %q", mode)
	}
	return nil
}

// a2aAnalyticsCarriesOnly asserts the whole block is one named dimension plus the required and
// identity dimensions. Asserting the entire key set, rather than listing what must be absent, is
// what keeps a newly added dimension from silently leaking onto card fetches and preflights.
func a2aAnalyticsCarriesOnly(block map[string]any, field string) error {
	for name, catchAll := range a2aAnalyticsRequired {
		if got, ok := block[name]; !ok || fmt.Sprintf("%v", got) != catchAll {
			return fmt.Errorf("expected the required A2A field %q to carry its catch-all %q, but it carries %v",
				name, catchAll, got)
		}
	}
	for _, name := range a2aAnalyticsIdentity {
		if got, ok := block[name]; !ok || got == nil || fmt.Sprintf("%v", got) == "" {
			return fmt.Errorf("expected the A2A block to carry a non-empty agent identity field %q, but it carries %v",
				name, got)
		}
	}
	var extra []string
	for name := range block {
		if _, required := a2aAnalyticsRequired[name]; required || name == field || isA2AIdentityField(name) {
			continue
		}
		extra = append(extra, name)
	}
	if block[field] == nil || len(extra) > 0 {
		return fmt.Errorf("expected the A2A block to carry only %q beside the required fields, but it carries {%s}",
			field, sortedAnyKeys(block))
	}
	return nil
}

func isA2AIdentityField(name string) bool {
	for _, identity := range a2aAnalyticsIdentity {
		if name == identity {
			return true
		}
	}
	return false
}

// a2aAnalyticsField reads one dimension by its dotted published path.
func a2aAnalyticsField(block map[string]any, path string) (any, error) {
	segments := strings.Split(path, ".")
	current := block
	for _, segment := range segments[:len(segments)-1] {
		nested, ok := current[segment].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("A2A analytics field %q not found: no %q sub-block in {%s}", path, segment, sortedAnyKeys(current))
		}
		current = nested
	}
	name := segments[len(segments)-1]
	value, ok := current[name]
	if !ok {
		return nil, fmt.Errorf("A2A analytics field %q not found in {%s}", path, sortedAnyKeys(current))
	}
	return value, nil
}

func sortedAnyKeys(m map[string]any) string {
	if len(m) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
