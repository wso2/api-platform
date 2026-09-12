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

@sentence-count-guardrail
Feature: Sentence count guardrail policy
  As an API developer
  I want to limit the sentence count in requests
  So that I can prevent overly long or short content from being processed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request exceeding the maximum sentence count is blocked
    Given I generate a unique value from "scg-max" and store it as "apiName"
    And I generate a unique API version from "scg-max" and store it as "apiVersion"
    And I generate a unique API context from "/scg-max" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":3,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello world. This is fine.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four. Five.
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request below the minimum sentence count is blocked
    Given I generate a unique value from "scg-min" and store it as "apiName"
    And I generate a unique API version from "scg-min" and store it as "apiVersion"
    And I generate a unique API context from "/scg-min" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":3,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Only one sentence here.
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      First sentence. Second sentence. Third sentence. Fourth sentence.
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath-extracted field sentence count is enforced
    Given I generate a unique value from "scg-jsonpath" and store it as "apiName"
    And I generate a unique API version from "scg-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/scg-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/chat","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":"$.message"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "message": "Hello there. How are you?",
        "metadata": "This field has many sentences. One here. Two here. Three here. Four here. But should be ignored."
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "message": "First. Second. Third. Fourth. Fifth. Sixth. This exceeds the limit!",
        "metadata": "short"
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Nested JSONPath-extracted field sentence count is enforced
    Given I generate a unique value from "scg-nested" and store it as "apiName"
    And I generate a unique API version from "scg-nested" and store it as "apiVersion"
    And I generate a unique API context from "/scg-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/chat","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":3,"jsonPath":"$.data.content"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "data": {
          "content": "Short message. Just two sentences.",
          "timestamp": "2025-01-01"
        },
        "metadata": "This outer field has many sentences. But should be ignored. Completely ignored."
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {
        "data": {
          "content": "First sentence. Second sentence. Third sentence. Fourth sentence. This exceeds!",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction of a missing field is blocked
    Given I generate a unique value from "scg-invalid-path" and store it as "apiName"
    And I generate a unique API version from "scg-invalid-path" and store it as "apiVersion"
    And I generate a unique API context from "/scg-invalid-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":10,"jsonPath":"$.nonexistent.field"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "message": "This field exists. But not the one we are looking for."
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "SENTENCE_COUNT_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted range blocks content that falls inside the excluded window
    Given I generate a unique value from "scg-invert" and store it as "apiName"
    And I generate a unique API version from "scg-invert" and store it as "apiVersion"
    And I generate a unique API context from "/scg-invert" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":2,"max":4,"jsonPath":"","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One sentence. Two sentences. Three sentences.
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Only one sentence here.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four. Five. Six sentences here!
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response includes the assessment detail when showAssessment is enabled
    Given I generate a unique value from "scg-assessment" and store it as "apiName"
    And I generate a unique API version from "scg-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/scg-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":2,"jsonPath":"","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      First. Second. Third. Fourth. This exceeds the max of two sentences!
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "sentence"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Empty request body is blocked
    Given I generate a unique value from "scg-empty" and store it as "apiName"
    And I generate a unique API version from "scg-empty" and store it as "apiVersion"
    And I generate a unique API context from "/scg-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Sentence counts exactly at the boundaries are accepted
    Given I generate a unique value from "scg-boundary" and store it as "apiName"
    And I generate a unique API version from "scg-boundary" and store it as "apiVersion"
    And I generate a unique API context from "/scg-boundary" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":2,"max":4,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      First sentence. Second sentence.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Only one sentence.
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four. Five.
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A request-only sentence count policy still declares a response threshold without enforcing it
    Given I generate a unique value from "scg-combined" and store it as "apiName"
    And I generate a unique API version from "scg-combined" and store it as "apiVersion"
    And I generate a unique API context from "/scg-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""},"response":{"min":1,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Two sentences here. This should pass.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four. Five. Six sentences total!
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple punctuation marks do not distort the sentence count
    Given I generate a unique value from "scg-punctuation" and store it as "apiName"
    And I generate a unique API version from "scg-punctuation" and store it as "apiVersion"
    And I generate a unique API context from "/scg-punctuation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":3,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    # Repeated terminators ("!!!", "???") do not create extra sentences.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello!!! World???
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      What! Really? Yes.
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Plain-text content is sentence-counted the same as JSON content
    Given I generate a unique value from "scg-plaintext" and store it as "apiName"
    And I generate a unique API version from "scg-plaintext" and store it as "apiVersion"
    And I generate a unique API context from "/scg-plaintext" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is plain text. It has three sentences. All should be counted.
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      One. Two. Three. Four. Five. Six. This exceeds the limit!
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response carries the complete guardrail error contract
    Given I generate a unique value from "scg-error-structure" and store it as "apiName"
    And I generate a unique API version from "scg-error-structure" and store it as "apiVersion"
    And I generate a unique API context from "/scg-error-structure" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"sentence-count-guardrail","version":"v1","params":{"request":{"min":1,"max":2,"jsonPath":"","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      First sentence. Second sentence. Third sentence. This will fail!
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "SENTENCE_COUNT_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "sentence-count-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
