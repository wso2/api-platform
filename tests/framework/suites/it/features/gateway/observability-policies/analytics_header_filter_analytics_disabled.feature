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
Feature: Analytics header filter policy with analytics disabled
  As an API developer
  I want the analytics header filter to be inert when analytics collection is disabled
  So that attaching it never affects API traffic

  # The filter attached to an API while analytics is disabled globally.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: The filter has no analytics effect when analytics is disabled globally
    Given I generate a unique value from "ahf-analytics-disabled" and store it as "apiName"
    And I generate a unique API version from "ahf-analytics-disabled" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-analytics-disabled" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3000/analytics-headers |
      | spec.operations        | [{"method":"GET","path":"/disabled","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":["authorization"]},"response":{"mode":"deny","headers":["x-denied-response"]}}}]}] |
    Then the response should be successful

    When I set header "Authorization" to "Bearer test-token"
    And I set header "User-Agent" to "test-client"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/disabled" until status 200
    Then the response should contain echoed header "Authorization" with value "Bearer test-token"
    And the response should contain echoed header "User-Agent" with value "test-client"
    And the response header "X-Denied-Response" should be "denied"
    And the response header "X-Allowed-Response" should be "allowed"
    And the analytics collector should have received 0 events

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/disabled" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"
