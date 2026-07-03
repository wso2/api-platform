Feature: A gateway recovers its served APIs across restarts
  As an API platform operator
  I want deployed APIs to keep being served after a gateway restarts
  So that pod bounces, upgrades and crashes do not drop live traffic.

  # These scenarios run on the postgres stack (the default), which is also the
  # only stack wired with a second gateway for the multi-gateway isolation case.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api
    And a REST API routed to the sample backend
    And the API is deployed to the gateway and served

  @smoke @restart
  Scenario: Restarting the gateway controller keeps the API served
    When I restart the gateway controller
    Then the gateway serves the API
    And a request to a path outside the API context returns 404

  @restart
  Scenario: Restarting the gateway runtime re-serves the API
    When I restart the gateway runtime
    Then the gateway serves the API

  @restart
  Scenario: Restarting the whole gateway recovers all served APIs
    When I restart the whole gateway
    Then the gateway serves the API

  @restart
  Scenario: A deployment made while the gateway is down is picked up on restart
    Given a second REST API routed to the sample backend
    When the gateway controller is stopped
    And I deploy the second API to the gateway while it is stopped
    And the gateway controller is started
    Then the gateway serves the API
    And the gateway serves the second API

  @restart
  Scenario: An undeployment made while the gateway is down is applied on restart
    When the gateway controller is stopped
    And I undeploy the API from the gateway while it is stopped
    And the gateway controller is started
    Then the gateway stops serving the API

  @restart @recovery
  Scenario: A gateway with an empty store recovers its APIs from the control plane
    When the gateway store is wiped and the gateway controller is restarted
    Then the gateway serves the API

  @restart @multigateway
  Scenario: Restarting one gateway leaves the other serving (restart isolation)
    Given the API is deployed to the second gateway and served
    When I restart the gateway controller
    Then the second gateway still serves the API
    And the gateway serves the API
