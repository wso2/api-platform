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

@agent-proxy @agent-proxy-api-keys
Feature: Agent proxy API keys are managed in the control plane and enforced by the gateway
  As an API publisher
  I want to issue, rotate and revoke API keys for an Agent proxy through the control plane
  So that callers authenticate to the agent at the gateway exactly as they do to a REST API

  # The actors are the suite's control-plane users: the administrator holds ap:agent_proxy:manage
  # and the ap:api_key:all:manage key-admin override, the publisher holds ap:agent_proxy:manage and
  # ap:api_key:read only, and the developer holds ap:agent_proxy:read only. Each fine-grained
  # ap:agent_proxy:api_key:* alternative on its own, cross-organization isolation and the 16 KiB
  # body ceiling are covered by the focused platform-api tests named in the Section 14 coverage
  # record.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "agent-key-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @cp14-19 @type:smoke
  Scenario: An API key is created, rotated and revoked, and the gateway follows each change on both bindings
    Given I generate a unique resource name from "agent-keys" and store it as "agentHandle"
    And I generate a unique API context from "/agent-keys" and store it as "agentContext"
    And I generate a unique resource name from "agent-key" and store it as "keyId"
    And I generate a unique value from "agent-key-rotated" and store it as "rotatedKey"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                            | ${CTX:agentHandle}                                                                                  |
      | displayName                   | Key Agent                                                                                           |
      | projectId                     | ${CTX:projectHandle}                                                                                |
      | context                       | ${CTX:agentContext}                                                                                 |
      | upstreamUrl                   | http://a2a-trip-planner:9099                                                                        |
      | transports                    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]                   |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "Generated key"}
      """
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}"
    And the JSON response field "status" should be "success"
    And the JSON response field "keyId" should be "${CTX:keyId}"
    And the JSON response string field "apiKey" should have length greater than 20
    And I store the JSON response field "apiKey" as "generatedKey"

    When I set header "API-Key" to "${CTX:generatedKey}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "jsonrpc" should be "2.0"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "status" equal to "active"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "displayName" equal to "Generated key"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should not have field "apiKey"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should not have field "apiKeyHashes"
    And the response body should not contain "${CTX:generatedKey}"

    # A PUT is a rotation to the supplied key, and repeating it leaves the same key in place.
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}" with body:
      """
      {"name": "${CTX:keyId}", "displayName": "Generated key", "apiKey": "${CTX:rotatedKey}"}
      """
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "keyId" should be "${CTX:keyId}"
    And the response body should not contain "${CTX:rotatedKey}"
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}" with body:
      """
      {"name": "${CTX:keyId}", "displayName": "Generated key", "apiKey": "${CTX:rotatedKey}"}
      """
    Then the response status code should be 200

    When I set header "API-Key" to "${CTX:rotatedKey}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 2, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:generatedKey}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}"
    Then the response status code should be 204
    And the response body should be empty
    When I set header "API-Key" to "${CTX:rotatedKey}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 3, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 401

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "status" equal to "revoked"
    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/no-such-agent-key"
    Then the response status code should be 404
    And the JSON response field "code" should be "REST_API_API_KEY_NOT_FOUND"

  @cp14-19
  Scenario: A taken key id is suffixed and an injected key is never echoed
    Given I generate a unique resource name from "agent-key-ids" and store it as "agentHandle"
    And I generate a unique API context from "/agent-key-ids" and store it as "agentContext"
    And I generate a unique resource name from "agent-key-taken" and store it as "keyId"
    And I generate a unique value from "agent-key-injected" and store it as "injectedKey"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Key Id Agent                                     |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "First key"}
      """
    And the response status code should be 201

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "Second key", "apiKey": "${CTX:injectedKey}"}
      """
    Then the response status code should be 201
    And the JSON response field "keyId" should not equal "${CTX:keyId}"
    And the JSON response field "keyId" should contain "${CTX:keyId}-"
    And I store the JSON response field "keyId" as "suffixedKeyId"
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:suffixedKeyId}"
    And the JSON response field "apiKey" should not exist
    And the response body should not contain "${CTX:injectedKey}"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:suffixedKeyId}" should have "displayName" equal to "Second key"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "displayName" equal to "First key"

  @cp14-20 @type:negative
  Scenario: Keys are creator-scoped, key administration widens that, and read-only callers cannot mint keys
    Given I generate a unique resource name from "agent-key-owner" and store it as "agentHandle"
    And I generate a unique resource name from "agent-key-other" and store it as "otherHandle"
    And I generate a unique API context from "/agent-key-owner" and store it as "agentContext"
    And I generate a unique API context from "/agent-key-other" and store it as "otherContext"
    And I generate a unique resource name from "agent-key-pub" and store it as "publisherKey"
    And I generate a unique resource name from "agent-key-admin" and store it as "adminKey"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                                   |
      | displayName | Owned Key Agent                                      |
      | projectId   | ${CTX:projectHandle}                                 |
      | context     | ${CTX:agentContext}                                  |
      | upstreamUrl | http://a2a-trip-planner:9099                         |
      | transports  | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    And the response status code should be 201
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:otherHandle}                                   |
      | displayName | Other Key Agent                                      |
      | projectId   | ${CTX:projectHandle}                                 |
      | context     | ${CTX:otherContext}                                  |
      | upstreamUrl | http://a2a-trip-planner:9099                         |
      | transports  | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" as "publisher" with body:
      """
      {"id": "${CTX:publisherKey}", "displayName": "Publisher key"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:adminKey}", "displayName": "Admin key"}
      """
    Then the response status code should be 201

    # The Agent proxy's own manage scope authorizes the operation, not access to another user's key.
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" as "publisher"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:publisherKey}"
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:adminKey}"
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:adminKey}" as "publisher" with body:
      """
      {"name": "${CTX:adminKey}", "displayName": "Taken over", "apiKey": "publisher-takeover-0123456789"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "REST_API_API_KEY_FORBIDDEN"
    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:adminKey}" as "publisher"
    Then the response status code should be 403
    And the JSON response field "code" should be "REST_API_API_KEY_FORBIDDEN"

    # The developer can read Agent proxies but holds no key scope at all.
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" as "developer" with body:
      """
      {"displayName": "Denied key"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:publisherKey}" as "developer" with body:
      """
      {"name": "${CTX:publisherKey}", "displayName": "Denied", "apiKey": "developer-denied-0123456789"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:publisherKey}" as "developer"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

    # The key administrator sees and manages every user's key on the Agent proxy.
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:publisherKey}" should have "createdBy" equal to "publisher"
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:adminKey}"
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:publisherKey}" with body:
      """
      {"name": "${CTX:publisherKey}", "displayName": "Publisher key", "apiKey": "admin-rotated-0123456789"}
      """
    Then the response status code should be 200

    # A key is addressed only through its own Agent proxy.
    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:otherHandle}/api-keys/${CTX:publisherKey}"
    Then the response status code should be 404
    And the JSON response field "code" should be "REST_API_API_KEY_NOT_FOUND"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:otherHandle}/api-keys"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    When I send a "GET" request to the control plane at "/agent-proxies/no-such-agent-proxy/api-keys"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:publisherKey}"
    Then the response status code should be 204

  @cp14-20 @type:negative
  Scenario: Key requests are validated and need a gateway to update or revoke
    Given I generate a unique resource name from "agent-key-nogw" and store it as "agentHandle"
    And I generate a unique API context from "/agent-key-nogw" and store it as "agentContext"
    And I generate a unique resource name from "agent-key-stranded" and store it as "keyId"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Undeployed Key Agent                             |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "Stranded key"}
      """
    Then the response status code should be 201

    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}" with body:
      """
      {"name": "${CTX:keyId}", "displayName": "Stranded key", "apiKey": "stranded-rotation-0123456789"}
      """
    Then the response status code should be 503
    And the JSON response field "code" should be "GATEWAY_CONNECTION_UNAVAILABLE"
    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}"
    Then the response status code should be 503
    And the JSON response field "code" should be "GATEWAY_CONNECTION_UNAVAILABLE"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "status" equal to "active"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "missing-display-name"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"displayName": "Blank key", "apiKey": "  "}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with header "Content-Type" set to "application/x-www-form-urlencoded" with body:
      """
      {"displayName": "Form key"}
      """
    Then the response status code should be 415
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}" with body:
      """
      {"displayName": "No key value"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "PUT" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys/${CTX:keyId}" with body:
      """
      {"name": "${CTX:keyId}-renamed", "displayName": "Renamed", "apiKey": "renamed-value-0123456789"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "POST" request to the control plane at "/agent-proxies/no-such-agent-proxy/api-keys" with body:
      """
      {"displayName": "Orphan key"}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

  @cp14-21
  Scenario: Agent proxy keys appear in the caller's own key listing under the control-plane kind
    Given I generate a unique resource name from "agent-key-me" and store it as "agentHandle"
    And I generate a unique API context from "/agent-key-me" and store it as "agentContext"
    And I generate a unique resource name from "agent-key-mine" and store it as "publisherKey"
    And I generate a unique resource name from "agent-key-theirs" and store it as "adminKey"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Listed Key Agent                                 |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" as "publisher" with body:
      """
      {"id": "${CTX:publisherKey}", "displayName": "Publisher listed key"}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:adminKey}", "displayName": "Admin listed key"}
      """
    And the response status code should be 201

    When I send a "GET" request to the control plane at "/me/api-keys?type=AgentProxy" as "publisher"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:publisherKey}" should have "artifactType" equal to "AgentProxy"
    And the JSON response array "list" item with "id" equal to "${CTX:publisherKey}" should have "artifactId" equal to "${CTX:agentHandle}"
    And the JSON response array "list" item with "id" equal to "${CTX:publisherKey}" should not have field "apiKey"
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:adminKey}"

    When I send a "GET" request to the control plane at "/me/api-keys" as "publisher"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:publisherKey}" should have "artifactType" equal to "AgentProxy"

    When I send a "GET" request to the control plane at "/me/api-keys?type=RestApi" as "publisher"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:publisherKey}"

    # The key administrator's listing spans every user's keys in the organization.
    When I send a "GET" request to the control plane at "/me/api-keys?type=AgentProxy"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:publisherKey}" should have "createdBy" equal to "publisher"
    And the JSON response array "list" item with "id" equal to "${CTX:adminKey}" should have "artifactId" equal to "${CTX:agentHandle}"

  @cp14-21
  Scenario: The gateway key backfill serves Agent proxy keys on its own route, filtered by issuer
    Given I generate a unique resource name from "agent-key-sync" and store it as "agentHandle"
    And I generate a unique API context from "/agent-key-sync" and store it as "agentContext"
    And I generate a unique resource name from "agent-key-portal" and store it as "portalKey"
    And I generate a unique resource name from "agent-key-plain" and store it as "plainKey"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Synced Key Agent                                 |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:portalKey}", "displayName": "Portal key", "issuer": "api-platform-devportal"}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/api-keys" with body:
      """
      {"id": "${CTX:plainKey}", "displayName": "Plain key"}
      """
    And the response status code should be 201
    And I obtain an API key for the registered gateway via the control plane
    And I resolve the gateway internal artifact id of Agent proxy deployment "${CTX:deploymentId}" and store it as "agentUuid"

    When I send a "GET" request to the gateway internal API at "/agents/api-keys"
    Then the response status code should be 200
    And the JSON response array "" item with "name" equal to "${CTX:portalKey}" should have "artifactUuid" equal to "${CTX:agentUuid}"
    And the JSON response array "" item with "name" equal to "${CTX:plainKey}" should have "artifactUuid" equal to "${CTX:agentUuid}"

    When I send a "GET" request to the gateway internal API at "/agents/api-keys?issuer=api-platform-devportal"
    Then the response status code should be 200
    And the JSON response array "" should contain an item with "name" equal to "${CTX:portalKey}"
    And the JSON response array "" should not contain an item with "name" equal to "${CTX:plainKey}"

    # The backfill is per kind: the REST API route never serves an Agent proxy's keys.
    When I send a "GET" request to the gateway internal API at "/apis/api-keys"
    Then the response status code should be 200
    And the JSON response array "" should not contain an item with "name" equal to "${CTX:portalKey}"
    And the JSON response array "" should not contain an item with "name" equal to "${CTX:plainKey}"

    When I send a "GET" request to the gateway internal API at "/agents/api-keys" without a gateway API key
    Then the response status code should be 401
