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

@agent-proxy @agent-proxy-import
Feature: Agents created on a gateway are imported into the control plane as read-only Agent proxies
  As a platform operator
  I want an Agent created directly on a gateway to appear in the control plane
  So that gateway-authored A2A agents are visible, keyed and governed centrally without being editable there

  # The gateway pushes kind Agent; the control plane stores it as kind AgentProxy with protocol a2a
  # and the same a2a layout, so spec.a2a.operationConfigs.transports becomes
  # a2a.operationConfigs.transports. The mapping edge cases (strict
  # refusals, dropped fields, last-in-wins, the builder inverse) are covered by the platform-api
  # package tests in internal/service/artifact_import_agent_proxy_test.go; these scenarios cover the
  # live gateway -> control plane path.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "agent-import-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @cp15-01 @type:smoke
  Scenario: A gateway-created Agent is imported as a read-only A2A Agent proxy
    Given I generate a unique resource name from "agent-import" and store it as "agentName"
    And I generate a unique API context from "/agent-import" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                           | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | name                                 | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | displayName                          | Imported Agent                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | context                              | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | upstreamUrl                          | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | transports                           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | metadata.annotations                 | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | spec.upstream.auth                   | {"type":"api-key","header":"X-Upstream-Key","value":"imported-credential-never-echoed"}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | spec.a2a.operationConfigs.operations | [{"name":"SendMessage","resilience":{"timeout":"30s"}}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | spec.a2a.agentCard                   | {"public":{"mode":"managed","content":{"name":"Imported Trip Planner Card","description":"Authored on the gateway","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}],"x-vendor-custom":{"kept":true}}},"protected":{"mode":"passthrough","rewriteUrls":false}} |
    Then the response should be successful
    And the control plane should receive the "Agent" artifact "${CTX:agentName}"
    And the control plane copy of the "Agent" artifact "${CTX:agentName}" should be marked as gateway-originated
    And the control plane should have deployed the "Agent" artifact "${CTX:agentName}"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:agentName}"
    And the JSON response field "kind" should be "AgentProxy"
    And the JSON response field "protocol" should be "a2a"
    And the JSON response field "readOnly" should be "true"
    And the JSON response field "displayName" should be "Imported Agent"
    And the JSON response field "version" should be "v1.0"
    And the JSON response field "projectId" should be "${CTX:projectHandle}"
    And the JSON response field "context" should be "${CTX:agentContext}"
    And the JSON response field "upstream.main.url" should be "http://a2a-trip-planner:9099"
    And the JSON response field "upstream.main.auth.header" should be "X-Upstream-Key"
    And the JSON response field "upstream.main.auth.value" should not exist
    And the response body should not contain "imported-credential-never-echoed"
    And the JSON response field "a2a.protocolVersion" should be "1.0"
    And the JSON response array field "a2a.operationConfigs.transports" should have 2 items
    And the JSON response field "a2a.operationConfigs.transports[0].protocolBinding" should be "JSONRPC"
    And the JSON response field "a2a.operationConfigs.transports[1].protocolBinding" should be "HTTP+JSON"
    And the JSON response field "a2a.operationConfigs.transports[1].pathPrefix" should be "/v1"
    And the JSON response field "a2a.operationConfigs.operations[0].name" should be "SendMessage"
    And the JSON response field "a2a.operationConfigs.operations[0].resilience.timeout" should be "30s"
    And the JSON response field "a2a.agentCard.public.mode" should be "managed"
    And the JSON response field "a2a.agentCard.public.content.name" should be "Imported Trip Planner Card"
    And the JSON response field "a2a.agentCard.public.content.x-vendor-custom.kept" should be "true"
    And the JSON response field "a2a.agentCard.protected.mode" should be "passthrough"
    And the JSON response field "a2a.agentCard.protected.rewriteUrls" should be "false"
    And the JSON response field "configuration" should not exist
    And the JSON response field "spec" should not exist

    When I send a "GET" request to the control plane at "/agent-proxies?protocol=a2a&limit=100"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:agentName}" should have "readOnly" equal to "true"
    And the JSON response array "list" item with "id" equal to "${CTX:agentName}" should have "projectId" equal to "${CTX:projectHandle}"

  @cp15-02
  Scenario: The control plane refuses to edit, redeploy or delete a deployed gateway-originated Agent proxy
    Given I generate a unique resource name from "agent-import-ro" and store it as "agentName"
    And I generate a unique API context from "/agent-import-ro" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion           | ${CTX:gatewaySpecVersion}                                           |
      | name                 | ${CTX:agentName}                                                    |
      | displayName          | Read Only Agent                                                     |
      | context              | ${CTX:agentContext}                                                 |
      | upstreamUrl          | http://a2a-trip-planner:9099                                        |
      | transports           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                    |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should have deployed the "Agent" artifact "${CTX:agentName}"

    When I update the Agent proxy "${CTX:agentName}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentName}                                 |
      | displayName | Edited In The Control Plane                      |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    Then the response status code should be 403
    And the JSON response field "code" should be "ARTIFACT_READ_ONLY"

    Given I store the registered gateway id as "gatewayId"
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentName}/deployments" with body:
      """
      {"name": "cp-redeploy", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "ARTIFACT_READ_ONLY"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 409
    And the JSON response field "code" should be "ARTIFACT_DEPLOYED"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Read Only Agent"
    And the JSON response field "readOnly" should be "true"

  @cp15-03
  Scenario: Updating the Agent on the gateway updates its control-plane copy
    Given I generate a unique resource name from "agent-import-upd" and store it as "agentName"
    And I generate a unique API context from "/agent-import-upd" and store it as "agentContext"
    And I generate a unique value from "Agent After Update" and store it as "updatedDisplayName"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion           | ${CTX:gatewaySpecVersion}                                           |
      | name                 | ${CTX:agentName}                                                    |
      | displayName          | Agent Before Update                                                 |
      | context              | ${CTX:agentContext}                                                 |
      | upstreamUrl          | http://a2a-trip-planner:9099                                        |
      | transports           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                    |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should receive the "Agent" artifact "${CTX:agentName}"

    When I update Agent "${CTX:agentName}" from "resources/templates/agent.yaml" with values:
      | apiVersion           | ${CTX:gatewaySpecVersion}                                                                            |
      | name                 | ${CTX:agentName}                                                                                     |
      | displayName          | ${CTX:updatedDisplayName}                                                                            |
      | context              | ${CTX:agentContext}                                                                                  |
      | upstreamUrl          | http://a2a-trip-planner:9099                                                                         |
      | transports           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]  |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"}                                  |
    Then the response should be successful
    And the control plane copy of the "Agent" artifact "${CTX:agentName}" configuration should contain "${CTX:updatedDisplayName}"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:updatedDisplayName}"
    And the JSON response array field "a2a.operationConfigs.transports" should have 2 items
    And the JSON response field "a2a.operationConfigs.transports[1].protocolBinding" should be "HTTP+JSON"
    And the JSON response field "readOnly" should be "true"

  @cp15-04
  Scenario: Deleting the Agent on the gateway leaves an undeployed control-plane copy that can then be deleted
    Given I generate a unique resource name from "agent-import-del" and store it as "agentName"
    And I generate a unique API context from "/agent-import-del" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion           | ${CTX:gatewaySpecVersion}                                           |
      | name                 | ${CTX:agentName}                                                    |
      | displayName          | Deleted On The Gateway                                              |
      | context              | ${CTX:agentContext}                                                 |
      | upstreamUrl          | http://a2a-trip-planner:9099                                        |
      | transports           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                    |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the control plane should have deployed the "Agent" artifact "${CTX:agentName}"

    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
    And the control plane should have undeployed the "Agent" artifact "${CTX:agentName}"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 200
    And the JSON response field "readOnly" should be "true"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 204
    And the response body should be empty
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"

  @cp15-05
  Scenario: Gateway configuration the control plane cannot represent is dropped rather than refused
    Given I generate a unique resource name from "agent-import-lossy" and store it as "agentName"
    And I generate a unique API context from "/agent-import-lossy" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                          |
      | name                     | ${CTX:agentName}                                                                   |
      | displayName              | Lossy Agent                                                                        |
      | context                  | ${CTX:agentContext}                                                                |
      | upstreamUrl              | http://a2a-trip-planner:9099                                                       |
      | transports               | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                                   |
      | metadata.annotations     | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"}                |
      | spec.upstream            | {"ref":"trip-planner","hostRewrite":"manual"}                                      |
      | spec.upstreamDefinitions | [{"name":"trip-planner","upstreams":[{"url":"http://a2a-trip-planner:9099"}]}]     |
    Then the response should be successful
    And the control plane should receive the "Agent" artifact "${CTX:agentName}"
    And the control plane should have deployed the "Agent" artifact "${CTX:agentName}"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentName}"
    Then the response status code should be 200
    And the JSON response field "readOnly" should be "true"
    And the JSON response field "upstream.main.ref" should be "trip-planner"
    And the JSON response field "upstream.main.url" should not exist
    And the JSON response field "upstream.main.hostRewrite" should not exist
    And the JSON response field "upstreamDefinitions" should not exist
    And the "platform-api" service logs should contain "Gateway Agent carries configuration the control plane cannot represent"

  @cp15-06
  Scenario: An API key issued in the control plane for an imported Agent authenticates at the gateway
    Given I generate a unique resource name from "agent-import-key" and store it as "agentName"
    And I generate a unique API context from "/agent-import-key" and store it as "agentContext"
    And I generate a unique resource name from "agent-import-key-id" and store it as "keyId"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                           |
      | name                               | ${CTX:agentName}                                                                                    |
      | displayName                        | Keyed Imported Agent                                                                                |
      | context                            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                        |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | metadata.annotations               | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"}                                 |
      | spec.a2a.operationConfigs.policies | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]                   |
    Then the response should be successful
    And the control plane should have deployed the "Agent" artifact "${CTX:agentName}"
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # The key is associated with the control plane's own UUID for the Agent; the gateway resolves
    # that to its local Agent through the cp_artifact_id it recorded when the push succeeded.
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentName}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "Imported agent key"}
      """
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentName}/api-keys/${CTX:keyId}"
    And I store the JSON response field "apiKey" as "generatedKey"

    When I set header "API-Key" to "${CTX:generatedKey}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "jsonrpc" should be "2.0"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentName}/api-keys/${CTX:keyId}"
    Then the response status code should be 204
    When I set header "API-Key" to "${CTX:generatedKey}"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

  # "fetch-agent-card" is the subject of this scenario, so it cannot be generated. Only this runner
  # creates it, each block runs against its own gateway and control plane, and it is cleaned up
  # like any other gateway resource.
  @cp15-07 @type:negative
  Scenario: A gateway Agent that takes a handle the control plane reserves is not imported
    Given I generate a unique API context from "/agent-import-reserved" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion           | ${CTX:gatewaySpecVersion}                                           |
      | name                 | fetch-agent-card                                                    |
      | displayName          | Reserved Handle Agent                                               |
      | context              | ${CTX:agentContext}                                                 |
      | upstreamUrl          | http://a2a-trip-planner:9099                                        |
      | transports           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                    |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"${CTX:projectHandle}"} |
    Then the response should be successful
    And the "platform-api" service logs should contain "is reserved by the control plane"
    And the control plane should not receive the "Agent" artifact "fetch-agent-card"
