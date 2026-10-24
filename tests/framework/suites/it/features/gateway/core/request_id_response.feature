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

@request-id-response
Feature: Request ID in responses
  As an API consumer
  I want every response to carry the request's correlation ID
  So that I can quote it when reporting a problem

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A successful response carries the generated request ID and the upstream receives the same ID
    Given I generate a unique resource name from "reqid-ok-api" and store it as "apiName"
    And I generate a unique API version from "reqid-ok-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-ok" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                   |
      | name                   | ${CTX:apiName}                              |
      | spec.displayName       | Reqid-Ok-Api                                |
      | spec.version           | ${CTX:apiVersion}                           |
      | spec.context           | ${CTX:apiContext}/$version                  |
      | spec.upstream.main.url | http://testbench:3002                       |
      | spec.operations        | [{"method":"GET","path":"/probe"}]          |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response header "x-request-id" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
    And I store the response header "x-request-id" as "generatedRequestId"
    And the response should contain echoed header "X-Request-Id" with value "${CTX:generatedRequestId}"

  Scenario: A request ID sent by the client is returned unchanged
    Given I generate a unique resource name from "reqid-client-api" and store it as "apiName"
    And I generate a unique API version from "reqid-client-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-client" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                   |
      | name                   | ${CTX:apiName}                              |
      | spec.displayName       | Reqid-Client-Api                            |
      | spec.version           | ${CTX:apiVersion}                           |
      | spec.context           | ${CTX:apiContext}/$version                  |
      | spec.upstream.main.url | http://testbench:3002                       |
      | spec.operations        | [{"method":"GET","path":"/probe"}]          |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200

    When I clear all headers
    And I set header "x-request-id" to "client-request-id-123"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response header "x-request-id" should be "client-request-id-123"
    And the response should contain echoed header "X-Request-Id" with value "client-request-id-123"

  Scenario: A request rejected by the policy engine still carries the request ID
    Given I generate a unique resource name from "reqid-401-api" and store it as "apiName"
    And I generate a unique API version from "reqid-401-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-401" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                  |
      | name                   | ${CTX:apiName}                                                                             |
      | spec.displayName       | Reqid-401-Api                                                                              |
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
    And the response header "x-request-id" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

    When I set header "x-request-id" to "client-request-id-401"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 401
    And the response header "x-request-id" should be "client-request-id-401"

  Scenario: A request to an unknown route still carries the request ID
    Given I generate a unique API context from "/reqid-missing" and store it as "missingContext"
    When I clear all headers
    And I send a "GET" request to "${CTX:missingContext}/nothing"
    Then the response status code should be 404
    And the response header "x-request-id" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  Scenario: A gateway error for an unreachable upstream still carries the request ID
    Given I generate a unique resource name from "reqid-503-api" and store it as "apiName"
    And I generate a unique API version from "reqid-503-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-503" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                   |
      | name                   | ${CTX:apiName}                              |
      | spec.displayName       | Reqid-503-Api                               |
      | spec.version           | ${CTX:apiVersion}                           |
      | spec.context           | ${CTX:apiContext}/$version                  |
      | spec.upstream.main.url | http://no-such-backend.invalid:3000         |
      | spec.operations        | [{"method":"GET","path":"/probe"}]          |
    Then the response status code should be 201
    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 503

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 503
    And the response header "x-request-id" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"

  Scenario: The gateway request ID replaces an X-Request-ID header set by a policy
    Given I generate a unique resource name from "reqid-respond-api" and store it as "apiName"
    And I generate a unique API version from "reqid-respond-api" and store it as "apiVersion"
    And I generate a unique API context from "/reqid-respond" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                  |
      | name                   | ${CTX:apiName}                                                                                                                             |
      | spec.displayName       | Reqid-Respond-Api                                                                                                                          |
      | spec.version           | ${CTX:apiVersion}                                                                                                                          |
      | spec.context           | ${CTX:apiContext}/$version                                                                                                                 |
      | spec.upstream.main.url | http://testbench:3002                                                                                                                      |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"respond","version":"v1","params":{"statusCode":200,"headers":[{"name":"X-Request-ID","value":"policy-set-value"}]}}]}] |
    Then the response status code should be 201
    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response header "X-Request-ID" should not be "policy-set-value"
    And the response header "X-Request-ID" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
