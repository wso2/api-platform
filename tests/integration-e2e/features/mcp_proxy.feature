@mcp_proxy
Feature: MCP proxy deployment with on-demand secret fetch
  As an API platform operator
  I want an MCP proxy configured with a secret-backed upstream API key to be deployed to the gateway
  So that the gateway controller fetches the secret value on demand when the deployment event arrives,
  confirming that secrets created after gateway startup are resolved correctly at deploy time.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  Scenario: An MCP proxy with a secret-backed upstream API key is deployed and active on the gateway
    Given a secret containing an MCP proxy upstream API key
    And an MCP proxy that references the secret
    When I deploy the MCP proxy to the gateway
    Then the gateway has the MCP proxy configured
