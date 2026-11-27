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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@api-key-value-length @APIKEY
Feature: API key value length
  As an API administrator
  I want a custom API key value that is too short to be refused
  So that a key cannot be guessed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @APIKEY-GW-001
  Scenario: A custom API key value shorter than the minimum is refused
    Given I generate a unique value from "weak-key-api" and store it as "weakKeyApi"
    And I generate a unique API context from "/weak-key" and store it as "weakKeyCtx"
    And I generate a unique resource name from "weak-key" and store it as "weakKeyName"
    And I generate a unique resource name from "strong-key" and store it as "strongKeyName"
    And I generate a unique value from "strong-custom-key-0123456789abcdef" and store it as "strongKey"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:weakKeyApi}                 |
      | spec.displayName       | Weak-Key-API                      |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:weakKeyCtx}                 |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:weakKeyApi}/api-keys" with body:
      """
      {"name": "${CTX:weakKeyName}", "apiKey": "aaaa"}
      """
    Then the response status should be 400
    And the response body should contain "too short"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:weakKeyApi}/api-keys" with body:
      """
      {"name": "${CTX:strongKeyName}", "apiKey": "${CTX:strongKey}"}
      """
    Then the response status should be 201
    When I send a "PUT" request to the "gateway-controller" service at "/rest-apis/${CTX:weakKeyApi}/api-keys/${CTX:strongKeyName}" with body:
      """
      {"apiKey": "aaaa"}
      """
    Then the response status should be 400
    When I clear all headers
    And I set header "API-Key" to "${CTX:strongKey}"
    And I send a "GET" request to "${CTX:weakKeyCtx}/data" until status 200
    When I set header "API-Key" to "aaaa"
    And I send a "GET" request to "${CTX:weakKeyCtx}/data"
    Then the response status code should be 401
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:weakKeyApi}"
    Then the response should be successful
