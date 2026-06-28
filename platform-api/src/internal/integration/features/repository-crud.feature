Feature: Repository CRUD lifecycle across database engines
  As a platform-api maintainer
  I want the REST API, gateway and API key repositories to round-trip correctly
  So that backend-specific bugs are caught on every supported database engine.

  Background:
    Given a clean platform-api database
    And an organization and project exist

  Scenario: REST API create, read, update and list
    When I create 3 REST APIs in the project
    Then reading the first REST API back returns lifecycle status "CREATED"
    And the first REST API handle is reported as existing
    And a random handle is reported as not existing
    When I update the first REST API to status "PUBLISHED"
    Then reading the first REST API back shows the updated name, status "PUBLISHED" and updater "it-user"
    And listing REST APIs by organization returns 3
    And listing REST APIs by project returns 3
    And listing REST APIs by organization filtered to another project returns 0

  Scenario: Gateway create, list, and registration token lifecycle
    When I create 3 gateways
    Then reading the first gateway back returns it as inactive with property region "us"
    And the first gateway is found by its handle
    And listing gateways by organization returns 3
    When I generate a registration token for the first gateway
    Then the first gateway has 1 active token
    And the token is found by its hash
    When I revoke the token
    Then the first gateway has 0 active tokens
    And the token is no longer found by its hash
    And the revoked token records status "revoked" by "it-user"

  Scenario: API key create, list, update and revoke
    Given a REST API exists to back the API keys
    When I create 2 API keys on the REST API
    Then reading the first API key back returns status "active" and target "ALL"
    And listing API keys by artifact returns 2
    When I update the first API key material
    Then reading the first API key back shows the updated masked key
    When I revoke the first API key
    Then reading the first API key back returns status "revoked"
