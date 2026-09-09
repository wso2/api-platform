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

@prompt-template
Feature: Prompt template
  As an API developer
  I want to use reusable prompt templates with parameters
  So that I can simplify client requests and enforce consistent prompts
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Replace simple template with parameters
    Given I generate a unique value from "pt-simple" and store it as "apiName"
    And I generate a unique API version from "pt-simple" and store it as "apiVersion"
    And I generate a unique API context from "/pt-simple" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"translate","template":"Translate from [[from]] to [[to]]: [[text]]"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"template://translate?from=english&to=spanish&text=Hello world"}
      """
    Then the response should be valid JSON
    And the JSON response field "json.prompt" should be "Translate from english to spanish: Hello world"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Replace template without query parameters
    Given I generate a unique value from "pt-no-params" and store it as "apiName"
    And I generate a unique API version from "pt-no-params" and store it as "apiVersion"
    And I generate a unique API context from "/pt-no-params" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"greeting","template":"You are a friendly assistant. Greet the user warmly."}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"template://greeting"}
      """
    Then the response should be valid JSON
    And the JSON response field "json.prompt" should be "You are a friendly assistant. Greet the user warmly."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Use multiple templates in configuration
    Given I generate a unique value from "pt-multi-config" and store it as "apiName"
    And I generate a unique API version from "pt-multi-config" and store it as "apiVersion"
    And I generate a unique API context from "/pt-multi-config" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"translate","template":"Translate from [[from]] to [[to]]: [[text]]"},{"name":"summarize","template":"Summarize in [[length]] sentences: [[content]]"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # First template
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"template://translate?from=english&to=french&text=Good morning"}
      """
    Then the JSON response field "json.prompt" should be "Translate from english to french: Good morning"

    # Second template
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" with body:
      """
      {"prompt":"template://summarize?length=3&content=This is a long article"}
      """
    Then the response status code should be 200
    And the JSON response field "json.prompt" should be "Summarize in 3 sentences: This is a long article"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Use multiple template references in a single request
    Given I generate a unique value from "pt-multi-ref" and store it as "apiName"
    And I generate a unique API version from "pt-multi-ref" and store it as "apiVersion"
    And I generate a unique API context from "/pt-multi-ref" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"system","template":"You are a [[role]] assistant."},{"name":"task","template":"Your task is to [[action]]."}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"system","content":"template://system?role=helpful"},{"role":"system","content":"template://task?action=answer%20questions"},{"role":"user","content":"Hello"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "You are a helpful assistant."
    And the JSON response field "json.messages[1].content" should be "Your task is to answer questions."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle URL encoded parameters
    Given I generate a unique value from "pt-encoded" and store it as "apiName"
    And I generate a unique API version from "pt-encoded" and store it as "apiVersion"
    And I generate a unique API context from "/pt-encoded" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"analyze","template":"Analyze this: [[text]]"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"template://analyze?text=Hello%20World%21%20How%20are%20you%3F"}
      """
    Then the response should be valid JSON
    And the JSON response field "json.prompt" should be "Analyze this: Hello World! How are you?"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Replace template in nested message content
    Given I generate a unique value from "pt-message" and store it as "apiName"
    And I generate a unique API version from "pt-message" and store it as "apiVersion"
    And I generate a unique API context from "/pt-message" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"intro","template":"I need help with [[topic]]."}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"template://intro?topic=coding"}]}
      """
    Then the response should be valid JSON
    And the JSON response field "json.messages[0].content" should be "I need help with coding."

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle template not found error
    Given I generate a unique value from "pt-not-found" and store it as "apiName"
    And I generate a unique API version from "pt-not-found" and store it as "apiVersion"
    And I generate a unique API context from "/pt-not-found" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"existing","template":"This exists"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"template://existing"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" with body:
      """
      {"prompt":"template://nonexistent?param=value"}
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the response body should contain "PROMPT_TEMPLATE_ERROR"
    And the response body should contain "not found"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle empty request body
    Given I generate a unique value from "pt-empty" and store it as "apiName"
    And I generate a unique API version from "pt-empty" and store it as "apiVersion"
    And I generate a unique API context from "/pt-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"test","template":"Test"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Empty body passes through unchanged - folds route readiness since there is nothing to
    # retry on 200 for; a fixed number of attempts isn't needed because an empty POST cannot
    # collide with any rate-limited state.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle request without a template reference
    Given I generate a unique value from "pt-no-ref" and store it as "apiName"
    And I generate a unique API version from "pt-no-ref" and store it as "apiVersion"
    And I generate a unique API context from "/pt-no-ref" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"test","template":"Test [[param]]"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"This is a regular prompt without template reference"}
      """
    Then the response should be valid JSON
    And the JSON response field "json.prompt" should be "This is a regular prompt without template reference"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Domain-specific templates that only differ in their template content and reference
  # parameters from the mechanics already covered above (parameter substitution into a
  # top-level "prompt" field). One outline with several examples avoids repeating the
  # deploy/health/cleanup boilerplate for each flavor while keeping every flavor documented.
  Scenario Outline: Domain-specific templates substitute parameters into the prompt field
    Given I generate a unique value from "pt-domain" and store it as "apiName"
    And I generate a unique API version from "pt-domain" and store it as "apiVersion"
    And I generate a unique API context from "/pt-domain" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/complete","policies":[{"name":"prompt-template","version":"v1","params":{"templates":[{"name":"<templateName>","template":"<template>"}]}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/complete" until status 200 with body:
      """
      {"prompt":"<reference>"}
      """
    Then the JSON response field "json.prompt" should be "<expected>"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

    Examples:
      | templateName | template                                                                            | reference                                                              | expected                                                       |
      | translate    | You are a professional translator. Translate from [[sourceLang]] to [[targetLang]]: [[text]] | template://translate?sourceLang=English&targetLang=French&text=Hello world | You are a professional translator. Translate from English to French: Hello world |
      | review       | Review this [[language]] code for bugs: [[code]]                                   | template://review?language=Python&code=def add(a, b): return a + b     | Review this Python code for bugs: def add(a, b): return a + b  |
      | sentiment    | Classify the sentiment of the following text: [[text]]                             | template://sentiment?text=This product is amazing!                     | Classify the sentiment of the following text: This product is amazing! |
