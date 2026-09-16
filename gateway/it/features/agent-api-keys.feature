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

# Agent API keys reuse every existing APIKey* schema, storage path and runtime
# enforcement — the kind was added to the key handlers without a database, xDS or
# policy-engine change. So most of this file is api-keys.feature with
# /rest-apis/ swapped for /agents/, and it is deliberately kept that close: the
# value is in proving the two kinds behave identically, which a rewritten set of
# scenarios would obscure.
#
# Two scenarios at the end have no counterpart there, because Agents expose five
# key operations rather than four and because storage survival is not the
# property that matters at runtime. See the comments on each.

Feature: Agent API Key Management Operations
  As an API administrator
  I want to manage API keys for Agents
  So that I can control access to A2A operations through API key authentication

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== API KEY LIFECYCLE - SUCCESS PATH ====================

  Scenario: Complete Agent API key lifecycle - generate, list, regenerate, and revoke
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-apikey-lifecycle
      spec:
        displayName: Agent APIKey Lifecycle
        version: v1.0
        context: /agent-apikey-lifecycle
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful

    # Generate API key
    When I send a POST request to the "gateway-controller" service at "/agents/agent-apikey-lifecycle/api-keys" with body:
      """
      {
        "name": "agent-key-1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    And the JSON response should have field "apiKey.name"
    And the JSON response should have field "apiKey.apiKey"
    And I wait for 2 seconds

    # List API keys - should have 1 key
    When I send a GET request to the "gateway-controller" service at "/agents/agent-apikey-lifecycle/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "agent-key-1"

    # Regenerate API key
    When I send a POST request to the "gateway-controller" service at "/agents/agent-apikey-lifecycle/api-keys/agent-key-1/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey.apiKey"

    # Revoke API key
    When I send a DELETE request to the "gateway-controller" service at "/agents/agent-apikey-lifecycle/api-keys/agent-key-1"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    # Verify key is revoked - list should be empty
    When I send a GET request to the "gateway-controller" service at "/agents/agent-apikey-lifecycle/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should not contain "agent-key-1"

    When I delete the Agent "agent-apikey-lifecycle"
    Then the response should be successful

  Scenario: Generate multiple API keys for the same Agent
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-multi-key
      spec:
        displayName: Agent Multi Key
        version: v1.0
        context: /agent-multi-key
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful

    When I send a POST request to the "gateway-controller" service at "/agents/agent-multi-key/api-keys" with body:
      """
      {
        "name": "agent-key-alpha"
      }
      """
    Then the response status should be 201

    When I send a POST request to the "gateway-controller" service at "/agents/agent-multi-key/api-keys" with body:
      """
      {
        "name": "agent-key-beta"
      }
      """
    Then the response status should be 201
    And I wait for 2 seconds

    When I send a GET request to the "gateway-controller" service at "/agents/agent-multi-key/api-keys"
    Then the response status should be 200
    And the response body should contain "agent-key-alpha"
    And the response body should contain "agent-key-beta"

    When I delete the Agent "agent-multi-key"
    Then the response should be successful

  Scenario: List API keys for an Agent with no keys returns an empty list
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-no-keys
      spec:
        displayName: Agent No Keys
        version: v1.0
        context: /agent-no-keys
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful

    When I send a GET request to the "gateway-controller" service at "/agents/agent-no-keys/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    When I delete the Agent "agent-no-keys"
    Then the response should be successful

  # ==================== ERROR PATHS ====================

  Scenario: Generate an API key for a non-existent Agent returns 404
    When I send a POST request to the "gateway-controller" service at "/agents/agent-does-not-exist/api-keys" with body:
      """
      {
        "name": "orphan-key"
      }
      """
    Then the response status should be 404

  Scenario: List API keys for a non-existent Agent returns 404
    When I send a GET request to the "gateway-controller" service at "/agents/agent-does-not-exist/api-keys"
    Then the response status should be 404

  Scenario: Regenerate an API key for a non-existent Agent returns 404
    When I send a POST request to the "gateway-controller" service at "/agents/agent-does-not-exist/api-keys/some-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404

  Scenario: Generate an API key with an invalid JSON body returns an error
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-invalid-json-key
      spec:
        displayName: Agent Invalid JSON Key
        version: v1.0
        context: /agent-invalid-json-key
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful

    When I send a POST request to the "gateway-controller" service at "/agents/agent-invalid-json-key/api-keys" with body:
      """
      { this is not json
      """
    Then the response status should be 400

    When I delete the Agent "agent-invalid-json-key"
    Then the response should be successful

  # ==================== RUNTIME ENFORCEMENT ====================

  # api-keys.feature stops at the management API. For Agents the interesting half
  # is downstream of it: a key is only useful if it authenticates a request on
  # the Agent's own routes, and the two scenarios below are the ones where a key
  # can exist, list correctly, and still not work.

  # PUT /agents/{id}/api-keys/{name} has no counterpart in api-keys.feature —
  # Agents expose five key operations, not four. Both halves are asserted on
  # purpose: checking only that the replacement works would pass even if the
  # superseded value stayed valid, which is the failure that matters.
  Scenario: Updating an Agent API key replaces the credential at runtime
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-key-update
      spec:
        displayName: Agent Key Update
        version: v1.0
        context: /agent-key-update
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: api-key-auth
                version: v1
                params:
                  key: API-Key
                  in: header
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I send a POST request to the "gateway-controller" service at "/agents/agent-key-update/api-keys" with body:
      """
      {
        "name": "rotating-key",
        "apiKey": "agent-key-before-rotation"
      }
      """
    Then the response status should be 201
    And I wait for 2 seconds

    # The supplied key authenticates.
    When I clear all headers
    And I set header "API-Key" to "agent-key-before-rotation"
    And I send an A2A "GET" request to "http://localhost:8080/agent-key-update/v1/tasks"
    Then the response status code should be 200

    # Replace it with a different externally supplied value.
    Given I authenticate using basic auth as "admin"
    When I send a PUT request to the "gateway-controller" service at "/agents/agent-key-update/api-keys/rotating-key" with body:
      """
      {
        "name": "rotating-key",
        "apiKey": "agent-key-after-rotation"
      }
      """
    Then the response status should be 200
    And I wait for 2 seconds

    # The replacement authenticates...
    When I clear all headers
    And I set header "API-Key" to "agent-key-after-rotation"
    And I send an A2A "GET" request to "http://localhost:8080/agent-key-update/v1/tasks"
    Then the response status code should be 200

    # ...and the superseded value no longer does.
    When I clear all headers
    And I set header "API-Key" to "agent-key-before-rotation"
    And I send an A2A "GET" request to "http://localhost:8080/agent-key-update/v1/tasks"
    Then the response status code should be 401

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-key-update"
    Then the response should be successful

  # TestAgentAPIKeys_SurviveUndeployRedeploy already pins that the row survives an
  # undeploy — undeploy keeps the configuration so it can be redeployed, and
  # revoking keys there would silently break every client over what is meant to be
  # a reversible operation. What a handler test structurally cannot see is whether
  # the key is re-attached to the redeployed Agent's routes. A key that survives in
  # storage but stops authenticating is indistinguishable from a revoked one to
  # every client, so that is what this asserts.
  Scenario: An Agent API key still authenticates after an undeploy and redeploy
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-key-redeploy
      spec:
        displayName: Agent Key Redeploy
        version: v1.0
        context: /agent-key-redeploy
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: api-key-auth
                version: v1
                params:
                  key: API-Key
                  in: header
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I send a POST request to the "gateway-controller" service at "/agents/agent-key-redeploy/api-keys" with body:
      """
      {
        "name": "surviving-key",
        "apiKey": "agent-key-survives-redeploy"
      }
      """
    Then the response status should be 201
    And I wait for 2 seconds

    When I clear all headers
    And I set header "API-Key" to "agent-key-survives-redeploy"
    And I send an A2A "GET" request to "http://localhost:8080/agent-key-redeploy/v1/tasks"
    Then the response status code should be 200

    # Undeploy: the configuration is kept, so the keys must be too.
    Given I authenticate using basic auth as "admin"
    When I update the Agent "agent-key-redeploy" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-key-redeploy
      spec:
        deploymentState: undeployed
        displayName: Agent Key Redeploy
        version: v1.0
        context: /agent-key-redeploy
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: api-key-auth
                version: v1
                params:
                  key: API-Key
                  in: header
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful

    # The key is still listed while undeployed.
    When I send a GET request to the "gateway-controller" service at "/agents/agent-key-redeploy/api-keys"
    Then the response status should be 200
    And the response body should contain "surviving-key"

    # Redeploy.
    When I update the Agent "agent-key-redeploy" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-key-redeploy
      spec:
        displayName: Agent Key Redeploy
        version: v1.0
        context: /agent-key-redeploy
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: api-key-auth
                version: v1
                params:
                  key: API-Key
                  in: header
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # The same key value still authenticates against the redeployed routes.
    When I clear all headers
    And I set header "API-Key" to "agent-key-survives-redeploy"
    And I send an A2A "GET" request to "http://localhost:8080/agent-key-redeploy/v1/tasks"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-key-redeploy"
    Then the response should be successful
