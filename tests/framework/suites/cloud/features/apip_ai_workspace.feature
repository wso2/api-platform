# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@apip-cloud
Feature: APIP cloud AI workspace
  As an APIP cloud user
  I want to create LLM providers and proxies, deploy them to the organisation's AI gateway and call them
  So that AI traffic reaches its upstream with the credential I stored and the policies I attached

  # The provider's upstream is the public echo backend, so the key and the request the gateway sends can be read back.
  # An upstream credential is copied into the header verbatim, so its scheme ("Bearer ") is part of the stored value.

  @TC_APIP_DEP_013 @ID_APIP_AI_WORKSPACE_001
  Scenario: A credential given when deploying a provider is sent upstream with the scheme the template defines
    Given I obtain an APIP cloud console token
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret holding a sample key with the prefix "Bearer "
    When I create a synthetic LLM provider with:
      | upstreamAuth | {"type":"api-key","header":"Authorization"} |
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment}         |
      | gatewayId   | ${CTX:cloudAIGatewayID}                  |
      | parameters  | {"apiKey":"${CTX:cloudSecretReference}"} |
    Then the cloud API response status code should be 201
    And the synthetic LLM provider deployment should become DEPLOYED
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProviderContext}/chat/completions" saying "hello from the cloud integration test" until status 200
    And the echoed request header "Authorization" should carry the synthetic secret's value

  @TC_APIP_DEP_014 @ID_APIP_AI_WORKSPACE_002
  Scenario: An organisation-level provider deploys to an environment directly, with no pipeline
    Given I obtain an APIP cloud console token
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
    Then the cloud API response status code should be 201
    And the deployment response should list 1 gateways
    And the synthetic LLM provider deployment should become DEPLOYED

  @TC_APIP_DEP_016 @ID_APIP_AI_WORKSPACE_003
  Scenario: The provider deploy and undeploy paths behave as the API paths do
    Given I obtain an APIP cloud console token
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the cloud API response status code should be 201
    And the synthetic LLM provider deployment should become DEPLOYED
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProviderContext}/chat/completions" saying "hello" until status 200
    When I undeploy the synthetic LLM provider
    Then the synthetic LLM provider deployment should become UNDEPLOYED
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProviderContext}/chat/completions" saying "hello" until status 404
    When I request a deployment of the synthetic LLM provider with:
      | environment | ${UNIQUE:missing-env} |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not an environment of this organization"
    When I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${UNIQUE:missing-gateway}        |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not in environment"

  @TC_APIP_AIW_001 @ID_APIP_AI_WORKSPACE_004
  Scenario: An LLM provider created in the organisation is listed
    Given I obtain an APIP cloud console token
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I list the LLM providers
    Then the cloud API response status code should be 200
    And the LLM provider list should include the synthetic LLM provider
    When I read the synthetic LLM provider
    Then the cloud API response should report:
      | template                | openai                            |
      | context                 | ${CTX:cloudLLMProviderContext}    |
      | upstream.main.url       | ${CTX:cloudEchoBackendURL}        |

  @TC_APIP_AIW_002 @ID_APIP_AI_WORKSPACE_005
  Scenario: A provider key can be created, replaced and revoked, and only the current key is sent
    Given I obtain an APIP cloud console token
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret
    When I create a synthetic LLM provider with:
      | upstreamAuth | {"type":"api-key","header":"x-apip-llm-key"} |
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment}         |
      | gatewayId   | ${CTX:cloudAIGatewayID}                  |
      | parameters  | {"apiKey":"${CTX:cloudSecretReference}"} |
    Then the cloud API response status code should be 201
    And the synthetic LLM provider deployment should become DEPLOYED
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProviderContext}/chat/completions" saying "first key" until status 200
    And the echoed request header "x-apip-llm-key" should carry the synthetic secret's value
    When I read the synthetic LLM provider
    Then the response should not reveal the synthetic secret's value
    When I create a replacement synthetic secret
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment}         |
      | gatewayId   | ${CTX:cloudAIGatewayID}                  |
      | parameters  | {"apiKey":"${CTX:cloudSecretReference}"} |
    Then the cloud API response status code should be 201
    And the synthetic LLM provider deployment should become DEPLOYED
    And a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProviderContext}/chat/completions" saying "second key" should echo the header "x-apip-llm-key" with the replacement secret value
    When I undeploy the synthetic LLM provider
    Then the synthetic LLM provider deployment should become UNDEPLOYED
    When I delete the synthetic secret
    Then the cloud API response status code should be 204

  @TC_APIP_AIW_003 @ID_APIP_AI_WORKSPACE_006
  Scenario: An LLM proxy deployed to the AI gateway becomes ready
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the cloud API response status code should be 201
    And the synthetic LLM provider deployment should become DEPLOYED
    When I create a synthetic LLM proxy over the synthetic LLM provider in the default project
    And I request a deployment of the synthetic LLM proxy with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the cloud API response status code should be 201
    And the synthetic LLM proxy deployment should become DEPLOYED

  @TC_APIP_AIW_004 @ID_APIP_AI_WORKSPACE_007
  Scenario: A deployed LLM proxy answers through the AI gateway
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the synthetic LLM provider deployment should become DEPLOYED
    When I create a synthetic LLM proxy over the synthetic LLM provider in the default project
    And I request a deployment of the synthetic LLM proxy with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the synthetic LLM proxy deployment should become DEPLOYED
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProxyContext}/chat/completions" saying "hello from the cloud integration test" until status 200
    And the echo backend should have received a request for "/chat/completions"

  @TC_APIP_AIW_005 @ID_APIP_AI_WORKSPACE_008
  Scenario: A guardrail attached to a deployed proxy blocks a request that violates it
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the synthetic LLM provider deployment should become DEPLOYED
    When I create a synthetic LLM proxy over the synthetic LLM provider in the default project with:
      | globalPolicies | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":"$.messages[0].content"}}}] |
    And I request a deployment of the synthetic LLM proxy with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the synthetic LLM proxy deployment should become DEPLOYED
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProxyContext}/chat/completions" saying "a short question" until status 200
    And I send a chat completion request to "${CTX:cloudAIGatewayEndpoint}${CTX:cloudLLMProxyContext}/chat/completions" saying "this message is deliberately far longer than the ten words that the guardrail on the proxy allows a request to carry" until status 422
    And the cloud API response body should contain "GUARDRAIL_INTERVENED"

  @TC_APIP_AIW_009 @ID_APIP_AI_WORKSPACE_009
  Scenario: AI and regular gateways each refuse the other's artifacts
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an AI gateway and a regular gateway of one APIP environment
    And an echo backend is available for deployment overrides
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "cannot run a RestApi"
    When I create a synthetic LLM provider
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudRegularGatewayID}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "cannot run a LlmProvider"

  # Known issue: nothing checks a stored provider key against the template's scheme (B13), so the deployment is accepted.
  @TC_APIP_AIW_010 @known-issue @ID_APIP_AI_WORKSPACE_010
  Scenario: A provider key stored without its required prefix is refused before it reaches the gateway
    Given I obtain an APIP cloud console token
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret
    When I create a synthetic LLM provider with:
      | upstreamAuth | {"type":"api-key","header":"Authorization"} |
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment}         |
      | gatewayId   | ${CTX:cloudAIGatewayID}                  |
      | parameters  | {"apiKey":"${CTX:cloudSecretReference}"} |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "prefix"

  # Known issue: the proxy deploy path checks no credential (B34), so the deployment is accepted.
  @TC_APIP_AIW_012 @known-issue @ID_APIP_AI_WORKSPACE_011
  Scenario: A proxy deployed without a provider key is refused when it is deployed
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP AI gateway
    And an echo backend is available for deployment overrides
    And I create a synthetic secret holding a sample key with the prefix "Bearer "
    When I create a synthetic LLM provider with:
      | upstreamAuth | {"type":"api-key","header":"Authorization"} |
    And I request a deployment of the synthetic LLM provider with:
      | environment | ${CTX:cloudAIGatewayEnvironment}         |
      | gatewayId   | ${CTX:cloudAIGatewayID}                  |
      | parameters  | {"apiKey":"${CTX:cloudSecretReference}"} |
    Then the synthetic LLM provider deployment should become DEPLOYED
    When I create a synthetic LLM proxy over the synthetic LLM provider in the default project with:
      | providerAuth | {"type":"api-key","header":"Authorization"} |
    And I request a deployment of the synthetic LLM proxy with:
      | environment | ${CTX:cloudAIGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudAIGatewayID}          |
    Then the cloud API response status code should be 400

  # Weak: with one organisation the suite cannot show that another organisation's items are absent.
  @TC_APIP_AIW_013 @ID_APIP_AI_WORKSPACE_012
  Scenario: The workspace lists the organisation's providers and proxies
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And an echo backend is available for deployment overrides
    When I create a synthetic LLM provider
    And I create a synthetic LLM proxy over the synthetic LLM provider in the default project
    And I list the LLM providers
    Then the LLM provider list should include the synthetic LLM provider
    When I list the LLM proxies
    Then the LLM proxy list should include the synthetic LLM proxy
    And every listed LLM provider and proxy should be readable by its handle

  @TC_APIP_AIW_014 @ID_APIP_AI_WORKSPACE_013
  Scenario: The AI gateway reports isActive only when its controller is connected
    Given I obtain an APIP cloud console token
    When I list the managed APIP gateways of type "ai"
    Then the cloud API response status code should be 200
    And the listed managed gateways should each report an identity, an environment and a status
    And every listed managed gateway should be of type "ai"
