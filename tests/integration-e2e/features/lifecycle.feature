Feature: REST API deployment lifecycle from platform-api to the gateway
  As an API platform operator
  I want update, delete and restore of a deployment to propagate to the gateway
  So that the full lifecycle — not just first deploy — works end to end.

  # postgres-only (@lifecycle), consistent with the other extended suites.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @lifecycle
  Scenario: Updating an API and redeploying propagates the new version to live traffic
    Given a REST API that injects the version header "v1"
    And the API is deployed to the gateway and served
    And the gateway injects the version header "v1"
    When I update the API to inject the version header "v2" and redeploy
    Then the gateway injects the version header "v2"

  @lifecycle
  Scenario: Deleting an API stops the gateway serving it
    Given a REST API routed to the sample backend
    And the API is deployed to the gateway and served
    When I delete the API from platform-api
    Then the gateway stops serving the API

  @lifecycle
  Scenario: Restoring an undeployed deployment resumes serving
    Given a REST API routed to the sample backend
    And the API is deployed to the gateway and served
    When I undeploy the API from the gateway
    Then the gateway stops serving the API
    When I restore the deployment
    Then the gateway serves the API
