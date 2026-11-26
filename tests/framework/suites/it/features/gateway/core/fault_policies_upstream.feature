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

# Fault policies on a gateway with policy_engine.fault_policies.handle_upstream_faults on: backend
# error statuses and router failures run the API's fault policies instead of its response
# policies.
@FAULT @fault-policies-upstream
Feature: Fault policies for backend and router errors
  As an API developer
  I want backend and router errors to reach my fault policies
  So that I can shape and act on every failure an API returns

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @FAULT-013
  Scenario: A backend error runs the API's fault policies
    Given I generate a unique value from "fault-upstream-backend-api" and store it as "apiName"
    And I generate a unique API context from "/fault-upstream-backend" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Upstream-Backend    |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-fault-handled","value":"yes"},{"name":"x-error-source","value":"${Source}"}]}}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    When I send a "GET" request to "${CTX:apiContext}/data" until status 200
    Then the response header "x-fault-handled" should not exist
    And the JSON response field "path" should be "/data"
    When I send a "GET" request to "${CTX:apiContext}/data?statusCode=500" until status 500
    Then the response header "x-fault-handled" should be "yes"
    And the response header "x-error-source" should be "backend"
    And the response should be valid JSON
    And the JSON response field "path" should not exist

  @FAULT-014
  Scenario: An unreachable upstream runs the API's fault policies
    Given I generate a unique value from "fault-upstream-router-api" and store it as "apiName"
    And I generate a unique API context from "/fault-upstream-router" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Upstream-Router     |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3099     |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"},{"name":"x-error-source","value":"${Source}"}]}}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    When I send a "GET" request to "${CTX:apiContext}/data" until status 503
    Then the response header "x-error-code" should match pattern "^(101503|303001)$"
    And the response header "x-error-source" should be "router"
    And the JSON response field "type" should be "upstream"

  @FAULT-015
  Scenario: A backend error runs the fault policies instead of the response policies
    Given I generate a unique value from "fault-upstream-response-api" and store it as "apiName"
    And I generate a unique API context from "/fault-upstream-response" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Upstream-Response   |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.policies          | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"x-response-policy","value":"ran"}]}}}] |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-fault-handled","value":"yes"}]}}] |
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext}/data" until status 200
    Then the response header "x-response-policy" should be "ran"
    And the response header "x-fault-handled" should not exist
    When I send a "GET" request to "${CTX:apiContext}/data?statusCode=502" until status 502
    Then the response header "x-fault-handled" should be "yes"
    And the response header "x-response-policy" should not exist

  @FAULT-016
  Scenario: A fault policy's execution condition separates gateway and backend failures
    Given I generate a unique value from "fault-upstream-source-api" and store it as "apiName"
    And I generate a unique API context from "/fault-upstream-source" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Upstream-Source     |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/data"},{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","executionCondition":"fault.Source == \"gateway\"","params":{"preset":"canonical","headersToSet":[{"name":"x-gateway-fault","value":"${Code}"}]}},{"name":"error-response-formatter","version":"v1","executionCondition":"fault.Source == \"backend\"","params":{"preset":"canonical","headersToSet":[{"name":"x-backend-fault","value":"${Status}"}]}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 422 with body:
      """
      {"message":"hello!"}
      """
    Then the response header "x-gateway-fault" should be "906205"
    And the response header "x-backend-fault" should not exist
    When I send a "GET" request to "${CTX:apiContext}/data?statusCode=500" until status 500
    Then the response header "x-backend-fault" should be "500"
    And the response header "x-gateway-fault" should not exist
