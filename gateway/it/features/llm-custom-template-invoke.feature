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

Feature: Custom LLM Provider Template deploy and invoke
  As an API administrator
  I want to deploy providers built from custom (managedBy != wso2) multi-version templates
  So that the gateway resolves the exact template version a provider references

  # This mirrors the platform-api -> gateway contract: platform-api sets the
  # deployed provider's spec.template to the template's *versioned id*
  # (e.g. "ecustom-v1-0"), not the bare group-version handle ("ecustom").

  Background:
    Given the gateway services are running

  Scenario: Provider referencing a custom template by its versioned id resolves and invokes
    # --- Custom template family "ecustom", version v1.0 (managedBy: customer) ---
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: ecustom-v1-0
      spec:
        displayName: E2E Custom v1
        groupId: ecustom
        managedBy: customer
        version: v1.0
        promptTokens:
          location: payload
          identifier: $.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "success"
    And the JSON response field "status.id" should be "ecustom-v1-0"

    # --- Second version v2.0 in the same family ---
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: ecustom-v2-0
      spec:
        displayName: E2E Custom v2
        groupId: ecustom
        managedBy: customer
        version: v2.0
        promptTokens:
          location: payload
          identifier: $.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response status code should be 201
    And the JSON response field "status.id" should be "ecustom-v2-0"

    # --- Provider P1 built from v1.0, referenced by versioned id ---
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: ecustom-provider-v1
      spec:
        displayName: E2E Custom Provider v1
        version: v1.0
        context: /ecustom-v1
        template: ecustom-v1-0
        upstream:
          url: http://mock-openapi:4010/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # --- Provider P2 built from v2.0, referenced by versioned id ---
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: ecustom-provider-v2
      spec:
        displayName: E2E Custom Provider v2
        version: v2.0
        context: /ecustom-v2
        template: ecustom-v2-0
        upstream:
          url: http://mock-openapi:4010/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for 3 seconds

    # --- Invoke P1 (v1.0) through the gateway ---
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/ecustom-v1/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello from v1"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    # --- Invoke P2 (v2.0) through the gateway ---
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/ecustom-v2/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello from v2"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    # --- Cleanup ---
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "ecustom-provider-v1"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "ecustom-provider-v2"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "ecustom-v2-0"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "ecustom-v1-0"
    Then the response status code should be 200
