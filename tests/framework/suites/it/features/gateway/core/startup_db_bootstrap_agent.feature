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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@startup-db-bootstrap-agent
Feature: Startup database bootstrap of Agents
  As a gateway operator
  I want the gateway-controller to restore persisted Agents from the database on startup
  So that an Agent keeps its routes, auth policy and gateway-served cards across a restart

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Restarted gateway-controller restores a persisted Agent from the database
    Given I generate a unique resource name from "startup-db-agent" and store it as "agentName"
    Given I generate a unique API context from "/startup-db-agent" and store it as "agentContext"
    Given I generate a unique value from "startup-db-agent-key" and store it as "agentKeyName"
    Given I generate a unique value from "startup-db-agent-secret" and store it as "agentKey"

    # The Agent carries an auth policy and managed public and protected cards, because restoring
    # its routes is not the same as restoring what hangs off them: a route restored with an empty
    # policy chain still answers, having dropped authentication and the gateway-served cards.
    # "startup_db_protected_skill" is on the stored protected card alone, and "book_trip" on the
    # agent's own extended card alone, so they tell the gateway's copy from the agent's.
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion} |
      | name                               | ${CTX:agentName}          |
      | displayName                        | Startup DB Agent          |
      | context                            | ${CTX:agentContext}       |
      | upstreamUrl                        | http://a2a-trip-planner:9099 |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"managed","content":{"name":"Startup DB Agent","description":"Persisted Agent restored from the database on restart","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}},"protected":{"mode":"managed","content":{"name":"Startup DB Agent","description":"Persisted protected Agent Card restored on restart","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]},{"id":"startup_db_protected_skill","name":"Only on the persisted protected card","description":"Served from the gateway's stored protected card","tags":["travel"]}]}}} |
    Then the response should be successful

    When I send a "POST" request to the "gateway-controller" service at "/agents/${CTX:agentName}/api-keys" with body:
      """
      {
        "name": "${CTX:agentKeyName}",
        "apiKey": "${CTX:agentKey}"
      }
      """
    Then the response status should be 201

    # --- Agent, before the restart ---
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:agentKey}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "startup_db_protected_skill"
    And the response body should not contain "book_trip"

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401

    # The public card carries no auth policy. Its bytes and ETag are kept to compare after the
    # restart: a changed tag on unchanged bytes would tell every client its cached copy is stale.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Startup DB Agent"
    And the response body should not contain "startup_db_protected_skill"
    And I store the response body as "agentCard"
    And I store the response header "ETag" as "agentCardETag"

    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint

    # --- Agent, after the restart ---
    Given I authenticate using basic auth as "admin"
    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response body should contain "${CTX:agentName}"

    Given I authenticate using basic auth as "admin"
    When I get the Agent "${CTX:agentName}"
    Then the response status should be 200
    And the response body should contain "${CTX:agentName}"

    # Both transports still route: asserting one would pass on a restore that brought back only
    # one transport's routes.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:agentKey}"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    # The protected card came back inside its operation chain: still the gateway's document.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "startup_db_protected_skill"
    And the response body should not contain "book_trip"
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "result.name" should be "Startup DB Agent"
    And the response body should contain "startup_db_protected_skill"

    # The policy chain came back with the routes: a 401, not a 404, means the route exists and
    # its auth policy still enforces.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 401
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "startup_db_protected_skill"

    # The public card is still served from the stored copy, byte for byte, under the same tag.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response header "ETag" should be "${CTX:agentCardETag}"
    And the response body should be:
      """
      ${CTX:agentCard}
      """

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
