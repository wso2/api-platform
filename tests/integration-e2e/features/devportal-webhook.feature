Feature: Credentials issued in the developer portal authorize gateway invocation via webhook
  As an API platform operator running the full product suite
  I want an API key and subscription created in the developer portal to reach the gateway
  through the signed webhook to platform-api
  So that a consumer using developer-portal-issued credentials can invoke the API end to end.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @devportal
  Scenario: A developer-portal API key and subscription propagate to the gateway and authorize a call
    Given a subscription plan "e2e-gold" allowing 10000 requests per hour
    And a published REST API secured with API key and subscription validation offering that plan
    When I deploy the secured API to the gateway
    Then an unauthenticated request to the secured API is rejected
    When the subscription plan is synced to the developer portal
    And the API is published to the developer portal linked to the platform API
    And an application subscribed to the API is created in the developer portal
    And an API key is generated in the developer portal
    Then invoking the secured API through the gateway with the developer portal credentials returns 200
    And invoking the secured API through the gateway without credentials is rejected

  @devportal @lifecycle
  Scenario: Developer-portal credential-lifecycle changes propagate to platform-api and the gateway
    Given a subscription plan "e2e-gold" allowing 10000 requests per hour
    And a published REST API secured with API key and subscription validation offering that plan
    When I deploy the secured API to the gateway
    Then an unauthenticated request to the secured API is rejected
    When the subscription plan is synced to the developer portal
    And a second subscription plan is synced to the developer portal
    And the API is published to the developer portal linked to the platform API
    And an application subscribed to the API is created in the developer portal
    And an API key is generated in the developer portal
    Then invoking the secured API through the gateway with the developer portal credentials returns 200
    # --- API key lifecycle (subscription stays ACTIVE throughout, so a rejection = 401 isolates the key) ---
    # Change the API key expiry (to the past); the gateway must reject the expired key, then serve after restore.
    When the API key is expired in the developer portal
    Then invoking with the expired API key is rejected
    When the API key expiry is restored in the developer portal
    Then invoking the secured API through the gateway with the developer portal credentials returns 200
    # Revoke the API key; the gateway rejects it (401), then re-issue a valid key for the subscription checks below.
    When the API key is revoked in the developer portal
    Then invoking with the revoked API key is unauthorized
    When a new API key is generated in the developer portal
    Then invoking the secured API through the gateway with the developer portal credentials returns 200
    # --- Subscription lifecycle (the API key stays valid throughout, so a rejection = 403 isolates the subscription) ---
    # Change the subscription plan, verify from the platform-api side.
    When the applied subscription plan of the API is switched in the developer portal
    Then platform-api receives the new subscription plan update of the API
    # Regenerate the subscription token; the current token works, then the new token works and the old one is rejected.
    Then invoking with the current subscription token returns 200
    When the subscription token is regenerated in the developer portal
    Then invoking with the new subscription token returns 200
    And invoking with the old subscription token is rejected
    # Pause the subscription (then resume), verify at the gateway.
    When the subscription is paused in the developer portal
    Then invoking the secured API through the gateway is rejected
    When the subscription is resumed in the developer portal
    Then invoking the secured API through the gateway with the developer portal credentials returns 200
    # Remove the subscription (terminal); the gateway stops honoring the credentials.
    When the subscription is removed in the developer portal
    Then invoking the secured API through the gateway is rejected
