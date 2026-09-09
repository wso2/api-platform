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

@content-length-guardrail
Feature: Content length guardrail policy
  As an API developer
  I want to validate the byte length of request payloads
  So that I can enforce content size constraints and protect my backend services

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request within the configured min/max range is allowed
    Given I generate a unique value from "clg-valid" and store it as "apiName"
    And I generate a unique API version from "clg-valid" and store it as "apiVersion"
    And I generate a unique API context from "/clg-valid" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":10,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "This is a valid message with 50 bytes"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request below the configured minimum length is blocked
    Given I generate a unique value from "clg-below-min" and store it as "apiName"
    And I generate a unique API version from "clg-below-min" and store it as "apiVersion"
    And I generate a unique API context from "/clg-below-min" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":50,"max":200,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"msg": "short"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request above the configured maximum length is blocked
    Given I generate a unique value from "clg-above-max" and store it as "apiName"
    And I generate a unique API version from "clg-above-max" and store it as "apiVersion"
    And I generate a unique API context from "/clg-above-max" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":10,"max":50,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "This is a very long message that exceeds the maximum allowed length of 50 bytes"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Empty request body is blocked
    Given I generate a unique value from "clg-empty-body" and store it as "apiName"
    And I generate a unique API version from "clg-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/clg-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":1,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request exactly at the minimum boundary is allowed
    Given I generate a unique value from "clg-min-boundary" and store it as "apiName"
    And I generate a unique API version from "clg-min-boundary" and store it as "apiVersion"
    And I generate a unique API context from "/clg-min-boundary" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":20,"max":100,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"msg":"exactly20byte"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request exactly at the maximum boundary is allowed
    Given I generate a unique value from "clg-max-boundary" and store it as "apiName"
    And I generate a unique API version from "clg-max-boundary" and store it as "apiVersion"
    And I generate a unique API context from "/clg-max-boundary" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":10,"max":50,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This message is exactly 50 bytes."}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath-extracted field within range is allowed
    Given I generate a unique value from "clg-jsonpath-valid" and store it as "apiName"
    And I generate a unique API version from "clg-jsonpath-valid" and store it as "apiVersion"
    And I generate a unique API context from "/clg-jsonpath-valid" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":5,"max":50,"jsonPath":"$.message"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "Hello World"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath-extracted field outside range is blocked
    Given I generate a unique value from "clg-jsonpath-invalid" and store it as "apiName"
    And I generate a unique API version from "clg-jsonpath-invalid" and store it as "apiVersion"
    And I generate a unique API context from "/clg-jsonpath-invalid" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":5,"max":10,"jsonPath":"$.message"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "This is a very long message"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Nested JSONPath-extracted field within range is allowed
    Given I generate a unique value from "clg-nested-jsonpath" and store it as "apiName"
    And I generate a unique API version from "clg-nested-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/clg-nested-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":10,"max":100,"jsonPath":"$.data.description"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"data": {"description": "Valid content here"}}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction of a missing field is blocked
    Given I generate a unique value from "clg-missing-field" and store it as "apiName"
    And I generate a unique API version from "clg-missing-field" and store it as "apiVersion"
    And I generate a unique API context from "/clg-missing-field" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":5,"max":50,"jsonPath":"$.nonexistent"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "Hello"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted range blocks content that falls inside the excluded window
    Given I generate a unique value from "clg-inverted-excluded" and store it as "apiName"
    And I generate a unique API version from "clg-inverted-excluded" and store it as "apiVersion"
    And I generate a unique API context from "/clg-inverted-excluded" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":20,"max":50,"jsonPath":"","invert":true,"showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message": "30 bytes content"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"
    And the response body should contain "outside the range"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Inverted range allows content that falls outside the excluded window
    Given I generate a unique value from "clg-inverted-allowed" and store it as "apiName"
    And I generate a unique API version from "clg-inverted-allowed" and store it as "apiVersion"
    And I generate a unique API context from "/clg-inverted-allowed" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":20,"max":50,"jsonPath":"","invert":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"msg":"x"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response includes the assessment detail when showAssessment is enabled
    Given I generate a unique value from "clg-show-assessment" and store it as "apiName"
    And I generate a unique API version from "clg-show-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/clg-show-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":50,"max":100,"jsonPath":"","showAssessment":true}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"msg": "short"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"
    And the response body should contain "assessments"
    And the response body should contain "between 50 and 100 bytes"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response omits the assessment detail when showAssessment is disabled
    Given I generate a unique value from "clg-no-assessment" and store it as "apiName"
    And I generate a unique API version from "clg-no-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/clg-no-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":50,"max":100,"jsonPath":"","showAssessment":false}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"msg": "short"}
      """
    Then the response status code should be 422
    And the response body should contain "CONTENT_LENGTH_GUARDRAIL"
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
