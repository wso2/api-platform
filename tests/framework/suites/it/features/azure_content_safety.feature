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

@azure-content-safety
Feature: Azure Content Safety content moderation policy
  As an API developer
  I want to validate request and response content using the Azure Content Safety API
  So that I can prevent harmful content from being processed
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request with safe content passes through
    Given I generate a unique value from "acs-safe-request" and store it as "apiName"
    And I generate a unique API version from "acs-safe-request" and store it as "apiVersion"
    And I generate a unique API context from "/acs-safe-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"","hateSeverityThreshold":4,"violenceSeverityThreshold":4,"sexualSeverityThreshold":4,"selfHarmSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Hello, this is safe content"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request with hate speech is blocked
    Given I generate a unique value from "acs-hate-block" and store it as "apiName"
    And I generate a unique API version from "acs-hate-block" and store it as "apiVersion"
    And I generate a unique API context from "/acs-hate-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","hateSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This content contains hate speech"}
      """
    Then the response status code should be 422
    And the response body should contain "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request with violence is blocked
    Given I generate a unique value from "acs-violence-block" and store it as "apiName"
    And I generate a unique API version from "acs-violence-block" and store it as "apiVersion"
    And I generate a unique API context from "/acs-violence-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","violenceSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This content contains violence"}
      """
    Then the response status code should be 422
    And the response body should contain "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request violation with detailed assessment
    Given I generate a unique value from "acs-assessment" and store it as "apiName"
    And I generate a unique API version from "acs-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/acs-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","showAssessment":true,"hateSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Response with violating content is blocked
    Given I generate a unique value from "acs-response-block" and store it as "apiName"
    And I generate a unique API version from "acs-response-block" and store it as "apiVersion"
    And I generate a unique API context from "/acs-response-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"response":{"jsonPath":"$.json.message","violenceSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Hello, this is safe content"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This response contains violence"}
      """
    Then the response status code should be 422
    And the response body should contain "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A disabled category allows content that would otherwise violate it
    Given I generate a unique value from "acs-disabled-category" and store it as "apiName"
    And I generate a unique API version from "acs-disabled-category" and store it as "apiVersion"
    And I generate a unique API context from "/acs-disabled-category" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","hateSeverityThreshold":-1,"violenceSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Hate content passes through because hateSeverityThreshold is disabled (-1)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"This contains hate speech but should pass"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple categories enabled with different thresholds each enforce independently
    Given I generate a unique value from "acs-multi-category" and store it as "apiName"
    And I generate a unique API version from "acs-multi-category" and store it as "apiVersion"
    And I generate a unique API context from "/acs-multi-category" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","hateSeverityThreshold":4,"violenceSeverityThreshold":4,"sexualSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"safe warm-up"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains hate"}
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains violence"}
      """
    Then the response status code should be 422

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains sexual content"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction validates only the targeted field
    Given I generate a unique value from "acs-jsonpath" and store it as "apiName"
    And I generate a unique API version from "acs-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/acs-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","violenceSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Safe message field, violating metadata field ignored since only $.message is checked
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Safe content","metadata":"This contains violence but should be ignored"}
      """

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This message contains violence","metadata":"Safe metadata"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A nested JSONPath extraction is validated
    Given I generate a unique value from "acs-nested-jsonpath" and store it as "apiName"
    And I generate a unique API version from "acs-nested-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/acs-nested-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.data.content","hateSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Safe nested content field, violating outer field ignored
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"data":{"content":"Safe nested message","timestamp":"2025-01-01"},"metadata":"This outer field contains hate but should be ignored"}
      """

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"data":{"content":"This nested content contains hate","timestamp":"2025-01-01"}}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request and response phases each validate their own content independently
    Given I generate a unique value from "acs-both-phases" and store it as "apiName"
    And I generate a unique API version from "acs-both-phases" and store it as "apiVersion"
    And I generate a unique API context from "/acs-both-phases" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","violenceSeverityThreshold":4},"response":{"jsonPath":"","hateSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"Safe request content"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An empty request body is handled gracefully
    Given I generate a unique value from "acs-empty-body" and store it as "apiName"
    And I generate a unique API version from "acs-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/acs-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"","violenceSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: passthroughOnError allows the request through despite a content safety API failure
    Given I generate a unique value from "acs-passthrough" and store it as "apiName"
    And I generate a unique API version from "acs-passthrough" and store it as "apiVersion"
    And I generate a unique API context from "/acs-passthrough" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","passthroughOnError":true,"violenceSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # The mock simulates a content-safety-service 500 for this exact keyword combination
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" until status 200 with body:
      """
      {"message":"This will simulate error in mock service"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A blocked response carries the complete error structure
    Given I generate a unique value from "acs-error-structure" and store it as "apiName"
    And I generate a unique API version from "acs-error-structure" and store it as "apiVersion"
    And I generate a unique API context from "/acs-error-structure" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/validate","policies":[{"name":"azure-content-safety-content-moderation","version":"v1","params":{"request":{"jsonPath":"$.message","showAssessment":true,"hateSeverityThreshold":4}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {"message":"This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
