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

@agent-proxy @agent-proxy-deployment
Feature: Agent proxies authored in the control plane are deployed to and served by the gateway
  As an API publisher
  I want an Agent proxy deployed through the control plane to be fetched, acknowledged and served
  by the gateway on both A2A bindings
  So that the control plane and the data plane agree on what is deployed and what callers reach

  # The control plane does not check the target gateway's version (Section 12 as implemented): a
  # gateway that predates Agent support never acknowledges the deployment, which then ends FAILED
  # with DEPLOYMENT_TIMEOUT. There is therefore no below-minimum refusal to assert here; the
  # source-built gateway in this block supports the Agent kind.
  #
  # A gateway fetching an Agent deployed only on another gateway, event payload shapes and the
  # derived event actions are covered by the focused platform-api tests named in the Section 14
  # coverage record; this block registers a single gateway.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "agent-deploy-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @cp14-11 @type:smoke
  Scenario: A deployed Agent proxy is invoked through the gateway on both bindings with its policies enforced
    Given I generate a unique resource name from "agent-e2e" and store it as "agentHandle"
    And I generate a unique API context from "/agent-e2e" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                              | ${CTX:agentHandle}                                                                                                                                           |
      | displayName                     | End To End Agent                                                                                                                                             |
      | projectId                       | ${CTX:projectHandle}                                                                                                                                         |
      | context                         | ${CTX:agentContext}                                                                                                                                          |
      | upstreamUrl                     | http://a2a-trip-planner:9099                                                                                                                                 |
      | transports                      | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                          |
      | a2a.operationConfigs.policies   | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Chain","value":"common"}]}}}]                                        |
      | a2a.operationConfigs.operations | [{"name":"SendMessage","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Op","value":"send-message"}]}}}]}] |
    And the response status code should be 201

    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    And the JSON response field "status" should be "DEPLOYING"
    And the JSON response field "gatewayId" should be "it-gateway"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    Then the JSON response field "deploymentId" should be "${CTX:deploymentId}"
    And the JSON response field "statusReason" should not exist
    And I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentHandle}" until status 200

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}/v1/message:send" until status 200 with body:
      """
      {"message": {"messageId": "cp-e2e-rest", "role": "ROLE_USER", "parts": [{"text": "Plan a 2 day trip to Galle"}]}}
      """
    Then the response body should contain "Trip plan for Galle: 2 days"
    And the response body should contain "Day 2: Botanical gardens in Galle"
    And the response header "X-Agent-Chain" should be "common"
    And the response header "X-Agent-Op" should be "send-message"

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 7, "method": "SendMessage", "params": {"message": {"messageId": "cp-e2e-rpc", "role": "ROLE_USER", "parts": [{"text": "Plan a 2 day trip to Galle"}]}}}
      """
    Then the response status code should be 200
    And the JSON response field "jsonrpc" should be "2.0"
    And the JSON response field "id" should be 7
    And the response body should contain "Trip plan for Galle: 2 days"
    And the response header "X-Agent-Chain" should be "common"
    And the response header "X-Agent-Op" should be "send-message"

    # Per-operation policies are additions for their operation only.
    When I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 200
    And the response header "X-Agent-Chain" should be "common"
    And the response header "X-Agent-Op" should not exist
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 8, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"
    And the response header "X-Agent-Chain" should be "common"
    And the response header "X-Agent-Op" should not exist

  @cp14-12
  Scenario: The gateway fetches an immutable artifact in its own vocabulary
    Given I generate a unique resource name from "agent-artifact" and store it as "agentHandle"
    And I generate a unique resource name from "agent-artifact-min" and store it as "minimalHandle"
    And I generate a unique API context from "/agent-artifact" and store it as "agentContext"
    And I generate a unique API context from "/agent-artifact-min" and store it as "minimalContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id            | ${CTX:agentHandle}                                                                                  |
      | displayName   | Artifact Agent                                                                                      |
      | projectId     | ${CTX:projectHandle}                                                                                |
      | context       | ${CTX:agentContext}                                                                                 |
      | upstreamUrl   | http://a2a-trip-planner:9099                                                                        |
      | transports    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | a2a.agentCard | {"public":{"mode":"passthrough","rewriteUrls":false}}                                               |
    And the response status code should be 201
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:minimalHandle}                             |
      | displayName | Minimal Artifact Agent                           |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:minimalContext}                            |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I deploy the Agent proxy "${CTX:minimalHandle}" to the gateway via the control plane and store the deployment id as "minimalDeployment"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:minimalHandle}/deployments/${CTX:minimalDeployment}" until the JSON field "status" is "DEPLOYED"
    And I obtain an API key for the registered gateway via the control plane
    And I resolve the gateway internal artifact id of Agent proxy deployment "${CTX:deploymentId}" and store it as "agentUuid"
    And I resolve the gateway internal artifact id of Agent proxy deployment "${CTX:minimalDeployment}" and store it as "minimalUuid"

    When I send a "GET" request to the gateway internal API at "/agents/${CTX:agentUuid}"
    Then the response status code should be 200
    And the response header "Content-Type" should be "application/zip"
    And the response header "Content-Disposition" should match pattern "^attachment; filename=.agent-${CTX:agentUuid}\.zip.$"
    And the response should be a ZIP archive containing only "agent-${CTX:agentUuid}.yaml"
    And the archived deployment YAML field "kind" should be "Agent"
    And the archived deployment YAML field "apiVersion" should be "gateway.api-platform.wso2.com/v1"
    And the archived deployment YAML field "metadata.name" should be "${CTX:agentHandle}"
    And the archived deployment YAML field "spec.displayName" should be "Artifact Agent"
    And the archived deployment YAML field "spec.version" should be "v1.0"
    And the archived deployment YAML field "spec.context" should be "${CTX:agentContext}"
    And the archived deployment YAML field "spec.a2a.protocolVersion" should be "1.0"
    And the archived deployment YAML field "spec.a2a.operationConfigs.transports[0].protocolBinding" should be "JSONRPC"
    And the archived deployment YAML field "spec.a2a.operationConfigs.transports[1].protocolBinding" should be "HTTP+JSON"
    And the archived deployment YAML field "spec.a2a.operationConfigs.transports[1].pathPrefix" should be "/v1"
    And the archived deployment YAML field "spec.a2a.agentCard.public.mode" should be "passthrough"
    And the archived deployment YAML field "spec.a2a.agentCard.public.rewriteUrls" should be "false"
    And the archived deployment YAML field "spec.a2a.agentCard.protected" should not exist
    And the archived deployment YAML field "spec.a2a.transports" should not exist
    And the archived deployment YAML field "spec.protocol" should not exist
    And the archived deployment YAML field "spec.associatedGateways" should not exist
    And the archived deployment YAML field "spec.projectId" should not exist

    When I send a "GET" request to the gateway internal API at "/agents/${CTX:minimalUuid}"
    Then the response status code should be 200
    And the response should be a ZIP archive containing only "agent-${CTX:minimalUuid}.yaml"
    And the archived deployment YAML field "spec.a2a.agentCard" should not exist
    And the archived deployment YAML field "spec.a2a.operationConfigs.transports[0].protocolBinding" should be "JSONRPC"
    And the archived deployment YAML field "spec.a2a.operationConfigs.policies" should not exist
    And the archived deployment YAML field "spec.a2a.operationConfigs.operations" should not exist

    When I fetch the gateway deployment batch for deployment "${CTX:deploymentId}" via the gateway internal API
    Then the response status code should be 200
    And the response should be a gzip tar archive containing only "${CTX:deploymentId}/agent-${CTX:agentUuid}.yaml"
    And the archived deployment YAML field "kind" should be "Agent"
    And the archived deployment YAML field "metadata.name" should be "${CTX:agentHandle}"

    # A replacement does not reach the gateway until it is deployed.
    Given I authenticate using basic auth as "admin"
    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName | Artifact Agent Revised                                                                              |
      | projectId   | ${CTX:projectHandle}                                                                                |
      | context     | ${CTX:agentContext}                                                                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                                                                        |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    Then the response status code should be 200
    When I send a "GET" request to the gateway internal API at "/agents/${CTX:agentUuid}"
    Then the response status code should be 200
    And the archived deployment YAML field "spec.displayName" should be "Artifact Agent"
    And the archived deployment YAML field "spec.a2a.agentCard.public.rewriteUrls" should be "false"

    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "revisedDeployment"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:revisedDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to the gateway internal API at "/agents/${CTX:agentUuid}"
    Then the response status code should be 200
    And the archived deployment YAML field "spec.displayName" should be "Artifact Agent Revised"
    And the archived deployment YAML field "spec.a2a.agentCard" should not exist

  @cp14-15 @type:negative
  Scenario: The gateway internal API is gateway-authenticated and keyed by the internal identifier
    Given I generate a unique resource name from "agent-internal" and store it as "agentHandle"
    And I generate a unique API context from "/agent-internal" and store it as "agentContext"
    And I store the registered gateway id as "gatewayId"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Internal API Agent                               |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I obtain an API key for the registered gateway via the control plane
    And I resolve the gateway internal artifact id of Agent proxy deployment "${CTX:deploymentId}" and store it as "agentUuid"

    When I send a "GET" request to the gateway internal API at "/agents/${CTX:agentUuid}" without a gateway API key
    Then the response status code should be 401
    And the JSON response field "description" should be "API key is required. Provide 'api-key' header."

    # The public handle is not the identifier the gateway addresses the artifact by.
    When I send a "GET" request to the gateway internal API at "/agents/${CTX:agentHandle}"
    Then the response status code should be 404

    # The static key-sync route is never read as an artifact identifier.
    When I send a "GET" request to the gateway internal API at "/agents/api-keys"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 202
    When I send a "GET" request to the gateway internal API at "/agents/${CTX:agentUuid}"
    Then the response status code should be 404
    And the JSON response field "description" should be "No active deployment found for this Agent on this gateway"

  @cp14-13
  Scenario: A deployment is undeployed, restored and, once undeployed, deleted without deleting the Agent proxy
    Given I generate a unique resource name from "agent-lifecycle" and store it as "agentHandle"
    And I generate a unique API context from "/agent-lifecycle" and store it as "agentContext"
    And I store the registered gateway id as "gatewayId"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                                   |
      | displayName | Lifecycle Agent                                      |
      | projectId   | ${CTX:projectHandle}                                 |
      | context     | ${CTX:agentContext}                                  |
      | upstreamUrl | http://a2a-trip-planner:9099                         |
      | transports  | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments?gatewayId=${CTX:gatewayId}&status=DEPLOYED"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "list[0].deploymentId" should be "${CTX:deploymentId}"

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "DEPLOYMENT_ACTIVE"
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy"
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy?gatewayId=no-such-agent-gateway"
    Then the response status code should be 404
    And the JSON response field "code" should be "GATEWAY_NOT_FOUND"
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/3fa85f64-5717-4562-b3fc-2c963f66afa6/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "DEPLOYMENT_NOT_FOUND"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 202
    And the JSON response field "status" should be "UNDEPLOYING"
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 404
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "DEPLOYMENT_NOT_ACTIVE"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/restore?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 202
    And the JSON response field "status" should be "DEPLOYING"
    And the response header "Location" should be "/api/v0.9/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/restore?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "DEPLOYMENT_RESTORE_CONFLICT"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 202
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    And I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    Then the response status code should be 204
    And the response body should be empty
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "DEPLOYMENT_NOT_FOUND"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 200

  @cp14-13
  Scenario: Redeploying from a stored build or restoring a prior deployment ships that snapshot
    Given I generate a unique resource name from "agent-snapshot" and store it as "agentHandle"
    And I generate a unique API context from "/agent-snapshot" and store it as "agentContext"
    And I store the registered gateway id as "gatewayId"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id                            | ${CTX:agentHandle}                                                                                                      |
      | displayName                   | Snapshot Agent                                                                                                          |
      | projectId                     | ${CTX:projectHandle}                                                                                                    |
      | context                       | ${CTX:agentContext}                                                                                                     |
      | upstreamUrl                   | http://a2a-trip-planner:9099                                                                                            |
      | transports                    | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                    |
      | a2a.operationConfigs.policies | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Snapshot","value":"first"}]}}}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "firstDeployment"
    And I store the JSON response field "buildId" as "firstBuild"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:firstDeployment}" until the JSON field "status" is "DEPLOYED"
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until header "X-Agent-Snapshot" is "first"

    When I update the Agent proxy "${CTX:agentHandle}" via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | displayName                   | Snapshot Agent                                                                                                           |
      | projectId                     | ${CTX:projectHandle}                                                                                                     |
      | context                       | ${CTX:agentContext}                                                                                                      |
      | upstreamUrl                   | http://a2a-trip-planner:9099                                                                                             |
      | transports                    | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                     |
      | a2a.operationConfigs.policies | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Agent-Snapshot","value":"second"}]}}}] |
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "secondDeployment"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:secondDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until header "X-Agent-Snapshot" is "second"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:firstDeployment}"
    Then the response status code should be 200
    And the JSON response field "status" should be "ARCHIVED"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments" with body:
      """
      {"name": "${UNIQUE:agent-build-dep}", "base": "build", "buildId": "${CTX:firstBuild}", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "DEPLOYING"
    And I store the JSON response field "deploymentId" as "buildDeployment"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:buildDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until header "X-Agent-Snapshot" is "first"

    When I send a "POST" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:secondDeployment}/restore?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 202
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:secondDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:agentContext}/v1/tasks" until header "X-Agent-Snapshot" is "second"

    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments?limit=1"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "pagination.total" should be 3
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments?status=ARCHIVED"
    Then the response status code should be 200
    And the JSON response field "count" should be 2
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments?status=RUNNING"
    Then the response status code should be 400
    And the JSON response field "code" should be "DEPLOYMENT_INVALID_STATUS"

  @cp14-13 @type:negative
  Scenario Outline: A malformed deployment request is rejected without storing a deployment: <case>
    Given I generate a unique resource name from "agent-bad-deploy" and store it as "agentHandle"
    And I generate a unique API context from "/agent-bad-deploy" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                               |
      | displayName | Bad Deploy Agent                                 |
      | projectId   | ${CTX:projectHandle}                             |
      | context     | ${CTX:agentContext}                              |
      | upstreamUrl | http://a2a-trip-planner:9099                     |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/agent-proxies/<target>/deployments" with body:
      """
      <body>
      """
    Then the response status code should be <status>
    And the JSON response field "code" should be "<code>"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments"
    Then the response status code should be 200
    And the JSON response field "count" should be 0

    Examples:
      | case                        | target              | status | code                                     | body                                                                                  |
      | missing name                | ${CTX:agentHandle}  | 400    | AGENT_PROXY_DEPLOYMENT_VALIDATION_FAILED | {"base": "current", "gatewayId": "it-gateway"}                                        |
      | missing base                | ${CTX:agentHandle}  | 400    | AGENT_PROXY_DEPLOYMENT_VALIDATION_FAILED | {"name": "bad", "gatewayId": "it-gateway"}                                            |
      | deployment id as base       | ${CTX:agentHandle}  | 400    | AGENT_PROXY_DEPLOYMENT_VALIDATION_FAILED | {"name": "bad", "base": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "gatewayId": "it-gateway"} |
      | build base without buildId  | ${CTX:agentHandle}  | 400    | AGENT_PROXY_DEPLOYMENT_VALIDATION_FAILED | {"name": "bad", "base": "build", "gatewayId": "it-gateway"}                           |
      | missing gatewayId           | ${CTX:agentHandle}  | 400    | AGENT_PROXY_DEPLOYMENT_VALIDATION_FAILED | {"name": "bad", "base": "current"}                                                    |
      | malformed body              | ${CTX:agentHandle}  | 400    | VALIDATION_FAILED                        | {"name":                                                                              |
      | unknown build               | ${CTX:agentHandle}  | 404    | BUILD_NOT_FOUND                          | {"name": "bad", "base": "build", "buildId": "no-such-build", "gatewayId": "it-gateway"} |
      | unknown gateway             | ${CTX:agentHandle}  | 404    | GATEWAY_NOT_FOUND                        | {"name": "bad", "base": "current", "gatewayId": "no-such-agent-gateway"}              |
      | unknown Agent proxy         | no-such-agent-proxy | 404    | AGENT_PROXY_NOT_FOUND                    | {"name": "bad", "base": "current", "gatewayId": "it-gateway"}                         |

  @cp14-14
  Scenario: A configuration the control plane accepts but the gateway rejects is reported through the acknowledgement
    Given I generate a unique resource name from "agent-gw-invalid" and store it as "agentHandle"
    And I generate a unique API context from "/agent-gw-invalid" and store it as "agentContext"
    # The managed card advertises only the JSON-RPC interface while both transports are exposed. The
    # control plane checks the card's required keys; interface agreement is the gateway's rule.
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id            | ${CTX:agentHandle}                                                                                                                                                                                                                                                                                                                                                         |
      | displayName   | Gateway Invalid Agent                                                                                                                                                                                                                                                                                                                                                      |
      | projectId     | ${CTX:projectHandle}                                                                                                                                                                                                                                                                                                                                                       |
      | context       | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                        |
      | upstreamUrl   | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                               |
      | transports    | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                        |
      | a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Half Advertised","description":"Advertises one of two transports","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://gateway.example.com${CTX:agentContext}"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}]}}} |
    Then the response status code should be 201
    When I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    Then the response status code should be 201
    And the JSON response field "status" should be "DEPLOYING"
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "FAILED"
    Then the JSON response field "statusReason" should be "AGENT_VALIDATION_FAILED"
    And the response body should not contain "supportedInterfaces"
    When I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentHandle}"
    Then the response status code should be 404

  @cp14-16
  Scenario: Deleting a deployed Agent proxy removes its routes from the gateway after the row is gone
    Given I generate a unique resource name from "agent-delete" and store it as "agentHandle"
    And I generate a unique API context from "/agent-delete" and store it as "agentContext"
    And I create an Agent proxy via the control plane from "resources/templates/agent-proxy.yaml" with values:
      | id          | ${CTX:agentHandle}                                                                                  |
      | displayName | Deleted Agent                                                                                       |
      | projectId   | ${CTX:projectHandle}                                                                                |
      | context     | ${CTX:agentContext}                                                                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                                                                        |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    And the response status code should be 201
    And I deploy the Agent proxy "${CTX:agentHandle}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentHandle}" until status 200
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I send a "DELETE" request to the control plane at "/agent-proxies/${CTX:agentHandle}"
    Then the response status code should be 204
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 404
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 404
    Given I authenticate using basic auth as "admin"
    Then I send a "GET" request to the "gateway-controller" service at "/agents/${CTX:agentHandle}" until status 404
    When I send a "GET" request to the control plane at "/agent-proxies/${CTX:agentHandle}/deployments"
    Then the response status code should be 404
    And the JSON response field "code" should be "AGENT_PROXY_NOT_FOUND"
