@rest_api_secret
Feature: REST API deployment with an on-demand secret-backed upstream credential
  As an API platform operator
  I want a REST API whose upstream auth references a secret to be deployed to the gateway
  So that the gateway controller fetches the secret value on demand when the deployment event arrives,
  confirming that secrets created after gateway startup are resolved correctly at deploy time.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  Scenario: A REST API with a secret-backed upstream credential is deployed and active on the gateway
    Given a secret containing a REST API upstream credential
    And a REST API whose upstream auth references the secret
    When I deploy the secret-backed REST API to the gateway
    Then the gateway has the secret-backed REST API configured
