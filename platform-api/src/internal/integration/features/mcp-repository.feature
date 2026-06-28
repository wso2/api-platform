Feature: MCP control-plane repository lifecycle across database engines
  As a platform-api maintainer
  I want the MCP proxy repository to round-trip correctly
  So that the MCP control plane is verified on every engine — with no real MCP server and no real API key.

  Background:
    Given a clean platform-api database
    And an organization and project exist

  Scenario: MCP proxy create, read, update, list and delete with a dummy upstream key
    When I create an MCP proxy "my-mcp" with upstream key "Bearer mcp-test-key"
    Then reading the MCP proxy back returns upstream key "Bearer mcp-test-key"
    And listing MCP proxies by organization returns 1
    And listing MCP proxies by project returns 1
    When I update the MCP proxy description to "updated by integration test"
    Then reading the MCP proxy back shows description "updated by integration test"
    When I delete the MCP proxy
    Then listing MCP proxies by organization returns 0
    And listing MCP proxies by project returns 0
