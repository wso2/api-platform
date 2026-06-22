Feature: Deploying APIs from platform-api to the gateway
  As an API platform operator
  I want an API created in platform-api to be served by the gateway data plane
  So that the control plane and data plane work together on every supported database.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @smoke
  Scenario: An API deployed to a gateway is served by the data plane
    Given a REST API routed to the sample backend
    When I deploy the API to the gateway
    Then the gateway serves the API
    And a request to a path outside the API context returns 404

  Scenario: Undeploying stops the gateway serving the API, and redeploying restores it
    Given a REST API routed to the sample backend
    And the API is deployed to the gateway and served
    When I undeploy the API from the gateway
    Then the gateway stops serving the API
    When I deploy the API to the gateway
    Then the gateway serves the API

  @multigateway
  Scenario: An API deployed to two gateways is served by both, and undeploy is isolated
    Given a REST API routed to the sample backend
    And the API is deployed to the gateway and served
    When I deploy the API to the second gateway
    Then the second gateway serves the API
    When I undeploy the API from the second gateway
    Then the second gateway stops serving the API
    And the gateway still serves the API
