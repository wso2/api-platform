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

@model-failover-suspension
Feature: Taking a failing target out of rotation, and putting it back
  Failing over rescues ONE request. Suspension is what stops every request after it paying
  the same failed attempt first: without it, a provider down for an hour costs a wasted
  attempt on every request for that hour, and the client is charged that latency each time.

  The attempt count is what makes suspension observable. A suspended target that was
  recorded but ignored still produces a 200, so the status code cannot tell the two apart;
  only the count can. It also shows that a skip costs no attempt - were skips to consume the
  budget, a chain of mostly suspended targets would exhaust itself stepping over dead entries.

  How long a suspension LASTS is a different observable again, and one no attempt count can
  reach. The scenarios below read it off the suspension record, which carries the duration
  applied and where the target sits on the backoff ladder. That is deliberate: waiting out a
  doubling window would make the scenario's runtime the thing under test, and a cap or a
  reset would need a run longer than any suite should hold. Reading the record tests the same
  decision at the moment it is taken.

  Every log assertion names the provider as well as the fact. The runtime log is shared by
  every runner on this gateway, so a suspension line carrying only a duration could have been
  written by any of them.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A suspended target stops being attempted on later requests
    Given I generate a unique resource name from "susp-dead" and store it as "deadName"
    And I generate a unique API version from "susp-dead" and store it as "deadVersion"
    And I generate a unique API context from "/susp-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:deadName}                           |
      | displayName        | ${CTX:deadName}                           |
      | version            | ${CTX:deadVersion}                        |
      | template           | openai                                    |
      | spec.context       | ${CTX:deadContext}                        |
      | spec.upstream.url  | http://susp-nonexistent:9080/dead-upstream|
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    Given I generate a unique resource name from "susp-alive" and store it as "aliveName"
    And I generate a unique API version from "susp-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/susp-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:aliveName}                          |
      | displayName        | ${CTX:aliveName}                          |
      | version            | ${CTX:aliveVersion}                       |
      | template           | openai                                    |
      | spec.context       | ${CTX:aliveContext}                       |
      | spec.upstream.url  | http://testbench:3000/susp-alive-upstream |
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    # One failure suspends, for a window far longer than the scenario, so it cannot expire
    # mid-run and make a pass look like a pass for the wrong reason.
    Given I generate a unique resource name from "susp-proxy" and store it as "proxyName"
    And I generate a unique API version from "susp-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/susp-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # Poll on the attempt count, not on 200, so the request this stops at is necessarily the
    # one that paid the failed attempt and was rescued - the failure that suspends the
    # leading target. A 200 poll could stop on a request that was already being skipped.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/susp-alive-upstream"

    # The next request does NOT pay it. The suspended target is dropped where the chain is
    # resolved, so the request leads with the healthy one and takes ONE attempt.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/susp-alive-upstream"

    # And it stays out: the one after is served the same way, not re-probed.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A suspension expires and the target is admitted as a probe
    # Suspension has to be temporary. A target taken out permanently on one bad minute never
    # comes back, and a publisher who configured three providers ends up running on one with
    # no indication why.
    #
    # The probe here FAILS - the target is still dead - and that is deliberate. What is
    # pinned is the re-admission, observable on its own as the attempt count going back up.
    # A scenario that also needed the probe to succeed would have to mutate a provider
    # mid-run, and would be testing two things at once.
    Given I generate a unique resource name from "expire-dead" and store it as "deadName"
    And I generate a unique API version from "expire-dead" and store it as "deadVersion"
    And I generate a unique API context from "/expire-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:deadName}                             |
      | displayName        | ${CTX:deadName}                             |
      | version            | ${CTX:deadVersion}                          |
      | template           | openai                                      |
      | spec.context       | ${CTX:deadContext}                          |
      | spec.upstream.url  | http://expire-nonexistent:9080/dead-upstream|
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    Given I generate a unique resource name from "expire-alive" and store it as "aliveName"
    And I generate a unique API version from "expire-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/expire-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:aliveName}                            |
      | displayName        | ${CTX:aliveName}                            |
      | version            | ${CTX:aliveVersion}                         |
      | template           | openai                                      |
      | spec.context       | ${CTX:aliveContext}                         |
      | spec.upstream.url  | http://testbench:3000/expire-alive-upstream |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    # A short window, so it can expire inside one scenario. maxSuspendMultiplier is 1 so the
    # re-suspension the failed probe causes lasts the same 5s rather than doubling - without
    # it the scenario's timing would depend on the escalation ladder.
    Given I generate a unique resource name from "expire-proxy" and store it as "proxyName"
    And I generate a unique API version from "expire-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/expire-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":5,"suspendAfterFailures":1,"maxSuspendMultiplier":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # The failure that suspends it.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Immediately after: suspended, skipped, one attempt is enough. This is sent as a single
    # request rather than a poll because the window is only 5s - a poll would sit inside it
    # and could observe either side of the expiry.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # Past the window. Polling for the count to return to 2 is how the expiry is awaited:
    # the condition IS the re-admission, so the wait ends exactly when the thing under test
    # happens rather than after a guessed interval. Every poll before that is a skipped
    # target answering in one attempt, and a skip is not a failure, so polling cannot itself
    # extend the suspension it is waiting out.
    #
    # Two attempts means the target re-entered the chain and was tried. Had the suspension
    # been permanent this would stay at 1 forever, with the client none the wiser.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/expire-alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A target that fails again is suspended for double the previous duration
    # The ladder's first two rungs, read off the record rather than waited out. The base
    # window is short so the target is back in rotation quickly and can be failed a second
    # time inside one scenario; what is asserted is the duration the policy APPLIED, which
    # is decided the moment it suspends and does not depend on how long the scenario runs.
    Given I generate a unique resource name from "ladder-dead" and store it as "deadName"
    And I generate a unique API version from "ladder-dead" and store it as "deadVersion"
    And I generate a unique API context from "/ladder-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:deadName}                             |
      | displayName        | ${CTX:deadName}                             |
      | version            | ${CTX:deadVersion}                          |
      | template           | openai                                      |
      | spec.context       | ${CTX:deadContext}                          |
      | spec.upstream.url  | http://ladder-nonexistent:9080/dead-upstream|
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    Given I generate a unique resource name from "ladder-alive" and store it as "aliveName"
    And I generate a unique API version from "ladder-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/ladder-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:aliveName}                            |
      | displayName        | ${CTX:aliveName}                            |
      | version            | ${CTX:aliveVersion}                         |
      | template           | openai                                      |
      | spec.context       | ${CTX:aliveContext}                         |
      | spec.upstream.url  | http://testbench:3000/ladder-alive-upstream |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    # BOTH providers are aliased, which the scenarios above do not need. A target's `provider`
    # names an additionalProviders alias; the proxy's main provider has none, and a chain
    # member naming nothing does not resolve on an LlmProxy - it is dropped, leaving a
    # one-member chain that exhausts instead of failing over. Here the leading target must be
    # named so the suspension record identifies itself in a log every runner shares, so the
    # dead provider is listed as an additional provider as well as being the proxy's main. backoffResetSeconds is held far above the scenario's own
    # runtime: a reset landing mid-run would put the second suspension back at the base and
    # the doubling would silently stop being what is measured.
    Given I generate a unique resource name from "ladder-proxy" and store it as "proxyName"
    And I generate a unique API version from "ladder-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/ladder-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:deadName}","as":"ladder-dead"},{"id":"${CTX:aliveName}","as":"ladder-alive"}]                                                                                                                        |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":3,"suspendAfterFailures":1,"maxSuspendMultiplier":8,"backoffResetSeconds":600,"targets":[{"model":"gpt-4o","provider":"ladder-dead","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"ladder-alive"}]}]}}]}] |
    Then the response should be successful

    # Rung one: the base duration, the first step on the ladder.
    #
    # The assertion spans the record's middle fields rather than naming only the two that
    # matter, because the provider that scopes it and the duration that is the claim sit at
    # opposite ends of the line. Splitting them would leave the duration unscoped, and a
    # duration alone could have been logged by any runner on this gateway.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the "gateway-runtime" log since the latest request should contain "provider=ladder-dead reason=consecutive_failures observed=1 sample_size=0 duration_seconds=3 escalation_step=1"

    # Rung two. Polling for a second two-attempt response waits out the base window and then
    # re-admits the target, which fails again - every poll in between is a skipped target
    # answering in one attempt, and a skip is not a failure, so polling cannot itself move
    # the ladder.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # DOUBLE the previous duration, and the next rung. A ladder that did not escalate would
    # report 3 again here; one that escalated per attempt rather than per suspension would
    # overshoot it.
    And the "gateway-runtime" log since the latest request should contain "provider=ladder-dead reason=consecutive_failures observed=1 sample_size=0 duration_seconds=6 escalation_step=2"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A target already at the maximum duration does not escalate further
    # The cap, with maxSuspendMultiplier at 2 so the ceiling is reached on the second
    # suspension and the third has somewhere to NOT go. Without a cap a flapping provider
    # ends up held out for hours on a fault that cleared in minutes, which is indistinguishable
    # from the gateway having forgotten about it.
    Given I generate a unique resource name from "cap-dead" and store it as "deadName"
    And I generate a unique API version from "cap-dead" and store it as "deadVersion"
    And I generate a unique API context from "/cap-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                |
      | name               | ${CTX:deadName}                          |
      | displayName        | ${CTX:deadName}                          |
      | version            | ${CTX:deadVersion}                       |
      | template           | openai                                   |
      | spec.context       | ${CTX:deadContext}                       |
      | spec.upstream.url  | http://cap-nonexistent:9080/dead-upstream|
      | accessControl.mode | allow_all                                |
    Then the response should be successful

    Given I generate a unique resource name from "cap-alive" and store it as "aliveName"
    And I generate a unique API version from "cap-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/cap-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                |
      | name               | ${CTX:aliveName}                         |
      | displayName        | ${CTX:aliveName}                         |
      | version            | ${CTX:aliveVersion}                      |
      | template           | openai                                   |
      | spec.context       | ${CTX:aliveContext}                      |
      | spec.upstream.url  | http://testbench:3000/cap-alive-upstream |
      | accessControl.mode | allow_all                                |
    Then the response should be successful

    Given I generate a unique resource name from "cap-proxy" and store it as "proxyName"
    And I generate a unique API version from "cap-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/cap-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:deadName}","as":"cap-dead"},{"id":"${CTX:aliveName}","as":"cap-alive"}]                                                                                                                           |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":2,"suspendAfterFailures":1,"maxSuspendMultiplier":2,"backoffResetSeconds":600,"targets":[{"model":"gpt-4o","provider":"cap-dead","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"cap-alive"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the "gateway-runtime" log since the latest request should contain "provider=cap-dead reason=consecutive_failures observed=1 sample_size=0 duration_seconds=2 escalation_step=1"

    # Doubling to 4 reaches the ceiling: the base is 2 and the multiplier is 2.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the "gateway-runtime" log since the latest request should contain "provider=cap-dead reason=consecutive_failures observed=1 sample_size=0 duration_seconds=4 escalation_step=2"

    # THE assertion. The step keeps counting - the target really is being re-suspended - but
    # the duration stays at the ceiling instead of doubling to 8.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the "gateway-runtime" log since the latest request should contain "provider=cap-dead reason=consecutive_failures observed=1 sample_size=0 duration_seconds=4 escalation_step=3"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: Failures with a success in between never reach the threshold
    # A target that fails intermittently is not a target that is down, and suspending it on a
    # tally rather than a run would withdraw a provider that is serving most of its traffic.
    # The threshold is two CONSECUTIVE failures, so a success between two failures has to put
    # the count back to nothing.
    #
    # The sequence is driven by a header rather than by a stored script: the stream-fault
    # service picks its mode per request and keeps no state, so each request's outcome is
    # chosen here and the readiness poll above cannot consume or reorder it.
    Given I generate a unique resource name from "reset-flaky" and store it as "flakyName"
    And I generate a unique API version from "reset-flaky" and store it as "flakyVersion"
    And I generate a unique API context from "/reset-flaky" and store it as "flakyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}      |
      | name               | ${CTX:flakyName}               |
      | displayName        | ${CTX:flakyName}               |
      | version            | ${CTX:flakyVersion}            |
      | template           | openai                         |
      | spec.context       | ${CTX:flakyContext}            |
      | spec.upstream.url  | http://testbench:3027/flaky    |
      | accessControl.mode | allow_all                      |
    Then the response should be successful

    Given I generate a unique resource name from "reset-alive" and store it as "aliveName"
    And I generate a unique API version from "reset-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/reset-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:aliveName}                           |
      | displayName        | ${CTX:aliveName}                           |
      | version            | ${CTX:aliveVersion}                        |
      | template           | openai                                     |
      | spec.context       | ${CTX:aliveContext}                        |
      | spec.upstream.url  | http://testbench:3000/reset-alive-upstream |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    # A threshold of two is the smallest that can tell a run from a tally: at one, every
    # failure suspends and there is nothing for a success to interrupt.
    Given I generate a unique resource name from "reset-proxy" and store it as "proxyName"
    And I generate a unique API version from "reset-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/reset-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:flakyName}                                                                                                                                                       |
      | spec.additionalProviders | [{"id":"${CTX:flakyName}","as":"reset-flaky"},{"id":"${CTX:aliveName}","as":"reset-alive"}]                                                                            |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":2,"statusCodes":[503],"targets":[{"model":"gpt-4o","provider":"reset-flaky","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"reset-alive"}]}]}}]}] |
    Then the response should be successful

    # Readiness on a healthy request, so the wait records no failure of its own however many
    # polls it takes.
    #
    # "plain" rather than "whole", and that is the difference between this scenario working
    # and silently measuring nothing. WHEN a success is counted depends on its shape: a
    # streamed 200 cannot be judged until the stream ends, so the gateway defers it to the
    # access log and the verdict arrives asynchronously - after the next request here has
    # already been graded. The counter would still be standing at one, the next failure would
    # make two, and the target would be suspended by the very run this scenario says cannot
    # form. A plain 200 is final when it arrives.
    When I set header "Content-Type" to "application/json"
    And I set header "x-stream-fault" to "plain"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    # Failure one. Two attempts prove the leading target was tried and failed.
    When I set header "x-stream-fault" to "fail"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # The success that breaks the run. One attempt: the leading target served it itself,
    # which is what makes it a success recorded against that target rather than a rescue.
    When I set header "x-stream-fault" to "plain"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # Failure two, and THE assertion. Counted as a tally this is the second failure and the
    # target would be suspended here; counted as a run it is the first since the success, and
    # nothing happens. The target was still led with, so it really was attempted.
    When I set header "x-stream-fault" to "fail"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the "gateway-runtime" log since the latest request should not contain "provider=reset-flaky reason=consecutive_failures"

    # CONTROL, and the reason the absence above means anything. One more failure, now
    # genuinely consecutive, and the target IS suspended - so the threshold is real and the
    # scenario is not passing because suspension never fires.
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the "gateway-runtime" log since the latest request should contain "provider=reset-flaky reason=consecutive_failures observed=2 sample_size=0 duration_seconds=300 escalation_step=1"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:flakyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A target that stays healthy long enough starts its next suspension from the base
    # The last rung of the ladder, and the one that stops it being a one-way ratchet. A
    # provider that had a bad minute an hour ago should not be punished for an hour; without
    # the reset, every later fault starts from wherever the last one left off and a
    # long-lived gateway ends up holding a healthy provider out for its maximum.
    #
    # This is the only suspension scenario that cannot read its answer off a record and must
    # let real time pass: the reset is earned by STAYING healthy, and nothing short of
    # elapsed healthy traffic produces that. The healthy period is spanned by drip requests,
    # which hold a real second each - the policy measures wall-clock health, so the scenario
    # pays it in the same currency rather than sleeping.
    Given I generate a unique resource name from "rebase-flaky" and store it as "flakyName"
    And I generate a unique API version from "rebase-flaky" and store it as "flakyVersion"
    And I generate a unique API context from "/rebase-flaky" and store it as "flakyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}      |
      | name               | ${CTX:flakyName}               |
      | displayName        | ${CTX:flakyName}               |
      | version            | ${CTX:flakyVersion}            |
      | template           | openai                         |
      | spec.context       | ${CTX:flakyContext}            |
      | spec.upstream.url  | http://testbench:3027/rebase   |
      | accessControl.mode | allow_all                      |
    Then the response should be successful

    Given I generate a unique resource name from "rebase-alive" and store it as "aliveName"
    And I generate a unique API version from "rebase-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/rebase-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:aliveName}                            |
      | displayName        | ${CTX:aliveName}                            |
      | version            | ${CTX:aliveVersion}                         |
      | template           | openai                                      |
      | spec.context       | ${CTX:aliveContext}                         |
      | spec.upstream.url  | http://testbench:3000/rebase-alive-upstream |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    # recoverAfterSuccesses is 1 so a single probe restores the target and starts its healthy
    # clock; the default of three would make the scenario pay for two more round trips before
    # the clock it is waiting on even begins.
    Given I generate a unique resource name from "rebase-proxy" and store it as "proxyName"
    And I generate a unique API version from "rebase-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/rebase-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:flakyName}                                                                                                                                                       |
      | spec.additionalProviders | [{"id":"${CTX:flakyName}","as":"rebase-flaky"},{"id":"${CTX:aliveName}","as":"rebase-alive"}]                                                                          |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":2,"suspendAfterFailures":1,"maxSuspendMultiplier":8,"backoffResetSeconds":2,"recoverAfterSuccesses":1,"statusCodes":[503],"targets":[{"model":"gpt-4o","provider":"rebase-flaky","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"rebase-alive"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I set header "x-stream-fault" to "plain"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    # Suspension one, at the base.
    When I set header "x-stream-fault" to "fail"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the "gateway-runtime" log since the latest request should contain "provider=rebase-flaky reason=consecutive_failures observed=1 sample_size=0 duration_seconds=2 escalation_step=1"

    # Wait the suspension out and restore the target, in one step. While it is suspended the
    # fallback answers and the body names the echo; once the window lapses the target is
    # probed, the probe succeeds, and the body names it instead. Polling on WHICH upstream
    # served is what makes this a wait for recovery rather than a wait for a status that was
    # already being returned.
    When I set header "x-stream-fault" to "plain"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until the response body contains "/rebase/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response header "x-envoy-attempt-count" should be "1"

    # The healthy period, paid in real seconds. Three drip requests of a second each clear
    # the two-second reset with margin; each is a complete stream the target serves itself,
    # so the clock keeps running rather than being restarted by a failure.
    When I set header "x-stream-fault" to "drip"
    And I set header "x-stream-fault-seconds" to "1"
    And I send 3 "POST" requests to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    # The reset is earned by staying healthy but OBSERVED when a success is recorded, so it
    # takes a success the gateway can record on the spot to collect it. The drips above spent
    # the time; a streamed success is judged from the access log and its record would not
    # arrive until after the failure below had already been graded, leaving the ladder exactly
    # where it was. One plain success closes that gap.
    When I set header "x-stream-fault" to "plain"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response header "x-envoy-attempt-count" should be "1"

    # THE assertion. The target has been suspended before, so an un-reset ladder would put
    # this one at double the base on rung two. Having stayed healthy long enough, it starts
    # again from the bottom.
    When I set header "x-stream-fault" to "fail"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the "gateway-runtime" log since the latest request should contain "provider=rebase-flaky reason=consecutive_failures observed=1 sample_size=0 duration_seconds=2 escalation_step=1"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:flakyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
