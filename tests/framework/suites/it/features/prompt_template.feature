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

@prompt-template
Feature: Prompt Template
  As an API developer
  I want to use reusable prompt templates with parameters
  So that I can simplify client requests and enforce consistent prompts

  Background:
    Given the gateway services are running

  # ============================================================================
  # BASIC TEMPLATE REPLACEMENT SCENARIOS
  # ============================================================================

  Scenario: Replace simple template with parameters
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-simple-api
      spec:
        displayName: Prompt Template Simple API
        version: v1.0
        context: /prompt-template-simple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "translate", "template": "Translate from [[from]] to [[to]]: [[text]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-simple/v1.0/health" until status 200

    # Send request with template reference
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-simple/v1.0/complete" with body:
      """
      {
        "prompt": "template://translate?from=english&to=spanish&text=Hello world"
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-simple-api"
    Then the response status code should be 200

  Scenario: Replace template without query parameters
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-no-params-api
      spec:
        displayName: Prompt Template No Params API
        version: v1.0
        context: /prompt-template-no-params/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "greeting", "template": "You are a friendly assistant. Greet the user warmly."}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-no-params/v1.0/health" until status 200

    # Send request with template reference (no query params)
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-no-params/v1.0/complete" with body:
      """
      {
        "prompt": "template://greeting"
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-no-params-api"
    Then the response status code should be 200

  # ============================================================================
  # MULTIPLE TEMPLATES SCENARIOS
  # ============================================================================

  Scenario: Use multiple templates in configuration
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-multi-config-api
      spec:
        displayName: Prompt Template Multi Config API
        version: v1.0
        context: /prompt-template-multi-config/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "translate", "template": "Translate from [[from]] to [[to]]: [[text]]"}, {"name": "summarize", "template": "Summarize in [[length]] sentences: [[content]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-multi-config/v1.0/health" until status 200

    # Use first template
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-multi-config/v1.0/complete" with body:
      """
      {
        "prompt": "template://translate?from=english&to=french&text=Good morning"
      }
      """
    Then the response status code should be 200

    # Use second template
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-multi-config/v1.0/complete" with body:
      """
      {
        "prompt": "template://summarize?length=3&content=This is a long article"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-multi-config-api"
    Then the response status code should be 200

  Scenario: Use multiple template references in single request
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-multi-ref-api
      spec:
        displayName: Prompt Template Multi Ref API
        version: v1.0
        context: /prompt-template-multi-ref/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "system", "template": "You are a [[role]] assistant."}, {"name": "task", "template": "Your task is to [[action]]."}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-multi-ref/v1.0/health" until status 200

    # Send request with multiple template references
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-multi-ref/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "system", "content": "template://system?role=helpful"},
          {"role": "system", "content": "template://task?action=answer questions"},
          {"role": "user", "content": "Hello"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-multi-ref-api"
    Then the response status code should be 200

  # ============================================================================
  # URL ENCODING AND SPECIAL CHARACTERS
  # ============================================================================

  Scenario: Handle URL encoded parameters
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-encoded-api
      spec:
        displayName: Prompt Template Encoded API
        version: v1.0
        context: /prompt-template-encoded/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "analyze", "template": "Analyze this: [[text]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-encoded/v1.0/health" until status 200

    # Send request with URL encoded parameters (spaces, special chars)
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-encoded/v1.0/complete" with body:
      """
      {
        "prompt": "template://analyze?text=Hello%20World%21%20How%20are%20you%3F"
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-encoded-api"
    Then the response status code should be 200

  Scenario: Handle parameters with special characters
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-special-chars-api
      spec:
        displayName: Prompt Template Special Chars API
        version: v1.0
        context: /prompt-template-special-chars/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "format", "template": "Format: [[pattern]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-special-chars/v1.0/health" until status 200

    # Send request with special characters in parameters
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-special-chars/v1.0/complete" with body:
      """
      {
        "prompt": "template://format?pattern=JSON"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-special-chars-api"
    Then the response status code should be 200

  # ============================================================================
  # TEMPLATE IN DIFFERENT LOCATIONS
  # ============================================================================

  Scenario: Replace template in prompt field
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-prompt-field-api
      spec:
        displayName: Prompt Template Prompt Field API
        version: v1.0
        context: /prompt-template-prompt-field/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "question", "template": "Answer this question: [[q]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-prompt-field/v1.0/health" until status 200

    # Template in prompt field
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-prompt-field/v1.0/complete" with body:
      """
      {
        "prompt": "template://question?q=What is AI"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-prompt-field-api"
    Then the response status code should be 200

  Scenario: Replace template in message content
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-message-api
      spec:
        displayName: Prompt Template Message API
        version: v1.0
        context: /prompt-template-message/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "intro", "template": "I need help with [[topic]]."}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-message/v1.0/health" until status 200

    # Template in message content
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-message/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "template://intro?topic=coding"}
        ]
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-message-api"
    Then the response status code should be 200

  # ============================================================================
  # ERROR SCENARIOS
  # ============================================================================

  Scenario: Handle template not found error
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-not-found-api
      spec:
        displayName: Prompt Template Not Found API
        version: v1.0
        context: /prompt-template-not-found/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "existing", "template": "This exists"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-not-found/v1.0/health" until status 200

    # Reference non-existent template
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-not-found/v1.0/complete" with body:
      """
      {
        "prompt": "template://nonexistent?param=value"
      }
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the JSON response field "type" should be "PROMPT_TEMPLATE_ERROR"
    And the JSON response field "message" should be:
      """
      Error resolving templates: template "nonexistent" not found
      """

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-not-found-api"
    Then the response status code should be 200

  Scenario: Handle empty request body
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-empty-api
      spec:
        displayName: Prompt Template Empty API
        version: v1.0
        context: /prompt-template-empty/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "test", "template": "Test"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-empty/v1.0/health" until status 200

    # Send empty body - should pass through unchanged
    When I send a "POST" request to "/prompt-template-empty/v1.0/complete" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-empty-api"
    Then the response status code should be 200

  Scenario: Handle request without template reference
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-no-ref-api
      spec:
        displayName: Prompt Template No Ref API
        version: v1.0
        context: /prompt-template-no-ref/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "test", "template": "Test [[param]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-no-ref/v1.0/health" until status 200

    # Send request without template reference - should pass through unchanged
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-no-ref/v1.0/complete" with body:
      """
      {
        "prompt": "This is a regular prompt without template reference"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-no-ref-api"
    Then the response status code should be 200

  # ============================================================================
  # REAL-WORLD SCENARIOS
  # ============================================================================

  Scenario: Translation template with language pairs
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-translation-api
      spec:
        displayName: Prompt Template Translation API
        version: v1.0
        context: /prompt-template-translation/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /translate
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "translate", "template": "You are a professional translator. Translate the following text from [[sourceLang]] to [[targetLang]]. Maintain the original tone and context.\n\nText: [[text]]\n\nTranslation:"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-translation/v1.0/health" until status 200

    # Use translation template
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-translation/v1.0/translate" with body:
      """
      {
        "prompt": "template://translate?sourceLang=English&targetLang=French&text=Hello world"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-translation-api"
    Then the response status code should be 200

  Scenario: Code review template with language parameter
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-code-review-api
      spec:
        displayName: Prompt Template Code Review API
        version: v1.0
        context: /prompt-template-code-review/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /review
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "review", "template": "Review this [[language]] code for bugs, performance issues, and best practices:\n\n[[code]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-code-review/v1.0/health" until status 200

    # Use code review template
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-code-review/v1.0/review" with body:
      """
      {
        "prompt": "template://review?language=Python&code=def add(a, b): return a + b"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-code-review-api"
    Then the response status code should be 200

  Scenario: Sentiment analysis template
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: prompt-template-sentiment-api
      spec:
        displayName: Prompt Template Sentiment API
        version: v1.0
        context: /prompt-template-sentiment/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /analyze
            policies:
              - name: prompt-template
                version: v1
                params:
                  templates: [{"name": "sentiment", "template": "Analyze the sentiment of the following text. Classify as positive, negative, or neutral:\n\n[[text]]"}]
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-template-sentiment/v1.0/health" until status 200

    # Use sentiment analysis template
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/prompt-template-sentiment/v1.0/analyze" with body:
      """
      {
        "prompt": "template://sentiment?text=This product is amazing!"
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-template-sentiment-api"
    Then the response status code should be 200
