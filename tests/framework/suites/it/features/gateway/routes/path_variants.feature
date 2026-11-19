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
