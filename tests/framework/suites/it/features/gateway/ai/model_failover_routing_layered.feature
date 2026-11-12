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

@model-failover-layered
Feature: Failover layered over a routing policy
  A routing policy decides WHICH target leads; failover decides what happens when that target
  fails. The routing policy publishes an ordered ranking once per client request, and failover
  traverses it.

  A rotation advances its cursor once per CLIENT request, so every request a scenario makes -
  including the ones it makes only to wait for the deployment to become reachable - moves the
  rotation on. That shapes how these
  scenarios are written. Polling is used once per scenario, to reach a known state, and after
  that each measured request is sent individually; and because the number of polls is not
  knowable in advance, nothing downstream of a poll may assert an absolute rotation position.
  What is asserted instead is the RELATIONSHIP between successive responses, which holds from
  whatever position the polling left the cursor in.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Failover continues through the routing policy's published order
    # Two providers, one dead and one live, so the rotation visits a failing target on a known
    # turn and a healthy one on the others.
    Given I generate a unique resource name from "layered-dead" and store it as "deadName"
    And I generate a unique API version from "layered-dead" and store it as "deadVersion"
    And I generate a unique API context from "/layered-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                     |
      | name               | ${CTX:deadName}                               |
      | displayName        | ${CTX:deadName}                               |
      | version            | ${CTX:deadVersion}                            |
      | template           | openai                                        |
      | spec.context       | ${CTX:deadContext}                            |
      | spec.upstream.url  | http://layered-nonexistent:9080/dead-upstream |
      | accessControl.mode | allow_all                                     |
    Then the response should be successful

    Given I generate a unique resource name from "layered-alive" and store it as "aliveName"
    And I generate a unique API version from "layered-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/layered-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:aliveName}                     |
      | displayName        | ${CTX:aliveName}                     |
      | version            | ${CTX:aliveVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:aliveContext}                  |
      | spec.upstream.url  | http://testbench:3000/alive-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    # The rotation alternates between a target on the DEAD primary and one on the live
    # provider. model-failover carries no fallbacks of its own, so the only depth available is
    # the ranking the routing policy published - which is exactly what this measures.
    #
    # suspendDuration is 0 so the dead target is never withdrawn. With suspension live the
    # alternation would decay into "always the live one" after the first failure, and the
    # turn-by-turn counts below would stop describing the rotation.
    Given I generate a unique resource name from "layered-order" and store it as "proxyName"
    And I generate a unique API version from "layered-order" and store it as "proxyVersion"
    And I generate a unique API context from "/layered-order" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.providers | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:aliveName}","alias":"alive-provider"}]                                                                                                                      |
      | spec.operationPolicies            | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"gpt-4o"},{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}}]},{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o"}]}}]}] |
    Then the response should be successful

    # The three turns below assert the ALTERNATION, not absolute attempt counts, and that is a
    # correction rather than a weakening. The claim this scenario exists to make is that the
    # routing policy advances its cursor once per CLIENT request and not once per attempt -
    # which alternation proves on its own, whatever position the cursor happened to be in.
    #
    # Absolute counts cannot be asserted here. The route and the policy chain do not become
    # reachable at the same instant, so an early request can reach the rotation while failover
    # is not yet attached, get a 503, and still advance the cursor. Pinning "turn one takes two
    # attempts" therefore asserts a deploy-timing accident. Measured: it took one.
    #
    # With a two-entry rotation the counts can only be 1 or 2, so "the next differs" and "the
    # one after matches" together mean both values occurred - the failover did happen.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And I store the response header "x-envoy-attempt-count" as "turnOneAttempts"

    # The cursor advanced, so this turn leads with the other entry and its attempt count must
    # differ. A rotation that did not advance - or advanced per attempt rather than per request
    # - fails here.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should not be "${CTX:turnOneAttempts}"

    # And back again. Had the cursor advanced twice on the turn that failed over - once per
    # attempt rather than once per request - it would be out of phase by now and this turn
    # would not match turn one.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "${CTX:turnOneAttempts}"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A suspended target is skipped without costing an attempt
    # The routing policy never learns that failover suspended a target, and is not supposed to:
    # failover's health is private per-instance state, published to nobody. So the rotation goes
    # on nominating the dead target, and failover drops it when it resolves the chain. A drop at
    # resolution time costs no upstream attempt, which is what separates it from attempting the
    # target and failing over.
    Given I generate a unique resource name from "layered-susp-dead" and store it as "deadName"
    And I generate a unique API version from "layered-susp-dead" and store it as "deadVersion"
    And I generate a unique API context from "/layered-susp-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                     |
      | name               | ${CTX:deadName}                               |
      | displayName        | ${CTX:deadName}                               |
      | version            | ${CTX:deadVersion}                            |
      | template           | openai                                        |
      | spec.context       | ${CTX:deadContext}                            |
      | spec.upstream.url  | http://layered-nonexistent:9080/dead-upstream |
      | accessControl.mode | allow_all                                     |
    Then the response should be successful

    Given I generate a unique resource name from "layered-susp-alive" and store it as "aliveName"
    And I generate a unique API version from "layered-susp-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/layered-susp-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:aliveName}                     |
      | displayName        | ${CTX:aliveName}                     |
      | version            | ${CTX:aliveVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:aliveContext}                  |
      | spec.upstream.url  | http://testbench:3000/alive-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    # One failure suspends, for a window far longer than the scenario, so it cannot expire
    # mid-run and turn a genuine skip into a coincidence. The ROUTING policy's own
    # suspendDuration stays 0 - it must keep nominating the dead target, or the skip under test
    # never gets exercised and the scenario would pass on the wrong mechanism.
    Given I generate a unique resource name from "layered-suspend" and store it as "proxyName"
    And I generate a unique API version from "layered-suspend" and store it as "proxyVersion"
    And I generate a unique API context from "/layered-suspend" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.providers | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:aliveName}","alias":"alive-provider"}]                                                                                                                      |
      | spec.operationPolicies            | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"models":[{"model":"gpt-4o"},{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}}]},{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o"}]}}]}] |
    Then the response should be successful

    # Poll on the attempt count rather than on 200, because the event this scenario turns on is
    # not "the proxy is serving" but "the dead target has now been attempted and has failed" -
    # which is the failure that suspends it. A 200 poll could stop on a turn that led with the
    # live target, leaving nothing suspended and the assertions below measuring nothing.
    # Two attempts can only mean the dead target led, failed, and was failed over from.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # The dead target is now suspended. The rotation has a period of two, so these three
    # consecutive requests cover both of its positions at least once - including the turn that
    # nominates the suspended target. Every one of them must cost a single attempt.
    #
    # This is the assertion that separates the two possible designs. Were the suspension
    # ignored, the nominated turn would attempt the dead target and report two; were the skip
    # implemented as a failed attempt rather than a removal, it would also report two. Only a
    # removal at chain-resolution time reports one.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "1"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "1"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "1"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A non-adopting routing policy composes via its single selection
    # model-weighted-round-robin publishes only the single-selection key. It has not adopted
    # the ordered-preference contract and is not required to - that is the compatibility
    # guarantee the whole design rests on, that no existing routing policy has to change for
    # failover to work. Failover must read that lone selection as a one-element ranking and
    # continue into its OWN configured chain from there.
    Given I generate a unique resource name from "layered-na-dead" and store it as "deadName"
    And I generate a unique API version from "layered-na-dead" and store it as "deadVersion"
    And I generate a unique API context from "/layered-na-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                     |
      | name               | ${CTX:deadName}                               |
      | displayName        | ${CTX:deadName}                               |
      | version            | ${CTX:deadVersion}                            |
      | template           | openai                                        |
      | spec.context       | ${CTX:deadContext}                            |
      | spec.upstream.url  | http://layered-nonexistent:9080/dead-upstream |
      | accessControl.mode | allow_all                                     |
    Then the response should be successful

    Given I generate a unique resource name from "layered-na-alive" and store it as "aliveName"
    And I generate a unique API version from "layered-na-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/layered-na-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:aliveName}                     |
      | displayName        | ${CTX:aliveName}                     |
      | version            | ${CTX:aliveVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:aliveContext}                  |
      | spec.upstream.url  | http://testbench:3000/alive-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    # Every unit of weight sits on the dead target, so the single selection is gpt-4o on every
    # request and the routing policy contributes no second option at all. Any success therefore
    # has to have come from failover's own fallback list and from nowhere else.
    Given I generate a unique resource name from "layered-nonadopting" and store it as "proxyName"
    And I generate a unique API version from "layered-nonadopting" and store it as "proxyVersion"
    And I generate a unique API context from "/layered-nonadopting" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.providers | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:aliveName}","alias":"alive-provider"}]                                                                                                                      |
      | spec.operationPolicies            | [{"name":"model-weighted-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"models":[{"model":"gpt-4o","weight":1}]}}]},{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}]}}]}] |
    Then the response should be successful

    # There is no rotation here, so polling costs this scenario nothing and the attempt count
    # can be asserted outright. A 200 is only reachable through the fallback: the lead target
    # is dead on every request and suspension is off, so the leading attempt always fails.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "2"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A routing policy without failover keeps its own suspension
    # Teaching model-round-robin to publish an ordered ranking must change nothing for a
    # deployment that does not use failover. Its own suspendDuration still governs, exactly as
    # it did before this feature existed, and no part of failover is involved in reaching that
    # outcome - because failover is not attached to this operation at all.
    Given I generate a unique resource name from "layered-solo-dead" and store it as "deadName"
    And I generate a unique API version from "layered-solo-dead" and store it as "deadVersion"
    And I generate a unique API context from "/layered-solo-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                     |
      | name               | ${CTX:deadName}                               |
      | displayName        | ${CTX:deadName}                               |
      | version            | ${CTX:deadVersion}                            |
      | template           | openai                                        |
      | spec.context       | ${CTX:deadContext}                            |
      | spec.upstream.url  | http://layered-nonexistent:9080/dead-upstream |
      | accessControl.mode | allow_all                                     |
    Then the response should be successful

    Given I generate a unique resource name from "layered-solo-alive" and store it as "aliveName"
    And I generate a unique API version from "layered-solo-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/layered-solo-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:aliveName}                     |
      | displayName        | ${CTX:aliveName}                     |
      | version            | ${CTX:aliveVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:aliveContext}                  |
      | spec.upstream.url  | http://testbench:3000/alive-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    # No model-failover on this operation. One failure must withdraw the dead target under the
    # routing policy's OWN suspendDuration, held long enough that it cannot lapse mid-scenario.
    Given I generate a unique resource name from "layered-solo" and store it as "proxyName"
    And I generate a unique API version from "layered-solo" and store it as "proxyVersion"
    And I generate a unique API context from "/layered-solo" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.providers | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:aliveName}","alias":"alive-provider"}]                                                                                                                      |
      | spec.operationPolicies            | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"models":[{"model":"gpt-4o"},{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}}]}] |
    Then the response should be successful

    # Reaching the state under test needs the dead target to have been nominated once with the
    # route live, and the request that does so is not identifiable in advance: the first
    # success may land on either rotation position, and whichever polls preceded it are not
    # countable. So the run below is deliberately unasserted - it exists to guarantee that both
    # positions of a two-entry rotation have been visited by the time the assertions start.
    #
    # The 503 the dead turn produces is not asserted either, for the same reason it cannot be
    # located. It is still required: without a failure there is nothing to suspend, and the
    # three assertions that follow could not all hold.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    # The dead target is withdrawn, so all three of these land on the live one. Three
    # consecutive requests over a two-entry rotation cover the withdrawn target's turn at least
    # once; were the suspension not honoured, that turn would answer 503 instead.
    #
    # No attempt-count assertion anywhere in this scenario, deliberately. The controller asks
    # for the count header on a virtual host only when one of its routes carries failover, so a
    # proxy without it emits none - asserting on it would test the absence of the feature
    # rather than this scenario's subject.
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"

    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"

    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A failover-only vendor takes no traffic until every in-house target has failed
    # A publisher's usual reason for a cross-vendor fallback is price: the in-house models are
    # paid for, the vendor is billed per call. "It works when everything else is down" is only
    # half the requirement. The other half - that it is never reached on an ordinary request -
    # is the half a publisher watches, and the one no scenario asserted.
    #
    # Pointing the vendor at the testbench capture service is what makes that half provable.
    # Asserting a response did not come from the vendor would also be satisfied by a vendor
    # that WAS called and merely lost the race; a capture holding nothing says the call was
    # never made. The positive control is read from the same capture state after the same
    # reset, so "recorded nothing" cannot quietly mean "recording was broken".
    #
    # Two APIs rather than two operations, because an Operation has no upstream of its own -
    # `spec.upstream.main.ref` is API-level, so one API cannot host both a healthy in-house
    # upstream and a dead one. Each API's chain therefore takes the shape that is already
    # proven elsewhere: the first target names nothing and so resolves to that API's own main
    # upstream, and only the FALLBACK names another definition.
    Given I resolve the "capture" service path at "/vendor-sink" and store it as "vendorPath"

    # API ONE - in-house healthy. Its chain can still reach the vendor; nothing should.
    Given I generate a unique resource name from "vendor-hold-ok" and store it as "okName"
    And I generate a unique API version from "vendor-hold-ok" and store it as "okVersion"
    And I generate a unique API context from "/vendor-hold-ok" and store it as "okContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:okName}             |
      | spec.displayName         | ${CTX:okName}             |
      | spec.version             | ${CTX:okVersion}          |
      | spec.context             | ${CTX:okContext}/$version |
      | spec.upstreamDefinitions | [{"name":"inhouse","basePath":"/inhouse-live","upstreams":[{"url":"http://testbench:3000"}]},{"name":"vendor","basePath":"${CTX:vendorPath}","upstreams":[{"url":"http://testbench:3010"}]}] |
      | spec.upstream.main.ref   | inhouse                   |
      | spec.operations          | [{"method":"POST","path":"/healthy","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"premium-1","provider":"vendor"}]}]}}]}] |
    Then the response should be successful

    # API TWO - the in-house upstream REFUSES the connection, so the chain advances and
    # reaches the vendor. This is the positive control.
    #
    # It points at a port nothing listens on rather than at a host that does not resolve,
    # and the distinction is the policy's own documented limit: a member whose host does not
    # resolve is answered locally, before any upstream request exists, and a local answer is
    # not retriable - so the chain would stop there and never reach the vendor. A member that
    # resolves but refuses fails over normally. The LLM proxy scenarios elsewhere in this
    # suite can use a non-resolving host because a proxy reaches its providers through a
    # loopback definition, which puts the resolution failure on the second hop; a RestApi
    # pointed straight at a bad host has no such indirection.
    Given I generate a unique resource name from "vendor-hold-dead" and store it as "deadName"
    And I generate a unique API version from "vendor-hold-dead" and store it as "deadVersion"
    And I generate a unique API context from "/vendor-hold-dead" and store it as "deadContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}   |
      | name                     | ${CTX:deadName}             |
      | spec.displayName         | ${CTX:deadName}             |
      | spec.version             | ${CTX:deadVersion}          |
      | spec.context             | ${CTX:deadContext}/$version |
      | spec.upstreamDefinitions | [{"name":"inhouse","upstreams":[{"url":"http://testbench:9099"}]},{"name":"vendor","basePath":"${CTX:vendorPath}","upstreams":[{"url":"http://testbench:3010"}]}] |
      | spec.upstream.main.ref   | inhouse                     |
      | spec.operations          | [{"method":"POST","path":"/allfailed","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"premium-1","provider":"vendor"}]}]}}]}] |
    Then the response should be successful

    # Readiness on both, before anything is measured. Reaching 200 on the dead-upstream API
    # proves its whole chain - including the vendor - is live, and the reset afterwards
    # discards everything these polls recorded.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:deadContext}/${CTX:deadVersion}/allfailed" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:okContext}/${CTX:okVersion}/healthy" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a "POST" request to the "capture" service at "/test/reset"
    Then the response should be successful

    # Three ordinary requests against the healthy API. Each is served by the in-house upstream
    # in a single attempt, so no attempt is left for the chain to spend on the vendor. Sent
    # plainly rather than polled: a polling send keeps going until it sees a 200 and so could
    # absorb the very request that broke the claim.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:okContext}/${CTX:okVersion}/healthy" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:okContext}/${CTX:okVersion}/healthy" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:okContext}/${CTX:okVersion}/healthy" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # POSITIVE CONTROL, after the same reset.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:deadContext}/${CTX:deadVersion}/allfailed" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the "capture" service at "/test/captured?path=/vendor-sink/allfailed"
    Then the response status code should be 200

    # THE CLAIM. 204 is the capture saying it holds nothing for the healthy API's path - three
    # requests went through a chain whose tail is the vendor, and the vendor was never called.
    # The control above proves this same capture was recording throughout.
    When I send a "GET" request to the "capture" service at "/test/captured?path=/vendor-sink/healthy"
    Then the response status code should be 204

    When I delete the API "${CTX:okName}"
    Then the response should be successful
    When I delete the API "${CTX:deadName}"
    Then the response should be successful
