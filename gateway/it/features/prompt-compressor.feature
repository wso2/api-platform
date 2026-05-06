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

@prompt-compressor
Feature: Prompt Compressor Policy
  As an API developer
  I want to compress prompts sent to LLMs
  So that I can reduce token usage and cost

  Background:
    Given the gateway services are running

  # ============================================================================
  # Section A: Deployment & Health
  # ============================================================================

  Scenario: Deploy API with prompt-compressor policy successfully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-deploy-test-api
      spec:
        displayName: Prompt Compressor Deploy Test
        version: v1.0
        context: /pc-deploy/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-deploy/v1.0/health" to be ready

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-deploy-test-api"
    Then the response should be successful

  Scenario: Deterministic compression produces exact expected output
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-deterministic-api
      spec:
        displayName: Prompt Compressor Deterministic Test
        version: v1.0
        context: /pc-deterministic/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-deterministic/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-deterministic/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "Artificial intelligence and machine learning have transformed the technology landscape significantly over the past decade. Deep learning models now power everything from natural language processing to computer vision applications. The advancement of transformer architectures has enabled breakthrough capabilities in text generation and understanding."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Artificial intelligence machine transformed technology landscape decade. Deep models everything natural language processing computer applications. advancement transformer enabled breakthrough capabilities text generation understanding."

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-deterministic-api"
    Then the response should be successful

  # ============================================================================
  # Section B: Ratio-Mode Compression
  # ============================================================================

  Scenario: Compress long prompt using ratio mode (default jsonPath)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-ratio-api
      spec:
        displayName: Prompt Compressor Ratio Test
        version: v1.0
        context: /pc-ratio/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-ratio/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-ratio/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    # Full length is 894, 0.5 ratio should make it significantly less. Let's say < 600
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100
    And the response body should not contain "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach."

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-ratio-api"
    Then the response should be successful

  Scenario: Short prompt remains unchanged (ratio mode, no compression needed)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-ratio-short-api
      spec:
        displayName: Prompt Compressor Ratio Short
        version: v1.0
        context: /pc-ratio-short/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.95
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-ratio-short/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-ratio-short/v1.0/chat" with body:
      """
      {
        "messages": [{"content": "Hi"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Hi"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-ratio-short-api"
    Then the response should be successful

  Scenario: Ratio value >= 1.0 skips compression
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-ratio-one-api
      spec:
        displayName: Prompt Compressor Ratio One
        version: v1.0
        context: /pc-ratio-one/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 1.0
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-ratio-one/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-ratio-one/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "json.messages[0].content" should contain "The deployment pipeline for the cloud-native application"
    And the JSON response field "json.messages[0].content" should contain "performance metrics across all environments."

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-ratio-one-api"
    Then the response should be successful

  # ============================================================================
  # Section C: Token-Mode Compression
  # ============================================================================

  Scenario: Compress using token mode with target below estimated count
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-token-api
      spec:
        displayName: Prompt Compressor Token Test
        version: v1.0
        context: /pc-token/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: token
                      value: 50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-token/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-token/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-token-api"
    Then the response should be successful

  Scenario: Token mode with target >= estimated tokens skips compression
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-token-skip-api
      spec:
        displayName: Prompt Compressor Token Skip
        version: v1.0
        context: /pc-token-skip/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: token
                      value: 9999
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-token-skip/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-token-skip/v1.0/chat" with body:
      """
      {
        "messages": [{"content": "Short text for skip"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Short text for skip"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-token-skip-api"
    Then the response should be successful

  # ============================================================================
  # Section D: JSONPath Targeting
  # ============================================================================

  Scenario: Custom jsonPath to flat field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-jsonpath-flat-api
      spec:
        displayName: Prompt Compressor JSONPath Flat
        version: v1.0
        context: /pc-jsonpath-flat/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.prompt"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-jsonpath-flat/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-jsonpath-flat/v1.0/chat" with body:
      """
      {
        "prompt": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments.",
        "model": "gpt-4"
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response string field "json.prompt" should have length less than 850
    And the JSON response string field "json.prompt" should have length greater than 100
    And the JSON response field "json.model" should be "gpt-4"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-jsonpath-flat-api"
    Then the response should be successful

  Scenario: Nested jsonPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-jsonpath-nested-api
      spec:
        displayName: Prompt Compressor JSONPath Nested
        version: v1.0
        context: /pc-jsonpath-nested/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.request.data.prompt"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-jsonpath-nested/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-jsonpath-nested/v1.0/chat" with body:
      """
      {
        "request": {
          "data": {
            "prompt": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          },
          "meta": "keep"
        }
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response string field "json.request.data.prompt" should have length less than 850
    And the JSON response string field "json.request.data.prompt" should have length greater than 100
    And the JSON response field "json.request.meta" should be "keep"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-jsonpath-nested-api"
    Then the response should be successful

  Scenario: Negative array index jsonPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-jsonpath-neg-api
      spec:
        displayName: Prompt Compressor Negative Index
        version: v1.0
        context: /pc-jsonpath-neg/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[-1].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-jsonpath-neg/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-jsonpath-neg/v1.0/chat" with body:
      """
      {
        "messages": [
          {"content": "First"},
          {"content": "Second"},
          {"content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response string field "json.messages[2].content" should have length less than 850
    And the JSON response string field "json.messages[2].content" should have length greater than 100
    And the JSON response field "json.messages[0].content" should be "First"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-jsonpath-neg-api"
    Then the response should be successful

  Scenario: jsonPath pointing to non-string value
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-jsonpath-nonstring-api
      spec:
        displayName: Prompt Compressor Non-String Target
        version: v1.0
        context: /pc-jsonpath-nonstring/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-jsonpath-nonstring/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-jsonpath-nonstring/v1.0/chat" with body:
      """
      {
        "messages": [{"content": 12345}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be 12345

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-jsonpath-nonstring-api"
    Then the response should be successful

  Scenario: jsonPath pointing to missing field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-jsonpath-missing-api
      spec:
        displayName: Prompt Compressor Missing Field
        version: v1.0
        context: /pc-jsonpath-missing/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.nonexistent.field"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-jsonpath-missing/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-jsonpath-missing/v1.0/chat" with body:
      """
      {
        "messages": [{"content": "Hello"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "Hello"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-jsonpath-missing-api"
    Then the response should be successful

  # ============================================================================
  # Section E: Selective Compression Tags
  # ============================================================================

  Scenario: Compress only tagged region, preserve untagged text
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-tags-api
      spec:
        displayName: Prompt Compressor Tags
        version: v1.0
        context: /pc-tags/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-tags/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-tags/v1.0/chat" with body:
      """
      {
        "messages": [{
          "role": "user",
          "content": "Instructions: Answer concisely.\n\n<APIP-COMPRESS>The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments.</APIP-COMPRESS>\n\nQuestion: What changed?"
        }]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "Instructions: Answer concisely."
    And the response body should contain "Question: What changed?"
    And the response body should not contain "<APIP-COMPRESS>"
    And the response body should not contain "</APIP-COMPRESS>"
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-tags-api"
    Then the response should be successful

  Scenario: Multiple tagged regions in one prompt
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-multi-tags-api
      spec:
        displayName: Prompt Compressor Multi Tags
        version: v1.0
        context: /pc-multi-tags/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-multi-tags/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-multi-tags/v1.0/chat" with body:
      """
      {
        "messages": [{
          "role": "user",
          "content": "Part 1:\n<APIP-COMPRESS>The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed.</APIP-COMPRESS>\n\nMiddle Preserved\n\nPart 2:\n<APIP-COMPRESS>The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments.</APIP-COMPRESS>"
        }]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "Part 1"
    And the response body should contain "Middle Preserved"
    And the response body should contain "Part 2"
    And the response body should not contain "<APIP-COMPRESS>"
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-multi-tags-api"
    Then the response should be successful

  Scenario: Unpaired closing tag is stripped
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-orphan-tag-api
      spec:
        displayName: Prompt Compressor Orphan Tag
        version: v1.0
        context: /pc-orphan-tag/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-orphan-tag/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-orphan-tag/v1.0/chat" with body:
      """
      {
        "messages": [{"content": "This is a prompt with an orphan tag</APIP-COMPRESS>"}]
      }
      """
    Then the response status code should be 200
    And the response body should not contain "</APIP-COMPRESS>"
    And the JSON response field "json.messages[0].content" should be "This is a prompt with an orphan tag"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-orphan-tag-api"
    Then the response should be successful

  # ============================================================================
  # Section F: Multi-Rule Evaluation
  # ============================================================================

  Scenario: Multiple rules, smallest limit matches
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-multi-rule-api
      spec:
        displayName: Prompt Compressor Multi Rule
        version: v1.0
        context: /pc-multi-rule/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: 100
                      type: ratio
                      value: 0.90
                    - upperTokenLimit: 500
                      type: ratio
                      value: 0.50
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.30
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-multi-rule/v1.0/health" to be ready

    # 100 tokens ~ 400 chars. Let's send short text to match rule 1 (ratio 0.9)
    When I send a POST request to "http://localhost:8080/pc-multi-rule/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "This is a moderately short text that should be under 100 tokens. The policy will evaluate the first rule with upperTokenLimit: 100 and apply a ratio of 0.90. Because 0.90 doesn't cause much compression, it will likely be forwarded unchanged due to NegativeGainError."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the JSON response string field "json.messages[0].content" should have length less than 320
    And the JSON response string field "json.messages[0].content" should have length greater than 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-multi-rule-api"
    Then the response should be successful

  Scenario: Multiple rules, fallback (-1) matches
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-multi-rule-fallback-api
      spec:
        displayName: Prompt Compressor Multi Rule Fallback
        version: v1.0
        context: /pc-multi-rule-fallback/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: 100
                      type: ratio
                      value: 0.90
                    - upperTokenLimit: 500
                      type: ratio
                      value: 0.50
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.30
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-multi-rule-fallback/v1.0/health" to be ready

    # Let's send long text (894 chars ~ 223 tokens). Actually, the fallback requires exceeding 500.
    # We'll just duplicate the text 3 times to get 2600+ chars (~650 tokens).
    When I send a POST request to "http://localhost:8080/pc-multi-rule-fallback/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments. The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments. The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    # 2684 total length, ratio 0.3 should give < 1200
    And the JSON response string field "json.messages[0].content" should have length less than 1200
    And the JSON response string field "json.messages[0].content" should have length greater than 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-multi-rule-fallback-api"
    Then the response should be successful

  Scenario: Invalid rules are dropped, valid rules still work
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-invalid-rules-api
      spec:
        displayName: Prompt Compressor Invalid Rules
        version: v1.0
        context: /pc-invalid-rules/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -5
                      type: ratio
                      value: 0.50
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-invalid-rules/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-invalid-rules/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-invalid-rules-api"
    Then the response should be successful

  # ============================================================================
  # Section G: Graceful Passthrough
  # ============================================================================

  Scenario: Empty request body
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-empty-body-api
      spec:
        displayName: Prompt Compressor Empty Body
        version: v1.0
        context: /pc-empty-body/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-empty-body/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-empty-body/v1.0/chat" with body:
      """
      """
    Then the response status code should be 200
    And the JSON response field "data" should be ""

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-empty-body-api"
    Then the response should be successful

  Scenario: Non-JSON request body
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-non-json-api
      spec:
        displayName: Prompt Compressor Non JSON
        version: v1.0
        context: /pc-non-json/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-non-json/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-non-json/v1.0/chat" with header "Content-Type" value "text/plain" with body:
      """
      just some text
      """
    Then the response status code should be 200
    And the JSON response field "data" should be "just some text"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-non-json-api"
    Then the response should be successful

  Scenario: Invalid JSON request body
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-invalid-json-api
      spec:
        displayName: Prompt Compressor Invalid JSON
        version: v1.0
        context: /pc-invalid-json/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-invalid-json/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-invalid-json/v1.0/chat" with body:
      """
      {invalid json}
      """
    Then the response status code should be 200
    And the JSON response field "data" should be "{invalid json}"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-invalid-json-api"
    Then the response should be successful

  # ============================================================================
  # Section H: Dynamic Metadata Verification
  # ============================================================================

  Scenario: Verify policy config dump shows prompt-compressor for deployed route
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-metadata-api
      spec:
        displayName: Prompt Compressor Metadata
        version: v1.0
        context: /pc-metadata/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.50
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-metadata/v1.0/health" to be ready

    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the config dump should contain route with basePath "/pc-metadata/v1.0"
    And the config dump should contain policy "prompt-compressor" for route "/pc-metadata/v1.0/chat"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-metadata-api"
    Then the response should be successful

  # ============================================================================
  # Section I: API Lifecycle
  # ============================================================================

  Scenario: Lifecycle operations (add, update, remove)
    Given I authenticate using basic auth as "admin"
    # Phase 1: Deploy without policy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-lifecycle-api
      spec:
        displayName: Prompt Compressor Lifecycle
        version: v1.0
        context: /pc-lifecycle/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-lifecycle/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/pc-lifecycle/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response body should contain "The deployment pipeline for the cloud-native application underwent significant changes"
    And the response body should contain "performance metrics across all environments"

    # Phase 2: Add policy (aggressive ratio 0.3)
    When I update the API "pc-lifecycle-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-lifecycle-api
      spec:
        displayName: Prompt Compressor Lifecycle
        version: v1.0
        context: /pc-lifecycle/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.30
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-lifecycle/v1.0/health" to be ready
    And I wait for 2 seconds

    When I send a POST request to "http://localhost:8080/pc-lifecycle/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the JSON response string field "json.messages[0].content" should have length less than 850
    And the JSON response string field "json.messages[0].content" should have length greater than 100

    # Phase 3: Update policy (high ratio 0.95 -> skip compression)
    When I update the API "pc-lifecycle-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-lifecycle-api
      spec:
        displayName: Prompt Compressor Lifecycle
        version: v1.0
        context: /pc-lifecycle/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-compressor
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  rules:
                    - upperTokenLimit: -1
                      type: ratio
                      value: 0.95
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-lifecycle/v1.0/health" to be ready
    And I wait for 2 seconds

    When I send a POST request to "http://localhost:8080/pc-lifecycle/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the JSON response field "json.messages[0].content" should contain "deployment pipeline"
    And the JSON response field "json.messages[0].content" should contain "microservices-based approach"
    And the JSON response field "json.messages[0].content" should contain "authentication module"

    # Phase 4: Remove policy entirely
    When I update the API "pc-lifecycle-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pc-lifecycle-api
      spec:
        displayName: Prompt Compressor Lifecycle
        version: v1.0
        context: /pc-lifecycle/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pc-lifecycle/v1.0/health" to be ready
    And I wait for 2 seconds

    When I send a POST request to "http://localhost:8080/pc-lifecycle/v1.0/chat" with body:
      """
      {
        "messages": [
          {
            "content": "The deployment pipeline for the cloud-native application underwent significant changes during the last quarter. The engineering team migrated from a monolithic architecture to a microservices-based approach. This transition involved refactoring the authentication module, updating the database connection pooling strategy, and implementing new caching mechanisms. The team also introduced automated regression testing suites that run on every pull request submission. Performance benchmarks showed a notable improvement in response latency after the migration was completed. The operations team documented all configuration changes and created runbooks for common incident response scenarios. Additionally, the security team conducted a comprehensive audit of all service endpoints and updated the firewall rules accordingly. The monitoring infrastructure was enhanced with new dashboards and alerting configurations to provide better visibility into system health and performance metrics across all environments."
          }
        ]
      }
      """
    Then the response status code should be 200
    And the response body should contain "The deployment pipeline for the cloud-native application underwent significant changes"
    And the response body should contain "performance metrics across all environments"

    Given I authenticate using basic auth as "admin"
    When I delete the API "pc-lifecycle-api"
    Then the response should be successful
