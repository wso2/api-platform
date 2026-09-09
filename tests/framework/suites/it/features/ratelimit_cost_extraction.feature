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

@ratelimit-cost-extraction
Feature: Advanced rate limit dynamic cost extraction
  As an API developer
  I want a request or response to consume a dynamically computed amount of quota
  So that I can rate-limit by a real cost signal instead of a flat per-request count

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Requests without cost extraction each consume exactly one unit
    Given I generate a unique value from "arl-cost-none" and store it as "apiName"
    And I generate a unique API version from "arl-cost-none" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-none" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":4,"duration":"1h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 4 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Cost is extracted from the response body via JSONPath
    Given I generate a unique value from "arl-cost-response-body" and store it as "apiName"
    And I generate a unique API version from "arl-cost-response-body" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-response-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/anything","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"token-quota","limits":[{"limit":100,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_body","jsonPath":"$.json.custom_cost"}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"custom_cost":50}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "50"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"custom_cost":50}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"custom_cost":10}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A response cost that overshoots the remaining quota clamps it to zero
    Given I generate a unique value from "arl-cost-clamp" and store it as "apiName"
    And I generate a unique API version from "arl-cost-clamp" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-clamp" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/anything","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"response-token-quota","limits":[{"limit":20,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_body","jsonPath":"$.json.custom_cost"}],"default":0}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"custom_cost":50}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"custom_cost":1}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Cost is extracted from the request body via JSONPath
    Given I generate a unique value from "arl-cost-request-body" and store it as "apiName"
    And I generate a unique API version from "arl-cost-request-body" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-request-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"token-quota","limits":[{"limit":100,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_body","jsonPath":"$.tokens"}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"tokens":40}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "60"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"tokens":40}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "20"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"tokens":30}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Cost is extracted from a request header
    Given I generate a unique value from "arl-cost-request-header" and store it as "apiName"
    And I generate a unique API version from "arl-cost-request-header" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-request-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"token-quota","limits":[{"limit":100,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_header","key":"X-Token-Cost"}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-Token-Cost" to "40"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "60"

    When I set header "X-Token-Cost" to "40"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "20"

    When I set header "X-Token-Cost" to "30"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A default cost applies when extraction fails
    Given I generate a unique value from "arl-cost-default" and store it as "apiName"
    And I generate a unique API version from "arl-cost-default" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-default" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"token-quota","limits":[{"limit":10,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_body","jsonPath":"$.nonexistent_field"}],"default":5}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"some_other_field":100}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "5"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"another_field":200}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"data":"test"}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: The default cost is used only when every configured source fails
    Given I generate a unique value from "arl-cost-partial-source" and store it as "apiName"
    And I generate a unique API version from "arl-cost-partial-source" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-partial-source" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"partial-source-quota","limits":[{"limit":10,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_header","key":"X-Token-Cost"},{"type":"request_body","jsonPath":"$.missing_field"}],"default":9}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-Token-Cost" to "2"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "8"

    When I set header "X-Token-Cost" to "8"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I set header "X-Token-Cost" to "1"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple cost sources are summed into a single quota
    Given I generate a unique value from "arl-cost-sum" and store it as "apiName"
    And I generate a unique API version from "arl-cost-sum" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-sum" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"summed-cost-quota","limits":[{"limit":10,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_header","key":"X-Header-Cost"},{"type":"request_body","jsonPath":"$.body_cost"}],"default":0}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-Header-Cost" to "3"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"body_cost":4}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "3"

    When I set header "X-Header-Cost" to "2"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {"body_cost":2}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Cost is extracted using a CEL expression over the response
    Given I generate a unique value from "arl-cost-response-cel" and store it as "apiName"
    And I generate a unique API version from "arl-cost-response-cel" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-response-cel" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"response-cel-quota","limits":[{"limit":4,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_cel","expression":"response.Status / 100"}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Cost is extracted from a response header
    Given I generate a unique value from "arl-cost-response-header" and store it as "apiName"
    And I generate a unique API version from "arl-cost-response-header" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-response-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"response-header-quota","limits":[{"limit":100,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_header","key":"Content-Length"}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A fractional multiplier scales the extracted cost
    Given I generate a unique value from "arl-cost-fractional" and store it as "apiName"
    And I generate a unique API version from "arl-cost-fractional" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-fractional" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"fractional-multiplier-quota","limits":[{"limit":4,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_header","key":"X-Token-Cost","multiplier":0.5}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-Token-Cost" to "4"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    When I set header "X-Token-Cost" to "4"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I set header "X-Token-Cost" to "4"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" with body:
      """
      {}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A zero extracted cost does not consume quota
    Given I generate a unique value from "arl-cost-zero" and store it as "apiName"
    And I generate a unique API version from "arl-cost-zero" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-zero" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"zero-cost-quota","limits":[{"limit":2,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_header","key":"X-Token-Cost"}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-Token-Cost" to "0"
    And I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I set header "X-Token-Cost" to "1"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I set header "X-Token-Cost" to "1"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I set header "X-Token-Cost" to "1"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A malformed JSON request body falls back to the default cost
    Given I generate a unique value from "arl-cost-malformed-json" and store it as "apiName"
    And I generate a unique API version from "arl-cost-malformed-json" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-malformed-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/anything","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"malformed-json-quota","limits":[{"limit":4,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"request_body","jsonPath":"$.tokens"}],"default":2}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {invalid-json
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {invalid-json
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {invalid-json
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A per-quota cost multiplier scales cost extracted from the response body
    Given I generate a unique value from "arl-cost-quota-multiplier" and store it as "apiName"
    And I generate a unique API version from "arl-cost-quota-multiplier" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-quota-multiplier" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/anything","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"token-limit","limits":[{"limit":100,"duration":"1h"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_body","jsonPath":"$.json.tokens","multiplier":2.0}],"default":1}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"tokens":25}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "50"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"tokens":25}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"tokens":10}
      """
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Two quotas independently track prompt and completion token costs per user
    Given I generate a unique value from "arl-cost-prompt-completion" and store it as "apiName"
    And I generate a unique API version from "arl-cost-prompt-completion" and store it as "apiVersion"
    And I generate a unique API context from "/arl-cost-prompt-completion" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/anything","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"prompt-tokens","limits":[{"limit":500,"duration":"1h"}],"keyExtraction":[{"type":"header","key":"X-User-ID"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_body","jsonPath":"$.json.usage.prompt_tokens","multiplier":1.0}],"default":0}},{"name":"completion-tokens","limits":[{"limit":200,"duration":"1h"}],"keyExtraction":[{"type":"header","key":"X-User-ID"}],"costExtraction":{"enabled":true,"sources":[{"type":"response_body","jsonPath":"$.json.usage.completion_tokens","multiplier":1.0}],"default":0}}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I set header "X-User-ID" to "user-A"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"usage":{"prompt_tokens":50,"completion_tokens":100}}
      """
    Then the response status code should be 200

    When I set header "X-User-ID" to "user-A"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"usage":{"prompt_tokens":50,"completion_tokens":100}}
      """
    Then the response status code should be 200

    When I set header "X-User-ID" to "user-A"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"usage":{"prompt_tokens":50,"completion_tokens":50}}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I set header "X-User-ID" to "user-B"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"usage":{"prompt_tokens":250,"completion_tokens":10}}
      """
    Then the response status code should be 200

    When I set header "X-User-ID" to "user-B"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"usage":{"prompt_tokens":250,"completion_tokens":10}}
      """
    Then the response status code should be 200

    When I set header "X-User-ID" to "user-B"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" with body:
      """
      {"usage":{"prompt_tokens":50,"completion_tokens":10}}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
