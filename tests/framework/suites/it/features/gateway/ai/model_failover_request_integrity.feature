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

@model-failover-request-integrity
Feature: What a re-attempted request carries, and what it must not
  The downstream chain runs ONCE and the upstream chain runs per attempt. Both scenarios
  here read the bytes a provider actually received, because both failures they guard
  against are silent: the client gets its 200 either way, every policy reports success
  either way, and the only trace is in a request body somebody else's backend was handed.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: An API without failover is untouched by one that has it, even sharing a provider
    # Per-attempt chains are keyed by CLUSTER, and two APIs attaching the same provider
    # reach it through API-scoped clusters. Were that keying wrong - were a chain stored
    # under a cluster name both APIs share - one API's chain would run on the other's
    # requests, and since a provider's credential is the last policy in that chain, API A's
    # key would be applied to API B's request.
    Given I generate a unique resource name from "xapi-dead" and store it as "deadName"
    And I generate a unique API version from "xapi-dead" and store it as "deadVersion"
    And I generate a unique API context from "/xapi-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}             |
      | name               | ${CTX:deadName}                       |
      | displayName        | ${CTX:deadName}                       |
      | version            | ${CTX:deadVersion}                    |
      | template           | openai                                |
      | spec.context       | ${CTX:deadContext}                    |
      | spec.upstream.url  | http://xapi-nonexistent:9080/dead     |
      | accessControl.mode | allow_all                             |
    Then the response should be successful

    Given I generate a unique resource name from "xapi-shared" and store it as "sharedName"
    And I generate a unique API version from "xapi-shared" and store it as "sharedVersion"
    And I generate a unique API context from "/xapi-shared" and store it as "sharedContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                |
      | name               | ${CTX:sharedName}                        |
      | displayName        | ${CTX:sharedName}                        |
      | version            | ${CTX:sharedVersion}                     |
      | template           | openai                                   |
      | spec.context       | ${CTX:sharedContext}                     |
      | spec.upstream.url  | http://testbench:3000/xapi-shared-upstream |
      | accessControl.mode | allow_all                                |
    Then the response should be successful

    # API A: failover attached, reaching the shared provider with ITS key.
    Given I generate a unique resource name from "xapi-a" and store it as "proxyAName"
    And I generate a unique API version from "xapi-a" and store it as "proxyAVersion"
    And I generate a unique API context from "/xapi-a" and store it as "proxyAContext"
    And I generate a unique value from "xapi-a-credential" and store it as "credentialA"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyAName}                                                                                                                                                      |
      | displayName              | ${CTX:proxyAName}                                                                                                                                                      |
      | version                  | ${CTX:proxyAVersion}                                                                                                                                                   |
      | context                  | ${CTX:proxyAContext}                                                                                                                                                   |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:sharedName}","as":"shared","auth":{"type":"api-key","header":"x-api-key","value":"${CTX:credentialA}"}}]                                                 |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"shared"}]}]}}]}] |
    Then the response should be successful

    # API B: the SAME provider, no failover at all, and a marker of its own set by an
    # ordinary operation policy. A marker rather than a credential deliberately -
    # primary-provider auth and additionalProviders auth do not reach the upstream by the
    # same route, and this is about chain isolation, not about characterising that.
    Given I generate a unique resource name from "xapi-b" and store it as "proxyBName"
    And I generate a unique API version from "xapi-b" and store it as "proxyBVersion"
    And I generate a unique API context from "/xapi-b" and store it as "proxyBContext"
    And I generate a unique value from "xapi-b-marker" and store it as "markerB"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion    | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name          | ${CTX:proxyBName}                                                                                                                                                      |
      | displayName   | ${CTX:proxyBName}                                                                                                                                                      |
      | version       | ${CTX:proxyBVersion}                                                                                                                                                   |
      | context       | ${CTX:proxyBContext}                                                                                                                                                   |
      | provider.id   | ${CTX:sharedName}                                                                                                                                                      |
      | spec.policies | [{"name":"set-headers","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"request":{"headers":[{"name":"x-xapi-b-marker","value":"${CTX:markerB}"}]}}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # Drive API A first, so its per-attempt chain is live before B is observed.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyAContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "${CTX:credentialA}"

    # Now API B, through the same provider.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyBContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/xapi-shared-upstream"
    # B's own configuration ran: the marker its policy sets is on the request the backend
    # received, which also anchors the absence assertion below against an empty body.
    And the response body should contain "${CTX:markerB}"
    # And nothing of A's did. A's credential on B's request would be a real leak between
    # two APIs, applied by a chain that should never have run here.
    And the response body should not contain "${CTX:credentialA}"

    # B DOES carry x-envoy-attempt-count, and that is a known deviation rather than the
    # intent. Envoy exposes include_attempt_count_in_response on a VirtualHost only - there
    # is no per-route equivalent - so enabling it for A's routes enables it for every API
    # sharing that vhost, and B's responses gain an informational header it would not
    # otherwise have. Asserted as it behaves, so the day someone scopes this properly the
    # line fails and is updated deliberately.
    And the response header "x-envoy-attempt-count" should be "1"

    When I delete the LLM proxy "${CTX:proxyAName}"
    Then the response should be successful
    When I delete the LLM proxy "${CTX:proxyBName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:sharedName}"
    Then the response should be successful

  Scenario: PII redacted downstream stays redacted on the fallback attempt
    # If the upstream position read a snapshot of the body taken before the downstream
    # chain mutated it, the first attempt would carry redacted data and every retry would
    # carry the ORIGINAL - so a fallback provider receives the raw PII that was redacted
    # precisely so it would not.
    #
    # Redaction rather than masking deliberately: masking restores the original value on
    # the response, which would put the raw address back into what this scenario reads and
    # hide the very thing it is looking for.
    Given I generate a unique resource name from "pii-dead" and store it as "deadName"
    And I generate a unique API version from "pii-dead" and store it as "deadVersion"
    And I generate a unique API context from "/pii-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}         |
      | name               | ${CTX:deadName}                   |
      | displayName        | ${CTX:deadName}                   |
      | version            | ${CTX:deadVersion}                |
      | template           | openai                            |
      | spec.context       | ${CTX:deadContext}                |
      | spec.upstream.url  | http://pii-nonexistent:9080/dead  |
      | accessControl.mode | allow_all                         |
    Then the response should be successful

    Given I generate a unique resource name from "pii-alive" and store it as "aliveName"
    And I generate a unique API version from "pii-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/pii-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:aliveName}                           |
      | displayName        | ${CTX:aliveName}                           |
      | version            | ${CTX:aliveVersion}                        |
      | template           | openai                                     |
      | spec.context       | ${CTX:aliveContext}                        |
      | spec.upstream.url  | http://testbench:3000/pii-alive-upstream   |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    # Redaction runs downstream, once. Failover walks the chain.
    Given I generate a unique resource name from "pii-proxy" and store it as "proxyName"
    And I generate a unique API version from "pii-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/pii-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"pii-masking-regex","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"customPIIEntities":[{"piiEntity":"EMAIL","piiRegex":"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+[.][a-zA-Z]{2,}"}],"jsonPath":"","redactPII":true}}]},{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "contact me at leak-probe@example.com please"}]}
      """
    # The first target failed and the SECOND served it.
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/pii-alive-upstream"
    # THE assertion. This is the body the fallback provider received, echoed back. The raw
    # address appearing here would mean the retry was built from a pre-redaction snapshot
    # and the redaction was undone by the failover.
    And the response body should not contain "leak-probe@example.com"
    # And the body reached it INTACT, with only the address taken out. Without these two
    # the assertion above would pass just as well on an empty body or a dropped field -
    # proving the address was absent rather than that it was redacted from a body that
    # otherwise arrived whole.
    And the response body should contain "contact me at"
    And the response body should contain "please"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
