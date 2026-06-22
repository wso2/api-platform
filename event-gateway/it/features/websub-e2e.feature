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

Feature: WebSub End-to-End Flow
  As an event publisher and subscriber
  I want to publish events through the event gateway
  So that registered subscribers receive them via webhook delivery

  Background:
    Given the event gateway services are running

  # ==================== SUBSCRIBE ====================

  Scenario: Subscribe to a topic receives a 202 Accepted
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-subscribe-v1-0" },
        "spec": {
          "displayName": "e2e-subscribe",
          "version": "v1.0",
          "context": "/e2e-subscribe",
          "channels": { "issues": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    When I subscribe to topic "issues" on API "e2e-subscribe" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-subscribe-v1-0"
    Then the response should be successful

  Scenario: Subscribe to a topic with basic-auth policy using correct credentials
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-auth-v1-0" },
        "spec": {
          "displayName": "e2e-auth",
          "version": "v1.0",
          "context": "/e2e-auth",
          "channels": { "events": {} },
          "allChannels": {
            "on_subscription": {
              "policies": [
                {
                  "name": "basic-auth",
                  "version": "v1",
                  "params": { "username": "admin", "password": "admin" }
                }
              ]
            }
          },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    Given I authenticate using basic auth as "admin"
    When I subscribe to topic "events" on API "e2e-auth" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-auth-v1-0"
    Then the response should be successful

  Scenario: Subscribe to an unknown topic on a valid API returns an error
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-unknown-topic-v1-0" },
        "spec": {
          "displayName": "e2e-unknown-topic",
          "version": "v1.0",
          "context": "/e2e-unknown-topic",
          "channels": { "known": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    When I subscribe to topic "does-not-exist" on API "e2e-unknown-topic" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-unknown-topic-v1-0"
    Then the response should be successful

  # ==================== PUBLISH ====================

  Scenario: Publish an event to a known topic returns 202
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-publish-v1-0" },
        "spec": {
          "displayName": "e2e-publish",
          "version": "v1.0",
          "context": "/e2e-publish",
          "channels": { "orders": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    When I publish event "order-created-001" to topic "orders" on API "e2e-publish" version "v1.0"
    Then the response status code should be 202

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-publish-v1-0"
    Then the response should be successful

  Scenario: Publish to an unknown topic returns an error
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-bad-topic-v1-0" },
        "spec": {
          "displayName": "e2e-bad-topic",
          "version": "v1.0",
          "context": "/e2e-bad-topic",
          "channels": { "real": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    When I publish event "event-body" to topic "imaginary" on API "e2e-bad-topic" version "v1.0"
    Then the response status code should be 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-bad-topic-v1-0"
    Then the response should be successful

  # ==================== FULL E2E FLOW ====================

  Scenario: Full subscribe, publish, and delivery flow
    # Step 1: Create the API
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-full-v1-0" },
        "spec": {
          "displayName": "e2e-full",
          "version": "v1.0",
          "context": "/e2e-full",
          "channels": {
            "issues": {},
            "pull-requests": {}
          },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    # Step 2: Subscribe to the "issues" topic
    When I subscribe to topic "issues" on API "e2e-full" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202

    # Step 3: Wait for subscription verification to complete
    And I wait for 2 seconds

    # Step 4: Publish an event
    When I publish event "issue-123-opened" to topic "issues" on API "e2e-full" version "v1.0"
    Then the response status code should be 202

    # Step 5: Wait for async delivery
    And I wait for event delivery for 3 seconds

    # Step 6: Confirm delivery was accepted
    Then the webhook listener should have received the event "issue-123-opened"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-full-v1-0"
    Then the response should be successful

  # ==================== UNSUBSCRIBE ====================

  Scenario: Unsubscribe from a topic
    Given I authenticate using basic auth as "admin"
    When I create a WebSub API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha2",
        "kind": "WebSubApi",
        "metadata": { "name": "e2e-unsub-v1-0" },
        "spec": {
          "displayName": "e2e-unsub",
          "version": "v1.0",
          "context": "/e2e-unsub",
          "channels": { "news": {} },
          "deploymentState": "deployed"
        }
      }
      """
    Then the response should be successful
    And I wait for 3 seconds

    When I subscribe to topic "news" on API "e2e-unsub" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202
    And I wait for 2 seconds

    When I unsubscribe from topic "news" on API "e2e-unsub" version "v1.0" with callback "http://wh-listener:8090/"
    Then the response status code should be 202

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebSub API "e2e-unsub-v1-0"
    Then the response should be successful
