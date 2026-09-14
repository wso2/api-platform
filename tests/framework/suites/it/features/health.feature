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

@health
Feature: Gateway health
  As a gateway operator
  I want to verify the health of the gateway services
  So that I can confirm the gateway is operational before serving traffic

  Background:
    Given the gateway services are running

  Scenario: Gateway controller admin health endpoint returns a healthy response
    When I send a GET request to the gateway controller admin health endpoint
    Then the response status code should be 200
    And the response should indicate healthy status

  Scenario: Gateway controller admin health endpoint returns valid JSON
    When I send a GET request to the gateway controller admin health endpoint
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Policy engine health endpoint returns a healthy response
    When I send a GET request to the policy engine health endpoint
    Then the response status code should be 200
    And the response should indicate healthy status

  Scenario: Policy engine health endpoint returns valid JSON
    When I send a GET request to the policy engine health endpoint
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Router becomes ready after an API route is deployed
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "health-router-api" and store it as "apiName"
    And I generate a unique value from "health-router-display" and store it as "apiDisplayName"
    And I generate a unique API version from "health-router" and store it as "apiVersion"
    And I generate a unique API context from "/health-router" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | ${CTX:apiDisplayName}              |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"POST","path":"/alerts/active"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    When I send a GET request to the router ready endpoint until status 200
    Then the response status code should be 200
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Gateway services report healthy status after an API route is deployed
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "health-services-api" and store it as "apiName"
    And I generate a unique value from "health-services-display" and store it as "apiDisplayName"
    And I generate a unique API version from "health-services" and store it as "apiVersion"
    And I generate a unique API context from "/health-services" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | ${CTX:apiDisplayName}              |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"},{"method":"GET","path":"/alerts/active"},{"method":"POST","path":"/alerts/active"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    When I check the health of all gateway services
    Then all services should report healthy status
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
