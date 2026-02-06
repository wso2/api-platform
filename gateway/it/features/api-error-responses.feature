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

Feature: API Error Responses
  As an API administrator
  I want clear validation and parse error responses
  So that I can correct invalid API payloads

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Create API returns validation errors with field details
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: create-error-response-api
      spec:
        displayName: Create-Error-Response-Api
        version: v1.0
        upstream:
          main:
            url: http://sample-backend:9080
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.context"
    And the response body should contain "Context is required"
    And the response body should contain "spec.operations"
    And the response body should contain "At least one operation is required"

  Scenario: Create API returns policy schema validation errors
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: policy-schema-error-api
      spec:
        displayName: Policy-Schema-Error-Api
        version: v1.0
        context: /policy-schema-error
        upstream:
          main:
            url: http://sample-backend:9080
        policies:
          - name: respond
            version: v0
            params:
              statusCode: "200"
        operations:
          - method: GET
            path: /test
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.policies[0].params.statusCode"
    And the response body should contain "Invalid type"

  Scenario: Create API returns policy not found errors
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: policy-not-found-error-api
      spec:
        displayName: Policy-Not-Found-Error-Api
        version: v1.0
        context: /policy-not-found-error
        upstream:
          main:
            url: http://sample-backend:9080
        policies:
          - name: policy-does-not-exist
            version: v0
        operations:
          - method: GET
            path: /test
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.policies[0].version"
    And the response body should contain "policy-does-not-exist"
    And the response body should contain "not found in loaded policy definitions"

  Scenario: Create API returns metadata name validation errors
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata: {}
      spec:
        displayName: Missing-Metadata-Name-Api
        version: v1.0
        context: /missing-metadata-name
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "metadata.name"
    And the response body should contain "Metadata name is required"

  Scenario: Create API returns version format validation errors
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: invalid-version-format-api
      spec:
        displayName: Invalid-Version-Format-Api
        version: v1.0.0-beta
        context: /invalid-version-format
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.version"
    And the response body should contain "semantic versioning pattern"

  Scenario: Update API parse errors include detailed message
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-parse-error-api
      spec:
        displayName: Update-Parse-Error-Api
        version: v1.0
        context: /update-parse-error
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    When I send a PUT request to the "gateway-controller" service at "/apis/update-parse-error-api" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "Failed to parse configuration:"
    And the JSON response field "message" should contain "failed to parse JSON"
    # Cleanup
    When I delete the API "update-parse-error-api"
    Then the response should be successful

  Scenario: Update API returns validation errors with field details
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-validation-error-api
      spec:
        displayName: Update-Validation-Error-Api
        version: v1.0
        context: /update-validation-error
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    When I update the API "update-validation-error-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: update-validation-error-api
      spec:
        displayName: Update-Validation-Error-Api
        version: v1.0
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.context"
    And the response body should contain "Context is required"
    # Cleanup
    When I delete the API "update-validation-error-api"
    Then the response should be successful
