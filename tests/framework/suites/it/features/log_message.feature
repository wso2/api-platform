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

@log-message
Feature: Log message policy
  As an API operator
  I want configured request and response data to be written to traffic logs
  So that API traffic can be diagnosed without changing the response

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "log-message-api" and store it as "apiName"
    And I generate a unique value from "log-message-display" and store it as "apiDisplayName"
    And I generate a unique API version from "log-message" and store it as "apiVersion"
    And I generate a unique API context from "/log-message" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | name                  | ${CTX:apiName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | spec.displayName      | ${CTX:apiDisplayName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | spec.version          | ${CTX:apiVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | spec.context          | ${CTX:apiContext}/$version                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | spec.upstream.main.url | http://testbench:3002                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | spec.operations       | [{"method":"POST","path":"/request-payload-headers","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true,"headers":true}}}]},{"method":"GET","path":"/response-payload-headers","policies":[{"name":"log-message","version":"v1","params":{"response":{"payload":true,"headers":true}}}]},{"method":"POST","path":"/request-response","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true,"headers":true},"response":{"payload":true,"headers":true}}}]},{"method":"GET","path":"/request-headers","policies":[{"name":"log-message","version":"v1","params":{"request":{"headers":true,"payload":false}}}]},{"method":"GET","path":"/response-headers","policies":[{"name":"log-message","version":"v1","params":{"response":{"headers":true,"payload":false}}}]},{"method":"POST","path":"/request-payload","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true,"headers":false}}}]},{"method":"GET","path":"/response-payload","policies":[{"name":"log-message","version":"v1","params":{"response":{"payload":true,"headers":false}}}]},{"method":"GET","path":"/request-exclusions","policies":[{"name":"log-message","version":"v1","params":{"request":{"headers":true,"excludeHeaders":["X-Excluded-Request"]}}}]},{"method":"GET","path":"/response-exclusions","policies":[{"name":"log-message","version":"v1","params":{"response":{"headers":true,"excludeHeaders":["Content-Type"]}}}]},{"method":"GET","path":"/combined","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Custom-Header","value":"CustomValue"} ]}}},{"name":"log-message","version":"v1","params":{"request":{"headers":true,"payload":false}}}]},{"method":"GET","path":"/methods-get","policies":[{"name":"log-message","version":"v1","params":{"request":{"headers":true}}}]},{"method":"POST","path":"/methods-post","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true,"headers":true}}}]},{"method":"PUT","path":"/methods-put","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true},"response":{"payload":true}}}]},{"method":"DELETE","path":"/methods-delete","policies":[{"name":"log-message","version":"v1","params":{"response":{"headers":true}}}]},{"method":"POST","path":"/large-payload","policies":[{"name":"log-message","version":"v1","params":{"request":{"payload":true},"response":{"payload":true}}}]}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/request-headers" until status 200

  Scenario: Log request payload and headers
    Given I set header "X-Log-Request" to "request-payload-headers"
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/request-payload-headers" with body:
      """
      {"marker":"request-payload-headers"}
      """
    Then the response status code should be 200
    And the response body should contain "request-payload-headers"
    And the "gateway-runtime" service logs should contain "request-payload-headers"

  Scenario: Log response payload and headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-payload-headers"
    Then the response status code should be 200
    And the response should be valid JSON
    And the "gateway-runtime" service logs should contain "response-payload-headers"

  Scenario: Log both request and response
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/request-response" with body:
      """
      {"marker":"request-response"}
      """
    Then the response status code should be 200
    And the response body should contain "request-response"
    And the "gateway-runtime" service logs should contain "request-response"

  Scenario: Log only request headers without payload
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/request-headers"
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "request-headers"

  Scenario: Log only response headers without payload
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-headers"
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "response-headers"

  Scenario: Log only request payload without headers
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/request-payload" with body:
      """
      {"marker":"request-payload"}
      """
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "request-payload"

  Scenario: Log only response payload without headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-payload"
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "response-payload"

  Scenario: Log request headers with exclusions
    And I set header "X-Excluded-Request" to "excluded-request"
    And I set header "X-Included-Request" to "included-request"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/request-exclusions"
    Then the response status code should be 200
    And the "gateway-runtime" service log event containing "included-request" should not contain "excluded-request"

  Scenario: Log response headers with exclusions
    And I set header "X-Log-Response" to "response-exclusions"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-exclusions"
    Then the response status code should be 200
    And the "gateway-runtime" service log event containing "response-exclusions" should not contain "Content-Type"

  Scenario: Log message policy combined with set-headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/combined"
    Then the response status code should be 200
    And the response should contain echoed header "x-custom-header" with value "CustomValue"
    And the "gateway-runtime" service logs should contain "CustomValue"

  Scenario: Log message supports GET
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/methods-get"
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "methods-get"

  Scenario: Log message supports POST
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/methods-post" with body:
      """
      {"marker":"methods-post"}
      """
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "methods-post"

  Scenario: Log message supports PUT
    When I send a "PUT" request to "${CTX:apiContext}/${CTX:apiVersion}/methods-put" with body:
      """
      {"marker":"methods-put"}
      """
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "methods-put"

  Scenario: Log message supports DELETE
    When I send a "DELETE" request to "${CTX:apiContext}/${CTX:apiVersion}/methods-delete"
    Then the response status code should be 200
    And the "gateway-runtime" service logs should contain "methods-delete"

  Scenario: Log message handles a large request payload
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/large-payload" with body:
      """
      {"marker":"large-payload","items":[{"id":1,"name":"Item One"},{"id":2,"name":"Item Two"},{"id":3,"name":"Item Three"}]}
      """
    Then the response status code should be 200
    And the response body should contain "Item One"
    And the response body should contain "Item Three"
    And the "gateway-runtime" service logs should contain "large-payload"
