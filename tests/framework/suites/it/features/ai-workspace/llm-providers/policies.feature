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

@AIW-LLM @llm-provider-policies
Feature: Guardrails and rate limits on an LLM provider through the control plane
  As an AI Workspace administrator
  I want a provider's guardrails and rate limits stored exactly as configured, and bad limits refused
  So that the policies I set are the policies the gateway receives

  # Sheet AIW-LLM, sub-features "Guardrails" (AIW-PRV-014) and "Rate limiting" (AIW-PRV-017,
  # AIW-PRV-019), at the API level. A guardrail is sent the way the AI Workspace sends it:
  # name, major version and parameters. Enforcement on traffic is covered with deployment.
  # The browser journey is suites/ui/features/aiw_llm/provider_policies.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "PRV Policies Provider" and store it as "displayName"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:displayName}              |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"

  @AIW-PRV-014
  Scenario: A guardrail attached with parameters is stored on the provider
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName    | ${CTX:displayName}                                                                                  |
      | template       | azure-openai                                                                                        |
      | upstreamUrl    | http://testbench:3008/openai/v1                                                                     |
      | globalPolicies | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"enabled":true,"min":1,"max":5}}}] |
    Then the response status code should be 200

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response array field "globalPolicies" should have 1 item
    And the JSON response field "globalPolicies[0].name" should be "word-count-guardrail"
    And the JSON response field "globalPolicies[0].version" should be "v1"
    And the JSON response field "globalPolicies[0].params.request.min" should be 1
    And the JSON response field "globalPolicies[0].params.request.max" should be 5

  @AIW-PRV-017
  Scenario: Request and token limits set on a provider are stored and read back
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName  | ${CTX:displayName}                                                                                                                                                                          |
      | template     | azure-openai                                                                                                                                                                                |
      | upstreamUrl  | http://testbench:3008/openai/v1                                                                                                                                                             |
      | rateLimiting | {"providerLevel":{"global":{"request":{"enabled":true,"count":100,"reset":{"duration":1,"unit":"hour"}},"token":{"enabled":true,"count":5000,"reset":{"duration":2,"unit":"day"}}}}} |
    Then the response status code should be 200

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response field "rateLimiting.providerLevel.global.request.count" should be 100
    And the JSON response field "rateLimiting.providerLevel.global.request.reset.duration" should be 1
    And the JSON response field "rateLimiting.providerLevel.global.request.reset.unit" should be "hour"
    And the JSON response field "rateLimiting.providerLevel.global.token.count" should be 5000
    And the JSON response field "rateLimiting.providerLevel.global.token.reset.duration" should be 2
    And the JSON response field "rateLimiting.providerLevel.global.token.reset.unit" should be "day"

  @AIW-PRV-019 @type:negative
  Scenario Outline: A provider request limit of <case> is rejected
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName  | ${CTX:displayName}                                                                                           |
      | template     | azure-openai                                                                                                 |
      | upstreamUrl  | http://testbench:3008/openai/v1                                                                              |
      | rateLimiting | {"providerLevel":{"global":{"request":{"enabled":true,"count":<count>,"reset":{"duration":1,"unit":"hour"}}}}} |
    Then the response status code should be 400

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response field "rateLimiting.providerLevel" should not exist

    Examples:
      | case                         | count               |
      | zero                         | 0                   |
      | negative                     | -5                  |
      | non-numeric                  | "abc"               |
      | decimal                      | 1.5                 |
      | beyond the platform maximum  | 9223372036854775808 |

  @AIW-PRV-019
  Scenario Outline: A provider request limit of <case> is stored exactly
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName  | ${CTX:displayName}                                                                                           |
      | template     | azure-openai                                                                                                 |
      | upstreamUrl  | http://testbench:3008/openai/v1                                                                              |
      | rateLimiting | {"providerLevel":{"global":{"request":{"enabled":true,"count":<count>,"reset":{"duration":1,"unit":"hour"}}}}} |
    Then the response status code should be 200

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the response body should match pattern "count.:<count>[,}]"

    Examples:
      | case                 | count               |
      | the minimum          | 1                   |
      | the platform maximum | 9223372036854775807 |
