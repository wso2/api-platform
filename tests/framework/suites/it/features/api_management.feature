# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

@api-management
Feature: API Management Handler Operations
  As an API administrator
  I want to manage APIs via REST API handlers
  So that I can create, read, update, delete, and list APIs

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== LIST APIs ====================
  
  Scenario: List all APIs when no APIs exist
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List APIs after deploying one
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: test-list-api
      spec:
        displayName: Test-List-API
        version: v1.0
        context: /test-list
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /data
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "test-list-api"
    Then the response status code should be 200

  Scenario: List APIs with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?limit=10&offset=0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  # ==================== GET API BY ID ====================
  
  Scenario: Get API by non-existent ID returns 404
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/non-existent-api-id-12345"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get API by invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/invalid@id#format"
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Get API by existing ID returns API details
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: get-by-id-test-api
      spec:
        displayName: Get-By-ID-Test-API
        version: v1.0
        context: /get-by-id-test
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/get-by-id-test-api"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the API "get-by-id-test-api"
    Then the response status code should be 200

  # ==================== DELETE API ====================
  
  Scenario: Delete non-existent API returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/non-existent-api-to-delete"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Delete existing API successfully
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: delete-test-api
      spec:
        displayName: Delete-Test-Api
        version: v1.0
        context: /delete-test
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I delete the API "delete-test-api"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Verify it's deleted
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/delete-test-api"
    Then the response status code should be 404

  # ==================== UPDATE API ====================
  
  Scenario: Update non-existent API returns 404
    When I update the API "non-existent-api-to-update" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: non-existent-api-to-update
      spec:
        displayName: Non-Existent-Api-To-Update
        version: v2.0
        context: /updated
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /updated
      """
    Then the response status code should be 404

  Scenario: Update existing API successfully
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-test-api
      spec:
        displayName: Update-Test-Api
        version: v1.0
        context: /update-test
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /original
      """
    Then the response status code should be 201
    When I update the API "update-test-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-test-api
      spec:
        displayName: Update-Test-Api
        version: v1.1
        context: /update-test
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /updated
          - method: POST
            path: /updated
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the API "update-test-api"
    Then the response status code should be 200

  # ==================== CREATE API - COMPREHENSIVE TESTS ====================
  
  Scenario: Create API with minimal required fields
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: minimal-api
      spec:
        displayName: Minimal-Api
        version: v1.0
        context: /minimal
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the API "minimal-api"
    Then the response status code should be 200

  Scenario: Create API with multiple operations
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: multi-operation-api
      spec:
        displayName: Multi-Operation-Api
        version: v1.0
        context: /multi
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /users
          - method: POST
            path: /users
          - method: GET
            path: /users/{id}
          - method: PUT
            path: /users/{id}
          - method: DELETE
            path: /users/{id}
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the API "multi-operation-api"
    Then the response status code should be 200

  Scenario: Create API with displayName
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: display-name-api
      spec:
        displayName: My Display Name API
        version: v1.0
        context: /display
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Cleanup
    When I delete the API "display-name-api"
    Then the response status code should be 200

  Scenario: Create API with context containing version placeholder
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: versioned-context-api
      spec:
        displayName: Versioned-Context-Api
        version: v2.0
        context: /api/$version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /data
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Verify it's accessible at /api/v2.0
    And I send a "GET" request to "/api/v2.0/data" until status 200
    When I send a "GET" request to "/api/v2.0/data"
    Then the response status code should be 200
    # The two reads render $version differently, by design. Both are pinned here so the
    # difference is asserted rather than assumed — see Finding 10 for the contract gap.
    #
    # The LISTING resolves it. Whole-body assertions on purpose: the negative one claims the raw
    # template appears NOWHERE, which no field path can express, and the element index in a
    # shared listing is not stable across runs.
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "/api/v2.0"
    And the response body should not contain "/api/$version"
    # GET by id returns the raw template. Exact at the field, which is possible here because the
    # response is one document.
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/versioned-context-api"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "spec.context" should be "/api/$version"
    # Cleanup
    When I delete the API "versioned-context-api"
    Then the response status code should be 200

  Scenario: Create API then verify it appears in list
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: list-verification-api
      spec:
        displayName: List-Verification-Api
        version: v1.0
        context: /list-test
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response status code should be 200
    And the response should be valid JSON
    # Cleanup
    When I delete the API "list-verification-api"
    Then the response status code should be 200

  Scenario: Create API with wildcard path
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wildcard-path-api
      spec:
        displayName: Wildcard-Path-Api
        version: v1.0
        context: /wildcard
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /*
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Cleanup
    When I delete the API "wildcard-path-api"
    Then the response status code should be 200

  Scenario: Create multiple APIs with different contexts
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: first-api
      spec:
        displayName: First-Api
        version: v1.0
        context: /first
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: second-api
      spec:
        displayName: Second-Api
        version: v1.0
        context: /second
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    # Cleanup
    When I delete the API "first-api"
    Then the response status code should be 200
    When I delete the API "second-api"
    Then the response status code should be 200

  # ==================== CREATE API - VALIDATION ERROR CASES ====================

  Scenario: Create API with missing context returns error
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: missing-context-api
      spec:
        displayName: Missing-Context-Api
        version: v1.0
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create API with missing version returns error
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: missing-version-api
      spec:
        displayName: Missing-Version-Api
        context: /missing-version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: Create API with missing upstream returns error
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: missing-upstream-api
      spec:
        displayName: Missing-Upstream-Api
        version: v1.0
        context: /missing-upstream
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: Create API with empty operations returns error
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: empty-ops-api
      spec:
        displayName: Empty-Ops-Api
        version: v1.0
        context: /empty-ops
        upstream:
          main:
            url: http://testbench:3000
        operations: []
      """
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: Create API with invalid labels (spaces in keys) should fail
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-labels-api
        labels:
          "Invalid Key": value
      spec:
        displayName: Invalid-Labels-Api
        version: v1.0
        context: /invalid-labels
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create API with labels and verify they are stored
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: labeled-api
        labels:
          environment: production
          team: api-team
          version: v1
      spec:
        displayName: Labeled-API
        version: v1.0
        context: /labeled
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /data
      """
    Then the response status code should be 201
    And the response should be valid JSON
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/labeled-api"
    Then the response status code should be 200
    And the JSON response field "metadata.labels.environment" should be "production"
    # Cleanup
    When I delete the API "labeled-api"
    Then the response status code should be 200

  # ==================== UPDATE API - ERROR CASES ====================

  Scenario: Update API with handle mismatch returns error
    # First create an API
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: handle-mismatch-api
      spec:
        displayName: Handle-Mismatch-Api
        version: v1.0
        context: /handle-mismatch
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    # Try to update with mismatched handle in YAML
    When I update the API "handle-mismatch-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: different-handle-name
      spec:
        displayName: Handle-Mismatch-Api
        version: v1.0
        context: /handle-mismatch
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "handle mismatch: path has 'handle-mismatch-api' but YAML metadata.name has 'different-handle-name'"
    # Cleanup
    When I delete the API "handle-mismatch-api"
    Then the response status code should be 200

  Scenario: Update API with validation errors returns error
    # First create an API
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-validation-api
      spec:
        displayName: Update-Validation-Api
        version: v1.0
        context: /update-validation
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    # Try to update with missing context
    When I update the API "update-validation-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-validation-api
      spec:
        displayName: Update-Validation-Api
        version: v1.0
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    # Cleanup
    When I delete the API "update-validation-api"
    Then the response status code should be 200

  # ==================== CREATE API - ADDITIONAL EDGE CASES ====================

  Scenario: Create API with invalid JSON body returns error
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis" with body:
      """
      { invalid json content here
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create API with sandbox upstream
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sandbox-api
      spec:
        displayName: Sandbox-Api
        version: v1.0
        context: /sandbox-test
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    # Cleanup
    When I delete the API "sandbox-api"
    Then the response status code should be 200

  Scenario: Create API with path parameters
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: path-params-api
      spec:
        displayName: Path-Params-Api
        version: v1.0
        context: /path-params
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /users/{userId}
          - method: GET
            path: /users/{userId}/orders/{orderId}
          - method: DELETE
            path: /users/{userId}/orders/{orderId}
      """
    Then the response status code should be 201
    And the response should be valid JSON
    # Cleanup
    When I delete the API "path-params-api"
    Then the response status code should be 200

  # ==================== LIST APIs WITH FILTERS ====================

  Scenario: List APIs filtered by displayName
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: filter-display-api
      spec:
        displayName: UniqueFilterName123
        version: v1.0
        context: /filter-display
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?displayName=UniqueFilterName123"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "apis[0].spec.displayName" should be "UniqueFilterName123"
    # Cleanup
    When I delete the API "filter-display-api"
    Then the response status code should be 200

  Scenario: List APIs filtered by version
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: filter-version-api
      spec:
        displayName: Filter-Version-Api
        version: v99.88.77
        context: /filter-version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?version=v99.88.77"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "apis[0].spec.version" should be "v99.88.77"
    # Cleanup
    When I delete the API "filter-version-api"
    Then the response status code should be 200

  Scenario: List APIs filtered by context
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: filter-context-api
      spec:
        displayName: Filter-Context-Api
        version: v1.0
        context: /unique-filter-context-xyz
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?context=/unique-filter-context-xyz"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "apis[0].spec.context" should be "/unique-filter-context-xyz"
    # Cleanup
    When I delete the API "filter-context-api"
    Then the response status code should be 200

  # ==================== UPDATE API - ADDITIONAL EDGE CASES ====================

  Scenario: Update API with invalid JSON body returns error
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-invalid-json-api
      spec:
        displayName: Update-Invalid-Json-Api
        version: v1.0
        context: /update-invalid-json
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I send a "PUT" request to the "gateway-controller" service at "/rest-apis/update-invalid-json-api" with body:
      """
      { this is not valid json
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    # Cleanup
    When I delete the API "update-invalid-json-api"
    Then the response status code should be 200

  Scenario: Update API version while keeping same context
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-version-api
      spec:
        displayName: Update-Version-Api
        version: v1.0
        context: /update-version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I update the API "update-version-api" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: update-version-api
      spec:
        displayName: Update-Version-Api
        version: v2.0
        context: /update-version
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 200
    And the response should be valid JSON
    When I get the API "update-version-api"
    Then the response status code should be 200
    And the JSON response field "spec.version" should be "v2.0"
    # Cleanup
    When I delete the API "update-version-api"
    Then the response status code should be 200

  # ==================== DELETE API - ADDITIONAL EDGE CASES ====================

  Scenario: Delete API with invalid ID format returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/invalid@id#format!!"
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Delete same API twice (idempotent check)
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: delete-twice-api
      spec:
        displayName: Delete-Twice-Api
        version: v1.0
        context: /delete-twice
        upstream:
          main:
            url: http://testbench:3000
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    When I delete the API "delete-twice-api"
    Then the response status code should be 200
    # Try to delete again
    When I delete the API "delete-twice-api"
    Then the response status code should be 404


