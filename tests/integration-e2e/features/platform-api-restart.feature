Feature: The system survives a platform-api (control plane) restart
  As an API platform operator
  I want the gateway to keep serving and the control plane to recover when
  platform-api restarts
  So that a control-plane pod bounce or upgrade does not drop live traffic and
  the control plane keeps working afterwards.

  # A control-plane restart is distinct from a gateway restart: the data plane
  # (gateway-runtime + the controller's locally persisted config) must keep
  # serving while platform-api is down, and the gateway-controller must
  # auto-reconnect its control-plane socket when platform-api comes back and pick
  # up new work — no gateway restart. postgres-only (@restart).

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @restart @cp-restart
  Scenario: The gateway keeps serving across a platform-api restart, and the control plane recovers
    Given a REST API routed to the sample backend
    And the API is deployed to the gateway and served
    When I restart the platform-api control plane
    Then the gateway still serves the API
    And the control plane accepts requests again

  @restart @cp-restart
  Scenario: The gateway auto-reconnects and picks up a new deployment after a platform-api restart
    Given a REST API routed to the sample backend
    And the API is deployed to the gateway and served
    And a second REST API routed to the sample backend
    When I restart the platform-api control plane
    And I deploy the second API to the gateway
    Then the gateway serves the second API
    And the gateway still serves the API
