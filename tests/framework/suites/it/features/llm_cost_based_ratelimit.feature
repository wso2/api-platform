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

@llm-cost-based-ratelimit
Feature: LLM cost-based rate limiting
  As an API developer
  I want to rate limit LLM APIs based on monetary budgets
  So that I can control costs by setting spending limits

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce cost-based rate limit on LLM API
    # gpt-4.1-2025-04-14: 19 prompt x $2/1M + 10 completion x $8/1M = $0.000118 per request
    # Budget $0.000236 = exactly 2 requests worth
    Given I generate a unique resource name from "cblr-enforce-template" and store it as "templateName"
    And I generate a unique value from "cblr-enforce-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblr-enforce-provider" and store it as "providerName"
    And I generate a unique value from "cblr-enforce-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblr-enforce" and store it as "providerVersion"
    And I generate a unique API context from "/cblr-enforce" and store it as "providerContext"
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
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000236,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # An unrecognized model returns cost=0 and is never billed (see "Zero cost requests do not
    # consume budget"), so it doubles as a free readiness canary: retry until the route and its
    # policy chain are live without touching the budget the real requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/chat" until status 200 with body:
      """
      {"model":"my-unknown-model-xyz","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  @known-issue
  Scenario: Cost-based rate limit with multiple budget time windows
    # Budget: $0.000236/1m (minute) AND $0.001180/1h (hourly - 10x per-request cost)
    # 2 requests exhaust the minute window even though the hourly budget still has room
    Given I generate a unique resource name from "cblr-multiwin-template" and store it as "templateName"
    And I generate a unique value from "cblr-multiwin-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblr-multiwin-provider" and store it as "providerName"
    And I generate a unique value from "cblr-multiwin-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblr-multiwin" and store it as "providerVersion"
    And I generate a unique API context from "/cblr-multiwin" and store it as "providerContext"
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
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000236,"duration":"1m"},{"amount":0.001180,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # A policy with two simultaneous budget windows takes slightly longer to fully initialize
    # both quota trackers than a single-window policy; wait for the policy chain to sync before
    # folding the first request into the readiness check, so the minute window is guaranteed
    # enforced from the very first request rather than only from the second.
    And I wait for policy snapshot sync
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-ratelimit-cost-remaining-dollars" should be "0.000000"

    # The tighter per-minute window blocks this request even though the hourly budget still
    # has $0.000944 remaining
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Cost accumulates correctly across real LLM responses
    # claude-3-5-haiku-20241022: 50 input x $0.80/1M + 25 output x $4.00/1M = $0.000140 per request
    # Budget $0.000280 = exactly 2 requests worth
    Given I generate a unique resource name from "cblr-anthropic-template" and store it as "templateName"
    And I generate a unique value from "cblr-anthropic-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblr-anthropic-provider" and store it as "providerName"
    And I generate a unique value from "cblr-anthropic-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblr-anthropic" and store it as "providerVersion"
    And I generate a unique API context from "/cblr-anthropic" and store it as "providerContext"
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
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000280,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    # An unrecognized model returns cost=0 and is never billed (see "Zero cost requests do not
    # consume budget"), so it doubles as a free readiness canary: retry until the route and its
    # policy chain are live without touching the budget the real requests below rely on.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/chat" until status 200 with body:
      """
      {"model":"my-unknown-model-xyz","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Cost-based rate limit returns proper rate limit headers
    Given I generate a unique resource name from "cblr-headers-template" and store it as "templateName"
    And I generate a unique value from "cblr-headers-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblr-headers-provider" and store it as "providerName"
    And I generate a unique value from "cblr-headers-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblr-headers" and store it as "providerVersion"
    And I generate a unique API context from "/cblr-headers" and store it as "providerContext"
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
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.001,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Per-provider cost rate limiting is isolated
    # Two separate providers each budgeted for exactly 2 requests. Exhausting provider A's
    # budget must not affect provider B's budget.
    Given I generate a unique resource name from "cblr-prov-a-template" and store it as "templateNameA"
    And I generate a unique value from "cblr-prov-a-template" and store it as "templateDisplayNameA"
    And I generate a unique resource name from "cblr-prov-a-provider" and store it as "providerNameA"
    And I generate a unique value from "cblr-prov-a-provider" and store it as "providerDisplayNameA"
    And I generate a unique API version from "cblr-prov-a" and store it as "providerVersionA"
    And I generate a unique API context from "/cblr-prov-a" and store it as "providerContextA"
    And I generate a unique resource name from "cblr-prov-b-template" and store it as "templateNameB"
    And I generate a unique value from "cblr-prov-b-template" and store it as "templateDisplayNameB"
    And I generate a unique resource name from "cblr-prov-b-provider" and store it as "providerNameB"
    And I generate a unique value from "cblr-prov-b-provider" and store it as "providerDisplayNameB"
    And I generate a unique API version from "cblr-prov-b" and store it as "providerVersionB"
    And I generate a unique API context from "/cblr-prov-b" and store it as "providerContextB"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateNameA}             |
      | displayName | ${CTX:templateDisplayNameA}      |
    Then the response status code should be 201

    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateNameB}             |
      | displayName | ${CTX:templateDisplayNameB}      |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerNameA}              |
      | displayName        | ${CTX:providerDisplayNameA}       |
      | version            | ${CTX:providerVersionA}           |
      | template           | ${CTX:templateNameA}              |
      | spec.context       | ${CTX:providerContextA}           |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000236,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerNameB}              |
      | displayName        | ${CTX:providerDisplayNameB}       |
      | version            | ${CTX:providerVersionB}           |
      | template           | ${CTX:templateNameB}              |
      | spec.context       | ${CTX:providerContextB}           |
      | spec.upstream.url  | http://testbench:3008             |
      | accessControl.mode | allow_all                          |
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000236,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContextA}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    And I send a "POST" request to "${CTX:providerContextB}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContextA}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContextA}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429

    When I send a "POST" request to "${CTX:providerContextB}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerNameA}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerNameB}"
    Then the response should be successful

  Scenario: Zero cost requests do not consume budget
    # An unrecognized model returns cost=0 (not_calculated) and must not consume budget, leaving
    # it intact for the known-model requests that follow.
    Given I generate a unique resource name from "cblr-zero-template" and store it as "templateName"
    And I generate a unique value from "cblr-zero-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblr-zero-provider" and store it as "providerName"
    And I generate a unique value from "cblr-zero-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblr-zero" and store it as "providerVersion"
    And I generate a unique API context from "/cblr-zero" and store it as "providerContext"
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
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000236,"duration":"1h"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/chat" until status 200 with body:
      """
      {"model":"my-unknown-model-xyz","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/chat" with body:
      """
      {"model":"my-unknown-model-xyz","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # Known model - the full budget is still available since the unknown-model requests above
    # never consumed any of it
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  @known-issue
  Scenario: Rate limit window resets after time window expires
    Given I generate a unique resource name from "cblr-reset-template" and store it as "templateName"
    And I generate a unique value from "cblr-reset-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "cblr-reset-provider" and store it as "providerName"
    And I generate a unique value from "cblr-reset-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "cblr-reset" and store it as "providerVersion"
    And I generate a unique API context from "/cblr-reset" and store it as "providerContext"
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
      | spec.policies      | [{"name":"llm-cost-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"budgetLimits":[{"amount":0.000236,"duration":"10s"}]}}]},{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429

    # Instead of a blind sleep past the 10s window, retry the same real request until the
    # window has actually reset and it is accepted - this can never over-wait and never
    # under-wait relative to the policy engine's own clock.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
