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
  statuses and delays and then forwards cleanly. Each assertion names the provider as well as
  the reason: the runtime log is shared by every runner on this gateway, and a suspension line
  carrying only a reason could have been written by any of them. That is also why each leading
  target names its provider explicitly rather than defaulting to the route's main upstream -
  an unnamed target is logged with an empty provider, which identifies nothing. A scripted sequence rather than a weighted
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
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendAfterFailures":100,"suspendDuration":120,"errorRatePercent":20,"minimumRequests":10,"windowSeconds":120,"statusCodes":[503],"targets":[{"model":"gpt-4o","provider":"flaky","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
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
    Then the "gateway-runtime" log since the latest request should contain "provider=flaky reason=error_rate"
    And the "gateway-runtime" log since the latest request should not contain "provider=flaky reason=consecutive_failures"

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
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":120,"errorRatePercent":0,"p95LatencyMs":200,"minimumRequests":10,"windowSeconds":120,"targets":[{"model":"gpt-4o","provider":"slow","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
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
    Then the "gateway-runtime" log since the latest request should contain "provider=slow reason=latency_p95"
    And the "gateway-runtime" log since the latest request should not contain "provider=slow reason=consecutive_failures"
    And the "gateway-runtime" log since the latest request should not contain "provider=slow reason=error_rate"

    # Traffic has moved to the fast target.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/latency-alive"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: The failover log record carries neither the credential nor the prompt
    # The record names targets, reasons and counts. It must never name the API key a provider
    # was reached with, nor anything the end user typed - this log is read far more widely
    # than the request was, and is routinely shipped to a third-party aggregator.
    #
    # Absence assertions are the easiest kind to pass by accident: get the scope wrong and
    # every one of them holds for free. So the two below are anchored by a POSITIVE assertion
    # on the very same scope, read since the very same request. If the suspension record were
    # missing - wrong window, wrong log, nothing emitted - the anchor fails first and the
    # absences are never reached on a scope that cannot see anything.
    Given I generate a unique value from "redact-credential" and store it as "vendorCredential"
    And I generate a unique value from "redact-prompt" and store it as "promptSecret"

    Given I generate a unique resource name from "redact-dead" and store it as "deadName"
    And I generate a unique API version from "redact-dead" and store it as "deadVersion"
    And I generate a unique API context from "/redact-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:deadName}                            |
      | displayName        | ${CTX:deadName}                            |
      | version            | ${CTX:deadVersion}                         |
      | template           | openai                                     |
      | spec.context       | ${CTX:deadContext}                         |
      | spec.upstream.url  | http://redact-nonexistent-host:9080/gone   |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    Given I generate a unique resource name from "redact-alive" and store it as "aliveName"
    And I generate a unique API version from "redact-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/redact-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}              |
      | name               | ${CTX:aliveName}                       |
      | displayName        | ${CTX:aliveName}                       |
      | version            | ${CTX:aliveVersion}                    |
      | template           | openai                                 |
      | spec.context       | ${CTX:aliveContext}                    |
      | spec.upstream.url  | http://testbench:3000/redact-upstream  |
      | accessControl.mode | allow_all                              |
    Then the response should be successful

    # The fallback carries a real credential, so the value is genuinely in play on the attempt
    # that serves - not merely declared somewhere the policy never touches.
    Given I generate a unique resource name from "redact-proxy" and store it as "proxyName"
    And I generate a unique API version from "redact-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/redact-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                   |
      | name                   | ${CTX:proxyName}                                                                                                            |
      | displayName            | ${CTX:proxyName}                                                                                                            |
      | version                | ${CTX:proxyVersion}                                                                                                         |
      | context                | ${CTX:proxyContext}                                                                                                         |
      | provider.id            | ${CTX:deadName}                                                                                                             |
      | spec.providers         | [{"id":"${CTX:deadName}","alias":"redact-dead","isPrimary":true},{"id":"${CTX:aliveName}","alias":"redact-alive","auth":{"type":"api-key","header":"x-vendor-key","value":"${CTX:vendorCredential}"}}] |
      | spec.operationPolicies | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o","provider":"redact-dead","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"redact-alive"}]}]}}]}] |
    Then the response should be successful

    # One request that fails over, carrying the user's text. The fallback answers, so the
    # credential was used and the prompt was forwarded - both had every chance to be logged.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "${CTX:promptSecret}"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/redact-upstream"

    # ANCHOR. A suspension record exists for this request, in this window. Everything below is
    # read from the same scope, so none of it can be vacuous.
    And the "gateway-runtime" log since the latest request should contain "provider=redact-dead reason=consecutive_failures observed=1 sample_size=0 duration_seconds=300 escalation_step=1"

    # THE CLAIM. The credential the fallback was reached with is not in the record, and
    # neither is anything the user typed.
    And the "gateway-runtime" log since the latest request should not contain "${CTX:vendorCredential}"
    And the "gateway-runtime" log since the latest request should not contain "${CTX:promptSecret}"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
