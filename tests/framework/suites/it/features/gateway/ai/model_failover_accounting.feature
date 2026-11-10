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

@model-failover-accounting
Feature: A failed-over request is still ONE request
  However many attempts a rescue took, the client made one call: it owes one unit of
  whatever is being counted, and it produced one record of having happened. Counting per
  attempt would bill a customer three times for one answer, exhaust a budget on a single
  call, and triple every usage figure a publisher reads.

  Every chain here is three deep on purpose. A per-attempt miscount is only visibly wrong
  when the chain is longer than two, and three attempts against a budget of two is what
  makes the difference observable rather than inferred.

  The two dead targets are reachable hosts on ports nothing listens on, so each attempt is a
  refused connection - retriable, and arriving at the policy as an ordinary failure.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: Three attempts produce one analytics record
    Given I generate a unique resource name from "acct-events" and store it as "apiName"
    And I generate a unique API version from "acct-events" and store it as "apiVersion"
    And I generate a unique API context from "/acct-events" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"/acct-dead-one","upstreams":[{"url":"http://testbench:9998"}]},{"name":"mid","basePath":"/acct-dead-two","upstreams":[{"url":"http://testbench:9997"}]},{"name":"alive","basePath":"/acct-alive","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"mid"},{"model":"claude-opus-4-1-20250805","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # Warm up first, then reset, then measure. Analytics publish asynchronously, so a
    # collector reset before the route is live lets a warm-up request's own event land inside
    # the measured window and be counted as the measured request's.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    And I wait for an analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with response status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # Two dead targets passed over, the third served - three attempts, one response.
    And the response header "x-envoy-attempt-count" should be "3"
    And the response body should contain "/acct-alive"

    # ONE record for the whole chain. An EXACT count, because "at least one" would pass
    # while recording three, which is the miscount this guards.
    When I wait for an analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with response status 200
    And I wait for the analytics collector to settle
    Then the analytics collector should have received 1 event

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Three attempts consume one rate-limit unit
    # The limit is the sharp instrument. With a budget of 2 against a three-deep chain, a
    # request counted once per ATTEMPT would exhaust the whole budget on its own and be
    # refused mid-chain - so a 200 carrying three attempts is itself the proof, before the
    # later requests pin where the limit actually falls.
    #
    # The rate-limited operation is NOT the one warmed up: polling it would spend the very
    # budget under test, and a 503 on the way to readiness spends it just as surely as a 200.
    # The sibling operation shares the API, the deployment and the xDS push, so a 200 from it
    # means the limited route is live too, with its counter untouched.
    Given I generate a unique resource name from "acct-rl" and store it as "apiName"
    And I generate a unique API version from "acct-rl" and store it as "apiVersion"
    And I generate a unique API context from "/acct-rl" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"/rl-dead-one","upstreams":[{"url":"http://testbench:9998"}]},{"name":"mid","basePath":"/rl-dead-two","upstreams":[{"url":"http://testbench:9997"}]},{"name":"alive","basePath":"/rl-alive","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/warmup","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"mid"},{"model":"claude-opus-4-1-20250805","provider":"alive"}]}]}}]},{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"mid"},{"model":"claude-opus-4-1-20250805","provider":"alive"}]}]}},{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":2,"duration":"1h"}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/warmup" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    # Unit one of two.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "3"
    And the response body should contain "/rl-alive"

    # Unit two of two. Counted per attempt, the budget would already be gone.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "3"

    # And the limit is real rather than absent: the next request is refused.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A rescued request is billed for the attempt that actually served it
    # The fallback is the OpenAI mock, which answers with a fixed usage block. Fixed numbers
    # are what make this assertable at all - an echo backend returns no usage, so there would
    # be nothing to attribute.
    #
    # The client asks for one model and the attempt that serves reports another: the fallback
    # target names a different model, and the mock echoes what it was actually sent. Billing
    # has to follow the response, not the ask.
    Given I generate a unique resource name from "bill-dead" and store it as "deadName"
    And I generate a unique API version from "bill-dead" and store it as "deadVersion"
    And I generate a unique API context from "/bill-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:deadName}                      |
      | displayName        | ${CTX:deadName}                      |
      | version            | ${CTX:deadVersion}                   |
      | template           | openai                               |
      | spec.context       | ${CTX:deadContext}                   |
      | spec.upstream.url  | http://bill-nonexistent:9080/v1      |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    Given I generate a unique resource name from "bill-alive" and store it as "aliveName"
    And I generate a unique API version from "bill-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/bill-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:aliveName}           |
      | displayName        | ${CTX:aliveName}           |
      | version            | ${CTX:aliveVersion}        |
      | template           | openai                     |
      | spec.context       | ${CTX:aliveContext}        |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                  |
    Then the response should be successful

    Given I generate a unique resource name from "bill-proxy" and store it as "proxyName"
    And I generate a unique API version from "bill-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/bill-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"gpt-4o-mini","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # The serving attempt's own numbers, exactly - not zero, and not doubled.
    When I wait for an analytics event for path "${CTX:proxyContext}/chat/completions" with response status 200
    Then the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "usage.prompt_tokens" with value "19"
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "usage.completion_tokens" with value "10"
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "usage.total_tokens" with value "29"
    # Attribution follows the attempt that answered: the model the client asked the dead
    # primary for is not the model recorded.
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "model" with value "gpt-4o-mini"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: Three attempts consume one unit of subscription quota
    # The same reasoning as the rate-limit scenario, through the quota path instead. A plan
    # of two requests per minute against a three-deep chain is the sharp part: if a rescued
    # request spent one unit PER ATTEMPT, the very first would overrun the plan on its own.
    #
    # Readiness is awaited WITHOUT a subscription key, so the warm-up is refused rather than
    # served and spends no quota - the route being live is exactly what a refusal proves.
    Given I generate a unique value from "quota-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":2,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup

    Given I generate a unique resource name from "acct-quota" and store it as "apiName"
    And I generate a unique API version from "acct-quota" and store it as "apiVersion"
    And I generate a unique API context from "/acct-quota" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.subscriptionPlans   | ["${CTX:planName}"]                                                                                                                                                    |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"/quota-dead-one","upstreams":[{"url":"http://testbench:9998"}]},{"name":"mid","basePath":"/quota-dead-two","upstreams":[{"url":"http://testbench:9997"}]},{"name":"alive","basePath":"/quota-alive","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}},{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"mid"},{"model":"claude-opus-4-1-20250805","provider":"alive"}]}]}}]}] |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 403 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    Given I generate a unique value from "quota-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}","subscriptionPlanId":"${CTX:planId}"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "subId"
    And I register the "subscription" "${CTX:subId}" for cleanup

    # Unit one of two.
    When I set header "Content-Type" to "application/json"
    And I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "3"
    And the response body should contain "/quota-alive"

    # Unit two of two. Counted per attempt, the plan would already be spent.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "3"

    # And the plan is real: the next request is refused.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
