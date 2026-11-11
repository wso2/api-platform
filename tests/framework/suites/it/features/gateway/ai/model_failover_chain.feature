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

@model-failover-chain
Feature: Walking a failover chain
  What a chain is made of and how far it is walked. The core feature covers the single
  rescue; these cover a chain deeper than two, what identifies one target as distinct from
  another, and the guarantee that a rescue leaves no trace in the response.

  A dead target is produced by pointing a provider's upstream at a host that does not
  resolve. On an LlmProxy each provider sits behind a loopback cluster which is always
  healthy, so the DNS failure happens on the provider's own inner route and reaches the
  proxy as an ordinary retriable upstream 503.

  suspendDuration is 0 throughout. A target withdrawn after its first failure would let a
  later request go straight to a healthy one, so the attempt counts below would stop
  describing the chain and start describing the suspension. Suspension has its own feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Two failing targets in a row are both passed over
    Given I generate a unique resource name from "chain-dead-first" and store it as "firstName"
    And I generate a unique API version from "chain-dead-first" and store it as "firstVersion"
    And I generate a unique API context from "/chain-dead-first" and store it as "firstContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                       |
      | name               | ${CTX:firstName}                                |
      | displayName        | ${CTX:firstName}                                |
      | version            | ${CTX:firstVersion}                             |
      | template           | openai                                          |
      | spec.context       | ${CTX:firstContext}                             |
      | spec.upstream.url  | http://chain-nonexistent-first:9080/dead-first  |
      | accessControl.mode | allow_all                                       |
    Then the response should be successful

    Given I generate a unique resource name from "chain-dead-second" and store it as "secondName"
    And I generate a unique API version from "chain-dead-second" and store it as "secondVersion"
    And I generate a unique API context from "/chain-dead-second" and store it as "secondContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                        |
      | name               | ${CTX:secondName}                                |
      | displayName        | ${CTX:secondName}                                |
      | version            | ${CTX:secondVersion}                             |
      | template           | openai                                           |
      | spec.context       | ${CTX:secondContext}                             |
      | spec.upstream.url  | http://chain-nonexistent-second:9080/dead-second |
      | accessControl.mode | allow_all                                        |
    Then the response should be successful

    Given I generate a unique resource name from "chain-third" and store it as "thirdName"
    And I generate a unique API version from "chain-third" and store it as "thirdVersion"
    And I generate a unique API context from "/chain-third" and store it as "thirdContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:thirdName}                     |
      | displayName        | ${CTX:thirdName}                     |
      | version            | ${CTX:thirdVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:thirdContext}                  |
      | spec.upstream.url  | http://testbench:3000/third-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    # Three targets in declared order. The first two cannot answer, so only a chain that
    # keeps walking past a SECOND failure ever reaches the third.
    Given I generate a unique resource name from "chain-deep" and store it as "proxyName"
    And I generate a unique API version from "chain-deep" and store it as "proxyVersion"
    And I generate a unique API context from "/chain-deep" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:firstName}                                                                                                                                                       |
      | spec.providers | [{"id":"${CTX:firstName}","isPrimary":true},{"id":"${CTX:secondName}","alias":"second"},{"id":"${CTX:thirdName}","alias":"third"}]                                                                                      |
      | spec.operationPolicies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"second"},{"model":"command-r","provider":"third"}]}]}}]}] |
    Then the response should be successful

    # THREE attempts, and the third target's response. Two would mean the chain gave up
    # after one failure; a 503 would mean it never reached the third at all.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/third-upstream"
    And the response header "x-envoy-attempt-count" should be "3"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:firstName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:secondName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:thirdName}"
    Then the response should be successful

  Scenario: A rescued request leaks nothing about the attempt that failed
    # The client asked one question and got one answer. Everything the gateway did to
    # produce it - which provider it tried first, how it steered the retry - is internal,
    # and a response carrying traces of it turns an implementation detail into something a
    # client can come to depend on.
    Given I generate a unique resource name from "chain-quiet-dead" and store it as "deadName"
    And I generate a unique API version from "chain-quiet-dead" and store it as "deadVersion"
    And I generate a unique API context from "/chain-quiet-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                      |
      | name               | ${CTX:deadName}                                |
      | displayName        | ${CTX:deadName}                                |
      | version            | ${CTX:deadVersion}                             |
      | template           | openai                                         |
      | spec.context       | ${CTX:deadContext}                             |
      | spec.upstream.url  | http://chain-quiet-nonexistent:9080/dead-quiet |
      | accessControl.mode | allow_all                                      |
    Then the response should be successful

    Given I generate a unique resource name from "chain-quiet-alive" and store it as "aliveName"
    And I generate a unique API version from "chain-quiet-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/chain-quiet-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:aliveName}                     |
      | displayName        | ${CTX:aliveName}                     |
      | version            | ${CTX:aliveVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:aliveContext}                  |
      | spec.upstream.url  | http://testbench:3000/quiet-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    Given I generate a unique resource name from "chain-quiet" and store it as "proxyName"
    And I generate a unique API version from "chain-quiet" and store it as "proxyVersion"
    And I generate a unique API context from "/chain-quiet" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.providers | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:aliveName}","alias":"alive"}]                                                                                                                               |
      | spec.operationPolicies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # A rescue did happen: two attempts, answered by the fallback. Without this the
    # assertions below would pass on a request that never failed over.
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/quiet-upstream"

    # The steering headers the gateway uses internally must not reach the client.
    And the response header "x-target-upstream" should not exist
    And the response header "x-wso2-internal-loopback" should not exist

    # No trace of the target that failed, in a body the client reads as an ordinary model
    # response - and no gateway-invented error text alongside the success it did return.
    And the response body should not contain "${CTX:deadName}"
    And the response body should not contain "dead-quiet"
    And the response body should not contain "no healthy upstream"
    And the response body should not contain "upstream connect error"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: The same model name under a second provider is reached with that provider's own endpoint
    # A publisher running one model in two regions, or through two accounts, names the SAME
    # model twice. Were a target keyed on the model alone, the second entry would be a
    # duplicate of the first and the fallback a no-op - the request would simply re-attempt
    # the broken configuration.
    #
    # This scenario, alone in the failover suite, is authored in the LEGACY provider shape -
    # `provider` plus `additionalProviders` with `as` - which the schema supports indefinitely
    # and which the controller normalises into the same internal attachment the canonical
    # `providers` list produces. Everything else here uses the canonical shape, so without one
    # scenario holding the older form the suite would stop covering it end to end. The
    # normalisation itself is unit-tested; what nothing else would show is that a
    # legacy-authored proxy still gets a working per-attempt chain.
    #
    # It takes a complete `spec` override, because the canonical template renders a
    # `providers` list and supplying both shapes is rejected. The template's placeholders are
    # still passed so rendering succeeds; the override replaces the spec before the request
    # goes out.
    Given I generate a unique resource name from "chain-region-a" and store it as "regionAName"
    And I generate a unique API version from "chain-region-a" and store it as "regionAVersion"
    And I generate a unique API context from "/chain-region-a" and store it as "regionAContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:regionAName}                          |
      | displayName        | ${CTX:regionAName}                          |
      | version            | ${CTX:regionAVersion}                       |
      | template           | openai                                      |
      | spec.context       | ${CTX:regionAContext}                       |
      | spec.upstream.url  | http://chain-region-a-nonexistent:9080/a    |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    Given I generate a unique resource name from "chain-region-b" and store it as "regionBName"
    And I generate a unique API version from "chain-region-b" and store it as "regionBVersion"
    And I generate a unique API context from "/chain-region-b" and store it as "regionBContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}               |
      | name               | ${CTX:regionBName}                      |
      | displayName        | ${CTX:regionBName}                      |
      | version            | ${CTX:regionBVersion}                   |
      | template           | openai                                  |
      | spec.context       | ${CTX:regionBContext}                   |
      | spec.upstream.url  | http://testbench:3000/region-b-upstream |
      | accessControl.mode | allow_all                               |
    Then the response should be successful

    # Both targets name gpt-4o. Only the provider differs.
    Given I generate a unique resource name from "chain-same-model" and store it as "proxyName"
    And I generate a unique API version from "chain-same-model" and store it as "proxyVersion"
    And I generate a unique API context from "/chain-same-model" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:regionAName}                                                                                                                                                     |
      | spec                     | {"displayName":"${CTX:proxyName}","version":"${CTX:proxyVersion}","context":"${CTX:proxyContext}","provider":{"id":"${CTX:regionAName}"},"additionalProviders":[{"id":"${CTX:regionBName}","as":"region-b"}],"operationPolicies":[{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"gpt-4o","provider":"region-b"}]}]}}]}]} |
    Then the response should be successful

    # Two attempts, answered by region B's OWN endpoint. One attempt would mean the second
    # entry was folded into the first and the chain had nowhere to go.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/region-b-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:regionAName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:regionBName}"
    Then the response should be successful

  Scenario: Suspending one provider's copy of a model leaves the other provider's untouched
    # The sibling scenario above proves two entries naming the SAME model are kept distinct
    # for ROUTING. This one proves they are kept distinct for HEALTH, which is a separate
    # store and a separate way to conflate them: a health view keyed on the model alone would
    # suspend "gpt-4o" outright, taking out the healthy region with the broken one.
    #
    # That failure is loud rather than subtle, which is what makes it assertable. With both
    # copies suspended the chain has no candidate left and the policy answers 503
    # failover_exhausted without dispatching anything - so a 200 from region B after region A
    # is suspended is the whole claim.
    #
    # Both providers carry an alias, including the primary: a suspension record names the
    # provider the target named, and a target naming nothing logs an empty one, which
    # identifies nothing in a log every runner on this gateway shares.
    Given I generate a unique resource name from "health-region-a" and store it as "sickName"
    And I generate a unique API version from "health-region-a" and store it as "sickVersion"
    And I generate a unique API context from "/health-region-a" and store it as "sickContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:sickName}                           |
      | displayName        | ${CTX:sickName}                           |
      | version            | ${CTX:sickVersion}                        |
      | template           | openai                                    |
      | spec.context       | ${CTX:sickContext}                        |
      | spec.upstream.url  | http://health-region-a-nonexistent:9080/a |
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    Given I generate a unique resource name from "health-region-b" and store it as "wellName"
    And I generate a unique API version from "health-region-b" and store it as "wellVersion"
    And I generate a unique API context from "/health-region-b" and store it as "wellContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                      |
      | name               | ${CTX:wellName}                                |
      | displayName        | ${CTX:wellName}                                |
      | version            | ${CTX:wellVersion}                             |
      | template           | openai                                         |
      | spec.context       | ${CTX:wellContext}                             |
      | spec.upstream.url  | http://testbench:3000/health-region-b-upstream |
      | accessControl.mode | allow_all                                      |
    Then the response should be successful

    # One model name, two providers. suspendAfterFailures is 1 so a single failed attempt
    # suspends region A, and the window is held well past the scenario's own runtime so the
    # second request cannot be served by region A quietly coming back.
    Given I generate a unique resource name from "health-same-model" and store it as "proxyName"
    And I generate a unique API version from "health-same-model" and store it as "proxyVersion"
    And I generate a unique API context from "/health-same-model" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                   |
      | name                   | ${CTX:proxyName}                                                                                                            |
      | displayName            | ${CTX:proxyName}                                                                                                            |
      | version                | ${CTX:proxyVersion}                                                                                                         |
      | context                | ${CTX:proxyContext}                                                                                                         |
      | provider.id            | ${CTX:sickName}                                                                                                             |
      | spec.providers         | [{"id":"${CTX:sickName}","alias":"region-a","isPrimary":true},{"id":"${CTX:wellName}","alias":"region-b"}]                  |
      | spec.operationPolicies | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o","provider":"region-a","fallbacks":[{"model":"gpt-4o","provider":"region-b"}]}]}}]}] |
    Then the response should be successful

    # Request one: region A is attempted, fails, and region B answers on the same model name.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/health-region-b-upstream"
    # The record names region A. Were health keyed on the model, there would be nothing in it
    # to distinguish which of the two copies of gpt-4o had just been withdrawn.
    And the "gateway-runtime" log since the latest request should contain "provider=region-a reason=consecutive_failures observed=1 sample_size=0 duration_seconds=300 escalation_step=1"

    # Request two, sent once rather than polled - a poll would wait out a wrong answer instead
    # of reporting it. ONE attempt proves region A was skipped rather than retried, and a 200
    # from region B proves its health survived its sibling's suspension. Had the suspension
    # applied to the model, this would be 503 failover_exhausted with nothing dispatched.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/health-region-b-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:sickName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:wellName}"
    Then the response should be successful
