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
      | spec.additionalProviders | [{"id":"${CTX:secondName}","as":"second"},{"id":"${CTX:thirdName}","as":"third"}]                                                                                      |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"second"},{"model":"command-r","provider":"third"}]}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

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
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

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
      | spec.additionalProviders | [{"id":"${CTX:regionBName}","as":"region-b"}]                                                                                                                          |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"gpt-4o","provider":"region-b"}]}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

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
