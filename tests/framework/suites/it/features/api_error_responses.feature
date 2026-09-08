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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@api-error-responses
Feature: API error responses
  As an API administrator
  I want invalid API requests to return useful errors
  So that I can correct the configuration

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Creating an API with missing required fields returns field errors
    Given I generate a unique value from "error-missing-fields-api" and store it as "apiName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Missing-Fields              |
      | spec.version           | v1.0                               |
      | spec.upstream.main.url | http://testbench:3000             |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.context"
    And the response body should contain "Context is required"
    And the response body should contain "spec.operations"
    And the response body should contain "At least one operation is required"

  Scenario: Creating an API with an invalid policy parameter returns a validation error
    Given I generate a unique value from "error-invalid-policy-api" and store it as "apiName"
    And I generate a unique API context from "error-invalid-policy" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Invalid-Policy              |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.policies          | [{"name":"respond","version":"v1","params":{"statusCode":"not-a-number"}}] |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.policies[0].params.statusCode"
    And the response body should contain "Invalid type"

  Scenario: Creating an API with an unknown policy returns a not-found validation error
    Given I generate a unique value from "error-unknown-policy-api" and store it as "apiName"
    And I generate a unique API context from "error-unknown-policy" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Unknown-Policy              |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.policies          | [{"name":"policy-does-not-exist","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.policies[0].version"
    And the response body should contain "policy-does-not-exist"
    And the response body should contain "not found in loaded policy definitions"

  Scenario: Creating an API without a metadata name returns a validation error
    Given I generate a unique API context from "error-missing-name" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   |                                  |
      | spec.displayName       | Error-Missing-Name                |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "metadata.name"
    And the response body should contain "Metadata name is required"

  Scenario: Creating an API with an invalid version returns a validation error
    Given I generate a unique value from "error-invalid-version-api" and store it as "apiName"
    And I generate a unique API context from "error-invalid-version" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Invalid-Version             |
      | spec.version           | v1.0.0-beta                       |
      | spec.context           | ${CTX:apiContext}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.version"
    And the response body should contain "semantic versioning pattern"

  Scenario: Updating an API with malformed JSON returns a detailed parse error
    Given I generate a unique value from "error-update-parse-api" and store it as "apiName"
    And I generate a unique API context from "error-update-parse" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Valid-API                   |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I set header "Content-Type" to "application/json"
    And I send a "PUT" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "Failed to parse configuration:"
    And the JSON response field "message" should contain "failed to parse JSON"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Updating an API with missing context returns validation errors
    Given I generate a unique value from "error-update-validation-api" and store it as "apiName"
    And I generate a unique API context from "error-update-validation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Valid-API                   |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | Error-Invalid-Update              |
      | spec.version           | v1.0                               |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.context"
    And the response body should contain "Context is required"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
