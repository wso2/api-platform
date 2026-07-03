Feature: API-key authentication enforced end to end
  As an API platform operator
  I want a deployed API to reject unauthenticated requests and accept a key
  generated in platform-api
  So that the control-plane API-key flow enforces access at the data plane.

  # postgres-only (@apikey). The key is generated in platform-api and broadcast to
  # the gateway (apikey.created event); the gateway then accepts it.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @apikey
  Scenario: A deployed API rejects requests without a key and accepts a generated key
    Given a REST API that requires an API key
    When I deploy the API to the gateway
    Then a request without an API key is rejected
    When I generate an API key for the API
    Then a request with the API key is accepted
