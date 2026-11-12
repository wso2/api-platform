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

@model-failover-abandoned-caller
Feature: Traversal stops when the caller goes away
  A chain is walked on behalf of someone waiting. Once they have gone there is nobody to
  serve, and every further attempt is an inference call billed for an answer no one will
  read. So the traversal has to stop rather than run to completion on its own.

  With the caller gone there is no response to assert on, so the fallback is pointed at the
  testbench capture service: if the attempt is made, the capture records it under that
  operation's own path; if the traversal stopped, no such record exists. Both halves are
  read from one capture state, so the positive control sits beside the negative claim rather
  than in an earlier run whose timing could blur it.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A caller that gives up mid-chain stops the traversal
    # The primary must FAIL, and slowly. Every step of the script delays well past the
    # per-attempt bound, which is what turns a slow upstream into a failure worth failing
    # over - without it a connected caller would simply get its answer and the fallback would
    # be untouched for an entirely different reason. A uniform script also means the
    # readiness poll cannot change what a later request meets.
    Given I generate a unique resource name from "giveup-script" and store it as "scriptId"
    When I send a "PUT" request to the "fault-injection" service at "/test/scripts/${CTX:scriptId}" with body:
      """
      {"forwardPort": 3000, "steps": [
        {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000},
        {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000},
        {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000},
        {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000}, {"delayMs": 9000}
      ]}
      """
    Then the response should be successful
    And I resolve the "fault-injection" service path at "/${CTX:scriptId}" and store it as "slowPath"
    And I resolve the "capture" service path at "/giveup-sink" and store it as "capturePath"

    Given I generate a unique resource name from "giveup" and store it as "apiName"
    And I generate a unique API version from "giveup" and store it as "apiVersion"
    And I generate a unique API context from "/giveup" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:apiName}                                                                                                                                                         |
      | spec.displayName         | ${CTX:apiName}                                                                                                                                                         |
      | spec.version             | ${CTX:apiVersion}                                                                                                                                                      |
      | spec.context             | ${CTX:apiContext}/$version                                                                                                                                             |
      | spec.upstreamDefinitions | [{"name":"primary","basePath":"${CTX:slowPath}","upstreams":[{"url":"http://testbench:3026"}]},{"name":"sink","basePath":"${CTX:capturePath}","upstreams":[{"url":"http://testbench:3010"}]}] |
      | spec.upstream.main.ref   | primary                                                                                                                                                                |
      | spec.operations          | [{"method":"GET","path":"/control","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"perTryTimeout":3,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"sink"}]}]}}]},{"method":"GET","path":"/subject","policies":[{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"perTryTimeout":3,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"sink"}]}]}}]}] |
    Then the response should be successful

    # CONTROL - identical configuration and timing, caller stays. The primary runs past its
    # per-attempt budget, so the chain advances to the sink.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/control" until status 200
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # Clear the record AFTER the control has proved the chain reaches the sink, so what
    # follows is read against a capture that only this scenario's next request can fill.
    When I send a "POST" request to the "capture" service at "/test/reset"
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/control" until status 200
    Then the response status code should be 200
    # The capture answers 200 with what it recorded, and 204 when it recorded nothing for
    # that path, so the two halves below are the same question asked twice.
    When I send a "GET" request to the "capture" service at "/test/captured?path=/giveup-sink/control"
    Then the response status code should be 200

    # SUBJECT - same shape, caller gives up well inside the first attempt's budget, so it
    # leaves while the chain is still being walked.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/subject" and give up after 1500ms
    Then the request should have been abandoned

    # Long enough that a fallback attempt, had one been made, would have run its course and
    # been recorded - otherwise the absence below would only mean "not yet". The primary's
    # own attempt is still running at this point, so this also outlasts it.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/control" until status 200
    And I send a "GET" request to the "capture" service at "/test/captured?path=/giveup-sink/subject"
    # 204: nothing was ever recorded for the abandoned request's own operation.
    Then the response status code should be 204

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
