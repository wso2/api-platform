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

@AIW-LLM @llm-provider-deploy
Feature: LLM providers are deployed to the gateway and serve traffic through it
  As an AI Workspace administrator
  I want a provider deployed through the control plane to serve chat completions on the gateway
  So that its configuration — upstream, credential, guardrails, limits — is what callers get

  # Sheet AIW-LLM, sub-features "Connection" (AIW-PRV-009), "Guardrails" (AIW-PRV-015,
  # AIW-PRV-016), "Rate limiting" (AIW-PRV-018), "Deploy", "Undeploy", "Redeploy"
  # (AIW-PRV-020, 022, 023) and "Invoke" (AIW-PRV-024, 025, 026), at the API level.
  # Each provider is set up the way the AI Workspace creates one — Azure OpenAI template,
  # API key security on X-API-Key — and fronts the testbench's keyed OpenAI-dialect mock,
  # which accepts only its valid key. Without a model in the request the OpenAI route answers
  # as gpt-4.1 and the Mistral route as mistral-small-latest, which tells the upstreams apart.
  # Redeploying means deploying the provider again; AIW-PRV-023 restores a stopped
  # deployment. The browser journeys are suites/ui/features/aiw_llm/provider_deployment.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I clear all headers
    And I store the registered gateway id as "gatewayId"
    And I generate a unique resource name from "j4-valid-key" and store it as "validSecret"
    And I create a secret "${CTX:validSecret}" with value "sk-testbench-valid-key" via the control plane
    And I generate a unique value from "J4 Provider" and store it as "displayName"
    And I generate a unique API context from "/j4" and store it as "providerContext"

  @AIW-PRV-020 @type:smoke
  Scenario: A provider deployed to the gateway reaches the deployed state
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments"
    Then the response status code should be 200
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deploymentId}" should have "status" equal to "DEPLOYED"

  @AIW-PRV-024
  Scenario: A chat completion through the deployed provider returns the upstream's answer
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "choices[0].message.content" should be "Hello! How can I assist you today?"

  @AIW-PRV-026
  Scenario: A streaming chat completion arrives as ordered chunks and ends normally
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}],"stream":true}
      """
    Then the response body should match pattern "(?s)role.:.assistant.*content.:.This .*content.:.streamed .*content.:.model[.].*finish_reason.:.stop.*data: \[DONE\]"

  @AIW-PRV-025 @type:negative
  Scenario: A wrong upstream credential returns the upstream's error without leaking the credential
    Given I generate a unique resource name from "j4-wrong-key" and store it as "wrongSecret"
    And I create a secret "${CTX:wrongSecret}" with value "sk-aiw-prv-025-wrong-key" via the control plane
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:wrongSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    # The gateway refuses the consumer key with its own 401 until the key reaches it, so wait
    # for the upstream's refusal specifically.
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 401 and the response body contains "invalid_api_key" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 401
    And the response body should not contain "sk-aiw-prv-025-wrong-key"
    And the response header "api-key" should not exist

  @AIW-PRV-015
  Scenario: A request that violates the provider's guardrail is blocked with the guardrail's error
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
      | globalPolicies     | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"enabled":true,"min":1,"max":5}}}] |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello there"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"messages":[{"role":"user","content":"one two three four five six seven eight"}]}
      """
    Then the response status code should be 422
    And the response body should contain "word"

  @AIW-PRV-016
  Scenario: A guardrail edited and then removed stops applying once the provider is redeployed
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
      | globalPolicies     | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"enabled":true,"min":1,"max":5}}}] |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 422 with body:
      """
      {"messages":[{"role":"user","content":"one two three four five six seven eight"}]}
      """

    # Edited: the limit is raised, so the eight-word request now passes.
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
      | globalPolicies     | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"enabled":true,"min":1,"max":10}}}] |
    Then the response status code should be 200
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "editedDeployment"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:editedDeployment}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"one two three four five six seven eight"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"messages":[{"role":"user","content":"one two three four five six seven eight nine ten eleven twelve"}]}
      """
    Then the response status code should be 422

    # Removed: no guardrail applies.
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    Then the response status code should be 200
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "removedDeployment"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:removedDeployment}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"one two three four five six seven eight nine ten eleven twelve"}]}
      """

  @AIW-PRV-009
  Scenario: A changed endpoint takes effect once the provider is redeployed
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "model" should be "gpt-4.1-2025-04-14"
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/mistral/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "model" should be "gpt-4.1-2025-04-14"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "redeployment"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:redeployment}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until the response body contains "mistral-small-latest" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

  @AIW-PRV-022
  Scenario: An undeployed provider no longer serves traffic
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response should be successful
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 404 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

  # Restoring a stopped deployment re-sends the settings it was deployed with, not the changed
  # ones: a product defect, tagged @known-issue.
  @AIW-PRV-023 @known-issue
  Scenario: Redeploying a stopped provider after its settings changed puts the new settings in effect
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response should be successful
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/mistral/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}/restore?gatewayId=${CTX:gatewayId}"
    Then the response should be successful
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "model" should be "mistral-small-latest"

  @AIW-PRV-018
  Scenario: Requests over the provider's limit get 429 and succeed again once the window resets
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                     |
      | template           | azure-openai                                                                           |
      | context            | ${CTX:providerContext}                                                                 |
      | upstreamUrl        | http://testbench:3008/openai-keyed/v1                                                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:validSecret}\" }}"} |
      | security           | {"enabled":true,"apiKey":{"enabled":true,"key":"X-API-Key","in":"header"}}           |
      | rateLimiting       | {"providerLevel":{"global":{"request":{"enabled":true,"count":2,"reset":{"duration":1,"unit":"minute"}}}}} |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    When I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:providerId}/api-keys" with body:
      """
      {"displayName": "J4 consumer key"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey" as "consumerKey"
    And I set header "X-API-Key" to "${CTX:consumerKey}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
