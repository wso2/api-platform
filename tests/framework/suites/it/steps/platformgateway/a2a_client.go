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
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/a2ax"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// A2A steps that speak the protocol through the official Go SDK.
//
// These own the conformant path only: typed requests, typed results and semantic assertions over
// them. Requests a conformant client cannot produce — a missing or contradictory protocol
// version, an unknown JSON-RPC method, exact status codes and framing — go through the ordinary
// data-plane funnel instead.
//
// SDK calls are not published as responses: the SDK builds and reads its own requests, and a
// typed result re-encoded into the published response would be bytes the test wrote, not bytes
// the gateway returned. Every SDK call therefore clears the published response, so a generic
// response assertion placed after one fails instead of passing against an earlier request.

// keyA2ASession holds the scenario's SDK clients and the task they are working with.
const keyA2ASession = "a2aSession"

// a2aProtocolVersion is the A2A version every SDK client states. Each Agent exposes exactly one
// version, and every Agent these scenarios deploy exposes this one.
const a2aProtocolVersion = "1.0"

// a2aPushReceiverURL is the notification URL registered on push configs. The configs are only
// created, read, listed and deleted, never delivered, so the host deliberately cannot resolve.
const a2aPushReceiverURL = "http://push-receiver.invalid/notifications"

// a2aCredentialHeaders are the scenario headers an SDK call carries. Only credentials travel:
// the SDK sets its own Content-Type, Accept and A2A-Version, and a scenario-wide value of one of
// those would contradict what the SDK negotiates.
var a2aCredentialHeaders = []string{"Authorization", "API-Key", "apikey", "X-API-Key"}

// a2aSession is one scenario's SDK state. The task identifier is shared across clients on
// purpose: a task created over one binding and read over the other is exactly the
// cross-transport claim these steps exist to check.
type a2aSession struct {
	clients map[string]*a2aSessionClient
	taskID  string
}

type a2aSessionClient struct {
	client *a2ax.Client
	last   *a2ax.Outcome
}

func newA2ASession() *a2aSession {
	return &a2aSession{clients: map[string]*a2aSessionClient{}}
}

// close releases every client and reports the first failure.
func (s *a2aSession) close() error {
	var first error
	for _, name := range s.names() {
		if err := s.clients[name].client.Close(); err != nil && first == nil {
			first = fmt.Errorf("closing A2A client %q: %w", name, err)
		}
		delete(s.clients, name)
	}
	s.taskID = ""
	return first
}

func (s *a2aSession) names() []string {
	names := make([]string, 0, len(s.clients))
	for name := range s.clients {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *a2aSession) client(name string) (*a2aSessionClient, error) {
	client, ok := s.clients[name]
	if !ok {
		created := "none"
		if names := s.names(); len(names) > 0 {
			created = strings.Join(names, ", ")
		}
		return nil, fmt.Errorf("no A2A client named %q has been created in this scenario (created: %s)", name, created)
	}
	return client, nil
}

// put stores a client under a name, closing any client the name previously held.
func (s *a2aSession) put(name string, client *a2ax.Client) error {
	if existing, ok := s.clients[name]; ok {
		if err := existing.client.Close(); err != nil {
			_ = client.Close()
			return fmt.Errorf("replacing A2A client %q: %w", name, err)
		}
	}
	s.clients[name] = &a2aSessionClient{client: client}
	return nil
}

// record stores an outcome and captures the task it names, so a later step can act on the task
// without the feature naming an identifier the agent minted.
func (s *a2aSession) record(client *a2aSessionClient, outcome *a2ax.Outcome) {
	client.last = outcome
	if id := outcome.TaskID(); id != "" {
		s.taskID = id
	}
}

func (s *a2aSession) requireTaskID() (string, error) {
	if s.taskID == "" {
		return "", fmt.Errorf("no A2A task has been created in this scenario, so there is nothing to act on")
	}
	return s.taskID, nil
}

// outcome returns the most recent outcome of a named client.
func (s *a2aSession) outcome(name string) (*a2aSessionClient, *a2ax.Outcome, error) {
	client, err := s.client(name)
	if err != nil {
		return nil, nil, err
	}
	if client.last == nil {
		return nil, nil, fmt.Errorf("the A2A client %q has not made a call in this scenario", name)
	}
	return client, client.last, nil
}

// succeeded returns the outcome of a call that was expected to work, failing with the SDK's own
// error when it did not.
func (s *a2aSession) succeeded(name string) (*a2ax.Outcome, error) {
	client, outcome, err := s.outcome(name)
	if err != nil {
		return nil, err
	}
	if outcome.Err != nil {
		return nil, fmt.Errorf("%s over %s through %s failed: %w",
			outcome.Method, client.client.Binding(), client.client.URL(), outcome.Err)
	}
	return outcome, nil
}

func a2aSessionOf(ctx context.Context) (*a2aSession, error) {
	value, ok := tcontext.Get(ctx, keyA2ASession)
	if !ok {
		return nil, fmt.Errorf("no A2A session is open for this scenario")
	}
	session, ok := value.(*a2aSession)
	if !ok || session == nil {
		return nil, fmt.Errorf("the A2A session is %T, not an A2A session", value)
	}
	return session, nil
}

// a2aCredentials selects the credential headers from the scenario headers.
func a2aCredentials(headers map[string]string) a2ax.Headers {
	selected := a2ax.Headers{}
	for name, value := range headers {
		for _, credential := range a2aCredentialHeaders {
			if strings.EqualFold(name, credential) && value != "" {
				selected[credential] = value
			}
		}
	}
	return selected
}

func (g *Gateway) registerA2AClientSteps(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return ctx, tcontext.Set(ctx, keyA2ASession, newA2ASession())
	})
	// Clients hold connections, so they are released when the scenario ends, failed or not.
	sc.After(func(ctx context.Context, _ *godog.Scenario, scenarioErr error) (context.Context, error) {
		session, err := a2aSessionOf(ctx)
		if err != nil {
			return ctx, nil //nolint:nilerr // a scenario that never opened a session has nothing to release
		}
		if closeErr := session.close(); closeErr != nil && scenarioErr == nil {
			return ctx, closeErr
		}
		return ctx, nil
	})

	sc.Step(`^I create an A2A client "([^"]*)" for the "(JSONRPC|HTTP\+JSON)" binding at "([^"]*)"$`, g.createA2AClient)
	sc.Step(`^I create an A2A client "([^"]*)" for the "(JSONRPC|HTTP\+JSON)" binding from the Agent Card at "([^"]*)"$`,
		g.createA2AClientFromCard)
	sc.Step(`^the A2A client "([^"]*)" should be talking to "([^"]*)"$`, g.a2aClientTalksTo)

	sc.Step(`^the A2A client "([^"]*)" sends the message "([^"]*)"( and returns immediately)?$`, g.a2aSendMessage)
	sc.Step(`^the A2A client "([^"]*)" streams the message "([^"]*)"$`, g.a2aStreamMessage)
	sc.Step(`^the A2A client "([^"]*)" (gets|cancels) the task$`, g.a2aActOnTask)
	sc.Step(`^the A2A client "([^"]*)" lists tasks$`, g.a2aListTasks)
	sc.Step(`^the A2A client "([^"]*)" subscribes to the task and reads (\d+) events?$`, g.a2aSubscribe)
	sc.Step(`^the A2A client "([^"]*)" (creates|gets|deletes) the push notification config "([^"]*)" for the task$`,
		g.a2aPushConfig)
	sc.Step(`^the A2A client "([^"]*)" lists push notification configs for the task$`, g.a2aListPushConfigs)
	sc.Step(`^the A2A client "([^"]*)" gets the extended Agent Card$`, g.a2aExtendedCard)

	sc.Step(`^the A2A client "([^"]*)" call should have (succeeded|failed)$`, g.a2aCallResult)
	sc.Step(`^the A2A client "([^"]*)" should have received a task in state "([^"]*)"$`, g.a2aTaskState)
	sc.Step(`^the A2A client "([^"]*)" should have received a task that is still running$`, g.a2aTaskRunning)
	sc.Step(`^the A2A client "([^"]*)" should have received an artifact containing "([^"]*)"$`, g.a2aArtifactContains)
	sc.Step(`^the A2A clients "([^"]*)" and "([^"]*)" should have received the same artifact$`, g.a2aSameArtifact)
	sc.Step(`^the A2A client "([^"]*)" should have received a task list containing the task$`, g.a2aTaskListed)
	sc.Step(`^the A2A client "([^"]*)" should have received a push config "([^"]*)"$`, g.a2aPushConfigIs)
	sc.Step(`^the A2A client "([^"]*)" should have received (\d+) push configs?$`, g.a2aPushConfigCount)
	sc.Step(`^the A2A client "([^"]*)" should have received an Agent Card named "([^"]*)"$`, g.a2aCardName)
	sc.Step(`^the A2A client "([^"]*)" should have received an Agent Card (with|without) the skill "([^"]*)"$`, g.a2aCardSkill)
	sc.Step(`^the A2A client "([^"]*)" should have received at least (\d+) stream events?$`, g.a2aStreamEventCount)
	sc.Step(`^the A2A client "([^"]*)" stream should end in state "([^"]*)"$`, g.a2aStreamEndState)
	sc.Step(`^the A2A client "([^"]*)" stream's first event should arrive before its last event$`, g.a2aStreamPaced)
	sc.Step(`^the A2A client "([^"]*)" should have received a stream event containing "([^"]*)"$`, g.a2aStreamContains)
}

// a2aGatewayEndpoint resolves a gateway-relative path to the data-plane URL a client dials.
//
// Only gateway paths are accepted. A client is never pointed at an arbitrary URL, because one
// that reached the agent directly would pass while exercising no gateway route at all.
func (g *Gateway) a2aGatewayEndpoint(ctx context.Context, path string) (string, error) {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return "", err
	}
	resolved = strings.TrimSpace(resolved)
	if !strings.HasPrefix(resolved, "/") {
		return "", fmt.Errorf("an A2A client endpoint must be a gateway path starting with \"/\", got %q", resolved)
	}
	return g.gatewayURL(resolved)
}

func (g *Gateway) createA2AClient(ctx context.Context, name, binding, path string) error {
	endpoint, err := g.a2aGatewayEndpoint(ctx, path)
	if err != nil {
		return err
	}
	return g.openA2AClient(ctx, name, binding, endpoint)
}

func (g *Gateway) openA2AClient(ctx context.Context, name, binding, endpoint string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("an A2A client needs a name")
	}
	session, err := a2aSessionOf(ctx)
	if err != nil {
		return err
	}
	client, err := a2ax.NewClient(ctx, binding, endpoint, a2ax.Options{ProtocolVersion: a2aProtocolVersion})
	if err != nil {
		return err
	}
	return session.put(name, client)
}

// createA2AClientFromCard bootstraps a client the way a real one does: fetch the Agent Card,
// pick the interface for the binding, and dial the URL it advertises.
//
// This is the only step that follows a card, and it is what makes interface URL rewriting
// testable end to end. The card is fetched through the data-plane funnel with the scenario's
// headers, so its response stays assertable.
func (g *Gateway) createA2AClientFromCard(ctx context.Context, name, binding, cardPath string) error {
	cardURL, err := g.a2aGatewayEndpoint(ctx, cardPath)
	if err != nil {
		return err
	}
	resp, err := g.funnel.Get(ctx, cardURL, g.scenarioHeaders(ctx))
	if err != nil {
		return fmt.Errorf("fetching the Agent Card at %s: %w", cardURL, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("fetching the Agent Card at %s: %s", cardURL, resp.Describe())
	}
	endpoint, err := a2ax.InterfaceURL(resp.Body, binding)
	if err != nil {
		return fmt.Errorf("the Agent Card at %s: %w", cardURL, err)
	}
	return g.openA2AClient(ctx, name, binding, endpoint)
}

// a2aClientTalksTo asserts which endpoint a client dials. For a card-derived client it names
// the URL the card advertised, so a rewrite that produced the upstream's address fails here
// with that address rather than later as an opaque connection error.
func (g *Gateway) a2aClientTalksTo(ctx context.Context, name, path string) error {
	session, err := a2aSessionOf(ctx)
	if err != nil {
		return err
	}
	client, err := session.client(name)
	if err != nil {
		return err
	}
	want, err := g.a2aGatewayEndpoint(ctx, path)
	if err != nil {
		return err
	}
	if got := client.client.URL(); got != want {
		return fmt.Errorf("A2A client %q is talking to %s, want %s", name, got, want)
	}
	return nil
}

// a2aCall runs one SDK call for a named client and records its outcome.
func (g *Gateway) a2aCall(
	ctx context.Context, name string, needsTask bool,
	call func(client *a2ax.Client, headers a2ax.Headers, taskID string) *a2ax.Outcome,
) error {
	session, err := a2aSessionOf(ctx)
	if err != nil {
		return err
	}
	client, err := session.client(name)
	if err != nil {
		return err
	}
	taskID := ""
	if needsTask {
		if taskID, err = session.requireTaskID(); err != nil {
			return err
		}
	}
	httpx.ClearPublished(ctx)
	session.record(client, call(client.client, a2aCredentials(g.scenarioHeaders(ctx)), taskID))
	return nil
}

func (g *Gateway) a2aSendMessage(ctx context.Context, name, text, immediately string) error {
	resolved, err := stepscommon.Expand(ctx, text)
	if err != nil {
		return err
	}
	return g.a2aCall(ctx, name, false, func(client *a2ax.Client, headers a2ax.Headers, _ string) *a2ax.Outcome {
		return client.SendMessage(ctx, headers, resolved, immediately != "")
	})
}

func (g *Gateway) a2aStreamMessage(ctx context.Context, name, text string) error {
	resolved, err := stepscommon.Expand(ctx, text)
	if err != nil {
		return err
	}
	return g.a2aCall(ctx, name, false, func(client *a2ax.Client, headers a2ax.Headers, _ string) *a2ax.Outcome {
		return client.StreamMessage(ctx, headers, resolved)
	})
}

func (g *Gateway) a2aActOnTask(ctx context.Context, name, action string) error {
	return g.a2aCall(ctx, name, true, func(client *a2ax.Client, headers a2ax.Headers, taskID string) *a2ax.Outcome {
		if action == "cancels" {
			return client.CancelTask(ctx, headers, taskID)
		}
		return client.GetTask(ctx, headers, taskID)
	})
}

func (g *Gateway) a2aListTasks(ctx context.Context, name string) error {
	return g.a2aCall(ctx, name, false, func(client *a2ax.Client, headers a2ax.Headers, _ string) *a2ax.Outcome {
		return client.ListTasks(ctx, headers)
	})
}

func (g *Gateway) a2aSubscribe(ctx context.Context, name string, count int) error {
	if count <= 0 {
		return fmt.Errorf("the number of events to read must be positive, got %d", count)
	}
	return g.a2aCall(ctx, name, true, func(client *a2ax.Client, headers a2ax.Headers, taskID string) *a2ax.Outcome {
		return client.SubscribeToTask(ctx, headers, taskID, count)
	})
}

func (g *Gateway) a2aPushConfig(ctx context.Context, name, action, configID string) error {
	resolved, err := stepscommon.Expand(ctx, configID)
	if err != nil {
		return err
	}
	return g.a2aCall(ctx, name, true, func(client *a2ax.Client, headers a2ax.Headers, taskID string) *a2ax.Outcome {
		switch action {
		case "creates":
			return client.CreatePushConfig(ctx, headers, taskID, resolved, a2aPushReceiverURL)
		case "deletes":
			return client.DeletePushConfig(ctx, headers, taskID, resolved)
		default:
			return client.GetPushConfig(ctx, headers, taskID, resolved)
		}
	})
}

func (g *Gateway) a2aListPushConfigs(ctx context.Context, name string) error {
	return g.a2aCall(ctx, name, true, func(client *a2ax.Client, headers a2ax.Headers, taskID string) *a2ax.Outcome {
		return client.ListPushConfigs(ctx, headers, taskID)
	})
}

func (g *Gateway) a2aExtendedCard(ctx context.Context, name string) error {
	return g.a2aCall(ctx, name, false, func(client *a2ax.Client, headers a2ax.Headers, _ string) *a2ax.Outcome {
		return client.GetExtendedCard(ctx, headers)
	})
}

// a2aSucceeded returns the named client's last outcome, failing unless the call succeeded.
func a2aSucceeded(ctx context.Context, name string) (*a2ax.Outcome, error) {
	session, err := a2aSessionOf(ctx)
	if err != nil {
		return nil, err
	}
	return session.succeeded(name)
}

func (g *Gateway) a2aCallResult(ctx context.Context, name, result string) error {
	session, err := a2aSessionOf(ctx)
	if err != nil {
		return err
	}
	if result == "succeeded" {
		_, err = session.succeeded(name)
		return err
	}
	client, outcome, err := session.outcome(name)
	if err != nil {
		return err
	}
	if outcome.Err == nil {
		return fmt.Errorf("expected %s over %s through %s to be refused, but it succeeded",
			outcome.Method, client.client.Binding(), client.client.URL())
	}
	return nil
}

func a2aTask(outcome *a2ax.Outcome) (*a2ax.Task, error) {
	if outcome.Task == nil {
		return nil, fmt.Errorf("%s returned no task (a %s result carries no task state)", outcome.Method, outcome.Kind())
	}
	return outcome.Task, nil
}

func (g *Gateway) a2aTaskState(ctx context.Context, name, want string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	task, err := a2aTask(outcome)
	if err != nil {
		return err
	}
	if task.State != want {
		return fmt.Errorf("expected task %s to be in state %q, got %q", task.ID, want, task.State)
	}
	return nil
}

// a2aTaskRunning asserts a task has not reached a terminal state. Deliberately not an equality
// check against submitted or working: a task created with returnImmediately is either,
// depending on how far the agent got before answering.
func (g *Gateway) a2aTaskRunning(ctx context.Context, name string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	task, err := a2aTask(outcome)
	if err != nil {
		return err
	}
	if task.Terminal {
		return fmt.Errorf("expected task %s to still be running, but it is in terminal state %q", task.ID, task.State)
	}
	return nil
}

func (g *Gateway) a2aArtifactContains(ctx context.Context, name, want string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	text, err := outcome.ArtifactText()
	if err != nil {
		return err
	}
	if !strings.Contains(text, resolved) {
		return fmt.Errorf("expected an artifact containing %q, got:\n%s", resolved, text)
	}
	return nil
}

// a2aSameArtifact is the positive half of cross-transport equivalence: the same operation,
// invoked through two clients bound to two gateway routes, produced the same result.
func (g *Gateway) a2aSameArtifact(ctx context.Context, first, second string) error {
	texts := make([]string, 0, 2)
	for _, name := range []string{first, second} {
		outcome, err := a2aSucceeded(ctx, name)
		if err != nil {
			return err
		}
		text, err := outcome.ArtifactText()
		if err != nil {
			return fmt.Errorf("client %q: %w", name, err)
		}
		texts = append(texts, text)
	}
	if texts[0] != texts[1] {
		return fmt.Errorf("the two bindings produced different artifacts\n%s: %s\n%s: %s", first, texts[0], second, texts[1])
	}
	return nil
}

func (g *Gateway) a2aTaskListed(ctx context.Context, name string) error {
	session, err := a2aSessionOf(ctx)
	if err != nil {
		return err
	}
	outcome, err := session.succeeded(name)
	if err != nil {
		return err
	}
	taskID, err := session.requireTaskID()
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(outcome.Tasks))
	for _, task := range outcome.Tasks {
		if task.ID == taskID {
			return nil
		}
		ids = append(ids, task.ID)
	}
	return fmt.Errorf("task %s is absent from the %d listed task(s): %v", taskID, len(ids), ids)
}

func (g *Gateway) a2aPushConfigIs(ctx context.Context, name, want string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	if outcome.PushConfig == nil {
		return fmt.Errorf("%s returned no push notification config", outcome.Method)
	}
	if outcome.PushConfig.ID != resolved {
		return fmt.Errorf("expected push notification config %q, got %q", resolved, outcome.PushConfig.ID)
	}
	return nil
}

func (g *Gateway) a2aPushConfigCount(ctx context.Context, name string, want int) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	if outcome.PushConfigs == nil {
		return fmt.Errorf("%s returned no push notification config list (a %s result)", outcome.Method, outcome.Kind())
	}
	if len(outcome.PushConfigs) != want {
		return fmt.Errorf("expected %d push notification config(s), got %d", want, len(outcome.PushConfigs))
	}
	return nil
}

func a2aCard(ctx context.Context, name string) (*a2ax.Card, error) {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return nil, err
	}
	if outcome.Card == nil {
		return nil, fmt.Errorf("%s returned no Agent Card", outcome.Method)
	}
	return outcome.Card, nil
}

func (g *Gateway) a2aCardName(ctx context.Context, name, want string) error {
	card, err := a2aCard(ctx, name)
	if err != nil {
		return err
	}
	if card.Name != want {
		return fmt.Errorf("expected an Agent Card named %q, got %q", want, card.Name)
	}
	return nil
}

// a2aCardSkill asserts a skill's presence or absence. A skill id is the cheapest unambiguous
// marker of which card a response carries: the agent's extended card declares one its public
// card does not.
func (g *Gateway) a2aCardSkill(ctx context.Context, name, presence, skill string) error {
	card, err := a2aCard(ctx, name)
	if err != nil {
		return err
	}
	present := false
	for _, id := range card.Skills {
		if id == skill {
			present = true
		}
	}
	switch {
	case presence == "with" && !present:
		return fmt.Errorf("expected the received Agent Card to declare skill %q, got skills %v", skill, card.Skills)
	case presence == "without" && present:
		return fmt.Errorf("the received Agent Card declares skill %q, which it must not: skills %v", skill, card.Skills)
	}
	return nil
}

func (g *Gateway) a2aStreamEventCount(ctx context.Context, name string, want int) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	if len(outcome.Events) < want {
		return fmt.Errorf("expected at least %d stream event(s), got %d: %s", want, len(outcome.Events), outcome.Summary())
	}
	return nil
}

func (g *Gateway) a2aStreamEndState(ctx context.Context, name, want string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	state, err := outcome.FinalState()
	if err != nil {
		return err
	}
	if state != want {
		return fmt.Errorf("expected the stream to end in state %q, got %q: %s", want, state, outcome.Summary())
	}
	return nil
}

// a2aStreamPaced separates a stream from a buffered response in SSE framing: a response held
// whole and released at the end yields every event at the same instant.
func (g *Gateway) a2aStreamPaced(ctx context.Context, name string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	if len(outcome.Events) < 2 {
		return fmt.Errorf("need at least 2 events to compare arrival times, got %d: %s", len(outcome.Events), outcome.Summary())
	}
	first, last := outcome.Events[0].Offset, outcome.Events[len(outcome.Events)-1].Offset
	if first >= last {
		return fmt.Errorf("first event did not arrive before the last: first=%s last=%s; the response was "+
			"delivered as one buffered unit rather than streamed: %s", first, last, outcome.Summary())
	}
	return nil
}

func (g *Gateway) a2aStreamContains(ctx context.Context, name, want string) error {
	outcome, err := a2aSucceeded(ctx, name)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	for _, event := range outcome.Events {
		if strings.Contains(event.Text, resolved) {
			return nil
		}
	}
	return fmt.Errorf("no stream event contained %q: %s", resolved, outcome.Summary())
}
