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

@GENAPP
Feature: API keys are mapped to applications, and the gateway attributes their calls to the application
  As an application owner
  I want to map my API keys to an application
  So that the gateway knows which application each call comes from

  # Scenarios GENAPP-001 to 003 (scenarios/on-prem/GENAPP.yaml). The key belongs to an LLM provider on the
  # gateway, protected by API key security. The gateway puts the application of the calling key in
  # each analytics event (applicationName), so the analytics shows whether the gateway holds a
  # mapping. An application created by the steps is named after its handle.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I clear all headers
    And I generate a unique resource name from "genapp-project" and store it as "project"
    And I create a project "${CTX:project}" on the control plane
    And I generate a unique resource name from "genapp-app" and store it as "app"
    And I create an application "${CTX:app}" via the control plane in project "${CTX:project}"
    And I generate a unique resource name from "genapp-key" and store it as "validSecret"
    And I create a secret "${CTX:validSecret}" with value "sk-testbench-valid-key" via the control plane
    And I generate a unique value from "GENAPP Provider" and store it as "displayName"
    And I generate a unique API context from "/genapp" and store it as "providerContext"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                  |
      | template           | azure-openai                                                                        |
      | context            | ${CTX:providerContext}                                                              |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                               |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}          |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    And I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I generate a unique resource name from "genapp-consumer" and store it as "keyId"
    And I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"id": "${CTX:keyId}", "displayName": "GENAPP consumer key"}
      """
    And the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

  @GENAPP-001 @type:smoke
  Scenario: A key mapped to an application is listed under it, and the gateway attributes the key's calls to the application
    Given I send a "POST" request to "${CTX:providerContext}/chat/completions" until its analytics event has no metadata field "applicationName" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/api-keys" with body:
      """
      {"apiKeys": [{"keyId": "${CTX:keyId}", "associatedEntity": {"id": "${CTX:providerId}"}}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/api-keys"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response array "list" item with "keyId" equal to "${CTX:keyId}" should have "associatedEntity.id" equal to "${CTX:providerId}"
    And the JSON response array "list" item with "keyId" equal to "${CTX:keyId}" should have "associatedEntity.kind" equal to "LlmProvider"
    When I wait for the analytics collector to settle
    And I reset the analytics collector
    Then I send a "POST" request to "${CTX:providerContext}/chat/completions" until its analytics event has metadata field "applicationName" with value "${CTX:app}" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

  @GENAPP-002 @type:security
  Scenario: Only the key's creator or a key admin can map the key to an application
    # The developer may manage applications but is neither the key's creator nor a key admin. The
    # key admin is another user with the administrator's role, which holds ap:api_key:all:manage.
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/api-keys" as "developer" with body:
      """
      {"apiKeys": [{"keyId": "${CTX:keyId}", "associatedEntity": {"id": "${CTX:providerId}"}}]}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "APPLICATION_API_KEY_FORBIDDEN"
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/api-keys"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    Given I generate a UUID and store it as "keyAdminSub"
    And I mint a control-plane token stored as "keyAdminToken" for the "admin" role with claims:
      | sub | ${CTX:keyAdminSub} |
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/api-keys" with token "keyAdminToken" with body:
      """
      {"apiKeys": [{"keyId": "${CTX:keyId}", "associatedEntity": {"id": "${CTX:providerId}"}}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/api-keys"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response array "list" should contain an item with "keyId" equal to "${CTX:keyId}"

  @GENAPP-003
  Scenario: A removed mapping is no longer listed, and the gateway stops attributing the key's calls to the application
    Given I send a "POST" request to the control plane at "/applications/${CTX:app}/api-keys" with body:
      """
      {"apiKeys": [{"keyId": "${CTX:keyId}", "associatedEntity": {"id": "${CTX:providerId}"}}]}
      """
    And the response status code should be 200
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until its analytics event has metadata field "applicationName" with value "${CTX:app}" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "DELETE" request to the control plane at "/applications/${CTX:app}/api-keys/${CTX:keyId}?entityID=${CTX:providerId}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/api-keys"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    And the JSON response array "list" should not contain an item with "keyId" equal to "${CTX:keyId}"
    When I wait for the analytics collector to settle
    And I reset the analytics collector
    Then I send a "POST" request to "${CTX:providerContext}/chat/completions" until its analytics event has no metadata field "applicationName" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

  # The response to adding mappings should list the application's mappings, as the listing does.
  # It lists none, though its pagination counts them: the service windows the list with limit -1,
  # which the pagination helper treats as an empty window (#4117).
  @known-issue @type:regression
  Scenario: The response to mapping a key lists the application's mappings
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/api-keys" with body:
      """
      {"apiKeys": [{"keyId": "${CTX:keyId}", "associatedEntity": {"id": "${CTX:providerId}"}}]}
      """
    Then the response status code should be 200
    And the JSON response field "pagination.total" should be 1
    And the JSON response field "count" should be 1
    And the JSON response array "list" item with "keyId" equal to "${CTX:keyId}" should have "associatedEntity.id" equal to "${CTX:providerId}"
