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

@SVR @platform-api-vhosts
Feature: A deployment through the control plane carries the API's vhosts to the gateway
  As an API publisher
  I want to choose the hosts an API answers on when I deploy it, or leave them to the gateway
  So that each gateway serves the API's main and sandbox traffic on the right hosts

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "svr-vhost-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I store the registered gateway id as "gatewayId"

  @SVR-017
  Scenario: A deployment with its own main and sandbox vhosts is served on those hosts
    Given I generate a unique resource name from "svr-017" and store it as "apiHandle"
    And I generate a unique API context from "/svr-017" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and a sandbox upstream
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/gateways" with body:
      """
      [{"gatewayId": "${CTX:gatewayId}"}]
      """
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "own-vhosts", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "${CTX:apiHandle}.example.com", "vhostSandbox": "${CTX:apiHandle}-sandbox.example.com"}}
      """
    Then the response status code should be 201
    And the JSON response field "metadata.vhostMain" should be "${CTX:apiHandle}.example.com"
    And the JSON response field "metadata.vhostSandbox" should be "${CTX:apiHandle}-sandbox.example.com"
    When I set request host to "${CTX:apiHandle}.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 200
    Then the JSON response field "path" should be "/whoami"
    When I clear all headers
    And I set request host to "${CTX:apiHandle}-sandbox.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 200
    Then the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  @SVR-018
  Scenario: A deployment with no vhosts carries the gateway-default placeholder, and the gateway stores its own defaults
    Given I generate a unique resource name from "svr-018-sb" and store it as "sandboxApi"
    And I generate a unique API context from "/svr-018-sb" and store it as "sandboxContext"
    And I generate a unique resource name from "svr-018-main" and store it as "mainApi"
    And I generate a unique API context from "/svr-018-main" and store it as "mainContext"
    And I create a REST API "${CTX:sandboxApi}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:sandboxContext}" and a sandbox upstream
    And I create a REST API "${CTX:mainApi}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:mainContext}"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:sandboxApi}/gateways" with body:
      """
      [{"gatewayId": "${CTX:gatewayId}"}]
      """
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:mainApi}/gateways" with body:
      """
      [{"gatewayId": "${CTX:gatewayId}"}]
      """
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:sandboxApi}/deployments" with body:
      """
      {"name": "default-vhosts", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And the JSON response field "metadata.vhostMain" should be "_gateway_default_"
    And the JSON response field "metadata.vhostSandbox" should be "_gateway_default_"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:mainApi}/deployments" with body:
      """
      {"name": "default-vhosts", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And the JSON response field "metadata.vhostMain" should be "_gateway_default_"
    And the JSON response field "metadata.vhostSandbox" should not exist
    # The gateway resolves the placeholder to its own defaults (* and sandbox-* in this block).
    When I send a "GET" request to "${CTX:sandboxContext}/whoami" until status 200
    And I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:sandboxApi}"
    Then the response status code should be 200
    And the JSON response field "spec.vhosts.main" should be "*"
    And the JSON response field "spec.vhosts.sandbox" should be "sandbox-*"
    When I set request host to "sandbox-svr-018.example.com"
    And I send a "GET" request to "${CTX:sandboxContext}/whoami" until status 200
    Then the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

  @SVR-019
  Scenario: Restoring an earlier deployment brings back the vhosts it was deployed with
    Given I generate a unique resource name from "svr-019" and store it as "apiHandle"
    And I generate a unique API context from "/svr-019" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and a sandbox upstream
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/gateways" with body:
      """
      [{"gatewayId": "${CTX:gatewayId}"}]
      """
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "d1-own-vhosts", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "${CTX:apiHandle}.example.com", "vhostSandbox": "${CTX:apiHandle}-sandbox.example.com"}}
      """
    Then the response status code should be 201
    And I store the JSON response field "deploymentId" as "d1"
    And I set request host to "${CTX:apiHandle}.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 200

    # D2 moves the API to other hosts, so D1's host no longer reaches it.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "d2-other-vhosts", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "${CTX:apiHandle}-d2.example.com", "vhostSandbox": "${CTX:apiHandle}-d2-sandbox.example.com"}}
      """
    Then the response status code should be 201
    When I set request host to "${CTX:apiHandle}-d2.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 200
    And I clear all headers
    And I set request host to "${CTX:apiHandle}.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 404

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments/${CTX:d1}/restore?gatewayId=${CTX:gatewayId}"
    Then the response should be successful
    And the JSON response field "metadata.vhostMain" should be "${CTX:apiHandle}.example.com"
    And the JSON response field "metadata.vhostSandbox" should be "${CTX:apiHandle}-sandbox.example.com"
    When I set request host to "${CTX:apiHandle}.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 200
    Then the JSON response field "path" should be "/whoami"
    When I clear all headers
    And I set request host to "${CTX:apiHandle}-sandbox.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 200
    Then the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"
    When I clear all headers
    And I set request host to "${CTX:apiHandle}-d2.example.com"
    And I send a "GET" request to "${CTX:apiContext}/whoami" until status 404

  # An invalid vhost is refused with 500 INTERNAL_ERROR: https://github.com/wso2/api-platform/issues/4166
  @SVR-020 @known-issue
  Scenario: A deployment with an invalid vhost is refused with a validation error, while the gateway-default placeholder is accepted
    Given I generate a unique resource name from "svr-020" and store it as "apiHandle"
    And I generate a unique API context from "/svr-020" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/gateways" with body:
      """
      [{"gatewayId": "${CTX:gatewayId}"}]
      """
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "placeholder", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "_gateway_default_"}}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "underscore", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "bad_host"}}
      """
    Then the response status code should be 400
    And the response body should contain "vhost"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "hyphen", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "-api.example.com"}}
      """
    Then the response status code should be 400
    And the response body should contain "vhost"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "long-label", "base": "current", "gatewayId": "${CTX:gatewayId}", "metadata": {"vhostMain": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com"}}
      """
    Then the response status code should be 400
    And the response body should contain "vhost"
