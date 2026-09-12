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

@cel-conditions
Feature: CEL policy execution conditions
  As an API developer
  I want to use CEL expressions to conditionally execute policies
  So that I can apply policies based on request attributes

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Policy executes only on POST requests using a method condition
    Given I generate a unique value from "cel-method" and store it as "apiName"
    And I generate a unique API version from "cel-method" and store it as "apiVersion"
    And I generate a unique API context from "/cel-method" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/resource","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method == \"POST\"","params":{"request":{"headers":[{"name":"X-Cel-Executed","value":"true"}]}}}]},{"method":"POST","path":"/resource","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method == \"POST\"","params":{"request":{"headers":[{"name":"X-Cel-Executed","value":"true"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200
    And the JSON response field "headers.X-Cel-Executed[0]" should not exist

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Cel-Executed[0]" should be "true"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Policy executes on multiple methods using the in operator
    Given I generate a unique value from "cel-multi-method" and store it as "apiName"
    And I generate a unique API version from "cel-multi-method" and store it as "apiVersion"
    And I generate a unique API context from "/cel-multi-method" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/data","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method in [\"POST\", \"PUT\", \"DELETE\"]","params":{"request":{"headers":[{"name":"X-Write-Operation","value":"true"}]}}}]},{"method":"POST","path":"/data","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method in [\"POST\", \"PUT\", \"DELETE\"]","params":{"request":{"headers":[{"name":"X-Write-Operation","value":"true"}]}}}]},{"method":"PUT","path":"/data","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method in [\"POST\", \"PUT\", \"DELETE\"]","params":{"request":{"headers":[{"name":"X-Write-Operation","value":"true"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the JSON response field "headers.X-Write-Operation[0]" should not exist

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/data" with body:
      """
      {"action": "create"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Write-Operation[0]" should be "true"

    When I send a "PUT" request to "${CTX:apiContext}/${CTX:apiVersion}/data" with body:
      """
      {"action": "update"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Write-Operation[0]" should be "true"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Policy executes only when a specific header is present
    Given I generate a unique value from "cel-header-presence" and store it as "apiName"
    And I generate a unique API version from "cel-header-presence" and store it as "apiVersion"
    And I generate a unique API context from "/cel-header-presence" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/check","policies":[{"name":"set-headers","version":"v1","executionCondition":"\"x-special-token\" in request.Headers","params":{"request":{"headers":[{"name":"X-Token-Validated","value":"true"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/check"
    Then the response status code should be 200
    And the JSON response field "headers.X-Token-Validated[0]" should not exist

    When I set header "X-Special-Token" to "my-secret-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/check"
    Then the response status code should be 200
    And the JSON response field "headers.X-Token-Validated[0]" should be "true"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Policy executes based on a header value
    Given I generate a unique value from "cel-header-value" and store it as "apiName"
    And I generate a unique API version from "cel-header-value" and store it as "apiVersion"
    And I generate a unique API context from "/cel-header-value" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/premium","policies":[{"name":"set-headers","version":"v1","executionCondition":"\"x-tier\" in request.Headers && request.Headers[\"x-tier\"][0] == \"premium\"","params":{"request":{"headers":[{"name":"X-Premium-Access","value":"granted"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "X-Tier" to "basic"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/premium"
    Then the response status code should be 200
    And the JSON response field "headers.X-Premium-Access[0]" should not exist

    When I set header "X-Tier" to "premium"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/premium"
    Then the response status code should be 200
    And the JSON response field "headers.X-Premium-Access[0]" should be "granted"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Policy executes based on a path prefix
    Given I generate a unique value from "cel-path-prefix" and store it as "apiName"
    And I generate a unique API version from "cel-path-prefix" and store it as "apiVersion"
    And I generate a unique API context from "/cel-path-prefix" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/public/info","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Path.startsWith(\"${CTX:apiContext}/${CTX:apiVersion}/admin\")","params":{"request":{"headers":[{"name":"X-Admin-Request","value":"true"}]}}}]},{"method":"GET","path":"/admin/settings","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Path.startsWith(\"${CTX:apiContext}/${CTX:apiVersion}/admin\")","params":{"request":{"headers":[{"name":"X-Admin-Request","value":"true"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/public/info"
    Then the response status code should be 200
    And the JSON response field "headers.X-Admin-Request[0]" should not exist

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/admin/settings"
    Then the response status code should be 200
    And the JSON response field "headers.X-Admin-Request[0]" should be "true"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Policy executes only when combined method and header conditions both hold
    Given I generate a unique value from "cel-combined" and store it as "apiName"
    And I generate a unique API version from "cel-combined" and store it as "apiVersion"
    And I generate a unique API context from "/cel-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/secure","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method == \"POST\" && \"x-auth-token\" in request.Headers","params":{"request":{"headers":[{"name":"X-Secure-Write","value":"authorized"}]}}}]},{"method":"POST","path":"/secure","policies":[{"name":"set-headers","version":"v1","executionCondition":"request.Method == \"POST\" && \"x-auth-token\" in request.Headers","params":{"request":{"headers":[{"name":"X-Secure-Write","value":"authorized"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/secure"
    Then the response status code should be 200
    And the JSON response field "headers.X-Secure-Write[0]" should not exist

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/secure" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Secure-Write[0]" should not exist

    When I set header "X-Auth-Token" to "valid-token"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/secure" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Secure-Write[0]" should be "authorized"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
