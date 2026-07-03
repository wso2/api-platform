Feature: A secret is resolved into a deployed LLM provider's upstream auth
  As an API platform operator
  I want a secret created in platform-api to be injected into an LLM provider's
  upstream authorization header when the provider is deployed to a gateway
  So that provider credentials reach the LLM backend at runtime, and keep working
  after the gateway restarts.

  # An LLM provider carries its upstream credential first-class in
  # upstream.main.auth.value (unlike a REST API, which uses a set-headers policy).
  # Secrets have no live push: the controller syncs secrets on connect, before
  # deployments, so {{ secret "..." }} resolves (c.syncOnce in client.go). The
  # scenarios therefore restart the controller once after deploy to sync the
  # secret, then invoke <context>/chat/completions and assert the resolved
  # credential reached the upstream (the sample backend echoes request headers).
  # postgres-only (@llm).
  #
  # @wip — QUARANTINED pending a platform-api bug (excluded from default runs;
  # run explicitly with E2E_TAGS=@llm to reproduce). Deploying a platform-api LLM
  # provider that uses a default template (e.g. openai) FAILS gateway validation:
  #   "LLM provider validation failed: version: Version must be
  #    'gateway.api-platform.wso2.com/v1'"  (gateway-controller llm_validator.go)
  # Root cause: platform-api templates carry no apiVersion — the default template
  # files (default-llm-provider-templates/*.yaml) omit it, the loader
  # (llm_provider_template_loader.go) reads apiVersion but never stores it on the
  # model, and the provider deploy YAML sends only the template handle. So the
  # template the gateway validates has an empty apiVersion. Secret resolution
  # itself works (the secret is retrieved, decrypted and ready to inject). Remove
  # @wip once platform-api propagates the template apiVersion to the gateway.

  Background:
    Given the platform-api control plane and gateway data plane are running
    And I am authenticated to platform-api

  @wip @smoke @llm @secret
  Scenario: A gateway resolves a secret into a deployed LLM provider's upstream auth
    Given a secret in platform-api
    And an LLM provider whose upstream authorization uses the secret
    When I deploy the LLM provider to the gateway
    And I restart the gateway controller
    Then invoking the LLM provider sends the resolved secret upstream

  @wip @llm @secret @restart
  Scenario: The LLM provider's resolved secret survives a full gateway restart
    Given a secret in platform-api
    And an LLM provider whose upstream authorization uses the secret
    And I deploy the LLM provider to the gateway
    And I restart the gateway controller
    And invoking the LLM provider sends the resolved secret upstream
    When I restart the whole gateway
    Then invoking the LLM provider sends the resolved secret upstream
