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

package a2ax

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/push"
	"github.com/stretchr/testify/require"
)

// planner is an in-process agent built on the reference server SDK. A message containing
// "slowly" parks its task in working until the call ends; any other completes with one artifact.
func planner() a2asrv.AgentExecutor {
	return a2asrv.AgentExecutorFunc(func(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
		return func(yield func(a2a.Event, error) bool) {
			text := ""
			if execCtx.Message != nil {
				text = partsText(execCtx.Message.Parts)
			}
			if execCtx.StoredTask == nil {
				if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
					return
				}
			}
			if !yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil), nil) {
				return
			}
			if strings.Contains(text, "slowly") {
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()
				deadline := time.After(5 * time.Second)
				for {
					select {
					case <-ctx.Done():
						return
					case <-deadline:
						yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil), nil)
						return
					case <-ticker.C:
						if !yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil), nil) {
							return
						}
					}
				}
			}
			if !yield(a2a.NewArtifactEvent(execCtx, a2a.NewTextPart("plan for "+text)), nil) {
				return
			}
			yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCompleted, nil), nil)
		}
	})
}

// authenticated marks every call as made by one user, which the in-memory task store requires
// before it lists tasks.
type authenticated struct {
	a2asrv.PassthroughCallInterceptor
}

func (authenticated) Before(ctx context.Context, callCtx *a2asrv.CallContext, _ *a2asrv.Request) (context.Context, any, error) {
	callCtx.User = a2asrv.NewAuthenticatedUser("tester", nil)
	return ctx, nil, nil
}

type agentServer struct {
	server *httptest.Server
	mu     sync.Mutex
	auth   []string
}

func (s *agentServer) authorizations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.auth...)
}

func newAgentServer(t *testing.T) *agentServer {
	t.Helper()
	extended := &a2a.AgentCard{Name: "Extended Planner", Skills: []a2a.AgentSkill{{ID: "plan_trip"}, {ID: "extended_only"}}}
	handler := a2asrv.NewHandler(planner(),
		a2asrv.WithExtendedAgentCard(extended),
		a2asrv.WithPushNotifications(push.NewInMemoryStore(), push.NewHTTPPushSender(nil)),
		a2asrv.WithCallInterceptors(authenticated{}),
	)
	mux := http.NewServeMux()
	mux.Handle("/rpc", a2asrv.NewJSONRPCHandler(handler))
	mux.Handle("/v1/", http.StripPrefix("/v1", a2asrv.NewRESTHandler(handler)))
	s := &agentServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.auth = append(s.auth, r.Header.Get("Authorization"))
		s.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *agentServer) endpoint(binding string) string {
	if binding == BindingJSONRPC {
		return s.server.URL + "/rpc"
	}
	return s.server.URL + "/v1"
}

func newTestClient(t *testing.T, s *agentServer, binding string) *Client {
	t.Helper()
	client, err := NewClient(context.Background(), binding, s.endpoint(binding), Options{
		ProtocolVersion: "1.0", CallTimeout: 5 * time.Second, StreamTimeout: 10 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestNewClientValidatesItsInputs(t *testing.T) {
	ctx := context.Background()
	_, err := NewClient(ctx, "GRPC", "http://agent", Options{ProtocolVersion: "1.0"})
	require.ErrorContains(t, err, "unsupported protocol binding")
	_, err = NewClient(ctx, "", "http://agent", Options{ProtocolVersion: "1.0"})
	require.ErrorContains(t, err, "unsupported protocol binding")
	_, err = NewClient(ctx, BindingJSONRPC, "  ", Options{ProtocolVersion: "1.0"})
	require.ErrorContains(t, err, "endpoint URL is required")
	_, err = NewClient(ctx, BindingHTTPJSON, "http://agent", Options{})
	require.ErrorContains(t, err, "protocol version is required")

	client, err := NewClient(ctx, BindingHTTPJSON, " http://agent/v1 ", Options{ProtocolVersion: "1.0"})
	require.NoError(t, err)
	require.Equal(t, BindingHTTPJSON, client.Binding())
	require.Equal(t, "http://agent/v1", client.URL())
	require.Equal(t, DefaultCallTimeout, client.callTimeoutOrDefault())
	require.Equal(t, DefaultStreamTimeout, client.streamTimeoutOrDefault())
	require.NoError(t, client.Close())
	require.NoError(t, client.Close(), "closing twice is safe")

	outcome := client.ListTasks(ctx, nil)
	require.ErrorContains(t, outcome.Err, "closed")
	require.Equal(t, "ListTasks", outcome.Method)

	var nilClient *Client
	require.NoError(t, nilClient.Close())
	require.Equal(t, DefaultCallTimeout, nilClient.callTimeoutOrDefault())
}

func TestEveryOperationOverBothBindings(t *testing.T) {
	server := newAgentServer(t)
	for _, binding := range []string{BindingJSONRPC, BindingHTTPJSON} {
		t.Run(binding, func(t *testing.T) {
			ctx := context.Background()
			client := newTestClient(t, server, binding)
			headers := Headers{"Authorization": "Bearer " + binding, "": "dropped", "X-Empty": ""}

			sent := client.SendMessage(ctx, headers, "Kandy", false)
			require.NoError(t, sent.Err)
			require.NotNil(t, sent.Task)
			require.Equal(t, string(a2a.TaskStateCompleted), sent.Task.State)
			require.True(t, sent.Task.Terminal)
			text, err := sent.ArtifactText()
			require.NoError(t, err)
			require.Equal(t, "plan for Kandy", text)
			require.Contains(t, server.authorizations(), "Bearer "+binding)

			streamed := client.StreamMessage(ctx, headers, "Ella")
			require.NoError(t, streamed.Err)
			require.GreaterOrEqual(t, len(streamed.Events), 3)
			state, err := streamed.FinalState()
			require.NoError(t, err)
			require.Equal(t, string(a2a.TaskStateCompleted), state)
			text, err = streamed.ArtifactText()
			require.NoError(t, err)
			require.Equal(t, "plan for Ella", text)
			require.NotEmpty(t, streamed.TaskID())
			require.Contains(t, streamed.Summary(), "event(s) over")

			running := client.SendMessage(ctx, headers, "Galle slowly", true)
			require.NoError(t, running.Err)
			require.NotNil(t, running.Task)
			require.False(t, running.Task.Terminal)
			taskID := running.TaskID()
			require.NotEmpty(t, taskID)

			got := client.GetTask(ctx, headers, taskID)
			require.NoError(t, got.Err)
			require.Equal(t, taskID, got.Task.ID)

			listed := client.ListTasks(ctx, headers)
			require.NoError(t, listed.Err)
			require.Equal(t, "task list", listed.Kind())
			ids := make([]string, 0, len(listed.Tasks))
			for _, task := range listed.Tasks {
				ids = append(ids, task.ID)
			}
			require.Contains(t, ids, taskID)

			created := client.CreatePushConfig(ctx, headers, taskID, "push-1", "https://receiver.example/notify")
			require.NoError(t, created.Err)
			require.Equal(t, "push-1", created.PushConfig.ID)
			fetched := client.GetPushConfig(ctx, headers, taskID, "push-1")
			require.NoError(t, fetched.Err)
			require.Equal(t, "push-1", fetched.PushConfig.ID)
			configs := client.ListPushConfigs(ctx, headers, taskID)
			require.NoError(t, configs.Err)
			require.Len(t, configs.PushConfigs, 1)
			require.NoError(t, client.DeletePushConfig(ctx, headers, taskID, "push-1").Err)
			configs = client.ListPushConfigs(ctx, headers, taskID)
			require.NoError(t, configs.Err)
			require.Empty(t, configs.PushConfigs)
			require.Equal(t, "push notification config list", configs.Kind())

			subscribed := client.SubscribeToTask(ctx, headers, taskID, 1)
			require.NoError(t, subscribed.Err)
			require.Len(t, subscribed.Events, 1)

			canceled := client.CancelTask(ctx, headers, taskID)
			require.NoError(t, canceled.Err)
			require.Equal(t, string(a2a.TaskStateCanceled), canceled.Task.State)

			card := client.GetExtendedCard(ctx, headers)
			require.NoError(t, card.Err)
			require.Equal(t, "Extended Planner", card.Card.Name)
			require.Equal(t, []string{"plan_trip", "extended_only"}, card.Card.Skills)

			missing := client.GetTask(ctx, headers, "no-such-task")
			require.Error(t, missing.Err, "a refused call is recorded, not raised")
			require.Nil(t, missing.Task)
		})
	}
}

func TestSubscribeRejectsANonPositiveEventCount(t *testing.T) {
	server := newAgentServer(t)
	client := newTestClient(t, server, BindingHTTPJSON)
	for _, count := range []int{0, -1} {
		outcome := client.SubscribeToTask(context.Background(), nil, "task", count)
		require.ErrorContains(t, outcome.Err, "must be positive")
	}
}

func TestOneClientServesConcurrentCalls(t *testing.T) {
	server := newAgentServer(t)
	client := newTestClient(t, server, BindingJSONRPC)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if outcome := client.SendMessage(context.Background(), nil, "Kandy", false); outcome.Err != nil {
				errs <- outcome.Err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestInterfaceURL(t *testing.T) {
	card := []byte(`{"name":"c","supportedInterfaces":[
		{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"http://gw/agent"},
		{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"http://gw/agent/v1"}]}`)
	url, err := InterfaceURL(card, BindingJSONRPC)
	require.NoError(t, err)
	require.Equal(t, "http://gw/agent", url)
	url, err = InterfaceURL(card, BindingHTTPJSON)
	require.NoError(t, err)
	require.Equal(t, "http://gw/agent/v1", url)

	_, err = InterfaceURL(card, "GRPC")
	require.ErrorContains(t, err, "unsupported protocol binding")
	_, err = InterfaceURL([]byte("not json"), BindingJSONRPC)
	require.ErrorContains(t, err, "not an Agent Card")
	_, err = InterfaceURL([]byte(`{"name":"c","supportedInterfaces":[]}`), BindingJSONRPC)
	require.ErrorContains(t, err, "advertises no JSONRPC interface")
	_, err = InterfaceURL([]byte(`{"name":"c","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":" "}]}`), BindingJSONRPC)
	require.ErrorContains(t, err, "advertises no url")
}

func TestOutcomeHelpers(t *testing.T) {
	var none *Outcome
	require.Empty(t, none.TaskID())
	require.Equal(t, "no outcome", none.Kind())
	require.Equal(t, "stream delivered no events", none.Summary())
	_, err := none.ArtifactText()
	require.Error(t, err)
	_, err = none.FinalState()
	require.ErrorContains(t, err, "no events")

	empty := &Outcome{Method: "SendMessage"}
	require.Equal(t, "no payload", empty.Kind())
	_, err = empty.ArtifactText()
	require.ErrorContains(t, err, "SendMessage produced no artifact (result: no payload)")

	message := "hello"
	require.Equal(t, "message", (&Outcome{Message: &message}).Kind())
	require.Equal(t, "agent card", (&Outcome{Card: &Card{}}).Kind())
	require.Equal(t, "push notification config", (&Outcome{PushConfig: &PushConfig{}}).Kind())
	require.Equal(t, "task", (&Outcome{Task: &Task{ID: "t"}}).Kind())

	streamed := &Outcome{Events: []Event{
		{Type: eventStatusUpdate, State: "TASK_STATE_WORKING", TaskID: "t-1"},
		{Type: eventArtifactUpdate, Artifact: "itinerary", Text: strings.Repeat("x", 300)},
	}}
	require.Equal(t, "t-1", streamed.TaskID())
	require.Equal(t, "2 stream event(s)", streamed.Kind())
	require.Contains(t, streamed.Summary(), "...")
	text, err := streamed.ArtifactText()
	require.NoError(t, err)
	require.Equal(t, "itinerary", text)
	_, err = streamed.FinalState()
	require.ErrorContains(t, err, "carries no task state")
}

func TestConversionsHandleNilAndEveryEventType(t *testing.T) {
	require.Nil(t, convertTask(nil))
	require.Nil(t, convertCard(nil))
	require.Nil(t, convertPushConfig(nil))

	info := a2a.TaskInfo{TaskID: "t-1", ContextID: "c-1"}
	status := a2a.NewStatusUpdateEvent(info, a2a.TaskStateWorking, a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("step")))
	require.Equal(t, Event{Type: eventStatusUpdate, State: "TASK_STATE_WORKING", Text: "TASK_STATE_WORKING step", TaskID: "t-1"},
		convertEvent(status, 0))

	artifact := a2a.NewArtifactEvent(info, a2a.NewTextPart("a"), nil, a2a.NewTextPart("b"))
	converted := convertEvent(artifact, time.Second)
	require.Equal(t, eventArtifactUpdate, converted.Type)
	require.Equal(t, "a\nb", converted.Artifact)
	require.Equal(t, time.Second, converted.Offset)

	task := &a2a.Task{ID: "t-1", Status: a2a.TaskStatus{State: a2a.TaskStateFailed}, Artifacts: []*a2a.Artifact{nil}}
	require.Equal(t, eventTask, convertEvent(task, 0).Type)
	require.True(t, convertTask(task).Terminal)
	require.Empty(t, convertTask(task).Artifacts)

	message := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("hi"))
	require.Equal(t, Event{Type: eventMessage, Text: "hi"}, convertEvent(message, 0))
}

func TestDrainRecordsErrorsOnlyBeforeTheRequestedCount(t *testing.T) {
	failing := func(after int) iter.Seq2[a2a.Event, error] {
		return func(yield func(a2a.Event, error) bool) {
			for i := 0; i < after; i++ {
				if !yield(a2a.NewStatusUpdateEvent(a2a.TaskInfo{TaskID: "t"}, a2a.TaskStateWorking, nil), nil) {
					return
				}
			}
			yield(nil, errors.New("connection closed"))
		}
	}
	require.ErrorContains(t, drain(failing(1), "SubscribeToTask", 2).Err, "connection closed")
	require.ErrorContains(t, drain(failing(1), "SendStreamingMessage", 0).Err, "connection closed")
	bounded := drain(failing(3), "SubscribeToTask", 2)
	require.NoError(t, bounded.Err)
	require.Len(t, bounded.Events, 2)
}
