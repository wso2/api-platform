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

@prompt-decorator
Feature: Prompt decorator
  As an API developer
  I want to modify LLM prompts by adding custom instructions
  So that I can enforce consistent behavior across all LLM requests
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Prepend a system message to chat completion messages
    Given I generate a unique value from "pd-prepend" and store it as "apiName"
    And I generate a unique API version from "pd-prepend" and store it as "apiVersion"
    And I generate a unique API context from "/pd-prepend" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"You are a helpful assistant."}]},"jsonPath":"$.messages","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "You are a helpful assistant."
    And the JSON response field "json.messages[1].content" should be "Hello"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Append a system message to chat completion messages
    Given I generate a unique value from "pd-append" and store it as "apiName"
    And I generate a unique API version from "pd-append" and store it as "apiVersion"
    And I generate a unique API context from "/pd-append" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"Always be concise."}]},"jsonPath":"$.messages","append":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"system","content":"You are a translator."},{"role":"user","content":"Translate to French: Hello"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "You are a translator."
    And the JSON response field "json.messages[2].content" should be "Always be concise."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Prepend multiple decoration messages to chat
    Given I generate a unique value from "pd-multi" and store it as "apiName"
    And I generate a unique API version from "pd-multi" and store it as "apiVersion"
    And I generate a unique API context from "/pd-multi" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"You are an expert."},{"role":"system","content":"Always verify facts."}]},"jsonPath":"$.messages","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"What is AI?"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "You are an expert."
    And the JSON response field "json.messages[1].content" should be "Always verify facts."
    And the JSON response field "json.messages[2].content" should be "What is AI?"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Prepend an instruction to a text prompt string
    Given I generate a unique value from "pd-text-prepend" and store it as "apiName"
    And I generate a unique API version from "pd-text-prepend" and store it as "apiVersion"
    And I generate a unique API context from "/pd-text-prepend" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"text":"Summarize the following:"},"jsonPath":"$.prompt","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"AI is artificial intelligence."}
      """
    Then the response should be valid JSON
    And the JSON response field "json.prompt" should contain "Summarize the following:"
    And the JSON response field "json.prompt" should contain "AI is artificial intelligence."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Append an instruction to a text prompt string
    Given I generate a unique value from "pd-text-append" and store it as "apiName"
    And I generate a unique API version from "pd-text-append" and store it as "apiVersion"
    And I generate a unique API context from "/pd-text-append" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"text":"Please be brief."},"jsonPath":"$.prompt","append":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"Explain quantum computing."}
      """
    Then the response should be valid JSON
    And the JSON response field "json.prompt" should contain "Explain quantum computing."
    And the JSON response field "json.prompt" should contain "Please be brief."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Decorate the last message's content, appended
    Given I generate a unique value from "pd-last-append" and store it as "apiName"
    And I generate a unique API version from "pd-last-append" and store it as "apiVersion"
    And I generate a unique API context from "/pd-last-append" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"text":"Format your answer in JSON."},"jsonPath":"$.messages[-1].content","append":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"List 3 colors"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "You are helpful."
    And the JSON response field "json.messages[1].content" should contain "List 3 colors"
    And the JSON response field "json.messages[1].content" should contain "Format your answer in JSON."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Decorate the last message's content, prepended
    Given I generate a unique value from "pd-last-prepend" and store it as "apiName"
    And I generate a unique API version from "pd-last-prepend" and store it as "apiVersion"
    And I generate a unique API context from "/pd-last-prepend" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"text":"Be creative!"},"jsonPath":"$.messages[-1].content","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Write a poem"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should contain "Be creative!"
    And the JSON response field "json.messages[0].content" should contain "Write a poem"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Decorate a nested prompt field
    Given I generate a unique value from "pd-nested" and store it as "apiName"
    And I generate a unique API version from "pd-nested" and store it as "apiVersion"
    And I generate a unique API context from "/pd-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"text":"Answer concisely:"},"jsonPath":"$.request.prompt","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"request":{"prompt":"What is machine learning?","temperature":0.7}}
      """
    Then the response should be valid JSON
    And the JSON response field "json.request.prompt" should contain "Answer concisely:"
    And the JSON response field "json.request.prompt" should contain "What is machine learning?"
    And the response body should contain "0.7"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle empty request body
    Given I generate a unique value from "pd-empty" and store it as "apiName"
    And I generate a unique API version from "pd-empty" and store it as "apiVersion"
    And I generate a unique API context from "/pd-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"Test"}]},"jsonPath":"$.messages","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"warm-up"}]}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the response body should contain "PROMPT_DECORATOR_ERROR"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle a JSONPath that does not resolve
    Given I generate a unique value from "pd-invalid-path" and store it as "apiName"
    And I generate a unique API version from "pd-invalid-path" and store it as "apiVersion"
    And I generate a unique API context from "/pd-invalid-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"Test"}]},"jsonPath":"$.nonexistent.field","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 500 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response should be valid JSON
    And the response body should contain "PROMPT_DECORATOR_ERROR"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle an invalid JSON payload
    Given I generate a unique value from "pd-invalid-json" and store it as "apiName"
    And I generate a unique API version from "pd-invalid-json" and store it as "apiVersion"
    And I generate a unique API context from "/pd-invalid-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-decorator","version":"v1","params":{"promptDecoratorConfig":{"messages":[{"role":"system","content":"Test"}]},"jsonPath":"$.messages","append":false}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"warm-up"}]}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {invalid json}
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the response body should contain "PROMPT_DECORATOR_ERROR"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
