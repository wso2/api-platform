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

@request-id-response-disabled
Feature: Request ID not returned when disabled
  As a gateway operator
  I want to turn off returning the request ID in responses
  So that responses keep their original headers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A request rejected by the policy engine does not carry a request ID
    Given I generate a unique resource name from "reqid-off-401-api" and store it as "apiName"
    And I generate a unique API version from "reqid-off-401-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-off-401" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                  |
      | name                   | ${CTX:apiName}                                                                             |
      | spec.displayName       | Reqid-Off-401-Api                                                                          |
      | spec.version           | ${CTX:apiVersion}                                                                          |
      | spec.context           | ${CTX:apiContext}/$version                                                                 |
      | spec.upstream.main.url | http://testbench:3002                                                                      |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}]        |
      | spec.operations        | [{"method":"GET","path":"/probe"}]                                                         |
    Then the response status code should be 201
    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 401

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 401
    And the response header "x-request-id" should not exist

  Scenario: A request to an unknown route does not carry a request ID
    Given I generate a unique API context from "/reqid-off-missing" and store it as "missingContext"
    When I clear all headers
    And I send a "GET" request to "${CTX:missingContext}/nothing"
    Then the response status code should be 404
    And the response header "x-request-id" should not exist

  Scenario: A gateway error for an unreachable upstream does not carry a request ID
    Given I generate a unique resource name from "reqid-off-503-api" and store it as "apiName"
    And I generate a unique API version from "reqid-off-503-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-off-503" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                   |
      | name                   | ${CTX:apiName}                              |
      | spec.displayName       | Reqid-Off-503-Api                           |
      | spec.version           | ${CTX:apiVersion}                           |
      | spec.context           | ${CTX:apiContext}/$version                  |
      | spec.upstream.main.url | http://no-such-backend.invalid:3000         |
      | spec.operations        | [{"method":"GET","path":"/probe"}]          |
    Then the response status code should be 201
    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 503

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 503
    And the response header "x-request-id" should not exist

  Scenario: An X-Request-ID header set by a policy is left unchanged
    Given I generate a unique resource name from "reqid-off-respond-api" and store it as "apiName"
    And I generate a unique API version from "reqid-off-respond-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-off-respond" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                  |
      | name                   | ${CTX:apiName}                                                                                                                             |
      | spec.displayName       | Reqid-Off-Respond-Api                                                                                                                      |
      | spec.version           | ${CTX:apiVersion}                                                                                                                          |
      | spec.context           | ${CTX:apiContext}/$version                                                                                                                 |
      | spec.upstream.main.url | http://testbench:3002                                                                                                                      |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"headers":[{"name":"X-Request-ID","value":"policy-set-value"}]}}]}] |
    Then the response status code should be 201
    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response header "X-Request-ID" should be "policy-set-value"
