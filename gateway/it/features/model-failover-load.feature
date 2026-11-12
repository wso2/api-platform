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

@model-failover @load
Feature: Failover under sustained load
  LOCAL ONLY — excluded from every ordinary run and from CI by the `~@load`
  default in suite_test.go. Run with `make test-load`.

  These check two properties a functional scenario cannot: that the attempt
  budget holds over a large number of requests, and what failover costs in
  latency. Both only become visible at volume or with a clock.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # SC-008 — the chain is walked at most once per request, forever
  Scenario: Ten thousand requests against a continuously failing target never loop
    # suspendDuration is 0 on purpose. With suspension on, the dead target would
    # be dropped after the first failure and every later request would see one
    # attempt — the invariant would hold trivially, proving nothing. Keeping the
    # target in rotation means all 10,000 requests actually walk the chain.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-load-v1.0
      spec:
        displayName: MF Load
        version: v1.0
        context: /mf-load/$version
        upstreamDefinitions:
          - name: primary
            basePath: /load-dead
            upstreams:
              - url: http://sample-backend:9998
          - name: alive
            basePath: /load-alive
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: primary
        operations:
          - method: POST
            path: /chat/completions
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response should be successful
    And I wait for 6 seconds

    When I set header "Content-Type" to "application/json"
    And I drive 10000 POST requests to "http://localhost:8080/mf-load/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # Every one was rescued — the dead target never took a request down with it.
    Then every response status should have been 200
    # The chain has two targets, so two attempts is the ceiling. A third would mean
    # a target was retried or the chain wrapped around.
    And no response should have reported more than 2 attempts

    When I delete the API "mf-load-v1.0"
    Then the response status code should be 200

  # SC-003 — what failover costs, and that the cost stops once the target is out
  Scenario: The failover penalty is paid once, then disappears with suspension
    # The dead target here TIMES OUT rather than refusing, because a refused
    # connection costs almost nothing and would make the latency claim
    # unmeasurable. perTryTimeout bounds that wait at 2s, which is the penalty
    # a rescued request is expected to pay while the target is still in rotation.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-load-latency-v1.0
      spec:
        displayName: MF Load Latency
        version: v1.0
        context: /mf-load-latency/$version
        upstreamDefinitions:
          - name: primary
            upstreams:
              - url: http://echo-backend:80
          - name: alive
            basePath: /latency-alive
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: primary
        operations:
          - method: POST
            path: /delay/8
            policies:
              - name: model-failover
                version: v0
                params:
                  perTryTimeout: 2
                  suspendAfterFailures: 1
                  suspendDuration: 120
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response should be successful
    And I wait for 6 seconds

    # While the target is still in rotation the request waits out its budget,
    # then the fallback answers.
    When I set header "Content-Type" to "application/json"
    And I record the current time as "first_request"
    And I send a POST request to "http://localhost:8080/mf-load-latency/v1.0/delay/8" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/latency-alive"
    And the request should have taken at least "2" seconds since "first_request"
    # The penalty is the per-attempt budget plus the fallback's own time, not the
    # dead target's full 8s sleep — which is the point of bounding the attempt.
    And the request should have taken at most "5" seconds since "first_request"

    # That one failure suspends the target, so the next request skips it entirely.
    When I record the current time as "second_request"
    And I send a POST request to "http://localhost:8080/mf-load-latency/v1.0/delay/8" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/latency-alive"
    # No dead attempt to wait out: the penalty is gone, not merely smaller.
    And the request should have taken at most "1" seconds since "second_request"

    When I delete the API "mf-load-latency-v1.0"
    Then the response status code should be 200
