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

Feature: WebBroker API Management
  As an API developer
  I want to manage WebBroker APIs via the gateway controller REST API
  So that I can register and configure bidirectional WebSocket channels

  Background:
    Given the event gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== CREATE ====================

  Scenario: Create a WebBroker API with multiple channels
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "wb-create-test-v1.0" },
        "spec": {
          "displayName": "WebBroker Create Test",
          "version": "v1.0",
          "context": "/wb-create-test/v1.0",
          "receiver": { "name": "websocket-receiver", "type": "websocket" },
          "broker": {
            "name": "kafka-driver",
            "type": "kafka",
            "properties": { "brokers": ["kafka:29092"] }
          },
          "allChannels": {
            "on_connection_init": { "policies": [] },
            "on_produce": { "policies": [] },
            "on_consume": { "policies": [] }
          },
          "channels": {
            "orders": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            },
            "notifications": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "wb-create-test-v1.0"
    Then the response should be successful

  Scenario: Create a WebBroker API with basic-auth policy on connection init
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "wb-auth-test-v1.0" },
        "spec": {
          "displayName": "WebBroker Auth Test",
          "version": "v1.0",
          "context": "/wb-auth-test/v1.0",
          "receiver": { "name": "websocket-receiver", "type": "websocket" },
          "broker": {
            "name": "kafka-driver",
            "type": "kafka",
            "properties": { "brokers": ["kafka:29092"] }
          },
          "allChannels": {
            "on_connection_init": {
              "policies": [
                {
                  "name": "basic-auth",
                  "version": "v1",
                  "params": { "username": "admin", "password": "admin" }
                }
              ]
            },
            "on_produce": { "policies": [] },
            "on_consume": { "policies": [] }
          },
          "channels": {
            "secure-channel": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "wb-auth-test-v1.0"
    Then the response should be successful

  # ==================== READ ====================

  Scenario: List all WebBroker APIs
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "wb-list-test-v1.0" },
        "spec": {
          "displayName": "WebBroker List Test",
          "version": "v1.0",
          "context": "/wb-list-test/v1.0",
          "receiver": { "name": "websocket-receiver", "type": "websocket" },
          "broker": {
            "name": "kafka-driver",
            "type": "kafka",
            "properties": { "brokers": ["kafka:29092"] }
          },
          "allChannels": {
            "on_connection_init": { "policies": [] },
            "on_produce": { "policies": [] },
            "on_consume": { "policies": [] }
          },
          "channels": {
            "events": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I list all WebBroker APIs
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "wb-list-test-v1.0"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "wb-list-test-v1.0"
    Then the response should be successful

  Scenario: Get a specific WebBroker API by name
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "wb-get-test-v1.0" },
        "spec": {
          "displayName": "WebBroker Get Test",
          "version": "v1.0",
          "context": "/wb-get-test/v1.0",
          "receiver": { "name": "websocket-receiver", "type": "websocket" },
          "broker": {
            "name": "kafka-driver",
            "type": "kafka",
            "properties": { "brokers": ["kafka:29092"] }
          },
          "allChannels": {
            "on_connection_init": { "policies": [] },
            "on_produce": { "policies": [] },
            "on_consume": { "policies": [] }
          },
          "channels": {
            "events": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I get the WebBroker API "wb-get-test-v1.0"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "wb-get-test-v1.0"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "wb-get-test-v1.0"
    Then the response should be successful

  Scenario: Get a non-existent WebBroker API returns 404
    When I get the WebBroker API "does-not-exist"
    Then the response status code should be 404

  # ==================== DELETE ====================

  Scenario: Delete a WebBroker API
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "wb-delete-test-v1.0" },
        "spec": {
          "displayName": "WebBroker Delete Test",
          "version": "v1.0",
          "context": "/wb-delete-test/v1.0",
          "receiver": { "name": "websocket-receiver", "type": "websocket" },
          "broker": {
            "name": "kafka-driver",
            "type": "kafka",
            "properties": { "brokers": ["kafka:29092"] }
          },
          "allChannels": {
            "on_connection_init": { "policies": [] },
            "on_produce": { "policies": [] },
            "on_consume": { "policies": [] }
          },
          "channels": {
            "events": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "wb-delete-test-v1.0"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I get the WebBroker API "wb-delete-test-v1.0"
    Then the response status code should be 404

  Scenario: Delete a non-existent WebBroker API returns 404
    When I delete the WebBroker API "non-existent-wb-api"
    Then the response status code should be 404
