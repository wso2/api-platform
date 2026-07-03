Feature: A secret created in platform-api is resolved by the gateway data plane
  As an API platform operator
  I want a secret created in platform-api to be resolved into deployed artifacts
  So that credentials never leave the control plane in plaintext yet reach the
  data plane at runtime, and survive a gateway restart.

  # Secrets have no live push event: the gateway controller syncs secrets on
  # connect, before deployments, so {{ secret "..." }} placeholders resolve
  # (c.syncOnce -> syncSecrets -> syncDeployments in pkg/controlplane/client.go).
  # These scenarios therefore restart the controller to pick the secret up, then
  # assert the resolved value reaches the upstream. postgres-only (@secret).

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @smoke @secret
  Scenario: A gateway resolves a platform-api secret into an upstream request header
    Given a secret in platform-api
    And a REST API that injects the secret into an upstream request header
    When I deploy the API to the gateway
    And I restart the gateway controller
    Then the gateway injects the resolved secret value into the upstream request

  @secret @restart
  Scenario: The resolved secret survives a full gateway restart
    Given a secret in platform-api
    And a REST API that injects the secret into an upstream request header
    And I deploy the API to the gateway
    And I restart the gateway controller
    And the gateway injects the resolved secret value into the upstream request
    When I restart the whole gateway
    Then the gateway injects the resolved secret value into the upstream request

  # Guards against the demo-mode ephemeral-key bug: the control plane must be able
  # to re-serve the stored secret after IT restarts (requires a stable
  # PLATFORM_SECRET_ENCRYPTION_KEY — set in docker-compose.yaml). The final wipe +
  # controller restart forces the gateway to re-fetch the secret from the
  # just-restarted control plane, so a wrong/undecryptable value would fail here.
  @secret @restart @cp-restart
  Scenario: The secret is still re-fetchable after the control plane restarts
    Given a secret in platform-api
    And a REST API that injects the secret into an upstream request header
    And I deploy the API to the gateway
    And I restart the gateway controller
    And the gateway injects the resolved secret value into the upstream request
    When I restart the platform-api control plane
    And the gateway store is wiped and the gateway controller is restarted
    Then the gateway injects the resolved secret value into the upstream request
