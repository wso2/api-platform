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

@regex-guardrail
Feature: Regex guardrail policy
  As an API developer
  I want to validate content against regular expression patterns
  So that I can enforce content rules and prevent unwanted data

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Content matching the configured pattern is allowed, non-matching content is blocked
    Given I generate a unique value from "rg-match" and store it as "apiName"
    And I generate a unique API version from "rg-match" and store it as "apiVersion"
    And I generate a unique API context from "/rg-match" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[A-Z]{3}-[0-9]{4}$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      ABC-1234
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      invalid-format
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "REGEX_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted pattern blocks banned words
    Given I generate a unique value from "rg-profanity" and store it as "apiName"
    And I generate a unique API version from "rg-profanity" and store it as "apiVersion"
    And I generate a unique API context from "/rg-profanity" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"(badword\|profanity\|offensive)","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is a clean message with no issues.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This message contains a badword and should fail.
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Email address format is validated
    Given I generate a unique value from "rg-email" and store it as "apiName"
    And I generate a unique API version from "rg-email" and store it as "apiVersion"
    And I generate a unique API context from "/rg-email" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      user@example.com
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      not-an-email
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted logic passes content without a social-security-number pattern
    Given I generate a unique value from "rg-invert-ssn" and store it as "apiName"
    And I generate a unique API version from "rg-invert-ssn" and store it as "apiVersion"
    And I generate a unique API context from "/rg-invert-ssn" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"\\\\b\\\\d{3}-\\\\d{2}-\\\\d{4}\\\\b","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is safe content without sensitive data.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      My SSN is 123-45-6789 please keep it safe.
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted logic blocks a credit-card-number pattern
    Given I generate a unique value from "rg-invert-cc" and store it as "apiName"
    And I generate a unique API version from "rg-invert-cc" and store it as "apiVersion"
    And I generate a unique API context from "/rg-invert-cc" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"\\\\b\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}[\\\\s-]?\\\\d{4}\\\\b","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Please process my order for 100 USD.
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Card number: 4532-1234-5678-9012
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath-extracted field is validated against the pattern
    Given I generate a unique value from "rg-jsonpath" and store it as "apiName"
    And I generate a unique API version from "rg-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/rg-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"$.code","regex":"^[A-Z]{2}[0-9]{3}$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "code": "AB123",
        "message": "This field can be anything INVALID999"
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "code": "invalid",
        "message": "AB123"
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Nested JSONPath-extracted field is validated against the pattern
    Given I generate a unique value from "rg-nested" and store it as "apiName"
    And I generate a unique API version from "rg-nested" and store it as "apiVersion"
    And I generate a unique API context from "/rg-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"$.user.username","regex":"^[a-z0-9_]{3,16}$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "user": {
          "username": "valid_user123",
          "email": "invalid@format"
        }
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "user": {
          "username": "Invalid-Username!",
          "email": "valid@example.com"
        }
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction of a missing field is blocked
    Given I generate a unique value from "rg-invalid-path" and store it as "apiName"
    And I generate a unique API version from "rg-invalid-path" and store it as "apiVersion"
    And I generate a unique API context from "/rg-invalid-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"$.nonexistent.field","regex":".*"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "message": "This field exists but not the one we want"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "REGEX_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response includes the assessment detail when showAssessment is enabled
    Given I generate a unique value from "rg-assessment" and store it as "apiName"
    And I generate a unique API version from "rg-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/rg-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[0-9]{5}$","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      12345ABC
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"
    And the response body should contain "regular expression"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Empty request body passes an unrestricted pattern
    Given I generate a unique value from "rg-empty" and store it as "apiName"
    And I generate a unique API version from "rg-empty" and store it as "apiVersion"
    And I generate a unique API context from "/rg-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":".*"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An alternation pattern accepts any of its listed values
    Given I generate a unique value from "rg-complex" and store it as "apiName"
    And I generate a unique API version from "rg-complex" and store it as "apiVersion"
    And I generate a unique API context from "/rg-complex" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^(approved\|pending\|rejected)$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      approved
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      pending
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      unknown
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Matching is case-sensitive by default
    Given I generate a unique value from "rg-case-sensitive" and store it as "apiName"
    And I generate a unique API version from "rg-case-sensitive" and store it as "apiVersion"
    And I generate a unique API context from "/rg-case-sensitive" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^UPPERCASE$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      UPPERCASE
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      uppercase
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An inline case-insensitive flag matches regardless of letter case
    Given I generate a unique value from "rg-case-insensitive" and store it as "apiName"
    And I generate a unique API version from "rg-case-insensitive" and store it as "apiVersion"
    And I generate a unique API context from "/rg-case-insensitive" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"(?i)^hello$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      hello
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      HELLO
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      HeLLo
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A request-only regex policy still declares a response pattern without enforcing it
    Given I generate a unique value from "rg-combined" and store it as "apiName"
    And I generate a unique API version from "rg-combined" and store it as "apiVersion"
    And I generate a unique API context from "/rg-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[A-Za-z0-9 ]+$"},"response":{"jsonPath":"","regex":".*"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Valid Content 123
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Invalid@Content#
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: International phone number format is validated
    Given I generate a unique value from "rg-phone" and store it as "apiName"
    And I generate a unique API version from "rg-phone" and store it as "apiVersion"
    And I generate a unique API context from "/rg-phone" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^\\\\+?[1-9]\\\\d{1,14}$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      +14155552671
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      not-a-phone
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response carries the complete guardrail error contract
    Given I generate a unique value from "rg-error-structure" and store it as "apiName"
    And I generate a unique API version from "rg-error-structure" and store it as "apiVersion"
    And I generate a unique API context from "/rg-error-structure" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[0-9]+$","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      abc123
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "REGEX_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "regex-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A response-side pattern is validated against a reflected JSON field
    Given I generate a unique value from "rg-response-json" and store it as "apiName"
    And I generate a unique API version from "rg-response-json" and store it as "apiVersion"
    And I generate a unique API context from "/rg-response-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"regex-guardrail","version":"v1","params":{"response":{"jsonPath":"$.method","regex":"^POST$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An inverted response-side pattern allows a response with no forbidden words
    Given I generate a unique value from "rg-response-block" and store it as "apiName"
    And I generate a unique API version from "rg-response-block" and store it as "apiVersion"
    And I generate a unique API context from "/rg-response-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/echo","policies":[{"name":"regex-guardrail","version":"v1","params":{"response":{"jsonPath":"","regex":"(internal\|confidential\|secret)","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/echo"
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A response-side pattern validates the reflected host header format
    Given I generate a unique value from "rg-response-format" and store it as "apiName"
    And I generate a unique API version from "rg-response-format" and store it as "apiVersion"
    And I generate a unique API context from "/rg-response-format" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/echo","policies":[{"name":"regex-guardrail","version":"v1","params":{"response":{"jsonPath":"$.headers.Host[0]","regex":"^[a-zA-Z0-9.-]+:[0-9]+$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/echo" with body:
      """
      test
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Unicode letters and numbers are accepted, symbols are blocked
    Given I generate a unique value from "rg-unicode" and store it as "apiName"
    And I generate a unique API version from "rg-unicode" and store it as "apiVersion"
    And I generate a unique API context from "/rg-unicode" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[\\\\p{L}\\\\p{N}\\\\s]+$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello 世界 123
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello@World!
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Content in different international scripts is accepted
    Given I generate a unique value from "rg-international" and store it as "apiName"
    And I generate a unique API version from "rg-international" and store it as "apiVersion"
    And I generate a unique API context from "/rg-international" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":".*"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      你好世界
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      مرحبا بالعالم
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Привет мир
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted alphanumeric-only pattern allows emoji through
    Given I generate a unique value from "rg-emoji" and store it as "apiName"
    And I generate a unique API version from "rg-emoji" and store it as "apiVersion"
    And I generate a unique API context from "/rg-emoji" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[a-zA-Z0-9\\\\s]+$","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello 😀 🌍
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Hello World 123
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Letters with diacritics are accepted, digits are blocked
    Given I generate a unique value from "rg-diacritics" and store it as "apiName"
    And I generate a unique API version from "rg-diacritics" and store it as "apiVersion"
    And I generate a unique API context from "/rg-diacritics" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"jsonPath":"","regex":"^[\\\\p{L}\\\\s]+$"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      café résumé naïve
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      café 123
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
