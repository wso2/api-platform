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

@word-count-guardrail
Feature: Word count guardrail policy
  As an API developer
  I want to limit the word count in requests
  So that I can prevent overly long or short content from being processed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request exceeding the maximum word count is blocked
    Given I generate a unique value from "wcg-max" and store it as "apiName"
    And I generate a unique API version from "wcg-max" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-max" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This has exactly five words
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is a much longer message that contains way more than ten words and should fail validation
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request below the minimum word count is blocked
    Given I generate a unique value from "wcg-min" and store it as "apiName"
    And I generate a unique API version from "wcg-min" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-min" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":5,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Too short
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This message has exactly seven words total
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath-extracted field word count is enforced
    Given I generate a unique value from "wcg-jsonpath" and store it as "apiName"
    And I generate a unique API version from "wcg-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/chat","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":20,"jsonPath":"$.message"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "message": "Hello this is a short message",
        "metadata": "This field has many many many words but should be ignored by the guardrail"
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "message": "This is a very long message that contains way more than twenty words and should definitely fail the word count validation because it exceeds the maximum limit",
        "metadata": "short"
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted range blocks content that falls inside the excluded window
    Given I generate a unique value from "wcg-invert" and store it as "apiName"
    And I generate a unique API version from "wcg-invert" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-invert" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":5,"max":10,"jsonPath":"","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This has exactly seven words here
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Only three words
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is a much longer message that has way more than ten words so it passes
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response includes the assessment detail when showAssessment is enabled
    Given I generate a unique value from "wcg-assessment" and store it as "apiName"
    And I generate a unique API version from "wcg-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":"","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This message has way more than five words and should fail
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "word"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Empty request body is blocked
    Given I generate a unique value from "wcg-empty" and store it as "apiName"
    And I generate a unique API version from "wcg-empty" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Word counts exactly at the boundaries are accepted
    Given I generate a unique value from "wcg-boundary" and store it as "apiName"
    And I generate a unique API version from "wcg-boundary" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-boundary" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":5,"max":10,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One two three four five
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One two three four five six seven eight nine ten
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One two three four
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One two three four five six seven eight nine ten eleven
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A request-only word count policy still declares a response threshold without enforcing it
    Given I generate a unique value from "wcg-combined" and store it as "apiName"
    And I generate a unique API version from "wcg-combined" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":""},"response":{"min":1,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Five words in this request
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is a much longer message that contains way more than ten words and should fail
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Nested JSONPath-extracted field word count is enforced
    Given I generate a unique value from "wcg-nested" and store it as "apiName"
    And I generate a unique API version from "wcg-nested" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/chat","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":"$.data.content"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "data": {
          "content": "Short nested message here",
          "timestamp": "2025-01-01"
        },
        "metadata": "This outer field has many words but should be ignored completely"
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "data": {
          "content": "This nested content field has way more than ten words and should fail validation",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction of a missing field is blocked
    Given I generate a unique value from "wcg-invalid-path" and store it as "apiName"
    And I generate a unique API version from "wcg-invalid-path" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-invalid-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":"$.nonexistent.field"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "message": "This field exists but not the one we are looking for"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "WORD_COUNT_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Punctuation and hyphenation do not distort the word count
    Given I generate a unique value from "wcg-punctuation" and store it as "apiName"
    And I generate a unique API version from "wcg-punctuation" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-punctuation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    # "Hello... world!!! How are you?" is 5 words once punctuation is ignored.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello... world!!! How are you?
      """
    Then the response status code should be 200

    # "well-known" counts as one word.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is a well-known fact
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Plain-text content is word-counted the same as JSON content
    Given I generate a unique value from "wcg-plaintext" and store it as "apiName"
    And I generate a unique API version from "wcg-plaintext" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-plaintext" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is plain text with seven words
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This plain text message has way more than the allowed ten words limit
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response carries the complete guardrail error contract
    Given I generate a unique value from "wcg-error-structure" and store it as "apiName"
    And I generate a unique API version from "wcg-error-structure" and store it as "apiVersion"
    And I generate a unique API context from "/wcg-error-structure" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":"","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This message has more than five words and will fail
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "WORD_COUNT_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "word-count-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
