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
Feature: Content Length Guardrail Policy
  As an API developer
  I want to validate the byte length of request payloads
  So that I can enforce content size constraints and protect my backend services

  Background:
    Given the gateway services are running

  # Whole Payload Validation Scenarios

  Scenario: Request with valid content length (whole payload)
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-valid-content-api
      spec:
        displayName: Content Length Guardrail - Valid Content
        version: v1.0
        context: /clg-valid/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 10
                    max: 100
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-valid/v1.0/health" until status 200

    When I send a "POST" request to "/clg-valid/v1.0/validate" with body:
      """
      {"message": "This is a valid message with 50 bytes"}
      """
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-valid-content-api"
    Then the response status code should be 200

  Scenario: Request with content below minimum length (whole payload)
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-below-min-api
      spec:
        displayName: Content Length Guardrail - Below Minimum
        version: v1.0
        context: /clg-below-min/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 50
                    max: 200
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-below-min/v1.0/health" until status 200

    When I send a "POST" request to "/clg-below-min/v1.0/validate" with body:
      """
      {"msg": "short"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-below-min-api"
    Then the response status code should be 200

  Scenario: Request with content above maximum length (whole payload)
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-above-max-api
      spec:
        displayName: Content Length Guardrail - Above Maximum
        version: v1.0
        context: /clg-above-max/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 10
                    max: 50
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-above-max/v1.0/health" until status 200

    When I send a "POST" request to "/clg-above-max/v1.0/validate" with body:
      """
      {"message": "This is a very long message that exceeds the maximum allowed length of 50 bytes"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-above-max-api"
    Then the response status code should be 200

  Scenario: Empty request body handling
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-empty-body-api
      spec:
        displayName: Content Length Guardrail - Empty Body
        version: v1.0
        context: /clg-empty/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 1
                    max: 100
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-empty/v1.0/health" until status 200

    When I send a "POST" request to "/clg-empty/v1.0/validate" with body:
      """
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-empty-body-api"
    Then the response status code should be 200

  Scenario: Request at exact minimum boundary
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-min-boundary-api
      spec:
        displayName: Content Length Guardrail - Min Boundary
        version: v1.0
        context: /clg-min-boundary/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 20
                    max: 100
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-min-boundary/v1.0/health" until status 200

    When I send a "POST" request to "/clg-min-boundary/v1.0/validate" with body:
      """
      {"msg":"exactly20byte"}
      """
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-min-boundary-api"
    Then the response status code should be 200

  Scenario: Request at exact maximum boundary
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-max-boundary-api
      spec:
        displayName: Content Length Guardrail - Max Boundary
        version: v1.0
        context: /clg-max-boundary/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 10
                    max: 50
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-max-boundary/v1.0/health" until status 200

    When I send a "POST" request to "/clg-max-boundary/v1.0/validate" with body:
      """
      {"message":"This message is exactly 50 bytes."}
      """
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-max-boundary-api"
    Then the response status code should be 200

  # JSONPath Extraction Scenarios

  Scenario: Request with valid content length using JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-jsonpath-valid-api
      spec:
        displayName: Content Length Guardrail - JSONPath Valid
        version: v1.0
        context: /clg-jsonpath-valid/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 5
                    max: 50
                    jsonPath: "$.message"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-jsonpath-valid/v1.0/health" until status 200

    When I send a "POST" request to "/clg-jsonpath-valid/v1.0/validate" with body:
      """
      {"message": "Hello World"}
      """
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-jsonpath-valid-api"
    Then the response status code should be 200

  Scenario: Request with invalid content length using JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-jsonpath-invalid-api
      spec:
        displayName: Content Length Guardrail - JSONPath Invalid
        version: v1.0
        context: /clg-jsonpath-invalid/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 5
                    max: 10
                    jsonPath: "$.message"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-jsonpath-invalid/v1.0/health" until status 200

    When I send a "POST" request to "/clg-jsonpath-invalid/v1.0/validate" with body:
      """
      {"message": "This is a very long message"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-jsonpath-invalid-api"
    Then the response status code should be 200

  Scenario: Request with nested JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-nested-jsonpath-api
      spec:
        displayName: Content Length Guardrail - Nested JSONPath
        version: v1.0
        context: /clg-nested-jsonpath/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 10
                    max: 100
                    jsonPath: "$.data.description"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-nested-jsonpath/v1.0/health" until status 200

    When I send a "POST" request to "/clg-nested-jsonpath/v1.0/validate" with body:
      """
      {"data": {"description": "Valid content here"}}
      """
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-nested-jsonpath-api"
    Then the response status code should be 200

  Scenario: JSONPath extraction with missing field
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-missing-field-api
      spec:
        displayName: Content Length Guardrail - Missing Field
        version: v1.0
        context: /clg-missing-field/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 5
                    max: 50
                    jsonPath: "$.nonexistent"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-missing-field/v1.0/health" until status 200

    When I send a "POST" request to "/clg-missing-field/v1.0/validate" with body:
      """
      {"message": "Hello"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-missing-field-api"
    Then the response status code should be 200

  # Inverted Logic Scenarios

  Scenario: Request with inverted logic - content in excluded range
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-inverted-excluded-api
      spec:
        displayName: Content Length Guardrail - Inverted Excluded
        version: v1.0
        context: /clg-inverted-excluded/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 20
                    max: 50
                    jsonPath: ""
                    invert: true
                    showAssessment: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-inverted-excluded/v1.0/health" until status 200

    When I send a "POST" request to "/clg-inverted-excluded/v1.0/validate" with body:
      """
      {"message": "30 bytes content"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"
    And the JSON response field "message.assessments" should be "Violation of content length detected. Expected content length to be outside the range of 20 to 50 bytes."

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-inverted-excluded-api"
    Then the response status code should be 200

  Scenario: Request with inverted logic - content outside excluded range
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-inverted-allowed-api
      spec:
        displayName: Content Length Guardrail - Inverted Allowed
        version: v1.0
        context: /clg-inverted-allowed/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 20
                    max: 50
                    jsonPath: ""
                    invert: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-inverted-allowed/v1.0/health" until status 200

    When I send a "POST" request to "/clg-inverted-allowed/v1.0/validate" with body:
      """
      {"msg":"x"}
      """
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-inverted-allowed-api"
    Then the response status code should be 200

  # showAssessment Scenarios

  Scenario: Error response with showAssessment enabled
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-show-assessment-api
      spec:
        displayName: Content Length Guardrail - Show Assessment
        version: v1.0
        context: /clg-show-assessment/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 50
                    max: 100
                    jsonPath: ""
                    showAssessment: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-show-assessment/v1.0/health" until status 200

    When I send a "POST" request to "/clg-show-assessment/v1.0/validate" with body:
      """
      {"msg": "short"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"
    And the JSON response should have field "message.assessments"
    And the JSON response field "message.assessments" should be "Violation of content length detected. Expected content length to be between 50 and 100 bytes."

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-show-assessment-api"
    Then the response status code should be 200

  Scenario: Error response with showAssessment disabled
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: clg-no-assessment-api
      spec:
        displayName: Content Length Guardrail - No Assessment
        version: v1.0
        context: /clg-no-assessment/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 50
                    max: 100
                    jsonPath: ""
                    showAssessment: false
      """
    Then the response status code should be 201
    And I send a "GET" request to "/clg-no-assessment/v1.0/health" until status 200

    When I send a "POST" request to "/clg-no-assessment/v1.0/validate" with body:
      """
      {"msg": "short"}
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "CONTENT_LENGTH_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"

    Given I authenticate using basic auth as "admin"
    When I delete the API "clg-no-assessment-api"
    Then the response status code should be 200
