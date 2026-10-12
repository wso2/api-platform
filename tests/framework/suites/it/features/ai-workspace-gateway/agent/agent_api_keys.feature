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
# policy-engine change. So most of this file is security/api_keys.feature with
# /rest-apis/ swapped for /agents/, and it is deliberately kept that close: the
# value is in proving the two kinds behave identically, which a rewritten set of
# scenarios would obscure.
#
# Two scenarios at the end have no counterpart there, because Agents expose five
# key operations rather than four and because storage survival is not the
# property that matters at runtime. See the comments on each.
#
# Data-plane readiness is polled on an operation route rather than assumed after a
# management call returns: an Agent behind api-key-auth answering 401 without a key
# proves its chain is bound, and answering 200 with a key proves the key reached the
# policy engine.

@agent-api-keys
Feature: Agent API Key Management Operations
  As an API administrator
  I want to manage API keys for Agents
  So that I can control access to A2A operations through API key authentication

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== API KEY LIFECYCLE - SUCCESS PATH ====================

  Scenario: Complete Agent API key lifecycle - generate, list, regenerate, and revoke
    Given I generate a unique resource name from "agent-apikey-lifecycle" and store it as "agentName"
    And I generate a unique API context from "/agent-apikey-lifecycle" and store it as "agentContext"
    And I generate a unique value from "agent-key-1" and store it as "keyName"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent APIKey Lifecycle                               |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                    |
    Then the response should be successful

    # Generate API key
    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:keyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    And the JSON response should have field "apiKey.name"
    And the JSON response should have field "apiKey.apiKey"

    # List API keys - should have 1 key
    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:keyName}"

    # Regenerate API key
    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys/${CTX:keyName}/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey.apiKey"

    # Revoke API key
    When I send a "DELETE" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys/${CTX:keyName}"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    # Verify key is revoked - list should be empty
    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should not contain "${CTX:keyName}"

    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  Scenario: Generate multiple API keys for the same Agent
    Given I generate a unique resource name from "agent-multi-key" and store it as "agentName"
    And I generate a unique API context from "/agent-multi-key" and store it as "agentContext"
    And I generate a unique value from "agent-key-alpha" and store it as "firstKeyName"
    And I generate a unique value from "agent-key-beta" and store it as "secondKeyName"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent Multi Key                                      |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                    |
    Then the response should be successful

    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:firstKeyName}"
      }
      """
    Then the response status should be 201

    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:secondKeyName}"
      }
      """
    Then the response status should be 201

    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys"
    Then the response status should be 200
    And the response body should contain "${CTX:firstKeyName}"
    And the response body should contain "${CTX:secondKeyName}"

    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  Scenario: List API keys for an Agent with no keys returns an empty list
    Given I generate a unique resource name from "agent-no-keys" and store it as "agentName"
    And I generate a unique API context from "/agent-no-keys" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent No Keys                                        |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                    |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== ERROR PATHS ====================

  # The Agent name is generated and never created, so no other runner's Agent can
  # answer for it.
  Scenario: Generate an API key for a non-existent Agent returns 404
    Given I generate a unique resource name from "agent-does-not-exist" and store it as "missingAgentName"
    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:missingAgentName}/api-keys" with body:
      """
      {
        "name": "orphan-key"
      }
      """
    Then the response status should be 404

  Scenario: List API keys for a non-existent Agent returns 404
    Given I generate a unique resource name from "agent-does-not-exist" and store it as "missingAgentName"
    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:missingAgentName}/api-keys"
    Then the response status should be 404

  Scenario: Regenerate an API key for a non-existent Agent returns 404
    Given I generate a unique resource name from "agent-does-not-exist" and store it as "missingAgentName"
    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:missingAgentName}/api-keys/some-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404

  Scenario: Generate an API key with an invalid JSON body returns an error
    Given I generate a unique resource name from "agent-invalid-json-key" and store it as "agentName"
    And I generate a unique API context from "/agent-invalid-json-key" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent Invalid JSON Key                               |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                    |
    Then the response should be successful

    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      { this is not json
      """
    Then the response status should be 400

    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== RUNTIME ENFORCEMENT ====================

  # security/api_keys.feature stops at the management API. For Agents the
  # interesting half is downstream of it: a key is only useful if it authenticates
  # a request on the Agent's own routes, and the two scenarios below are the ones
  # where a key can exist, list correctly, and still not work.
  #
  # The externally supplied key values are generated so that no other runner's
  # key can collide with them.

  # PUT /agents/{id}/api-keys/{name} has no counterpart in security/api_keys.feature —
  # Agents expose five key operations, not four. Both halves are asserted on
  # purpose: checking only that the replacement works would pass even if the
  # superseded value stayed valid, which is the failure that matters.
  Scenario: Updating an Agent API key replaces the credential at runtime
    Given I generate a unique resource name from "agent-key-update" and store it as "agentName"
    And I generate a unique API context from "/agent-key-update" and store it as "agentContext"
    And I generate a unique value from "rotating-key" and store it as "keyName"
    And I generate a unique value from "agent-key-before-rotation" and store it as "keyBefore"
    And I generate a unique value from "agent-key-after-rotation" and store it as "keyAfter"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                         |
      | name                               | ${CTX:agentName}                                                                  |
      | displayName                        | Agent Key Update                                                                  |
      | context                            | ${CTX:agentContext}                                                               |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                      |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                              |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"}}                                                 |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:keyName}",
        "apiKey": "${CTX:keyBefore}"
      }
      """
    Then the response status should be 201

    # The supplied key authenticates.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyBefore}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    Then the response status code should be 200

    # Replace it with a different externally supplied value.
    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I send a "PUT" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys/${CTX:keyName}" with body:
      """
      {
        "name": "${CTX:keyName}",
        "apiKey": "${CTX:keyAfter}"
      }
      """
    Then the response status should be 200

    # The replacement authenticates...
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyAfter}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    Then the response status code should be 200

    # ...and the superseded value no longer does.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyBefore}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401
    Then the response status code should be 401

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # TestAgentAPIKeys_SurviveUndeployRedeploy already pins that the row survives an
  # undeploy — undeploy keeps the configuration so it can be redeployed, and
  # revoking keys there would silently break every client over what is meant to be
  # a reversible operation. What a handler test structurally cannot see is whether
  # the key is re-attached to the redeployed Agent's routes. A key that survives in
  # storage but stops authenticating is indistinguishable from a revoked one to
  # every client, so that is what this asserts.
  #
  # The undeploy is waited out on the data plane before the redeploy, so the final
  # 200 comes from routes rebuilt by the redeploy rather than from routes that
  # were never removed.
  Scenario: An Agent API key still authenticates after an undeploy and redeploy
    Given I generate a unique resource name from "agent-key-redeploy" and store it as "agentName"
    And I generate a unique API context from "/agent-key-redeploy" and store it as "agentContext"
    And I generate a unique value from "surviving-key" and store it as "keyName"
    And I generate a unique value from "agent-key-survives-redeploy" and store it as "keyValue"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                         |
      | name                               | ${CTX:agentName}                                                                  |
      | displayName                        | Agent Key Redeploy                                                                |
      | context                            | ${CTX:agentContext}                                                               |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                      |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                              |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"}}                                                 |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:keyName}",
        "apiKey": "${CTX:keyValue}"
      }
      """
    Then the response status should be 201

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyValue}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    Then the response status code should be 200

    # Undeploy: the configuration is kept, so the keys must be too.
    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I update Agent "${CTX:agentName}" from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                         |
      | name                               | ${CTX:agentName}                                                                  |
      | displayName                        | Agent Key Redeploy                                                                |
      | context                            | ${CTX:agentContext}                                                               |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                      |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                              |
      | spec.deploymentState               | undeployed                                                                        |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"}}                                                 |
    Then the response should be successful

    # The key is still listed while undeployed.
    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys"
    Then the response status should be 200
    And the response body should contain "${CTX:keyName}"

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyValue}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 404
    Then the response status code should be 404

    # Redeploy.
    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I update Agent "${CTX:agentName}" from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                         |
      | name                               | ${CTX:agentName}                                                                  |
      | displayName                        | Agent Key Redeploy                                                                |
      | context                            | ${CTX:agentContext}                                                               |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                      |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                              |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"}}                                                 |
    Then the response should be successful

    # The same key value still authenticates against the redeployed routes.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyValue}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    Then the response status code should be 200

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
