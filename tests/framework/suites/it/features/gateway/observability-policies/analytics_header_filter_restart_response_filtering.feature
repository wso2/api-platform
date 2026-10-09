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
Feature: Analytics header filter precedence over backend response headers across gateway restarts
  As an API platform operator
  I want restarted gateway instances to keep filtering backend response headers from analytics
  So that response headers I filter out stay uncollected while gateway replicas restart

  # A restarted gateway runtime, or a runtime configured by a restarted controller, applying
  # the same filter to response headers returned by the backend. The request-header Examples
  # are in analytics_header_filter_restart.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  # The runtime holds its configuration only in memory, so after a restart it must receive the
  # route and the filter again from the controller. The filter denies a generated marker header,
  # and the policy-engine config dump must show it before post-restart traffic is checked.
  Scenario Outline: A restarted gateway runtime applies the same analytics header filter configuration (<side> headers)
    Given I generate a unique value from "ahf-runtime-restart-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-runtime-restart-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-runtime-restart-<side>" and store it as "apiContext"
    And I generate a unique value from "x-ahf-restart-marker" and store it as "filterMarker"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/restarted","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key","${CTX:filterMarker}"]},"response":{"mode":"allow","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful

    When I clear all headers
    And I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<otherDenied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I restart the "gateway-runtime" service
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 200
    And I clear all headers
    And I authenticate using basic auth as "admin"
    Then the config dump should contain policy "analytics-header-filter" for route "${CTX:apiContext}/${CTX:apiVersion}/restarted" with parameter "request.headers.2" set to "${CTX:filterMarker}"
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<otherDenied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    # The request filter denies <denied> and <otherDenied>; the response filter allows only
    # <kept>, so the backend's other response headers are dropped.
    Examples: response headers
      | side     | denied            | otherDenied        | kept               | keptValue |
      | response | x-denied-response | x-removed-response | x-allowed-response | allowed   |

  # The controller must reload the API and its filter from its database. The runtime is then
  # restarted so its configuration can only come from the restarted controller.
  Scenario Outline: A gateway runtime receives the same analytics header filter configuration from a restarted gateway controller (<side> headers)
    Given I generate a unique value from "ahf-controller-restart-<side>" and store it as "apiName"
    And I generate a unique API version from "ahf-controller-restart-<side>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-controller-restart-<side>" and store it as "apiContext"
    And I generate a unique value from "x-ahf-restart-marker" and store it as "filterMarker"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/restarted","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization","x-api-key","${CTX:filterMarker}"]},"response":{"mode":"allow","headers":["x-allowed-response"]}}}]}] |
    Then the response should be successful

    When I clear all headers
    And I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<otherDenied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status code should be 200
    And the response body should contain "${CTX:filterMarker}"

    When I restart the "gateway-runtime" service
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 200
    And I clear all headers
    And I authenticate using basic auth as "admin"
    Then the config dump should contain policy "analytics-header-filter" for route "${CTX:apiContext}/${CTX:apiVersion}/restarted" with parameter "request.headers.2" set to "${CTX:filterMarker}"
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "X-Client-Id" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 200
    Then the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<denied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should not contain <side> header "<otherDenied>"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/restarted" should contain <side> header "<kept>" with value "<keptValue>"
    And I wait for the analytics collector to settle

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/restarted" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

    # The request filter denies <denied> and <otherDenied>; the response filter allows only
    # <kept>, so the backend's other response headers are dropped.
    Examples: response headers
      | side     | denied            | otherDenied        | kept               | keptValue |
      | response | x-denied-response | x-removed-response | x-allowed-response | allowed   |
