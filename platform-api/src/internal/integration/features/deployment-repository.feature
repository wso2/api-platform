Feature: Deployment repository lifecycle across database engines
  As a platform-api maintainer
  I want the deployment repository's create / status / current-lookup / list / undeploy
  paths to behave identically on every engine
  So that the SQL Server-specific deployment SQL (upserts, FETCH-first and the
  ROW_NUMBER ranking) is verified, not just the SQLite unit-test path.

  Background:
    Given a clean platform-api database
    And an organization and project exist
    And a REST API and a gateway exist

  Scenario: Deployment create, current status, current lookup, list and undeploy
    When I create 3 deployments for the API on the gateway
    Then the current deployment status is "DEPLOYED"
    And the API has an active deployment
    And the current deployment for the gateway is the latest one
    And reading the current deployment back returns its content
    And listing deployments with state returns 3
    When I undeploy the current deployment
    Then the current deployment status is "UNDEPLOYED"
    And the API has no active deployment
    And there is no current deployment for the gateway

  Scenario: Creating deployments beyond the hard limit bounds the stored count
    When I create 5 deployments for the API on the gateway with a hard limit of 3
    Then the gateway retains at most 3 deployments
    And the API has an active deployment
