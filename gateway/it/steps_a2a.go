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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// A2A steps for the Agent kind.
//
// Most A2A traffic is ordinary HTTP and needs no step of its own: a JSON-RPC
// call is a POST with a JSON body, and nine of the eleven HTTP+JSON bindings are
// a GET, POST or DELETE against a literal path. Those go through the shared HTTP
// steps. What is here is the three things they cannot do.
//
// The first is identifier capture. Task ids are minted by the agent, so a
// scenario that sends a message and then fetches, subscribes to, or cancels that
// task has to carry a value it could not have written into the feature file.
// Steps below save a field out of a response and substitute {{name}} into a
// later URL or body.
//
// The second is streaming. The shared steps read a body to completion, which
// answers "what did the response say" but not "when did each event arrive" —
// and for A2A the second question is the whole point, because an SSE-framed
// response that only becomes readable once the task finished is a buffered
// response wearing streaming clothes, and every assertion about its content
// still passes.
//
// The third is byte-identity: comparing the card the gateway serves against the
// card the agent serves, which needs both bodies held at once.
//
// Everything here is a raw wire probe, and that is the division of labour with
// steps_a2a_client.go: the conformant path is driven by the official Go A2A SDK
// there, while these steps keep what a conformant client cannot produce or
// expose — a missing or contradictory protocol version, an unknown JSON-RPC
// method, a numeric request id that must come back numeric, exact status codes
// and framing, and the Agent Card's byte and cache semantics.

// a2aStreamEvent is one SSE data payload and when it arrived, measured from the
// moment the response headers came back rather than from when the request was
// sent, so connection setup is not counted as stream latency.
type a2aStreamEvent struct {
	Data   string
	Offset time.Duration
}

// a2aStream is a captured server-sent event stream.
type a2aStream struct {
	Response *http.Response
	Events   []a2aStreamEvent
	Duration time.Duration
}

// a2aSavedCard is a card body held for a later byte-identity comparison.
type a2aSavedCard struct {
	Body []byte
	ETag string
}

// A2ASteps holds per-scenario A2A state.
//
// A fresh instance is built for every scenario by InitializeScenario, so nothing
// here needs resetting between scenarios and no value can leak from one into the
// next.
type A2ASteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps

	// vars holds values captured out of earlier responses, keyed by the name the
	// feature file gave them.
	vars map[string]string

	// stream is the most recently captured SSE stream.
	stream *a2aStream

	// savedCard is the most recently saved Agent Card body.
	savedCard *a2aSavedCard
}

// a2aStreamTimeout bounds a captured stream.
//
// Generous on purpose: it is a stuck-test backstop, not an assertion. A stream
// that should close on a terminal event and does not is caught by the terminal
// assertion with the events it did deliver, which says far more than a timeout
// would.
const a2aStreamTimeout = 60 * time.Second

// RegisterA2ASteps registers the Agent (A2A) step definitions.
func RegisterA2ASteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	a := &A2ASteps{
		state:     state,
		httpSteps: httpSteps,
		vars:      make(map[string]string),
	}

	// ---- Management API ----

	ctx.Step(`^I deploy this Agent configuration:$`, a.deployAgent)
	ctx.Step(`^I list all Agents$`, a.listAgents)
	ctx.Step(`^I get the Agent "([^"]*)"$`, a.getAgent)
	ctx.Step(`^I update the Agent "([^"]*)" with:$`, a.updateAgent)
	ctx.Step(`^I delete the Agent "([^"]*)"$`, a.deleteAgent)

	// ---- Captured values ----

	ctx.Step(`^I save the JSON response field "([^"]*)" as "([^"]*)"$`, a.saveJSONField)

	// ---- Invocation ----

	ctx.Step(`^I send an A2A JSON-RPC request to "([^"]*)":$`, a.sendJSONRPC)
	ctx.Step(`^I send an A2A "([A-Z]+)" request to "([^"]*)"$`, a.sendHTTPJSON)
	ctx.Step(`^I send an A2A "([A-Z]+)" request to "([^"]*)" with:$`, a.sendHTTPJSONWithBody)

	// Protocol-version variants. "no version" omits the header, which the
	// specification reads as 0.3 — see a2aVersionHeader.
	ctx.Step(`^I send an A2A JSON-RPC request with no version to "([^"]*)":$`,
		func(url string, body *godog.DocString) error { return a.sendJSONRPCWithVersion("", url, body) })
	ctx.Step(`^I send an A2A JSON-RPC request with version "([^"]*)" to "([^"]*)":$`, a.sendJSONRPCWithVersion)
	ctx.Step(`^I send an A2A "([A-Z]+)" request with no version to "([^"]*)"$`,
		func(method, url string) error { return a.sendHTTPJSONWithVersion("", method, url) })
	ctx.Step(`^I send an A2A "([A-Z]+)" request with version "([^"]*)" to "([^"]*)"$`,
		func(method, version, url string) error { return a.sendHTTPJSONWithVersion(version, method, url) })

	// ---- Streaming ----

	ctx.Step(`^I open an A2A stream with a JSON-RPC request to "([^"]*)":$`, a.openJSONRPCStream)
	ctx.Step(`^I open an A2A stream with a "([A-Z]+)" request to "([^"]*)"$`, a.openHTTPJSONStream)
	ctx.Step(`^I open an A2A stream with a "([A-Z]+)" request to "([^"]*)" with:$`, a.openHTTPJSONStreamWithBody)
	ctx.Step(`^I read (\d+) events? from an A2A stream opened with a JSON-RPC request to "([^"]*)":$`, a.readJSONRPCStreamEvents)
	ctx.Step(`^I read (\d+) events? from an A2A stream opened with a "([A-Z]+)" request to "([^"]*)"$`, a.readHTTPJSONStreamEvents)
	ctx.Step(`^the A2A stream should have received at least (\d+) events?$`, a.streamEventCountAtLeast)
	ctx.Step(`^the A2A stream's first event should arrive before its last event$`, a.streamFirstBeforeLast)
	ctx.Step(`^the A2A stream should contain an event containing "([^"]*)"$`, a.streamContainsEvent)
	ctx.Step(`^the A2A stream's last event should contain "([^"]*)"$`, a.streamLastEventContains)
	ctx.Step(`^the A2A stream response header "([^"]*)" should contain "([^"]*)"$`, a.streamHeaderContains)
	ctx.Step(`^the A2A stream response should not have a "([^"]*)" header$`, a.streamHeaderAbsent)

	// ---- Agent Card ----

	ctx.Step(`^I save the Agent Card at "([^"]*)"$`, a.saveAgentCard)
	ctx.Step(`^I fetch the Agent Card at "([^"]*)" with the saved ETag$`, a.fetchCardWithSavedETag)
	ctx.Step(`^the Agent Card at "([^"]*)" should be byte-identical to the saved card$`, a.cardShouldMatchSaved)
	ctx.Step(`^the response ETag should differ from the saved ETag$`, a.etagShouldDiffer)
	ctx.Step(`^the response ETag should match the saved ETag$`, a.etagShouldMatch)
}

// ---- Management API ----

func (a *A2ASteps) deployAgent(body *godog.DocString) error {
	a.httpSteps.SetHeader("Content-Type", "application/yaml")
	if err := a.httpSteps.SendPOSTToService("gateway-controller", "/agents", body); err != nil {
		return err
	}
	time.Sleep(policyPropagationDelay)
	return nil
}

func (a *A2ASteps) listAgents() error {
	return a.httpSteps.SendGETToService("gateway-controller", "/agents")
}

func (a *A2ASteps) getAgent(name string) error {
	return a.httpSteps.SendGETToService("gateway-controller", "/agents/"+name)
}

func (a *A2ASteps) updateAgent(name string, body *godog.DocString) error {
	a.httpSteps.SetHeader("Content-Type", "application/yaml")
	if err := a.httpSteps.SendPUTToService("gateway-controller", "/agents/"+name, body); err != nil {
		return err
	}
	time.Sleep(policyPropagationDelay)
	return nil
}

func (a *A2ASteps) deleteAgent(name string) error {
	if err := a.httpSteps.SendDELETEToService("gateway-controller", "/agents/"+name); err != nil {
		return err
	}
	time.Sleep(policyPropagationDelay)
	return nil
}

// ---- Captured values ----

// saveJSONField stores a field from the last response under a name a later step
// can substitute as {{name}}.
func (a *A2ASteps) saveJSONField(field, name string) error {
	value, err := a2aResolveJSONPath(a.httpSteps.LastBody(), field)
	if err != nil {
		return err
	}
	text, ok := a2aScalarString(value)
	if !ok {
		return fmt.Errorf("field %q is %T, which cannot be saved as a substitution value", field, value)
	}
	a.vars[name] = text
	return nil
}

// substitute replaces every {{name}} with a previously saved value.
//
// An unknown name is an error rather than an empty string: silently substituting
// nothing turns "fetch the task we just created" into "fetch the task named
// empty string", which the agent answers with a perfectly ordinary 404 and the
// scenario then reports as a gateway routing failure.
func (a *A2ASteps) substitute(text string) (string, error) {
	var out strings.Builder
	rest := text
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			out.WriteString(rest)
			return out.String(), nil
		}
		end := strings.Index(rest[start:], "}}")
		if end < 0 {
			out.WriteString(rest)
			return out.String(), nil
		}
		end += start
		name := strings.TrimSpace(rest[start+2 : end])
		value, ok := a.vars[name]
		if !ok {
			return "", fmt.Errorf("no saved value named %q (saved: %s)", name, a2aKnownNames(a.vars))
		}
		out.WriteString(rest[:start])
		out.WriteString(value)
		rest = rest[end+2:]
	}
}

func a2aKnownNames(vars map[string]string) string {
	if len(vars) == 0 {
		return "none"
	}
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// ---- Invocation ----

// a2aVersionHeader identifies the caller's A2A protocol version.
//
// A2A 1.0 section 3.6.1 requires a client to declare its protocol version on
// every request; 3.6.2 says an absent or empty value means 0.3, which a 1.0
// agent then rejects. So it is a client obligation, and these steps carry it on
// every operation request the way any conformant A2A client would.
//
// The gateway enforces it before it resolves anything (Section 8A), so an
// operation request that omits it is refused by the gateway rather than by the
// agent behind it. That is why every operation step below sends it and why the
// scenarios that leave it out, send a wrong one, or contradict themselves assert
// a rejection: sending it is no longer only good manners.
//
// Not sent on Agent Card fetches. Card discovery is a plain GET of a static
// document, not an operation in either binding — the gateway leaves that route
// unversioned deliberately, since a client commonly fetches the card in order to
// learn which versions the Agent speaks.
const a2aVersionHeader = "A2A-Version"

// a2aProtocolVersion is the version every Agent in the suite exposes.
const a2aProtocolVersion = "1.0"

// sendJSONRPCWithVersion and its siblings send an operation request stating a
// specific protocol version, or none at all.
//
// They exist for the Section 8A scenarios and nothing else: the ordinary steps
// always send the correct version, because an A2A client that does not is
// non-conformant and a scenario written that way would be asserting the
// rejection path by accident. A version of "" omits the header entirely, which is
// the case the specification reads as 0.3.
func (a *A2ASteps) sendJSONRPCWithVersion(version, url string, body *godog.DocString) error {
	a.httpSteps.SetHeader("Content-Type", "application/json")
	a.httpSteps.SetHeader("Accept", "application/json, text/event-stream")
	return a.sendOperationWithVersion(version, http.MethodPost, url, body)
}

func (a *A2ASteps) sendHTTPJSONWithVersion(version, method, url string) error {
	return a.sendOperationWithVersion(version, method, url, nil)
}

func (a *A2ASteps) sendJSONRPC(url string, body *godog.DocString) error {
	// Accept both media types because one JSON-RPC endpoint serves both: the
	// same route answers GetTask as JSON and SendStreamingMessage as an event
	// stream, and which one comes back is decided by the method in the body.
	a.httpSteps.SetHeader("Content-Type", "application/json")
	a.httpSteps.SetHeader("Accept", "application/json, text/event-stream")
	return a.sendOperation(http.MethodPost, url, body)
}

func (a *A2ASteps) sendHTTPJSON(method, url string) error {
	return a.sendOperation(method, url, nil)
}

func (a *A2ASteps) sendHTTPJSONWithBody(method, url string, body *godog.DocString) error {
	a.httpSteps.SetHeader("Content-Type", "application/json")
	return a.sendOperation(method, url, body)
}

// sendOperation is send with the protocol-version header an A2A client owes the
// agent, scoped to this one request so it cannot leak onto an unrelated one.
func (a *A2ASteps) sendOperation(method, url string, body *godog.DocString) error {
	return a.sendOperationWithVersion(a2aProtocolVersion, method, url, body)
}

// sendOperationWithVersion is sendOperation with the stated version chosen by the
// caller. An empty version sends no header at all.
func (a *A2ASteps) sendOperationWithVersion(version, method, url string, body *godog.DocString) error {
	previous := a.httpSteps.Header(a2aVersionHeader)
	if version == "" {
		a.httpSteps.ClearHeader(a2aVersionHeader)
	} else {
		a.httpSteps.SetHeader(a2aVersionHeader, version)
	}
	defer func() {
		if previous == "" {
			// Cleared, not set to "": an empty A2A-Version means 0.3, so
			// restoring it as an empty string would leave a different header
			// behind than the one this found.
			a.httpSteps.ClearHeader(a2aVersionHeader)
			return
		}
		a.httpSteps.SetHeader(a2aVersionHeader, previous)
	}()
	return a.send(method, url, body)
}

// send substitutes captured values into the URL and body, then performs the
// request through the shared HTTP steps so the shared assertions apply to it.
//
// An event-stream response is drained here and the first event's payload kept as
// the body. That makes a streaming operation reachable from a scenario that only
// cares whether it routes at all; a scenario that cares about the stream itself
// uses the stream steps instead, which keep every event and its arrival time.
func (a *A2ASteps) send(method, url string, body *godog.DocString) error {
	resolvedURL, err := a.substitute(url)
	if err != nil {
		return err
	}

	var payload []byte
	if body != nil {
		resolvedBody, err := a.substitute(body.Content)
		if err != nil {
			return err
		}
		payload = []byte(resolvedBody)
	}

	if err := a.httpSteps.SendRequest(method, resolvedURL, payload); err != nil {
		return err
	}

	resp := a.httpSteps.LastResponse()
	if resp != nil && a2aIsEventStream(resp.Header) {
		events := a2aParseSSE(a.httpSteps.LastBody())
		if len(events) > 0 {
			a.httpSteps.RecordResponse(resp, []byte(events[0]))
		}
	}
	return nil
}

// ---- Streaming ----

func (a *A2ASteps) openJSONRPCStream(url string, body *godog.DocString) error {
	return a.openStream(http.MethodPost, url, body, "application/json", 0)
}

func (a *A2ASteps) openHTTPJSONStream(method, url string) error {
	return a.openStream(method, url, nil, "", 0)
}

func (a *A2ASteps) openHTTPJSONStreamWithBody(method, url string, body *godog.DocString) error {
	return a.openStream(method, url, body, "application/json", 0)
}

// The bounded-read variants stop after a fixed number of events instead of
// reading to close.
//
// Necessary for SubscribeToTask, which attaches to a task that is deliberately
// still running: that stream stays open until the task reaches a terminal state,
// which for a slow-mode task is the whole configured hold. Reading it to close
// would make the scenario wait out the hold, and — worse — a hold longer than
// the client timeout would fail on a read error rather than on an assertion,
// reporting a timeout where the actual finding is "the stream worked".
//
// The bound is on events, not time: what the scenario is asserting is that live
// events arrive on a re-attached stream, and it has that answer as soon as they
// do.
func (a *A2ASteps) readJSONRPCStreamEvents(count int, url string, body *godog.DocString) error {
	return a.openStream(http.MethodPost, url, body, "application/json", count)
}

func (a *A2ASteps) readHTTPJSONStreamEvents(count int, method, url string) error {
	return a.openStream(method, url, nil, "", count)
}

// openStream performs a request and reads the response incrementally, recording
// when each event became readable.
//
// It does not use the shared HTTP steps, and that is the point: those read the
// body to completion before returning, which collapses every arrival time onto
// the moment the stream closed. The response is still handed back to them
// afterwards so the ordinary status, header and body assertions work unchanged.
// maxEvents of 0 reads until the stream closes; a positive value stops as soon
// as that many events have arrived.
func (a *A2ASteps) openStream(method, url string, body *godog.DocString, contentType string, maxEvents int) error {
	resolvedURL, err := a.substitute(url)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		resolvedBody, err := a.substitute(body.Content)
		if err != nil {
			return err
		}
		reader = strings.NewReader(resolvedBody)
	}

	req, err := http.NewRequest(method, resolvedURL, reader)
	if err != nil {
		return fmt.Errorf("failed to create A2A stream request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set(a2aVersionHeader, a2aProtocolVersion)
	for name, value := range a.streamHeaders() {
		req.Header.Set(name, value)
	}

	// A dedicated client: the shared one carries a whole-request timeout, which
	// on a long-lived stream would cut the response off mid-flight and report it
	// as a gateway failure.
	client := &http.Client{Timeout: a2aStreamTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to open A2A stream to %s: %w", resolvedURL, err)
	}
	defer resp.Body.Close()

	start := time.Now()
	stream := &a2aStream{Response: resp}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if !strings.HasPrefix(line, "data:") {
			// Comments (": ping"), event/id/retry fields and the blank lines
			// separating events are all framing, not payload.
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		stream.Events = append(stream.Events, a2aStreamEvent{
			Data:   data,
			Offset: time.Since(start),
		})
		if maxEvents > 0 && len(stream.Events) >= maxEvents {
			// Enough to answer the question. The deferred Close drops the
			// connection, which the agent sees as a client disconnect — the
			// same thing a real client that has read what it needed does.
			break
		}
	}
	stream.Duration = time.Since(start)
	// A read error after the requested number of events is not a failure: it is
	// what abandoning a still-open stream looks like from this side.
	if err := scanner.Err(); err != nil && !(maxEvents > 0 && len(stream.Events) >= maxEvents) {
		return fmt.Errorf("error reading A2A stream from %s: %w", resolvedURL, err)
	}

	a.stream = stream

	// Hand the response to the shared steps so status and header assertions can
	// follow. The body is the concatenated event payloads: a scenario asserting
	// on stream content is served by the stream steps, and this keeps
	// "the response body should contain" meaningful rather than empty.
	var combined bytes.Buffer
	for _, event := range stream.Events {
		combined.WriteString(event.Data)
		combined.WriteByte('\n')
	}
	a.httpSteps.RecordResponse(resp, combined.Bytes())
	return nil
}

// streamHeaders returns the headers a stream request should carry.
//
// Only the ones a scenario sets deliberately for authentication travel: the
// shared step state also holds Content-Type and Accept values left over from
// earlier steps, and an Accept of application/json on a stream request would ask
// the agent for the wrong framing.
func (a *A2ASteps) streamHeaders() map[string]string {
	carried := map[string]string{}
	for _, name := range []string{"Authorization", "api-key", "apikey", "X-API-Key"} {
		if value := a.httpSteps.Header(name); value != "" {
			carried[name] = value
		}
	}
	return carried
}

func (a *A2ASteps) requireStream() (*a2aStream, error) {
	if a.stream == nil {
		return nil, fmt.Errorf("no A2A stream has been opened in this scenario")
	}
	return a.stream, nil
}

func (a *A2ASteps) streamEventCountAtLeast(expected int) error {
	stream, err := a.requireStream()
	if err != nil {
		return err
	}
	if len(stream.Events) < expected {
		return fmt.Errorf("expected at least %d stream event(s), got %d: %s",
			expected, len(stream.Events), a2aStreamSummary(stream))
	}
	return nil
}

// streamFirstBeforeLast is the assertion that separates a stream from a buffered
// response in SSE clothing.
//
// A response the gateway held whole and released at the end delivers every event
// at the same instant, so its first and last offsets are equal. A real stream's
// first event is readable while the task is still running. The agent paces its
// updates precisely so this gap exists and is observable.
func (a *A2ASteps) streamFirstBeforeLast() error {
	stream, err := a.requireStream()
	if err != nil {
		return err
	}
	if len(stream.Events) < 2 {
		return fmt.Errorf("need at least 2 events to compare arrival times, got %d: %s",
			len(stream.Events), a2aStreamSummary(stream))
	}
	first := stream.Events[0].Offset
	last := stream.Events[len(stream.Events)-1].Offset
	if first >= last {
		return fmt.Errorf(
			"first event did not arrive before the last: first=%s last=%s — the response was "+
				"delivered as one buffered unit rather than streamed: %s",
			first, last, a2aStreamSummary(stream))
	}
	return nil
}

func (a *A2ASteps) streamContainsEvent(expected string) error {
	stream, err := a.requireStream()
	if err != nil {
		return err
	}
	for _, event := range stream.Events {
		if strings.Contains(event.Data, expected) {
			return nil
		}
	}
	return fmt.Errorf("no stream event contained %q: %s", expected, a2aStreamSummary(stream))
}

func (a *A2ASteps) streamLastEventContains(expected string) error {
	stream, err := a.requireStream()
	if err != nil {
		return err
	}
	if len(stream.Events) == 0 {
		return fmt.Errorf("the stream delivered no events")
	}
	last := stream.Events[len(stream.Events)-1]
	if !strings.Contains(last.Data, expected) {
		return fmt.Errorf("last stream event did not contain %q: %s", expected, truncateA2A(last.Data, 400))
	}
	return nil
}

func (a *A2ASteps) streamHeaderContains(name, expected string) error {
	stream, err := a.requireStream()
	if err != nil {
		return err
	}
	actual := stream.Response.Header.Get(name)
	if actual == "" && strings.EqualFold(name, "Transfer-Encoding") {
		// net/http removes Transfer-Encoding from the header map and reports it
		// on the typed field instead, so reading the header alone would report
		// a chunked response as having no transfer encoding at all.
		actual = strings.Join(stream.Response.TransferEncoding, ", ")
	}
	if !strings.Contains(strings.ToLower(actual), strings.ToLower(expected)) {
		return fmt.Errorf("expected stream response header %q to contain %q, got %q", name, expected, actual)
	}
	return nil
}

// streamHeaderAbsent asserts a header the response must not carry.
//
// Used for Content-Length on a streamed response: its presence means the gateway
// knew the full size before sending, which it cannot on a stream it is
// forwarding chunk by chunk.
func (a *A2ASteps) streamHeaderAbsent(name string) error {
	stream, err := a.requireStream()
	if err != nil {
		return err
	}
	if values := stream.Response.Header.Values(name); len(values) > 0 {
		return fmt.Errorf("expected no %q header on the streamed response, got %q", name, strings.Join(values, ", "))
	}
	// Go strips Content-Length into the typed field rather than leaving it in
	// the header map, so the header check alone would pass on a response that
	// carries one.
	if strings.EqualFold(name, "Content-Length") && stream.Response.ContentLength >= 0 {
		return fmt.Errorf("expected no Content-Length on the streamed response, got %d",
			stream.Response.ContentLength)
	}
	return nil
}

func a2aStreamSummary(stream *a2aStream) string {
	if len(stream.Events) == 0 {
		return "stream delivered no events"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d event(s) over %s:", len(stream.Events), stream.Duration.Round(time.Millisecond))
	for i, event := range stream.Events {
		fmt.Fprintf(&b, "\n  [%d] +%s %s", i, event.Offset.Round(time.Millisecond), truncateA2A(event.Data, 200))
	}
	return b.String()
}

// ---- Agent Card ----

func (a *A2ASteps) saveAgentCard(url string) error {
	if err := a.send(http.MethodGet, url, nil); err != nil {
		return err
	}
	resp := a.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received from %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected 200 fetching the Agent Card at %s, got %d", url, resp.StatusCode)
	}
	body := append([]byte(nil), a.httpSteps.LastBody()...)
	a.savedCard = &a2aSavedCard{Body: body, ETag: resp.Header.Get("ETag")}
	return nil
}

func (a *A2ASteps) fetchCardWithSavedETag(url string) error {
	if a.savedCard == nil {
		return fmt.Errorf("no Agent Card has been saved in this scenario")
	}
	if a.savedCard.ETag == "" {
		return fmt.Errorf("the saved Agent Card carried no ETag, so a conditional request cannot be made")
	}
	a.httpSteps.SetHeader("If-None-Match", a.savedCard.ETag)
	err := a.send(http.MethodGet, url, nil)
	// Scoped to this one request: leaving it set would make every later card
	// fetch in the scenario conditional, and a 304 where the scenario expected a
	// body is a confusing way to discover that. Cleared rather than emptied — an
	// empty If-None-Match is still a conditional request.
	a.httpSteps.ClearHeader("If-None-Match")
	return err
}

// cardShouldMatchSaved compares two card bodies byte for byte.
//
// Byte equality rather than JSON equality on purpose. The gateway's contract is
// that it serves the card as supplied — in passthrough mode the upstream's own
// bytes, in managed mode the author's — and a re-encoding that happens to be
// semantically equal still breaks the signature that will eventually be computed
// over those bytes.
func (a *A2ASteps) cardShouldMatchSaved(url string) error {
	if a.savedCard == nil {
		return fmt.Errorf("no Agent Card has been saved in this scenario")
	}
	if err := a.send(http.MethodGet, url, nil); err != nil {
		return err
	}
	actual := a.httpSteps.LastBody()
	if !bytes.Equal(actual, a.savedCard.Body) {
		return fmt.Errorf(
			"Agent Card at %s is not byte-identical to the saved card\nsaved (%d bytes): %s\nserved (%d bytes): %s",
			url, len(a.savedCard.Body), truncateA2A(string(a.savedCard.Body), 400),
			len(actual), truncateA2A(string(actual), 400))
	}
	return nil
}

// etagShouldMatch asserts the tag is unchanged.
//
// Byte-identity of the card already implies this, since the tag is derived from
// those bytes — which is exactly why a mismatch here is worth catching
// separately: it would mean the tag and the card it identifies disagree, and a
// client holding the old tag would be told its cached copy is stale when it is
// not, or worse, that a changed card is unchanged.
func (a *A2ASteps) etagShouldMatch() error {
	if a.savedCard == nil {
		return fmt.Errorf("no Agent Card has been saved in this scenario")
	}
	resp := a.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	actual := resp.Header.Get("ETag")
	if actual != a.savedCard.ETag {
		return fmt.Errorf("expected the ETag to still be %q, got %q", a.savedCard.ETag, actual)
	}
	return nil
}

func (a *A2ASteps) etagShouldDiffer() error {
	if a.savedCard == nil {
		return fmt.Errorf("no Agent Card has been saved in this scenario")
	}
	resp := a.httpSteps.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	actual := resp.Header.Get("ETag")
	if actual == "" {
		return fmt.Errorf("the response carried no ETag")
	}
	if actual == a.savedCard.ETag {
		return fmt.Errorf("expected the ETag to change, but it is still %q", actual)
	}
	return nil
}

// ---- Helpers ----

func a2aIsEventStream(header http.Header) bool {
	return strings.Contains(strings.ToLower(header.Get("Content-Type")), "text/event-stream")
}

// a2aParseSSE extracts the data payloads from an already-buffered event stream.
func a2aParseSSE(body []byte) []string {
	var events []string
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		if data := strings.TrimSpace(strings.TrimPrefix(line, "data:")); data != "" {
			events = append(events, data)
		}
	}
	return events
}

// a2aResolveJSONPath walks a dotted path with optional [n] indexes, matching the
// shared assertion steps' path syntax so a feature file can use one spelling.
func a2aResolveJSONPath(body []byte, path string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse the last response as JSON: %w", err)
	}

	current := data
	for _, part := range strings.Split(path, ".") {
		key := part
		index := -1
		if i := strings.Index(part, "["); i != -1 && strings.HasSuffix(part, "]") {
			key = part[:i]
			parsed, err := strconv.Atoi(part[i+1 : len(part)-1])
			if err != nil {
				return nil, fmt.Errorf("invalid array index in path %q: %w", part, err)
			}
			index = parsed
		}

		if key != "" {
			object, ok := current.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected an object at %q in path %q, got %T", key, path, current)
			}
			value, exists := object[key]
			if !exists {
				return nil, fmt.Errorf("path %q not found in the last response: %q is absent", path, key)
			}
			current = value
		}

		if index >= 0 {
			list, ok := current.([]interface{})
			if !ok {
				return nil, fmt.Errorf("expected an array at %q in path %q, got %T", part, path, current)
			}
			if index >= len(list) {
				return nil, fmt.Errorf("index %d out of range at %q in path %q (length %d)", index, part, path, len(list))
			}
			current = list[index]
		}
	}
	return current, nil
}

// a2aScalarString renders a JSON scalar as the string a later request should
// carry. Numbers come back from encoding/json as float64, and a task id or page
// size formatted as "3.000000" would fail in a way that looks like a routing
// problem rather than a formatting one.
func a2aScalarString(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	default:
		return "", false
	}
}

func truncateA2A(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
