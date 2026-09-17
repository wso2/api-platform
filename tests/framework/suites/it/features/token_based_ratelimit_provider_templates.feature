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

@token-based-ratelimit-provider-templates
Feature: Token-based rate limiting with built-in provider templates
  As an API developer
  I want token-based rate limits to charge real usage for built-in provider templates
  So that a provider-wide or wildcard-attached quota is not silently undercharged

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A provider-wide quota charges actual usage and is shared across every resource
    Given I generate a unique resource name from "tbrl-tmpl-provider-wide" and store it as "providerName"
    And I generate a unique value from "tbrl-tmpl-provider-wide" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-tmpl-provider-wide" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-tmpl-provider-wide" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | anthropic                          |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3008             |
      | accessControl.mode     | allow_all                          |
      | spec.globalPolicies    | [{"name":"token-based-ratelimit","version":"v1","params":{"totalTokenLimits":[{"count":75,"duration":"1h"}]}}] |
    Then the response status code should be 201
    # The quota is global to the provider, so every path consumes it — there is no operation left
    # unattached to serve as a free readiness canary. The first real request doubles as the
    # readiness check: retry until the route (and its policy chain) is actually live, then assert
    # on that same, now-published response.
    And I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages" until status 200 with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hi"}],"max_tokens":100}
      """
    Then the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hi"}],"max_tokens":100}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages-web-search" with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hi"}],"max_tokens":100}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: A quota is not double-counted when the built-in template already defines a total-token field
    Given I generate a unique resource name from "tbrl-tmpl-no-double-count" and store it as "providerName"
    And I generate a unique value from "tbrl-tmpl-no-double-count" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-tmpl-no-double-count" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-tmpl-no-double-count" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | mistralai                          |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3008             |
      | accessControl.mode     | allow_all                          |
      | spec.globalPolicies    | [{"name":"token-based-ratelimit","version":"v1","params":{"totalTokenLimits":[{"count":1000,"duration":"1h"}]}}] |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:providerContext}/mistral/v1/chat/completions" until status 200 with body:
      """
      {"model":"mistral-small-latest","messages":[{"role":"user","content":"Hi"}]}
      """
    # The "mistralai" template defines promptTokens, completionTokens, AND totalTokens. Charging
    # must use totalTokens alone (100+50=150) — never the sum of all three fields (100+50+150=300)
    # — so remaining is 1000-150=850, not 1000-300=700.
    Then the response header "X-RateLimit-Remaining" should be "850"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: A wildcard-attached quota charges actual usage rather than a flat cost of one
    Given I generate a unique resource name from "tbrl-tmpl-wildcard" and store it as "providerName"
    And I generate a unique value from "tbrl-tmpl-wildcard" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-tmpl-wildcard" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-tmpl-wildcard" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | anthropic                          |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3008             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"totalTokenLimits":[{"count":1000,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory","consumerBased":true}}]}] |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:providerContext}/anthropic/v1/messages" until status 200 with body:
      """
      {"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hi"}],"max_tokens":100}
      """
    Then the response header "X-RateLimit-Remaining" should be "925"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
