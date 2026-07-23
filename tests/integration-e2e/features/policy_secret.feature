@policy_secret
Feature: REST API deployment with a secret reference inside a policy configuration
  As an API platform operator
  I want a REST API operation's set-headers policy to reference a secret for its header value
  So that the gateway controller resolves {{ secret "..." }} placeholders wherever they appear in
  an artifact's rendered configuration, not only in an upstream auth block.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  Scenario: A REST API with a secret-backed policy header value is deployed and active on the gateway
    Given a secret containing a header value
    And a REST API with a set-headers policy referencing the secret
    When I deploy the policy-secret REST API to the gateway
    Then the gateway has the policy-secret REST API configured
