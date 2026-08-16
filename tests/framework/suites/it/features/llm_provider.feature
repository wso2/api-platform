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

@llm-provider
Feature: LLM Provider Management
  As an API administrator
  I want to manage LLM providers in the gateway
  So that I can configure and control access to LLM services

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ========================================
  # Scenario Group 1: CRUD Operations (Happy Path)
  # ========================================

  Scenario: Complete LLM provider lifecycle - create, retrieve, update, and delete
    # Create
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: openai-provider
        spec:
          displayName: OpenAI Provider
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-key
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And the JSON response field "status.id" should be "openai-provider"
    And the JSON response field "metadata.name" should be "openai-provider"

    # Retrieve by ID
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/openai-provider"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And the JSON response field "metadata.name" should be "openai-provider"
    And the JSON response field "spec.displayName" should be "OpenAI Provider"
    And the JSON response field "spec.version" should be "v1.0"
    And the JSON response field "spec.template" should be "openai"
    And the JSON response field "spec.accessControl.mode" should be "allow_all"

    # Update
    When I update the LLM provider "openai-provider" with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: openai-provider
        spec:
          displayName: OpenAI Provider Updated
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
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
    And the JSON response field "status.state" should be "deployed"
    And the JSON response field "status.id" should be "openai-provider"
    And the JSON response field "metadata.name" should be "openai-provider"

    # Verify update
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/openai-provider"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "OpenAI Provider Updated"
    And the JSON response field "spec.accessControl.mode" should be "deny_all"
    And the JSON response field "spec.accessControl.exceptions[0].path" should be "/chat/completions"

    # Delete
    When I delete the LLM provider "openai-provider"
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should be "LLM provider deleted successfully"

    # Verify deletion
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/openai-provider"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ========================================
  # Scenario Group 2: List and Filter Operations
  # ========================================

  Scenario: List all LLM providers
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: provider-1
        spec:
          displayName: Provider One
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3008
          accessControl:
            mode: deny_all
        """
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be greater than 1

    # Cleanup
    When I delete the LLM provider "provider-1"
    Then the response status code should be 200

    When I delete the LLM provider "provider-2"
    Then the response status code should be 200

  Scenario: List all LLM providers when none exist
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM providers with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers?limit=10&offset=0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Get LLM provider with invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/invalid@provider#id"
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Filter LLM providers by displayName
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: filter-test-1
        spec:
          displayName: Test Provider Alpha
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers?displayName=Test%20Provider%20Alpha"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "providers[0].spec.displayName" should be "Test Provider Alpha"

    # Cleanup
    When I delete the LLM provider "filter-test-1"
    Then the response status code should be 200

  Scenario: Filter LLM providers by version
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: version-test
        spec:
          displayName: Version Test Provider
          version: v2.5
          template: openai
          upstream:
            url: http://testbench:3008
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    When I send a "GET" request to the "gateway-controller" service at "/llm-providers?version=v2.5"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "providers[0].spec.version" should be "v2.5"

    # Cleanup
    When I delete the LLM provider "version-test"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 5: Virtual Host and Context Path
  # ========================================

  Scenario: LLM provider with vhost configuration
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Verify vhost configuration
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/vhost-provider"
    Then the response status code should be 200
    And the JSON response field "spec.vhost" should be "api.openai.local"

    # Cleanup
    When I delete the LLM provider "vhost-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 6: Template Integration
  # ========================================

  Scenario: LLM provider using openai template
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: openai-template-test
        spec:
          displayName: OpenAI Template Test
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Verify template is set
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/openai-template-test"
    Then the response status code should be 200
    And the JSON response field "spec.template" should be "openai"

    # Cleanup
    When I delete the LLM provider "openai-template-test"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 7: Policy Attachment
  # ========================================

  Scenario: LLM provider with attached policies
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: policy-provider
        spec:
          displayName: Provider With Policies
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
          policies:
            - name: set-headers
              version: v1
              paths:
                - path: /chat/completions
                  methods: [POST]
                  params:
                    request:
                      headers:
                        - name: x-custom-header
                          value: "test-value"
        """
    Then the response status code should be 201

    # Verify policies are attached
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/policy-provider"
    Then the response status code should be 200
    And the JSON response field "spec.policies[0].name" should be "set-headers"
    And the JSON response field "spec.policies[0].version" should be "v1"

    # Cleanup
    When I delete the LLM provider "policy-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 8: Error Scenarios
  # ========================================

  Scenario: Create LLM provider with invalid configuration - missing required fields
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: invalid-provider
        spec:
          displayName: Invalid Provider
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider referencing a non-existent policy is rejected
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: bad-policy-name-provider
        spec:
          displayName: Bad Policy Name Provider
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
          globalPolicies:
            - name: this-policy-does-not-exist
              version: v1
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider referencing a non-existent policy version is rejected
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: bad-policy-version-provider
        spec:
          displayName: Bad Policy Version Provider
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
          globalPolicies:
            - name: basic-ratelimit
              version: v999
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # An empty policy version is a valid input: it resolves to the latest loaded version.
  Scenario: Create LLM provider with an empty policy version resolves to the latest
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: empty-version-policy-provider
        spec:
          displayName: Empty Version Policy Provider
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test
          accessControl:
            mode: allow_all
          globalPolicies:
            - name: basic-ratelimit
              version: ""
              params:
                limits:
                  - requests: 10
                    duration: "1h"
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the LLM provider "empty-version-policy-provider"
    Then the response status code should be 200

  Scenario: Retrieve non-existent LLM provider
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/non-existent-provider"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Delete non-existent LLM provider
    When I delete the LLM provider "non-existent-delete"
    Then the response status code should be 404
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider with invalid JSON body returns error
    When I send a "POST" request to the "gateway-controller" service at "/llm-providers" with body:
      """
      { this is not valid json content
      """
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: Update LLM provider with invalid JSON body returns error
    When I send a "PUT" request to the "gateway-controller" service at "/llm-providers/some-provider" with body:
      """
      { invalid json
      """
    Then the response status code should be 400
    And the response should be valid JSON

  # ========================================
  # Scenario Group 10: Minimal Configuration
  # ========================================

  Scenario: Create LLM provider with minimal required fields
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: minimal-provider
        spec:
          displayName: Minimal Provider
          version: v1.0
          template: openai
          upstream:
            url: http://testbench:3008
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201

    # Verify minimal configuration is accepted
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/minimal-provider"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "Minimal Provider"

    # Cleanup
    When I delete the LLM provider "minimal-provider"
    Then the response status code should be 200

  # ========================================
  # Scenario Group 11: API Invocation Tests
  # ========================================

  Scenario: Invoke LLM provider chat completions endpoint via context path
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: invoke-context-provider
        spec:
          displayName: Invoke Context Provider
          version: v1.0
          template: openai
          context: /llm-invoke-context
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-key
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I send a "POST" request to "/llm-invoke-context/chat/completions" until status 200

    # Invoke chat completions endpoint
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/llm-invoke-context/chat/completions" with body:
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
    And the JSON response field "object" should be "chat.completion"
    And the JSON response should have field "choices"
    And the JSON response field "object" should be "chat.completion"

    # Cleanup
    When I delete the LLM provider "invoke-context-provider"
    Then the response status code should be 200

  Scenario: Invoke LLM provider - access control deny_all allows exception paths
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: invoke-acl-provider
        spec:
          displayName: Invoke ACL Provider
          version: v1.0
          template: openai
          context: /llm-acl-test
          upstream:
            url: http://testbench:3008
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
    And I send a "POST" request to "/llm-acl-test/chat/completions" until status 200

    # Allowed endpoint should work
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/llm-acl-test/chat/completions" with body:
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
    When I delete the LLM provider "invoke-acl-provider"
    Then the response status code should be 200

  Scenario: Invoke LLM provider - verify upstream auth header is added
    When I create LLM provider with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProvider
        metadata:
          name: invoke-auth-provider
        spec:
          displayName: Invoke Auth Provider
          version: v1.0
          template: openai
          context: /llm-auth-test
          upstream:
            url: http://testbench:3008
            auth:
              type: api-key
              header: Authorization
              value: Bearer sk-test-auth-key-12345
          accessControl:
            mode: allow_all
        """
    Then the response status code should be 201
    And I send a "POST" request to "/llm-auth-test/chat/completions" until status 200

    # Request should succeed (mock validates auth header presence)
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/llm-auth-test/chat/completions" with body:
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
    When I delete the LLM provider "invoke-auth-provider"
    Then the response status code should be 200
