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

@semantic-tool-filtering
Feature: Semantic Tool Filtering policy
  As an API developer
  I want to keep only tools relevant to the user query
  So that the LLM receives a smaller, focused tool set

  Background:
    Given the gateway services are running

  Scenario: By Rank mode keeps only the most relevant tool in JSON payload
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: semantic-tool-filtering-rank-provider
      spec:
        displayName: Semantic Tool Filtering - By Rank
        version: v1.0
        context: /semantic-tool-filtering-rank
        template: gemini
        upstream:
          url: http://echo-backend:80/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: semantic-tool-filtering
            version: v1
            paths:
              - path: /gemini/v1/models/gemini-1.5-flash-002:generateContent
                methods: [POST]
                params:
                  selectionMode: "By Rank"
                  limit: 1
                  queryJSONPath: "$.contents[0].parts[0].text"
                  toolsJSONPath: "$.tools[0].function_declarations"
                  userQueryIsJson: true
                  toolsIsJson: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-tool-filtering-rank/gemini/v1/models/gemini-1.5-flash-002:generateContent" to be ready with method "POST" and body '{"contents":[{"role":"user","parts":[{"text":"warmup"}]}],"tools":[{"function_declarations":[{"name":"warmup_tool","description":"Warmup tool"}]}]}'

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-tool-filtering-rank/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """
      {
        "contents": [
          {
            "role": "user",
            "parts": [
              {
                "text": "Please check weather forecast for London"
              }
            ]
          }
        ],
        "tools": [
          {
            "function_declarations": [
              {
                "name": "send_email",
                "description": "Send email notifications to recipients"
              },
              {
                "name": "get_weather",
                "description": "Get current weather and short-term forecast for a city"
              },
              {
                "name": "book_venue",
                "description": "Reserve a conference room for meetings"
              }
            ]
          }
        ]
      }
      """

    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response array field "json.tools[0].function_declarations" should have 1 items

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "semantic-tool-filtering-rank-provider"
    Then the response should be successful

  Scenario: Mistral request keeps only the top 2 relevant tools
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: semantic-tool-filtering-mistral-provider
      spec:
        displayName: Semantic Tool Filtering - Mistral
        version: v1.0
        context: /semantic-tool-filtering-mistral
        template: mistralai
        upstream:
          url: http://echo-backend:80/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST]
        policies:
          - name: semantic-tool-filtering
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  selectionMode: "By Rank"
                  limit: 2
                  queryJSONPath: "$.messages[0].content"
                  toolsJSONPath: "$.tools[*].function"
                  userQueryIsJson: true
                  toolsIsJson: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-tool-filtering-mistral/chat/completions" to be ready with method "POST" and body '{"model":"mistral-large-latest","messages":[{"role":"user","content":"warmup"}],"tools":[{"type":"function","function":{"name":"warmup_tool","description":"Warmup tool","parameters":{"type":"object","properties":{}}}}]}'

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-tool-filtering-mistral/chat/completions" with body:
      """
      {
        "model": "mistral-large-latest",
        "messages": [
          {
            "role": "user",
            "content": "Give weather in Colombo Sri lanka."
          }
        ],
        "tools": [
          {
            "type": "function",
            "function": {
              "name": "get_weather",
              "description": "Get current weather",
              "parameters": {
                "type": "object",
                "properties": {}
              }
            }
          },
          {
            "type": "function",
            "function": {
              "name": "send_email",
              "description": "Send email notification",
              "parameters": {
                "type": "object",
                "properties": {}
              }
            }
          },
          {
            "type": "function",
            "function": {
              "name": "calculate_tax",
              "description": "Calculate sales tax",
              "parameters": {
                "type": "object",
                "properties": {}
              }
            }
          },
          {
            "type": "function",
            "function": {
              "name": "search_database",
              "description": "Search internal records",
              "parameters": {
                "type": "object",
                "properties": {}
              }
            }
          },
          {
            "type": "function",
            "function": {
              "name": "reset_password",
              "description": "Reset user credentials",
              "parameters": {
                "type": "object",
                "properties": {}
              }
            }
          }
        ]
      }
      """

    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response array field "json.tools" should have 2 items

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "semantic-tool-filtering-mistral-provider"
    Then the response should be successful
