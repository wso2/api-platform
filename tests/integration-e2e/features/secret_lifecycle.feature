@secret_lifecycle
Feature: Live secret rotation and deletion push events to a connected gateway
  As an API platform operator
  I want a connected gateway to pick up a secret rotation or deletion immediately
  So that credential changes take effect without waiting for the gateway's next
  reconnect-triggered poll, and without restarting the gateway-controller.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  Scenario: Rotating a secret pushes the new value to an already-connected gateway
    Given a secret containing a REST API upstream credential
    And a REST API whose upstream auth references the secret
    And I deploy the secret-backed REST API to the gateway
    And the gateway has the secret-backed REST API configured
    When I rotate the secret to a new value
    Then the gateway's local copy of the secret has the rotated value

  Scenario: Deleting a secret evicts it from an already-connected gateway
    Given a secret containing a REST API upstream credential
    And a REST API whose upstream auth references the secret
    And I deploy the secret-backed REST API to the gateway
    And the gateway has the secret-backed REST API configured
    When I update the REST API to reference a different secret instead
    Then the gateway evicts the original secret from its local store
