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

@analytics-header-filter
Feature: Analytics header filter policy
  As an API developer
  I want to control which headers are included in analytics data
  So that I can prevent sensitive or noisy headers from being collected

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: Both request and response header filtering configured
    Given I generate a unique value from "ahf-both" and store it as "apiName"
    And I generate a unique API version from "ahf-both" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-both" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key"]},"response":{"mode":"allow","headers":["content-type","x-custom-header"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "User-Agent" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should not contain request header "x-api-key"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Only request header filtering configured
    Given I generate a unique value from "ahf-request" and store it as "apiName"
    And I generate a unique API version from "ahf-request" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/data","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["content-type","user-agent"]}}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I set header "User-Agent" to "test-client"
    And I set header "Authorization" to "Bearer secret-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should contain request header "content-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should contain request header "user-agent"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should not contain request header "authorization"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Only response header filtering configured
    Given I generate a unique value from "ahf-response" and store it as "apiName"
    And I generate a unique API version from "ahf-response" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-response" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/headers","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"deny","headers":["server","x-powered-by","x-internal-debug"]}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/headers" until status 200
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An invalid policy configuration missing the mode field is rejected
    Given I generate a unique value from "ahf-invalid-mode" and store it as "apiName"
    And I generate a unique API version from "ahf-invalid-mode" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-invalid-mode" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"headers":["authorization"]}}}]}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"

  Scenario: An invalid policy configuration with an invalid mode value is rejected
    Given I generate a unique value from "ahf-invalid-op" and store it as "apiName"
    And I generate a unique API version from "ahf-invalid-op" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-invalid-op" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"invalid","headers":["authorization"]}}}]}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"

  Scenario: An omitted headers field defaults to an empty array
    Given I generate a unique value from "ahf-no-headers" and store it as "apiName"
    And I generate a unique API version from "ahf-no-headers" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-no-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"allow"}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/test" until status 200
    Then the response should be successful

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Header matching is case-insensitive with allow mode
    Given I generate a unique value from "ahf-case" and store it as "apiName"
    And I generate a unique API version from "ahf-case" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-case" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/case-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["Content-Type","USER-AGENT","x-custom-header"]}}}]}] |
    Then the response should be successful

    When I set header "content-type" to "application/json"
    And I set header "user-agent" to "test-client"
    And I set header "X-Custom-Header" to "test-value"
    And I set header "Authorization" to "Bearer secret"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should contain request header "content-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should contain request header "user-agent"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should contain request header "x-custom-header"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should not contain request header "authorization"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An empty headers array with deny mode denies nothing
    Given I generate a unique value from "ahf-empty" and store it as "apiName"
    And I generate a unique API version from "ahf-empty" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/empty-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":[]},"response":{"mode":"allow","headers":[]}}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/empty-test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/empty-test" should contain request header "content-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/empty-test" should contain request header "authorization"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
