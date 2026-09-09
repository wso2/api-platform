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

@ratelimit-headers
Feature: Advanced rate limit enforcement and response headers
  As an API developer
  I want the advanced-ratelimit policy to enforce quotas and report standard headers
  So that clients can observe their remaining quota and back off correctly

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce rate limit on an operation
    Given I generate a unique value from "arl-basic" and store it as "apiName"
    And I generate a unique API version from "arl-basic" and store it as "apiVersion"
    And I generate a unique API context from "/arl-basic" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/limited","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":10,"duration":"1h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 10 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multi-quota responses report IETF RateLimit headers alongside the legacy ones
    Given I generate a unique value from "arl-ietf-headers" and store it as "apiName"
    And I generate a unique API version from "arl-ietf-headers" and store it as "apiVersion"
    And I generate a unique API context from "/arl-ietf-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"burst","limits":[{"limit":10,"duration":"1m"}]},{"name":"daily","limits":[{"limit":100,"duration":"24h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should exist
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist
    And the response header "RateLimit-Policy" should exist
    And the response header "RateLimit" should exist
    And the response header "RateLimit-Policy" should contain "burst"
    And the response header "RateLimit-Policy" should contain "daily"
    And the response header "RateLimit" should contain "burst"
    And the response header "RateLimit" should contain "daily"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A 429 response identifies which quota was violated
    Given I generate a unique value from "arl-violated-quota" and store it as "apiName"
    And I generate a unique API version from "arl-violated-quota" and store it as "apiVersion"
    And I generate a unique API context from "/arl-violated-quota" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":4,"duration":"1h"}]},{"name":"daily","limits":[{"limit":100,"duration":"24h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 4 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response header "X-RateLimit-Quota" should be "request-limit"
    And the response header "RateLimit-Policy" should exist
    And the response header "RateLimit-Policy" should contain "request-limit"
    And the response header "RateLimit" should exist
    And the response header "RateLimit" should contain "request-limit"
    And the response header "Retry-After" should exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A successful response reports the configured limit and remaining quota
    Given I generate a unique value from "arl-headers" and store it as "apiName"
    And I generate a unique API version from "arl-headers" and store it as "apiVersion"
    And I generate a unique API context from "/arl-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/check","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":100,"duration":"1h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/check"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A custom JSON error response replaces the default 429 body
    Given I generate a unique value from "arl-custom-error" and store it as "apiName"
    And I generate a unique API version from "arl-custom-error" and store it as "apiVersion"
    And I generate a unique API context from "/arl-custom-error" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/custom","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":5,"duration":"1h"}]}],"onRateLimitExceeded":{"statusCode":429,"body":"{\"error\": \"Too Many Requests\", \"code\": 429001}","bodyFormat":"json"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/custom"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/custom"
    Then the response status code should be 429
    And the response should be valid JSON
    And the JSON response field "error" should be "Too Many Requests"
    And the JSON response field "code" should be 429001

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A custom plain-text error response can use a non-429 status code
    Given I generate a unique value from "arl-plain-status" and store it as "apiName"
    And I generate a unique API version from "arl-plain-status" and store it as "apiVersion"
    And I generate a unique API context from "/arl-plain-status" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"plain-error-limit","limits":[{"limit":1,"duration":"1h"}]}],"onRateLimitExceeded":{"statusCode":503,"body":"throttled","bodyFormat":"plain"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 503
    And the response body should contain "throttled"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A custom JSON error response can use a non-429 status code
    Given I generate a unique value from "arl-json-status" and store it as "apiName"
    And I generate a unique API version from "arl-json-status" and store it as "apiVersion"
    And I generate a unique API context from "/arl-json-status" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"json-error-quota","limits":[{"limit":1,"duration":"1h"}]}],"onRateLimitExceeded":{"statusCode":503,"body":"{\"error\":\"Throttled\",\"code\":503001}","bodyFormat":"json"}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 503
    And the response should be valid JSON
    And the JSON response field "error" should be "Throttled"
    And the JSON response field "code" should be 503001

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
