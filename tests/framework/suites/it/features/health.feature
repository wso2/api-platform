@health
Feature: Gateway Health Check
  As an operator
  I want to verify that gateway services are healthy
  So that I can ensure the gateway is operational

  # Migrated with its scenarios unchanged. What changed is underneath: the legacy steps reached
  # the three endpoints at fixed host ports, and here each is resolved from the running
  # topology, because this framework publishes ports dynamically so blocks can run at once.
  #
  # The three are deliberately separate rather than collapsed into one "is the gateway up"
  # check. They are three processes and they can disagree — a controller answering while the
  # router is not yet accepting traffic is exactly the state worth catching, and a single
  # aggregate check would hide it.

  Background:
    Given the gateway services are running

  Scenario: Gateway controller admin health endpoint returns OK
    When I send a GET request to the gateway controller admin health endpoint
    Then the response status code should be 200
    And the response should indicate healthy status

  Scenario: Router is ready to accept traffic
    When I send a GET request to the router ready endpoint
    Then the response status code should be 200

  Scenario: All gateway services are healthy
    When I check the health of all gateway services
    Then all services should report healthy status

  # ==================== HEALTH ENDPOINT RESPONSE VALIDATION ====================

  Scenario: Gateway controller admin health endpoint returns valid JSON
    When I send a GET request to the gateway controller admin health endpoint
    Then the response status code should be 200
    And the response should be valid JSON

  # ==================== POLICY ENGINE HEALTH ====================

  Scenario: Policy engine health endpoint returns OK
    When I send a GET request to the policy engine health endpoint
    Then the response status code should be 200
    And the response should indicate healthy status

  Scenario: Policy engine health endpoint returns valid JSON
    When I send a GET request to the policy engine health endpoint
    Then the response status code should be 200
    And the response should be valid JSON
