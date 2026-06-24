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

Feature: WebSub API Webhook Secret Management
  As an API developer
  I want to manage HMAC secrets for WebSub APIs via the gateway controller
  So that incoming webhook events can be authenticated by the websub-hmac-auth policy

  Background:
    Given the event gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== CREATE ====================

  Scenario: Create a webhook secret for a WebSub API
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-create-v1-0" },
        "spec": {
          "displayName": "secret-create",
          "version": "v1.0",
          "context": "/secret-create",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "github-prod" for WebSub API "secret-create-v1-0"
    Then the response status code should be 201
    And the response should be valid JSON
    And the response body should contain "webhookSecret"
    And the response body should contain "github-prod"
    And the webhook secret value should start with "whsec_"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-create-v1-0"
    Then the response should be successful

  # ==================== LIST ====================

  Scenario: List webhook secrets for a WebSub API
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-list-v1-0" },
        "spec": {
          "displayName": "secret-list",
          "version": "v1.0",
          "context": "/secret-list",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "github-list" for WebSub API "secret-list-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list webhook secrets for WebSub API "secret-list-v1-0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "github-list"
    And the response body should contain "totalCount"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-list-v1-0"
    Then the response should be successful

  Scenario: Listing webhook secrets does not expose the secret value in the response
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-list-masked-v1-0" },
        "spec": {
          "displayName": "secret-list-masked",
          "version": "v1.0",
          "context": "/secret-list-masked",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "masked-secret" for WebSub API "secret-list-masked-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list webhook secrets for WebSub API "secret-list-masked-v1-0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "masked-secret"
    And the response body should not contain "whsec_"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-list-masked-v1-0"
    Then the response should be successful

  # ==================== REGENERATE ====================

  Scenario: Regenerate a webhook secret returns a new secret value
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-regen-v1-0" },
        "spec": {
          "displayName": "secret-regen",
          "version": "v1.0",
          "context": "/secret-regen",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "regen-secret" for WebSub API "secret-regen-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I regenerate the saved webhook secret for WebSub API "secret-regen-v1-0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should contain "regenerated successfully"
    And the webhook secret value should start with "whsec_"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-regen-v1-0"
    Then the response should be successful

  # ==================== DELETE ====================

  Scenario: Delete a webhook secret removes it from the list
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-del-v1-0" },
        "spec": {
          "displayName": "secret-del",
          "version": "v1.0",
          "context": "/secret-del",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "to-be-deleted" for WebSub API "secret-del-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I delete the saved webhook secret from WebSub API "secret-del-v1-0"
    Then the response status code should be 204

    Given I authenticate using basic auth as "admin"
    When I list webhook secrets for WebSub API "secret-del-v1-0"
    Then the response status code should be 200
    And the response body should not contain "to-be-deleted"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-del-v1-0"
    Then the response should be successful

  # ==================== ERROR CASES ====================

  Scenario: Creating a webhook secret for a non-existent API returns 404
    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "any-secret" for WebSub API "does-not-exist"
    Then the response status code should be 404

  Scenario: Creating a duplicate webhook secret name returns 409
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-dup-v1-0" },
        "spec": {
          "displayName": "secret-dup",
          "version": "v1.0",
          "context": "/secret-dup",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "dup-secret" for WebSub API "secret-dup-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "dup-secret" for WebSub API "secret-dup-v1-0"
    Then the response status code should be 409

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-dup-v1-0"
    Then the response should be successful

  Scenario: Regenerating a non-existent webhook secret returns 404
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-regen-404-v1-0" },
        "spec": {
          "displayName": "secret-regen-404",
          "version": "v1.0",
          "context": "/secret-regen-404",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I regenerate webhook secret "no-such-secret" for WebSub API "secret-regen-404-v1-0"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-regen-404-v1-0"
    Then the response should be successful

  Scenario: Deleting a non-existent webhook secret returns 404
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-del-404-v1-0" },
        "spec": {
          "displayName": "secret-del-404",
          "version": "v1.0",
          "context": "/secret-del-404",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete webhook secret "no-such-secret" from WebSub API "secret-del-404-v1-0"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-del-404-v1-0"
    Then the response should be successful

  # ==================== CROSS-API ISOLATION ====================

  Scenario: Listing secrets of another API does not expose secrets belonging to a different API
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-iso-a-v1-0" },
        "spec": {
          "displayName": "secret-iso-a",
          "version": "v1.0",
          "context": "/secret-iso-a",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-iso-b-v1-0" },
        "spec": {
          "displayName": "secret-iso-b",
          "version": "v1.0",
          "context": "/secret-iso-b",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "iso-secret-a" for WebSub API "secret-iso-a-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I list webhook secrets for WebSub API "secret-iso-b-v1-0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should not contain "iso-secret-a"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-iso-a-v1-0"
    Then the response should be successful
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-iso-b-v1-0"
    Then the response should be successful

  Scenario: Deleting a webhook secret using a different API context returns 404
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-del-iso-a-v1-0" },
        "spec": {
          "displayName": "secret-del-iso-a",
          "version": "v1.0",
          "context": "/secret-del-iso-a",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-del-iso-b-v1-0" },
        "spec": {
          "displayName": "secret-del-iso-b",
          "version": "v1.0",
          "context": "/secret-del-iso-b",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "del-iso-secret" for WebSub API "secret-del-iso-a-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I delete webhook secret "del-iso-secret" from WebSub API "secret-del-iso-b-v1-0"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-del-iso-a-v1-0"
    Then the response should be successful
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-del-iso-b-v1-0"
    Then the response should be successful

  Scenario: Regenerating a webhook secret using a different API context returns 404
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-regen-iso-a-v1-0" },
        "spec": {
          "displayName": "secret-regen-iso-a",
          "version": "v1.0",
          "context": "/secret-regen-iso-a",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "secret-regen-iso-b-v1-0" },
        "spec": {
          "displayName": "secret-regen-iso-b",
          "version": "v1.0",
          "context": "/secret-regen-iso-b",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "regen-iso-secret" for WebSub API "secret-regen-iso-a-v1-0"
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I regenerate webhook secret "regen-iso-secret" for WebSub API "secret-regen-iso-b-v1-0"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-regen-iso-a-v1-0"
    Then the response should be successful
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secret-regen-iso-b-v1-0"
    Then the response should be successful

  # ==================== HMAC POLICY E2E ====================

  Scenario: Publishing with a valid HMAC signature passes the websub-hmac-auth policy
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "hmac-valid-v1-0" },
        "spec": {
          "displayName": "hmac-valid",
          "version": "v1.0",
          "context": "/hmac-valid",
          "allChannels": {
            "on_message_received": {
              "policies": [
                {
                  "name": "websub-hmac-auth",
                  "version": "v1"
                }
              ]
            }
          },
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "hmac-signing-key" for WebSub API "hmac-valid-v1-0"
    Then the response status code should be 201
    And I wait for 3 seconds

    When I subscribe to topic "events" on API "hmac-valid" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202
    And I wait for 2 seconds

    When I publish event "hmac-payload-001" to topic "events" on API "hmac-valid" version "v1.0" with the saved HMAC signature
    Then the response status code should be 202

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "hmac-valid-v1-0"
    Then the response should be successful

  Scenario: Publishing without an HMAC signature is rejected by the websub-hmac-auth policy
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "hmac-reject-v1-0" },
        "spec": {
          "displayName": "hmac-reject",
          "version": "v1.0",
          "context": "/hmac-reject",
          "allChannels": {
            "on_message_received": {
              "policies": [
                {
                  "name": "websub-hmac-auth",
                  "version": "v1"
                }
              ]
            }
          },
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "hmac-reject-key" for WebSub API "hmac-reject-v1-0"
    Then the response status code should be 201
    And I wait for 3 seconds

    When I subscribe to topic "events" on API "hmac-reject" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202
    And I wait for 2 seconds

    When I publish event "unauthorized-payload" to topic "events" on API "hmac-reject" version "v1.0"
    Then the response status code should be 401

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "hmac-reject-v1-0"
    Then the response should be successful

  Scenario: Publishing with the regenerated secret value passes the websub-hmac-auth policy
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "hmac-regen-v1-0" },
        "spec": {
          "displayName": "hmac-regen",
          "version": "v1.0",
          "context": "/hmac-regen",
          "allChannels": {
            "on_message_received": {
              "policies": [
                {
                  "name": "websub-hmac-auth",
                  "version": "v1"
                }
              ]
            }
          },
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "hmac-regen-key" for WebSub API "hmac-regen-v1-0"
    Then the response status code should be 201
    And I wait for 3 seconds

    When I subscribe to topic "events" on API "hmac-regen" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202
    And I wait for 2 seconds

    # Rotate the secret — the saved value is updated to the new plaintext
    Given I authenticate using basic auth as "admin"
    When I regenerate the saved webhook secret for WebSub API "hmac-regen-v1-0"
    Then the response status code should be 200
    And I wait for 3 seconds

    # Publish signed with the regenerated secret must be accepted
    When I publish event "hmac-regen-payload-001" to topic "events" on API "hmac-regen" version "v1.0" with the saved HMAC signature
    Then the response status code should be 202

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "hmac-regen-v1-0"
    Then the response should be successful

  Scenario: Publishing is rejected after the only webhook secret is deleted
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebSubApi",
        "metadata": { "name": "hmac-deleted-v1-0" },
        "spec": {
          "displayName": "hmac-deleted",
          "version": "v1.0",
          "context": "/hmac-deleted",
          "allChannels": {
            "on_message_received": {
              "policies": [
                {
                  "name": "websub-hmac-auth",
                  "version": "v1"
                }
              ]
            }
          },
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    Given I authenticate using basic auth as "admin"
    When I create a webhook secret with display name "hmac-deleted-key" for WebSub API "hmac-deleted-v1-0"
    Then the response status code should be 201
    And I wait for 3 seconds

    When I subscribe to topic "events" on API "hmac-deleted" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202
    And I wait for 2 seconds

    # Delete the only secret — the policy now has nothing to validate against
    Given I authenticate using basic auth as "admin"
    When I delete the saved webhook secret from WebSub API "hmac-deleted-v1-0"
    Then the response status code should be 204
    And I wait for 3 seconds

    # Publish must be rejected because no secrets remain for the API
    When I publish event "hmac-deleted-payload-001" to topic "events" on API "hmac-deleted" version "v1.0" with the saved HMAC signature
    Then the response status code should be 401

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "hmac-deleted-v1-0"
    Then the response should be successful
