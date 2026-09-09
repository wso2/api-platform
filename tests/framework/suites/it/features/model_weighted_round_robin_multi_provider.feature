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

@model-weighted-round-robin-multi-provider
Feature: Model weighted round-robin multi-provider routing
  Test the model-weighted-round-robin policy when weighted targets select both a model
  and an additional LLM provider. Weights build a repeating sequence; each entry may name
  a provider. Targets without a "provider" use the LlmProxy primary provider; targets
  naming a provider are routed to that provider's named upstream.

  Each provider forwards to a distinct upstream path on the same echo backend, and the
  echo backend reflects the request path and body, so the echoed upstream path proves which
  provider was selected and the echoed model proves the model rewrite.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Weighted sequence distributes across providers by weight
    Given I generate a unique resource name from "mpw-rotate-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mpw-rotate-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mpw-rotate-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mpw-rotate-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mpw-rotate-anthropic" and store it as "anthropicProviderName"
    And I generate a unique value from "mpw-rotate-anthropic" and store it as "anthropicProviderDisplayName"
    And I generate a unique API version from "mpw-rotate-anthropic" and store it as "anthropicProviderVersion"
    And I generate a unique API context from "/mpw-rotate-anthropic" and store it as "anthropicProviderContext"
    And I generate a unique resource name from "mpw-rotate-bedrock" and store it as "bedrockProviderName"
    And I generate a unique value from "mpw-rotate-bedrock" and store it as "bedrockProviderDisplayName"
    And I generate a unique API version from "mpw-rotate-bedrock" and store it as "bedrockProviderVersion"
    And I generate a unique API context from "/mpw-rotate-bedrock" and store it as "bedrockProviderContext"
    And I generate a unique resource name from "mpw-rotate-proxy" and store it as "proxyName"
    And I generate a unique value from "mpw-rotate-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mpw-rotate-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mpw-rotate-proxy" and store it as "proxyContext"
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

    # Weights 2 / 1 / 1 -> sequence: gpt-4o, gpt-4o, claude, bedrock
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:proxyName}                  |
      | displayName              | ${CTX:proxyDisplayName}           |
      | version                  | ${CTX:proxyVersion}               |
      | context                  | ${CTX:proxyContext}               |
      | provider.id              | ${CTX:primaryProviderName}         |
      | spec.additionalProviders | [{"id":"${CTX:anthropicProviderName}","as":"anthropic-provider"},{"id":"${CTX:bedrockProviderName}","as":"bedrock-provider"}] |
      | spec.operationPolicies   | [{"name":"model-weighted-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"gpt-4o","weight":2},{"model":"claude-sonnet-4-5-20250929","provider":"anthropic-provider","weight":1},{"model":"bedrock-claude-3-5-sonnet","provider":"bedrock-provider","weight":1}],"suspendDuration":60}}]}] |
    Then the response status code should be 201

    # Request 1 -> gpt-4o on the primary provider (weight slot 1 of 2)
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"

    # Request 2 -> gpt-4o again on the primary provider (weight slot 2 of 2)
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"

    # Request 3 -> anthropic-provider
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/anthropic-upstream"
    And the response body should contain "claude-sonnet-4-5-20250929"

    # Request 4 -> bedrock-provider
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/bedrock-upstream"
    And the response body should contain "bedrock-claude-3-5-sonnet"

    # Request 5 -> sequence wraps back to gpt-4o on the primary provider
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

  Scenario: Weighted same model name on two providers is suspended independently
    Given I generate a unique resource name from "mpw-susp-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mpw-susp-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mpw-susp-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mpw-susp-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mpw-susp-alt" and store it as "altProviderName"
    And I generate a unique value from "mpw-susp-alt" and store it as "altProviderDisplayName"
    And I generate a unique API version from "mpw-susp-alt" and store it as "altProviderVersion"
    And I generate a unique API context from "/mpw-susp-alt" and store it as "altProviderContext"
    And I generate a unique resource name from "mpw-susp-proxy" and store it as "proxyName"
    And I generate a unique value from "mpw-susp-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mpw-susp-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mpw-susp-proxy" and store it as "proxyContext"
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

    # Equal weights, SAME model name, different providers
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:proxyName}                  |
      | displayName              | ${CTX:proxyDisplayName}           |
      | version                  | ${CTX:proxyVersion}               |
      | context                  | ${CTX:proxyContext}               |
      | provider.id              | ${CTX:primaryProviderName}         |
      | spec.additionalProviders | [{"id":"${CTX:altProviderName}","as":"alt-provider"}] |
      | spec.operationPolicies   | [{"name":"model-weighted-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"shared-model","weight":1},{"model":"shared-model","provider":"alt-provider","weight":1}],"suspendDuration":30}}]}] |
    Then the response status code should be 201

    # Request 1 -> primary/shared-model, healthy
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response body should contain "/primary-upstream"

    # Request 2 -> alt-provider/shared-model, forced 500 suspends ONLY that pair
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions?statusCode=500" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 500

    # Request 3 -> primary/shared-model still serves
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"original-model","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"

    # Request 4 -> alt pair still suspended, primary serves again
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

  Scenario: Weighted targets without a provider all use the primary provider
    Given I generate a unique resource name from "mpw-default-primary" and store it as "primaryProviderName"
    And I generate a unique value from "mpw-default-primary" and store it as "primaryProviderDisplayName"
    And I generate a unique API version from "mpw-default-primary" and store it as "primaryProviderVersion"
    And I generate a unique API context from "/mpw-default-primary" and store it as "primaryProviderContext"
    And I generate a unique resource name from "mpw-default-extra" and store it as "extraProviderName"
    And I generate a unique value from "mpw-default-extra" and store it as "extraProviderDisplayName"
    And I generate a unique API version from "mpw-default-extra" and store it as "extraProviderVersion"
    And I generate a unique API context from "/mpw-default-extra" and store it as "extraProviderContext"
    And I generate a unique resource name from "mpw-default-proxy" and store it as "proxyName"
    And I generate a unique value from "mpw-default-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "mpw-default-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/mpw-default-proxy" and store it as "proxyContext"
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
      | spec.operationPolicies   | [{"name":"model-weighted-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"model-one","weight":2},{"model":"model-two","weight":1}]}}]}] |
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
    And the response body should contain "model-one"

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
