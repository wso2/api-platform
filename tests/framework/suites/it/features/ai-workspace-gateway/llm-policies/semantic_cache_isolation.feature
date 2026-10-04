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
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@semantic-cache-isolation
Feature: Semantic cache caller isolation
  As an API developer
  I want cached responses isolated between authenticated callers
  So that one caller cannot receive another caller's response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A different authenticated caller does not receive another caller's cached response
    Given I generate a unique value from "sc-isolation" and store it as "apiName"
    And I generate a unique API version from "sc-isolation" and store it as "apiVersion"
    And I generate a unique API context from "/sc-isolation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys" with body:
      """
      {"name":"isolation-caller-a"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "callerKey"
    And I set header "API-Key" to "${CTX:callerKey}"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"isolation test prompt about coral reefs"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until header "X-Cache-Status" is "HIT" with body:
      """
      {"prompt":"isolation test prompt about coral reefs"}
      """

    Given I authenticate using basic auth as "admin"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys" with body:
      """
      {"name":"isolation-caller-b"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "callerKey"
    And I set header "API-Key" to "${CTX:callerKey}"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"isolation test prompt about coral reefs"}
      """
    Then the response header "X-Cache-Status" should not exist

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404
