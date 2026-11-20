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

@path-variants
Feature: Path variants
  As an API developer
  I want a variant spelling of an operation's path to be refused
  So that it cannot reach a broader operation without the policies of the one it names

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: A case variant of an operation's path gets 404 instead of reaching the wildcard operation
    Given I generate a unique value from "path-variants-2" and store it as "apiName2"
    And I generate a unique API context from "/path-variants-2" and store it as "apiContext2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}        |
      | name                  | ${CTX:apiName2}                  |
      | spec.displayName      | Path-Case-Variant-API             |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext2}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/*"},{"method":"GET","path":"/secure/data"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/secure/data" until status 200

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/secure/data"
    Then the response status code should be 200
    And the JSON response field "path" should be "/secure/data"

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/Secure/data"
    Then the response status code should be 404

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/SECURE/DATA"
    Then the response status code should be 404

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/other"
    Then the response status code should be 200
    And the JSON response field "path" should be "/other"

    When I delete the API "${CTX:apiName2}"
    Then the response should be successful

  Scenario: Case variants of several operations are all refused while their parameters keep any case
    Given I generate a unique value from "path-variants-3" and store it as "apiName3"
    And I generate a unique API context from "/path-variants-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}        |
      | name                  | ${CTX:apiName3}                  |
      | spec.displayName      | Path-Case-Variant-Shared-API      |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext3}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/*"},{"method":"GET","path":"/secure/data"},{"method":"GET","path":"/alerts/active"},{"method":"GET","path":"/orders/open"},{"method":"GET","path":"/items/{id}"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/orders/open" until status 200

    When I send a "GET" request to "${CTX:apiContext3}/v1.0/secure/data"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/alerts/active"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/items/ABC"
    Then the response status code should be 200
    And the JSON response field "path" should be "/items/ABC"

    When I send a "GET" request to "${CTX:apiContext3}/v1.0/SECURE/DATA"
    Then the response status code should be 404
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/Alerts/Active"
    Then the response status code should be 404
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/orders/OPEN"
    Then the response status code should be 404
    When I send a "GET" request to "${CTX:apiContext3}/v1.0/Items/abc"
    Then the response status code should be 404

    When I send a "GET" request to "${CTX:apiContext3}/v1.0/other"
    Then the response status code should be 200

    When I delete the API "${CTX:apiName3}"
    Then the response should be successful

  Scenario: An operation spelt with capitals keeps its own route next to a lowercase one
    Given I generate a unique value from "path-variants-4" and store it as "apiName4"
    And I generate a unique API context from "/path-variants-4" and store it as "apiContext4"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}        |
      | name                  | ${CTX:apiName4}                  |
      | spec.displayName      | Path-Case-Variant-Spelling-API    |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext4}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/*","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"wildcard"}]}}}]},{"method":"GET","path":"/users","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"lowercase"}]}}}]},{"method":"GET","path":"/Users","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"capitalised"}]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0/users" until status 200

    When I send a "GET" request to "${CTX:apiContext4}/v1.0/users"
    Then the response status code should be 200
    And the response header "X-Matched-Operation" should be "lowercase"

    When I send a "GET" request to "${CTX:apiContext4}/v1.0/Users"
    Then the response status code should be 200
    And the response header "X-Matched-Operation" should be "capitalised"

    When I send a "GET" request to "${CTX:apiContext4}/v1.0/USERS"
    Then the response status code should be 404

    When I delete the API "${CTX:apiName4}"
    Then the response should be successful

  Scenario: A case variant is refused only for the methods of the operation it names
    Given I generate a unique value from "path-variants-5" and store it as "apiName5"
    And I generate a unique API context from "/path-variants-5" and store it as "apiContext5"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}        |
      | name                  | ${CTX:apiName5}                  |
      | spec.displayName      | Path-Case-Variant-Method-API      |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:apiContext5}/$version      |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/*"},{"method":"GET","path":"/secure/data"},{"method":"POST","path":"/*","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"post-wildcard"}]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0/secure/data" until status 200

    When I send a "GET" request to "${CTX:apiContext5}/v1.0/Secure/data"
    Then the response status code should be 404

    When I send a "POST" request to "${CTX:apiContext5}/v1.0/Secure/data"
    Then the response status code should be 200
    And the response header "X-Matched-Operation" should be "post-wildcard"

    When I delete the API "${CTX:apiName5}"
    Then the response should be successful

  Scenario: A case variant is refused when another API's wildcard operation would catch it
    Given I generate a unique value from "path-variants-outer" and store it as "outerName"
    And I generate a unique API context from "/path-variants-outer" and store it as "outerContext"
    And I generate a unique value from "path-variants-inner" and store it as "innerName"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}        |
      | name                  | ${CTX:outerName}                 |
      | spec.displayName      | Path-Case-Variant-Outer-API       |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:outerContext}/$version     |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/*","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"X-Matched-Operation","value":"outer-wildcard"}]}}}]}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}        |
      | name                  | ${CTX:innerName}                 |
      | spec.displayName      | Path-Case-Variant-Inner-API       |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:outerContext}/v1.0/inner/$version |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations       | [{"method":"GET","path":"/secure/data"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:outerContext}/v1.0/inner/v1.0/secure/data" until status 200

    When I send a "GET" request to "${CTX:outerContext}/v1.0/other"
    Then the response status code should be 200
    And the response header "X-Matched-Operation" should be "outer-wildcard"

    When I send a "GET" request to "${CTX:outerContext}/v1.0/inner/v1.0/Secure/data"
    Then the response status code should be 404

    When I delete the API "${CTX:innerName}"
    Then the response should be successful
    When I delete the API "${CTX:outerName}"
    Then the response should be successful
