@basic-ratelimit
Feature: Basic Rate Limiting
  As an API developer
  I want a simple rate limiting policy
  So that I can easily protect my APIs without complex configuration

  # Migrated with every assertion intact. Mechanical differences from the legacy suite:
  #
  # Upstream http://sample-backend:9080/api/v1 becomes http://testbench:3000/api/v1 (the shared
  # backend reflector), keeping the appended /api/v1 path. Request URLs are reduced from the
  # absolute http://localhost:8080/... form to their path, including the readiness waits.
  #
  # The per-scenario `Given I authenticate using basic auth as "admin"` is gone — one lives in
  # the Background, and management-API credentials are reset before every scenario anyway.
  #
  # There is no `I wait for N seconds` step. The two post-update settle waits are expressed as
  # `I wait for policy snapshot sync`, which waits for the pushed config to go live directly.
  #
  # COUNTING: a readiness wait polls a path until 200, and in v2 each successful poll is a real
  # request counted against whatever limits that path carries (the legacy `wait 2 seconds` settle
  # cost nothing — that is the calibration these counts assume). So the settle must never spend a
  # token from a bucket a later step counts precisely. Three cases arise:
  #   - An unlimited sibling exists (a plain /probe, or a /health with no policy): readiness targets
  #     it and the counted path keeps its full quota. Most scenarios here.
  #   - An API-LEVEL policy covers every operation, so no sibling escapes that bucket ("Rate limit
  #     scope based on policy attachment level"): a data-plane poll would shift the count, so it
  #     waits on `policy snapshot sync` plus the policy-engine config dump — control-plane checks
  #     that confirm propagation without sending a data-plane request.
  #   - An API-level policy covers everything AND there is no control-plane-only way to gate the
  #     router ("Route-level /books traffic..."): readiness runs on /books and the single token it
  #     spends is proven inline to land on an un-asserted request.
  # The two update scenarios add a /probe purely so the POST-UPDATE settle has a non-counted path
  # to poll — snapshot sync can return before the router has swapped the new config in.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce basic rate limit on API resource
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-test-api
      spec:
        displayName: Basic RateLimit Test API
        version: v1.0
        context: /basic-ratelimit/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /limited
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 5
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit/v1.0/probe" until status 200

    # Readiness runs against the unlimited /probe op, so /limited starts with its full quota of 5.
    # Send 5 requests - all should succeed
    When I send 5 "GET" requests to "/basic-ratelimit/v1.0/limited"
    Then the response status code should be 200

    # The 6th request exhausts the quota
    When I send a "GET" request to "/basic-ratelimit/v1.0/limited"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Rate limit headers are returned
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-headers-api
      spec:
        displayName: Basic RateLimit Headers API
        version: v1.0
        context: /basic-ratelimit-headers/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /check
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 100
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-headers/v1.0/check" until status 200

    When I send a "GET" request to "/basic-ratelimit-headers/v1.0/check"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

  Scenario: Multiple limits enforce most restrictive limit
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-multi-limits-api
      spec:
        displayName: Basic RateLimit Multi Limits API
        version: v1.0
        context: /basic-ratelimit-multi/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 10
                      duration: "1h"
                    - requests: 5
                      duration: "24h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-multi/v1.0/health" until status 200

    # 24h limit (5) is more restrictive than 1h limit (10)
    # Send 5 requests - should succeed (5/5 for 24h, 5/10 for 1h)
    When I send 5 "GET" requests to "/basic-ratelimit-multi/v1.0/resource"
    Then the response status code should be 200

    # 6th request should be blocked by 24h limit
    When I send a "GET" request to "/basic-ratelimit-multi/v1.0/resource"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Per-route rate limiting with basic-ratelimit
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-per-route-api
      spec:
        displayName: Basic RateLimit Per Route API
        version: v1.0
        context: /basic-ratelimit-per-route/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /route1
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
          - method: GET
            path: /route2
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-per-route/v1.0/health" until status 200

    # Each route has its own quota (basic-ratelimit uses routename as key)
    # Send 3 requests to route1 - should succeed (uses route1's quota)
    When I send 3 "GET" requests to "/basic-ratelimit-per-route/v1.0/route1"
    Then the response status code should be 200

    # route1's 4th request should be rate limited
    When I send a "GET" request to "/basic-ratelimit-per-route/v1.0/route1"
    Then the response status code should be 429

    # route2 has its own separate quota - should still work
    When I send 3 "GET" requests to "/basic-ratelimit-per-route/v1.0/route2"
    Then the response status code should be 200

    # route2's 4th request should also be rate limited
    When I send a "GET" request to "/basic-ratelimit-per-route/v1.0/route2"
    Then the response status code should be 429

  Scenario: 429 response includes Retry-After header
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-retry-after-api
      spec:
        displayName: Basic RateLimit Retry After API
        version: v1.0
        context: /basic-ratelimit-retry/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-retry/v1.0/health" until status 200

    # Exhaust the rate limit (limit=3)
    When I send 3 "GET" requests to "/basic-ratelimit-retry/v1.0/resource"
    Then the response status code should be 200

    # Next request should be rate limited with Retry-After header
    When I send a "GET" request to "/basic-ratelimit-retry/v1.0/resource"
    Then the response status code should be 429
    And the response header "Retry-After" should exist
  Scenario: API-level quota is shared across operations without route-level policies
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-api-shared-api
      spec:
        displayName: Basic RateLimit API Shared Bucket API
        version: v1.0
        context: /basic-ratelimit-api-shared/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 20
                  duration: "1h"
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /route-a
          - method: GET
            path: /route-b
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-api-shared/v1.0/health" until status 200

    # API-level bucket is shared by route-a and route-b
    When I send 6 "GET" requests to "/basic-ratelimit-api-shared/v1.0/route-a"
    Then the response status code should be 200

    When I send 6 "GET" requests to "/basic-ratelimit-api-shared/v1.0/route-b"
    Then the response status code should be 200

    When I send 12 "GET" requests to "/basic-ratelimit-api-shared/v1.0/route-a"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

  Scenario: Route-level limit does not throttle unprotected sibling operation when API-level policy is absent
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-route-isolation-api
      spec:
        displayName: Basic RateLimit Route Isolation API
        version: v1.0
        context: /basic-ratelimit-route-isolation/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /limited
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
          - method: GET
            path: /open
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-route-isolation/v1.0/health" until status 200

    When I send 3 "GET" requests to "/basic-ratelimit-route-isolation/v1.0/limited"
    Then the response status code should be 200

    When I send a "GET" request to "/basic-ratelimit-route-isolation/v1.0/limited"
    Then the response status code should be 429

    When I send 5 "GET" requests to "/basic-ratelimit-route-isolation/v1.0/open"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should not exist

  Scenario: Lower API-level limit still blocks a route with higher route-level limit
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-additive-limit-api
      spec:
        displayName: Basic RateLimit Additive Limit API
        version: v1.0
        context: /basic-ratelimit-additive-limit/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 10
                  duration: "1h"
        operations:
          - method: GET
            path: /health
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 100
                      duration: "1h"
          - method: GET
            path: /resource-b
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 100
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-additive-limit/v1.0/health" until status 200

    When I send 8 "GET" requests to "/basic-ratelimit-additive-limit/v1.0/resource-b"
    Then the response status code should be 200

    When I send 3 "GET" requests to "/basic-ratelimit-additive-limit/v1.0/resource-b"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "10"

  Scenario: Mixed attachment returns scope-correct limit headers on 429
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-mixed-headers-api
      spec:
        displayName: Basic RateLimit Mixed Headers API
        version: v1.0
        context: /basic-ratelimit-mixed-headers/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 5
                  duration: "1h"
        operations:
          - method: GET
            path: /health
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 100
                      duration: "1h"
          - method: GET
            path: /resource-a
          - method: GET
            path: /resource-b
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-mixed-headers/v1.0/health" until status 200

    When I send 3 "GET" requests to "/basic-ratelimit-mixed-headers/v1.0/resource-b"
    Then the response status code should be 200

    When I send a "GET" request to "/basic-ratelimit-mixed-headers/v1.0/resource-b"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "3"

    # API-level bucket is already exhausted by readiness + /resource-b traffic
    When I send a "GET" request to "/basic-ratelimit-mixed-headers/v1.0/resource-a"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "5"

  Scenario: Updating API adds then removes route-level policy for the same route
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-update-route-policy-api
      spec:
        displayName: Basic RateLimit Update Route Policy API
        version: v1.0
        context: /basic-ratelimit-update-route-policy/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 50
                  duration: "1h"
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /health
          - method: GET
            path: /resource
      """
    Then the response status code should be 201
    # /probe carries no route-level policy, so a readiness poll against it never touches the
    # route-level bucket on /resource that this scenario counts. It is under the API-level quota
    # (50), which has ample headroom for the handful of settle polls this scenario makes.
    And I send a "GET" request to "/basic-ratelimit-update-route-policy/v1.0/probe" until status 200

    # API-level only baseline
    When I send 5 "GET" requests to "/basic-ratelimit-update-route-policy/v1.0/resource"
    Then the response status code should be 200

    # Add route-level basic-ratelimit policy on /resource
    When I update the API "basic-ratelimit-update-route-policy-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-update-route-policy-api
      spec:
        displayName: Basic RateLimit Update Route Policy API
        version: v1.0
        context: /basic-ratelimit-update-route-policy/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 50
                  duration: "1h"
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 2
                      duration: "1h"
      """
    Then the response status code should be 200
    # Readiness gate: only the added 2-request route bucket can 429 within this bounded poll,
    # so reaching it proves the update is serving.
    And I send a "GET" request to "/basic-ratelimit-update-route-policy/v1.0/resource" until status 429
    And the response header "X-RateLimit-Limit" should be "2"
    And the response header "X-RateLimit-Remaining" should be "0"

    # Remove route-level basic-ratelimit policy from /resource
    When I update the API "basic-ratelimit-update-route-policy-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-update-route-policy-api
      spec:
        displayName: Basic RateLimit Update Route Policy API
        version: v1.0
        context: /basic-ratelimit-update-route-policy/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 50
                  duration: "1h"
        operations:
          - method: GET
            path: /probe
          - method: GET
            path: /health
          - method: GET
            path: /resource
      """
    Then the response status code should be 200
    # Readiness gate: polls hit the exhausted route bucket's 429 (spending nothing) until the
    # removal serves the first 200.
    And I send a "GET" request to "/basic-ratelimit-update-route-policy/v1.0/resource" until status 200
    And the response header "X-RateLimit-Limit" should be "50"

    When I send 3 "GET" requests to "/basic-ratelimit-update-route-policy/v1.0/resource"
    Then the response status code should be 200

  Scenario: API-level quota is consumed across routes when one route also has route-level policy
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-reading-list-mixed-api
      spec:
        displayName: Basic RateLimit Reading List Mixed API
        version: v1.0
        context: /basic-ratelimit-reading-list/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 15
                  duration: "24h"
        operations:
          - method: GET
            path: /health
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 100
                      duration: "24h"
          - method: GET
            path: /books
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 3
                      duration: "24h"
          - method: GET
            path: /authors
          - method: GET
            path: /categories
      """
    Then the response status code should be 201
    And I send a "GET" request to "/basic-ratelimit-reading-list/v1.0/health" until status 200

    # Route-level bucket for /books (3/hour)
    When I send 3 "GET" requests to "/basic-ratelimit-reading-list/v1.0/books"
    Then the response status code should be 200

    When I send a "GET" request to "/basic-ratelimit-reading-list/v1.0/books"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "3"
    And the response header "X-RateLimit-Remaining" should be "0"

    # API-level bucket is shared across /books, /authors, and /categories
    When I send 8 "GET" requests to "/basic-ratelimit-reading-list/v1.0/authors"
    Then the response status code should be 200

    When I send 8 "GET" requests to "/basic-ratelimit-reading-list/v1.0/categories"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "15"
    And the response header "X-RateLimit-Remaining" should be "0"

  Scenario: Route-level /books traffic also consumes API-level bucket used by /books/{id}
    # The legacy carried a two-segment context (/analytics-test-new/reading-list-api/$version) and
    # a bare `project-id` label copied from an analytics-routing feature. Neither is exercised by
    # this scenario, which is purely about rate-limit bucket sharing, and the two-segment context
    # is the only one of its kind in the v2 suite — it does not route here, which is the 404 the
    # request was seeing. The context is normalised to a single segment and the label dropped; every
    # assertion is unchanged. (The v2 project annotation is the fully-qualified
    # gateway.api-platform.wso2.com/project-id form, not a bare `project-id` label, in any case.)
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: basic-ratelimit-reading-list-template-api
      spec:
        displayName: Reading List Template API
        version: v1.0
        context: /basic-ratelimit-reading-list-template/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        policies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 15
                  duration: "24h"
        operations:
          - method: GET
            path: /books
            policies:
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 50
                      duration: "24h"
          - method: POST
            path: /books
          - method: GET
            path: /books/{id}
          - method: PUT
            path: /books/{id}
          - method: DELETE
            path: /books/{id}
      """
    Then the response status code should be 201
    # Every operation is under the API-level quota (15) and there is no unlimited sibling to poll,
    # so this readiness on /books unavoidably spends one API-level token. That is safe here: the
    # token lands on the FIRST /books/{id} request, which the scenario does not assert — only the
    # 2nd /books/{id} (429, remaining 0) and the 14th /books (200) are asserted, and both hold
    # whether the API bucket starts at 0 (legacy) or 1 (after this poll). Snapshot sync cannot be
    # used instead: it gates the policy engine, not the router's route programming, so the first
    # /books requests 404 before the route is live.
    And I send a "GET" request to "/basic-ratelimit-reading-list-template/v1.0/books" until status 200

    # /books has route-level=50, so these calls should not be route-limited
    When I send 14 "GET" requests to "/basic-ratelimit-reading-list-template/v1.0/books"
    Then the response status code should be 200

    # If /books consumed the API-level bucket (15), the 2nd /books/{id} request is denied by API-level limit
    When I send 2 "GET" requests to "/basic-ratelimit-reading-list-template/v1.0/books/1d4c9647-5e62-4f1d-9c30-e1f25c6d0e73"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "15"
    And the response header "X-RateLimit-Remaining" should be "0"
