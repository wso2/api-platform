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

@url-guardrail
Feature: URL guardrail policy
  As an API developer
  I want to validate URLs mentioned in requests
  So that I can prevent invalid or unreachable URLs from being processed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request containing an unreachable URL is blocked
    Given I generate a unique value from "ug-http-check" and store it as "apiName"
    And I generate a unique API version from "ug-http-check" and store it as "apiVersion"
    And I generate a unique API context from "/ug-http-check" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://testbench:3000/health
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://nonexistent-host-12345.invalid/test
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "URL_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Request without any URLs is allowed
    Given I generate a unique value from "ug-no-urls" and store it as "apiName"
    And I generate a unique API version from "ug-no-urls" and store it as "apiVersion"
    And I generate a unique API context from "/ug-no-urls" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This is a message with no URLs at all, just plain text.
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Every URL in a multi-URL request must be reachable
    Given I generate a unique value from "ug-multiple" and store it as "apiName"
    And I generate a unique API version from "ug-multiple" and store it as "apiVersion"
    And I generate a unique API context from "/ug-multiple" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check these URLs: http://testbench:3000/health and http://testbench:3000/get
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check http://testbench:3000/health and http://invalid-domain-xyz.invalid/test
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: DNS-only mode accepts a resolvable domain and rejects a non-resolvable one
    Given I generate a unique value from "ug-dns-only" and store it as "apiName"
    And I generate a unique API version from "ug-dns-only" and store it as "apiVersion"
    And I generate a unique API context from "/ug-dns-only" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","onlyDNS":true,"timeout":3000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://testbench:3000/test
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://nonexistent-domain-xyz12345.invalid/test
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath-extracted URL is validated while other fields are ignored
    Given I generate a unique value from "ug-jsonpath" and store it as "apiName"
    And I generate a unique API version from "ug-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/ug-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.url","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "url": "http://testbench:3000/health",
        "other": "http://invalid-domain-xyz.invalid/test"
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "url": "http://nonexistent-host-abc123.invalid/test",
        "other": "http://testbench:3000/health"
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Nested JSONPath-extracted URL is validated
    Given I generate a unique value from "ug-nested" and store it as "apiName"
    And I generate a unique API version from "ug-nested" and store it as "apiVersion"
    And I generate a unique API context from "/ug-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.data.link","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "data": {
          "link": "http://testbench:3000/get",
          "timestamp": "2025-01-01"
        },
        "badUrl": "http://invalid.invalid/test"
      }
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "data": {
          "link": "http://nonexistent-domain-xyz.invalid/test",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction of a missing field is blocked
    Given I generate a unique value from "ug-invalid-path" and store it as "apiName"
    And I generate a unique API version from "ug-invalid-path" and store it as "apiVersion"
    And I generate a unique API context from "/ug-invalid-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"$.nonexistent.field","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      {
        "url": "http://testbench:3000/health"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "URL_GUARDRAIL"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A short custom timeout still allows a reachable URL through
    Given I generate a unique value from "ug-timeout" and store it as "apiName"
    And I generate a unique API version from "ug-timeout" and store it as "apiVersion"
    And I generate a unique API context from "/ug-timeout" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":1000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://testbench:3000/health
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response includes the invalid URLs in its assessment detail
    Given I generate a unique value from "ug-assessment" and store it as "apiName"
    And I generate a unique API version from "ug-assessment" and store it as "apiVersion"
    And I generate a unique API context from "/ug-assessment" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","showAssessment":true,"timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://invalid-domain-xyz123.invalid/test
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"
    And the response body should contain "invalidUrls"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Empty request body has no URLs to reject
    Given I generate a unique value from "ug-empty" and store it as "apiName"
    And I generate a unique API version from "ug-empty" and store it as "apiVersion"
    And I generate a unique API context from "/ug-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Text resembling a URL but using an unsupported scheme is not treated as a URL
    Given I generate a unique value from "ug-malformed" and store it as "apiName"
    And I generate a unique API version from "ug-malformed" and store it as "apiVersion"
    And I generate a unique API context from "/ug-malformed" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      This has text that looks like a URL: htp://wrong-protocol.com
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A request-only URL policy still declares a response check without enforcing it
    Given I generate a unique value from "ug-combined" and store it as "apiName"
    And I generate a unique API version from "ug-combined" and store it as "apiVersion"
    And I generate a unique API context from "/ug-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000},"response":{"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://testbench:3000/health
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://invalid-domain-xyz.invalid/test
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A URL with query parameters is validated correctly
    Given I generate a unique value from "ug-special-chars" and store it as "apiName"
    And I generate a unique API version from "ug-special-chars" and store it as "apiVersion"
    And I generate a unique API context from "/ug-special-chars" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://testbench:3000/get?param=value&other=123
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Plain-text content is scanned for URLs the same as JSON content
    Given I generate a unique value from "ug-plaintext" and store it as "apiName"
    And I generate a unique API version from "ug-plaintext" and store it as "apiVersion"
    And I generate a unique API context from "/ug-plaintext" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Please check http://testbench:3000/health for status
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Please check http://nonexistent-host-abc.invalid/health
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Blocked response carries the complete guardrail error contract
    Given I generate a unique value from "ug-error-structure" and store it as "apiName"
    And I generate a unique API version from "ug-error-structure" and store it as "apiVersion"
    And I generate a unique API context from "/ug-error-structure" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"url-guardrail","version":"v1","params":{"request":{"enabled":true,"jsonPath":"","showAssessment":true,"timeout":5000}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      Check this URL: http://invalid-domain-test123.invalid/test
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "URL_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "url-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
