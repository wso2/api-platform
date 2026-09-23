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
Feature: Analytics header capture alongside the analytics header filter
  As an API developer
  I want headers on a side without an analytics header filter to be captured normally
  So that filtering one side of the traffic does not hide the other side from analytics

  # Header capture on a side with no filter: the unfiltered side of a one-sided filter, an
  # operation or API without the filter, and an API the filter is removed from. The other
  # Examples of the Scenario Outlines here are in
  # analytics_header_filter_response_filtering_unfiltered_side_capture.feature and
  # analytics_header_filter_response_filtering.feature.

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

  Scenario: Only request header filtering configured leaves response analytics unaffected
    Given I generate a unique value from "ahf-request-only" and store it as "apiName"
    And I generate a unique API version from "ahf-request-only" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-request-only" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/request-only","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/request-only" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/request-only" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/request-only" should contain response header "x-allowed-response" with value "allowed"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/request-only" should contain response header "x-denied-response" with value "denied"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/request-only" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  # Operation scoping does not depend on the header side, so only request headers are asserted
  # here. Response-side operation scoping is covered by "Each operation applies only its own
  # operation-level filter" in analytics_header_filter_response_filtering.feature.
  Scenario: An operation-level policy filters analytics headers only for its own operation
    Given I generate a unique value from "ahf-op-scope" and store it as "apiName"
    And I generate a unique API version from "ahf-op-scope" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-op-scope" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/filtered","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]},{"method":"GET","path":"/unfiltered"}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/filtered" until status 200
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/unfiltered" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/filtered" should not contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/filtered" should contain request header "x-client-id"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/unfiltered" should contain request header "authorization" with value "Bearer test-token"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/unfiltered" should contain request header "x-client-id"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/filtered" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Analytics captures headers normally when the filter is not attached
    Given I generate a unique value from "ahf-not-attached" and store it as "apiName"
    And I generate a unique API version from "ahf-not-attached" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-not-attached" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/not-attached"}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/not-attached" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/not-attached" should contain request header "authorization" with value "Bearer test-token"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/not-attached" should contain request header "x-api-key" with value "secret-key"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/not-attached" should contain request header "x-client-id" with value "test-client"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/not-attached" should contain response header "x-allowed-response" with value "allowed"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/not-attached" should contain response header "x-denied-response" with value "denied"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/not-attached" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  # Removing the filter has no data-plane effect, so propagation of the update is observed
  # through the policy-engine config dump. The filter denies a generated marker header whose
  # name appears nowhere else in the dump, so the marker is present exactly while the filter is
  # attached. The collector is reset beforehand so the assertions cannot match an event recorded
  # while the filter applied.
  Scenario Outline: Removing the filter from an API restores normal analytics header collection (<side> headers)
    Given I generate a unique value from "ahf-removed-later-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-removed-later-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-removed-later-<side>" and store it as "apiContext"
    And I generate a unique value from "x-ahf-filter-marker" and store it as "filterMarker"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/removed-later","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","${CTX:filterMarker}"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/removed-later" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed-later" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed-later" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the response body should contain "${CTX:filterMarker}"

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/removed-later"}] |
    Then the response should be successful
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until the response body does not contain "${CTX:filterMarker}"
    Then the config dump should contain route with base path "${CTX:apiContext}/${CTX:apiVersion}"

    When I clear all headers
    And I set header "Authorization" to "Bearer test-token"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/removed-later"
    Then the response status code should be 200
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed-later" should contain <side> header "<denied>" with value "<deniedValue>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/removed-later" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/removed-later" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    Examples: request headers
      | side    | denied        | deniedValue       | kept        | keptValue   |
      | request | authorization | Bearer test-token | x-client-id | test-client |

  # The client-visible assertions are the backend's normal response, so the case shows the
  # filter leaves traffic unchanged. It configures only the request side, so it checks that
  # behaviour under a filter on releases that cannot filter response headers. The case that
  # filters both sides is in analytics_header_filter_response_filtering.feature.
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

    Examples: request headers filtered
      | case             | requestAnalytics | responseAnalytics | operations |
      | request-filtered | not contain      | contain           | [{"method":"POST","path":"/many-headers","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key","x-request-id","x-client-version","x-tenant-id","x-trace-id","x-session-id","x-correlation-id"]}}}]}] |
