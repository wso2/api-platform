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

  # Request-header filtering in deny and allow modes, case-insensitive matching, empty and
  # omitted header lists, request and response filters configured together or alone,
  # unchanged upstream traffic, other policies left working, policy scope and order,
  # configuration changes, concurrency, and sensitive values. The response-header Examples of
  # the Scenario Outlines here are in analytics_header_filter_response_filtering.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: Both request and response header filtering configured
    Given I generate a unique value from "ahf-both" and store it as "apiName"
    And I generate a unique API version from "ahf-both" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-both" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
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
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should contain response header "content-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should not contain response header "x-custom-header"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Only request header filtering configured
    Given I generate a unique value from "ahf-request" and store it as "apiName"
    And I generate a unique API version from "ahf-request" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-request" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/data","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-payload-type","x-client-id"]}}}]}] |
    Then the response should be successful

    When I set header "X-Payload-Type" to "application/json"
    And I set header "X-Client-Id" to "test-client"
    And I set header "Authorization" to "Bearer secret-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should contain request header "x-payload-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should contain request header "x-client-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should not contain request header "authorization"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Only response header filtering configured
    Given I generate a unique value from "ahf-response" and store it as "apiName"
    And I generate a unique API version from "ahf-response" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-response" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/headers","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"deny","headers":["server","x-powered-by","x-internal-debug"]}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/headers" until status 200
    Then the response should be successful
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/headers" should not contain response header "server"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/headers" should not contain response header "x-powered-by"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/headers" should not contain response header "x-internal-debug"
    And I wait for the analytics collector to settle

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/headers" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: An omitted headers field defaults to an empty array
    Given I generate a unique value from "ahf-no-headers" and store it as "apiName"
    And I generate a unique API version from "ahf-no-headers" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-no-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny"},"response":{"mode":"allow"}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/test" until status 200
    Then the response should be successful
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should contain request header "authorization" with value "Bearer test-token"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should contain request header "x-client-id" with value "test-client"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should contain response header "x-allowed-response" with value "allowed"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/test" should contain response header "x-denied-response" with value "denied"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Header matching is case-insensitive with allow mode
    Given I generate a unique value from "ahf-case" and store it as "apiName"
    And I generate a unique API version from "ahf-case" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-case" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/case-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["X-Payload-Type","X-CLIENT-ID","x-custom-header"]}}}]}] |
    Then the response should be successful

    When I set header "x-payload-type" to "application/json"
    And I set header "x-client-id" to "test-client"
    And I set header "X-Custom-Header" to "test-value"
    And I set header "Authorization" to "Bearer secret"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should contain request header "x-payload-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should contain request header "x-client-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should contain request header "x-custom-header"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-test" should not contain request header "authorization"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: An empty headers array with deny mode denies nothing
    Given I generate a unique value from "ahf-empty" and store it as "apiName"
    And I generate a unique API version from "ahf-empty" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/empty-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":[]},"response":{"mode":"allow","headers":[]}}}]}] |
    Then the response should be successful

    When I set header "X-Payload-Type" to "application/json"
    And I set header "Authorization" to "Bearer token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/empty-test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/empty-test" should contain request header "x-payload-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/empty-test" should contain request header "authorization"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/empty-test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  # The scenarios from here on, and the omitted headers scenario above, use
  # http://testbench:3000/analytics-headers as their upstream. For every operation path under
  # that prefix, the testbench backend (testbench/services/backend/backend.go) returns these
  # fixed response headers:
  #
  #   X-Allowed-Response: allowed   -- the header a scenario expects to be kept
  #   X-Denied-Response:  denied    -- the header a scenario expects to be filtered out
  #   X-Removed-Response: removed   -- the header a scenario removes with remove-headers
  #   X-Multi-Response:   first     -- sent as two separate header lines, so the
  #   X-Multi-Response:   second       header carries multiple values
  #   Content-Type: application/json
  #
  # When the request carries X-Correlation-Id, the backend also returns its value as
  # X-Correlation-Response.
  #
  # It also reflects the request it received as JSON, with the request headers under
  # "headers". The "echoed header" steps read that to check what reached the upstream.

  Scenario: Request header deny mode with a single denied header
    Given I generate a unique value from "ahf-single-deny" and store it as "apiName"
    And I generate a unique API version from "ahf-single-deny" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-single-deny" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/single-deny","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/single-deny" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/single-deny" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/single-deny" should contain request header "x-client-id"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/single-deny" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Request header deny mode with multiple denied headers
    Given I generate a unique value from "ahf-multi-deny" and store it as "apiName"
    And I generate a unique API version from "ahf-multi-deny" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-multi-deny" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/multi-deny","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key","x-internal-id"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Internal-ID" to "internal-12345"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" should not contain request header "x-api-key"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" should not contain request header "x-internal-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" should contain request header "x-client-id"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Header matching is case-insensitive when the configured header name itself uses different casing
    Given I generate a unique value from "ahf-case-config" and store it as "apiName"
    And I generate a unique API version from "ahf-case-config" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-case-config" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/case-config-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["AUTHORIZATION"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-config-test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-config-test" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-config-test" should contain request header "x-client-id"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-config-test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: An empty headers array with allow mode does not remove analytics headers
    Given I generate a unique value from "ahf-empty-allow" and store it as "apiName"
    And I generate a unique API version from "ahf-empty-allow" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-empty-allow" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/empty-allow-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"allow","headers":[]}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/empty-allow-test" until status 200
    Then the response should be successful
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/empty-allow-test" should contain response header "x-allowed-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/empty-allow-test" should contain response header "x-removed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/empty-allow-test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Denied request headers are still sent unchanged to the actual upstream request
    Given I generate a unique value from "ahf-upstream-unaffected" and store it as "apiName"
    And I generate a unique API version from "ahf-upstream-unaffected" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-upstream-unaffected" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/upstream-unaffected","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-internal-trace"]}}}]}] |
    Then the response should be successful

    When I set header "X-Internal-Trace" to "trace-12345"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/upstream-unaffected" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/upstream-unaffected" should not contain request header "x-internal-trace"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/upstream-unaffected" should contain request header "x-client-id"
    And the response should contain echoed header "X-Internal-Trace" with value "trace-12345"
    And the response should contain echoed header "X-Client-Id" with value "test-client"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/upstream-unaffected" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Denying a sensitive authentication header keeps it available to upstream processing
    Given I generate a unique value from "ahf-sensitive-auth" and store it as "apiName"
    And I generate a unique API version from "ahf-sensitive-auth" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-sensitive-auth" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/sensitive-auth","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer sensitive-token"
    And I set header "X-API-Key" to "sensitive-key-12345"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/sensitive-auth" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/sensitive-auth" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/sensitive-auth" should not contain request header "x-api-key"
    And the response should contain echoed header "Authorization" with value "Bearer sensitive-token"
    And the response should contain echoed header "X-API-Key" with value "sensitive-key-12345"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/sensitive-auth" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Allow mode excludes additional unexpected headers while capturing configured ones
    Given I generate a unique value from "ahf-allow-unexpected" and store it as "apiName"
    And I generate a unique API version from "ahf-allow-unexpected" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-allow-unexpected" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/allow-unexpected","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-request-id","x-client-version"]}}}]}] |
    Then the response should be successful

    When I set header "X-Request-ID" to "req-12345"
    And I set header "X-Client-Version" to "2.0.0"
    And I set header "X-Custom-Header" to "unexpected-value"
    And I set header "X-Another-Unexpected" to "also-unexpected"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/allow-unexpected" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/allow-unexpected" should contain request header "x-request-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/allow-unexpected" should contain request header "x-client-version"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/allow-unexpected" should not contain request header "x-custom-header"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/allow-unexpected" should not contain request header "x-another-unexpected"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/allow-unexpected" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Deny mode excludes only the denied headers while capturing newly introduced ones normally
    Given I generate a unique value from "ahf-deny-unexpected" and store it as "apiName"
    And I generate a unique API version from "ahf-deny-unexpected" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-deny-unexpected" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/deny-unexpected","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Custom-Header" to "unexpected-value"
    And I set header "X-Another-Unexpected" to "also-unexpected"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/deny-unexpected" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/deny-unexpected" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/deny-unexpected" should not contain request header "x-api-key"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/deny-unexpected" should contain request header "x-custom-header"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/deny-unexpected" should contain request header "x-another-unexpected"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/deny-unexpected" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  # The framework sends each request header as a single line, so the request header's
  # multiple values are sent as one comma-separated field value. The backend returns the
  # response header as two separate lines.
  Scenario Outline: A header with multiple values is filtered as a whole (<side> headers)
    Given I generate a unique value from "ahf-multi-value-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-multi-value-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-multi-value-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/multi-deny","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-multi-request"]},"response":{"mode":"deny","headers":["x-multi-response"]}}}]},{"method":"GET","path":"/multi-allow","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-multi-request"]},"response":{"mode":"allow","headers":["x-multi-response"]}}}]}] |
    Then the response should be successful

    When I set header "X-Multi-Request" to "first, second"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" should not contain <side> header "<multi>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" should contain <side> header "<other>"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi-allow" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-allow" should contain <side> header "<multi>" <multiValues>
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/multi-allow" should not contain <side> header "<other>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi-deny" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | multi           | multiValues                | other       |
      | request | x-multi-request | with value "first, second" | x-client-id |

  Scenario Outline: An API-level policy filters analytics headers for every operation (<side> headers)
    Given I generate a unique value from "ahf-api-level-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-api-level-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-api-level-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.policies          | [{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}] |
      | spec.operations        | [{"method":"GET","path":"/orders"},{"method":"GET","path":"/customers"}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/orders" until status 200
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/customers" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/orders" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/orders" should contain <side> header "<kept>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/customers" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/customers" should contain <side> header "<kept>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/orders" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | denied        | kept        |
      | request | authorization | x-client-id |

  # Each operation carries its own filter, so every asserted header comes from a filtered side.
  # This checks operation scoping for response headers, and for request headers on gateway
  # releases that publish headers only for a side with a filter configured, where an operation
  # without the policy has no headers to compare.
  Scenario Outline: Each operation applies only its own operation-level filter (<side> headers)
    Given I generate a unique value from "ahf-op-own-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-op-own-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-op-own-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/first","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]},{"method":"GET","path":"/second","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-client-id"]},"response":{"mode":"deny","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/first" until status 200
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/second" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/first" should not contain <side> header "<firstDenied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/first" should contain <side> header "<secondDenied>" with value "<secondDeniedValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/second" should contain <side> header "<firstDenied>" with value "<firstDeniedValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/second" should not contain <side> header "<secondDenied>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/first" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | firstDenied   | firstDeniedValue  | secondDenied | secondDeniedValue |
      | request | authorization | Bearer test-token | x-client-id  | test-client       |

  # A request rejected by api-key-auth or basic-ratelimit short-circuits before the filter
  # runs, so only the successful request's analytics event is asserted. The collector is
  # reset after the rejected probe so that event cannot be mistaken for the successful one.
  Scenario: The filter does not change authentication or rate limiting behaviour
    Given I generate a unique value from "ahf-combined" and store it as "apiName"
    And I generate a unique API version from "ahf-combined" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/secured","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}},{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["api-key"]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys" with body:
      """
      {"name":"ahf-combined-caller"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "callerKey"

    When I clear all headers
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/secured" until status 401
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I set header "API-Key" to "${CTX:callerKey}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/secured"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "3"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/secured" should have response status 200
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/secured" should not contain request header "api-key"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/secured" should contain request header "x-client-id"

    When I send 2 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/secured"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/secured"
    Then the response status code should be 429
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario Outline: A header added by set-headers before the filter is filtered by the <mode> rule (<side> headers)
    Given I generate a unique value from "ahf-set-headers-<mode>-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-set-headers-<mode>-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-set-headers-<mode>-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/added","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Added-Request","value":"added"}]},"response":{"headers":[{"name":"X-Added-Response","value":"added"}]}}},{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"<mode>","headers":["x-added-request"]},"response":{"mode":"<mode>","headers":["x-added-response"]}}}]}] |
    Then the response should be successful

    When I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/added" until status 200
    Then the response should contain echoed header "X-Added-Request" with value "added"
    And the response header "X-Added-Response" should be "added"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/added" should <addedAssertion> <side> header "<added>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/added" should <otherAssertion> <side> header "<other>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/added" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | mode  | side    | added           | other       | addedAssertion | otherAssertion |
      | deny  | request | x-added-request | x-client-id | not contain    | contain        |
      | allow | request | x-added-request | x-client-id | contain        | not contain    |

  Scenario Outline: Headers removed by remove-headers before the filter are absent from traffic and analytics (<side> headers)
    Given I generate a unique value from "ahf-remove-headers-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-remove-headers-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-remove-headers-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/removed","policies":[{"name":"remove-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Removed-Request"}]},"response":{"headers":[{"name":"X-Removed-Response"}]}}},{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-filtered-request"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I set header "X-Removed-Request" to "removed"
    And I set header "X-Filtered-Request" to "filtered"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/removed" until status 200
    Then the response should not contain echoed header "X-Removed-Request"
    And the response should contain echoed header "X-Filtered-Request" with value "filtered"
    And the response should contain echoed header "X-Client-Id" with value "test-client"
    And the response header "X-Removed-Response" should not exist
    And the response header "X-Denied-Response" should be "denied"
    And the response header "X-Allowed-Response" should be "allowed"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed" should not contain <side> header "<removed>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/removed" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | removed           | denied             | kept        | keptValue   |
      | request | x-removed-request | x-filtered-request | x-client-id | test-client |

  # The filter has no data-plane effect, so propagation of the update is observed through the
  # policy-engine config dump. The API serves traffic before the update and the collector is
  # reset afterwards, so the assertions cannot match an event recorded before the update. The
  # events recorded before the update are not asserted: analytics without the filter is covered
  # by "Analytics captures headers normally when the filter is not attached" in
  # analytics_header_filter_unfiltered_side_capture.feature, which keeps this scenario runnable on
  # releases that publish headers only for a side with a filter.
  Scenario Outline: Adding the filter to an already deployed API filters subsequent analytics events (<side> headers)
    Given I generate a unique value from "ahf-added-later-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-added-later-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-added-later-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/added-later"}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/added-later" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/added-later","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful
    And the config dump should contain policy "analytics-header-filter" for route "${CTX:apiContext}/${CTX:apiVersion}/added-later"

    When I clear all headers
    And I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/added-later"
    Then the response status code should be 200
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/added-later" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/added-later" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/added-later" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | denied        | kept        | keptValue   |
      | request | authorization | x-client-id | test-client |

  # Each update keeps the policy name, so propagation is observed through a parameter that
  # changes in that update. The collector is reset before each update so every assertion reads
  # an event produced under the configuration being asserted.
  Scenario Outline: Updating the filter configuration applies the new rules to subsequent analytics events (<side> headers)
    Given I generate a unique value from "ahf-reconfigured-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-reconfigured-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-reconfigured-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/reconfigured","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-first"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I set header "X-First" to "first"
    And I set header "X-Second" to "second"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should not contain <side> header "<first>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should contain <side> header "<second>" with value "<secondValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should contain <side> header "<other>" with value "<otherValue>"
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/reconfigured","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-first"]},"response":{"mode":"allow","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful
    And the config dump should contain policy "analytics-header-filter" for route "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" with parameter "request.mode" set to "allow"

    When I clear all headers
    And I set header "X-First" to "first"
    And I set header "X-Second" to "second"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/reconfigured"
    Then the response status code should be 200
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should contain <side> header "<first>" with value "<firstValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should not contain <side> header "<second>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should not contain <side> header "<other>"
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/reconfigured","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-second"]},"response":{"mode":"allow","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful
    And the config dump should contain policy "analytics-header-filter" for route "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" with parameter "request.headers.0" set to "x-second"

    When I clear all headers
    And I set header "X-First" to "first"
    And I set header "X-Second" to "second"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/reconfigured"
    Then the response status code should be 200
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should contain <side> header "<second>" with value "<secondValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should not contain <side> header "<first>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" should not contain <side> header "<other>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/reconfigured" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    # Per side, the filter first denies <first>, then allows only <first>, then allows only
    # <second>. <other> is never configured, so it shows whether each mode keeps or drops the
    # headers the configuration does not name.
    Examples: request headers
      | side    | first   | firstValue | second   | secondValue | other       | otherValue  |
      | request | x-first | first      | x-second | second      | x-client-id | test-client |

  # Every request targets one operation, so all events share one URI. Each request carries its
  # own correlation id, which the backend reflects as a response header, so every event can be
  # tied to the request that produced it.
  Scenario Outline: Concurrent requests with different headers each produce their own filtered analytics event (<side> headers)
    Given I generate a unique value from "ahf-concurrent-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-concurrent-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-concurrent-<side>" and store it as "apiContext"
    And I generate a unique value from "ahf-req" and store it as "requestId"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/concurrent","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-secret-token"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/concurrent" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I send 100 concurrent "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/concurrent" with per-request headers:
      | X-Correlation-Id | ${CTX:requestId}        |
      | X-Tenant-Data    | ${CTX:requestId}-data   |
      | X-Secret-Token   | ${CTX:requestId}-secret |
    And I wait for the analytics collector to settle
    Then the analytics events for path "${CTX:apiContext}/${CTX:apiVersion}/concurrent" should record 100 requests identified by request header "X-Correlation-Id" with prefix "${CTX:requestId}" and headers:
      | <side> | <perRequest> | contain     | <perRequestValue> |
      | <side> | <denied>     | not contain |                   |

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/concurrent" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | perRequest    | perRequestValue       | denied         |
      | request | x-tenant-data | ${CTX:requestId}-data | x-secret-token |

  # The raw collector payload is searched, so a sensitive value is caught wherever it appears in
  # an event, not only under its own header name. Each example sends one sensitive value. The
  # correlation value is also reflected by the backend as a response header, so it must be
  # filtered from both sides of the event. Denying further request headers by name is covered
  # by the request header deny mode scenarios.
  Scenario Outline: Filtered sensitive values are absent from the published analytics events (<case>)
    Given I generate a unique value from "ahf-sensitive-<case>" and store it as "apiName"
    And I generate a unique API version from "ahf-sensitive-<case>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-sensitive-<case>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/sensitive-values","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-correlation-id"]},"response":{"mode":"deny","headers":["x-correlation-response"]}}}]}] |
    Then the response should be successful

    When I clear all headers
    And I set header "<header>" to "<value>"
    And I set header "X-Visible" to "visible-header-value"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/sensitive-values" until status 200
    Then the response <delivered>
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/sensitive-values" should contain request header "x-visible" with value "visible-header-value"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I send a "GET" request to the "analytics" service at "/test/events"
    Then the response status code should be 200
    And the response body should contain "visible-header-value"
    And the response body should not contain "<sensitiveValue>"

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/sensitive-values" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | case       | header        | value                             | sensitiveValue             | delivered                                                                                |
      | auth-token | Authorization | Bearer sensitive-auth-token-value | sensitive-auth-token-value | should contain echoed header "Authorization" with value "Bearer sensitive-auth-token-value" |

  # The API is recreated with the same name, version, and context, so only the filter
  # configuration distinguishes the two deployments. The collector settles and is reset after
  # the old route is gone, so events from the deletion polling cannot be read as new ones.
  Scenario Outline: Recreating a deleted API with a different filter configuration applies only the new rules (<side> headers)
    Given I generate a unique value from "ahf-recreated-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-recreated-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-recreated-<side>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/recreated","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["x-first"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I clear all headers
    And I set header "X-First" to "first"
    And I set header "X-Second" to "second"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/recreated" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/recreated" should not contain <side> header "<first>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/recreated" should contain <side> header "<second>" with value "<secondValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/recreated" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/recreated","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-first"]},"response":{"mode":"allow","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful
    And the config dump should contain policy "analytics-header-filter" for route "${CTX:apiContext}/${CTX:apiVersion}/recreated" with parameter "request.mode" set to "allow"

    When I clear all headers
    And I set header "X-First" to "first"
    And I set header "X-Second" to "second"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/recreated" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/recreated" should contain <side> header "<first>" with value "<firstValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/recreated" should not contain <side> header "<second>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/recreated" should not contain <side> header "<other>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/recreated" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    # Per side, the first deployment denies <first> and the recreated one allows only <first>.
    # <other> is never configured, so the recreated allow rule must drop it.
    Examples: request headers
      | side    | first   | firstValue | second   | secondValue | other       |
      | request | x-first | first      | x-second | second      | x-client-id |
