Feature: AI Workspace CLI publish and persistence
  As a platform user driving the `ap` CLI
  I want to publish AI Workspace artifacts to platform-api
  So that I can confirm the CLI create/update/read behaviour persists on the backend

  The suite boots a real platform-api (the AI Workspace backend), logs in as
  admin/admin for a bearer token, creates a server-side project, and registers the
  gateways the artifacts associate with. Each scenario scaffolds a project with
  `ap project init`, applies the "create" demo content, reads it back with both
  `get` and `list`, then applies the "edit" demo content (which adds
  spec.associatedGateways) to confirm the update path persists too.

  Background:
    Given the platform-api AI Workspace backend is running
    And I am authenticated to the AI Workspace

  @llm-provider
  Scenario: Publish and update an LLM provider through the CLI
    When the "llm-provider" project artifact is initialized
    And I build the "llm-provider" artifact
    And I apply the "llm-provider" artifact
    Then the CLI reports the "llm-provider" artifact was created
    And the "llm-provider" artifact is retrievable from the AI Workspace
    And the "llm-provider" artifact is listed in the AI Workspace
    When I edit the "llm-provider" artifact
    And I build the "llm-provider" artifact
    And I re-apply the "llm-provider" artifact
    Then the CLI reports the "llm-provider" artifact was updated
    And the "llm-provider" artifact is associated with gateway "prod-eu-01"

  @llm-proxy
  Scenario: Publish and update an LLM proxy through the CLI
    When the "llm-proxy" project artifact is initialized
    And I build the "llm-proxy" artifact
    And I apply the "llm-proxy" artifact
    Then the CLI reports the "llm-proxy" artifact was created
    And the "llm-proxy" artifact is retrievable from the AI Workspace
    And the "llm-proxy" artifact is listed in the AI Workspace
    When I edit the "llm-proxy" artifact
    And I build the "llm-proxy" artifact
    And I re-apply the "llm-proxy" artifact
    Then the CLI reports the "llm-proxy" artifact was updated
    And the "llm-proxy" artifact is associated with gateway "prod-eu-01"

  @mcp-proxy
  Scenario: Publish and update an MCP proxy through the CLI
    When the "mcp-proxy" project artifact is initialized
    And I build the "mcp-proxy" artifact
    And I apply the "mcp-proxy" artifact
    Then the CLI reports the "mcp-proxy" artifact was created
    And the "mcp-proxy" artifact is retrievable from the AI Workspace
    And the "mcp-proxy" artifact is listed in the AI Workspace
    When I edit the "mcp-proxy" artifact
    And I build the "mcp-proxy" artifact
    And I re-apply the "mcp-proxy" artifact
    Then the CLI reports the "mcp-proxy" artifact was updated
    And the "mcp-proxy" artifact is associated with gateway "prod-eu-01"
