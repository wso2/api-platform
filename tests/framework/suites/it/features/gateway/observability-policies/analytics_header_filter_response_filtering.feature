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
Feature: Analytics header filter precedence over backend response headers
  As an API developer
  I want the analytics header filter to decide which backend response headers reach analytics
  So that response headers I filter out are never collected

  # Filtering of response headers returned by the backend: deny and allow modes, request and
  # response filters combined, case-insensitive matching, and the client response left
  # unchanged. The request-header Examples of the Scenario Outlines here are in
  # analytics_header_filter.feature and analytics_header_filter_unfiltered_side_capture.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  # Every scenario in this feature uses http://testbench:3000/analytics-headers as its
  # upstream. For every operation path under that prefix, the testbench backend
  # (testbench/services/backend/backend.go) returns these fixed response headers:
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

  Scenario: Response header deny mode excludes a header actually returned by the backend
    Given I generate a unique value from "ahf-response-deny-real" and store it as "apiName"
    And I generate a unique API version from "ahf-response-deny-real" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-response-deny-real" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/response-deny-real","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-deny-real" until status 200
    Then the response should be successful
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-deny-real" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-deny-real" should contain response header "x-allowed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-deny-real" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Response header allow mode captures only the selected response headers
    Given I generate a unique value from "ahf-response-allow" and store it as "apiName"
    And I generate a unique API version from "ahf-response-allow" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-response-allow" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/response-allow","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"allow","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-allow" until status 200
    Then the response should be successful
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-allow" should contain response header "x-allowed-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-allow" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-allow" should not contain response header "x-removed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-allow" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Both request and response filters in deny mode
    Given I generate a unique value from "ahf-both-deny" and store it as "apiName"
    And I generate a unique API version from "ahf-both-deny" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-both-deny" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/both-deny","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/both-deny" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-deny" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-deny" should contain request header "x-client-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-deny" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-deny" should contain response header "x-allowed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/both-deny" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Both request and response filters in allow mode
    Given I generate a unique value from "ahf-both-allow" and store it as "apiName"
    And I generate a unique API version from "ahf-both-allow" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-both-allow" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/both-allow","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["x-client-id"]},"response":{"mode":"allow","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful

    When I set header "X-Client-Id" to "test-client"
    And I set header "Authorization" to "Bearer secret-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/both-allow" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-allow" should contain request header "x-client-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-allow" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-allow" should contain response header "x-allowed-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-allow" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/both-allow" should not contain response header "x-removed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/both-allow" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Request deny mode and response allow mode filter independently
    Given I generate a unique value from "ahf-mixed-modes" and store it as "apiName"
    And I generate a unique API version from "ahf-mixed-modes" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-mixed-modes" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/mixed-modes","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"allow","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/mixed-modes" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/mixed-modes" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/mixed-modes" should contain request header "x-client-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/mixed-modes" should contain response header "x-allowed-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/mixed-modes" should not contain response header "x-denied-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/mixed-modes" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Header matching is case-insensitive for mixed-case headers configured on both request and response
    Given I generate a unique value from "ahf-case-both" and store it as "apiName"
    And I generate a unique API version from "ahf-case-both" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-case-both" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/case-both-test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"allow","headers":["X-Payload-Type","X-CUSTOM-HEADER"]},"response":{"mode":"deny","headers":["X-DENIED-Response"]}}}]}] |
    Then the response should be successful

    When I set header "X-Payload-Type" to "application/json"
    And I set header "X-Custom-Header" to "test-value"
    And I set header "Authorization" to "Bearer secret-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" should contain request header "x-payload-type"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" should contain request header "x-custom-header"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" should contain response header "x-allowed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/case-both-test" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Denied response headers are still returned unchanged in the actual client response
    Given I generate a unique value from "ahf-client-unaffected" and store it as "apiName"
    And I generate a unique API version from "ahf-client-unaffected" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-client-unaffected" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/client-unaffected","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/client-unaffected" until status 200

    Then the response header "X-Denied-Response" should be "denied"
    And the response header "X-Allowed-Response" should be "allowed"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/client-unaffected" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/client-unaffected" should contain response header "x-allowed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/client-unaffected" until status 404
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

    Examples: response headers
      | side     | multi            | multiValues                | other              |
      | response | x-multi-response | with values "first,second" | x-allowed-response |

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

    Examples: response headers
      | side     | denied            | kept               |
      | response | x-denied-response | x-allowed-response |

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

    Examples: response headers
      | side     | firstDenied       | firstDeniedValue | secondDenied       | secondDeniedValue |
      | response | x-denied-response | denied           | x-allowed-response | allowed           |

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

    Examples: response headers
      | mode  | side     | added            | other              | addedAssertion | otherAssertion |
      | deny  | response | x-added-response | x-allowed-response | not contain    | contain        |
      | allow | response | x-added-response | x-allowed-response | contain        | not contain    |

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

    Examples: response headers
      | side     | removed            | denied            | kept               | keptValue |
      | response | x-removed-response | x-denied-response | x-allowed-response | allowed   |

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

    Examples: response headers
      | side     | denied            | kept               | keptValue |
      | response | x-denied-response | x-allowed-response | allowed   |

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
    Examples: response headers
      | side     | first             | firstValue | second             | secondValue | other              | otherValue |
      | response | x-denied-response | denied     | x-allowed-response | allowed     | x-removed-response | removed    |

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

    Examples: response headers
      | side     | perRequest             | perRequestValue  | denied            |
      | response | x-correlation-response | ${CTX:requestId} | x-denied-response |

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

    Examples: response headers
      | case        | header           | value                       | sensitiveValue              | delivered                                                         |
      | correlation | X-Correlation-Id | sensitive-correlation-value | sensitive-correlation-value | header "X-Correlation-Response" should be "sensitive-correlation-value" |

  # The client-visible assertions are the backend's normal response, so the case shows the
  # filter leaves traffic unchanged. The case that configures only the request side is in
  # analytics_header_filter_unfiltered_side_capture.feature.
  Scenario Outline: Filtering many headers leaves the API response unchanged (<case>)
    Given I generate a unique value from "ahf-many-<case>" and store it as "apiName"
    And I generate a unique API version from "ahf-many-<case>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-many-<case>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | <operations> |
    Then the response should be successful

    When I clear all headers
    And I set header "Authorization" to "Bearer many-token"
    And I set header "X-API-Key" to "many-key"
    And I set header "X-Request-ID" to "req-many"
    And I set header "X-Client-Version" to "3.1.4"
    And I set header "X-Tenant-ID" to "tenant-many"
    And I set header "X-Trace-ID" to "trace-many"
    And I set header "X-Session-ID" to "session-many"
    And I set header "X-Correlation-Id" to "corr-many"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/many-headers" until status 200 with body:
      """
      {"order":"12345","items":[1,2,3]}
      """
    Then the response header "Content-Type" should be "application/json"
    And the JSON response field "method" should be "POST"
    And the JSON response field "body" should be:
      """
      {"order":"12345","items":[1,2,3]}
      """
    And the response should contain echoed header "Authorization" with value "Bearer many-token"
    And the response should contain echoed header "X-API-Key" with value "many-key"
    And the response should contain echoed header "X-Request-ID" with value "req-many"
    And the response should contain echoed header "X-Client-Version" with value "3.1.4"
    And the response should contain echoed header "X-Tenant-ID" with value "tenant-many"
    And the response should contain echoed header "X-Trace-ID" with value "trace-many"
    And the response should contain echoed header "X-Session-ID" with value "session-many"
    And the response should contain echoed header "X-Correlation-Id" with value "corr-many"
    And the response header "X-Allowed-Response" should be "allowed"
    And the response header "X-Denied-Response" should be "denied"
    And the response header "X-Removed-Response" should be "removed"
    And the response header "X-Correlation-Response" should be "corr-many"
    And the response header "X-Multi-Response" should exist
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/many-headers" should <requestAnalytics> request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/many-headers" should <requestAnalytics> request header "x-session-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/many-headers" should <responseAnalytics> response header "x-removed-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/many-headers" should <responseAnalytics> response header "x-correlation-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/many-headers" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request and response headers filtered
      | case     | requestAnalytics | responseAnalytics | operations |
      | filtered | not contain      | not contain       | [{"method":"POST","path":"/many-headers","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key","x-request-id","x-client-version","x-tenant-id","x-trace-id","x-session-id","x-correlation-id"]},"response":{"mode":"allow","headers":["content-type"]}}}]}] |

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
    Examples: response headers
      | side     | first             | firstValue | second             | secondValue | other              |
      | response | x-denied-response | denied     | x-allowed-response | allowed     | x-removed-response |
