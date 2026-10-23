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

# Runs in the mcp-policies block, whose platform-gateway carries the mcp-jwt-auth.toml overlay
# registering the "mock-jwks" issuer jwt-auth validates against. Every policy here runs once, at
# the handshake: a WebSocket connection is a single HTTP exchange as far as the gateway is
# concerned, so the policy verdict decides whether the connection is upgraded at all. Header-based
# api-key-auth is covered in websocket_deploy.feature.
@websocket-policies
Feature: WebSocket API policy enforcement at the handshake
  As a gateway operator
  I want authentication and rate-limit policies applied to WebSocket handshakes
  So that only permitted clients open connections to the backend

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # jwt-auth reads the token from a request header, so this protects non-browser clients; a
  # browser cannot set Authorization on a WebSocket handshake.
  Scenario: A WebSocket API with jwt-auth rejects handshakes without a valid token
    Given I generate a unique resource name from "websocket-jwt" and store it as "wsName"
    And I generate a unique value from "websocket-jwt" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-jwt" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-jwt" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                           |
      | name                   | ${CTX:wsName}                                                       |
      | spec.displayName       | ${CTX:wsDisplayName}                                                |
      | spec.version           | ${CTX:wsVersion}                                                    |
      | spec.context           | ${CTX:wsContext}                                                    |
      | spec.upstream.main.url | ws://testbench:3019/ws                                              |
      | spec.channels          | [{"path":"/rooms"}]                                                 |
      | spec.policies          | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 401
    Then the response status code should be 401

    When I set header "Authorization" to "Bearer not-a-valid-token"
    And I open a WebSocket connection to "${CTX:wsContext}/rooms"
    Then the response status code should be 401

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "wsToken"
    And I set header "Authorization" to "Bearer ${CTX:wsToken}"
    And I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I send a WebSocket text frame "jwt"
    Then I should receive the WebSocket text frame "jwt"
    When I close the WebSocket connection

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful

  # basic-ratelimit counts handshakes, since each connection is one request to the gateway: with
  # a limit of 3, the readiness handshake and two more are upgraded and the fourth gets 429.
  # Frames on an open connection are not requests and are never counted.
  Scenario: A WebSocket API with basic-ratelimit limits how often connections can be opened
    Given I generate a unique resource name from "websocket-ratelimit" and store it as "wsName"
    And I generate a unique value from "websocket-ratelimit" and store it as "wsDisplayName"
    And I generate a unique API version from "websocket-ratelimit" and store it as "wsVersion"
    And I generate a unique API context from "/websocket-ratelimit" and store it as "wsContext"
    When I create WebSocket API from "resources/templates/websocket-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                  |
      | name                   | ${CTX:wsName}                                                                              |
      | spec.displayName       | ${CTX:wsDisplayName}                                                                       |
      | spec.version           | ${CTX:wsVersion}                                                                           |
      | spec.context           | ${CTX:wsContext}                                                                           |
      | spec.upstream.main.url | ws://testbench:3019/ws                                                                     |
      | spec.channels          | [{"path":"/rooms"}]                                                                        |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}] |
    Then the response should be successful

    When I open a WebSocket connection to "${CTX:wsContext}/rooms" until status 101
    Then the response status code should be 101
    When I send a WebSocket text frame "frame-1"
    Then I should receive the WebSocket text frame "frame-1"
    When I send a WebSocket text frame "frame-2"
    Then I should receive the WebSocket text frame "frame-2"
    When I send a WebSocket text frame "frame-3"
    Then I should receive the WebSocket text frame "frame-3"
    When I send a WebSocket text frame "frame-4"
    Then I should receive the WebSocket text frame "frame-4"
    When I close the WebSocket connection

    When I open a WebSocket connection to "${CTX:wsContext}/rooms"
    Then the response status code should be 101
    When I close the WebSocket connection
    When I open a WebSocket connection to "${CTX:wsContext}/rooms"
    Then the response status code should be 101
    When I close the WebSocket connection
    When I open a WebSocket connection to "${CTX:wsContext}/rooms"
    Then the response status code should be 429

    When I delete the WebSocket API "${CTX:wsName}"
    Then the response should be successful
