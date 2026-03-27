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

Feature: LLM Proxy Management Operations
  As an API administrator
  I want to manage LLM proxies via REST API handlers
  So that I can create, read, update, delete, and list LLM proxies

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== LIST LLM PROXIES ====================
  
  Scenario: List all LLM proxies when none exist
    When I send a GET request to the "gateway-controller" service at "/llm-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM proxies with pagination parameters
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM proxies with different limit values
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?limit=5"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: List LLM proxies with offset only
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?offset=10"
    Then the response should be successful
    And the response should be valid JSON

  # ==================== GET LLM PROXY BY ID ===================
  
  Scenario: Get LLM proxy by non-existent ID returns 404
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-id-12345"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get LLM proxy with invalid ID format returns 404
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/invalid@proxy#id$format"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== DELETE LLM PROXY ====================
  
  Scenario: Delete non-existent LLM proxy returns 404
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-delete-123"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Delete LLM proxy with invalid ID format returns 404
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/invalid-delete@id"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== UPDATE LLM PROXY ====================

  Scenario: Update non-existent LLM proxy returns 400
    When I send a PUT request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-update" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "LlmProxy",
        "metadata": {
          "name": "non-existent-proxy-update"
        },
        "spec": {
          "displayName": "Test",
          "version": "v1.0",
          "context": "/test"
        }
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== CREATE LLM PROXY - VALIDATION ====================

  Scenario: Create LLM proxy with missing required fields returns error
    When I send a POST request to the "gateway-controller" service at "/llm-proxies" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "LlmProxy",
        "metadata": {
          "name": "invalid-proxy"
        },
        "spec": {
          "displayName": "Invalid Proxy"
        }
      }
      """
    Then the response should be a client error
    And the response should be valid JSON

  # ==================== COMPLETE LLM PROXY LIFECYCLE ====================

  Scenario: Complete LLM proxy lifecycle - create, get, update, and delete
    # First, create the LLM provider that the proxy will reference
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: lifecycle-test-provider
      spec:
        displayName: Lifecycle Test Provider
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
    # Create LLM proxy referencing the provider
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: lifecycle-llm-proxy
      spec:
        displayName: Lifecycle LLM Proxy
        version: v1.0
        provider:
          id: lifecycle-test-provider
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Get the LLM proxy
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "Lifecycle LLM Proxy"
    # Update the LLM proxy
    When I update the LLM proxy "lifecycle-llm-proxy" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: lifecycle-llm-proxy
      spec:
        displayName: Updated Lifecycle LLM Proxy
        version: v1.1
        provider:
          id: lifecycle-test-provider
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Verify update
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response should be successful
    And the response body should contain "Updated Lifecycle LLM Proxy"
    # Delete the LLM proxy
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response should be successful
    And the JSON response field "status" should be "success"
    # Verify deletion
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response status should be 404
    # Cleanup: delete the provider
    When I delete the LLM provider "lifecycle-test-provider"
    Then the response status code should be 200

  Scenario: List LLM proxies after creating one
    # First, create the LLM provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: list-test-provider
      spec:
        displayName: List Test Provider
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
    # Create LLM proxy
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: list-test-llm-proxy
      spec:
        displayName: List Test LLM Proxy
        version: v1.0
        provider:
          id: list-test-provider
      """
    Then the response status should be 201
    # List LLM proxies
    When I send a GET request to the "gateway-controller" service at "/llm-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "list-test-llm-proxy"
    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/list-test-llm-proxy"
    Then the response should be successful
    When I delete the LLM provider "list-test-provider"
    Then the response status code should be 200

  # ==================== CREATE LLM PROXY - ADDITIONAL ERROR CASES ====================

  Scenario: Create LLM proxy with invalid JSON body returns error
    When I send a POST request to the "gateway-controller" service at "/llm-proxies" with body:
      """
      { invalid json content here
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create LLM proxy referencing non-existent provider
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: orphan-llm-proxy
      spec:
        displayName: Orphan LLM Proxy
        version: v1.0
        provider:
          id: non-existent-provider-12345
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Update LLM proxy with invalid JSON body returns error
    When I send a PUT request to the "gateway-controller" service at "/llm-proxies/some-proxy" with body:
      """
      { not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON

  # ==================== LIST LLM PROXIES WITH FILTERS ====================

  Scenario: List LLM proxies with displayName filter
    # First, create the LLM provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: filter-llm-provider
      spec:
        displayName: Filter LLM Provider
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
    # Create LLM proxy with unique displayName
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: unique-displayname-proxy
      spec:
        displayName: UniqueProxyDisplayName
        version: v1.0
        provider:
          id: filter-llm-provider
      """
    Then the response status should be 201
    # Search by displayName
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?displayName=UniqueProxyDisplayName"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "UniqueProxyDisplayName"
    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/unique-displayname-proxy"
    Then the response should be successful
    When I delete the LLM provider "filter-llm-provider"
    Then the response status code should be 200

  Scenario: List LLM proxies with version filter
    # First, create the LLM provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: version-filter-provider
      spec:
        displayName: Version Filter Provider
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
    # Create LLM proxy with specific version
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: version-filter-proxy
      spec:
        displayName: Version Filter Proxy
        version: v99.0
        provider:
          id: version-filter-provider
      """
    Then the response status should be 201
    # Search by version
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?version=v99.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/version-filter-proxy"
    Then the response should be successful
    When I delete the LLM provider "version-filter-provider"
    Then the response status code should be 200

  Scenario: List LLM proxies with non-matching filter returns empty
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?displayName=NonExistentProxyName99999"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0

  # ==================== API INVOCATION TESTS ====================

  Scenario: Invoke LLM proxy chat completions endpoint
    # First, create the LLM provider that the proxy will reference
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: invoke-proxy-provider
      spec:
        displayName: Invoke Proxy Provider
        version: v1.0
        template: openai
        context: /provider-for-invoke-proxy
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

    # Create LLM proxy with context path
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: invoke-test-proxy
      spec:
        displayName: Invoke Test Proxy
        version: v1.0
        context: /proxy-invoke-test
        provider:
          id: invoke-proxy-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # Invoke chat completions through the proxy
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/proxy-invoke-test/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [
          {"role": "user", "content": "Hello from proxy test!"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"
    And the response body should contain "choices"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/invoke-test-proxy"
    Then the response should be successful
    When I delete the LLM provider "invoke-proxy-provider"
    Then the response status code should be 200

  Scenario: Invoke LLM proxy - provider access control allows exception paths
    # Create provider with restricted access control
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: acl-proxy-provider
      spec:
        displayName: ACL Proxy Provider
        version: v1.0
        template: openai
        context: /provider-for-acl-proxy
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

    # Create LLM proxy
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: acl-invoke-proxy
      spec:
        displayName: ACL Invoke Proxy
        version: v1.0
        context: /proxy-acl-test
        provider:
          id: acl-proxy-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # Allowed endpoint should work
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/proxy-acl-test/chat/completions" with body:
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
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/acl-invoke-proxy"
    Then the response should be successful
    When I delete the LLM provider "acl-proxy-provider"
    Then the response status code should be 200

  Scenario: Multiple sequential requests through LLM proxy
    # Create provider
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: multi-request-provider
      spec:
        displayName: Multi Request Provider
        version: v1.0
        template: openai
        context: /provider-for-multi-proxy
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

    # Create LLM proxy
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: multi-request-proxy
      spec:
        displayName: Multi Request Proxy
        version: v1.0
        context: /proxy-multi-test
        provider:
          id: multi-request-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # First request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/proxy-multi-test/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "First request"}]
      }
      """
    Then the response status code should be 200
    And the JSON response field "object" should be "chat.completion"

    # Second request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/proxy-multi-test/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Second request"}]
      }
      """
    Then the response status code should be 200
    And the JSON response field "object" should be "chat.completion"

    # Third request with different model
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/proxy-multi-test/chat/completions" with body:
      """
      {
        "model": "gpt-3.5-turbo",
        "messages": [
          {"role": "system", "content": "Be concise"},
          {"role": "user", "content": "Third request"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/multi-request-proxy"
    Then the response should be successful
    When I delete the LLM provider "multi-request-provider"
    Then the response status code should be 200
