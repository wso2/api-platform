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

@route-path-matching-prefix
Feature: Route path matching with wildcard prefixes
  As an API developer
  I want wildcard route prefixes to be preserved upstream
  So that requests reach the backend with their matched path

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Wildcard /foo/* preserves the matched prefix (and method) on the upstream path
    Given I generate a unique value from "route-path-matching-8" and store it as "apiName8"
    And I generate a unique API context from "/route-path-matching-8" and store it as "apiContext8"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName8}                    |
      | spec.displayName       | Route-Wildcard-Prefix-API         |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext8}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/forecast/*"},{"method":"PUT","path":"/put/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext8}/v1.0/forecast" until status 200

    When I send a "GET" request to "${CTX:apiContext8}/v1.0/forecast"
    Then the response status code should be 200
    And the JSON response field "path" should be "/forecast"

    When I send a "GET" request to "${CTX:apiContext8}/v1.0/forecast/today"
    Then the response status code should be 200
    And the JSON response field "path" should be "/forecast/today"

    When I send a "GET" request to "${CTX:apiContext8}/v1.0/forecast/a/b/c"
    Then the response status code should be 200
    And the JSON response field "path" should be "/forecast/a/b/c"

    When I send a "PUT" request to "${CTX:apiContext8}/v1.0/put" with body:
      """
      {"hello":"world"}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/put"
    And the JSON response field "method" should be "PUT"

    When I delete the API "${CTX:apiName8}"
    Then the response should be successful
