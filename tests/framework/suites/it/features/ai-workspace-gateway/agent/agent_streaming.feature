# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------


# A2A's streaming operations are the reason the Agent kind disables its route
# timeout by default and leans on idleTimeout instead. What makes these scenarios
# worth having is not that a stream comes back — it is WHEN each event does.
#
# A response the gateway buffered whole and released at the end is still framed
# as text/event-stream and still contains every event in order, so every
# content assertion passes on it. The only thing that separates it from a real
# stream is that its first event arrives at the same instant as its last. That is
# what "first event should arrive before its last event" asserts, and it is why
# the trip planner paces its status updates rather than emitting them at once.
#
# The two bindings frame their SSE payloads differently, and both are exercised:
#   JSON-RPC:  data: {"result": {"statusUpdate": {...}}, "id": N, "jsonrpc": "2.0"}
#   HTTP+JSON: data: {"statusUpdate": {...}}
# Assertions below are written against whichever shape the binding under test
# actually produces.
#
# Data-plane readiness is polled on an operation route rather than assumed after
# the management call returns: a route answering is what proves the deployment
# reached the router and the policy engine.

@agent-streaming
Feature: Agent streaming operations
  As an A2A client
  I want streaming operations to stream
  So that I see task progress before the task completes

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: SendStreamingMessage streams over both bindings and closes on a terminal state
    Given I generate a unique resource name from "agent-streaming" and store it as "agentName"
    And I generate a unique API context from "/agent-streaming" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Streaming                                                                                     |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.resilience    | {"idleTimeout":"30s"}                                                                               |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # --- JSON-RPC binding ---
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I open an A2A stream with a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "SendStreamingMessage", "params": {"message": {"messageId": "stream-rpc-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 3 day trip to Kandy"}]}}}
      """
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/event-stream"
    And the response header "Transfer-Encoding" should contain "chunked"
    # A content-length means the gateway knew the whole size before sending,
    # which it cannot on a response it is forwarding chunk by chunk.
    And the response header "Content-Length" should not exist
    And the A2A stream should have received at least 3 events
    And the A2A stream's first event should arrive before its last event
    And the response body should contain "TASK_STATE_WORKING"
    And the response body should contain "Trip plan for Kandy: 3 days"
    And the A2A stream's last event should contain "TASK_STATE_COMPLETED"

    # --- HTTP+JSON binding, same operation ---
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I open an A2A stream with a "POST" request to "${CTX:agentContext}/v1/message:stream" with body:
      """
      {"message": {"messageId": "stream-rest-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 3 day trip to Kandy"}]}}
      """
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/event-stream"
    And the response header "Transfer-Encoding" should contain "chunked"
    And the response header "Content-Length" should not exist
    And the A2A stream should have received at least 3 events
    And the A2A stream's first event should arrive before its last event
    And the response body should contain "Trip plan for Kandy: 3 days"
    And the A2A stream's last event should contain "TASK_STATE_COMPLETED"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # SubscribeToTask re-attaches to a task that is still running, so it needs a
  # task that has not finished. returnImmediately mints one: the agent answers
  # straight away with a submitted task and keeps working on it in the
  # background.
  #
  # The HTTP+JSON leg uses POST. The gateway routes this operation as POST,
  # following the specification document; the proto says GET, and upstream
  # disagrees with itself at the pinned commit. The reference SDK happens to
  # register both verbs, so the divergence costs nothing at runtime — but the
  # gateway only generates the POST route, which is what this asserts.
  Scenario: SubscribeToTask re-attaches to a running task on both bindings
    Given I generate a unique resource name from "agent-subscribe" and store it as "agentName"
    And I generate a unique API context from "/agent-subscribe" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Subscribe                                                                                     |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.resilience    | {"idleTimeout":"30s"}                                                                               |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # Start a long-running task and keep its id.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}/v1/message:send" with body:
      """
      {"message": {"messageId": "subscribe-seed-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 5 day trip to Galle slowly"}]}, "configuration": {"returnImmediately": true}}
      """
    Then the response status code should be 200
    And I store the JSON response field "task.id" as "slowTask"

    # Re-attach over HTTP+JSON and receive live events.
    #
    # Read a bounded number of events rather than reading to close: the task is
    # deliberately still running, so this stream stays open until the agent's
    # hold ends. Two events are enough to show live events reaching a re-attached
    # subscriber, which is the whole claim.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I read 2 events from an A2A stream opened with a "POST" request to "${CTX:agentContext}/v1/tasks/${CTX:slowTask}:subscribe"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/event-stream"
    And the A2A stream should have received at least 2 events
    And the A2A stream's first event should arrive before its last event

    # And over JSON-RPC, against the same still-running task.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I read 2 events from an A2A stream opened with a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 2, "method": "SubscribeToTask", "params": {"id": "${CTX:slowTask}"}}
      """
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/event-stream"
    And the A2A stream should have received at least 2 events
    And the A2A stream's first event should arrive before its last event

    # Cancelling ends both the task and the stream, rather than leaving the
    # agent working for the rest of its hold.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 3, "method": "CancelTask", "params": {"id": "${CTX:slowTask}"}}
      """
    Then the response status code should be 200
    And the response body should contain "TASK_STATE_CANCELED"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # This is the fact that makes the gateway's streaming heuristic sufficient.
  #
  # Nothing at runtime knows which operations are streaming ones: the engine
  # decides from what the upstream framed, recognising chunked or
  # text/event-stream. That works only because an A2A error is never framed as an
  # event stream — a failed streaming call comes back as a buffered JSON document
  # with a content-length. If that stopped being true, a chunked JSON error would
  # be mistaken for a stream, and this is the scenario that would say so.
  Scenario: A failed streaming call returns a buffered JSON error, not a stream
    Given I generate a unique resource name from "agent-stream-error" and store it as "agentName"
    And I generate a unique API context from "/agent-stream-error" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Stream Error                                                                                  |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # A streaming operation with bad params. Over JSON-RPC the error rides a 200.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "SendStreamingMessage", "params": {}}
      """
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the response header "Content-Length" should exist
    And the response header "Transfer-Encoding" should not exist
    And the JSON response should have field "error"

    # Subscribing to a task that does not exist: a 404 as a buffered document,
    # not an empty event stream.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}/v1/tasks/no-such-task:subscribe"
    Then the response status code should be 404
    And the response header "Content-Type" should contain "application/json"
    And the response header "Content-Length" should exist

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # S13.3: for an Agent with no user policies attached, the analytics system
  # policy is the only response-body policy in the chain, so it alone decides
  # whether the chain can stream. Flipping its ResponseBodyMode to Buffer would
  # buffer every A2A stream with no error anywhere — just a client that waits for
  # the task to finish. Every other scenario in this file would still pass, since
  # the events all arrive eventually and in order. This one would not.
  Scenario: Streaming still streams with the analytics policy in the chain
    Given I reset the analytics collector
    And I generate a unique resource name from "agent-stream-analytics" and store it as "agentName"
    And I generate a unique API context from "/agent-stream-analytics" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent Stream Analytics                               |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.resilience    | {"idleTimeout":"30s"}                                |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                    |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I open an A2A stream with a "POST" request to "${CTX:agentContext}/v1/message:stream" with body:
      """
      {"message": {"messageId": "stream-analytics-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 2 day trip to Galle"}]}}
      """
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/event-stream"
    And the A2A stream should have received at least 3 events
    And the A2A stream's first event should arrive before its last event
    And the A2A stream's last event should contain "TASK_STATE_COMPLETED"

    # The event reports it as streamed rather than buffered, which is the
    # other half of the same claim: the chain negotiated streaming, and the
    # analytics policy saw it as a stream.
    When I wait for the analytics collector to settle
    Then the latest analytics event for path "${CTX:agentContext}/v1/message:stream" should have A2A field "operation" with value "SendStreamingMessage"
    And the latest analytics event for path "${CTX:agentContext}/v1/message:stream" should have A2A field "response.is_streaming" with value "true"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The raw scenarios above assert the wire: SSE framing, chunked transfer, no
  # content-length, and the bytes of each event as it arrives. Those are written
  # per binding, because the two frame their payloads differently.
  #
  # This one asserts what a real client sees: decoded events, in order, with a
  # terminal state and an artifact — through the official Go SDK, which does the
  # per-binding decoding itself. A gateway that mangled the JSON-RPC envelope or
  # dropped the StreamResponse wrapper would still emit plausible-looking bytes
  # and still pass every raw assertion; the SDK would fail to decode them.
  #
  # The arrival-time assertion is repeated here rather than left to the raw
  # scenarios, because the timestamps taken here are the ones a client actually
  # experiences: recorded as each event is yielded by the SDK's iterator, after
  # its own decoding, not as bytes land on the socket.
  Scenario: A conformant SDK client observes decoded events incrementally on both bindings
    Given I generate a unique resource name from "agent-stream-sdk" and store it as "agentName"
    And I generate a unique API context from "/agent-stream-sdk" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Stream SDK                                                                                    |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.resilience    | {"idleTimeout":"30s"}                                                                               |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding at "${CTX:agentContext}"
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"

    When the A2A client "rpc" streams the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rpc" should have received at least 3 stream events
    And the A2A client "rpc" stream's first event should arrive before its last event
    And the A2A client "rpc" should have received a stream event containing "TASK_STATE_WORKING"
    And the A2A client "rpc" should have received a stream event containing "Trip plan for Kandy: 3 days"
    And the A2A client "rpc" stream should end in state "TASK_STATE_COMPLETED"

    When the A2A client "rest" streams the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" should have received at least 3 stream events
    And the A2A client "rest" stream's first event should arrive before its last event
    And the A2A client "rest" should have received a stream event containing "TASK_STATE_WORKING"
    And the A2A client "rest" should have received a stream event containing "Trip plan for Kandy: 3 days"
    And the A2A client "rest" stream should end in state "TASK_STATE_COMPLETED"

    # The two bindings decoded to the same artifact, which is the streamed half of
    # the cross-transport claim: one operation, one chain, two framings.
    Then the A2A clients "rpc" and "rest" should have received the same artifact

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
