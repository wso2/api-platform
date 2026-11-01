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
Feature: Only an undeployed deployment can be deleted
  As an API publisher
  I want to delete deployments I no longer need from an artifact's history, but never a live one
  So that clearing history cannot take an artifact off its gateway

  Background:
    Given I generate a unique resource name from "deplc-proj" and store it as "project"
    And I generate a unique resource name from "deplc-art" and store it as "artifact"
    And I generate a unique API context from "/deplc" and store it as "artifactContext"
    And I create a project "${CTX:project}" on the control plane
    And I store the registered gateway id as "gatewayId"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"

  @DEPLC-004 @type:negative
  Scenario Outline: Deleting a deployed deployment is refused, and it stays listed and served
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to "${CTX:artifactContext}<probe>" until status 200 with body:
      """
      <body>
      """
    When I send a "DELETE" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "DEPLOYMENT_ACTIVE"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments"
    Then the response status code should be 200
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

    # GraphQL deployments need a gateway newer than 1.2.0; older gateway legs skip @graphql rows.
    @graphql
    Examples:
      | kind       | collection   | probe | body                    |
      | GraphQLApi | graphql-apis |       | {"query":"{ hello }"} |

  @DEPLC-005
  Scenario Outline: Deleting an undeployed deployment removes it from the artifact's history
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    And the response status code should be 200
    # Wait for the gateway's acknowledgement, so the delete meets a settled deployment.
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    When I send a "DELETE" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deploymentId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "DEPLOYMENT_NOT_FOUND"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "deploymentId" equal to "${CTX:deploymentId}"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}"
    Then the response status code should be 200

    Examples:
      | kind        | collection    |
      | RestApi     | rest-apis     |
      | LlmProvider | llm-providers |
      | LlmProxy    | llm-proxies   |
      | Mcp         | mcp-proxies   |

    @graphql
    Examples:
      | kind       | collection   |
      | GraphQLApi | graphql-apis |
