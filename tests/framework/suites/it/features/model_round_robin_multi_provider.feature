# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@model-round-robin-multi-provider
Feature: Model round-robin multi-provider routing
  Test the model-round-robin policy when round-robin targets select both a model and an
  additional LLM provider. Targets without a "provider" use the LlmProxy primary provider;
  targets naming a provider are routed to that provider's named upstream and carry
  provider-scoped auth so conditional provider auth applies only to the matching target.

  Each provider forwards to a distinct upstream path on the same echo backend, and the
  echo backend reflects the request path and body, so the echoed upstream path proves which
  provider was selected and the echoed model proves the model rewrite.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Round-robin rotates across primary and two additional providers
    Given I generate a unique resource name from "mprr-rotate-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mprr-rotate-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mprr-rotate-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mprr-rotate-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mprr-rotate-anthropic" and store it as "anthropicProviderName"
    And I generate a unique value from "mprr-rotate-anthropic" and store it as "anthropicProviderDisplayName"
    And I generate a unique API version from "mprr-rotate-anthropic" and store it as "anthropicProviderVersion"
    And I generate a unique API context from "/mprr-rotate-anthropic" and store it as "anthropicProviderContext"
    And I generate a unique resource name from "mprr-rotate-bedrock" and store it as "bedrockProviderName"
    And I generate a unique value from "mprr-rotate-bedrock" and store it as "bedrockProviderDisplayName"
    And I generate a unique API version from "mprr-rotate-bedrock" and store it as "bedrockProviderVersion"
    And I generate a unique API context from "/mprr-rotate-bedrock" and store it as "bedrockProviderContext"
    And I generate a unique resource name from "mprr-rotate-proxy" and store it as "proxyName"
    And I generate a unique value from "mprr-rotate-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mprr-rotate-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mprr-rotate-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:primaryProviderName}         |
      | displayName        | ${CTX:primaryProviderDisplayName}  |
      | version            | ${CTX:primaryProviderVersion}      |
      | template           | openai                              |
      | spec.context       | ${CTX:primaryProviderContext}      |
      | spec.upstream.url  | http://testbench:3002/primary-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:anthropicProviderName}       |
      | displayName        | ${CTX:anthropicProviderDisplayName} |
      | version            | ${CTX:anthropicProviderVersion}    |
      | template           | openai                              |
      | spec.context       | ${CTX:anthropicProviderContext}    |
      | spec.upstream.url  | http://testbench:3002/anthropic-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:bedrockProviderName}         |
      | displayName        | ${CTX:bedrockProviderDisplayName}  |
      | version            | ${CTX:bedrockProviderVersion}      |
      | template           | openai                              |
      | spec.context       | ${CTX:bedrockProviderContext}      |
      | spec.upstream.url  | http://testbench:3002/bedrock-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:proxyName}                  |
      | displayName              | ${CTX:proxyDisplayName}           |
      | version                  | ${CTX:proxyVersion}               |
      | context                  | ${CTX:proxyContext}               |
      | provider.id              | ${CTX:primaryProviderName}         |
      | spec.additionalProviders | [{"id":"${CTX:anthropicProviderName}","as":"anthropic-provider","auth":{"type":"api-key","header":"X-Round-Robin-Provider","value":"anthropic-round-robin-secret"}},{"id":"${CTX:bedrockProviderName}","as":"bedrock-provider"}] |
      | spec.operationPolicies   | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"gpt-4o"},{"model":"claude-sonnet-4-5-20250929","provider":"anthropic-provider"},{"model":"bedrock-claude-3-5-sonnet","provider":"bedrock-provider"}],"suspendDuration":60}}]}] |
    Then the response status code should be 201

    # Request 1 -> primary provider (no provider on target), model gpt-4o
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"
    And the response body should not contain "anthropic-round-robin-secret"

    # Request 2 -> anthropic-provider upstream
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/anthropic-upstream"
    And the response body should contain "claude-sonnet-4-5-20250929"
    And the response body should contain "anthropic-round-robin-secret"

    # Request 3 -> bedrock-provider upstream
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/bedrock-upstream"
    And the response body should contain "bedrock-claude-3-5-sonnet"

    # Request 4 -> cycles back to the primary provider
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:primaryProviderName}"
    Then the response status code should be 200
    When I delete the LLM provider "${CTX:anthropicProviderName}"
    Then the response status code should be 200
    When I delete the LLM provider "${CTX:bedrockProviderName}"
    Then the response status code should be 200

  Scenario: Same model name on two providers is suspended independently
    Given I generate a unique resource name from "mprr-susp-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mprr-susp-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mprr-susp-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mprr-susp-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mprr-susp-alt" and store it as "altProviderName"
    And I generate a unique value from "mprr-susp-alt" and store it as "altProviderDisplayName"
    And I generate a unique API version from "mprr-susp-alt" and store it as "altProviderVersion"
    And I generate a unique API context from "/mprr-susp-alt" and store it as "altProviderContext"
    And I generate a unique resource name from "mprr-susp-proxy" and store it as "proxyName"
    And I generate a unique value from "mprr-susp-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mprr-susp-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mprr-susp-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:primaryProviderName}         |
      | displayName        | ${CTX:primaryProviderDisplayName}  |
      | version            | ${CTX:primaryProviderVersion}      |
      | template           | openai                              |
      | spec.context       | ${CTX:primaryProviderContext}      |
      | spec.upstream.url  | http://testbench:3002/primary-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:altProviderName}             |
      | displayName        | ${CTX:altProviderDisplayName}      |
      | version            | ${CTX:altProviderVersion}          |
      | template           | openai                              |
      | spec.context       | ${CTX:altProviderContext}          |
      | spec.upstream.url  | http://testbench:3002/alt-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    # Both targets use the SAME model name, on different providers
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:proxyName}                  |
      | displayName              | ${CTX:proxyDisplayName}           |
      | version                  | ${CTX:proxyVersion}               |
      | context                  | ${CTX:proxyContext}               |
      | provider.id              | ${CTX:primaryProviderName}         |
      | spec.additionalProviders | [{"id":"${CTX:altProviderName}","as":"alt-provider"}] |
      | spec.operationPolicies   | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"shared-model"},{"model":"shared-model","provider":"alt-provider"}],"suspendDuration":30}}]}] |
    Then the response status code should be 201

    # Request 1 -> primary/shared-model, healthy
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response body should contain "/primary-upstream"

    # Request 2 -> alt-provider/shared-model, forced 429 suspends ONLY that pair
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions?statusCode=429" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429

    # Request 3 -> primary/shared-model is NOT suspended and still serves
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"

    # Request 4 -> alt pair still suspended, so primary serves again
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should not contain "/alt-upstream"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:primaryProviderName}"
    Then the response status code should be 200
    When I delete the LLM provider "${CTX:altProviderName}"
    Then the response status code should be 200

  Scenario: Targets without a provider all use the primary provider
    Given I generate a unique resource name from "mprr-default-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mprr-default-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mprr-default-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mprr-default-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mprr-default-extra" and store it as "extraProviderName"
    And I generate a unique value from "mprr-default-extra" and store it as "extraProviderDisplayName"
    And I generate a unique API version from "mprr-default-extra" and store it as "extraProviderVersion"
    And I generate a unique API context from "/mprr-default-extra" and store it as "extraProviderContext"
    And I generate a unique resource name from "mprr-default-proxy" and store it as "proxyName"
    And I generate a unique value from "mprr-default-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mprr-default-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mprr-default-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:primaryProviderName}         |
      | displayName        | ${CTX:primaryProviderDisplayName}  |
      | version            | ${CTX:primaryProviderVersion}      |
      | template           | openai                              |
      | spec.context       | ${CTX:primaryProviderContext}      |
      | spec.upstream.url  | http://testbench:3002/primary-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:extraProviderName}           |
      | displayName        | ${CTX:extraProviderDisplayName}    |
      | version            | ${CTX:extraProviderVersion}        |
      | template           | openai                              |
      | spec.context       | ${CTX:extraProviderContext}        |
      | spec.upstream.url  | http://testbench:3002/extra-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    # An additional provider is declared but NO target references it
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:proxyName}                  |
      | displayName              | ${CTX:proxyDisplayName}           |
      | version                  | ${CTX:proxyVersion}               |
      | context                  | ${CTX:proxyContext}               |
      | provider.id              | ${CTX:primaryProviderName}         |
      | spec.additionalProviders | [{"id":"${CTX:extraProviderName}","as":"extra-provider"}] |
      | spec.operationPolicies   | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"model-one"},{"model":"model-two"}]}}]}] |
    Then the response status code should be 201

    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response body should contain "/primary-upstream"
    And the response body should contain "model-one"
    And the response body should not contain "/extra-upstream"

    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "model-two"
    And the response body should not contain "/extra-upstream"

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:primaryProviderName}"
    Then the response status code should be 200
    When I delete the LLM provider "${CTX:extraProviderName}"
    Then the response status code should be 200

  Scenario: Proxy referencing a non-existent additional provider is rejected
    Given I generate a unique resource name from "mprr-invalid-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mprr-invalid-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mprr-invalid-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mprr-invalid-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mprr-invalid-proxy" and store it as "proxyName"
    And I generate a unique value from "mprr-invalid-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mprr-invalid-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mprr-invalid-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:primaryProviderName}         |
      | displayName        | ${CTX:primaryProviderDisplayName}  |
      | version            | ${CTX:primaryProviderVersion}      |
      | template           | openai                              |
      | spec.context       | ${CTX:primaryProviderContext}      |
      | spec.upstream.url  | http://testbench:3002/primary-upstream |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:proxyName}                  |
      | displayName              | ${CTX:proxyDisplayName}           |
      | version                  | ${CTX:proxyVersion}               |
      | context                  | ${CTX:proxyContext}               |
      | provider.id              | ${CTX:primaryProviderName}         |
      | spec.additionalProviders | [{"id":"does-not-exist-provider","as":"ghost-provider"}] |
      | spec.operationPolicies   | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"gpt-4o"},{"model":"ghost-model","provider":"ghost-provider"}]}}]}] |
    Then the response should be a client error

    When I delete the LLM provider "${CTX:primaryProviderName}"
    Then the response status code should be 200
