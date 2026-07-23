@llm_provider
Feature: LLM provider deployment with on-demand secret fetch
  As an API platform operator
  I want an LLM provider configured with a secret-backed API key to be deployed to the gateway
  So that the gateway controller fetches the secret value on demand when the deployment event arrives,
  confirming that secrets created after gateway startup are resolved correctly at deploy time.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  Scenario: An LLM provider with a secret-backed API key is deployed and active on the gateway
    Given a secret containing an LLM provider API key
    And an LLM provider that references the secret
    When I deploy the LLM provider to the gateway
    Then the gateway has the LLM provider configured
