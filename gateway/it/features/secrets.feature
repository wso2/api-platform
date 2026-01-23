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

Feature: Secrets Management
  As an API administrator
  I want to manage secrets securely in the gateway
  So that I can store and retrieve sensitive configuration data encrypted at rest

  Background:
    Given the gateway services are running

  # ========================================
  # Scenario Group 1: Secret Lifecycle (Happy Path)
  # ========================================

  Scenario: Complete secret lifecycle - create, retrieve, update, and delete
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: wso2-openai-key
        spec:
          displayName: WSO2 OpenAI Key
          description: WSO2 OpenAI provider API Key
          type: default
          value: sk_xxx
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "wso2-openai-key"
    And the JSON response field "value" should be ""

    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "wso2-openai-key"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "id" should be "wso2-openai-key"
    And the JSON response field "value" should be "sk_xxx"

    Given I authenticate using basic auth as "admin"
    When I update the secret "wso2-openai-key" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: wso2-openai-key
        spec:
          displayName: WSO2 OpenAI Key
          description: WSO2 OpenAI provider API Key
          type: default
          value: sk_yyy
        """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "id" should be "wso2-openai-key"
    And the JSON response field "value" should be ""

    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "wso2-openai-key"
    Then the response status code should be 200
    And the JSON response field "value" should be "sk_yyy"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "wso2-openai-key"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "wso2-openai-key"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

    Given I authenticate using basic auth as "admin"
    When I create a secret with value size 10000
    Then the response status code should be 201

# ========================================
  # Scenario Group 2: Listing and Filtering
  # ========================================

  Scenario: List all secrets returns metadata without sensitive values
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: list-test-secret-1
        spec:
          displayName: Test Secret 1
          description: First test secret for listing
          type: default
          value: super-secret-value-1
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: list-test-secret-2
        spec:
          displayName: Test Secret 2
          description: Second test secret for listing
          type: default
          value: super-secret-value-2
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: list-test-secret-3
        spec:
          displayName: Test Secret 3
          description: Third test secret for listing
          type: default
          value: super-secret-value-3
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list all secrets
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "list-test-secret-1"
    And the response body should contain "list-test-secret-2"
    And the response body should contain "list-test-secret-3"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "list-test-secret-1"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the secret "list-test-secret-2"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the secret "list-test-secret-3"
    Then the response status code should be 200

  Scenario: List secrets includes metadata fields
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: metadata-test-secret
        spec:
          displayName: Metadata Test Secret
          description: Secret for testing metadata fields
          type: default
          value: test-value-123
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list all secrets
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "metadata-test-secret"
    And the JSON response should have field "secrets"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "metadata-test-secret"
    Then the response status code should be 200

  Scenario: Empty secrets list returns valid response
    Given I authenticate using basic auth as "admin"
    When I list all secrets
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List secrets after creating and deleting shows correct state
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: temporary-secret
        spec:
          displayName: Temporary Secret
          description: Will be deleted
          type: default
          value: temporary-value
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list all secrets
    Then the response status code should be 200
    And the response body should contain "temporary-secret"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "temporary-secret"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I list all secrets
    Then the response status code should be 200
    And the response body should not contain "temporary-secret"

  # ========================================
  # Scenario Group 3: Validation & Error Handling
  # ========================================

  Scenario: Create secret with missing metadata name field
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          labels:
            test: "true"
        spec:
          displayName: Missing Name Secret
          description: Secret without name in metadata
          type: default
          value: some-value
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "name"

  Scenario: Create secret with missing value field
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: missing-value-secret
        spec:
          displayName: Missing Value Secret
          description: Secret without value field
          type: default
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "value"

  Scenario: Create secret with empty value field
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: empty-value-secret
        spec:
          displayName: Empty Value Secret
          description: Secret with empty value
          type: default
          value: ""
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create secret with oversized value exceeding 10KB limit
    Given I authenticate using basic auth as "admin"
    When I create a secret with oversized value
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Secret value must be less than 10KB"

  Scenario: Update secret with missing value field
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: update-validation-test-secret
        spec:
          displayName: Update Validation Test
          description: Testing update validation
          type: default
          value: original-value
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I update the secret "update-validation-test-secret" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: update-validation-test-secret
        spec:
          displayName: Update Validation Test
          description: Attempting to update without value
          type: default
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "value"

    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "update-validation-test-secret"
    Then the response status code should be 200
    And the JSON response field "value" should be "original-value"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "update-validation-test-secret"
    Then the response status code should be 200

  Scenario: Update secret with empty value field
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: empty-update-test-secret
        spec:
          displayName: Empty Update Test
          description: Testing empty value on update
          type: default
          value: original-value
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I update the secret "empty-update-test-secret" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: empty-update-test-secret
        spec:
          displayName: Empty Update Test
          description: Attempting to update with empty value
          type: default
          value: ""
        """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "empty-update-test-secret"
    Then the response status code should be 200
    And the JSON response field "value" should be "original-value"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "empty-update-test-secret"
    Then the response status code should be 200

  Scenario: Access secrets endpoints without authentication returns unauthorized
    When I clear all headers
    And I list all secrets
    Then the response status code should be 401
    And the response should be valid JSON
    And the JSON response field "error" should be "no valid authentication credentials provided"

    When I clear all headers
    And I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: unauthorized-test-secret
        spec:
          displayName: Unauthorized Test
          description: Testing without authentication
          type: default
          value: test-value
        """
    Then the response status code should be 401
    And the response should be valid JSON
    And the JSON response field "error" should be "no valid authentication credentials provided"

    When I clear all headers
    And I retrieve the secret "some-secret-id"
    Then the response status code should be 401
    And the response should be valid JSON
    And the JSON response field "error" should be "no valid authentication credentials provided"

    When I clear all headers
    And I update the secret "some-secret-id" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: some-secret-id
        spec:
          displayName: Unauthorized Update
          description: Testing update without authentication
          type: default
          value: new-value
        """
    Then the response status code should be 401
    And the response should be valid JSON
    And the JSON response field "error" should be "no valid authentication credentials provided"

    When I clear all headers
    And I delete the secret "some-secret-id"
    Then the response status code should be 401
    And the response should be valid JSON
    And the JSON response field "error" should be "no valid authentication credentials provided"

  # ========================================
  # Scenario Group 4: Conflict & Not Found
  # ========================================

  Scenario: Create duplicate secret returns conflict error
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: duplicate-test-secret
        spec:
          displayName: Duplicate Test Secret
          description: First secret creation
          type: default
          value: first-value
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "duplicate-test-secret"

    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: duplicate-test-secret
        spec:
          displayName: Duplicate Test Secret
          description: Second secret creation with same name
          type: default
          value: second-value
        """
    Then the response status code should be 409
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "already exists"

    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "duplicate-test-secret"
    Then the response status code should be 200
    And the JSON response field "value" should be "first-value"

    Given I authenticate using basic auth as "admin"
    When I delete the secret "duplicate-test-secret"
    Then the response status code should be 200

  Scenario: Retrieve non-existent secret returns not found
    Given I authenticate using basic auth as "admin"
    When I retrieve the secret "non-existent-secret-xyz-12345"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "secret configuration not found"

  Scenario: Update non-existent secret returns not found
    Given I authenticate using basic auth as "admin"
    When I update the secret "non-existent-update-secret-abc" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: non-existent-update-secret-abc
        spec:
          displayName: Non-existent Secret
          description: Attempting to update non-existent secret
          type: default
          value: new-value
        """
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "secret configuration not found"

  Scenario: Delete non-existent secret returns not found
    Given I authenticate using basic auth as "admin"
    When I delete the secret "non-existent-delete-secret-xyz"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "secret configuration not found"

  Scenario: Delete operation is idempotent - second delete returns not found
    Given I authenticate using basic auth as "admin"
    When I create this secret:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: Secret
        metadata:
          name: idempotent-delete-test
        spec:
          displayName: Idempotent Delete Test
          description: Testing delete idempotency
          type: default
          value: test-value
        """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I delete the secret "idempotent-delete-test"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the secret "idempotent-delete-test"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "secret configuration not found"

  Scenario: Update secret with an existing secret name returns conflict
    Given I authenticate using basic auth as "admin"
    When I create this secret:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Secret
      metadata:
        name: update-conflict-secret-1
      spec:
        displayName: Update Conflict Secret 1
        description: First secret for update conflict test
        type: default
        value: first-value
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this secret:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Secret
      metadata:
        name: update-conflict-secret-2
      spec:
        displayName: Update Conflict Secret 2
        description: Second secret for update conflict test
        type: default
        value: second-value
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I update the secret "update-conflict-secret-1" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Secret
      metadata:
        name: update-conflict-secret-2
      spec:
        displayName: Update Conflict Secret 1
        description: Attempting to update secret with duplicate name
        type: default
        value: updated-value
      """
    Then the response status code should be 409
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "already exists"
