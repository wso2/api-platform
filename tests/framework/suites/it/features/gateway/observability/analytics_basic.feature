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

@analytics-basic
Feature: Analytics basic event capture
  As a platform administrator
  I want analytics events to be captured and published
  So that I can monitor API usage and performance
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: A REST API request generates an analytics event
    Given I generate a unique value from "analytics-basic" and store it as "apiName"
    And I generate a unique API version from "analytics-basic" and store it as "apiVersion"
    And I generate a unique API context from "/analytics-basic" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/info"}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 200
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/info" should have request method "GET"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/info" should have response status 200
    And I wait for the analytics collector to settle

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: An analytics event contains API metadata
    Given I generate a unique value from "analytics-metadata" and store it as "apiName"
    And I generate a unique API version from "analytics-metadata" and store it as "apiVersion"
    And I generate a unique API context from "/analytics-metadata" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/data"}] |
    Then the response should be successful

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 200 with body:
      """
      {"test":"data"}
      """
    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should have metadata field "apiContext" with value "${CTX:apiContext}/${CTX:apiVersion}"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should have metadata field "apiName" with value "${CTX:apiName}"
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/data" should have metadata field "apiVersion" with value "${CTX:apiVersion}"
    And I wait for the analytics collector to settle

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"

  Scenario: Multiple requests generate multiple analytics events
    Given I generate a unique value from "analytics-multi" and store it as "apiName"
    And I generate a unique API version from "analytics-multi" and store it as "apiVersion"
    And I generate a unique API context from "/analytics-multi" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/ping"}] |
    Then the response should be successful

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/ping" until status 200
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/ping"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/ping"
    Then the response status code should be 200
    And the analytics collector should have received at least 3 events
    And I wait for the analytics collector to settle

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/ping" until status 404
    And I wait for the config dump to stop containing a route with base path "${CTX:apiContext}"
