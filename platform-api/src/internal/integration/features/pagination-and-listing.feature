Feature: Pagination and filtered listing across database engines
  As a platform-api maintainer
  I want paginated and filtered repository queries to behave identically on every engine
  So that the SQL Server LIMIT/OFFSET rework (PaginationClause / FetchFirstClause) is verified.

  Background:
    Given a clean platform-api database

  Scenario: Organization pagination pages through all rows without overlap
    Given 5 additional organizations exist
    When I page through organizations 2 at a time
    Then every organization is seen exactly once

  Scenario: Subscription plan existence check and paginated list
    Given an organization exists
    Then checking existence of a missing subscription plan returns false
    When I create 3 subscription plans with a throttle limit of 5 per "min"
    Then listing subscription plans 2 at a time returns 2
    And each listed plan round-trips its throttle limit
    And reading the first listed plan back round-trips its throttle limit
    When I clear the throttle limit on the first listed plan
    Then reading it back shows no throttle limit

  Scenario: Project pagination pages through all rows without overlap
    Given an organization exists
    And 5 projects exist
    When I page through projects 2 at a time
    Then every project is seen exactly once

  Scenario: Subscription listing filters by status
    Given a seeded organization object graph
    Then listing subscriptions with no filter returns 1
    And listing subscriptions filtered by status "ACTIVE" returns 1
    And listing subscriptions filtered by status "REVOKED" returns 0

  Scenario: Application lookup by UUID or handle
    Given an organization, project and application exist
    Then the application is found by its UUID
    And the application is found by its handle
    And a missing application identifier returns nothing
