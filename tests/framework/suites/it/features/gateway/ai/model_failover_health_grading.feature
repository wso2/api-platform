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

@model-failover-health-grading
Feature: The two degradations a consecutive-failure rule never sees
  A target can be badly degraded without ever failing twice in a row, and it can stop being
  usable without failing at all. Neither is visible to a rule that counts consecutive
  failures, and each needs its own criterion - which is also why each scenario asserts WHICH
  criterion fired, not merely that something did. An operator triaging a suspension has to
  be able to tell a hard outage from a slow decline.

  Both degraded targets are the testbench fault injector, which answers a stored sequence of
  statuses and delays and then forwards cleanly. A scripted sequence rather than a weighted
  random one: the old fixture drew at random, so the shape of a run was never the same twice
  and a scenario could only assert what it got on average.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A target failing roughly one request in three is suspended on its error rate
    # Badly degraded, but rarely failing twice running, so a consecutive-failure run never
    # accumulates. suspendAfterFailures is pushed to 100 so that rule CANNOT fire at all,
    # leaving the error-rate rule as the only thing that can suspend this target - which is
    # what makes the reason assertion meaningful rather than circumstantial.
    Given I generate a unique resource name from "rate-script" and store it as "scriptId"
    When I send a "PUT" request to the "fault-injection" service at "/test/scripts/${CTX:scriptId}" with body:
      """
      {"forwardPort": 3000, "steps": [
        {"status": 503}, {"delayMs": 1}, {"delayMs": 1},
        {"status": 503}, {"delayMs": 1}, {"delayMs": 1},
        {"status": 503}, {"delayMs": 1}, {"delayMs": 1},
        {"status": 503}, {"delayMs": 1}, {"delayMs": 1},
        {"status": 503}, {"delayMs": 1}, {"delayMs": 1},
        {"status": 503}
      ]}
      """
    Then the response should be successful
    And I resolve the "fault-injection" service path at "/${CTX:scriptId}" and store it as "flakyPath"

    Given I generate a unique resource name from "rate" and store it as "apiName"
    And I generate a unique API version from "rate" and store it as "apiVersion"
    And I generate a unique API context from "/rate" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"flaky","basePath":"${CTX:flakyPath}","upstreams":[{"url":"http://testbench:3026"}]},{"name":"alive","basePath":"/rate-alive","upstreams":[{"url":"http://testbench:3000"}]}]                |
      | spec.upstream.main.ref   | flaky                                                                                                                                                                  |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendAfterFailures":100,"suspendDuration":120,"errorRatePercent":20,"minimumRequests":10,"windowSeconds":120,"statusCodes":[503],"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    And I send 16 "POST" requests to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # Suspended, and the record names WHICH criterion did it.
    Then the "gateway-runtime" log since the latest request should contain "reason=error_rate"
    And the "gateway-runtime" log since the latest request should not contain "reason=consecutive_failures"

    # The effect, not just the record: traffic has moved off the flaky target, so the
    # fallback's own base path now appears in the response.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/rate-alive"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A target that is slow but never fails is suspended on its latency percentile
    # The case a timeout-only rule cannot see: every request completes, none errors, none
    # comes close to a timeout - the target has simply got slow. Before this rule existed
    # the only signal was a client complaint.
    #
    # Note what is NOT configured: no perTryTimeout. Nothing is being cut off. The target is
    # suspended on the aggregate alone.
    Given I generate a unique resource name from "latency-script" and store it as "scriptId"
    When I send a "PUT" request to the "fault-injection" service at "/test/scripts/${CTX:scriptId}" with body:
      """
      {"forwardPort": 3000, "steps": [
        {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300},
        {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300},
        {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300},
        {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300}, {"delayMs": 300}
      ]}
      """
    Then the response should be successful
    And I resolve the "fault-injection" service path at "/${CTX:scriptId}" and store it as "slowPath"

    Given I generate a unique resource name from "latency" and store it as "apiName"
    And I generate a unique API version from "latency" and store it as "apiVersion"
    And I generate a unique API context from "/latency" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"slow","basePath":"${CTX:slowPath}","upstreams":[{"url":"http://testbench:3026"}]},{"name":"alive","basePath":"/latency-alive","upstreams":[{"url":"http://testbench:3000"}]}]               |
      | spec.upstream.main.ref   | slow                                                                                                                                                                   |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":120,"errorRatePercent":0,"p95LatencyMs":200,"minimumRequests":10,"windowSeconds":120,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    And I send 15 "POST" requests to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # Suspended on the percentile, with no failure anywhere in the run. This is the
    # assertion that would not have been reachable before: the rule was only ever evaluated
    # when something failed, and nothing here does.
    Then the "gateway-runtime" log since the latest request should contain "reason=latency_p95"
    And the "gateway-runtime" log since the latest request should not contain "reason=consecutive_failures"
    And the "gateway-runtime" log since the latest request should not contain "reason=error_rate"

    # Traffic has moved to the fast target.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/latency-alive"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
