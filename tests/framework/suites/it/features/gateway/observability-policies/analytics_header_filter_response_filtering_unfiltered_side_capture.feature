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
Feature: Analytics header filter precedence alongside unfiltered header capture
  As an API developer
  I want filtered backend response headers excluded while unfiltered headers are still captured
  So that analytics collects exactly the headers my filter configuration allows

  # Response-header filtering together with capture on the unfiltered side: a response-only
  # filter leaving request headers captured, and removing the filter restoring response-header
  # capture. The request-header Examples are in
  # analytics_header_filter_unfiltered_side_capture.feature.

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

  Scenario: Only response header filtering configured leaves request analytics unaffected
    Given I generate a unique value from "ahf-response-only" and store it as "apiName"
    And I generate a unique API version from "ahf-response-only" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-response-only" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/response-only","policies":[{"name":"analytics-header-filter","version":"v1","params":{"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-only" until status 200

    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-only" should contain request header "authorization"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-only" should not contain response header "x-denied-response"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/response-only" should contain response header "x-allowed-response"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/response-only" until status 404
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

    Examples: response headers
      | side     | denied            | deniedValue | kept               | keptValue |
      | response | x-denied-response | denied      | x-allowed-response | allowed   |
