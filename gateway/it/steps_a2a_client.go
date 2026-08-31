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
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// A2A steps that speak the protocol through the official Go SDK.
//
// The upstream fixture is a real agent on the official Python SDK. Driving it
// from hand-built JSON-RPC envelopes and REST paths would make both halves of
// the conformant path *our* reading of the specification: if we misread a field
// name the same way in the fixture's Agent resource and in the step that calls
// it, the scenario still passes. Calling through the official Go client makes
// the positive path a cross-SDK interoperability test — two independent
// implementations of the same protocol version, with the gateway between them.
//
// These steps therefore own the conformant path only: typed requests, typed
// results, and semantic assertions over them. The raw net/http steps in
// steps_a2a.go keep everything the SDK cannot faithfully produce or expose —
// missing and malformed protocol versions, unknown JSON-RPC methods, numeric
// request-id preservation, exact status codes and response framing, and the
// Agent Card's byte and cache semantics. A typed SDK result is never marshalled
// back into the shared response state and asserted as though it were the bytes
// the gateway returned: those bytes would be the test's own.
//
// Each client is built from an explicit gateway endpoint — never by resolving a
// card and following the URL in it. A passthrough card legitimately advertises
// the *upstream agent's* address, so a client that followed it would bypass the
// gateway entirely and pass having exercised no route and no policy.

// a2aSDKCallTimeout bounds one unary SDK call.
const a2aSDKCallTimeout = 30 * time.Second

// a2aSDKStreamTimeout bounds a whole streamed call, including the time spent
// waiting for events. Generous: it is a stuck-test backstop, not an assertion.
const a2aSDKStreamTimeout = 60 * time.Second

// a2aSDKEvent is one event yielded by an SDK stream iterator, with the moment
// it was handed to us measured from the start of the call.
//
// The offset is what keeps buffering observable through the SDK: a response the
// gateway held whole and released at the end yields every event at effectively
// the same instant, and every content assertion over it still passes.
type a2aSDKEvent struct {
	Event  a2a.Event
	Offset time.Duration
}

// a2aSDKOutcome is the typed result of the most recent call on one client.
//
// Exactly one of the payload fields is populated per call, or Err is. Keeping
// them typed — rather than re-encoding to JSON and asserting on text — is the
// point of this layer: an assertion reads the field the protocol defines.
type a2aSDKOutcome struct {
	Method string
	Err    error

	Task        *a2a.Task
	Message     *a2a.Message
	Tasks       []*a2a.Task
	Card        *a2a.AgentCard
	PushConfig  *a2a.PushConfig
	PushConfigs []*a2a.PushConfig

	Events         []a2aSDKEvent
	StreamDuration time.Duration
}

// a2aSDKClient is one named client bound to one gateway endpoint and one
// protocol binding.
type a2aSDKClient struct {
	name    string
	binding a2a.TransportProtocol
	url     string
	client  *a2aclient.Client
	last    *a2aSDKOutcome
}

// A2AClientSteps holds the per-scenario SDK clients and their results.
type A2AClientSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps

	clients map[string]*a2aSDKClient

	// taskID is the task the scenario is working with, captured from whichever
	// call first produced one. It is shared across clients on purpose: a task
	// created over JSON-RPC and then read over HTTP+JSON is exactly the
	// cross-transport claim these tests exist to check.
	taskID a2a.TaskID
}

// RegisterA2AClientSteps registers the SDK-backed A2A step definitions.
func RegisterA2AClientSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	a := &A2AClientSteps{
		state:     state,
		httpSteps: httpSteps,
		clients:   make(map[string]*a2aSDKClient),
	}

	// Clients hold connections; a scenario that created several would otherwise
	// leave them open until the suite ends.
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		a.destroyAll()
		return ctx, nil
	})

	// ---- Client construction ----

	ctx.Step(`^I create an A2A client "([^"]*)" for the "(JSONRPC|HTTP\+JSON)" binding at "([^"]*)"$`, a.createClient)

	// ---- Operations ----

	ctx.Step(`^the A2A client "([^"]*)" sends the message "([^"]*)"$`,
		func(name, text string) error { return a.sendMessage(name, text, false) })
	ctx.Step(`^the A2A client "([^"]*)" sends the message "([^"]*)" and returns immediately$`,
		func(name, text string) error { return a.sendMessage(name, text, true) })
	ctx.Step(`^the A2A client "([^"]*)" streams the message "([^"]*)"$`, a.streamMessage)
	ctx.Step(`^the A2A client "([^"]*)" gets the task$`, a.getTask)
	ctx.Step(`^the A2A client "([^"]*)" lists tasks$`, a.listTasks)
	ctx.Step(`^the A2A client "([^"]*)" cancels the task$`, a.cancelTask)
	ctx.Step(`^the A2A client "([^"]*)" subscribes to the task and reads (\d+) events?$`, a.subscribeToTask)
	ctx.Step(`^the A2A client "([^"]*)" creates a push notification config "([^"]*)" for the task$`, a.createPushConfig)
	ctx.Step(`^the A2A client "([^"]*)" gets the push notification config "([^"]*)" for the task$`, a.getPushConfig)
	ctx.Step(`^the A2A client "([^"]*)" lists push notification configs for the task$`, a.listPushConfigs)
	ctx.Step(`^the A2A client "([^"]*)" deletes the push notification config "([^"]*)" for the task$`, a.deletePushConfig)
	ctx.Step(`^the A2A client "([^"]*)" gets the extended Agent Card$`, a.getExtendedCard)

	// ---- Assertions ----

	ctx.Step(`^the A2A client "([^"]*)" call should have succeeded$`, a.callShouldSucceed)
	ctx.Step(`^the A2A client "([^"]*)" call should have failed$`, a.callShouldFail)
	ctx.Step(`^the A2A client "([^"]*)" should have received a task in state "([^"]*)"$`, a.taskStateShouldBe)
	ctx.Step(`^the A2A client "([^"]*)" should have received a task that is still running$`, a.taskShouldBeRunning)
	ctx.Step(`^the A2A client "([^"]*)" should have received an artifact containing "([^"]*)"$`, a.artifactShouldContain)
	ctx.Step(`^the A2A client "([^"]*)" should have received a task list containing the task$`, a.taskListShouldContainTask)
	ctx.Step(`^the A2A client "([^"]*)" should have received a push config "([^"]*)"$`, a.pushConfigShouldBe)
	ctx.Step(`^the A2A client "([^"]*)" should have received (\d+) push configs?$`, a.pushConfigCountShouldBe)
	ctx.Step(`^the A2A client "([^"]*)" should have received an Agent Card named "([^"]*)"$`, a.cardNameShouldBe)
	ctx.Step(`^the A2A client "([^"]*)" should have received an Agent Card with the skill "([^"]*)"$`,
		func(name, skill string) error { return a.cardSkill(name, skill, true) })
	ctx.Step(`^the A2A client "([^"]*)" should have received an Agent Card without the skill "([^"]*)"$`,
		func(name, skill string) error { return a.cardSkill(name, skill, false) })
	ctx.Step(`^the A2A clients "([^"]*)" and "([^"]*)" should have received the same artifact$`, a.artifactsShouldMatch)
	ctx.Step(`^the A2A client "([^"]*)" should have received at least (\d+) stream events?$`, a.streamEventCountAtLeast)
	ctx.Step(`^the A2A client "([^"]*)" stream should end in state "([^"]*)"$`, a.streamShouldEndInState)
	ctx.Step(`^the A2A client "([^"]*)" stream's first event should arrive before its last event$`, a.streamFirstBeforeLast)
	ctx.Step(`^the A2A client "([^"]*)" should have received a stream event containing "([^"]*)"$`, a.streamShouldContain)
}

// ---- Client construction ----

// createClient builds a client for exactly one endpoint and one binding.
//
// Defaults are disabled and a single transport factory added, so a client asked
// for JSON-RPC cannot quietly fall back to REST — which would turn a scenario
// claiming "both bindings" into one that tested one binding twice.
func (a *A2AClientSteps) createClient(name, binding, url string) error {
	protocol := a2a.TransportProtocol(binding)

	opts := []a2aclient.FactoryOption{a2aclient.WithDefaultsDisabled()}
	httpClient := &http.Client{Timeout: a2aSDKStreamTimeout}
	switch protocol {
	case a2a.TransportProtocolJSONRPC:
		opts = append(opts, a2aclient.WithJSONRPCTransport(httpClient))
	case a2a.TransportProtocolHTTPJSON:
		opts = append(opts, a2aclient.WithRESTTransport(httpClient))
	default:
		return fmt.Errorf("unsupported A2A protocol binding %q", binding)
	}

	endpoint := &a2a.AgentInterface{
		URL:             url,
		ProtocolBinding: protocol,
		// Stated rather than defaulted: the SDK sends this version's
		// A2A-Version on every request, and the gateway rejects a request whose
		// version is not the one its Agent exposes.
		ProtocolVersion: a2a.ProtocolVersion(a2aProtocolVersion),
	}

	ctx, cancel := context.WithTimeout(context.Background(), a2aSDKCallTimeout)
	defer cancel()

	client, err := a2aclient.NewFromEndpoints(ctx, []*a2a.AgentInterface{endpoint}, opts...)
	if err != nil {
		return fmt.Errorf("failed to create the %s A2A client %q for %s: %w", binding, name, url, err)
	}

	if existing, ok := a.clients[name]; ok {
		_ = existing.destroy()
	}
	a.clients[name] = &a2aSDKClient{name: name, binding: protocol, url: url, client: client}
	return nil
}

func (c *a2aSDKClient) destroy() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Destroy()
}

func (a *A2AClientSteps) destroyAll() {
	for name, client := range a.clients {
		_ = client.destroy()
		delete(a.clients, name)
	}
	a.taskID = ""
}

func (a *A2AClientSteps) require(name string) (*a2aSDKClient, error) {
	client, ok := a.clients[name]
	if !ok {
		return nil, fmt.Errorf("no A2A client named %q has been created in this scenario (created: %s)",
			name, a2aSDKClientNames(a.clients))
	}
	return client, nil
}

func a2aSDKClientNames(clients map[string]*a2aSDKClient) string {
	if len(clients) == 0 {
		return "none"
	}
	names := make([]string, 0, len(clients))
	for name := range clients {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// callContext carries the scenario's credentials to the SDK.
//
// The shared HTTP steps are where a scenario authenticates — "get a JWT token",
// "set header API-Key" — and those values have to reach a client that builds its
// own requests. Read at call time rather than at construction, so a token
// fetched after the client was created still applies. The SDK adds A2A-Version
// itself from the endpoint's declared protocol version.
func (a *A2AClientSteps) callContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	params := a2aclient.ServiceParams{}
	for _, header := range []string{"Authorization", "API-Key", "apikey", "X-API-Key"} {
		if value := a.httpSteps.Header(header); value != "" {
			params.Append(header, value)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return a2aclient.AttachServiceParams(ctx, params), cancel
}

// record stores an outcome and captures the task identifier it carries, so a
// later step can act on the task without the feature file naming an id the
// agent minted.
func (a *A2AClientSteps) record(client *a2aSDKClient, outcome *a2aSDKOutcome) error {
	client.last = outcome
	if outcome.Task != nil && outcome.Task.ID != "" {
		a.taskID = outcome.Task.ID
	}
	// A failed call is recorded, not returned as a step error: whether a call
	// was supposed to succeed is the assertion's business, and a step that
	// failed here could never express "this call must be refused".
	return nil
}

func (a *A2AClientSteps) requireTaskID() (a2a.TaskID, error) {
	if a.taskID == "" {
		return "", fmt.Errorf("no A2A task has been created in this scenario, so there is nothing to act on")
	}
	return a.taskID, nil
}

// ---- Operations ----

func (a *A2AClientSteps) sendMessage(name, text string, returnImmediately bool) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}

	request := &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text)),
	}
	if returnImmediately {
		// The only way to obtain a task that is genuinely still running: without
		// it a "plan ... slowly" request blocks for the whole hold, and every
		// operation that exists for a live task would be exercised against a
		// finished one.
		request.Config = &a2a.SendMessageConfig{ReturnImmediately: true}
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	result, err := client.client.SendMessage(ctx, request)
	outcome := &a2aSDKOutcome{Method: "SendMessage", Err: err}
	switch typed := result.(type) {
	case *a2a.Task:
		outcome.Task = typed
	case *a2a.Message:
		outcome.Message = typed
	}
	return a.record(client, outcome)
}

// streamMessage drains a streamed send to its terminal event.
func (a *A2AClientSteps) streamMessage(name, text string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}

	request := &a2a.SendMessageRequest{
		Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text)),
	}

	ctx, cancel := a.callContext(a2aSDKStreamTimeout)
	defer cancel()

	return a.record(client, a.drain(client.client.SendStreamingMessage(ctx, request), "SendStreamingMessage", 0))
}

func (a *A2AClientSteps) getTask(name string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	task, err := client.client.GetTask(ctx, &a2a.GetTaskRequest{ID: taskID})
	return a.record(client, &a2aSDKOutcome{Method: "GetTask", Err: err, Task: task})
}

func (a *A2AClientSteps) listTasks(name string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	response, err := client.client.ListTasks(ctx, &a2a.ListTasksRequest{})
	outcome := &a2aSDKOutcome{Method: "ListTasks", Err: err}
	if response != nil {
		outcome.Tasks = response.Tasks
	}
	return a.record(client, outcome)
}

func (a *A2AClientSteps) cancelTask(name string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	task, err := client.client.CancelTask(ctx, &a2a.CancelTaskRequest{ID: taskID})
	return a.record(client, &a2aSDKOutcome{Method: "CancelTask", Err: err, Task: task})
}

// subscribeToTask re-attaches to a running task and stops after a fixed number
// of events.
//
// Bounded by events rather than by the terminal state: the task it attaches to
// is deliberately long-running, so reading to close would make the scenario wait
// out the agent's whole hold — and a hold longer than the client timeout would
// fail on a read error rather than on an assertion, reporting a timeout where
// the finding is "the subscription worked".
func (a *A2AClientSteps) subscribeToTask(name string, count int) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKStreamTimeout)
	defer cancel()

	events := client.client.SubscribeToTask(ctx, &a2a.SubscribeToTaskRequest{ID: taskID})
	return a.record(client, a.drain(events, "SubscribeToTask", count))
}

func (a *A2AClientSteps) createPushConfig(name, configID string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	config, err := client.client.CreateTaskPushConfig(ctx, &a2a.PushConfig{
		TaskID: taskID,
		ID:     configID,
		URL:    "http://push-receiver.invalid/notifications",
	})
	return a.record(client, &a2aSDKOutcome{Method: "CreateTaskPushNotificationConfig", Err: err, PushConfig: config})
}

func (a *A2AClientSteps) getPushConfig(name, configID string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	config, err := client.client.GetTaskPushConfig(ctx, &a2a.GetTaskPushConfigRequest{TaskID: taskID, ID: configID})
	return a.record(client, &a2aSDKOutcome{Method: "GetTaskPushNotificationConfig", Err: err, PushConfig: config})
}

func (a *A2AClientSteps) listPushConfigs(name string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	configs, err := client.client.ListTaskPushConfigs(ctx, &a2a.ListTaskPushConfigRequest{TaskID: taskID})
	return a.record(client, &a2aSDKOutcome{Method: "ListTaskPushNotificationConfigs", Err: err, PushConfigs: configs})
}

func (a *A2AClientSteps) deletePushConfig(name, configID string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	err = client.client.DeleteTaskPushConfig(ctx, &a2a.DeleteTaskPushConfigRequest{TaskID: taskID, ID: configID})
	return a.record(client, &a2aSDKOutcome{Method: "DeleteTaskPushNotificationConfig", Err: err})
}

func (a *A2AClientSteps) getExtendedCard(name string) error {
	client, err := a.require(name)
	if err != nil {
		return err
	}

	ctx, cancel := a.callContext(a2aSDKCallTimeout)
	defer cancel()

	card, err := client.client.GetExtendedAgentCard(ctx, &a2a.GetExtendedAgentCardRequest{})
	return a.record(client, &a2aSDKOutcome{Method: "GetExtendedAgentCard", Err: err, Card: card})
}

// drain consumes an SDK event iterator, timestamping each event as it is
// yielded.
//
// maxEvents of 0 reads to the end of the stream; a positive value stops as soon
// as that many events have arrived, which the deferred close in the SDK turns
// into an ordinary client disconnect.
func (a *A2AClientSteps) drain(events func(func(a2a.Event, error) bool), method string, maxEvents int) *a2aSDKOutcome {
	outcome := &a2aSDKOutcome{Method: method}
	start := time.Now()

	for event, err := range events {
		if err != nil {
			// A read error after the requested number of events is what
			// abandoning a still-open stream looks like from this side, not a
			// failure of the call.
			if maxEvents == 0 || len(outcome.Events) < maxEvents {
				outcome.Err = err
			}
			break
		}
		outcome.Events = append(outcome.Events, a2aSDKEvent{Event: event, Offset: time.Since(start)})
		if info := event.TaskInfo(); info.TaskID != "" {
			a.taskID = info.TaskID
		}
		if maxEvents > 0 && len(outcome.Events) >= maxEvents {
			break
		}
	}

	outcome.StreamDuration = time.Since(start)
	return outcome
}

// ---- Assertions ----

func (a *A2AClientSteps) outcome(name string) (*a2aSDKClient, *a2aSDKOutcome, error) {
	client, err := a.require(name)
	if err != nil {
		return nil, nil, err
	}
	if client.last == nil {
		return nil, nil, fmt.Errorf("the A2A client %q has not made a call in this scenario", name)
	}
	return client, client.last, nil
}

// succeeded returns the outcome of a call that was expected to work, failing
// with the SDK's own error when it did not.
func (a *A2AClientSteps) succeeded(name string) (*a2aSDKClient, *a2aSDKOutcome, error) {
	client, outcome, err := a.outcome(name)
	if err != nil {
		return nil, nil, err
	}
	if outcome.Err != nil {
		return nil, nil, fmt.Errorf("%s over %s through %s failed: %w",
			outcome.Method, client.binding, client.url, outcome.Err)
	}
	return client, outcome, nil
}

func (a *A2AClientSteps) callShouldSucceed(name string) error {
	_, _, err := a.succeeded(name)
	return err
}

func (a *A2AClientSteps) callShouldFail(name string) error {
	client, outcome, err := a.outcome(name)
	if err != nil {
		return err
	}
	if outcome.Err == nil {
		return fmt.Errorf("expected %s over %s through %s to be refused, but it succeeded",
			outcome.Method, client.binding, client.url)
	}
	return nil
}

func (a *A2AClientSteps) taskStateShouldBe(name, expected string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if outcome.Task == nil {
		return fmt.Errorf("%s returned no task (a %s result carries no task state)",
			outcome.Method, a2aSDKResultKind(outcome))
	}
	if string(outcome.Task.Status.State) != expected {
		return fmt.Errorf("expected task %s to be in state %q, got %q",
			outcome.Task.ID, expected, outcome.Task.Status.State)
	}
	return nil
}

// taskShouldBeRunning asserts a task has not reached a terminal state.
//
// Deliberately not an equality check against `submitted` or `working`: a task
// created with returnImmediately is one or the other depending on how far the
// agent got before answering, and pinning either would make the scenario fail
// on timing rather than on behaviour. What the operations that follow need is
// only that the task is still live.
func (a *A2AClientSteps) taskShouldBeRunning(name string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if outcome.Task == nil {
		return fmt.Errorf("%s returned no task (a %s result carries no task state)",
			outcome.Method, a2aSDKResultKind(outcome))
	}
	if outcome.Task.Status.State.Terminal() {
		return fmt.Errorf("expected task %s to still be running, but it is in terminal state %q",
			outcome.Task.ID, outcome.Task.Status.State)
	}
	return nil
}

func (a *A2AClientSteps) artifactShouldContain(name, expected string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	text, err := a2aSDKArtifactText(outcome)
	if err != nil {
		return err
	}
	if !strings.Contains(text, expected) {
		return fmt.Errorf("expected an artifact containing %q, got:\n%s", expected, truncateA2A(text, 600))
	}
	return nil
}

// artifactsShouldMatch is the positive half of cross-transport equivalence:
// the same operation, invoked through two clients bound to two different
// gateway routes, produced the same protocol result.
func (a *A2AClientSteps) artifactsShouldMatch(first, second string) error {
	_, firstOutcome, err := a.succeeded(first)
	if err != nil {
		return err
	}
	_, secondOutcome, err := a.succeeded(second)
	if err != nil {
		return err
	}

	firstText, err := a2aSDKArtifactText(firstOutcome)
	if err != nil {
		return fmt.Errorf("client %q: %w", first, err)
	}
	secondText, err := a2aSDKArtifactText(secondOutcome)
	if err != nil {
		return fmt.Errorf("client %q: %w", second, err)
	}

	if firstText != secondText {
		return fmt.Errorf(
			"the two bindings produced different artifacts\n%s: %s\n%s: %s",
			first, truncateA2A(firstText, 400), second, truncateA2A(secondText, 400))
	}
	return nil
}

func (a *A2AClientSteps) taskListShouldContainTask(name string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	taskID, err := a.requireTaskID()
	if err != nil {
		return err
	}
	for _, task := range outcome.Tasks {
		if task != nil && task.ID == taskID {
			return nil
		}
	}
	return fmt.Errorf("task %s is absent from the %d listed task(s): %s",
		taskID, len(outcome.Tasks), a2aSDKTaskIDs(outcome.Tasks))
}

func (a *A2AClientSteps) pushConfigShouldBe(name, expected string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if outcome.PushConfig == nil {
		return fmt.Errorf("%s returned no push notification config", outcome.Method)
	}
	if outcome.PushConfig.ID != expected {
		return fmt.Errorf("expected push notification config %q, got %q", expected, outcome.PushConfig.ID)
	}
	return nil
}

func (a *A2AClientSteps) pushConfigCountShouldBe(name string, expected int) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if len(outcome.PushConfigs) != expected {
		return fmt.Errorf("expected %d push notification config(s), got %d", expected, len(outcome.PushConfigs))
	}
	return nil
}

func (a *A2AClientSteps) cardNameShouldBe(name, expected string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if outcome.Card == nil {
		return fmt.Errorf("%s returned no Agent Card", outcome.Method)
	}
	if outcome.Card.Name != expected {
		return fmt.Errorf("expected an Agent Card named %q, got %q", expected, outcome.Card.Name)
	}
	return nil
}

// cardSkill asserts a skill's presence or absence in a received card.
//
// Which card a response carries is what the extended-card scenarios turn on,
// and a skill id is the cheapest unambiguous marker of it: the fixture's
// extended card carries one its public card does not, and a managed card
// carries one neither of them has.
func (a *A2AClientSteps) cardSkill(name, skillID string, expectPresent bool) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if outcome.Card == nil {
		return fmt.Errorf("%s returned no Agent Card", outcome.Method)
	}

	present := false
	ids := make([]string, 0, len(outcome.Card.Skills))
	for _, skill := range outcome.Card.Skills {
		ids = append(ids, skill.ID)
		if skill.ID == skillID {
			present = true
		}
	}

	if present != expectPresent {
		if expectPresent {
			return fmt.Errorf("expected the received Agent Card to declare skill %q, got skills [%s]",
				skillID, strings.Join(ids, ", "))
		}
		return fmt.Errorf("the received Agent Card declares skill %q, which it must not: skills [%s]",
			skillID, strings.Join(ids, ", "))
	}
	return nil
}

func (a *A2AClientSteps) streamEventCountAtLeast(name string, expected int) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if len(outcome.Events) < expected {
		return fmt.Errorf("expected at least %d stream event(s), got %d: %s",
			expected, len(outcome.Events), a2aSDKStreamSummary(outcome))
	}
	return nil
}

func (a *A2AClientSteps) streamShouldEndInState(name, expected string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if len(outcome.Events) == 0 {
		return fmt.Errorf("the stream delivered no events")
	}

	last := outcome.Events[len(outcome.Events)-1].Event
	state, ok := a2aSDKEventState(last)
	if !ok {
		return fmt.Errorf("the last stream event is a %T, which carries no task state: %s",
			last, a2aSDKStreamSummary(outcome))
	}
	if string(state) != expected {
		return fmt.Errorf("expected the stream to end in state %q, got %q: %s",
			expected, state, a2aSDKStreamSummary(outcome))
	}
	return nil
}

// streamFirstBeforeLast is what separates a stream from a buffered response in
// SSE clothing: a response held whole and released at the end yields every event
// at the same instant, and every content assertion over it still passes.
func (a *A2AClientSteps) streamFirstBeforeLast(name string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	if len(outcome.Events) < 2 {
		return fmt.Errorf("need at least 2 events to compare arrival times, got %d: %s",
			len(outcome.Events), a2aSDKStreamSummary(outcome))
	}

	first := outcome.Events[0].Offset
	last := outcome.Events[len(outcome.Events)-1].Offset
	if first >= last {
		return fmt.Errorf(
			"first event did not arrive before the last: first=%s last=%s — the response was delivered "+
				"as one buffered unit rather than streamed: %s",
			first, last, a2aSDKStreamSummary(outcome))
	}
	return nil
}

func (a *A2AClientSteps) streamShouldContain(name, expected string) error {
	_, outcome, err := a.succeeded(name)
	if err != nil {
		return err
	}
	for _, event := range outcome.Events {
		if strings.Contains(a2aSDKEventText(event.Event), expected) {
			return nil
		}
	}
	return fmt.Errorf("no stream event contained %q: %s", expected, a2aSDKStreamSummary(outcome))
}

// ---- Helpers ----

// a2aSDKArtifactText concatenates the text parts of every artifact an outcome
// carries, whether it arrived as a task or as streamed artifact events.
func a2aSDKArtifactText(outcome *a2aSDKOutcome) (string, error) {
	var parts []string

	if outcome.Task != nil {
		for _, artifact := range outcome.Task.Artifacts {
			parts = append(parts, a2aSDKPartsText(artifact.Parts))
		}
	}
	for _, event := range outcome.Events {
		if update, ok := event.Event.(*a2a.TaskArtifactUpdateEvent); ok && update.Artifact != nil {
			parts = append(parts, a2aSDKPartsText(update.Artifact.Parts))
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("%s produced no artifact (result: %s)", outcome.Method, a2aSDKResultKind(outcome))
	}
	return strings.Join(parts, "\n"), nil
}

func a2aSDKPartsText(parts a2a.ContentParts) string {
	var texts []string
	for _, part := range parts {
		if part == nil {
			continue
		}
		if text := part.Text(); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

// a2aSDKEventState reads the task state out of whichever event type carries
// one.
func a2aSDKEventState(event a2a.Event) (a2a.TaskState, bool) {
	switch typed := event.(type) {
	case *a2a.TaskStatusUpdateEvent:
		return typed.Status.State, true
	case *a2a.Task:
		return typed.Status.State, true
	default:
		return "", false
	}
}

// a2aSDKEventText renders an event as the text a scenario would assert on.
func a2aSDKEventText(event a2a.Event) string {
	switch typed := event.(type) {
	case *a2a.TaskStatusUpdateEvent:
		if typed.Status.Message != nil {
			return string(typed.Status.State) + " " + a2aSDKPartsText(typed.Status.Message.Parts)
		}
		return string(typed.Status.State)
	case *a2a.TaskArtifactUpdateEvent:
		if typed.Artifact != nil {
			return a2aSDKPartsText(typed.Artifact.Parts)
		}
		return ""
	case *a2a.Task:
		return string(typed.Status.State)
	case *a2a.Message:
		return a2aSDKPartsText(typed.Parts)
	default:
		return fmt.Sprintf("%T", event)
	}
}

func a2aSDKResultKind(outcome *a2aSDKOutcome) string {
	switch {
	case outcome.Task != nil:
		return "task"
	case outcome.Message != nil:
		return "message"
	case outcome.Card != nil:
		return "agent card"
	case outcome.PushConfig != nil:
		return "push notification config"
	case len(outcome.PushConfigs) > 0:
		return "push notification config list"
	case len(outcome.Tasks) > 0:
		return "task list"
	case len(outcome.Events) > 0:
		return fmt.Sprintf("%d stream event(s)", len(outcome.Events))
	default:
		return "no payload"
	}
}

func a2aSDKTaskIDs(tasks []*a2a.Task) string {
	if len(tasks) == 0 {
		return "none"
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			ids = append(ids, string(task.ID))
		}
	}
	return strings.Join(ids, ", ")
}

func a2aSDKStreamSummary(outcome *a2aSDKOutcome) string {
	if len(outcome.Events) == 0 {
		return "stream delivered no events"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d event(s) over %s:", len(outcome.Events), outcome.StreamDuration.Round(time.Millisecond))
	for i, event := range outcome.Events {
		fmt.Fprintf(&b, "\n  [%d] +%s %T %s", i, event.Offset.Round(time.Millisecond), event.Event,
			truncateA2A(a2aSDKEventText(event.Event), 200))
	}
	return b.String()
}
