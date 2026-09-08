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

@header-routing
Feature: Header-based route selection
  As an API user
  I want gateway routing behavior to follow the configured rules
  So that requests reach the intended endpoint and response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"


  Scenario: Requests are routed to the operation whose header match they satisfy
    Given I generate a unique value from "header-routing-1" and store it as "apiName1"
    And I generate a unique API context from "/header-routing-1" and store it as "apiContext1"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | name                  | ${CTX:apiName1}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | spec.displayName      | Header-Routing-API                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | spec.version          | v1.0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | spec.context          | ${CTX:apiContext1}/$version                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | spec.upstream.main.url | http://testbench:3000                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | spec.operations       | [{"method":"GET","path":"/ready"},{"match":{"method":"GET","path":{"value":"/pick"},"headers":[{"name":"x-variant","value":"alpha"}]},"policies":[{"name":"respond","version":"v1","params":{"statusCode":201}}]},{"match":{"method":"GET","path":{"value":"/pick"},"headers":[{"name":"x-variant","value":"beta"}]},"policies":[{"name":"respond","version":"v1","params":{"statusCode":202}}]},{"match":{"method":"GET","path":{"value":"/pick"},"headers":[{"name":"x-variant","type":"RegularExpression","value":"^v[0-9]+$"}]},"policies":[{"name":"respond","version":"v1","params":{"statusCode":203}}]},{"method":"GET","path":"/pick","policies":[{"name":"respond","version":"v1","params":{"statusCode":200}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/ready" until status 200

    When I clear all headers
    And I set header "x-variant" to "alpha"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/pick"
    Then the response status code should be 201

    When I clear all headers
    And I set header "x-variant" to "beta"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/pick"
    Then the response status code should be 202

    When I clear all headers
    And I set header "x-variant" to "v12"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/pick"
    Then the response status code should be 203

    When I clear all headers
    And I set header "x-variant" to "does-not-match"
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/pick"
    Then the response status code should be 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext1}/v1.0/pick"
    Then the response status code should be 200


  Scenario: A simple operation and a match operation on the same path coexist by header precedence
    Given I generate a unique value from "header-routing-2" and store it as "apiName2"
    And I generate a unique API context from "/header-routing-2" and store it as "apiContext2"

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                                                                                                                                                                                 |
      | name                  | ${CTX:apiName2}                                                                                                                                                                                                                  |
      | spec.displayName      | Mixed-Form-API                                                                                                                                                                                                                   |
      | spec.version          | v1.0                                                                                                                                                                                                                              |
      | spec.context          | ${CTX:apiContext2}/$version                                                                                                                                                                                                       |
      | spec.upstream.main.url | http://testbench:3000                                                                                                                                                                                                            |
      | spec.operations       | [{"method":"GET","path":"/ready"},{"method":"GET","path":"/via-match","policies":[{"name":"respond","version":"v1","params":{"statusCode":200}}]},{"match":{"method":"GET","path":{"value":"/via-match"},"headers":[{"name":"x-variant","value":"alpha","type":"Exact"}]},"policies":[{"name":"respond","version":"v1","params":{"statusCode":210}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/ready" until status 200

    When I clear all headers
    And I set header "x-variant" to "alpha"
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/via-match"
    Then the response status code should be 210

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext2}/v1.0/via-match"
    Then the response status code should be 200
