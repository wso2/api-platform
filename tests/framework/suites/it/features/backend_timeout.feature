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

@backend-route-timeout @resilience
Feature: Backend route timeouts
  As an API developer
  I want API and operation timeout settings to control slow upstream requests
  So that the gateway applies the configured timeout contract

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: API-level resilience timeout terminates a slow backend
    Given I generate a unique value from "api-resilience-timeout" and store it as "apiName"
    And I generate a unique value from "api-resilience-timeout-display" and store it as "apiDisplayName"
    And I generate a unique API version from "api-resilience-timeout" and store it as "apiVersion"
    And I generate a unique API context from "/api-resilience-timeout" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName}                   |
      | spec.displayName            | ${CTX:apiDisplayName}             |
      | spec.version                | ${CTX:apiVersion}                 |
      | spec.context                | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.resilience.timeout     | 2s                                |
      | spec.operations             | [{"method":"GET","path":"/get"},{"method":"GET","path":"/delay/5"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Operation-level resilience timeout overrides the API timeout
    Given I generate a unique value from "operation-resilience-timeout" and store it as "apiName"
    And I generate a unique value from "operation-resilience-timeout-display" and store it as "apiDisplayName"
    And I generate a unique API version from "operation-resilience-timeout" and store it as "apiVersion"
    And I generate a unique API context from "/operation-resilience-timeout" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName}                   |
      | spec.displayName            | ${CTX:apiDisplayName}             |
      | spec.version                | ${CTX:apiVersion}                 |
      | spec.context                | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.resilience.timeout     | 10s                               |
      | spec.operations             | [{"method":"GET","path":"/get"},{"method":"GET","path":"/delay/5","resilience":{"timeout":"2s"}}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Missing resilience configuration uses the gateway default timeout
    Given I generate a unique value from "default-resilience-timeout" and store it as "apiName"
    And I generate a unique value from "default-resilience-timeout-display" and store it as "apiDisplayName"
    And I generate a unique API version from "default-resilience-timeout" and store it as "apiVersion"
    And I generate a unique API context from "/default-resilience-timeout" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:apiName}                   |
      | spec.displayName            | ${CTX:apiDisplayName}             |
      | spec.version                | ${CTX:apiVersion}                 |
      | spec.context                | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url      | http://testbench:3000             |
      | spec.operations             | [{"method":"GET","path":"/get"},{"method":"GET","path":"/delay/2"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/delay/2"
    Then the response status code should be 200
    And the response should be valid JSON
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
