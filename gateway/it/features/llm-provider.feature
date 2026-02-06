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
            url: https://mock-openapi-https:9443/openai/v1
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

#   Scenario: List all LLM providers
#     Given I authenticate using basic auth as "admin"
#     When I create this LLM provider:
#         """
#         apiVersion: gateway.api-platform.wso2.com/v1alpha1
#         kind: LlmProvider
#         metadata:
#           name: provider-1
#         spec:
#           displayName: Provider One
#           version: v1.0
#           template: openai
#           upstream:
#             url: https://mock-openapi-https:9443/openai/v1
#           accessControl:
#             mode: allow_all
#         """
#     Then the response status code should be 201

#     Given I authenticate using basic auth as "admin"
#     When I create this LLM provider:
#         """
#         apiVersion: gateway.api-platform.wso2.com/v1alpha1
#         kind: LlmProvider
#         metadata:
#           name: provider-2
#         spec:
#           displayName: Provider Two
#           version: v2.0
#           template: openai
#           context: /openai
#           vhost: api.openai.local
#           upstream:
#             url: https://mock-openapi-https:9443/openai/v1
#           accessControl:
#             mode: deny_all
#         """
#     Then the response status code should be 201

#     Given I authenticate using basic auth as "admin"
#     When I list all LLM providers
#     Then the response status code should be 200
#     And the response should be valid JSON
#     And the JSON response field "status" should be "success"
#     And the JSON response field "count" should be at least 2

    # Cleanup
#     Given I authenticate using basic auth as "admin"
#     When I delete the LLM provider "provider-1"
#     Then the response status code should be 200

#     Given I authenticate using basic auth as "admin"
#     When I delete the LLM provider "provider-2"
#     Then the response status code should be 200

  Scenario: List all LLM providers when none exist
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/llm-providers"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM providers with pagination parameters
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/llm-providers?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Get LLM provider with invalid ID format returns 404
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/llm-providers/invalid@provider#id"
    Then the response status should be 404
    And the response should be valid JSON

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
            url: https://mock-openapi-https:9443/openai/v1
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list LLM providers with filter "displayName" as "Test%20Provider%20Alpha"
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
            url: https://mock-openapi-https:9443/openai/v1
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
  # Scenario Group 5: Virtual Host and Context Path
  # ========================================

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
            url: https://mock-openapi-https:9443/openai/v1
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
            url: https://mock-openapi-https:9443/openai/v1
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
            url: https://mock-openapi-https:9443/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
          policies:
            - name: modify-headers
              version: v1
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
    And the JSON response field "provider.configuration.spec.policies[0].version" should be "v1"

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

  Scenario: Retrieve non-existent LLM provider
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider "non-existent-provider"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Delete non-existent LLM provider
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "non-existent-delete"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider with invalid JSON body returns error
    Given I authenticate using basic auth as "admin"
    When I send a POST request to the "gateway-controller" service at "/llm-providers" with body:
      """
      { this is not valid json content
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Update LLM provider with invalid JSON body returns error
    Given I authenticate using basic auth as "admin"
    When I send a PUT request to the "gateway-controller" service at "/llm-providers/some-provider" with body:
      """
      { invalid json
      """
    Then the response should be a client error
    And the response should be valid JSON

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
            url: https://mock-openapi-https:9443/openai/v1
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

  # ========================================
  # Scenario Group 11: API Invocation Tests
  # ========================================

  Scenario: Invoke LLM provider chat completions endpoint via context path
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: invoke-context-provider
        spec:
          displayName: Invoke Context Provider
          version: v1.0
          template: openai
          context: /llm-invoke-context
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

    # Invoke chat completions endpoint
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/llm-invoke-context/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [
          {"role": "user", "content": "Hello, how are you?"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "chat.completion"
    And the response body should contain "choices"
    And the JSON response field "object" should be "chat.completion"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "invoke-context-provider"
    Then the response status code should be 200

  Scenario: Invoke LLM provider - access control deny_all allows exception paths
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: invoke-acl-provider
        spec:
          displayName: Invoke ACL Provider
          version: v1.0
          template: openai
          context: /llm-acl-test
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-key
          accessControl:
            mode: deny_all
            exceptions:
              - path: /chat/completions
                methods: [POST]
        """
    Then the response status code should be 201
    And I wait for 3 seconds

    # Allowed endpoint should work
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/llm-acl-test/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "invoke-acl-provider"
    Then the response status code should be 200

  Scenario: Invoke LLM provider - verify upstream auth header is added
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProvider
        metadata:
          name: invoke-auth-provider
        spec:
          displayName: Invoke Auth Provider
          version: v1.0
          template: openai
          context: /llm-auth-test
          upstream:
            url: http://mock-openapi:4010/openai/v1
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-auth-key-12345
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I wait for 3 seconds

    # Request should succeed (mock validates auth header presence)
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/llm-auth-test/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Test auth"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "invoke-auth-provider"
    Then the response status code should be 200
