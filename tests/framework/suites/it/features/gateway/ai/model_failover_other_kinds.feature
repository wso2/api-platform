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

@model-failover-other-kinds
Feature: Failover on a kind that is not an LlmProxy
  Applicability is decided by whether a target RESOLVES, not by a list of permitted kinds.
  On an LlmProxy a target's provider names an alias from the proxy's providers list; on a
  RestApi it names an upstreamDefinition. Name them the same and the policy block is byte-for-byte
  identical - only the wrapper around it differs, because that is each kind's own
  attachment schema and not part of the failover configuration.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: The same failover configuration behaves the same on a RestApi
    # The dead upstream here REFUSES rather than failing DNS, and that difference is the
    # point rather than an incidental choice. A RestApi has no loopback hop: Envoy answers
    # an unresolvable host locally with no_healthy_upstream, which is terminal, and no
    # failover occurs. On an LlmProxy the same name fails on the provider's inner route and
    # returns a retriable 503. A refused connection is retriable on both, so it is the
    # failure mode that lets one configuration be compared across the two kinds at all.
    #
    # The two definitions carry DIFFERENT basePaths deliberately. Selecting the fallback
    # cluster and sending it the fallback's own base path are separate mechanisms: the
    # first is Envoy's priority_group and was always kind-agnostic, the second runs in the
    # per-attempt upstream filter chain. That chain used to be built for an LlmProxy only,
    # so a RestApi fallback was reached but served on the PRIMARY's base path - a 200 from
    # the wrong resource, with no error anywhere. Give both definitions the same basePath
    # and this scenario passes either way, which is exactly why it must not.
    Given I generate a unique resource name from "rest-failover" and store it as "apiName"
    And I generate a unique API version from "rest-failover" and store it as "apiVersion"
    And I generate a unique API context from "/rest-failover" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"/rest-dead-upstream","upstreams":[{"url":"http://testbench:9998"}]},{"name":"alive","basePath":"/rest-alive-upstream","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # Identical outcome to the LlmProxy scenarios: the first target refuses, the second
    # answers, and the client sees one success after two attempts.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/rest-alive-upstream"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
