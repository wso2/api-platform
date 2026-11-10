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

@model-failover-error-surface
Feature: What the client sees when a chain cannot serve the request
  A chain that runs out, and a chain already withdrawn, both end the same way: one
  consistent gateway error, with nothing of the upstream's own reply in it. And a status the
  publisher never nominated is not a failure at all - it comes back untouched and costs the
  target nothing.

  Every upstream here is the testbench echo on a path under /status/ that names no valid
  code, so it answers 400 with a body of its own. That shape is what these scenarios need:
  a chain of unresolvable hosts fails at transport level and Envoy answers those itself
  without ever calling the policy, leaving nothing for the policy to replace. A real error
  from a reachable upstream is a response the policy actually sees, and its body is the
  thing that must not survive.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: An exhausted chain returns the gateway's own error, not the last provider's
    # The two base paths differ so the attempts are distinguishable; neither names a valid
    # status code, so both answer 400 and the chain is spent. statusCodes nominates 400 for
    # the same reason the old fixture nominated 404: the parameter exists precisely so a
    # publisher can say what counts as a failure.
    Given I generate a unique resource name from "exhaust" and store it as "apiName"
    And I generate a unique API version from "exhaust" and store it as "apiVersion"
    And I generate a unique API context from "/exhaust" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"/status/exhaust-primary","upstreams":[{"url":"http://testbench:3002"}]},{"name":"secondary","basePath":"/status/exhaust-secondary","upstreams":[{"url":"http://testbench:3002"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"statusCodes":[400],"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"secondary"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 503 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # One consistent status for exhaustion, NOT the 400 the last target answered. A chain
    # that ends on the upstream's own error is not that error - the resource is there, the
    # gateway simply had nowhere left to send the request.
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"
    # THE assertion this scenario exists for. Anything the upstream wrote must be gone:
    # here the echo's own complaint, standing in for the provider error that in production
    # names the account, region or key that was refused.
    And the response body should not contain "status code must be 200-599"
    # And the body says nothing about the chain either. Which models a publisher configured
    # is not something the caller was ever shown.
    And the response body should not contain "gpt-4o"
    And the response body should not contain "claude"
    And the response body should not contain "exhaust-secondary"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Once every target is suspended the next request is refused without an attempt
    # Suspension that still sends traffic to the suspended target is not suspension. With
    # every member out of rotation the policy used to publish no ranking and go inert, which
    # handed the request to the route's configured upstream - the primary, the very target
    # health had just withdrawn.
    Given I generate a unique resource name from "allsusp" and store it as "apiName"
    And I generate a unique API version from "allsusp" and store it as "apiVersion"
    And I generate a unique API context from "/allsusp" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"/status/allsusp-primary","upstreams":[{"url":"http://testbench:3002"}]},{"name":"secondary","basePath":"/status/allsusp-secondary","upstreams":[{"url":"http://testbench:3002"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendAfterFailures":1,"suspendDuration":120,"statusCodes":[400],"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"secondary"}]}]}}]}] |
    Then the response should be successful

    # The first request walks the whole chain and fails it. Both members are graded - the
    # first from the attempt count, the second from the response in hand - so one request is
    # enough to put the entire chain out of rotation.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" until status 503 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"

    # The next request never leaves the gateway. The suspension window is far longer than
    # the scenario, so a pass here cannot be a window quietly expiring.
    #
    # The response ALONE cannot prove that: refusing before any attempt and exhausting a
    # chain of attempts return byte-identical responses, which is the design - one
    # consistent error, whatever the reason. So the client-side assertions establish only
    # that the request was refused, and the log assertions after them establish that it was
    # refused WITHOUT being tried, which is the actual subject here.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"
    # Had the request been dispatched to the suspended primary, this is the body that would
    # have come back.
    And the response body should not contain "status code must be 200-599"
    # THE discriminator. This line is written only on the request-phase path, before
    # anything is dispatched...
    And the "gateway-runtime" log since the latest request should contain "every target is suspended; nothing was attempted"
    # ...and this one only after a chain has actually been walked and spent. Its absence is
    # what rules out the request having been attempted again.
    And the "gateway-runtime" log since the latest request should not contain "every target in the chain failed"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A status outside the failure conditions is returned unchanged and fails over to nothing
    # Retrying a validation error on another provider fails the same way, or worse, bills a
    # second inference call to mask a client bug - so the status has to come back as-is and
    # the target must not be penalised for it.
    #
    # Two operations, ONE policy configuration, differing only in what the upstream answers.
    # The 503 operation is a positive control and is the point: "no failover occurred" is
    # satisfied just as well by a policy that never fails over at all, so without a sibling
    # operation proving the same configuration DOES fail over, this scenario would pass
    # against a completely broken policy.
    Given I generate a unique resource name from "passthru" and store it as "apiName"
    And I generate a unique API version from "passthru" and store it as "apiVersion"
    And I generate a unique API context from "/passthru" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","upstreams":[{"url":"http://testbench:3002"}]},{"name":"alive","basePath":"/passthru-alive","upstreams":[{"url":"http://testbench:3000"}]}]         |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/status/422","policies":[{"name":"model-failover","version":"v0","params":{"suspendAfterFailures":1,"suspendDuration":120,"statusCodes":[503],"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]},{"method":"POST","path":"/status/503","policies":[{"name":"model-failover","version":"v0","params":{"suspendAfterFailures":1,"suspendDuration":120,"statusCodes":[503],"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # The control first, so a failure here reads as "failover is broken" rather than as a
    # finding about pass-through.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/status/503" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/passthru-alive"

    # The subject. Same policy, same chain, a status the publisher did not nominate: one
    # attempt, and the upstream's own status reaches the client.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/status/422" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 422
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should not contain "/passthru-alive"

    # And the target was not penalised for the client's mistake. A second request still
    # reaches it, which it would not if the 422 had counted towards suspension.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/status/422" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 422
    And the response body should not contain "/passthru-alive"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: However the primary fails, a healthy fallback means the client never sees a 5xx
    # The property the whole feature exists for, asserted as its own subject rather than as a
    # side effect of scenarios testing something else.
    #
    # Two failure modes, because they reach the guarantee by different routes and only one of
    # them involves this policy at all. An error status is graded by the policy; an attempt
    # cut off at perTryTimeout is a local reply Envoy never calls the policy for, so the
    # rescue there is Envoy's retry policy alone. A guarantee that holds for the path the
    # policy drives says nothing about the path it never sees.
    #
    # They sit on two APIs rather than one because an operation cannot name its own upstream,
    # and the two modes need different ones: a status the echo returns outright, and a delay
    # only the fault injector can produce. A refused connection is the third mode and is
    # deliberately elsewhere - it is the subject of the RestApi parity scenario, which turns
    # on refusal being retriable on both API kinds.
    Given I generate a unique resource name from "no5xx-graded" and store it as "gradedName"
    And I generate a unique API version from "no5xx-graded" and store it as "gradedVersion"
    And I generate a unique API context from "/no5xx-graded" and store it as "gradedContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:gradedName}                                                                                                                                                      |
      | spec.displayName         | ${CTX:gradedName}                                                                                                                                                      |
      | spec.version             | ${CTX:gradedVersion}                                                                                                                                                   |
      | spec.context             | ${CTX:gradedContext}/$version                                                                                                                                          |
      | spec.upstreamDefinitions | [{"name":"primary","upstreams":[{"url":"http://testbench:3002"}]},{"name":"alive","basePath":"/no5xx-graded-alive","upstreams":[{"url":"http://testbench:3000"}]}]     |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/status/503","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # The primary answers with an error status. The policy grades it, and the retry lands on
    # the fallback.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:gradedContext}/${CTX:gradedVersion}/status/503" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/no5xx-graded-alive"

    # Every step of the script delays, so the readiness poll below can consume as many as it
    # needs without changing what the next request meets.
    Given I generate a unique resource name from "no5xx-cutoff-script" and store it as "scriptId"
    When I send a "PUT" request to the "fault-injection" service at "/test/scripts/${CTX:scriptId}" with body:
      """
      {"forwardPort": 3000, "steps": [
        {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000},
        {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000},
        {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000},
        {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000}, {"delayMs": 5000}
      ]}
      """
    Then the response should be successful
    And I resolve the "fault-injection" service path at "/${CTX:scriptId}" and store it as "cutoffPath"

    Given I generate a unique resource name from "no5xx-cutoff" and store it as "cutoffName"
    And I generate a unique API version from "no5xx-cutoff" and store it as "cutoffVersion"
    And I generate a unique API context from "/no5xx-cutoff" and store it as "cutoffContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:cutoffName}                                                                                                                                                      |
      | spec.displayName         | ${CTX:cutoffName}                                                                                                                                                      |
      | spec.version             | ${CTX:cutoffVersion}                                                                                                                                                   |
      | spec.context             | ${CTX:cutoffContext}/$version                                                                                                                                          |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"${CTX:cutoffPath}","upstreams":[{"url":"http://testbench:3026"}]},{"name":"alive","basePath":"/no5xx-cutoff-alive","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"POST","path":"/chat/completions","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"perTryTimeout":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful

    # The primary hangs five seconds against a one-second per-try bound. No response ever
    # reaches the policy, so nothing here is graded; the client is still served, and still
    # never sees a 5xx.
    When I send a "POST" request to "${CTX:cutoffContext}/${CTX:cutoffVersion}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/no5xx-cutoff-alive"

    When I delete the API "${CTX:gradedName}"
    Then the response should be successful
    When I delete the API "${CTX:cutoffName}"
    Then the response should be successful
