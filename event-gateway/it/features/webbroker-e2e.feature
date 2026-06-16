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

Feature: WebBroker End-to-End Flow
  As a WebSocket client
  I want to produce and consume messages through the event gateway
  So that bidirectional event streaming works correctly

  # Each scenario uses a unique API name and context path so that scenarios
  # are fully independent and do not interfere with each other.
  #
  # API channel layout (shared across scenarios that need both channels):
  #   prices — produceTo: stock.prices, consumeFrom: dummy.prices (separate topics)
  #   echo   — no mapping; produces and consumes from the "echo" topic (self-echo)

  Background:
    Given the event gateway services are running

  # ==================== CONNECTION ====================

  Scenario: Connect to an unknown channel is rejected
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "stock-trading-conn-v1.0" },
        "spec": {
          "displayName": "Stock Trading Conn API",
          "version": "v1.0",
          "context": "/stock-trading-conn/v1.0",
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
            "prices": {
              "produceTo": { "topic": "stock.prices" },
              "consumeFrom": { "topic": "dummy.prices" },
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            },
            "echo": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the WebBroker API at "/stock-trading-conn/v1.0" is reachable within 30 seconds

    When I connect to WebBroker API "/stock-trading-conn/v1.0" on channel "ghost"
    Then the WebSocket connection should be rejected with HTTP status 404

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "stock-trading-conn-v1.0"
    Then the response should be successful

  # ==================== ECHO CHANNEL ====================

  Scenario: Echo channel - send a message and receive it back
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "stock-trading-echo-v1.0" },
        "spec": {
          "displayName": "Stock Trading Echo API",
          "version": "v1.0",
          "context": "/stock-trading-echo/v1.0",
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
            "prices": {
              "produceTo": { "topic": "stock.prices" },
              "consumeFrom": { "topic": "dummy.prices" },
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            },
            "echo": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the WebBroker API at "/stock-trading-echo/v1.0" is reachable within 30 seconds

    When I connect to WebBroker API "/stock-trading-echo/v1.0" on channel "echo"
    And I wait for 1 seconds
    And I send WebSocket message "echo-hello-world"
    Then I should receive a WebSocket message containing "echo-hello-world" within 10 seconds

    When I close the WebSocket connection

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "stock-trading-echo-v1.0"
    Then the response should be successful

  # ==================== PRICES CHANNEL ====================

  Scenario: Prices channel - produced message arrives at stock.prices Kafka topic
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "stock-trading-pprod-v1.0" },
        "spec": {
          "displayName": "Stock Trading PProd API",
          "version": "v1.0",
          "context": "/stock-trading-pprod/v1.0",
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
            "prices": {
              "produceTo": { "topic": "stock.prices" },
              "consumeFrom": { "topic": "dummy.prices" },
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            },
            "echo": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the WebBroker API at "/stock-trading-pprod/v1.0" is reachable within 30 seconds

    When I connect to WebBroker API "/stock-trading-pprod/v1.0" on channel "prices"
    And I wait for 1 seconds
    And I send WebSocket message "price-update-001"
    Then the Kafka topic "stock.prices" should contain a message with "price-update-001" within 20 seconds

    When I close the WebSocket connection

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "stock-trading-pprod-v1.0"
    Then the response should be successful

  Scenario: Prices channel - message published to dummy.prices is received via WebSocket
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "stock-trading-pcons-v1.0" },
        "spec": {
          "displayName": "Stock Trading PCons API",
          "version": "v1.0",
          "context": "/stock-trading-pcons/v1.0",
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
            "prices": {
              "produceTo": { "topic": "stock.prices" },
              "consumeFrom": { "topic": "dummy.prices" },
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            },
            "echo": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the WebBroker API at "/stock-trading-pcons/v1.0" is reachable within 30 seconds

    # Connect first so the gateway consumer is ready before we publish.
    When I connect to WebBroker API "/stock-trading-pcons/v1.0" on channel "prices"
    And I wait for 1 seconds
    And I publish "dummy-price-001" to Kafka topic "dummy.prices"
    Then I should receive a WebSocket message containing "dummy-price-001" within 10 seconds

    When I close the WebSocket connection

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "stock-trading-pcons-v1.0"
    Then the response should be successful

  # ==================== AUTH ON CONNECTION INIT ====================

  Scenario: Connect without credentials is rejected when basic-auth is on connection init
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "stock-trading-auth-rej-v1.0" },
        "spec": {
          "displayName": "Stock Trading Auth Rej API",
          "version": "v1.0",
          "context": "/stock-trading-auth-rej/v1.0",
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
            "echo": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the WebBroker API at "/stock-trading-auth-rej/v1.0" is reachable within 30 seconds

    # Connect without any Authorization header.
    Given I clear all authentication headers
    When I connect to WebBroker API "/stock-trading-auth-rej/v1.0" on channel "echo"
    Then the WebSocket connection should be rejected with HTTP status 401

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "stock-trading-auth-rej-v1.0"
    Then the response should be successful

  Scenario: Connect with correct credentials succeeds and echo works
    Given I authenticate using basic auth as "admin"
    When I create a WebBroker API with the following configuration:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "WebBrokerApi",
        "metadata": { "name": "stock-trading-auth-ok-v1.0" },
        "spec": {
          "displayName": "Stock Trading Auth OK API",
          "version": "v1.0",
          "context": "/stock-trading-auth-ok/v1.0",
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
            "echo": {
              "on_connection_init": { "policies": [] },
              "on_produce": { "policies": [] },
              "on_consume": { "policies": [] }
            }
          }
        }
      }
      """
    Then the response should be successful
    And the WebBroker API at "/stock-trading-auth-ok/v1.0" is reachable within 30 seconds

    Given I authenticate using basic auth as "admin"
    When I connect to WebBroker API "/stock-trading-auth-ok/v1.0" on channel "echo"
    And I wait for 1 seconds
    And I send WebSocket message "auth-echo-test"
    Then I should receive a WebSocket message containing "auth-echo-test" within 10 seconds

    When I close the WebSocket connection

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the WebBroker API "stock-trading-auth-ok-v1.0"
    Then the response should be successful
