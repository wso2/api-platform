# Migrated from gateway/it/features/upstream-connect-timeout.feature.
#
# What changed mechanically:
#   - Absolute data-plane URLs (http://localhost:8080/...) reduced to relative paths.
#   - `I wait for 10 seconds` (a fixed propagation sleep) replaced by a wait on the condition
#     actually required: the route being programmed. The upstreams here are unreachable
#     (192.0.2.1, RFC 5737 TEST-NET-1) so nothing ever returns 200 — a readiness-to-200 wait is
#     impossible, hence the route-programmed form (it accepts any non-404, which the eventual
#     503 satisfies once the route exists).
#   - The one authenticate step moved into Background; the per-scenario/pre-delete duplicates
#     dropped. Each scenario deploys its own resource, waits, and deletes it.
#   - Upstream host sample-backend:9080 -> testbench:3000 (see the parked HCM scenario below).
#
# TIMING SENSITIVITY (Finding 2, docs/FINDINGS.md). The gateway emits the 503 ~130ms BEFORE the
# configured connect timeout elapses. The timeout steps carry a 5% proportional tolerance, which
# absorbs it: an 8s bound accepts >= 7.6s against a measured ~7.87s. The bounds are kept as the
# legacy feature wrote them, NOT relaxed further — the lower bound is the whole assertion, since
# an upstream that refuses instantly returns the same 503 as one that timed out correctly.
#
# READINESS here polls until 503 rather than until a success: the upstream is a blackholed
# 192.0.2.x address, so no request to these routes can ever answer 200. Each probe therefore
# blocks for the whole configured timeout, and the scenario pays that window twice — once to
# confirm the route is live, once to measure it. Do not "simplify" by dropping the assertion
# afterwards: the gate proves the route errors, while the assertion proves it errored LATE,
# which is the only thing separating an honoured timeout from an instant refusal.
#
# The global-default scenario additionally depends on this harness's global connect default being
# >= the asserted 5s. Neither the legacy it-config nor any v2 overlay sets one, so both fall
# through to the gateway's own default.

@backend-timeout # UNVERIFIED, and it matters only once Finding 2 is resolved: the "global defaults" scenario
# asserts >= 5 seconds, which holds only if the gateway's GLOBAL connect default really is >= 5s.
# Neither the legacy it-config nor any v2 overlay sets one, so both fall through to whatever the
# product ships. If that shipped default is smaller, that scenario will 503 early and fail for a
# reason unrelated to Finding 2 — check it before reading a failure there as the finding.

@upstream-connect-timeout @timeouts
Feature: Timeouts
  As an API developer
  I want upstream (connect) and HTTP Connection Manager timeouts to be enforced by the gateway
  So that requests to slow or unreachable backends, and slow downstream clients,
  fail within the configured timeout

  # request_timeout, stream_idle_timeout and idle_timeout are not exercised here:
  # small values would affect the whole shared suite

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # Tests cluster connect_timeout: upstream does not accept TCP connection in time.
  # Uses unreachable IP (192.0.2.1 per RFC 5737) so connect attempt hangs until connect_timeout.
  # The definition timeout (8s) is chosen deliberately above the global default (5s) plus the
  # 1s assertion tolerance: the "at least 8 seconds" check can only pass if the per-upstream 8s
  # timeout actually reaches Envoy. If the connect timeout were dropped and the cluster fell
  # back to the 5s default, the request would 503 at ~5s and this assertion would fail.
  @finding-2-early-timeout
  Scenario: RestApi backend timeout using upstreamDefinitions
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: timeout-api-v1.0
      spec:
        displayName: Timeout-API
        version: v1.0
        context: /timeout-api/$version
        upstreamDefinitions:
          - name: my-timeout-upstream
            timeout:
              connect: 8000ms
            upstreams:
              - url: http://192.0.2.1:8080
        upstream:
          main:
            ref: my-timeout-upstream
        operations:
          - method: GET
            path: /
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/timeout-api/v1.0/" until status 503
    When I send a "GET" request to "/timeout-api/v1.0/"
    Then the gateway should have timed out after "8" seconds with status 503
    When I delete the API "timeout-api-v1.0"
    Then the response status code should be 200

  # Global-default scenario: route timeout comes from the harness config; elapsed-time
  # assertion verifies the configured global timeout applies when no per-upstream one is set.
  @finding-2-early-timeout
  Scenario: RestApi without upstream timeout uses global defaults
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: timeout-api-global-v1.0
      spec:
        displayName: Timeout-API-Global
        version: v1.0
        context: /timeout-global/$version
        upstreamDefinitions:
          - name: my-timeout-upstream-global
            upstreams:
              - url: http://192.0.2.1:8080
        upstream:
          main:
            ref: my-timeout-upstream-global
        operations:
          - method: GET
            path: /
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/timeout-global/v1.0/" until status 503
    When I send a "GET" request to "/timeout-global/v1.0/"
    Then the gateway should have timed out after "5" seconds with status 503
    When I delete the API "timeout-api-global-v1.0"
    Then the response status code should be 200

  # PARKED — needs a raw-socket step the framework does not have.
  #
  # This scenario tests the HCM request_headers_timeout: a raw client sends partial request
  # headers and never terminates them, and the gateway must close the stream with 408 once
  # request_headers_timeout elapses. Two blockers:
  #
  #   1. NO STEP EXISTS for opening a raw TCP connection and dribbling an incomplete request.
  #      The legacy feature used, verbatim:
  #        When I open a raw connection to "localhost:8080" and send incomplete request headers for path "/headers-timeout/v1.0/"
  #        Then the raw response status code should be "408"
  #      Both need implementing in suites/it/steps (a raw net.Dial that writes a partial
  #      request line + headers, holds the socket open, then reads the status line). The gateway
  #      address must come from the topology, not a hardcoded localhost:8080.
  #   2. HARNESS CONFIG: request_headers_timeout was set to "5s" in the legacy it/test-config.toml.
  #      The v2 platform-gateway overlay must set the same for the 408 to fire (and for the
  #      "at least 4 seconds" assertion to be meaningful).
  #   3. NO TIMING STEP FITS. The timeout steps read Elapsed off the published httpx response, and
  #      a raw socket produces none. This needs its own elapsed measurement over the raw exchange;
  #      the wall-clock mark steps the legacy body uses below no longer exist.
  #
  # Upstream sample-backend:9080 -> testbench:3000 (host[:port] only, no path — valid for an
  # upstreamDefinitions URL). Left in place for whoever unblocks this.
  #
  # Scenario: HCM request_headers_timeout terminates a slow-header request
  #   When I create API with configuration:
  #     """
  #     apiVersion: gateway.api-platform.wso2.com/v1
  #     kind: RestApi
  #     metadata:
  #       name: headers-timeout-api-v1.0
  #     spec:
  #       displayName: Headers-Timeout-API
  #       version: v1.0
  #       context: /headers-timeout/$version
  #       upstreamDefinitions:
  #         - name: headers-timeout-upstream
  #           upstreams:
  #             - url: http://testbench:3000
  #       upstream:
  #         main:
  #           ref: headers-timeout-upstream
  #       operations:
  #         - method: GET
  #           path: /
  #     """
  #   Then the response should be successful
  #   And the response should be valid JSON
  #   And the JSON response field "status" should be "success"
  #   And I send a "GET" request to "/headers-timeout/v1.0/" until status 200
  #   And I record the current time as "request_start"
  #   When I open a raw connection to "localhost:8080" and send incomplete request headers for path "/headers-timeout/v1.0/"
  #   Then the raw response status code should be "408"
  #   And the request should have taken at least "4" seconds since "request_start"
  #   When I delete the API "headers-timeout-api-v1.0"
  #   Then the response should be successful

  # LLM Provider connect_timeout via upstreamDefinitions ref: the provider's upstream references
  # a definition whose only target is unreachable (192.0.2.1). The connect attempt hangs until
  # the per-upstream connect timeout (8s), then the gateway returns 503. Proves the ref ->
  # upstreamDefinition connect timeout works for LLM providers exactly as it does for RestApi.
  # The 8s value (above the 5s global default + 1s tolerance) makes "at least 8 seconds" pass
  # only when the timeout truly applies.
  @finding-2-early-timeout
  Scenario: LLM provider backend connect timeout using upstreamDefinitions ref
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-connect-timeout-provider
      spec:
        displayName: LLM Connect Timeout Provider
        version: v1.0
        template: openai
        context: /llm-connect-timeout
        upstreamDefinitions:
          - name: llm-unreachable-upstream
            timeout:
              connect: 8000ms
            upstreams:
              - url: http://192.0.2.1:8080
        upstream:
          ref: llm-unreachable-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I send a "GET" request to "/llm-connect-timeout/get" until status 503
    When I send a "GET" request to "/llm-connect-timeout/get"
    Then the gateway should have timed out after "8" seconds with status 503
    When I delete the LLM provider "llm-connect-timeout-provider"
    Then the response status code should be 200

  # MCP connect_timeout via upstreamDefinitions ref: the MCP backend reference is unreachable, so
  # the synthesized /mcp route's connect attempt hangs until the per-upstream connect timeout
  # (8s) -> 503. The 8s value (above the 5s global default + 1s tolerance) makes "at least 8
  # seconds" pass only when the per-upstream timeout truly reaches Envoy.
  @finding-2-early-timeout
  Scenario: MCP backend connect timeout using upstreamDefinitions ref
    When I create MCP proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: mcp-connect-timeout
      spec:
        displayName: MCP Connect Timeout
        version: v1.0
        context: /mcp-connect-timeout
        specVersion: "2025-06-18"
        upstreamDefinitions:
          - name: mcp-unreachable-upstream
            timeout:
              connect: 8000ms
            upstreams:
              - url: http://192.0.2.1:3001
        upstream:
          ref: mcp-unreachable-upstream
        tools: []
        resources: []
        prompts: []
      """
    Then the response status code should be 201
    And I send a "GET" request to "/mcp-connect-timeout/mcp" until status 503
    When I send a "GET" request to "/mcp-connect-timeout/mcp"
    Then the gateway should have timed out after "8" seconds with status 503
    When I delete the MCP proxy "mcp-connect-timeout"
    Then the response status code should be 200
