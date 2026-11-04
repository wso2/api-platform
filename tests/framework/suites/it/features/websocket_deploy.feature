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
      | spec.upstream.main.url | ws://testbench:3024/ws         |
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

  # The update adds a channel, so its effect is observable on the data plane: the new path is
  # rejected before the update and upgrades after it, while the existing channel keeps working.
  Scenario: Update a deployed WebSocket API, and verify the change takes effect
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
      | spec.upstream.main.url | ws://testbench:3024/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection
    When I open a WebSocket connection to "${CTX:wsContext}/lobby"
    Then the response status code should be 404

    When I update WebSocket API "${CTX:wsName}" from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}               |
      | name                   | ${CTX:wsName}                           |
      | spec.displayName       | ${CTX:wsDisplayName} v2                 |
      | spec.version           | ${CTX:wsVersion}                        |
      | spec.context           | ${CTX:wsContext}                        |
      | spec.upstream.main.url | ws://testbench:3024/ws                  |
      | spec.channels          | [{"path":"/rooms"},{"path":"/lobby"}]   |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "spec.displayName" should be "${CTX:wsDisplayName} v2"

    When I open a WebSocket connection to "${CTX:wsContext}/lobby" until status 101
    Then the response status code should be 101
    When I send a WebSocket text frame "lobby"
    Then I should receive the WebSocket text frame "lobby"
    When I close the WebSocket connection

    When I open a WebSocket connection to "${CTX:wsContext}/rooms"
    Then the response status code should be 101
    When I close the WebSocket connection

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # The client's query string reaches the backend, so ?subprotocol asks the testbench to accept
  # the offered subprotocol; the gateway must pass the negotiated value back unchanged.
  Scenario: Subprotocol negotiation passes through the gateway
    Given I generate a unique resource name from "websocket-subprotocol" and store it as "wsName"
    And I generate a unique value from "websocket-subprotocol" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-subprotocol" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-subprotocol" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName}           |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3024/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms?subprotocol=v2" with subprotocol "v2" until status 101
    Then the response status code should be 101
    And the response header "Sec-WebSocket-Protocol" should be "v2"
    When I close the WebSocket connection

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # The upstream's own handshake rejection must pass through unmodified rather than being
  # swallowed or rewritten — this is what proves the gateway proxies the handshake rather than
  # terminating it as its own HTTP response. The route is first confirmed ready with an upgrade,
  # then the client's ?rejectStatus asks the testbench to refuse with 418: a status the gateway
  # never produces itself, so it can only have come from the backend.
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
      | spec.upstream.main.url | ws://testbench:3024/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection

    When I open a WebSocket connection to "${CTX:wsContext}/rooms?rejectStatus=418"
    Then the response status code should be 418

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # api-key-auth runs at the handshake. A 401 without a key proves the policy chain is bound;
  # the first upgrade with a generated key proves the key reached the policy engine; and a 401
  # with that same key after revocation proves revocation reaches the data plane.
  Scenario: An API-level API key policy secures the WebSocket handshake
    Given I generate a unique resource name from "websocket-apikey" and store it as "wsName"
    And I generate a unique value from "websocket-apikey" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-apikey" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-apikey" and store it as "wsContext"
    And I generate a unique value from "websocket-key" and store it as "keyName"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                          |
      | name                   | ${CTX:wsName}                                                                      |
      | spec.displayName       | ${CTX:wsDisplayName}                                                               |
      | spec.version           | ${CTX:wsVersion}                                                                   |
      | spec.context           | ${CTX:wsContext}                                                                   |
      | spec.upstream.main.url | ws://testbench:3024/ws                                                             |
      | spec.channels          | [{"path":"/rooms"}]                                                                |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 401
    Then the response status code should be 401

    When I set header "X-API-Key" to "not-a-valid-key"
    And I open a WebSocket connection to "${CTX:wsContext}/rooms"
    Then the response status code should be 401

    When I send a "POST" request to the "gateway-controller" service at "/websocket-apis/${CTX:wsName}/api-keys" with body:
      """
      {
        "name": "${CTX:keyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And I store the JSON response field "apiKey.apiKey" as "wsKey"

    When I set header "X-API-Key" to "${CTX:wsKey}"
    And I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I send a WebSocket text frame "secured"
    Then I should receive the WebSocket text frame "secured"
    When I close the WebSocket connection

    When I send a "DELETE" request to the "gateway-controller" service at "/websocket-apis/${CTX:wsName}/api-keys/${CTX:keyName}"
    Then the response status should be 200

    When I set header "X-API-Key" to "${CTX:wsKey}"
    And I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 401
    Then the response status code should be 401

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # A channel's own policies apply to that channel only: /private requires a key while /public,
  # on the same API, keeps accepting handshakes without one.
  Scenario: A channel-level API key policy secures only its own channel
    Given I generate a unique resource name from "websocket-channel-key" and store it as "wsName"
    And I generate a unique value from "websocket-channel-key" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-channel-key" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-channel-key" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                     |
      | name                   | ${CTX:wsName}                                                                                                                 |
      | spec.displayName       | ${CTX:wsDisplayName}                                                                                                          |
      | spec.version           | ${CTX:wsVersion}                                                                                                              |
      | spec.context           | ${CTX:wsContext}                                                                                                              |
      | spec.upstream.main.url | ws://testbench:3024/ws                                                                                                        |
      | spec.channels          | [{"path":"/public"},{"path":"/private","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}]}] |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/private" until status 401
    Then the response status code should be 401

    When I open a WebSocket connection to "${CTX:wsContext}/public"
    Then the response status code should be 101
    When I send a WebSocket text frame "open"
    Then I should receive the WebSocket text frame "open"
    When I close the WebSocket connection

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # A handshake to a path with no matching channel must not route to the backend — only GET
  # requests to a configured channel path carry the upgrade. The sibling "/rooms" channel is
  # opened and closed first, purely to confirm this API's routes have propagated to Envoy (the
  # same propagation gap the other scenarios retry through); once confirmed, the unconfigured
  # path is asserted without a retry, since its outcome no longer depends on timing.
  #
  # The expected status is 404, the same as a plain request to an unknown path: the gateway's
  # handshake-only catch-all route enables the upgrade so Envoy does not reject it with 403, and
  # targets a cluster that is never defined, so nothing is forwarded.
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
      | spec.upstream.main.url | ws://testbench:3024/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection

    When I open a WebSocket connection to "${CTX:wsContext}/not-a-channel"
    Then the response status code should be 404

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # A channel route matches only a WebSocket handshake. A plain GET to the same path must get
  # the gateway's own 404 and never reach the backend; before the route required
  # "Upgrade: websocket" it was proxied as an ordinary HTTP request, and this testbench backend
  # answered such a request with 400. The handshake runs first so a 404 here cannot mean the
  # route had not yet propagated.
  Scenario: A plain GET to a channel path is not proxied to the backend
    Given I generate a unique resource name from "websocket-plain-get" and store it as "wsName"
    And I generate a unique value from "websocket-plain-get" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-plain-get" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-plain-get" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:wsName}                  |
      | spec.displayName       | ${CTX:wsDisplayName}           |
      | spec.version           | ${CTX:wsVersion}               |
      | spec.context           | ${CTX:wsContext}               |
      | spec.upstream.main.url | ws://testbench:3024/ws         |
      | spec.channels          | [{"path":"/rooms"}]            |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I close the WebSocket connection

    When I send a "GET" request to "${CTX:wsContext}/rooms"
    Then the response status code should be 404
    And the response body should contain "Not Found"

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful
