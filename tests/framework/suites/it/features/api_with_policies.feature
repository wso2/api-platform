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

@api-with-policies
Feature: API configuration with policies
  As an API developer
  I want to deploy APIs with various policy configurations
  So that I can test policy integration and handler coverage

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy API without any policies
    Given I generate a unique value from "no-policy-api" and store it as "resourceName1_1"
    Given I generate a unique API context from "/no-policy" and store it as "resourceContext1_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName1_1}             |
      | spec.displayName       | No-Policy-Api                       |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext1_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    When I delete the API "${CTX:resourceName1_1}"
    Then the response should be successful

  Scenario: Deploy API with operation-level policy
    Given I generate a unique value from "operation-policy-api" and store it as "resourceName2_1"
    Given I generate a unique API context from "/op-policy" and store it as "resourceContext2_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName2_1}             |
      | spec.displayName       | Operation-Policy-Api                |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext2_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"cors","version":"v1","params":{"allowedOrigins":["*"],"allowedMethods":["GET","POST"],"allowedHeaders":["*"]}}]}] |
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:resourceName2_1}"
    Then the response should be successful

  Scenario: Deploy API with API-level policy
    Given I generate a unique value from "api-level-policy-api" and store it as "resourceName3_1"
    Given I generate a unique API context from "/api-policy" and store it as "resourceContext3_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName3_1}             |
      | spec.displayName       | Api-Level-Policy-Api                |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext3_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://testbench:3000"],"allowedMethods":["GET"],"allowedHeaders":["Content-Type"]}}] |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:resourceName3_1}"
    Then the response should be successful

  Scenario: Update API to add policies
    Given I generate a unique value from "update-add-policy-api" and store it as "resourceName4_1"
    Given I generate a unique API context from "/update-policy" and store it as "resourceContext4_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName4_1}             |
      | spec.displayName       | Update-Add-Policy-Api              |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext4_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I update API "${CTX:resourceName4_1}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName4_1}             |
      | spec.displayName       | Update-Add-Policy-Api              |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext4_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["*"],"allowedMethods":["GET"],"allowedHeaders":["*"]}}] |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I delete the API "${CTX:resourceName4_1}"
    Then the response should be successful

  Scenario: Update API to remove policies
    Given I generate a unique value from "update-remove-policy-api" and store it as "resourceName5_1"
    Given I generate a unique API context from "/update-remove" and store it as "resourceContext5_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName5_1}             |
      | spec.displayName       | Update-Remove-Policy-Api            |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext5_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["*"],"allowedMethods":["GET"],"allowedHeaders":["*"]}}] |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I update API "${CTX:resourceName5_1}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName5_1}             |
      | spec.displayName       | Update-Remove-Policy-Api            |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext5_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I delete the API "${CTX:resourceName5_1}"
    Then the response should be successful

  Scenario: Deploy API with API-level policy using empty version resolves to latest
    Given I generate a unique value from "empty-version-api-level-api" and store it as "resourceName6_1"
    Given I generate a unique API context from "/empty-version-api" and store it as "resourceContext6_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName6_1}             |
      | spec.displayName       | Empty-Version-Api-Level-Api        |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext6_1}           |
      | spec.upstream.main.url | http://testbench:3000/api/v1        |
      | spec.policies          | [{"name":"cors","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET"],"allowedHeaders":["Content-Type"]}}] |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:resourceContext6_1}/us/seattle" until status 200
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:resourceContext6_1}/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:resourceName6_1}"
    Then the response should be successful

  Scenario: Deploy API with operation-level policy using empty version resolves to latest
    Given I generate a unique value from "empty-version-op-level-api" and store it as "resourceName7_1"
    Given I generate a unique API context from "/empty-version-op" and store it as "resourceContext7_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName7_1}             |
      | spec.displayName       | Empty-Version-Op-Level-Api          |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext7_1}           |
      | spec.upstream.main.url | http://testbench:3000/api/v1        |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}","policies":[{"name":"cors","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET"],"allowedHeaders":["Content-Type"]}}]}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:resourceContext7_1}/us/seattle" until status 200
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:resourceContext7_1}/us/seattle"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:resourceName7_1}"
    Then the response should be successful

  Scenario: Deploy API with different HTTP methods
    Given I generate a unique value from "http-methods-api" and store it as "resourceName8_1"
    Given I generate a unique API context from "/methods" and store it as "resourceContext8_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:resourceName8_1}             |
      | spec.displayName       | Http-Methods-Api                    |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:resourceContext8_1}           |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/resource"},{"method":"POST","path":"/resource"},{"method":"PUT","path":"/resource"},{"method":"DELETE","path":"/resource"},{"method":"PATCH","path":"/resource"}] |
    Then the response should be successful
    And the response should be valid JSON
    When I delete the API "${CTX:resourceName8_1}"
    Then the response should be successful
