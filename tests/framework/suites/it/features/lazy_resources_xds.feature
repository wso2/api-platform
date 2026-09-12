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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@lazy-resources-xds
Feature: Lazy resources xDS synchronization
  As an API platform operator
  I want lazy resources to be synchronized to the policy engine
  So that deployed provider configuration is available to runtime services

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: LLM provider template is synchronized to policy engine via xDS
    Given I generate a unique resource name from "xds-template" and store it as "resourceName1_1"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1                         |
      | name                       | ${CTX:resourceName1_1}                                  |
      | displayName                | xDS Test Template                                        |
      | spec.promptTokens           | {"location":"payload","identifier":"$.usage.prompt_tokens"} |
      | spec.completionTokens       | {"location":"payload","identifier":"$.usage.completion_tokens"} |
      | spec.totalTokens            | {"location":"payload","identifier":"$.usage.total_tokens"} |
      | spec.remainingTokens        | {"location":"header","identifier":"x-ratelimit-remaining-tokens"} |
      | spec.requestModel           | {"location":"payload","identifier":"$.model"}      |
      | spec.responseModel          | {"location":"payload","identifier":"$.model"}      |
    Then the response status code should be 201
    And the JSON response field "status.id" should be "${CTX:resourceName1_1}"
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:resourceName1_1}" has display name "xDS Test Template"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "lazy_resources.total_resources" should be greater than 0
    And the lazy resources should contain template "${CTX:resourceName1_1}" of type "LlmProviderTemplate"
    When I delete the LLM provider template "${CTX:resourceName1_1}"
    Then the response status code should be 200

  Scenario: OOB templates are available in policy engine lazy resources
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the response should be valid JSON
    And the lazy resources should contain template "openai" of type "LlmProviderTemplate"
    And the lazy resources should contain template "anthropic" of type "LlmProviderTemplate"
    And the lazy resources should contain template "gemini" of type "LlmProviderTemplate"

  Scenario: Updated template is reflected in policy engine lazy resources
    Given I generate a unique resource name from "update-template" and store it as "lazyName3_1"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion   | gateway.api-platform.wso2.com/v1 |
      | name         | ${CTX:lazyName3_1}              |
      | displayName  | Original Display Name            |
      | spec.promptTokens | {"location":"payload","identifier":"$.usage.prompt_tokens"} |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:lazyName3_1}" has display name "Original Display Name"
    Then the response status code should be 200
    And the lazy resources should contain template "${CTX:lazyName3_1}" of type "LlmProviderTemplate"
    And the lazy resource "${CTX:lazyName3_1}" should have display name "Original Display Name"
    When I update LLM provider template "${CTX:lazyName3_1}" from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion   | gateway.api-platform.wso2.com/v1 |
      | name         | ${CTX:lazyName3_1}              |
      | displayName  | Updated Display Name             |
      | spec.promptTokens | {"location":"payload","identifier":"$.usage.prompt_tokens"} |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:lazyName3_1}" has display name "Updated Display Name"
    Then the response status code should be 200
    And the lazy resource "${CTX:lazyName3_1}" should have display name "Updated Display Name"
    When I delete the LLM provider template "${CTX:lazyName3_1}"
    Then the response status code should be 200

  Scenario: Deleted template is removed from policy engine lazy resources
    Given I generate a unique resource name from "delete-template" and store it as "resourceName4_1"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:resourceName4_1}           |
      | displayName | Delete Test Template             |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:resourceName4_1}" has display name "Delete Test Template"
    Then the response status code should be 200
    And the lazy resources should contain template "${CTX:resourceName4_1}" of type "LlmProviderTemplate"
    When I delete the LLM provider template "${CTX:resourceName4_1}"
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:resourceName4_1}" of type "LlmProviderTemplate" is absent
    Then the response status code should be 200
    And the lazy resources should not contain template "${CTX:resourceName4_1}"

  Scenario: LLM provider creation creates a provider-template mapping
    Given I generate a unique resource name from "provider" and store it as "resourceName5_1"
    And I generate a unique API context from "/lazy-provider" and store it as "resourceContext5_1"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName5_1}           |
      | displayName        | Test OpenAI Provider              |
      | version            | v1.0                              |
      | template           | openai                            |
      | spec.context       | ${CTX:resourceContext5_1}        |
      | spec.upstream.url  | https://api.openai.com            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    And the JSON response field "status.id" should be "${CTX:resourceName5_1}"
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until provider template mapping "${CTX:resourceName5_1}" maps to template "openai"
    Then the response status code should be 200
    And the lazy resources should contain resource "${CTX:resourceName5_1}" of type "ProviderTemplateMapping"
    And the provider template mapping "${CTX:resourceName5_1}" should map to template "openai"
    When I delete the LLM provider "${CTX:resourceName5_1}"
    Then the response status code should be 200

  Scenario: LLM provider update changes its provider-template mapping
    Given I generate a unique resource name from "provider-template" and store it as "resourceName6_1"
    And I generate a unique resource name from "provider-update" and store it as "resourceName6_2"
    And I generate a unique API context from "/lazy-provider-update" and store it as "resourceContext6_2"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:resourceName6_1}           |
      | displayName       | Update Mapping Provider          |
      | version           | v1.0                              |
      | spec.upstream.url      | https://api.openai.com            |
      | spec.accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName6_2}           |
      | displayName        | Update Mapping Provider            |
      | version            | v1.0                              |
      | template           | openai                            |
      | spec.context       | ${CTX:resourceContext6_2}        |
      | spec.upstream.url  | https://api.openai.com            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until provider template mapping "${CTX:resourceName6_2}" maps to template "openai"
    Then the response status code should be 200
    And the lazy resources should contain resource "${CTX:resourceName6_2}" of type "ProviderTemplateMapping"
    When I update LLM provider "${CTX:resourceName6_2}" from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName6_2}           |
      | displayName        | Update Mapping Provider            |
      | version            | v1.0                              |
      | template           | ${CTX:resourceName6_1}            |
      | spec.context       | ${CTX:resourceContext6_2}        |
      | spec.upstream.url  | https://api.openai.com            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until provider template mapping "${CTX:resourceName6_2}" maps to template "${CTX:resourceName6_1}"
    Then the response status code should be 200
    And the provider template mapping "${CTX:resourceName6_2}" should map to template "${CTX:resourceName6_1}"
    When I delete the LLM provider "${CTX:resourceName6_2}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:resourceName6_1}"
    Then the response status code should be 200

  Scenario: LLM provider deletion removes its provider-template mapping
    Given I generate a unique resource name from "deleting-provider" and store it as "resourceName7_1"
    And I generate a unique API context from "/lazy-provider-delete" and store it as "resourceContext7_1"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName7_1}           |
      | displayName        | Delete Mapping Provider            |
      | version            | v1.0                              |
      | template           | anthropic                         |
      | spec.context       | ${CTX:resourceContext7_1}        |
      | spec.upstream.url  | https://api.anthropic.com         |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until provider template mapping "${CTX:resourceName7_1}" maps to template "anthropic"
    Then the response status code should be 200
    And the lazy resources should contain resource "${CTX:resourceName7_1}" of type "ProviderTemplateMapping"
    When I delete the LLM provider "${CTX:resourceName7_1}"
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:resourceName7_1}" of type "ProviderTemplateMapping" is absent
    Then the response status code should be 200
    And the lazy resources should not contain resource "${CTX:resourceName7_1}"

  Scenario: Provider name is propagated to route metadata
    Given I generate a unique resource name from "route-provider" and store it as "resourceName8_1"
    And I generate a unique API context from "/lazy-route-provider" and store it as "resourceContext8_1"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName8_1}           |
      | displayName        | Route Metadata Test Provider      |
      | version            | v1.0                              |
      | template           | openai                            |
      | spec.context       | ${CTX:resourceContext8_1}        |
      | spec.upstream.url  | https://api.openai.com            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until provider template mapping "${CTX:resourceName8_1}" maps to template "openai"
    Then the response status code should be 200
    And the policy engine route metadata should contain provider_name "${CTX:resourceName8_1}"
    When I delete the LLM provider "${CTX:resourceName8_1}"
    Then the response status code should be 200

  Scenario: Template and provider with the same name coexist
    Given I generate a unique resource name from "collision" and store it as "resourceName9_1"
    And I generate a unique API context from "/lazy-collision" and store it as "resourceContext9_1"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:resourceName9_1}           |
      | displayName       | Collision Test Template           |
      | spec.promptTokens      | {"location":"payload","identifier":"$.usage.prompt_tokens"} |
      | spec.completionTokens  | {"location":"payload","identifier":"$.usage.completion_tokens"} |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName9_1}           |
      | displayName        | Collision Test Provider            |
      | version            | v1.0                              |
      | template           | ${CTX:resourceName9_1}            |
      | spec.context       | ${CTX:resourceContext9_1}        |
      | spec.upstream.url  | https://api.example.com            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until provider template mapping "${CTX:resourceName9_1}" maps to template "${CTX:resourceName9_1}"
    Then the response status code should be 200
    And the lazy resources should contain template "${CTX:resourceName9_1}" of type "LlmProviderTemplate"
    And the lazy resources should contain resource "${CTX:resourceName9_1}" of type "ProviderTemplateMapping"
    And the lazy resources should have at least 2 resources with id "${CTX:resourceName9_1}"
    When I delete the LLM provider "${CTX:resourceName9_1}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:resourceName9_1}"
    Then the response status code should be 200
