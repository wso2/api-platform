Feature: An LLM proxy deployed from platform-api is served by the gateway
  As an API platform operator
  I want an LLM proxy (over a provider) created in platform-api to be served by
  the gateway data plane
  So that the control-plane → data-plane path works for LLM proxies too.

  # @wip — QUARANTINED, excluded from default runs (E2E_TAGS=@llm-proxy to
  # reproduce). An LLM proxy references an LLM provider by handle; deploying the
  # proxy requires the referenced provider AND its template to be deployed on the
  # gateway first. That provider deploy currently FAILS the gateway's
  # template-apiVersion validation (the same open platform-api bug as
  # features/llm-provider-secret.feature — the default LLM provider templates
  # carry no apiVersion). So the LLM-proxy path is transitively blocked by the
  # same bug. This scenario captures the intended flow; remove @wip once the
  # provider template apiVersion is propagated to the gateway.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @wip @llm-proxy @secret
  Scenario: An LLM proxy forwards the provider's resolved secret upstream
    Given a secret in platform-api
    And an LLM provider whose upstream authorization uses the secret
    And I deploy the LLM provider to the gateway
    And I restart the gateway controller
    And an LLM proxy over that provider
    When I deploy the LLM proxy to the gateway
    And I restart the gateway controller
    Then invoking the LLM proxy sends the resolved secret upstream
