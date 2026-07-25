@llm_proxy
Feature: LLM proxy deployment with on-demand secret fetch
  As an API platform operator
  I want an LLM proxy configured with a secret-backed auth override to be deployed to the gateway
  So that the gateway controller fetches the secret value on demand when the deployment event arrives,
  confirming that secrets created after gateway startup are resolved correctly at deploy time.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  Scenario: An LLM proxy with a secret-backed auth override is deployed and active on the gateway
    Given an LLM provider deployed to the gateway for the proxy to reference
    And a secret containing an LLM proxy API key
    And an LLM proxy that references the provider and the secret
    When I deploy the LLM proxy to the gateway
    Then the gateway has the LLM proxy configured
