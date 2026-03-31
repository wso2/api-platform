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

Feature: Secret Management Operations
  As an API administrator
  I want to manage secrets for APIs and providers
  So that I can securely store sensitive configuration data

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    
  # ==================== CREATE SECRET - SUCCESS CASES ====================

  Scenario: Create a new secret successfully
    When I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "test-secret-1"
        },
        "spec": {
          "displayName": "Test Secret 1",
          "description": "A test secret for validation",
          "value": "my-secret-value-123"
        }
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "test-secret-1"
    # Cleanup
    When I delete the secret "test-secret-1"
    Then the response status should be 200

  Scenario: Create a secret with simple name and value
    When I create a secret named "simple-secret" with value "simple-value-123"
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "simple-secret"
    # Cleanup
    When I delete the secret "simple-secret"
    Then the response status should be 200

  Scenario: Create secret with special characters in value
    When I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "special-secret"
        },
        "spec": {
          "displayName": "Special Secret",
          "description": "Secret with special characters",
          "value": "!@#$%^&*()_+-=[]{}|;':\",./<>?"
        }
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "special-secret"
    # Cleanup
    When I delete the secret "special-secret"
    Then the response status should be 200

  Scenario: Create secret with long value
    When I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "long-secret"
        },
        "spec": {
          "displayName": "Long Secret",
          "description": "Secret with a very long value",
          "value": "this-is-a-very-long-secret-value-with-many-characters-to-test-that-the-system-can-handle-secrets-of-reasonable-length"
        }
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "long-secret"
    # Cleanup
    When I delete the secret "long-secret"
    Then the response status should be 200

  # ==================== CREATE SECRET - ERROR CASES ====================

  Scenario: Create secret without name returns error
    When I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "spec": {
          "displayName": "No Name Secret",
          "description": "Secret without a name",
          "value": "my-secret-value"
        }
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create secret without value returns error
    When I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "no-value-secret"
        },
        "spec": {
          "displayName": "No Value Secret",
          "description": "Secret without a value"
        }
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create duplicate secret returns conflict error
    Given I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "duplicate-secret"
        },
        "spec": {
          "displayName": "Duplicate Secret",
          "description": "Original secret",
          "value": "original-value"
        }
      }
      """
    Then the response status should be 201
    When I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "duplicate-secret"
        },
        "spec": {
          "displayName": "Duplicate Secret",
          "description": "Duplicate secret",
          "value": "duplicate-value"
        }
      }
      """
    Then the response status should be 409
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    # Cleanup
    When I delete the secret "duplicate-secret"
    Then the response status should be 200

  # ==================== GET SECRET - SUCCESS CASES ====================

  Scenario: Get secret by name returns secret details
    Given I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "get-test-secret"
        },
        "spec": {
          "displayName": "Get Test Secret",
          "description": "Secret for get testing",
          "value": "retrievable-secret-value"
        }
      }
      """
    Then the response status should be 201
    When I get the secret "get-test-secret"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "secret"
    And the JSON response field "secret.configuration.metadata.name" should be "get-test-secret"
    # Cleanup
    When I delete the secret "get-test-secret"
    Then the response status should be 200

  Scenario: Get secret list contains created secret
    Given I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "list-test-secret"
        },
        "spec": {
          "displayName": "List Test Secret",
          "description": "Secret for list testing",
          "value": "listable-secret-value"
        }
      }
      """
    Then the response status should be 201
    When I list all secrets
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "list-test-secret"
    # Cleanup
    When I delete the secret "list-test-secret"
    Then the response status should be 200

  # ==================== GET SECRET - ERROR CASES ====================

  Scenario: Get non-existent secret returns 404
    When I get the secret "non-existent-secret-12345"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== UPDATE SECRET - SUCCESS CASES ====================

  Scenario: Update secret value successfully
    Given I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "update-test-secret"
        },
        "spec": {
          "displayName": "Update Test Secret",
          "description": "Original secret description",
          "value": "original-value"
        }
      }
      """
    Then the response status should be 201
    When I update the secret "update-test-secret" with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "update-test-secret"
        },
        "spec": {
          "displayName": "Updated Secret Name",
          "description": "Updated secret description",
          "value": "updated-value-123"
        }
      }
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "id" should be "update-test-secret"
    # Cleanup
    When I delete the secret "update-test-secret"
    Then the response status should be 200

  Scenario: Update secret with simple value
    Given I create a secret named "simple-update-secret" with value "original-simple-value"
    Then the response status should be 201
    When I update the secret "simple-update-secret" with value "updated-simple-value"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "id" should be "simple-update-secret"
    # Cleanup
    When I delete the secret "simple-update-secret"
    Then the response status should be 200

  # ==================== UPDATE SECRET - ERROR CASES ====================

  Scenario: Update non-existent secret returns 404
    When I update the secret "non-existent-secret-12345" with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "non-existent-secret-12345"
        },
        "spec": {
          "displayName": "Non-existent Secret",
          "description": "This secret does not exist",
          "value": "new-value"
        }
      }
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== DELETE SECRET - SUCCESS CASES ====================

  Scenario: Delete secret successfully
    Given I create a secret with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "Secret",
        "metadata": {
          "name": "delete-test-secret"
        },
        "spec": {
          "displayName": "Delete Test Secret",
          "description": "Secret for deletion testing",
          "value": "deletable-secret-value"
        }
      }
      """
    Then the response status should be 201
    When I delete the secret "delete-test-secret"
    Then the response status should be 200
    # Verify deletion
    When I get the secret "delete-test-secret"
    Then the response status should be 404

  Scenario: Delete secret is idempotent - deleting non-existent secret returns 404
    When I delete the secret "non-existent-secret-99999"
    Then the response status should be 404
