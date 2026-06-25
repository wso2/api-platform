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

Feature: WebSub API Management
  As an API developer
  I want to manage WebSub APIs via the gateway controller REST API
  So that I can register and configure pub/sub channels

  Background:
    Given the event gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== CREATE ====================

  Scenario: Create a WebSub API with multiple channels
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": {
          "name": "repo-watcher-v1-0"
        },
        "spec": {
          "displayName": "repo-watcher",
          "version": "v1.0",
          "context": "/repos",
          "channels": {
            "issues": {},
            "pull-requests": {},
            "commits": {}
          },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "repo-watcher-v1-0"
    Then the response should be successful

  Scenario: Create a WebSub API with basic-auth policy
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": {
          "name": "secure-events-v1-0"
        },
        "spec": {
          "displayName": "secure-events",
          "version": "v1.0",
          "context": "/secure",
          "channels": {
            "alerts": {}
          },
          "allChannels": {
            "on_subscription": {
              "policies": [
                {
                  "name": "basic-auth",
                  "version": "v1",
                  "params": {
                    "username": "admin",
                    "password": "admin"
                  }
                }
              ]
            }
          },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "secure-events-v1-0"
    Then the response should be successful

  # ==================== READ ====================

  Scenario: List all WebSub APIs
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": { "name": "list-test-v1-0" },
        "spec": {
          "displayName": "list-test",
          "version": "v1.0",
          "context": "/list-test",
          "channels": { "ch1": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I list all WebSub APIs
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "list-test-v1-0"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "list-test-v1-0"
    Then the response should be successful

  Scenario: Get a specific WebSub API by name
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": { "name": "get-test-v1-0" },
        "spec": {
          "displayName": "get-test",
          "version": "v1.0",
          "context": "/get-test",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I get the WebSub API "get-test-v1-0"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "get-test-v1-0"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "get-test-v1-0"
    Then the response should be successful

  Scenario: Get a non-existent WebSub API returns 404
    Given I authenticate using basic auth as "admin"
    When I get the WebSub API "does-not-exist"
    Then the response status code should be 404

  # ==================== UPDATE ====================

  Scenario: Update a WebSub API to add a new channel
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": { "name": "update-test-v1-0" },
        "spec": {
          "displayName": "update-test",
          "version": "v1.0",
          "context": "/update-test",
          "channels": { "channel-a": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I update the WebSub API "update-test-v1-0" with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": { "name": "update-test-v1-0" },
        "spec": {
          "displayName": "update-test",
          "version": "v1.0",
          "context": "/update-test",
          "channels": {
            "channel-a": {},
            "channel-b": {}
          },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And the response should be valid JSON

    # Verify
    Given I authenticate using basic auth as "admin"
    When I get the WebSub API "update-test-v1-0"
    Then the response should be successful
    And the response body should contain "channel-b"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "update-test-v1-0"
    Then the response should be successful

  # ==================== DELETE ====================

  Scenario: Delete a WebSub API
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1",
        "kind": "WebSubApi",
        "metadata": { "name": "delete-test-v1-0" },
        "spec": {
          "displayName": "delete-test",
          "version": "v1.0",
          "context": "/delete-test",
          "channels": { "events": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "delete-test-v1-0"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I get the WebSub API "delete-test-v1-0"
    Then the response status code should be 404

  Scenario: Delete a non-existent WebSub API returns 404
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "non-existent-api"
    Then the response status code should be 404
