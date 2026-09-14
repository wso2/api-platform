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

@interceptor-service
Feature: Interceptor service policies
  As an API developer
  I want interceptor policies to modify requests and responses
  So that I can enforce interception behavior at the gateway

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request interceptor mutates the path, headers, and body
    Given I generate a unique value from "interceptor-request-api" and store it as "apiName"
    And I generate a unique value from "interceptor-request-display" and store it as "apiDisplayName"
    And I generate a unique API version from "interceptor-request" and store it as "apiVersion"
    And I generate a unique API context from "/interceptor-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:apiName}                   |
      | spec.displayName      | ${CTX:apiDisplayName}             |
      | spec.version          | ${CTX:apiVersion}                 |
      | spec.context          | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations       | [{"method":"POST","path":"/mutate","policies":[{"name":"interceptor-service","version":"v1","params":{"endpoint":"http://testbench:3003","request":{"includeRequestHeaders":true,"includeRequestBody":true,"passthroughOnError":false}}}]}] |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/mutate" until status 200 with body:
      """
      {"client":"payload"}
      """
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/mutate" with body:
      """
      {"client":"payload"}
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "path" should contain "/anything/intercepted"
    And the JSON response field "headers.X-Interceptor-Request[0]" should be "true"
    And the JSON response field "body" should contain "mutated-by-interceptor"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request interceptor returns a direct response
    Given I generate a unique value from "interceptor-direct-api" and store it as "apiName"
    And I generate a unique value from "interceptor-direct-display" and store it as "apiDisplayName"
    And I generate a unique API version from "interceptor-direct" and store it as "apiVersion"
    And I generate a unique API context from "/interceptor-direct" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:apiName}                   |
      | spec.displayName      | ${CTX:apiDisplayName}             |
      | spec.version          | ${CTX:apiVersion}                 |
      | spec.context          | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations       | [{"method":"GET","path":"/block","policies":[{"name":"interceptor-service","version":"v1","params":{"endpoint":"http://testbench:3003","request":{"includeRequestHeaders":true,"includeRequestBody":false,"passthroughOnError":false}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/block" until status 403
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/block"
    Then the response status code should be 403
    And the response header "X-Interceptor-Decision" should be "blocked"
    And the response body should contain "blocked by interceptor"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Response interceptor rewrites the status, headers, and body
    Given I generate a unique value from "interceptor-response-api" and store it as "apiName"
    And I generate a unique value from "interceptor-response-display" and store it as "apiDisplayName"
    And I generate a unique API version from "interceptor-response" and store it as "apiVersion"
    And I generate a unique API context from "/interceptor-response" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:apiName}                   |
      | spec.displayName      | ${CTX:apiDisplayName}             |
      | spec.version          | ${CTX:apiVersion}                 |
      | spec.context          | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations       | [{"method":"GET","path":"/response-rewrite","policies":[{"name":"interceptor-service","version":"v1","params":{"endpoint":"http://testbench:3003","request":{"includeRequestHeaders":true,"includeRequestBody":false,"passthroughOnError":false},"response":{"includeRequestHeaders":true,"includeRequestBody":false,"includeResponseHeaders":true,"includeResponseBody":true,"passthroughOnError":false}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-rewrite" until status 202
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-rewrite"
    Then the response status code should be 202
    And the response header "X-Interceptor-Response" should be "true"
    And the response header "X-Interceptor-Trace" should be "request-phase"
    And the response body should contain "response-overridden"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
