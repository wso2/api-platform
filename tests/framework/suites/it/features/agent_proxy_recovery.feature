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

@agent-proxy @agent-proxy-recovery
Feature: Agent proxies recover across gateway and control-plane restarts
  As a platform operator
  I want Agent proxy deployments, API keys and card previews to converge after a component restarts
  or reconnects
  So that a transient outage never leaves the gateway serving a stale or missing Agent proxy

  # Every scenario here stops or restarts a component mid-scenario, so the feature runs alone in the
  # agent-proxy-recovery block rather than beside runners that expect those components to stay up.
  #
  # Stale acknowledgements, a failed event write, and a failed or non-ZIP artifact fetch need fault
  # injection a live topology cannot provide deterministically, and on-prem key sync needs a
  # different control-plane mode; they are covered by the focused platform-api and
  # gateway-controller tests named in the Section 14 coverage record.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "agent-recovery-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  # A disconnected gateway catches up through its reconnect sync, which applies the deployment and
  # the undeployment but acknowledges neither (true for every kind, not just Agent). The control
  # plane therefore keeps reporting DEPLOYING and UNDEPLOYING here, so the evidence is what the
  # gateway serves, not the deployment status.
  @cp14-17
  Scenario: Deployments and undeployments made while the gateway is disconnected are applied on reconnect
    Given I generate a unique resource name from "agent-replay" and store it as "agentHandle"
    And I generate a unique API context from "/agent-replay" and store it as "agentContext"
    And I store the registered gateway id as "gatewayId"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                                                                                  |
      | displayName | Replayed Agent                                                                                      |
      | projectId   | ${CTX:projectHandle}                                                                                |
      | context     | ${CTX:agentContext}                                                                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                                                                        |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    And the response status code should be 201

    When I stop the gateway service "gateway-controller"
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    Then the response status code should be 201
    And the JSON response field "status" should be "DEPLOYING"
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentHandle}" until status 200
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    Then I send a "POST" request to "${CTX:agentContext}/v1/message:send" until status 200 with body:
      """
      {"message": {"messageId": "cp-replay", "role": "ROLE_USER", "parts": [{"text": "Plan a 2 day trip to Galle"}]}}
      """
    And the response body should contain "Trip plan for Galle: 2 days"

    Given I authenticate using basic auth as "admin"
    When I stop the gateway service "gateway-controller"
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 202
    And the JSON response field "status" should be "UNDEPLOYING"
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 404

  @cp14-17
  Scenario: A restarted gateway restores the active Agent proxy and its policy chain
    Given I generate a unique resource name from "agent-restart" and store it as "agentHandle"
    And I generate a unique API context from "/agent-restart" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                            | ${CTX:agentHandle}                                                                                                      |
      | displayName                   | Restarted Agent                                                                                                         |
      | projectId                     | ${CTX:projectHandle}                                                                                                    |
      | context                       | ${CTX:agentContext}                                                                                                     |
      | upstreamUrl                   | http://a2a-trip-planner:9099                                                                                            |
      | transports                    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                     |
      | a2a.operationConfigs.policies | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Chain","value":"restored"}]}}}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until header "X-Agent-Chain" is "restored"

    When I restart the "gateway-controller" service
    And I restart the "gateway-runtime" service
    Given I authenticate using basic auth as "admin"
    And I wait for the gateway controller health endpoint
    And I send a GET request to the router ready endpoint until status 200
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until header "X-Agent-Chain" is "restored"
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the response header "X-Agent-Chain" should be "restored"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    Then the response status code should be 200
    And the JSON response field "status" should be "DEPLOYED"

  # Keys issued, and keys revoked, while the gateway is away reach it only through the reconnect
  # backfill. Each is proven where it matters, by authenticating or failing to authenticate at the
  # restarted gateway, for both Agent proxy keys and REST API keys; the gateway internal API then
  # shows each kind's backfill is populated with exactly its own keys and their current status.
  @cp14-22
  Scenario: A reconnecting gateway restores Agent proxy and REST API key authentication from the backfill
    Given I generate a unique resource name from "agent-key-resync" and store it as "agentHandle"
    And I generate a unique API context from "/agent-key-resync" and store it as "agentContext"
    And I generate a unique resource name from "agent-resync-early" and store it as "agentEarlyKeyId"
    And I generate a unique value from "agent-resync-early-value" and store it as "agentEarlyKeyValue"
    And I generate a unique resource name from "agent-resync-key" and store it as "keyId"
    And I generate a unique value from "agent-resync-value" and store it as "keyValue"
    And I generate a unique resource name from "agent-resync-rest" and store it as "apiHandle"
    And I generate a unique API context from "/agent-resync-rest" and store it as "apiContext"
    And I generate a unique resource name from "rest-kept" and store it as "restKeptKeyId"
    And I generate a unique value from "rest-kept-value" and store it as "restKeptKeyValue"
    And I generate a unique resource name from "rest-revoked" and store it as "restRevokedKeyId"
    And I generate a unique value from "rest-revoked-value" and store it as "restRevokedKeyValue"
    And I generate a unique resource name from "rest-new" and store it as "restNewKeyId"
    And I generate a unique value from "rest-new-value" and store it as "restNewKeyValue"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                            | ${CTX:agentHandle}                                                                                  |
      | displayName                   | Resynced Key Agent                                                                                  |
      | projectId                     | ${CTX:projectHandle}                                                                                |
      | context                       | ${CTX:agentContext}                                                                                 |
      | upstreamUrl                   | http://a2a-trip-planner:9099                                                                        |
      | transports                    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]                   |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and API key authentication
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane

    # Keys that exist before the outage, each proven live at the gateway first.
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:agentEarlyKeyId}", "displayName": "Revoked while away", "apiKey": "${CTX:agentEarlyKeyValue}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/api-keys" with body:
      """
      {"id": "${CTX:restKeptKeyId}", "displayName": "Kept across the outage", "apiKey": "${CTX:restKeptKeyValue}"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/api-keys" with body:
      """
      {"id": "${CTX:restRevokedKeyId}", "displayName": "Revoked while away", "apiKey": "${CTX:restRevokedKeyValue}"}
      """
    Then the response status code should be 201
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:agentEarlyKeyValue}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I clear all headers
    And I set header "API-Key" to "${CTX:restKeptKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I set header "API-Key" to "${CTX:restRevokedKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200

    # While the gateway is away one key of each kind is issued and one is revoked.
    Given I authenticate using basic auth as "admin"
    When I stop the gateway service "gateway-controller"
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "Resynced key", "apiKey": "${CTX:keyValue}"}
      """
    Then the response status code should be 201
    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:agentEarlyKeyId}"
    Then the response status code should be 204
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/api-keys" with body:
      """
      {"id": "${CTX:restNewKeyId}", "displayName": "Issued while away", "apiKey": "${CTX:restNewKeyValue}"}
      """
    Then the response status code should be 201
    When I send a "DELETE" request to the control plane at "/rest-apis/${CTX:apiHandle}/api-keys/${CTX:restRevokedKeyId}"
    Then the response status code should be 204
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint

    # Agent proxy keys: the issued key authenticates on both bindings, the revoked one no longer does.
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I set header "API-Key" to "${CTX:keyValue}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:agentEarlyKeyValue}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401
    When I set header "API-Key" to "not-a-resynced-key"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 401

    # REST API keys: the kept and the newly issued keys authenticate, the revoked one and a missing
    # key do not.
    When I clear all headers
    And I set header "API-Key" to "${CTX:restNewKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I set header "API-Key" to "${CTX:restKeptKeyValue}"
    And I send a "GET" request to "${CTX:apiContext}/health"
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:restRevokedKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 401
    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/health"
    Then the response status code should be 401

    # Each kind's backfill carries exactly its own keys, a revocation included: revocation is a
    # status, so the revoked key is backfilled as revoked rather than dropped.
    Given I authenticate using basic auth as "admin"
    And I obtain an API key for the registered gateway via the control plane
    And I resolve the gateway internal artifact id of Agent proxy deployment "${CTX:deploymentId}" and store it as "agentUuid"
    When I send a "GET" request to the gateway internal API at "/agents/api-keys"
    Then the response status code should be 200
    And the JSON response array "" item with "name" equal to "${CTX:keyId}" should have "artifactUuid" equal to "${CTX:agentUuid}"
    And the JSON response array "" item with "name" equal to "${CTX:keyId}" should have "status" equal to "active"
    And the JSON response array "" item with "name" equal to "${CTX:agentEarlyKeyId}" should have "artifactUuid" equal to "${CTX:agentUuid}"
    And the JSON response array "" item with "name" equal to "${CTX:agentEarlyKeyId}" should have "status" equal to "revoked"
    And the JSON response array "" should not contain an item with "name" equal to "${CTX:restKeptKeyId}"
    When I send a "GET" request to the gateway internal API at "/apis/api-keys"
    Then the response status code should be 200
    And the JSON response array "" item with "name" equal to "${CTX:restKeptKeyId}" should have "status" equal to "active"
    And the JSON response array "" item with "name" equal to "${CTX:restNewKeyId}" should have "status" equal to "active"
    And the JSON response array "" item with "name" equal to "${CTX:restRevokedKeyId}" should have "status" equal to "revoked"
    And the JSON response array "" should not contain an item with "name" equal to "${CTX:keyId}"

  @cp14-10
  Scenario: A control-plane restart empties the Agent Card display cache
    Given I generate a unique resource name from "card-restart" and store it as "cardScope"
    And I generate a unique resource name from "card-restart-agent" and store it as "agentHandle"
    And I generate a unique API context from "/card-restart" and store it as "agentContext"
    And I store the Agent Card fixture URL for scope "${CTX:cardScope}" in mode "ok" as "okUrl"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Restarted Cache Agent                            |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | ${CTX:okUrl}                                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    And the response status code should be 200
    And I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    And the response status code should be 200
    And the Agent Card fixture should have received 1 request for scope "${CTX:cardScope}"

    When I restart the "platform-api" service
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}" until the JSON field "id" is "${CTX:agentHandle}"
    And I send a "POST" request to the control plane at "/agent-proxies/fetch-agent-card" with body:
      """
      {"agentProxyId": "${CTX:agentHandle}"}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Card ${CTX:cardScope}"
    And the response header "Age" should be "0"
    And the Agent Card fixture should have received 2 requests for scope "${CTX:cardScope}"
