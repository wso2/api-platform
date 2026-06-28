Feature: LLM control-plane repository lifecycle across database engines
  As a platform-api maintainer
  I want the LLM provider-template, provider and proxy repositories to round-trip correctly
  So that the AI control plane is verified on every engine — with no real LLM and no real API key.

  Background:
    Given a clean platform-api database
    And an organization and project exist

  Scenario: LLM provider template create, new version and list
    When I create an LLM provider template "openai" version "v1.0"
    Then reading the template by its handle returns version "v1.0"
    When I create a new version "v2.0" of the template
    Then reading the original template handle still returns version "v1.0"
    And the latest version of the template family is "v2.0"
    And listing template versions returns 2

  Scenario: LLM provider create, read, update and delete with a dummy upstream key
    Given an LLM provider template "openai" version "v1.0"
    When I create an LLM provider "my-openai" with upstream key "Bearer sk-test-key"
    Then reading the provider back returns upstream key "Bearer sk-test-key"
    And listing LLM providers by organization returns 1
    When I update the provider description to "updated by integration test"
    Then reading the provider back shows description "updated by integration test"
    When I delete the provider
    Then listing LLM providers by organization returns 0

  Scenario: LLM proxy create, list by provider and project, and delete
    Given an LLM provider template "openai" version "v1.0"
    And an LLM provider "my-openai" with upstream key "Bearer sk-test-key"
    When I create an LLM proxy "my-proxy" for that provider
    Then reading the proxy back references the provider
    And listing LLM proxies by organization returns 1
    And listing LLM proxies by project returns 1
    And listing LLM proxies by provider returns 1
    When I delete the proxy
    Then listing LLM proxies by organization returns 0
