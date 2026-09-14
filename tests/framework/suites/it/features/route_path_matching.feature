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

@route-path-matching
Feature: Route path matching
  As an API developer
  I want gateway route matching to handle path variants correctly
  So that valid requests are routed and invalid variants are rejected

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Wildcard path /* matches requests with a subpath
    Given I generate a unique value from "route-path-matching-1" and store it as "apiName1"
    And I generate a unique API context from "/route-path-matching-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName1}                    |
      | spec.displayName       | Route-Wildcard-API                |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext1}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/us/seattle" until status 200

    When I send a "GET" request to "${CTX:apiContext1}/v1.0/us/seattle"
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext1}/v1.0/data"
    Then the response should be successful

    When I delete the API "${CTX:apiName1}"
    Then the response should be successful


  Scenario: Wildcard path /* enforces HTTP method
    Given I generate a unique value from "route-path-matching-2" and store it as "apiName2"
    And I generate a unique API context from "/route-path-matching-2" and store it as "apiContext2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName2}                    |
      | spec.displayName       | Route-Wildcard-Method-API         |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext2}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/data" until status 200

    When I send a "GET" request to "${CTX:apiContext2}/v1.0/data"
    Then the response should be successful

    When I send a "POST" request to "${CTX:apiContext2}/v1.0/data"
    Then the response status code should be 404

    When I delete the API "${CTX:apiName2}"
    Then the response should be successful


  Scenario: Root path / matches request with trailing slash
    Given I generate a unique value from "route-path-matching-3" and store it as "apiName3"
    And I generate a unique API context from "/route-path-matching-3" and store it as "apiContext3"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName3}                    |
      | spec.displayName       | Route-Root-API                    |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext3}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/" until status 200

    When I send a "GET" request to "${CTX:apiContext3}/v1.0/"
    Then the response should be successful

    When I delete the API "${CTX:apiName3}"
    Then the response should be successful


  Scenario: Root path / matches request without trailing slash
    Given I generate a unique value from "route-path-matching-4" and store it as "apiName4"
    And I generate a unique API context from "/route-path-matching-4" and store it as "apiContext4"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName4}                    |
      | spec.displayName       | Route-Root-NoSlash-API            |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext4}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext4}/v1.0" until status 200

    When I send a "GET" request to "${CTX:apiContext4}/v1.0"
    Then the response should be successful

    When I delete the API "${CTX:apiName4}"
    Then the response should be successful


  Scenario: Wildcard /* does not match a sibling context prefix
    Given I generate a unique value from "route-path-matching-5" and store it as "apiName5"
    And I generate a unique API context from "/route-path-matching-5" and store it as "apiContext5"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName5}                    |
      | spec.displayName       | Route-Wildcard-Boundary-API       |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext5}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/*"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext5}/v1.0/data" until status 200

    When I send a "GET" request to "${CTX:apiContext5}/v1.0/data"
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext5}/v1.0"
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext5}/v1.0beta/data"
    Then the response status code should be 404

    When I delete the API "${CTX:apiName5}"
    Then the response should be successful


  Scenario: Exact path matches both with and without trailing slash
    Given I generate a unique value from "route-path-matching-6" and store it as "apiName6"
    And I generate a unique API context from "/route-path-matching-6" and store it as "apiContext6"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName6}                    |
      | spec.displayName       | Route-Exact-Slash-API             |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext6}/$version        |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/weather"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext6}/v1.0/weather" until status 200

    When I send a "GET" request to "${CTX:apiContext6}/v1.0/weather"
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext6}/v1.0/weather/"
    Then the response should be successful

    When I delete the API "${CTX:apiName6}"
    Then the response should be successful


  Scenario: Exact path preserves trailing slash to upstream
    Given I generate a unique value from "route-path-matching-7" and store it as "apiName7"
    And I generate a unique API context from "/route-path-matching-7" and store it as "apiContext7"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName7}                    |
      | spec.displayName       | Route-Exact-Upstream-Slash-API    |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiContext7}/$version        |
      | spec.upstream.main.url | http://testbench:3002/anything     |
      | spec.operations        | [{"method":"GET","path":"/weather"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext7}/v1.0/weather" until status 200

    When I send a "GET" request to "${CTX:apiContext7}/v1.0/weather"
    Then the response status code should be 200
    And the JSON response field "url" should be "/anything/weather"

    When I send a "GET" request to "${CTX:apiContext7}/v1.0/weather/"
    Then the response status code should be 200
    And the JSON response field "url" should be "/anything/weather/"

    When I delete the API "${CTX:apiName7}"
    Then the response should be successful


  Scenario: Wildcard /foo/* preserves the matched prefix (and method) on the upstream path
    Given I generate a unique value from "route-path-matching-8" and store it as "apiName8"
    And I generate a unique API context from "/route-path-matching-8" and store it as "apiContext8"


    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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
