# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: LLM Provider Management
  As an API administrator
  I want to manage LLM providers in the gateway
  So that I can configure and control access to LLM services

  Background:
    Given the gateway services are running

  # ========================================
  # Scenario Group 1: CRUD Operations (Happy Path)
  # ========================================

  Scenario: Complete LLM provider lifecycle - create, retrieve, update, and delete
    # Create
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: openai-provider
        spec:
          displayName: OpenAI Provider
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi-https:9443/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-key
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "id" should be "openai-provider"
    And the JSON response field "message" should be "LLM provider created successfully"

    # Retrieve by ID
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "openai-provider"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "provider.configuration.metadata.name" should be "openai-provider"
    And the JSON response field "provider.configuration.spec.displayName" should be "OpenAI Provider"
    And the JSON response field "provider.configuration.spec.version" should be "v1.0"
    And the JSON response field "provider.configuration.spec.template" should be "openai"
    And the JSON response field "provider.configuration.spec.accessControl.mode" should be "allow_all"

    # Update
    Given I authenticate using basic auth as "admin"
    When I update the LLM provider "openai-provider" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: openai-provider
        spec:
          displayName: OpenAI Provider Updated
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi-https:9443/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-updated-key
          accessControl:
            mode: deny_all
            exceptions:
              - path: /chat/completions
                methods: [POST]
        """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "id" should be "openai-provider"
    And the JSON response field "message" should be "LLM provider updated successfully"

    # Verify update
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "openai-provider"
    Then the response status code should be 200
    And the JSON response field "provider.configuration.spec.displayName" should be "OpenAI Provider Updated"
    And the JSON response field "provider.configuration.spec.accessControl.mode" should be "deny_all"
    And the JSON response field "provider.configuration.spec.accessControl.exceptions[0].path" should be "/chat/completions"

    # Delete
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "openai-provider"
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should be "LLM provider deleted successfully"

    # Verify deletion
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "openai-provider"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ========================================
  # Scenario Group 2: List and Filter Operations
  # ========================================

  Scenario: List all LLM providers
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: provider-1
        spec:
          displayName: Provider One
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: provider-2
        spec:
          displayName: Provider Two
          version: v2.0
          template: openai
          context: /openai
          vhost: api.openai.local
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: deny_all
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list all LLM providers
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be at least 2

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "provider-1"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "provider-2"
    Then the response status code should be 200

  Scenario: Filter LLM providers by displayName
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: filter-test-1
        spec:
          displayName: Test Provider Alpha
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list LLM providers with filter "displayName" as "Test Provider Alpha"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "providers[0].displayName" should be "Test Provider Alpha"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "filter-test-1"
    Then the response status code should be 200

  Scenario: Filter LLM providers by version
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: version-test
        spec:
          displayName: Version Test Provider
          version: v2.5
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list LLM providers with filter "version" as "v2.5"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "providers[0].version" should be "v2.5"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "version-test"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 3: Access Control Testing
  # ========================================

  Scenario: LLM provider with allow_all access control mode
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: allow-all-provider
        spec:
          displayName: Allow All Provider
          version: v1.0
          template: openai
          context: /allow-all
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I wait for 2 seconds

    # Test that any path is accessible
    When I send a POST request to "http://localhost:8080/allow-all/chat/completions" with body:
        """
        {
          "model": "gpt-4",
          "messages": [
            {"role": "user", "content": "Hello"}
          ]
        }
        """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "allow-all-provider"
    Then the response status code should be 200

  Scenario: LLM provider with deny_all access control and exceptions
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: deny-all-provider
        spec:
          displayName: Deny All Provider
          version: v1.0
          template: openai
          context: /deny-all
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: deny_all
            exceptions:
              - path: /chat/completions
                methods: [POST]
              - path: /embeddings
                methods: [POST]
        """
    Then the response status code should be 201
    And I wait for 2 seconds

    # Test allowed path
    When I send a POST request to "http://localhost:8080/deny-all/chat/completions" with body:
        """
        {
          "model": "gpt-4",
          "messages": [
            {"role": "user", "content": "Hello"}
          ]
        }
        """
    Then the response status code should be 200

    # Test denied path (not in exceptions)
    When I send a GET request to "http://localhost:8080/deny-all/models"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "deny-all-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 4: Upstream Configuration
  # ========================================

  Scenario: LLM provider with API key authentication
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: apikey-auth-provider
        spec:
          displayName: API Key Auth Provider
          version: v1.0
          template: openai
          context: /apikey-test
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-custom-key-123
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I wait for 2 seconds

    # Test invocation with configured API key
    When I send a POST request to "http://localhost:8080/apikey-test/chat/completions" with body:
        """
        {
          "model": "gpt-4",
          "messages": [
            {"role": "user", "content": "Test message"}
          ]
        }
        """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "model" should be "gpt-4.1-2025-04-14"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "apikey-auth-provider"
    Then the response status code should be 200

  Scenario: LLM provider with bearer token authentication
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: bearer-auth-provider
        spec:
          displayName: Bearer Auth Provider
          version: v1.0
          template: openai
          context: /bearer-test
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: bearer
              header: Authorization
              value: token-abc-123
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "bearer-auth-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 5: Virtual Host and Context Path
  # ========================================

  Scenario: LLM provider with custom context path
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: context-path-provider
        spec:
          displayName: Context Path Provider
          version: v1.0
          template: openai
          context: /custom/openai/v1
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I wait for 2 seconds

    # Test that context path is applied correctly
    When I send a POST request to "http://localhost:8080/custom/openai/v1/chat/completions" with body:
        """
        {
          "model": "gpt-4",
          "messages": [
            {"role": "user", "content": "Test"}
          ]
        }
        """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "context-path-provider"
    Then the response status code should be 200

  Scenario: LLM provider with vhost configuration
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: vhost-provider
        spec:
          displayName: VHost Provider
          version: v1.0
          template: openai
          vhost: api.openai.local
          context: /v1
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Verify vhost configuration
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "vhost-provider"
    Then the response status code should be 200
    And the JSON response field "provider.configuration.spec.vhost" should be "api.openai.local"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "vhost-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 6: Template Integration
  # ========================================

  Scenario: LLM provider using openai template
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: openai-template-test
        spec:
          displayName: OpenAI Template Test
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Verify template is set
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "openai-template-test"
    Then the response status code should be 200
    And the JSON response field "provider.configuration.spec.template" should be "openai"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "openai-template-test"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 7: Policy Attachment
  # ========================================

  Scenario: LLM provider with attached policies
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: policy-provider
        spec:
          displayName: Provider With Policies
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
          policies:
            - name: modify-headers
              version: v1.0.0
              paths:
                - path: /chat/completions
                  methods: [POST]
                  params:
                    request:
                      add:
                        x-custom-header: "test-value"
        """
    Then the response status code should be 201

    # Verify policies are attached
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "policy-provider"
    Then the response status code should be 200
    And the JSON response field "provider.configuration.spec.policies[0].name" should be "modify-headers"
    And the JSON response field "provider.configuration.spec.policies[0].version" should be "v1.0.0"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "policy-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 8: Error Scenarios
  # ========================================

  Scenario: Create LLM provider with invalid configuration - missing required fields
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: invalid-provider
        spec:
          displayName: Invalid Provider
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create duplicate LLM provider
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: duplicate-provider
        spec:
          displayName: Duplicate Test
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: duplicate-provider
        spec:
          displayName: Duplicate Test
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 409
    And the JSON response field "status" should be "error"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "duplicate-provider"
    Then the response status code should be 200

  Scenario: Retrieve non-existent LLM provider
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "non-existent-provider"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Update non-existent LLM provider
    Given I authenticate using basic auth as "admin"
    When I update the LLM provider "non-existent-update" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: non-existent-update
        spec:
          displayName: Does Not Exist
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Delete non-existent LLM provider
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "non-existent-delete"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  # ========================================
  # Scenario Group 9: End-to-End Invocation Tests
  # ========================================

  Scenario: Complete LLM invocation flow - create provider and make chat completion request
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: e2e-openai-provider
        spec:
          displayName: E2E OpenAI Provider
          version: v1.0
          template: openai
          context: /e2e-openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-e2e
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I wait for 2 seconds

    # Make a chat completion request
    When I send a POST request to "http://localhost:8080/e2e-openai/chat/completions" with body:
        """
        {
          "model": "gpt-4",
          "messages": [
            {
              "role": "system",
              "content": "You are a helpful assistant."
            },
            {
              "role": "user",
              "content": "What is the capital of France?"
            }
          ]
        }
        """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"
    And the JSON response field "model" should be "gpt-4.1-2025-04-14"
    And the JSON response field "choices[0].message.role" should be "assistant"
    And the JSON response field "usage.prompt_tokens" should be 19
    And the JSON response field "usage.completion_tokens" should be 10
    And the JSON response field "usage.total_tokens" should be 29

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "e2e-openai-provider"
    Then the response status code should be 200

  Scenario: LLM invocation with embeddings endpoint
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: embeddings-provider
        spec:
          displayName: Embeddings Provider
          version: v1.0
          template: openai
          context: /embeddings-test
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I wait for 2 seconds

    # Make an embeddings request
    When I send a POST request to "http://localhost:8080/embeddings-test/embeddings" with body:
        """
        {
          "model": "text-embedding-ada-002",
          "input": "The quick brown fox jumps over the lazy dog"
        }
        """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "embeddings-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 10: Minimal Configuration
  # ========================================

  Scenario: Create LLM provider with minimal required fields
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: minimal-provider
        spec:
          displayName: Minimal Provider
          version: v1.0
          template: openai
          upstream:
            url: http://mock-openapi:4010/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Verify minimal configuration is accepted
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "minimal-provider"
    Then the response status code should be 200
    And the JSON response field "provider.configuration.spec.displayName" should be "Minimal Provider"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "minimal-provider"
    Then the response status code should be 200
