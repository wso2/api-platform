# Migrated from gateway/it/features/backend-timeout.feature.
#
# What changed mechanically:
#   - Upstream http://echo-backend:80 -> http://testbench:3000. echo-backend (go-httpbin) is
#     NOT wired into any gateway block; the testbench backend reflector is the upstream every
#     migrated gateway feature uses.
#   - Absolute data-plane URLs (http://localhost:8080/...) reduced to relative paths.
#   - The one authenticate step moved into Background; the per-scenario/pre-delete duplicates
#     dropped. Each scenario deploys its own API, waits for its own readiness, and deletes its
#     own API (createAPI registers per-scenario cleanup).
#
# BLOCKED — DO NOT WIRE INTO it-suite.yaml YET.
#
# These scenarios need a SLOW upstream: a route timeout (Envoy RouteAction.Timeout) can only
# be proven by an upstream that takes LONGER than the timeout to respond. The testbench backend
# reflector (services/backend/backend.go, port 3000) answers instantly and has no delay knob,
# so as written:
#   - "API-level resilience timeout terminates a slow backend" and "Operation-level resilience
#     timeout overrides the API level" would get an instant 200, never the 504 they assert.
#   - "No resilience block falls back to the global default and succeeds" would pass, but
#     vacuously — it would not be exercising the >2s-but-under-default delay it is meant to.
#
# UNBLOCK: teach the testbench backend reflector to honour GET /delay/{seconds} by sleeping
# {seconds} seconds and then returning its normal 200 reflector JSON — i.e. reproduce
# go-httpbin's /delay/{n} contract, which echo-backend originally provided and which this
# feature was written against. (A query form such as ?delayMs=N would work equally well; the
# path form is chosen here because it keeps the operation paths byte-for-byte identical to the
# legacy feature.) An unreachable address is deliberately NOT substituted here: that produces a
# CONNECT timeout (503), a different code path and status from the route/response timeout (504)
# these scenarios assert.
#
# TIMING SENSITIVITY: the two 504 scenarios assert "at least 2 seconds" against a 2s route
# timeout, and the v2 elapsedAtLeast step is strict (no tolerance). This is the same assertion
# shape that Finding 2 makes fragile for connect timeouts; whether route timeouts also fire a
# touch early is unverified. Flagged rather than pre-adjusted.

@backend-timeout @resilience
Feature: Backend route timeouts via the resilience block
  As an API developer
  I want to configure the route timeout via a resilience block at the API and operation level
  So that requests to slow backends are terminated by the gateway within the configured time

  # These scenarios exercise resilience.timeout (Envoy RouteAction.Timeout). The slow backend
  # is the testbench reflector's /delay/{n} (see BLOCKED note above), which sleeps n seconds
  # before responding; when the route timeout is shorter than the delay, the gateway returns
  # 504. resilience.idleTimeout maps to RouteAction.IdleTimeout and is covered by unit tests
  # (it cannot be exercised deterministically over HTTP here).

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # API-level resilience.timeout (2s) is shorter than the backend delay (5s), so the
  # gateway must time the route out with 504 at ~2s instead of waiting for the backend.
  @finding-2-early-timeout
  Scenario: API-level resilience timeout terminates a slow backend
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: backend-timeout-api-v1.0
      spec:
        displayName: Backend-Timeout-API
        version: v1.0
        context: /backend-timeout-api/$version
        upstream:
          main:
            url: http://testbench:3000
        resilience:
          timeout: 2s
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /delay/5
      """
    Then the response status code should be 201
    And I send a "GET" request to "/backend-timeout-api/v1.0/get" until status 200
    When I send a "GET" request to "/backend-timeout-api/v1.0/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the API "backend-timeout-api-v1.0"
    Then the response status code should be 200

  # Operation-level resilience overrides the API level: the API-level timeout (10s) is
  # longer than the backend delay (5s) and would let the request succeed, but the
  # operation-level timeout (2s) wins, so the gateway returns 504 at ~2s.
  @finding-2-early-timeout
  Scenario: Operation-level resilience timeout overrides the API level
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: backend-timeout-override-v1.0
      spec:
        displayName: Backend-Timeout-Override
        version: v1.0
        context: /backend-timeout-override/$version
        upstream:
          main:
            url: http://testbench:3000
        resilience:
          timeout: 10s
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /delay/5
            resilience:
              timeout: 2s
      """
    Then the response status code should be 201
    And I send a "GET" request to "/backend-timeout-override/v1.0/get" until status 200
    When I send a "GET" request to "/backend-timeout-override/v1.0/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the API "backend-timeout-override-v1.0"
    Then the response status code should be 200

  # Without a resilience block the global route timeout default (60s) applies, so a
  # backend that responds within it succeeds normally.
  Scenario: No resilience block falls back to the global default and succeeds
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: backend-timeout-default-v1.0
      spec:
        displayName: Backend-Timeout-Default
        version: v1.0
        context: /backend-timeout-default/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /delay/2
      """
    Then the response status code should be 201
    When I send a "GET" request to "/backend-timeout-default/v1.0/delay/2" until status 200
    Then the response status code should be 200
    When I delete the API "backend-timeout-default-v1.0"
    Then the response status code should be 200
