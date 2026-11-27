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

@backend-management @GWBE
Feature: Network access control for templated Backends
  As a platform administrator who restricts where the gateway may connect
  I want a templated Backend held to the same rules as any upstream
  So that a key a request carries can never reach a refused host or address

  # The block refuses private networks and hosts under blocked.example
  # (overlays/backend-access-control.toml).
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @GWBE-009 @type:negative
  Scenario: A templated Backend is refused by host when saved and by address when a request resolves it
    Given I generate a unique resource name from "ep-blocked" and store it as "blockedName"
    And I generate a unique resource name from "ep-private" and store it as "backendName"
    And I generate a unique value from "backend-private-api" and store it as "apiName"
    And I generate a unique API context from "/backend-private" and store it as "apiContext"

    # The template's fixed domain is denied by name.
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                     |
      | name                 | ${CTX:blockedName}                           |
      | spec.endpoints       | [{"url":"http://api-{key}.blocked.example"}]  |
    Then the response status code should be 400
    And the response body should contain "spec.endpoints[0].url"

    # The template itself is permitted; the hosts it expands to resolve to private addresses.
    When I create Backend from "resources/templates/backend.yaml" with values:
      | apiVersion           | ${CTX:backendSpecVersion}                     |
      | name                 | ${CTX:backendName}                           |
      | spec.endpoints       | [{"url":"http://env-{key}.testbench:3000"}]   |
    Then the response status code should be 201

    # The API's own upstream is never used: every request goes to the templated Backend. Its name
    # doesn't resolve, so access control leaves it to the router.
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://unused.invalid:3000          |
      | spec.policies          | [{"name":"dynamic-endpoint","version":"v1","params":{"targetUpstream":"${CTX:backendName}","backendKeyExpression":"request.Headers['env'][0]"}}] |
      | spec.operations        | [{"method":"GET","path":"/items"}]  |
    Then the response should be successful

    # env-dev.testbench resolves (GWBE-008 reaches it in the gateway-core block), but only to a
    # private address, which the router drops, so the request never reaches the testbench.
    When I set header "env" to "dev"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items" until status 503
    Then the response body should not contain "env-dev.testbench"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I delete the Backend "${CTX:backendName}"
    Then the response status code should be 200
