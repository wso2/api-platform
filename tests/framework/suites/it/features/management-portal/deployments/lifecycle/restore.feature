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
Feature: Restoring brings an undeployed deployment back, and only an undeployed one
  As an API publisher
  I want to restore a deployment I undeployed, the same way for every kind of artifact
  So that I can bring back exactly the build that ran before without preparing a new one

  Background:
    Given I generate a unique resource name from "deplc-proj" and store it as "project"
    And I generate a unique resource name from "deplc-art" and store it as "artifact"
    And I generate a unique API context from "/deplc" and store it as "artifactContext"
    And I create a project "${CTX:project}" on the control plane
    And I store the registered gateway id as "gatewayId"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"

  @DEPLC-002
  Scenario Outline: Restoring an undeployed deployment serves the same build again
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I store the JSON response field "buildId" as "buildId"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """
    And I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    And the response status code should be 200
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 404 with body:
      """
      <body>
      """
    When I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/restore?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 200
    And the JSON response field "deploymentId" should be "${CTX:deploymentId}"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And the JSON response field "buildId" should be "${CTX:buildId}"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """

    Examples:
      | kind        | collection    | probe             | body                                                                                                                               |
      | LlmProvider | llm-providers | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | LlmProxy    | llm-proxies   | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | Mcp         | mcp-proxies   | /mcp              | {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"deplc","version":"1.0.0"}}} |
      | RestApi     | rest-apis     | /health           | {}                                                                                                                                 |

    # GraphQL deployments need a gateway newer than 1.2.0; older gateway legs skip @graphql rows.
    @graphql
    Examples:
      | kind       | collection   | probe | body                    |
      | GraphQLApi | graphql-apis |       | {"query":"{ hello }"} |

  @DEPLC-003 @type:negative
  Scenario Outline: Restoring a deployment that is already deployed is refused and changes nothing
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """
    When I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/restore?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "DEPLOYMENT_RESTORE_CONFLICT"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deploymentId}" should have "status" equal to "DEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """

    Examples:
      | kind        | collection    | probe             | body                                                                                                                               |
      | LlmProvider | llm-providers | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | LlmProxy    | llm-proxies   | /chat/completions | {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}                                                                  |
      | Mcp         | mcp-proxies   | /mcp              | {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"deplc","version":"1.0.0"}}} |
      | RestApi     | rest-apis     | /health           | {}                                                                                                                                 |

    # GraphQL deployments need a gateway newer than 1.2.0; older gateway legs skip @graphql rows.
    @graphql
    Examples:
      | kind       | collection   | probe | body                    |
      | GraphQLApi | graphql-apis |       | {"query":"{ hello }"} |
