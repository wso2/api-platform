@ratelimit
Feature: Rate Limiting
  As an API developer
  I want to limit the rate of requests to my API
  So that I can protect my upstream services from excessive traffic

  # Migrated with every assertion intact. Mechanical differences from the legacy suite:
  #
  # Upstream http://sample-backend:9080/api/v1 becomes http://testbench:3000/api/v1. The
  # echo-backend cost-extraction scenarios read the request body back out of the RESPONSE via
  # $.json.<field> and query args via $.args; that parsed-body shape is what the echo (go-httpbin)
  # service on testbench:3002 serves — the plain backend on :3000 reflects the body only under a
  # raw "body" string, so http://echo-backend:80 maps to http://testbench:3002, NOT :3000. (See
  # the report note: this deviates from the stated echo-backend->:3000 rule for a documented reason.)
  #
  # Request URLs are reduced from http://localhost:8080/... to their path. The per-scenario
  # `Given I authenticate using basic auth as "admin"` is gone — one lives in the Background.
  #
  # There is no `I wait for N seconds` step; the two post-update settle waits use
  # `I wait for policy snapshot sync`.
  #
  # COUNTING: a readiness poll counts one request against the polled path's limit. Scenarios with
  # a single rate-limited operation and nothing unlimited got a plain /probe operation added, and
  # readiness targets it so the limited quota starts full. Where that reshaped the request counts,
  # the batch now sends exactly the limit and one extra — the 200/429 assertions are unchanged.
  # Every other scenario already has an unlimited /health (or a policy-free operation) to poll.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce rate limit on API resource
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-test-api
      spec:
        displayName: RateLimit Test API
        version: v1.0
        context: /ratelimit/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /limited
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 10
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit/v1.0/probe" until status 200

    # Readiness runs against the unlimited /probe op, so /limited starts with its full quota of 10.
    # Send 10 requests - all should succeed
    When I send 10 "GET" requests to "/ratelimit/v1.0/limited"
    Then the response status code should be 200

    # The 11th request exhausts the quota
    When I send a "GET" request to "/ratelimit/v1.0/limited"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Multi-quota rate limit headers in IETF format
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-multi-quota-headers-api
      spec:
        displayName: RateLimit Multi-Quota Headers API
        version: v1.0
        context: /ratelimit-multi-quota-headers/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: burst
                      limits:
                        - limit: 10
                          duration: "1m"
                    - name: daily
                      limits:
                        - limit: 100
                          duration: "24h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-multi-quota-headers/v1.0/health" until status 200

    When I send a "GET" request to "/ratelimit-multi-quota-headers/v1.0/resource"
    Then the response status code should be 200
    # Check X-RateLimit-* headers (legacy format - most restrictive quota)
    And the response header "X-RateLimit-Limit" should exist
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist
    # Check IETF RateLimit headers (multi-quota format)
    And the response header "RateLimit-Policy" should exist
    And the response header "RateLimit" should exist
    # Verify IETF headers contain both quota names
    And the response header "RateLimit-Policy" should contain "burst"
    And the response header "RateLimit-Policy" should contain "daily"
    And the response header "RateLimit" should contain "burst"
    And the response header "RateLimit" should contain "daily"

  Scenario: 429 response includes IETF RateLimit headers for violated quota
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-multi-quota-429-api
      spec:
        displayName: RateLimit Multi-Quota 429 API
        version: v1.0
        context: /ratelimit-multi-quota-429/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 4
                          duration: "1h"
                    - name: daily
                      limits:
                        - limit: 100
                          duration: "24h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-multi-quota-429/v1.0/health" until status 200

    # Send requests to exhaust the request-limit quota (limit=4, 1h fixed window)
    When I send 4 "GET" requests to "/ratelimit-multi-quota-429/v1.0/resource"
    Then the response status code should be 200

    # This request should be rate limited by the request-limit quota
    When I send a "GET" request to "/ratelimit-multi-quota-429/v1.0/resource"
    Then the response status code should be 429
    # Check that violated quota is identified in legacy header
    And the response header "X-RateLimit-Quota" should be "request-limit"
    # Check IETF headers are present in 429 response with violated quota info
    And the response header "RateLimit-Policy" should exist
    And the response header "RateLimit-Policy" should contain "request-limit"
    And the response header "RateLimit" should exist
    And the response header "RateLimit" should contain "request-limit"
    And the response header "Retry-After" should exist

  Scenario: Rate limit headers are returned
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-headers-api
      spec:
        displayName: RateLimit Headers API
        version: v1.0
        context: /ratelimit-headers/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /check
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 100
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-headers/v1.0/check" until status 200

    When I send a "GET" request to "/ratelimit-headers/v1.0/check"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

  Scenario: Custom rate limit error response
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-custom-api
      spec:
        displayName: RateLimit Custom API
        version: v1.0
        context: /ratelimit-custom/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /custom
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 5
                          duration: "1h"
                  onRateLimitExceeded:
                    statusCode: 429
                    body: '{"error": "Too Many Requests", "code": 429001}'
                    bodyFormat: json
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-custom/v1.0/probe" until status 200

    # Readiness runs against the unlimited /probe op, so /custom starts with its full quota of 5.
    When I send 5 "GET" requests to "/ratelimit-custom/v1.0/custom"
    Then the response status code should be 200

    # Next request exceeds the limit and returns the configured custom JSON error
    When I send a "GET" request to "/ratelimit-custom/v1.0/custom"
    Then the response status code should be 429
    And the response should be valid JSON
    And the JSON response field "error" should be "Too Many Requests"
    And the JSON response field "code" should be 429001

  Scenario: Basic rate limiting without cost extraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-basic-api
      spec:
        displayName: RateLimit Basic API
        version: v1.0
        context: /ratelimit-basic/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 4
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-basic/v1.0/probe" until status 200

    # Readiness runs against the unlimited /probe op, so /resource starts with its full quota of 4.
    # Send 4 requests - all should succeed (each request costs 1 token by default)
    When I send 4 "GET" requests to "/ratelimit-basic/v1.0/resource"
    Then the response status code should be 200

    # Next request should be rate limited
    When I send a "GET" request to "/ratelimit-basic/v1.0/resource"
    Then the response status code should be 429

  Scenario: Cost extraction from response body using echo backend
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-cost-extraction-api
      spec:
        displayName: RateLimit Cost Extraction API
        version: v1.0
        context: /ratelimit-cost-extraction/$version
        upstream:
          main:
            url: http://testbench:3002
        operations:
          - method: GET
            path: /anything
          - method: POST
            path: /anything
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: token-quota
                      limits:
                        - limit: 100
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_body
                            jsonPath: "$.json.custom_cost"
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-cost-extraction/v1.0/anything" until status 200

    # Send a POST request with custom_cost=50 in the body
    # The echo backend echoes back the JSON, and cost extraction reads $.json.custom_cost
    When I send a "POST" request to "/ratelimit-cost-extraction/v1.0/anything" with body:
      """
      {"custom_cost": 50}
      """
    Then the response status code should be 200
    # After first request: 100 - 50 = 50 remaining
    And the response header "X-RateLimit-Remaining" should be "50"

    # Send another request with custom_cost=50
    When I send a "POST" request to "/ratelimit-cost-extraction/v1.0/anything" with body:
      """
      {"custom_cost": 50}
      """
    Then the response status code should be 200
    # After second request: 50 - 50 = 0 remaining
    And the response header "X-RateLimit-Remaining" should be "0"

    # Third request should be rate limited since quota is exhausted
    When I send a "POST" request to "/ratelimit-cost-extraction/v1.0/anything" with body:
      """
      {"custom_cost": 10}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Response cost overage clamps quota to zero
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-response-clamp-api
      spec:
        displayName: RateLimit Response Clamp API
        version: v1.0
        context: /ratelimit-response-clamp/$version
        upstream:
          main:
            url: http://testbench:3002
        operations:
          - method: GET
            path: /anything
          - method: POST
            path: /anything
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: response-token-quota
                      limits:
                        - limit: 20
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_body
                            jsonPath: "$.json.custom_cost"
                        default: 0
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-response-clamp/v1.0/anything" until status 200

    # custom_cost=50 exceeds remaining=20 on first request.
    # Expected clamp behavior: consume remaining quota, return 200, remaining becomes 0.
    When I send a "POST" request to "/ratelimit-response-clamp/v1.0/anything" with body:
      """
      {"custom_cost": 50}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Next request must be blocked because previous overage clamped quota to zero.
    When I send a "POST" request to "/ratelimit-response-clamp/v1.0/anything" with body:
      """
      {"custom_cost": 1}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: API-level rate limiting with apiname key extraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-api-level-api
      spec:
        displayName: RateLimit API Level Test
        version: v1.0
        context: /ratelimit-api-level/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /route0
          - method: GET
            path: /route1
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: api-quota
                      limits:
                        - limit: 10
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
          - method: GET
            path: /route2
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: api-quota
                      limits:
                        - limit: 10
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-api-level/v1.0/route0" until status 200

    # Send 5 requests to route1 - uses API-level bucket
    When I send 5 "GET" requests to "/ratelimit-api-level/v1.0/route1"
    Then the response status code should be 200

    # Send 4 requests to route2 - shares the same API-level bucket
    # Total: 5 + 4 = 9 requests (still within limit 10)
    When I send 4 "GET" requests to "/ratelimit-api-level/v1.0/route2"
    Then the response status code should be 200

    # The 10th request (either route) should work
    When I send a "GET" request to "/ratelimit-api-level/v1.0/route1"
    Then the response status code should be 200

    # The 11th request should be rate limited - quota exhausted across both routes
    When I send a "GET" request to "/ratelimit-api-level/v1.0/route2"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    When I send a "GET" request to "/ratelimit-api-level/v1.0/route1"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Updating API does not reset rate limit state
    # Deploy initial API with rate limit
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-update-test-api
      spec:
        displayName: RateLimit Update Test API
        version: v1.0
        context: /ratelimit-update-test/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /test
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 5
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-update-test/v1.0/test" until status 200

    # Exhaust the rate limit quota (5 requests)
    When I send 5 "GET" requests to "/ratelimit-update-test/v1.0/resource"
    Then the response status code should be 200

    # Verify quota is exhausted
    When I send a "GET" request to "/ratelimit-update-test/v1.0/resource"
    Then the response status code should be 429

    # Update the API by adding a new unrelated route (PUT /handle)
    When I update the API "ratelimit-update-test-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-update-test-api
      spec:
        displayName: RateLimit Update Test API
        version: v1.0
        context: /ratelimit-update-test/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /test
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 5
                          duration: "1h"
          - method: PUT
            path: /handle
      """
    Then the response status code should be 200
    And I wait for policy snapshot sync

    # Verify rate limit state was preserved - should still be 429
    When I send a "GET" request to "/ratelimit-update-test/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Multi-dimensional rate limiting with quotas
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-quotas-api
      spec:
        displayName: RateLimit Quotas API
        version: v1.0
        context: /ratelimit-quotas/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /multi1
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-route
                      limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: routename
                    - name: per-api
                      limits:
                        - limit: 8
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
          - method: GET
            path: /multi2
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-route
                      limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: routename
                    - name: per-api
                      limits:
                        - limit: 8
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-quotas/v1.0/health" until status 200

    # Send 5 requests to multi1 - hits per-route limit (5/5)
    # Per-api usage: 5/8
    When I send 5 "GET" requests to "/ratelimit-quotas/v1.0/multi1"
    Then the response status code should be 200

    # 6th request to multi1 blocked by per-route quota
    When I send a "GET" request to "/ratelimit-quotas/v1.0/multi1"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # multi2 has its own per-route quota (0/5), so requests should work
    # But per-api is shared: 5 + 3 = 8/8 (reaching per-api limit)
    When I send 3 "GET" requests to "/ratelimit-quotas/v1.0/multi2"
    Then the response status code should be 200

    # This request proves per-api works: multi2's per-route is only 3/5,
    # but per-api is exhausted at 8/8, so it should be blocked
    When I send a "GET" request to "/ratelimit-quotas/v1.0/multi2"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Per-quota cost extraction with multiplier
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-quota-cost-api
      spec:
        displayName: RateLimit Quota Cost API
        version: v1.0
        context: /ratelimit-quota-cost/$version
        upstream:
          main:
            url: http://testbench:3002
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /anything
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: token-limit
                      limits:
                        - limit: 100
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_body
                            jsonPath: "$.json.tokens"
                            multiplier: 2.0
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-quota-cost/v1.0/get" until status 200

    # Send request with tokens=25, with multiplier 2.0 -> cost=50
    When I send a "POST" request to "/ratelimit-quota-cost/v1.0/anything" with body:
      """
      {"tokens": 25}
      """
    Then the response status code should be 200
    # After first request: 100 - 50 = 50 remaining
    And the response header "X-RateLimit-Remaining" should be "50"

    # Send another request with tokens=25 -> cost=50
    When I send a "POST" request to "/ratelimit-quota-cost/v1.0/anything" with body:
      """
      {"tokens": 25}
      """
    Then the response status code should be 200
    # After second request: 50 - 50 = 0 remaining
    And the response header "X-RateLimit-Remaining" should be "0"

    # Third request should be rate limited
    When I send a "POST" request to "/ratelimit-quota-cost/v1.0/anything" with body:
      """
      {"tokens": 10}
      """
    Then the response status code should be 429

  Scenario: Header-based key extraction for per-user rate limiting
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-peruser-api
      spec:
        displayName: RateLimit Per-User API
        version: v1.0
        context: /ratelimit-peruser/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /user
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-user-limit
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: header
                          key: X-User-ID
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-peruser/v1.0/user" until status 200

    # User-A sends 3 requests - should succeed
    When I set header "X-User-ID" to "user-A"
    And I send 3 "GET" requests to "/ratelimit-peruser/v1.0/user"
    Then the response status code should be 200

    # User-A's 4th request should be rate limited
    When I set header "X-User-ID" to "user-A"
    And I send a "GET" request to "/ratelimit-peruser/v1.0/user"
    Then the response status code should be 429

    # User-B should still have full quota (separate bucket)
    When I set header "X-User-ID" to "user-B"
    And I send 3 "GET" requests to "/ratelimit-peruser/v1.0/user"
    Then the response status code should be 200

    # User-B's 4th request should be rate limited
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-peruser/v1.0/user"
    Then the response status code should be 429

  Scenario: Multiple limits per quota - enforces most restrictive limit
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-multilimits-api
      spec:
        displayName: RateLimit Multiple Limits API
        version: v1.0
        context: /ratelimit-multilimits/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: "request-quota"
                      limits:
                        - limit: 10
                          duration: "1h"
                        - limit: 8
                          duration: "24h"
                      keyExtraction:
                        - type: routename
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-multilimits/v1.0/health" until status 200

    # Phase 1: Test that 24h limit is enforced (more restrictive than 1h)
    # Limits: 10/1h and 8/24h - the 24h limit should be hit first

    # Send 8 requests - should succeed (8/8 for 24h, 8/10 for 1h)
    When I send 8 "GET" requests to "/ratelimit-multilimits/v1.0/resource"
    Then the response status code should be 200

    # 9th request should be blocked by 24h limit (8/8 exhausted)
    # Note: 1h limit still has 2 tokens left (8/10), proving 24h limit is enforced
    When I send a "GET" request to "/ratelimit-multilimits/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Phase 2: Update API to make 1h limit more restrictive
    # New limits: 12/1h and 10000/24h - the 1h limit should be hit first
    When I update the API "ratelimit-multilimits-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-multilimits-api
      spec:
        displayName: RateLimit Multiple Limits API
        version: v1.0
        context: /ratelimit-multilimits/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: "request-quota"
                      limits:
                        - limit: 12
                          duration: "1h"
                        - limit: 10000
                          duration: "24h"
                      keyExtraction:
                        - type: routename
      """
    Then the response status code should be 200
    # The settle, and the assertion that the counter reset. 11 remaining is the new 12/1h bucket
    # with exactly this request counted, so reaching it proves the engine swapped config AND
    # started a fresh bucket. A status poll cannot see either — 200 arrives before and after the
    # swap. Polls taken before it are charged to the old bucket, which phase 1 left exhausted, so
    # they cost the new one nothing. A counter that carried over never reaches 11 and fails here
    # naming what it saw, instead of throttling mid-count for reasons that look like a quota bug.
    And I send a "GET" request to "/ratelimit-multilimits/v1.0/resource" until the rate limit reports 11 remaining

    # 11 more exhaust the new 12/1h budget; the gate above spent the first.
    When I send 11 "GET" requests to "/ratelimit-multilimits/v1.0/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # 13th request should be blocked by 1h limit (12/12 exhausted)
    # Note: 24h limit still has plenty of tokens (12/10000), proving 1h limit is enforced
    When I send a "GET" request to "/ratelimit-multilimits/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Cost extraction from request body
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-request-body-cost-api
      spec:
        displayName: RateLimit Request Body Cost API
        version: v1.0
        context: /ratelimit-request-body-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: token-quota
                      limits:
                        - limit: 100
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_body
                            jsonPath: "$.tokens"
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-request-body-cost/v1.0/health" until status 200

    # Send a POST request with tokens=40 in the body
    # Cost extraction will read $.tokens from request body
    When I send a "POST" request to "/ratelimit-request-body-cost/v1.0/resource" with body:
      """
      {"tokens": 40}
      """
    Then the response status code should be 200
    # After first request: 100 - 40 = 60 remaining
    And the response header "X-RateLimit-Remaining" should be "60"

    # Send another request with tokens=40
    When I send a "POST" request to "/ratelimit-request-body-cost/v1.0/resource" with body:
      """
      {"tokens": 40}
      """
    Then the response status code should be 200
    # After second request: 60 - 40 = 20 remaining
    And the response header "X-RateLimit-Remaining" should be "20"

    # Send a request with tokens=30 - should be rate limited (need 30, only 20 remaining)
    When I send a "POST" request to "/ratelimit-request-body-cost/v1.0/resource" with body:
      """
      {"tokens": 30}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Multiple quotas with prompt and completion token cost extraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-prompt-tokens-api
      spec:
        displayName: RateLimit Prompt Tokens API
        version: v1.0
        context: /ratelimit-prompt-tokens/$version
        upstream:
          main:
            url: http://testbench:3002
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /anything
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: "prompt-tokens"
                      limits:
                        - limit: 500
                          duration: "1h"
                      keyExtraction:
                        - type: header
                          key: X-User-ID
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_body
                            jsonPath: "$.json.usage.prompt_tokens"
                            multiplier: 1.0
                        default: 0
                    - name: "completion-tokens"
                      limits:
                        - limit: 200
                          duration: "1h"
                      keyExtraction:
                        - type: header
                          key: X-User-ID
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_body
                            jsonPath: "$.json.usage.completion_tokens"
                            multiplier: 1.0
                        default: 0
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-prompt-tokens/v1.0/get" until status 200

    # Test completion-tokens quota with user-A
    # Send request with low prompt tokens (50) but high completion tokens (100)
    When I set header "X-User-ID" to "user-A"
    And I send a "POST" request to "/ratelimit-prompt-tokens/v1.0/anything" with body:
      """
      {"usage": {"prompt_tokens": 50, "completion_tokens": 100}}
      """
    Then the response status code should be 200
    # prompt-tokens: 500 - 50 = 450 remaining
    # completion-tokens: 200 - 100 = 100 remaining

    # Send another request with same pattern
    When I set header "X-User-ID" to "user-A"
    And I send a "POST" request to "/ratelimit-prompt-tokens/v1.0/anything" with body:
      """
      {"usage": {"prompt_tokens": 50, "completion_tokens": 100}}
      """
    Then the response status code should be 200
    # prompt-tokens: 450 - 50 = 400 remaining
    # completion-tokens: 100 - 100 = 0 remaining (exhausted!)

    # This request should be blocked by completion-tokens quota
    # Note: prompt-tokens still has 400 remaining, proving completion-tokens is enforced
    When I set header "X-User-ID" to "user-A"
    And I send a "POST" request to "/ratelimit-prompt-tokens/v1.0/anything" with body:
      """
      {"usage": {"prompt_tokens": 50, "completion_tokens": 50}}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Test prompt-tokens quota with user-B
    # Send request with high prompt tokens (250) but low completion tokens (10)
    When I set header "X-User-ID" to "user-B"
    And I send a "POST" request to "/ratelimit-prompt-tokens/v1.0/anything" with body:
      """
      {"usage": {"prompt_tokens": 250, "completion_tokens": 10}}
      """
    Then the response status code should be 200
    # prompt-tokens: 500 - 250 = 250 remaining
    # completion-tokens: 200 - 10 = 190 remaining

    # Send another request with same pattern
    When I set header "X-User-ID" to "user-B"
    And I send a "POST" request to "/ratelimit-prompt-tokens/v1.0/anything" with body:
      """
      {"usage": {"prompt_tokens": 250, "completion_tokens": 10}}
      """
    Then the response status code should be 200
    # prompt-tokens: 250 - 250 = 0 remaining (exhausted!)
    # completion-tokens: 190 - 10 = 180 remaining

    # This request should be blocked by prompt-tokens quota
    # Note: completion-tokens still has 180 remaining, proving prompt-tokens is enforced
    When I set header "X-User-ID" to "user-B"
    And I send a "POST" request to "/ratelimit-prompt-tokens/v1.0/anything" with body:
      """
      {"usage": {"prompt_tokens": 50, "completion_tokens": 10}}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: IP-based rate limiting using X-Forwarded-For header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-ip-based-api
      spec:
        displayName: RateLimit IP Based API
        version: v1.0
        context: /ratelimit-ip-based/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-ip-limit
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: ip
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-ip-based/v1.0/resource" until status 200

    # IP 192.168.1.100 sends 3 requests - should succeed
    When I set header "X-Forwarded-For" to "192.168.1.100"
    And I send 3 "GET" requests to "/ratelimit-ip-based/v1.0/resource"
    Then the response status code should be 200

    # IP 192.168.1.100's 4th request should be rate limited
    When I set header "X-Forwarded-For" to "192.168.1.100"
    And I send a "GET" request to "/ratelimit-ip-based/v1.0/resource"
    Then the response status code should be 429

    # Different IP 192.168.1.200 should have separate quota
    When I set header "X-Forwarded-For" to "192.168.1.200"
    And I send 3 "GET" requests to "/ratelimit-ip-based/v1.0/resource"
    Then the response status code should be 200

    # IP 192.168.1.200's 4th request should be rate limited
    When I set header "X-Forwarded-For" to "192.168.1.200"
    And I send a "GET" request to "/ratelimit-ip-based/v1.0/resource"
    Then the response status code should be 429

  Scenario: Cost extraction from request header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-request-header-cost-api
      spec:
        displayName: RateLimit Request Header Cost API
        version: v1.0
        context: /ratelimit-request-header-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: token-quota
                      limits:
                        - limit: 100
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_header
                            key: X-Token-Cost
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-request-header-cost/v1.0/health" until status 200

    # Send a request with X-Token-Cost=40 header
    When I set header "X-Token-Cost" to "40"
    And I send a "POST" request to "/ratelimit-request-header-cost/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    # After first request: 100 - 40 = 60 remaining
    And the response header "X-RateLimit-Remaining" should be "60"

    # Send another request with X-Token-Cost=40
    When I set header "X-Token-Cost" to "40"
    And I send a "POST" request to "/ratelimit-request-header-cost/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    # After second request: 60 - 40 = 20 remaining
    And the response header "X-RateLimit-Remaining" should be "20"

    # Send a request with X-Token-Cost=30 - should be rate limited (need 30, only 20 remaining)
    When I set header "X-Token-Cost" to "30"
    And I send a "POST" request to "/ratelimit-request-header-cost/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Composite key extraction with multiple components
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-composite-key-api
      spec:
        displayName: RateLimit Composite Key API
        version: v1.0
        context: /ratelimit-composite-key/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-user-per-api
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
                        - type: header
                          key: X-User-ID
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-composite-key/v1.0/resource" until status 200

    # User-A on this API sends 3 requests - should succeed
    When I set header "X-User-ID" to "user-A"
    And I send 3 "GET" requests to "/ratelimit-composite-key/v1.0/resource"
    Then the response status code should be 200

    # User-A's 4th request should be rate limited
    When I set header "X-User-ID" to "user-A"
    And I send a "GET" request to "/ratelimit-composite-key/v1.0/resource"
    Then the response status code should be 429

    # User-B on same API should have separate quota (different composite key)
    When I set header "X-User-ID" to "user-B"
    And I send 3 "GET" requests to "/ratelimit-composite-key/v1.0/resource"
    Then the response status code should be 200

    # User-B's 4th request should be rate limited
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-composite-key/v1.0/resource"
    Then the response status code should be 429

  Scenario: Default cost fallback when extraction fails
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-default-cost-api
      spec:
        displayName: RateLimit Default Cost API
        version: v1.0
        context: /ratelimit-default-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: token-quota
                      limits:
                        - limit: 10
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_body
                            jsonPath: "$.nonexistent_field"
                        default: 5
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-default-cost/v1.0/health" until status 200

    # Send a request with body that doesn't contain the expected field
    # Cost extraction will fail, so default cost (5) should be used
    When I send a "POST" request to "/ratelimit-default-cost/v1.0/resource" with body:
      """
      {"some_other_field": 100}
      """
    Then the response status code should be 200
    # After first request: 10 - 5 = 5 remaining (default cost used)
    And the response header "X-RateLimit-Remaining" should be "5"

    # Send another request - again default cost (5) should be used
    When I send a "POST" request to "/ratelimit-default-cost/v1.0/resource" with body:
      """
      {"another_field": 200}
      """
    Then the response status code should be 200
    # After second request: 5 - 5 = 0 remaining
    And the response header "X-RateLimit-Remaining" should be "0"

    # Third request should be rate limited
    When I send a "POST" request to "/ratelimit-default-cost/v1.0/resource" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Different APIs with apiname key extraction have isolated quotas
    # This test verifies that two different APIs using apiname key extraction
    # each get their own separate rate limit bucket (not shared).
    # This guards against the bug where empty/missing apiName causes cache key collisions.

    # Deploy first API with apiname-based rate limiting
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-apiname-isolation-api-a
      spec:
        displayName: RateLimit ApiName Isolation API A
        version: v1.0
        context: /ratelimit-apiname-isolation-a/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: api-quota
                      limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-apiname-isolation-a/v1.0/health" until status 200

    # Deploy second API with the same rate limit configuration but different API name
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-apiname-isolation-api-b
      spec:
        displayName: RateLimit ApiName Isolation API B
        version: v1.0
        context: /ratelimit-apiname-isolation-b/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: api-quota
                      limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-apiname-isolation-b/v1.0/health" until status 200

    # Exhaust API-A's quota (5 requests)
    When I send 5 "GET" requests to "/ratelimit-apiname-isolation-a/v1.0/resource"
    Then the response status code should be 200

    # Verify API-A is rate limited
    When I send a "GET" request to "/ratelimit-apiname-isolation-a/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # API-B should still have its full quota (proves isolation)
    # If there was a cache key collision, API-B would also be rate limited
    When I send 5 "GET" requests to "/ratelimit-apiname-isolation-b/v1.0/resource"
    Then the response status code should be 200

    # Verify API-B is now rate limited (after exhausting its own quota)
    When I send a "GET" request to "/ratelimit-apiname-isolation-b/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Double-check API-A is still rate limited (not affected by API-B usage)
    When I send a "GET" request to "/ratelimit-apiname-isolation-a/v1.0/resource"
    Then the response status code should be 429

  Scenario: Resource grouping using constant key extraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-constant-key-api
      spec:
        displayName: RateLimit Constant Key API
        version: v1.0
        context: /ratelimit-constant/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /group-a-1
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
                        - type: constant
                          key: "group-A"
          - method: GET
            path: /group-a-2
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
                        - type: constant
                          key: "group-A"
          - method: GET
            path: /group-b-1
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - limits:
                        - limit: 5
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
                        - type: constant
                          key: "group-B"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-constant/v1.0/health" until status 200

    # Group A: Send 3 requests to /group-a-1
    When I send 3 "GET" requests to "/ratelimit-constant/v1.0/group-a-1"
    Then the response status code should be 200

    # Group A: Send 2 requests to /group-a-2 (should share bucket with group-a-1)
    # Total Group A usage: 3 + 2 = 5 (Limit reached)
    When I send 2 "GET" requests to "/ratelimit-constant/v1.0/group-a-2"
    Then the response status code should be 200

    # Group A: Verify Limit Exceeded on /group-a-1
    When I send a "GET" request to "/ratelimit-constant/v1.0/group-a-1"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Group A: Verify Limit Exceeded on /group-a-2
    When I send a "GET" request to "/ratelimit-constant/v1.0/group-a-2"
    Then the response status code should be 429

    # Group B: Should be independent (0 usage)
    When I send 5 "GET" requests to "/ratelimit-constant/v1.0/group-b-1"
    Then the response status code should be 200

    # Group B: 6th request should fail
    When I send a "GET" request to "/ratelimit-constant/v1.0/group-b-1"
    Then the response status code should be 429

  Scenario: CEL expression based key extraction for per-user rate limiting
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-cel-key-api
      spec:
        displayName: RateLimit CEL Key API
        version: v1.0
        context: /ratelimit-cel-key/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-user-cel
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: cel
                          expression: 'request.Headers["x-user-id"][0]'
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-cel-key/v1.0/resource" until status 200

    # User-A sends 3 requests using CEL key extraction - should succeed
    When I set header "X-User-ID" to "cel-user-A"
    And I send 3 "GET" requests to "/ratelimit-cel-key/v1.0/resource"
    Then the response status code should be 200

    # User-A's 4th request should be rate limited
    When I set header "X-User-ID" to "cel-user-A"
    And I send a "GET" request to "/ratelimit-cel-key/v1.0/resource"
    Then the response status code should be 429

    # User-B should have separate quota (different CEL-extracted key)
    When I set header "X-User-ID" to "cel-user-B"
    And I send 3 "GET" requests to "/ratelimit-cel-key/v1.0/resource"
    Then the response status code should be 200

    # User-B's 4th request should be rate limited
    When I set header "X-User-ID" to "cel-user-B"
    And I send a "GET" request to "/ratelimit-cel-key/v1.0/resource"
    Then the response status code should be 429

  Scenario: CEL expression based composite key extraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-cel-composite-key-api
      spec:
        displayName: RateLimit CEL Composite Key API
        version: v1.0
        context: /ratelimit-cel-composite-key/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: per-user-per-api-cel
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: cel
                          expression: 'api.Name + ":" + request.Headers["x-user-id"][0]'
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-cel-composite-key/v1.0/resource" until status 200

    # User-A sends 3 requests - composite key includes API name + user ID
    When I set header "X-User-ID" to "composite-user-A"
    And I send 3 "GET" requests to "/ratelimit-cel-composite-key/v1.0/resource"
    Then the response status code should be 200

    # User-A's 4th request should be rate limited
    When I set header "X-User-ID" to "composite-user-A"
    And I send a "GET" request to "/ratelimit-cel-composite-key/v1.0/resource"
    Then the response status code should be 429

    # User-B should have separate composite key and full quota
    When I set header "X-User-ID" to "composite-user-B"
    And I send 3 "GET" requests to "/ratelimit-cel-composite-key/v1.0/resource"
    Then the response status code should be 200

  Scenario: CEL expression based cost extraction from request header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-cel-cost-api
      spec:
        displayName: RateLimit CEL Cost API
        version: v1.0
        context: /ratelimit-cel-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: token-quota-cel
                      limits:
                        - limit: 100
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_cel
                            expression: 'int(request.Headers["x-token-cost"][0])'
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-cel-cost/v1.0/health" until status 200

    # Send a request with X-Token-Cost=40 header, CEL extracts as integer
    When I set header "X-Token-Cost" to "40"
    And I send a "POST" request to "/ratelimit-cel-cost/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    # After first request: 100 - 40 = 60 remaining
    And the response header "X-RateLimit-Remaining" should be "60"

    # Send another request with X-Token-Cost=40
    When I set header "X-Token-Cost" to "40"
    And I send a "POST" request to "/ratelimit-cel-cost/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    # After second request: 60 - 40 = 20 remaining
    And the response header "X-RateLimit-Remaining" should be "20"

    # Send a request with X-Token-Cost=30 - should be rate limited (need 30, only 20 remaining)
    When I set header "X-Token-Cost" to "30"
    And I send a "POST" request to "/ratelimit-cel-cost/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Global keyExtraction is inherited by quota when quota keyExtraction is omitted
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-global-key-inherited-api
      spec:
        displayName: RateLimit Global Key Inheritance API
        version: v1.0
        context: /ratelimit-global-key-inherited/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  keyExtraction:
                    - type: header
                      key: X-User-ID
                  quotas:
                    - name: inherited-global-key-limit
                      limits:
                        - limit: 3
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-global-key-inherited/v1.0/health" until status 200

    # User-A should have a dedicated bucket derived from global keyExtraction
    When I set header "X-User-ID" to "user-A"
    And I send 3 "GET" requests to "/ratelimit-global-key-inherited/v1.0/resource"
    Then the response status code should be 200
    When I set header "X-User-ID" to "user-A"
    And I send a "GET" request to "/ratelimit-global-key-inherited/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # User-B should have an independent bucket with full quota
    When I set header "X-User-ID" to "user-B"
    And I send 3 "GET" requests to "/ratelimit-global-key-inherited/v1.0/resource"
    Then the response status code should be 200
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-global-key-inherited/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Per-quota keyExtraction overrides global keyExtraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-quota-key-override-api
      spec:
        displayName: RateLimit Quota Key Override API
        version: v1.0
        context: /ratelimit-quota-key-override/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  keyExtraction:
                    - type: header
                      key: X-User-ID
                  quotas:
                    - name: override-key-limit
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: constant
                          key: shared-group
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-quota-key-override/v1.0/health" until status 200

    # User-A consumes 2/3 from the shared constant-key bucket
    When I set header "X-User-ID" to "user-A"
    And I send 2 "GET" requests to "/ratelimit-quota-key-override/v1.0/resource"
    Then the response status code should be 200

    # User-B shares the same bucket due to quota-level constant key override
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-quota-key-override/v1.0/resource"
    Then the response status code should be 200
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-quota-key-override/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Multiple costExtraction sources are summed in a single quota
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-cost-sum-api
      spec:
        displayName: RateLimit Cost Summation API
        version: v1.0
        context: /ratelimit-cost-sum/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: summed-cost-quota
                      limits:
                        - limit: 10
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_header
                            key: X-Header-Cost
                          - type: request_body
                            jsonPath: "$.body_cost"
                        default: 0
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-cost-sum/v1.0/health" until status 200

    # 3 (header) + 4 (body) = 7 consumed, so 3 should remain
    When I set header "X-Header-Cost" to "3"
    And I send a "POST" request to "/ratelimit-cost-sum/v1.0/resource" with body:
      """
      {"body_cost": 4}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "3"

    # 2 (header) + 2 (body) = 4 requested cost > remaining 3, should be blocked
    When I set header "X-Header-Cost" to "2"
    And I send a "POST" request to "/ratelimit-cost-sum/v1.0/resource" with body:
      """
      {"body_cost": 2}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: onRateLimitExceeded supports plain body and custom status code
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-custom-plain-status-api
      spec:
        displayName: RateLimit Custom Plain Status API
        version: v1.0
        context: /ratelimit-custom-plain-status/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: plain-error-limit
                      limits:
                        - limit: 1
                          duration: "1h"
                  onRateLimitExceeded:
                    statusCode: 503
                    body: "throttled"
                    bodyFormat: plain
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-custom-plain-status/v1.0/health" until status 200

    # First request should pass
    When I send a "GET" request to "/ratelimit-custom-plain-status/v1.0/resource"
    Then the response status code should be 200

    # Second request should return configured plain error with custom status
    When I send a "GET" request to "/ratelimit-custom-plain-status/v1.0/resource"
    Then the response status code should be 503
    And the response body should be:
      """
      throttled
      """

  Scenario: Missing header key component does not fail requests and still enforces quota
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-missing-header-key-api
      spec:
        displayName: RateLimit Missing Header Key API
        version: v1.0
        context: /ratelimit-missing-header-key/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: missing-header-quota
                      limits:
                        - limit: 2
                          duration: "1h"
                      keyExtraction:
                        - type: header
                          key: X-User-ID
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-missing-header-key/v1.0/health" until status 200

    # Requests without X-User-ID should not fail with extraction errors
    When I send 2 "GET" requests to "/ratelimit-missing-header-key/v1.0/resource"
    Then the response status code should be 200

    # Quota should still be enforced for requests with missing header component
    When I send a "GET" request to "/ratelimit-missing-header-key/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Empty global keyExtraction defaults to routename buckets
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-empty-global-key-api
      spec:
        displayName: RateLimit Empty Global Key API
        version: v1.0
        context: /ratelimit-empty-global-key/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /route1
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  keyExtraction: []
                  quotas:
                    - name: default-route-key
                      limits:
                        - limit: 2
                          duration: "1h"
          - method: GET
            path: /route2
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  keyExtraction: []
                  quotas:
                    - name: default-route-key
                      limits:
                        - limit: 2
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-empty-global-key/v1.0/health" until status 200

    # route1 should have its own default route-name bucket
    When I send 2 "GET" requests to "/ratelimit-empty-global-key/v1.0/route1"
    Then the response status code should be 200
    When I send a "GET" request to "/ratelimit-empty-global-key/v1.0/route1"
    Then the response status code should be 429

    # route2 should have an independent bucket when defaulting to routename
    When I send 2 "GET" requests to "/ratelimit-empty-global-key/v1.0/route2"
    Then the response status code should be 200
    When I send a "GET" request to "/ratelimit-empty-global-key/v1.0/route2"
    Then the response status code should be 429

  Scenario: Default cost is used only when all costExtraction sources fail
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-partial-cost-source-api
      spec:
        displayName: RateLimit Partial Cost Source API
        version: v1.0
        context: /ratelimit-partial-cost-source/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: partial-source-quota
                      limits:
                        - limit: 10
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_header
                            key: X-Token-Cost
                          - type: request_body
                            jsonPath: "$.missing_field"
                        default: 9
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-partial-cost-source/v1.0/health" until status 200

    # Header source succeeds (2), body source fails, so only successful extraction should apply
    When I set header "X-Token-Cost" to "2"
    And I send a "POST" request to "/ratelimit-partial-cost-source/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "8"

    # Consume remaining 8 using successful header extraction
    When I set header "X-Token-Cost" to "8"
    And I send a "POST" request to "/ratelimit-partial-cost-source/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Next request should be blocked
    When I set header "X-Token-Cost" to "1"
    And I send a "POST" request to "/ratelimit-partial-cost-source/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Cost extraction from response CEL expression
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-response-cel-cost-api
      spec:
        displayName: RateLimit Response CEL Cost API
        version: v1.0
        context: /ratelimit-response-cel-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: response-cel-quota
                      limits:
                        - limit: 4
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_cel
                            expression: 'response.Status / 100'
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-response-cel-cost/v1.0/health" until status 200

    # response.Status=200 -> extracted cost=2
    When I send a "GET" request to "/ratelimit-response-cel-cost/v1.0/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    # Second request consumes remaining 2
    When I send a "GET" request to "/ratelimit-response-cel-cost/v1.0/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Third request should be blocked
    When I send a "GET" request to "/ratelimit-response-cel-cost/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Cost extraction from response header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-response-header-cost-api
      spec:
        displayName: RateLimit Response Header Cost API
        version: v1.0
        context: /ratelimit-response-header-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: response-header-quota
                      limits:
                        - limit: 100
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: response_header
                            key: Content-Length
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-response-header-cost/v1.0/health" until status 200

    # backend responses are >100 bytes; first response should clamp remaining to 0
    When I send a "GET" request to "/ratelimit-response-header-cost/v1.0/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Next request should be blocked
    When I send a "GET" request to "/ratelimit-response-header-cost/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Fractional multiplier is applied to extracted cost
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-fractional-multiplier-api
      spec:
        displayName: RateLimit Fractional Multiplier API
        version: v1.0
        context: /ratelimit-fractional-multiplier/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: fractional-multiplier-quota
                      limits:
                        - limit: 4
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_header
                            key: X-Token-Cost
                            multiplier: 0.5
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-fractional-multiplier/v1.0/health" until status 200

    # 4 * 0.5 = 2 cost
    When I set header "X-Token-Cost" to "4"
    And I send a "POST" request to "/ratelimit-fractional-multiplier/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    # Another 2 consumed -> remaining 0
    When I set header "X-Token-Cost" to "4"
    And I send a "POST" request to "/ratelimit-fractional-multiplier/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Next request should be blocked
    When I set header "X-Token-Cost" to "4"
    And I send a "POST" request to "/ratelimit-fractional-multiplier/v1.0/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Zero extracted cost does not consume quota
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-zero-cost-api
      spec:
        displayName: RateLimit Zero Cost API
        version: v1.0
        context: /ratelimit-zero-cost/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: zero-cost-quota
                      limits:
                        - limit: 2
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_header
                            key: X-Token-Cost
                        default: 1
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-zero-cost/v1.0/health" until status 200

    # Zero-cost requests should not consume quota
    When I set header "X-Token-Cost" to "0"
    And I send 3 "GET" requests to "/ratelimit-zero-cost/v1.0/resource"
    Then the response status code should be 200

    # Two cost=1 requests should exhaust quota
    When I set header "X-Token-Cost" to "1"
    And I send a "GET" request to "/ratelimit-zero-cost/v1.0/resource"
    Then the response status code should be 200
    When I set header "X-Token-Cost" to "1"
    And I send a "GET" request to "/ratelimit-zero-cost/v1.0/resource"
    Then the response status code should be 200

    # Third cost=1 request should be blocked
    When I set header "X-Token-Cost" to "1"
    And I send a "GET" request to "/ratelimit-zero-cost/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Mixed global and per-quota key extraction are enforced independently
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-mixed-key-strategy-api
      spec:
        displayName: RateLimit Mixed Key Strategy API
        version: v1.0
        context: /ratelimit-mixed-key-strategy/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  keyExtraction:
                    - type: header
                      key: X-User-ID
                  quotas:
                    - name: per-user-quota
                      limits:
                        - limit: 2
                          duration: "1h"
                    - name: shared-quota
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: constant
                          key: shared
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-mixed-key-strategy/v1.0/health" until status 200

    # user-A consumes 2/2 in per-user quota and 2/3 in shared quota
    When I set header "X-User-ID" to "user-A"
    And I send 2 "GET" requests to "/ratelimit-mixed-key-strategy/v1.0/resource"
    Then the response status code should be 200

    # user-B has own per-user capacity, and consumes shared quota's final token
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-mixed-key-strategy/v1.0/resource"
    Then the response status code should be 200

    # shared quota now exhausted, should block even though user-B per-user quota has room
    When I set header "X-User-ID" to "user-B"
    And I send a "GET" request to "/ratelimit-mixed-key-strategy/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: APIVersion key extraction remains route-scoped for route-level policies
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-apiversion-key-api
      spec:
        displayName: RateLimit APIVersion Key API
        version: v1.0
        context: /ratelimit-apiversion-key/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /route1
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: version-shared-quota
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: apiversion
          - method: GET
            path: /route2
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: version-shared-quota
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: apiversion
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-apiversion-key/v1.0/health" until status 200

    # Route1 consumes from its own route-level quota
    When I send 2 "GET" requests to "/ratelimit-apiversion-key/v1.0/route1"
    Then the response status code should be 200

    # Route2 should still have its own full quota
    When I send 3 "GET" requests to "/ratelimit-apiversion-key/v1.0/route2"
    Then the response status code should be 200

    # Route2 should now be exhausted
    When I send a "GET" request to "/ratelimit-apiversion-key/v1.0/route2"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Route1 should still have remaining quota independently
    When I send a "GET" request to "/ratelimit-apiversion-key/v1.0/route1"
    Then the response status code should be 200

  Scenario: IP key extraction prioritizes X-Forwarded-For over X-Real-IP
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-ip-precedence-api
      spec:
        displayName: RateLimit IP Precedence API
        version: v1.0
        context: /ratelimit-ip-precedence/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: ip-precedence-quota
                      limits:
                        - limit: 2
                          duration: "1h"
                      keyExtraction:
                        - type: ip
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-ip-precedence/v1.0/health" until status 200

    When I set header "X-Forwarded-For" to "192.168.10.10"

    # Different X-Real-IP values should still map to same bucket if X-Forwarded-For is prioritized
    When I set header "X-Real-IP" to "10.0.0.1"
    And I send a "GET" request to "/ratelimit-ip-precedence/v1.0/resource"
    Then the response status code should be 200
    When I set header "X-Real-IP" to "10.0.0.2"
    And I send a "GET" request to "/ratelimit-ip-precedence/v1.0/resource"
    Then the response status code should be 200
    When I set header "X-Real-IP" to "10.0.0.3"
    And I send a "GET" request to "/ratelimit-ip-precedence/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Missing component in composite key extraction still enforces quota
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-missing-composite-component-api
      spec:
        displayName: RateLimit Missing Composite Component API
        version: v1.0
        context: /ratelimit-missing-composite-component/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: missing-composite-quota
                      limits:
                        - limit: 2
                          duration: "1h"
                      keyExtraction:
                        - type: apiname
                        - type: header
                          key: X-User-ID
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-missing-composite-component/v1.0/health" until status 200

    # Missing X-User-ID should not fail requests; quota should still enforce
    When I send 2 "GET" requests to "/ratelimit-missing-composite-component/v1.0/resource"
    Then the response status code should be 200
    When I send a "GET" request to "/ratelimit-missing-composite-component/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: onRateLimitExceeded supports custom JSON body with non-429 status
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-custom-json-status-api
      spec:
        displayName: RateLimit Custom JSON Status API
        version: v1.0
        context: /ratelimit-custom-json-status/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: json-error-quota
                      limits:
                        - limit: 1
                          duration: "1h"
                  onRateLimitExceeded:
                    statusCode: 503
                    body: '{"error":"Throttled","code":503001}'
                    bodyFormat: json
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-custom-json-status/v1.0/health" until status 200

    # First request should pass
    When I send a "GET" request to "/ratelimit-custom-json-status/v1.0/resource"
    Then the response status code should be 200

    # Second request should return configured JSON body with custom status
    When I send a "GET" request to "/ratelimit-custom-json-status/v1.0/resource"
    Then the response status code should be 503
    And the response should be valid JSON
    And the JSON response field "error" should be "Throttled"
    And the JSON response field "code" should be 503001

  Scenario: Malformed JSON request body falls back to default cost
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: ratelimit-malformed-json-default-cost-api
      spec:
        displayName: RateLimit Malformed JSON Default Cost API
        version: v1.0
        context: /ratelimit-malformed-json-default-cost/$version
        upstream:
          main:
            url: http://testbench:3002
        operations:
          - method: GET
            path: /anything
          - method: POST
            path: /anything
            policies:
              - name: advanced-ratelimit
                version: v1
                params:
                  quotas:
                    - name: malformed-json-quota
                      limits:
                        - limit: 4
                          duration: "1h"
                      costExtraction:
                        enabled: true
                        sources:
                          - type: request_body
                            jsonPath: "$.tokens"
                        default: 2
      """
    Then the response status code should be 201
    And I send a "GET" request to "/ratelimit-malformed-json-default-cost/v1.0/anything" until status 200

    # Malformed JSON should fail extraction and consume default cost=2
    When I send a "POST" request to "/ratelimit-malformed-json-default-cost/v1.0/anything" with body:
      """
      {invalid-json
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    # Another malformed JSON request should consume remaining 2
    When I send a "POST" request to "/ratelimit-malformed-json-default-cost/v1.0/anything" with body:
      """
      {invalid-json
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Next request should be blocked
    When I send a "POST" request to "/ratelimit-malformed-json-default-cost/v1.0/anything" with body:
      """
      {invalid-json
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."
