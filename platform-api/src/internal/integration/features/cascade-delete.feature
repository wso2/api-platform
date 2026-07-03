Feature: Foreign-key cascade deletes across database engines
  As a platform-api maintainer
  I want deletes to cascade through the kept foreign-key edges
  So that the SQL Server NO ACTION rework still cleans up dependent rows on every engine.

  Background:
    Given a clean platform-api database

  Scenario: Deleting a REST API cascades to its subscriptions
    Given a seeded organization object graph
    When I delete the REST API artifact
    Then the subscription is removed

  Scenario: Deleting a gateway cascades to its deployments and deployment status
    Given a seeded organization object graph
    When I delete the gateway
    Then the deployment is removed
    And the deployment status is removed

  Scenario: Deleting an application cascades to its key and artifact mappings
    Given a seeded organization object graph
    When I delete the application
    Then the application api key mapping is removed
    And the application artifact mapping is removed

  Scenario: Deleting a project retains its applications
    Given a seeded organization object graph
    When I delete the REST API artifact
    And I delete the project
    Then the application is retained

  Scenario: Deleting a subscription plan cascades to its limits
    Given a seeded organization object graph
    When I delete the subscription and its plan
    Then the subscription plan limit is removed
