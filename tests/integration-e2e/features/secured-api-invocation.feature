Feature: A secured API published to platform-api is deployed and invoked through the gateway
  As an API platform operator
  I want a published API protected by an API key and subscription validation to be
  invocable through the gateway data plane only with valid credentials
  So that the control plane, subscription model and data plane work together end to end.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @secured
  Scenario: A published, secured API is invocable through the gateway only with valid credentials
    Given a subscription plan "e2e-gold" allowing 10000 requests per hour
    And a published REST API secured with API key and subscription validation offering that plan
    When I deploy the secured API to the gateway
    Then an unauthenticated request to the secured API is rejected
    When an application is subscribed to the API under that plan
    And an API key is issued for the API
    Then invoking the secured API through the gateway with valid credentials returns 200
    And invoking the secured API through the gateway without credentials is rejected
