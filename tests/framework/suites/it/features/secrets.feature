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

@secrets
Feature: Secret management operations
  As an API administrator
  I want to create, read, update, and delete secrets
  So that I can securely store sensitive configuration data referenced by other resources

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Create a new secret successfully
    Given I generate a unique value from "secret-basic" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Test Secret",
          "description": "A test secret for validation",
          "value": "my-secret-value-123"
        }
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:secretName}"
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Create a secret with a plain value
    Given I generate a unique value from "secret-simple" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Simple Secret",
          "description": "Auto-generated secret",
          "value": "simple-value-123"
        }
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:secretName}"
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Create a secret whose value contains special characters
    Given I generate a unique value from "secret-special" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
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
    And the JSON response field "status.id" should be "${CTX:secretName}"
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Create a secret with a long value
    Given I generate a unique value from "secret-long" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
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
    And the JSON response field "status.id" should be "${CTX:secretName}"
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Create secret without a name returns an error
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
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

  Scenario: Create secret without a value returns an error
    Given I generate a unique value from "secret-no-value" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
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

  Scenario: Create duplicate secret returns a conflict error
    Given I generate a unique value from "secret-duplicate" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Duplicate Secret",
          "description": "Original secret",
          "value": "original-value"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
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

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Get secret by name returns its details
    Given I generate a unique value from "secret-get" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Get Test Secret",
          "description": "Secret for get testing",
          "value": "retrievable-secret-value"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "GET" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "kind" should be "Secret"
    And the JSON response field "metadata.name" should be "${CTX:secretName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Listing secrets includes a created secret
    Given I generate a unique value from "secret-list" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "List Test Secret",
          "description": "Secret for list testing",
          "value": "listable-secret-value"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "GET" request to the "gateway-controller" service at "/secrets"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:secretName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Getting a non-existent secret returns 404
    When I send a "GET" request to the "gateway-controller" service at "/secrets/non-existent-secret-12345"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Update secret value successfully
    Given I generate a unique value from "secret-update" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Update Test Secret",
          "description": "Original secret description",
          "value": "original-value"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "PUT" request to the "gateway-controller" service at "/secrets/${CTX:secretName}" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
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
    And the JSON response field "status.id" should be "${CTX:secretName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Update secret with a plain replacement value
    Given I generate a unique value from "secret-update-simple" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Simple Update Secret",
          "description": "Auto-generated secret",
          "value": "original-simple-value"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "PUT" request to the "gateway-controller" service at "/secrets/${CTX:secretName}" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Simple Update Secret",
          "description": "Auto-generated secret",
          "value": "updated-simple-value"
        }
      }
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:secretName}"

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

  Scenario: Updating a non-existent secret returns 404
    When I send a "PUT" request to the "gateway-controller" service at "/secrets/non-existent-secret-12345" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
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

  Scenario: Delete secret successfully
    Given I generate a unique value from "secret-delete" and store it as "secretName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "Secret",
        "metadata": {
          "name": "${CTX:secretName}"
        },
        "spec": {
          "displayName": "Delete Test Secret",
          "description": "Secret for deletion testing",
          "value": "deletable-secret-value"
        }
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 200

    When I send a "GET" request to the "gateway-controller" service at "/secrets/${CTX:secretName}"
    Then the response status should be 404

  Scenario: Deleting a non-existent secret is idempotent
    When I send a "DELETE" request to the "gateway-controller" service at "/secrets/non-existent-secret-99999"
    Then the response status should be 404
