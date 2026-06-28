Feature: Secret repository lifecycle across database engines
  As a platform-api maintainer
  I want the secret store to round-trip encrypted secret material correctly
  So that secret create, existence, paginated list, rotation and soft-delete are verified on every engine.

  Background:
    Given a clean platform-api database
    And an organization and project exist

  Scenario: Secret create, encrypted round-trip, paginated list, rotate and soft-delete
    When I create 5 secrets
    Then reading the first secret back returns its ciphertext and hash
    And the first secret reports type "GENERIC" provider "IN_BUILT" status "ACTIVE"
    And checking existence of the first secret returns true
    And checking existence of a missing secret returns false
    And counting secrets returns 5
    And paging secrets 2 at a time covers all 5 without overlap
    When I rotate the first secret's ciphertext
    Then reading the first secret back returns the rotated ciphertext
    When I soft-delete the first secret
    Then reading the first secret back reports status "DEPRECATED"
