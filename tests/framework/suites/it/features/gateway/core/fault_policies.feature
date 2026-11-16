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

# Fault policies on a gateway with the default configuration, where
# policy_engine.fault_policies.handle_upstream_faults is off: a policy's rejection runs the API's
# fault policies, and a backend or router error does not.
@FAULT @fault-policies
Feature: Fault policies
  As an API developer
  I want policies that run only when a request fails
  So that I can shape, log and act on failures without changing the success path

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @FAULT-001
  Scenario: A guardrail rejection runs the API's fault policies
    Given I generate a unique value from "fault-guardrail-api" and store it as "apiName"
    And I generate a unique API context from "/fault-guardrail" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Guardrail-Api       |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"},{"name":"x-error-source","value":"${Source}"}]}}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    Then the JSON response field "path" should be "/check"
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "x-error-code" should be "906205"
    And the response header "x-error-source" should be "gateway"
    And the response should be valid JSON
    And the JSON response field "code" should be "906205"
    And the JSON response field "type" should be "guardrail"

  @FAULT-002
  Scenario: A successful request runs no fault policy
    Given I generate a unique value from "fault-success-api" and store it as "apiName"
    And I generate a unique API context from "/fault-success" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Success-Api         |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"}]}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    Then the response header "x-error-code" should not exist
    And the JSON response field "method" should be "POST"
    And the JSON response field "path" should be "/check"
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "x-error-code" should be "906205"

  @FAULT-003
  Scenario: An authentication rejection runs the API's fault policies
    Given I generate a unique value from "fault-auth-api" and store it as "apiName"
    And I generate a unique API context from "/fault-auth" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Auth-Api            |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/secured","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"}]}}] |
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext}/secured" until status 401
    Then the response header "x-error-code" should be "900902"
    And the response should be valid JSON
    And the JSON response field "code" should be "900902"
    And the JSON response field "type" should be "authentication"

  @FAULT-004
  Scenario: A fault policy's execution condition selects the failures it handles
    Given I generate a unique value from "fault-condition-api" and store it as "apiName"
    And I generate a unique API context from "/fault-condition" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Condition-Api       |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]},{"method":"GET","path":"/secured","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"}]}},{"name":"error-response-formatter","version":"v1","executionCondition":"fault.Type == \"guardrail\"","params":{"preset":"canonical","headersToSet":[{"name":"x-guardrail","value":"${Policy}"}]}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "x-error-code" should be "906205"
    And the response header "x-guardrail" should be "regex-guardrail"
    When I send a "GET" request to "${CTX:apiContext}/secured" until status 401
    Then the response header "x-error-code" should be "900902"
    And the response header "x-guardrail" should not exist

  @FAULT-005
  Scenario: Operation-level fault policies run before the API's fault policies
    Given I generate a unique value from "fault-scope-api" and store it as "apiName"
    And I generate a unique API context from "/fault-scope" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Scope-Api           |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/payments","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}],"faultPolicies":[{"name":"error-response-formatter","version":"v1","params":{"template":{"scope":"operation","code":"${Code}"},"headersToSet":[{"name":"x-operation-fault","value":"handled"},{"name":"x-fault-scope","value":"operation"}]}}]},{"method":"POST","path":"/orders","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-api-fault","value":"handled"},{"name":"x-fault-scope","value":"api"}]}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/payments" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/payments" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "x-operation-fault" should be "handled"
    And the response header "x-api-fault" should be "handled"
    And the response header "x-fault-scope" should be "api"
    And the JSON response field "code" should be "906205"
    And the JSON response field "scope" should not exist
    When I send a "POST" request to "${CTX:apiContext}/orders" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/orders" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "x-api-fault" should be "handled"
    And the response header "x-operation-fault" should not exist

  @FAULT-006
  Scenario Outline: The error response formatter renders a failure in a JSON preset shape
    Given I generate a unique value from "fault-preset-api" and store it as "apiName"
    And I generate a unique API context from "/fault-preset" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Preset-Api          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"<preset>"}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "Content-Type" should contain "application/json"
    And the JSON response field "<field1>" should be "<value1>"
    And the JSON response field "<field2>" should be "<value2>"

    Examples:
      | preset    | field1     | value1  | field2        | value2                                   |
      | canonical | code       | 906205  | type          | guardrail                                |
      | openai    | error.code | 906205  | error.message | Violation of regular expression detected |
      | jsonrpc   | jsonrpc    | 2.0     | error.code    | -32600                                   |

  @FAULT-006
  Scenario Outline: The error response formatter renders a failure in a SOAP preset shape
    Given I generate a unique value from "fault-preset-api" and store it as "apiName"
    And I generate a unique API context from "/fault-preset" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Preset-Api          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"<preset>"}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "Content-Type" should contain "<contentType>"
    And the response body should match pattern "<bodyPattern>"

    Examples:
      | preset | contentType          | bodyPattern                                |
      | soap11 | text/xml             | <faultcode>[a-zA-Z0-9]+:Client</faultcode> |
      | soap12 | application/soap+xml | <[a-zA-Z0-9]+:Value>[a-zA-Z0-9]+:Sender</  |

  @FAULT-007
  Scenario: A fault policy changes the status of the error response
    Given I generate a unique value from "fault-status-api" and store it as "apiName"
    And I generate a unique API context from "/fault-status" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Status-Api          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","statusCode":400}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "906205"

  @FAULT-008
  Scenario: Log Message as a fault policy logs the failure
    Given I generate a unique value from "fault-log-api" and store it as "apiName"
    And I generate a unique API context from "/fault-log" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Log-Api             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"log-message","version":"v1","params":{"fault":{"payload":true,"headers":true}}}] |
    Then the response should be successful
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the "gateway-runtime" service log event containing "${CTX:apiContext}/check" should contain "FAULT"

  @FAULT-009
  Scenario: A backend error does not run fault policies by default
    Given I generate a unique value from "fault-backend-api" and store it as "apiName"
    And I generate a unique API context from "/fault-backend" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Backend-Api         |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/data"},{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"}]}}] |
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext}/data?statusCode=500" until status 500
    Then the response header "x-error-code" should not exist
    And the JSON response field "code" should not exist
    And the JSON response field "method" should be "GET"
    And the JSON response field "path" should be "/data"
    When I send a "POST" request to "${CTX:apiContext}/check" until status 422 with body:
      """
      {"message":"hello!"}
      """
    Then the response header "x-error-code" should be "906205"

  @FAULT-010
  Scenario: An unreachable upstream does not run fault policies by default
    Given I generate a unique value from "fault-unreachable-api" and store it as "apiName"
    And I generate a unique API context from "/fault-unreachable" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Unreachable-Api     |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3099     |
      | spec.operations        | [{"method":"GET","path":"/data"},{"method":"GET","path":"/secured","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}]}] |
      | spec.faultPolicies     | [{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"}]}}] |
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext}/secured" until status 401
    Then the response header "x-error-code" should be "900902"
    When I send a "GET" request to "${CTX:apiContext}/data" until status 503
    Then the response header "x-error-code" should not exist
    And the response body should not contain "101503"

  @FAULT-011 @type:negative
  Scenario: A fault policy that does not exist is rejected at deployment
    Given I generate a unique value from "fault-unknown-api" and store it as "apiName"
    And I generate a unique API context from "/fault-unknown" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Unknown-Api         |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.faultPolicies     | [{"name":"fault-policy-does-not-exist","version":"v1"}] |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "spec.faultPolicies[0]"
    When I get the API "${CTX:apiName}"
    Then the response status code should be 404
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Unknown-Api         |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.faultPolicies     | [{"name":"log-message","version":"v1","params":{"fault":{"payload":false}}}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    When I get the API "${CTX:apiName}"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiName}"

  @FAULT-012
  Scenario: A policy that cannot run as a fault policy is skipped
    Given I generate a unique value from "fault-skipped-api" and store it as "apiName"
    And I generate a unique API context from "/fault-skipped" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | Fault-Skipped-Api         |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:apiContext}         |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"POST","path":"/check","policies":[{"name":"regex-guardrail","version":"v1","params":{"request":{"enabled":true,"regex":"^[A-Za-z0-9 ]+$","jsonPath":"$.message"}}}]}] |
      | spec.faultPolicies     | [{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"x-set-headers-fault","value":"ran"}]}}},{"name":"error-response-formatter","version":"v1","params":{"preset":"canonical","headersToSet":[{"name":"x-error-code","value":"${Code}"}]}}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    When I send a "POST" request to "${CTX:apiContext}/check" until status 200 with body:
      """
      {"message":"hello world"}
      """
    When I send a "POST" request to "${CTX:apiContext}/check" with body:
      """
      {"message":"hello!"}
      """
    Then the response status code should be 422
    And the response header "x-error-code" should be "906205"
    And the response header "x-set-headers-fault" should not exist
    And the "gateway-runtime" service logs should contain "route=POST|${CTX:apiContext}/check|* hint="
