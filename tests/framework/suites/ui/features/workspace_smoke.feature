Feature: AI Workspace is reachable
  The smallest true statement about the UI stack: a user pointing a browser at the
  workspace sees the sign-in form. Everything else builds on this.

  Scenario: The sign-in page renders
    When the user opens the workspace
    Then the user sees the sign-in form
