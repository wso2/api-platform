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

@websocket-deploy
Feature: WebSocket API CRUD and connectivity
  As a gateway operator
  I want to deploy a WebSocketApi configuration against the gateway-controller
  So that I can verify handshake routing and connectivity behavior

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy a WebSocket API and exchange frames over the upgraded connection
    Given I generate a unique resource name from "websocket-e2e" and store it as "wsName"
    And I generate a unique value from "websocket-e2e" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-e2e" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-e2e" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName}           |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3019/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101

    When I send a WebSocket text frame "hello"
    Then I should receive the WebSocket text frame "hello"

    When I send a WebSocket ping
    Then I should receive a WebSocket pong

    When I close the WebSocket connection

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Update a deployed WebSocket API's upstream, and verify the change takes effect
    Given I generate a unique resource name from "websocket-update" and store it as "wsName"
    And I generate a unique value from "websocket-update" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-update" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-update" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName}           |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3019/ws?subprotocol=v1 |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" with subprotocol "v1" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection

    When I update WebSocket API "${CTX:wsName}" from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName} v2        |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3019/ws?subprotocol=v2 |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "spec.displayName" should be "${CTX:wsDisplayName} v2"

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" with subprotocol "v2" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # The upstream's own handshake rejection must pass through unmodified rather than being
  # swallowed or rewritten — this is what proves the gateway proxies the handshake rather than
  # terminating it as its own HTTP response. "until status 403" absorbs the same deploy-to-xDS
  # propagation gap every other first data-plane call in this suite retries through; without it,
  # a handshake fired before the channel route propagates would still land on 403 (Envoy rejects
  # any Upgrade request against a route that hasn't enabled that upgrade type), making the
  # scenario pass without ever actually reaching the backend it claims to exercise.
  Scenario: A rejected upstream handshake is passed through to the client
    Given I generate a unique resource name from "websocket-reject" and store it as "wsName"
    And I generate a unique value from "websocket-reject" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-reject" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-reject" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName}           |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3019/ws?rejectStatus=403 |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 403
    Then the response status code should be 403

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # A handshake to a path with no matching channel must not route to the backend — only GET
  # requests to a configured channel path carry the upgrade. The sibling "/rooms" channel is
  # opened and closed first, purely to confirm this API's routes have propagated to Envoy (the
  # same propagation gap the other scenarios retry through); once confirmed, the unconfigured
  # path is asserted without a retry, since its outcome no longer depends on timing.
  #
  # The expected status is 403, not 404: Envoy's upgrade handling rejects any Upgrade request
  # against a route that hasn't enabled that upgrade type before that route's own action (here,
  # the catch-all's direct_response 404) ever runs. "No matching channel" and "no route at all"
  # are therefore indistinguishable from the client's perspective — both surface as 403.
  Scenario: A handshake to an unconfigured channel path is rejected
    Given I generate a unique resource name from "websocket-unknown" and store it as "wsName"
    And I generate a unique value from "websocket-unknown" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-unknown" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-unknown" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName}           |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3019/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection

    When I open a WebSocket connection to "${CTX:wsContext}/not-a-channel"
    Then the response status code should be 403

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful
