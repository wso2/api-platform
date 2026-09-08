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

@api-management
Feature: API management
  As an API administrator
  I want to manage APIs through the management API
  So that API state is persisted and available to clients

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: List all APIs when no APIs exist
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List APIs after deploying one
    Given I generate a unique value from "management-api-1" and store it as "apiName1"
    And I generate a unique API context from "/management-1" and store it as "apiContext1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName1} |
      | spec | {"displayName":"Test-List-API","version":"v1.0","context":"${CTX:apiContext1}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the API "${CTX:apiName1}"
    Then the response should be successful

  Scenario: List APIs with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Get API by non-existent ID returns 404
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/non-existent-api-id-12345"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get API by invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/invalid@id#format"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Get API by existing ID returns API details
    Given I generate a unique value from "management-api-2" and store it as "apiName2"
    And I generate a unique API context from "/management-2" and store it as "apiContext2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName2} |
      | spec | {"displayName":"Get-By-ID-Test-API","version":"v1.0","context":"${CTX:apiContext2}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName2}"
    Then the API retrieval response should describe API "${CTX:apiName2}" at context "${CTX:apiContext2}"
    When I delete the API "${CTX:apiName2}"
    Then the response should be successful

  Scenario: Delete non-existent API returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/non-existent-api-to-delete"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Delete existing API successfully
    Given I generate a unique value from "management-api-3" and store it as "apiName3"
    And I generate a unique API context from "/management-3" and store it as "apiContext3"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName3} |
      | spec | {"displayName":"Delete-Test-Api","version":"v1.0","context":"${CTX:apiContext3}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I delete the API "${CTX:apiName3}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName3}"
    Then the response status should be 404

  Scenario: Update non-existent API returns 404
    Given I generate a unique value from "management-api-4" and store it as "apiName4"
    And I generate a unique API context from "/management-4" and store it as "apiContext4"
    When I update API "${CTX:apiName4}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName4} |
      | spec | {"displayName":"Non-Existent-Api-To-Update","version":"v2.0","context":"${CTX:apiContext4}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/updated"}]} |
    Then the response status should be 404

  Scenario: Update existing API successfully
    Given I generate a unique value from "management-api-5" and store it as "apiName5"
    And I generate a unique API context from "/management-5" and store it as "apiContext5"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName5} |
      | spec | {"displayName":"Update-Test-Api","version":"v1.0","context":"${CTX:apiContext5}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/original"}]} |
    Then the response should be successful
    When I update API "${CTX:apiName5}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName5} |
      | spec | {"displayName":"Update-Test-Api","version":"v1.1","context":"${CTX:apiContext5}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/updated"},{"method":"POST","path":"/updated"}]} |
    Then the response should be successful
    And the response should be valid JSON
    And the API update response should indicate successful deployment
    When I delete the API "${CTX:apiName5}"
    Then the response should be successful

  Scenario: Create API with minimal required fields
    Given I generate a unique value from "management-api-7" and store it as "apiName7"
    And I generate a unique API context from "/management-7" and store it as "apiContext7"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName7} |
      | spec | {"displayName":"Minimal-Api","version":"v1.0","context":"${CTX:apiContext7}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    When I delete the API "${CTX:apiName7}"
    Then the response should be successful

  Scenario: Create API with multiple operations
    Given I generate a unique value from "management-api-8" and store it as "apiName8"
    And I generate a unique API context from "/management-8" and store it as "apiContext8"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName8} |
      | spec | {"displayName":"Multi-Operation-Api","version":"v1.0","context":"${CTX:apiContext8}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/users"},{"method":"POST","path":"/users"},{"method":"GET","path":"/users/{id}"},{"method":"PUT","path":"/users/{id}"},{"method":"DELETE","path":"/users/{id}"}]} |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    When I delete the API "${CTX:apiName8}"
    Then the response should be successful

  Scenario: Create API with displayName
    Given I generate a unique value from "management-api-9" and store it as "apiName9"
    And I generate a unique API context from "/management-9" and store it as "apiContext9"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName9} |
      | spec | {"displayName":"My Display Name API","version":"v1.0","context":"${CTX:apiContext9}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:apiName9}"
    Then the response should be successful

  Scenario: Create API then verify it appears in list
    Given I generate a unique value from "management-api-10" and store it as "apiName10"
    And I generate a unique API context from "/management-10" and store it as "apiContext10"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName10} |
      | spec | {"displayName":"List-Verification-Api","version":"v1.0","context":"${CTX:apiContext10}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis"
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:apiName10}"
    Then the response should be successful

  Scenario: Create API with wildcard path
    Given I generate a unique value from "management-api-11" and store it as "apiName11"
    And I generate a unique API context from "/management-11" and store it as "apiContext11"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName11} |
      | spec | {"displayName":"Wildcard-Path-Api","version":"v1.0","context":"${CTX:apiContext11}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/*"}]} |
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:apiName11}"
    Then the response should be successful

  Scenario: Create multiple APIs with different contexts
    Given I generate a unique value from "management-api-12" and store it as "apiName12"
    And I generate a unique API context from "/management-12" and store it as "apiContext12"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName12} |
      | spec | {"displayName":"First-Api","version":"v1.0","context":"${CTX:apiContext12}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    Given I generate a unique value from "management-api-13" and store it as "apiName13"
    And I generate a unique API context from "/management-13" and store it as "apiContext13"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName13} |
      | spec | {"displayName":"Second-Api","version":"v1.0","context":"${CTX:apiContext13}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I delete the API "${CTX:apiName12}"
    Then the response should be successful
    When I delete the API "${CTX:apiName13}"
    Then the response should be successful

  Scenario: Create API with missing context returns error
    Given I generate a unique value from "management-api-14" and store it as "apiName14"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName14} |
      | spec | {"displayName":"Missing-Context-Api","version":"v1.0","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create API with missing version returns error
    Given I generate a unique value from "management-api-15" and store it as "apiName15"
    And I generate a unique API context from "/management-15" and store it as "apiContext15"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName15} |
      | spec | {"displayName":"Missing-Version-Api","context":"${CTX:apiContext15}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create API with missing upstream returns error
    Given I generate a unique value from "management-api-16" and store it as "apiName16"
    And I generate a unique API context from "/management-16" and store it as "apiContext16"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName16} |
      | spec | {"displayName":"Missing-Upstream-Api","version":"v1.0","context":"${CTX:apiContext16}","operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create API with empty operations returns error
    Given I generate a unique value from "management-api-17" and store it as "apiName17"
    And I generate a unique API context from "/management-17" and store it as "apiContext17"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName17} |
      | spec | {"displayName":"Empty-Ops-Api","version":"v1.0","context":"${CTX:apiContext17}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[]} |
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Create API with invalid labels (spaces in keys) should fail
    Given I generate a unique value from "management-api-18" and store it as "apiName18"
    And I generate a unique API context from "/management-18" and store it as "apiContext18"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName18} |
      | spec | {"displayName":"Invalid-Labels-Api","version":"v1.0","context":"${CTX:apiContext18}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
      | metadata.labels | {"Invalid Key":"value"} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create API with labels and verify they are stored
    Given I generate a unique value from "management-api-19" and store it as "apiName19"
    And I generate a unique API context from "/management-19" and store it as "apiContext19"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName19} |
      | metadata.labels | {"environment":"production","team":"api-team","version":"v1"} |
      | spec | {"displayName":"Labeled-API","version":"v1.0","context":"${CTX:apiContext19}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    Then the response should be successful
    And the response should be valid JSON
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName19}"
    Then the response should be successful
    And the response body should contain "environment"
    And the response body should contain "production"
    When I delete the API "${CTX:apiName19}"
    Then the response should be successful

  Scenario: Update API with handle mismatch returns error
    Given I generate a unique value from "management-api-20" and store it as "apiName20"
    And I generate a unique value from "management-api-20-update" and store it as "updateApiName20"
    And I generate a unique API context from "/management-20" and store it as "apiContext20"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName20} |
      | spec | {"displayName":"Handle-Mismatch-Api","version":"v1.0","context":"${CTX:apiContext20}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I update API "${CTX:apiName20}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:updateApiName20} |
      | spec | {"displayName":"Handle-Mismatch-Api","version":"v1.0","context":"${CTX:apiContext20}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "mismatch"
    When I delete the API "${CTX:apiName20}"
    Then the response should be successful

  Scenario: Update API with validation errors returns error
    Given I generate a unique value from "management-api-22" and store it as "apiName22"
    And I generate a unique API context from "/management-22" and store it as "apiContext22"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName22} |
      | spec | {"displayName":"Update-Validation-Api","version":"v1.0","context":"${CTX:apiContext22}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I update API "${CTX:apiName22}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName22} |
      | spec | {"displayName":"Update-Validation-Api","version":"v1.0","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    When I delete the API "${CTX:apiName22}"
    Then the response should be successful

  Scenario: Create API with invalid JSON body returns error
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis" with body:
      """
      { invalid json content here
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create API with sandbox upstream
    Given I generate a unique value from "management-api-24" and store it as "apiName24"
    And I generate a unique API context from "/management-24" and store it as "apiContext24"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName24} |
      | spec | {"displayName":"Sandbox-Api","version":"v1.0","context":"${CTX:apiContext24}","upstream":{"main":{"url":"http://testbench:3000"},"sandbox":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    When I delete the API "${CTX:apiName24}"
    Then the response should be successful

  Scenario: Create API with path parameters
    Given I generate a unique value from "management-api-25" and store it as "apiName25"
    And I generate a unique API context from "/management-25" and store it as "apiContext25"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName25} |
      | spec | {"displayName":"Path-Params-Api","version":"v1.0","context":"${CTX:apiContext25}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/users/{userId}"},{"method":"GET","path":"/users/{userId}/orders/{orderId}"},{"method":"DELETE","path":"/users/{userId}/orders/{orderId}"}]} |
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:apiName25}"
    Then the response should be successful

  Scenario: Update API with invalid JSON body returns error
    Given I generate a unique value from "management-api-26" and store it as "apiName26"
    And I generate a unique API context from "/management-26" and store it as "apiContext26"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName26} |
      | spec | {"displayName":"Update-Invalid-Json-Api","version":"v1.0","context":"${CTX:apiContext26}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I send a "PUT" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName26}" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    When I delete the API "${CTX:apiName26}"
    Then the response should be successful

  Scenario: Update API version while keeping same context
    Given I generate a unique value from "management-api-27" and store it as "apiName27"
    And I generate a unique API context from "/management-27" and store it as "apiContext27"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName27} |
      | spec | {"displayName":"Update-Version-Api","version":"v1.0","context":"${CTX:apiContext27}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I update API "${CTX:apiName27}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName27} |
      | spec | {"displayName":"Update-Version-Api","version":"v2.0","context":"${CTX:apiContext27}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    And the response should be valid JSON
    When I get the API "${CTX:apiName27}"
    Then the response should be successful
    And the response body should contain "v2.0"
    When I delete the API "${CTX:apiName27}"
    Then the response should be successful

  Scenario: Delete API with invalid ID format returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/invalid@id#format!!"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Delete same API twice (idempotent check)
    Given I generate a unique value from "management-api-29" and store it as "apiName29"
    And I generate a unique API context from "/management-29" and store it as "apiContext29"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion | gateway.api-platform.wso2.com/v1 |
      | name | ${CTX:apiName29} |
      | spec | {"displayName":"Delete-Twice-Api","version":"v1.0","context":"${CTX:apiContext29}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/test"}]} |
    Then the response should be successful
    When I delete the API "${CTX:apiName29}"
    Then the response should be successful
    When I delete the API "${CTX:apiName29}"
    Then the response status should be 404
