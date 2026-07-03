Feature: An MCP proxy deployed from platform-api is served by the gateway
  As an API platform operator
  I want an MCP proxy created in platform-api to be served by the gateway data
  plane
  So that the control-plane → data-plane path works for MCP proxies, not just
  REST APIs.

  # postgres-only (@mcp). Needs a real MCP (JSON-RPC streamable-HTTP) server —
  # the echo sample-backend can't satisfy the initialize handshake — so the
  # mcp-backend container is started on demand. The gateway serves the proxy at
  # <context>/mcp.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @mcp
  Scenario: A gateway serves a deployed MCP proxy
    Given the MCP backend is running
    And an MCP proxy routed to the MCP backend
    When I deploy the MCP proxy to the gateway
    Then the gateway serves the MCP proxy
