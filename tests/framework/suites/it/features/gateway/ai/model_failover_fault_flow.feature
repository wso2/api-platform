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

@model-failover-fault-flow
Feature: Failover on the fault flow
  With handle_upstream_faults on, a failing response runs through the FAULT chain instead of
  the response chain. For model-failover those are exactly the responses it grades, so a
  regression on this path leaves the policy not degraded but inert: no health recorded, no
  target suspended, the upstream's own error reaching the client - while every scenario in
  the ordinary blocks keeps passing, because they never run this path.

  The posture is process-level, which is why it gets a block of its own with its own overlay
  rather than a tag or a scenario setting: the gateway has to be started with it.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Failover keeps grading and keeps substituting when the fault flow handles the failure
    # Both members are the same live service on paths that name no valid status code, so each
    # answers 400 with a body of its own and the chain is spent. A reachable upstream's real
    # error is the point: a chain of unresolvable hosts fails at transport level, which Envoy
    # answers itself without ever calling the policy.
    Given I generate a unique resource name from "faultflow" and store it as "apiName"
    And I generate a unique API version from "faultflow" and store it as "apiVersion"
    And I generate a unique API context from "/faultflow" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"faultflow-lead","basePath":"/status/faultflow-primary","upstreams":[{"url":"http://testbench:3002"}]},{"name":"faultflow-next","basePath":"/status/faultflow-secondary","upstreams":[{"url":"http://testbench:3002"}]}] |
      | spec.upstream.main.ref   | faultflow-lead                                                                                                                                                         |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendAfterFailures":1,"suspendDuration":120,"statusCodes":[400],"targets":[{"model":"gpt-4o","provider":"faultflow-lead","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"faultflow-next"}]}]}}]}] |
    Then the response should be successful

    # Polled on the body as well as the status, so a cold-start 503 on the way to readiness
    # cannot end the wait on a response the policy never produced. The request this stops at
    # is the first to reach the policy, and therefore the one that walks the chain: nothing
    # is suspended yet.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 503 and the response body contains "failover_exhausted" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """

    # THE posture assertion, and the scenario's first job. A block whose overlay silently
    # stopped applying would be indistinguishable from an ordinary run: everything below
    # passes on both paths by design, so without this line the scenario would quietly become
    # a second copy of the exhaustion scenario in the core block.
    Then the "gateway-runtime" log since the latest request should contain "skipping the response policies for the fault policies"

    # The substitution survives the different return type this path uses. There is no
    # immediate-response mechanism in the fault chain, so the same decision has to be
    # expressed as an edit to the error the client is already receiving.
    And the response status code should be 503
    And the response body should contain "failover_exhausted"
    And the response body should not contain "status code must be 200-599"

    # And health was graded too, by the hook that runs INSTEAD of the response one. Both
    # members were charged, so the chain is out of rotation and this request is refused
    # before anything is dispatched. This is the assertion that fails if the two chains do
    # not share a health store: grading would land where nothing reads it, and the request
    # would traverse again instead.
    #
    # Read off the record's target lists rather than its message, matching the core block's
    # equivalent: a refusal skips every member and attempts none, a walked chain fails them.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"
    And the "gateway-runtime" log since the latest request should contain "skipped_targets=faultflow-lead:gpt-4o,faultflow-next:claude-sonnet-4-5-20250929"
    And the "gateway-runtime" log since the latest request should not contain "failed_targets=faultflow-lead:gpt-4o"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
