Feature: LLM provider and app LLM proxy lifecycle
  The journey ported from the product's own Cypress suite (001-provider-and-proxy), driven
  entirely through the UI against the real control plane and the block's real gateway.
  Every assertion is something the user SEES — a page reached, a deployment turning
  Active, the model's answer coming back — never a database row or a config dump.

  The provider comes from the built-in Azure AI Foundry template: the built-in that leaves
  the upstream URL to the user (here the block's mock LLM). Built-ins are the only
  templates a real gateway can serve — workspace-defined templates never reach it, so
  providers built from them fail to deploy.

  Scenario: An administrator publishes a provider and proxy, invokes the LLM through the gateway, then retires the proxy
    Given the user is signed in

    When the user creates a project named "E2E Project"
    Then the user sees "E2E Project" among the projects

    When the user starts adding a provider from the "Azure AI Foundry" template
    And the user creates the provider "E2E OpenAI Provider" pointed at the mock LLM
    Then the user is on the provider's overview page
    And the user sees "E2E OpenAI Provider" on the page

    When the user deploys it to the gateway
    Then the user sees the deployment is active
    When the user returns to the provider overview
    And the user generates an API key named "e2e-provider-key"

    When the user creates an app LLM proxy "E2E OpenAI Proxy" in project "E2E Project" using that key
    Then the user is on the proxy's overview page
    And the user sees "E2E OpenAI Proxy" on the page

    When the user deploys it to the gateway
    Then the user sees the deployment is active
    When the user returns to the proxy overview
    And the user generates an API key named "e2e-proxy-key"
    And the user invokes the proxy's chat completions endpoint with that key
    Then the completion answers "Hello! How can I assist you today?"

    When the user deletes the proxy
    Then the user is back on the proxy list
    And the user no longer sees "E2E OpenAI Proxy"
