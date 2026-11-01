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

@DEPLC
Feature: Undeploying takes an artifact off its gateway, and only off the gateway it runs on
  As an API publisher
  I want to undeploy any kind of artifact from the gateway that serves it
  So that I can stop traffic to it without deleting the artifact or its deployment history

  Background:
    Given I generate a unique resource name from "deplc-proj" and store it as "project"
    And I generate a unique resource name from "deplc-art" and store it as "artifact"
    And I generate a unique API context from "/deplc" and store it as "artifactContext"
    And I create a project "${CTX:project}" on the control plane
    And I store the registered gateway id as "gatewayId"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"

  @DEPLC-001
  Scenario Outline: Undeploying a deployed artifact stops the gateway serving it
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """
    When I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 200
    And the JSON response field "deploymentId" should be "${CTX:deploymentId}"
    And the JSON response field "status" should match pattern "^UNDEPLOY(ING|ED)$"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 404 with body:
      """
      <body>
      """

    Examples:
      | kind        | collection    | probe             | body                                                                                                                               |
      | LlmProvider | llm-providers | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | LlmProxy    | llm-proxies   | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | Mcp         | mcp-proxies   | /mcp              | {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"deplc","version":"1.0.0"}}} |

    # GraphQL deployments need a gateway newer than 1.2.0; older gateway legs skip @graphql rows.
    @graphql
    Examples:
      | kind       | collection   | probe | body                    |
      | GraphQLApi | graphql-apis |       | {"query":"{ hello }"} |

  @DEPLC-006 @type:negative
  Scenario Outline: Undeploying from another gateway is refused, and undeploying twice is refused the second time
    Given I generate a unique resource name from "deplc-other-gw" and store it as "otherGateway"
    And I register a gateway "${CTX:otherGateway}" on the control plane
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """
    When I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:otherGateway}"
    Then the response status code should be 400
    And the JSON response field "code" should be "DEPLOYMENT_GATEWAY_MISMATCH"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}"
    Then the response status code should be 200
    And the JSON response field "status" should be "DEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """
    When I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 200
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    When I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "DEPLOYMENT_NOT_ACTIVE"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}"
    Then the JSON response field "status" should be "UNDEPLOYED"

    Examples:
      | kind        | collection    | probe             | body                                                                                                                               |
      | LlmProvider | llm-providers | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | LlmProxy    | llm-proxies   | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | Mcp         | mcp-proxies   | /mcp              | {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"deplc","version":"1.0.0"}}} |

    # GraphQL deployments need a gateway newer than 1.2.0; older gateway legs skip @graphql rows.
    @graphql
    Examples:
      | kind       | collection   | probe | body                    |
      | GraphQLApi | graphql-apis |       | {"query":"{ hello }"} |
