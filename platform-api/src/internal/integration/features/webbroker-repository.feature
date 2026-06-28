Feature: WebBroker API repository lifecycle across database engines
  As a platform-api maintainer
  I want the WebBroker API repository to round-trip correctly
  So that the event-broker API store is verified on every engine (create, paginate, update, delete).

  Background:
    Given a clean platform-api database
    And an organization and project exist

  Scenario: WebBroker API create, read, update, paginated list and delete
    When I create 4 WebBroker APIs in the project
    Then reading the first WebBroker API back returns lifecycle status "CREATED"
    And paging WebBroker APIs 2 at a time covers all 4 without overlap
    And listing WebBroker APIs by project returns 4
    When I update the first WebBroker API to status "PUBLISHED"
    Then reading the first WebBroker API back returns lifecycle status "PUBLISHED"
    When I delete the first WebBroker API
    Then listing WebBroker APIs by project returns 3
