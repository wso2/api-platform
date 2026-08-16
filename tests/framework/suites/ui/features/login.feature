Feature: Signing in
  The sign-in form is the one screen every user meets. It gets a real, through-the-UI
  scenario here; every other feature starts from a saved signed-in state instead, so the
  form is exercised deliberately rather than incidentally five hundred times.

  Scenario: An administrator signs in through the form
    When the user opens the workspace
    And the user signs in as the administrator
    Then the user lands on the organization home

  Scenario: A signed-in session is reusable without the form
    Given the user is signed in
    Then the user lands on the organization home
