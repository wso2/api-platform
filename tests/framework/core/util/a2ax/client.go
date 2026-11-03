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
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// Protocol bindings a Client can be bound to, spelled as an Agent Card and an Agent resource
// spell them.
const (
	BindingJSONRPC  = string(a2a.TransportProtocolJSONRPC)
	BindingHTTPJSON = string(a2a.TransportProtocolHTTPJSON)
)

// Default bounds for one SDK call. They are stuck-test backstops rather than assertions: a call
// that should succeed and does not is reported with the SDK's own error long before either fires.
const (
	DefaultCallTimeout   = 30 * time.Second
	DefaultStreamTimeout = 60 * time.Second
)

// Options configure a Client.
type Options struct {
	// ProtocolVersion is the A2A version the client states on every request. It is required:
	// the gateway rejects an operation whose stated version is not the one its Agent exposes,
	// so a defaulted value would make the rejection path the one a scenario exercises.
	ProtocolVersion string
	// CallTimeout bounds one unary call. Zero selects DefaultCallTimeout.
	CallTimeout time.Duration
	// StreamTimeout bounds one streamed call, including the time spent waiting for events.
	// Zero selects DefaultStreamTimeout.
	StreamTimeout time.Duration
}

// Client is one SDK client bound to one endpoint and one protocol binding.
type Client struct {
	binding       string
	url           string
	callTimeout   time.Duration
	streamTimeout time.Duration
	sdk           *a2aclient.Client
}

// NewClient builds a client for exactly one endpoint and one binding.
func NewClient(ctx context.Context, binding, url string, opts Options) (*Client, error) {
	protocol, err := parseBinding(binding)
	if err != nil {
		return nil, err
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("a2ax: an endpoint URL is required")
	}
	version := strings.TrimSpace(opts.ProtocolVersion)
	if version == "" {
		return nil, fmt.Errorf("a2ax: a protocol version is required")
	}
	callTimeout := opts.CallTimeout
	if callTimeout <= 0 {
		callTimeout = DefaultCallTimeout
	}
	streamTimeout := opts.StreamTimeout
	if streamTimeout <= 0 {
		streamTimeout = DefaultStreamTimeout
	}

	// Bounded by the longest call the client makes; each call also carries its own deadline.
	httpClient := &http.Client{Timeout: streamTimeout}
	factory := []a2aclient.FactoryOption{a2aclient.WithDefaultsDisabled()}
	switch protocol {
	case a2a.TransportProtocolJSONRPC:
		factory = append(factory, a2aclient.WithJSONRPCTransport(httpClient))
	default:
		factory = append(factory, a2aclient.WithRESTTransport(httpClient))
	}
	endpoint := &a2a.AgentInterface{
		URL:             url,
		ProtocolBinding: protocol,
		ProtocolVersion: a2a.ProtocolVersion(version),
	}

	buildCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	sdk, err := a2aclient.NewFromEndpoints(buildCtx, []*a2a.AgentInterface{endpoint}, factory...)
	if err != nil {
		return nil, fmt.Errorf("a2ax: creating the %s client for %s: %w", binding, url, err)
	}
	return &Client{
		binding: string(protocol), url: url,
		callTimeout: callTimeout, streamTimeout: streamTimeout, sdk: sdk,
	}, nil
}

func parseBinding(binding string) (a2a.TransportProtocol, error) {
	switch strings.TrimSpace(binding) {
	case BindingJSONRPC:
		return a2a.TransportProtocolJSONRPC, nil
	case BindingHTTPJSON:
		return a2a.TransportProtocolHTTPJSON, nil
	default:
		return "", fmt.Errorf("a2ax: unsupported protocol binding %q: supported bindings are %s and %s",
			binding, BindingJSONRPC, BindingHTTPJSON)
	}
}

// Binding returns the protocol binding the client speaks.
func (c *Client) Binding() string { return c.binding }

// URL returns the endpoint the client dials.
func (c *Client) URL() string { return c.url }

// Close releases the client's connections. It is safe to call on a nil or closed client.
func (c *Client) Close() error {
	if c == nil || c.sdk == nil {
		return nil
	}
	err := c.sdk.Destroy()
	c.sdk = nil
	return err
}

// Headers are request headers attached to a call, such as the credentials a scenario set. They
// are read per call, so a token obtained after the client was built still applies.
type Headers map[string]string

func (c *Client) call(ctx context.Context, headers Headers, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if c == nil || c.sdk == nil {
		return nil, nil, fmt.Errorf("a2ax: the client is closed")
	}
	params := a2aclient.ServiceParams{}
	for name, value := range headers {
		if strings.TrimSpace(name) != "" && value != "" {
			params.Append(name, value)
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	return a2aclient.AttachServiceParams(callCtx, params), cancel, nil
}

// SendMessage sends one user text message. returnImmediately asks the agent to answer as soon
// as the task exists, which is the only way to obtain a task that is still running.
func (c *Client) SendMessage(ctx context.Context, headers Headers, text string, returnImmediately bool) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "SendMessage", Err: err}
	}
	defer cancel()
	request := &a2a.SendMessageRequest{Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))}
	if returnImmediately {
		request.Config = &a2a.SendMessageConfig{ReturnImmediately: true}
	}
	result, err := c.sdk.SendMessage(callCtx, request)
	outcome := &Outcome{Method: "SendMessage", Err: err}
	switch typed := result.(type) {
	case *a2a.Task:
		outcome.Task = convertTask(typed)
	case *a2a.Message:
		text := partsText(typed.Parts)
		outcome.Message = &text
	}
	return outcome
}

// StreamMessage sends one user text message as a stream and drains it to its end.
func (c *Client) StreamMessage(ctx context.Context, headers Headers, text string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.streamTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "SendStreamingMessage", Err: err}
	}
	defer cancel()
	request := &a2a.SendMessageRequest{Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))}
	return drain(c.sdk.SendStreamingMessage(callCtx, request), "SendStreamingMessage", 0)
}

// GetTask fetches one task.
func (c *Client) GetTask(ctx context.Context, headers Headers, taskID string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "GetTask", Err: err}
	}
	defer cancel()
	task, err := c.sdk.GetTask(callCtx, &a2a.GetTaskRequest{ID: a2a.TaskID(taskID)})
	return &Outcome{Method: "GetTask", Err: err, Task: convertTask(task)}
}

// ListTasks lists the tasks the agent holds.
func (c *Client) ListTasks(ctx context.Context, headers Headers) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "ListTasks", Err: err}
	}
	defer cancel()
	response, err := c.sdk.ListTasks(callCtx, &a2a.ListTasksRequest{})
	outcome := &Outcome{Method: "ListTasks", Err: err}
	if response != nil {
		outcome.Tasks = make([]Task, 0, len(response.Tasks))
		for _, task := range response.Tasks {
			if converted := convertTask(task); converted != nil {
				outcome.Tasks = append(outcome.Tasks, *converted)
			}
		}
	}
	return outcome
}

// CancelTask cancels one task.
func (c *Client) CancelTask(ctx context.Context, headers Headers, taskID string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "CancelTask", Err: err}
	}
	defer cancel()
	task, err := c.sdk.CancelTask(callCtx, &a2a.CancelTaskRequest{ID: a2a.TaskID(taskID)})
	return &Outcome{Method: "CancelTask", Err: err, Task: convertTask(task)}
}

// SubscribeToTask re-attaches to a task and stops after maxEvents events.
//
// Bounded by events rather than by the terminal state: the task it attaches to is normally
// still running, so reading to the end would wait out the agent's whole hold. maxEvents must be
// positive.
func (c *Client) SubscribeToTask(ctx context.Context, headers Headers, taskID string, maxEvents int) *Outcome {
	if maxEvents <= 0 {
		return &Outcome{Method: "SubscribeToTask", Err: fmt.Errorf("a2ax: the event count must be positive, got %d", maxEvents)}
	}
	callCtx, cancel, err := c.call(ctx, headers, c.streamTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "SubscribeToTask", Err: err}
	}
	defer cancel()
	return drain(c.sdk.SubscribeToTask(callCtx, &a2a.SubscribeToTaskRequest{ID: a2a.TaskID(taskID)}),
		"SubscribeToTask", maxEvents)
}

// CreatePushConfig registers a push notification config for a task.
func (c *Client) CreatePushConfig(ctx context.Context, headers Headers, taskID, configID, url string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "CreateTaskPushNotificationConfig", Err: err}
	}
	defer cancel()
	config, err := c.sdk.CreateTaskPushConfig(callCtx, &a2a.PushConfig{
		TaskID: a2a.TaskID(taskID), ID: configID, URL: url,
	})
	return &Outcome{Method: "CreateTaskPushNotificationConfig", Err: err, PushConfig: convertPushConfig(config)}
}

// GetPushConfig fetches one push notification config of a task.
func (c *Client) GetPushConfig(ctx context.Context, headers Headers, taskID, configID string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "GetTaskPushNotificationConfig", Err: err}
	}
	defer cancel()
	config, err := c.sdk.GetTaskPushConfig(callCtx, &a2a.GetTaskPushConfigRequest{TaskID: a2a.TaskID(taskID), ID: configID})
	return &Outcome{Method: "GetTaskPushNotificationConfig", Err: err, PushConfig: convertPushConfig(config)}
}

// ListPushConfigs lists the push notification configs of a task.
func (c *Client) ListPushConfigs(ctx context.Context, headers Headers, taskID string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "ListTaskPushNotificationConfigs", Err: err}
	}
	defer cancel()
	configs, err := c.sdk.ListTaskPushConfigs(callCtx, &a2a.ListTaskPushConfigRequest{TaskID: a2a.TaskID(taskID)})
	outcome := &Outcome{Method: "ListTaskPushNotificationConfigs", Err: err, PushConfigs: []PushConfig{}}
	for _, config := range configs {
		if converted := convertPushConfig(config); converted != nil {
			outcome.PushConfigs = append(outcome.PushConfigs, *converted)
		}
	}
	return outcome
}

// DeletePushConfig removes one push notification config of a task.
func (c *Client) DeletePushConfig(ctx context.Context, headers Headers, taskID, configID string) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "DeleteTaskPushNotificationConfig", Err: err}
	}
	defer cancel()
	err = c.sdk.DeleteTaskPushConfig(callCtx, &a2a.DeleteTaskPushConfigRequest{TaskID: a2a.TaskID(taskID), ID: configID})
	return &Outcome{Method: "DeleteTaskPushNotificationConfig", Err: err}
}

// GetExtendedCard fetches the authenticated extended Agent Card.
func (c *Client) GetExtendedCard(ctx context.Context, headers Headers) *Outcome {
	callCtx, cancel, err := c.call(ctx, headers, c.callTimeoutOrDefault())
	if err != nil {
		return &Outcome{Method: "GetExtendedAgentCard", Err: err}
	}
	defer cancel()
	card, err := c.sdk.GetExtendedAgentCard(callCtx, &a2a.GetExtendedAgentCardRequest{})
	return &Outcome{Method: "GetExtendedAgentCard", Err: err, Card: convertCard(card)}
}

func (c *Client) callTimeoutOrDefault() time.Duration {
	if c == nil || c.callTimeout <= 0 {
		return DefaultCallTimeout
	}
	return c.callTimeout
}

func (c *Client) streamTimeoutOrDefault() time.Duration {
	if c == nil || c.streamTimeout <= 0 {
		return DefaultStreamTimeout
	}
	return c.streamTimeout
}

// InterfaceURL returns the URL an Agent Card advertises for one binding, the way a real client
// bootstraps from a card.
func InterfaceURL(card []byte, binding string) (string, error) {
	protocol, err := parseBinding(binding)
	if err != nil {
		return "", err
	}
	var decoded a2a.AgentCard
	if err := json.Unmarshal(card, &decoded); err != nil {
		return "", fmt.Errorf("a2ax: the document is not an Agent Card: %w", err)
	}
	for _, iface := range decoded.SupportedInterfaces {
		if iface == nil || iface.ProtocolBinding != protocol {
			continue
		}
		if strings.TrimSpace(iface.URL) == "" {
			return "", fmt.Errorf("a2ax: the Agent Card's %s interface advertises no url", binding)
		}
		return iface.URL, nil
	}
	return "", fmt.Errorf("a2ax: the Agent Card advertises no %s interface", binding)
}

// drain consumes an SDK event iterator, timestamping each event as it is yielded. maxEvents of
// zero reads to the end of the stream; a positive value stops once that many have arrived, and
// a read error after that point is how abandoning a still-open stream looks, not a failure.
func drain(events iter.Seq2[a2a.Event, error], method string, maxEvents int) *Outcome {
	outcome := &Outcome{Method: method, Events: []Event{}}
	started := time.Now()
	for event, err := range events {
		if err != nil {
			if maxEvents == 0 || len(outcome.Events) < maxEvents {
				outcome.Err = err
			}
			break
		}
		outcome.Events = append(outcome.Events, convertEvent(event, time.Since(started)))
		if maxEvents > 0 && len(outcome.Events) >= maxEvents {
			break
		}
	}
	outcome.StreamDuration = time.Since(started)
	return outcome
}
