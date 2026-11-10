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

@model-failover-streaming
Feature: A failure after the response has started
  Once a 200 has reached the client it cannot be un-sent, so an attempt that fails after its
  headers cannot be retried - mid-stream retry is closed by design, not a gap. These
  scenarios pin both halves of what that means: the request is NOT failed over, and the
  attempt is still GRADED.

  The grading half is the one that was missing. A response filter sees an ordinary 200, so a
  truncating provider was recorded as succeeding: the false success reset the
  consecutive-failure counter and diluted the error-rate window, which immunised a provider
  truncating EVERYTHING against the suspension its real failures would have triggered. The
  outcome is published only as a response flag on the access log, so what these exercise is
  the whole grading transport - the filtered access-log sink, the verdict decoder, and the
  correlation of a cluster back to the target health is keyed by.

  The failure shape comes from the testbench stream-fault service, which names its mode in a
  request header. The gateway forwards that header upstream, so one provider URL serves
  every shape and a scenario drives the sequence it wants.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A failure after the response has started is not failed over
    # Two operations carrying byte-identical failover configuration, and that pairing is the
    # whole point. The first fails BEFORE any body, so it must fail over - it is the control
    # proving failover is live here. The second sends its headers at once and then streams,
    # so the route timeout cuts it mid-body. Without the control, "no second attempt" on the
    # streaming operation would be indistinguishable from failover never having engaged.
    Given I generate a unique resource name from "stream-safety" and store it as "apiName"
    And I generate a unique API version from "stream-safety" and store it as "apiVersion"
    And I generate a unique API context from "/stream-safety" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","upstreams":[{"url":"http://testbench:3027"}]},{"name":"alive","basePath":"/stream-fallback","upstreams":[{"url":"http://testbench:3000"}]}]        |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.resilience          | {"timeout":"2s"}                                                                                                                                                       |
      | spec.operations          | [{"method":"GET","path":"/before","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]},{"method":"GET","path":"/after","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # CONTROL - the refusal is in the headers, so nothing has reached the client yet and the
    # attempt is retriable. Failover works on this API.
    When I set header "x-stream-fault" to "fail"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/before" until status 200
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/stream-fallback"
    And the response body should not have been cut off

    # SUBJECT - same configuration, same API, the failure moved to after the headers. The
    # upstream streams for ten seconds against a two-second route timeout.
    When I set header "x-stream-fault" to "drip"
    And I set header "x-stream-fault-seconds" to "10"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/after" allowing a cut-off body
    # The headers were real and already delivered - this is not an error response.
    Then the response status code should be 200
    # One attempt only: the fallback was never reached.
    And the response header "x-envoy-attempt-count" should be "1"
    # And the client saw the consequence rather than a silent rescue.
    And the response body should have been cut off
    And the response body should not contain "/stream-fallback"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A provider that truncates every response is suspended like any other failure
    Given I generate a unique resource name from "trunc-provider" and store it as "truncName"
    And I generate a unique API version from "trunc-provider" and store it as "truncVersion"
    And I generate a unique API context from "/trunc-provider" and store it as "truncContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:truncName}                |
      | displayName        | ${CTX:truncName}                |
      | version            | ${CTX:truncVersion}             |
      | template           | openai                          |
      | spec.context       | ${CTX:truncContext}             |
      | spec.upstream.url  | http://testbench:3027/truncate  |
      | accessControl.mode | allow_all                       |
    Then the response should be successful

    Given I generate a unique resource name from "trunc-alive" and store it as "aliveName"
    And I generate a unique API version from "trunc-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/trunc-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:aliveName}                           |
      | displayName        | ${CTX:aliveName}                           |
      | version            | ${CTX:aliveVersion}                        |
      | template           | openai                                     |
      | spec.context       | ${CTX:aliveContext}                        |
      | spec.upstream.url  | http://testbench:3000/trunc-alive-upstream |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    # One truncation suspends, for longer than the scenario runs, so a pass cannot come from
    # the window expiring rather than from the grading working.
    Given I generate a unique resource name from "trunc-proxy" and store it as "proxyName"
    And I generate a unique API version from "trunc-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/trunc-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:truncName}                                                                                                                                                       |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # Reaching the truncating target is polled on the cut-off itself rather than on a status,
    # because the status is 200 either way. Every poll before the route is live gets a 404 or
    # a 503, which is a whole body, so the poll ends exactly on the first request that
    # actually reached the target - the one that suspends it.
    When I set header "Content-Type" to "application/json"
    And I set header "x-stream-fault" to "truncate"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until the response body is cut off with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # The client wears it: a real 200 whose body stops early, one attempt, no rescue.
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should have been cut off

    # THE assertion. Both requests take ONE attempt, so the attempt count cannot tell them
    # apart - the body is the discriminator. A body from the alive upstream means the
    # truncation was graded as a failure and the leading target withdrawn. A second cut-off
    # body would mean it was recorded as a success and the target is still leading.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/trunc-alive-upstream"
    And the response body should not have been cut off

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:truncName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A complete streamed response of the same shape suspends nothing
    # The control for the scenario above, and the reason it means anything. The grading
    # signal is a response flag, and a streamed response is exactly the shape most likely to
    # be flagged by accident - chunked, no content length, ending when the upstream closes.
    # If an ordinary complete stream also suspended its target, the previous scenario would
    # pass for a reason that has nothing to do with truncation, and every streaming provider
    # on the gateway would be suspended after a single successful call.
    Given I generate a unique resource name from "whole-provider" and store it as "wholeName"
    And I generate a unique API version from "whole-provider" and store it as "wholeVersion"
    And I generate a unique API context from "/whole-provider" and store it as "wholeContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}    |
      | name               | ${CTX:wholeName}             |
      | displayName        | ${CTX:wholeName}             |
      | version            | ${CTX:wholeVersion}          |
      | template           | openai                       |
      | spec.context       | ${CTX:wholeContext}          |
      | spec.upstream.url  | http://testbench:3027/whole  |
      | accessControl.mode | allow_all                    |
    Then the response should be successful

    Given I generate a unique resource name from "whole-alive" and store it as "aliveName"
    And I generate a unique API version from "whole-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/whole-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:aliveName}                           |
      | displayName        | ${CTX:aliveName}                           |
      | version            | ${CTX:aliveVersion}                        |
      | template           | openai                                     |
      | spec.context       | ${CTX:aliveContext}                        |
      | spec.upstream.url  | http://testbench:3000/whole-alive-upstream |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    Given I generate a unique resource name from "whole-proxy" and store it as "proxyName"
    And I generate a unique API version from "whole-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/whole-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:wholeName}                                                                                                                                                       |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # Same streaming shape as the truncating provider, delivered whole.
    When I set header "Content-Type" to "application/json"
    And I set header "x-stream-fault" to "whole"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until the response body contains "/whole/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should not have been cut off

    # And the target is still leading the chain on the next request - it was not graded as a
    # failure for merely having streamed.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/whole/chat/completions"
    And the response body should not contain "/whole-alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:wholeName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A truncation and an outright failure count on the same target, not against each other
    # The immunity this removes was not only "a truncating provider is never suspended". It
    # was also that a truncation CANCELLED real failures: the false success reset the
    # consecutive-failure counter, so a target alternating between truncating and failing
    # outright never reached a threshold above one.
    #
    # The two failures arrive by different routes - the truncation through the access-log
    # verdict, the refusal through the ordinary response phase - and the point is that they
    # land on one counter. With the threshold at 2, neither alone suspends, so a pass means
    # both were counted.
    Given I generate a unique resource name from "mixed-provider" and store it as "mixedName"
    And I generate a unique API version from "mixed-provider" and store it as "mixedVersion"
    And I generate a unique API context from "/mixed-provider" and store it as "mixedContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}    |
      | name               | ${CTX:mixedName}             |
      | displayName        | ${CTX:mixedName}             |
      | version            | ${CTX:mixedVersion}          |
      | template           | openai                       |
      | spec.context       | ${CTX:mixedContext}          |
      | spec.upstream.url  | http://testbench:3027/mixed  |
      | accessControl.mode | allow_all                    |
    Then the response should be successful

    Given I generate a unique resource name from "mixed-alive" and store it as "aliveName"
    And I generate a unique API version from "mixed-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/mixed-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:aliveName}                           |
      | displayName        | ${CTX:aliveName}                           |
      | version            | ${CTX:aliveVersion}                        |
      | template           | openai                                     |
      | spec.context       | ${CTX:aliveContext}                        |
      | spec.upstream.url  | http://testbench:3000/mixed-alive-upstream |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    Given I generate a unique resource name from "mixed-proxy" and store it as "proxyName"
    And I generate a unique API version from "mixed-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mixed-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:mixedName}                                                                                                                                                       |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":2,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # Failure 1 of 2: a truncation. One attempt, no rescue, the client wears it. Polled on
    # the cut-off, so the first counted failure is the first request that reached the target.
    When I set header "Content-Type" to "application/json"
    And I set header "x-stream-fault" to "truncate"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until the response body is cut off with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should have been cut off

    # Failure 2 of 2: an outright refusal, before any body, so this one DOES fail over. The
    # target is still leading at this point - two attempts prove it was tried.
    When I set header "x-stream-fault" to "fail"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/mixed-alive-upstream"

    # Threshold reached across both causes: the target is out, so this request leads with
    # the fallback and costs one attempt instead of two.
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/mixed-alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:mixedName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
