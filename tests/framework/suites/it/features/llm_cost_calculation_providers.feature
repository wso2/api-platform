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

@llm-cost-calculation-providers
Feature: LLM cost calculation across provider response shapes
  As an API developer
  I want cost-based rate limiting to charge the correct amount for each provider's own usage
  and pricing shape
  So that per-provider quirks (multipliers, cache tiers, service tiers, reasoning tokens,
  per-call fees, and URL-embedded model IDs) never under- or over-charge a budget

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Gemini cost calculation triggers rate limit at correct token count
    # gemini-1.5-flash-002: 100 prompt x 7.5e-8 + 100 completion x 3e-7 = $0.0000375 per request
    # Budget $0.000075 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-gemini-template" and store it as "templateName"
    And I generate a unique value from "cblp-gemini-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-gemini-provider" and store it as "providerName"
    And I generate a unique value from "cblp-gemini-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-gemini" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-gemini" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000075,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A request with no "model" field returns cost=0 and is never billed (see "No model field in
    # response - zero cost never triggers rate limit"), so it doubles as a free readiness canary:
    # retry until the route and its policy chain are live without touching the budget the real
    # requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/models/gemini-1.5-flash-002:generateContent" until status 429 with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Anthropic geo and speed multipliers inflate cost correctly
    # claude-opus-4-6: baseCost = 20 input x 5e-6 + 10 output x 2.5e-5 = 3.5e-4
    # multiplier = 1.1 (us) x 6.0 (fast) = 6.6 -> finalCost = $0.00231 per request
    # Budget $0.004620 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-anthropic-geo-speed-template" and store it as "templateName"
    And I generate a unique value from "cblp-anthropic-geo-speed-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-anthropic-geo-speed-provider" and store it as "providerName"
    And I generate a unique value from "cblp-anthropic-geo-speed-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-anthropic-geo-speed" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-anthropic-geo-speed" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.004620,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-geo-speed" until status 200 with body:
      """
      {"model":"claude-opus-4-6","messages":[{"role":"user","content":"Hello"}],"max_tokens":100,"speed":"fast"}
      """

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-geo-speed" with body:
      """
      {"model":"claude-opus-4-6","messages":[{"role":"user","content":"Hello"}],"max_tokens":100,"speed":"fast"}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-geo-speed" until status 429 with body:
      """
      {"model":"claude-opus-4-6","messages":[{"role":"user","content":"Hello"}],"max_tokens":100,"speed":"fast"}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Anthropic 1-hour TTL cache writes billed at higher rate
    # claude-opus-4-6: 10 input x 5e-6 + 5 output x 2.5e-5 + 100 5m-write x 6.25e-6 + 500 1hr-write x 1e-5
    #                 = 0.00005 + 0.000125 + 0.000625 + 0.005 = $0.0058 per request
    # Budget $0.011600 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-anthropic-cache1hr-template" and store it as "templateName"
    And I generate a unique value from "cblp-anthropic-cache1hr-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-anthropic-cache1hr-provider" and store it as "providerName"
    And I generate a unique value from "cblp-anthropic-cache1hr-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-anthropic-cache1hr" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-anthropic-cache1hr" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.011600,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-cache-1hr" until status 200 with body:
      """
      {"model":"claude-opus-4-6","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-cache-1hr" with body:
      """
      {"model":"claude-opus-4-6","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-cache-1hr" until status 429 with body:
      """
      {"model":"claude-opus-4-6","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Anthropic web search tool per-query cost added to token cost
    # claude-3-5-haiku-20241022: 50 input x 8e-7 + 25 output x 4e-6 + 2 web-search queries x 0.01
    #                          = 0.00004 + 0.00010 + 0.02 = $0.02014 per request
    # Budget $0.040280 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-anthropic-websearch-template" and store it as "templateName"
    And I generate a unique value from "cblp-anthropic-websearch-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-anthropic-websearch-provider" and store it as "providerName"
    And I generate a unique value from "cblp-anthropic-websearch-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-anthropic-websearch" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-anthropic-websearch" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.040280,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-web-search" until status 200 with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Search the web"}],"max_tokens":100}
      """

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-web-search" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Search the web"}],"max_tokens":100}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-web-search" until status 429 with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Search the web"}],"max_tokens":100}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Gemini context caching - cached tokens billed at reduced rate
    # gemini-2.0-flash: (500-200) prompt x 1e-7 + 200 cached x 2.5e-8 + 100 completion x 4e-7
    #                  = 3e-5 + 5e-6 + 4e-5 = $0.000075 per request
    # Budget $0.000150 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-gemini-cached-template" and store it as "templateName"
    And I generate a unique value from "cblp-gemini-cached-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-gemini-cached-provider" and store it as "providerName"
    And I generate a unique value from "cblp-gemini-cached-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-gemini-cached" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-gemini-cached" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000150,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A request with no "model" field returns cost=0 and is never billed (see "No model field in
    # response - zero cost never triggers rate limit"), so it doubles as a free readiness canary:
    # retry until the route and its policy chain are live without touching the budget the real
    # requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/cached/gemini-2.0-flash:generateContent" with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/cached/gemini-2.0-flash:generateContent" with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/cached/gemini-2.0-flash:generateContent" until status 429 with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Gemini thinking model - thinking tokens billed at reasoning rate
    # gemini-2.5-flash-preview-04-17: 100 prompt x 1.5e-7 + (80-30) output x 6e-7 + 30 thoughts x 3.5e-6
    #                                = 1.5e-5 + 3e-5 + 1.05e-4 = $0.00015 per request
    # Budget $0.000300 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-gemini-thinking-template" and store it as "templateName"
    And I generate a unique value from "cblp-gemini-thinking-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-gemini-thinking-provider" and store it as "providerName"
    And I generate a unique value from "cblp-gemini-thinking-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-gemini-thinking" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-gemini-thinking" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000300,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A request with no "model" field returns cost=0 and is never billed (see "No model field in
    # response - zero cost never triggers rate limit"), so it doubles as a free readiness canary:
    # retry until the route and its policy chain are live without touching the budget the real
    # requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" until status 429 with body:
      """
      {"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Anthropic cache reads billed at reduced cache read rate
    # claude-3-5-haiku-20241022: 50 input x 8e-7 + 200 cache-read x 8e-8 + 25 output x 4e-6
    #                          = 4e-5 + 1.6e-5 + 1e-4 = $0.000156 per request
    # Budget $0.000312 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-anthropic-cache-read-template" and store it as "templateName"
    And I generate a unique value from "cblp-anthropic-cache-read-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-anthropic-cache-read-provider" and store it as "providerName"
    And I generate a unique value from "cblp-anthropic-cache-read-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-anthropic-cache-read" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-anthropic-cache-read" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000312,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A request with no "model" field returns cost=0 and is never billed (see "No model field in
    # response - zero cost never triggers rate limit"), so it doubles as a free readiness canary:
    # retry until the route and its policy chain are live without touching the budget the real
    # requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-cache-read" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-cache-read" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-cache-read" until status 429 with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: OpenAI prompt caching - cached tokens billed at reduced rate
    # gpt-4.1-2025-04-14: (200-100) prompt x 2e-6 + 100 cached x 5e-7 + 50 completion x 8e-6
    #                    = 2e-4 + 5e-5 + 4e-4 = $0.00065 per request
    # Budget $0.001300 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-openai-cached-template" and store it as "templateName"
    And I generate a unique value from "cblp-openai-cached-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-openai-cached-provider" and store it as "providerName"
    And I generate a unique value from "cblp-openai-cached-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-openai-cached" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-openai-cached" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.001300,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: OpenAI flex service tier - lower rates applied for flex tier
    # gpt-5.4 flex: 100 prompt x 1.25e-6 + 50 completion x 7.5e-6 = 1.25e-4 + 3.75e-4 = $0.0005 per request
    # Budget $0.001000 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-openai-flex-template" and store it as "templateName"
    And I generate a unique value from "cblp-openai-flex-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-openai-flex-provider" and store it as "providerName"
    And I generate a unique value from "cblp-openai-flex-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-openai-flex" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-openai-flex" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.001000,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-flex" until status 200 with body:
      """
      {"model":"gpt-5.4","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-flex" with body:
      """
      {"model":"gpt-5.4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-flex" until status 429 with body:
      """
      {"model":"gpt-5.4","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: OpenAI priority service tier - higher rates applied for priority tier
    # gpt-4.1 priority: 100 prompt x 3.5e-6 + 50 completion x 1.4e-5 = 3.5e-4 + 7.0e-4 = $0.00105 per request
    # Budget $0.002100 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-openai-priority-template" and store it as "templateName"
    And I generate a unique value from "cblp-openai-priority-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-openai-priority-provider" and store it as "providerName"
    And I generate a unique value from "cblp-openai-priority-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-openai-priority" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-openai-priority" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.002100,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-priority" until status 200 with body:
      """
      {"model":"gpt-4.1","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-priority" with body:
      """
      {"model":"gpt-4.1","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-priority" until status 429 with body:
      """
      {"model":"gpt-4.1","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: OpenAI batch service tier - batch rates applied for batch tier
    # gpt-4.1 batch: 100 prompt x 1e-6 + 50 completion x 4e-6 = 1e-4 + 2e-4 = $0.0003 per request
    # Budget $0.000600 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-openai-batch-template" and store it as "templateName"
    And I generate a unique value from "cblp-openai-batch-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-openai-batch-provider" and store it as "providerName"
    And I generate a unique value from "cblp-openai-batch-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-openai-batch" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-openai-batch" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000600,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-batch" until status 200 with body:
      """
      {"model":"gpt-4.1","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-batch" with body:
      """
      {"model":"gpt-4.1","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-batch" until status 429 with body:
      """
      {"model":"gpt-4.1","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: OpenAI reasoning tokens billed at standard output rate
    # o4-mini-2025-04-16: 100 prompt x 1.1e-6 + 80 completion (includes 30 reasoning tokens billed
    # at the output rate) x 4.4e-6 = 1.1e-4 + 3.52e-4 = $0.000462 per request
    # Budget $0.000924 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-openai-reasoning-template" and store it as "templateName"
    And I generate a unique value from "cblp-openai-reasoning-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-openai-reasoning-provider" and store it as "providerName"
    And I generate a unique value from "cblp-openai-reasoning-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-openai-reasoning" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-openai-reasoning" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000924,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-reasoning" until status 200 with body:
      """
      {"model":"o4-mini-2025-04-16","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-reasoning" with body:
      """
      {"model":"o4-mini-2025-04-16","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-reasoning" until status 429 with body:
      """
      {"model":"o4-mini-2025-04-16","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: OpenAI web search tool - url_citation annotation adds flat per-call fee
    # gpt-4.1-2025-04-14: 50 prompt x 2e-6 + 25 completion x 8e-6 + 1 web-search call x 0.01
    #                    = 1e-4 + 2e-4 + 0.01 = $0.0103 per request
    # Budget $0.020600 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-openai-web-search-template" and store it as "templateName"
    And I generate a unique value from "cblp-openai-web-search-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-openai-web-search-provider" and store it as "providerName"
    And I generate a unique value from "cblp-openai-web-search-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-openai-web-search" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-openai-web-search" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.020600,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-web-search" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-web-search" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-web-search" until status 429 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Mistral - bare model name resolved and cost calculated correctly
    # mistral-small-latest: 100 prompt x 1e-7 + 50 completion x 3e-7 = 1e-5 + 1.5e-5 = $0.000025 per request
    # Budget $0.000050 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-mistral-template" and store it as "templateName"
    And I generate a unique value from "cblp-mistral-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-mistral-provider" and store it as "providerName"
    And I generate a unique value from "cblp-mistral-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-mistral" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-mistral" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000050,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A request with no "model" field returns cost=0 and is never billed (see "No model field in
    # response - zero cost never triggers rate limit"), so it doubles as a free readiness canary:
    # retry until the route and its policy chain are live without touching the budget the real
    # requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/mistral/v1/chat/completions" with body:
      """
      {"model":"mistral-small-latest","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/mistral/v1/chat/completions" with body:
      """
      {"model":"mistral-small-latest","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/mistral/v1/chat/completions" until status 429 with body:
      """
      {"model":"mistral-small-latest","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: No model field in response - zero cost never triggers rate limit
    # The upstream response contains no model field, so cost calculation returns 0. With a very
    # tight budget, all 3 requests still pass because zero cost never consumes it.
    Given I generate a unique resource name from "cblp-no-model-template" and store it as "templateName"
    And I generate a unique value from "cblp-no-model-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-no-model-provider" and store it as "providerName"
    And I generate a unique value from "cblp-no-model-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-no-model" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-no-model" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000001,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  @bedrock
  Scenario: Bedrock Converse model is extracted from URL and cost is calculated
    # apac.amazon.nova-micro-v1:0: 10 input x 3.7e-8 + 3 output x 1.48e-7 = $0.000000814 per request
    # Budget $0.000001628 = exactly 2 requests worth
    Given I generate a unique resource name from "cblp-bedrock-template" and store it as "templateName"
    And I generate a unique value from "cblp-bedrock-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblp-bedrock-provider" and store it as "providerName"
    And I generate a unique value from "cblp-bedrock-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblp-bedrock" and store it as "providerVersion"
    And I generate a unique API context from "/cblp-bedrock" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000001628,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A request with no "model" field returns cost=0 and is never billed (see "No model field in
    # response - zero cost never triggers rate limit"), so it doubles as a free readiness canary:
    # retry until the route and its policy chain are live without touching the budget the real
    # requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/model/apac.amazon.nova-micro-v1:0/converse" with body:
      """
      {"messages":[{"role":"user","content":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200
    And the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    When I send a "POST" request to "${CTX:providerContext}/model/apac.amazon.nova-micro-v1:0/converse" with body:
      """
      {"messages":[{"role":"user","content":[{"text":"Hello"}]}]}
      """
    Then the response status code should be 200

    # The cost charge for the prior request commits asynchronously after its response, so a
    # single-shot request here can observe a not-yet-exhausted budget; poll until the charge
    # has settled instead of asserting on the first response.
    When I send a "POST" request to "${CTX:providerContext}/model/apac.amazon.nova-micro-v1:0/converse" until status 429 with body:
      """
      {"messages":[{"role":"user","content":[{"text":"Hello"}]}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
