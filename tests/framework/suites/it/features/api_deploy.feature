# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

@api-deploy
Feature: API Deployment and Invocation
  As an API developer
  I want to deploy an API configuration and invoke it
  So that I can verify the gateway routes requests correctly

  Background:
    Given the gateway services are running

  Scenario: Deploy a simple HTTP API and invoke it successfully
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: weather-api-v1.0
      spec:
        displayName: Weather-API
        version: v1.0
        context: /weather/$version
        upstream:
          main:
            url: http://testbench:3000/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: POST
            path: /alerts/active
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/weather/v1.0/us/seattle" until status 200
    When I send a "GET" request to "/weather/v1.0/us/seattle"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "path" should be "/api/v2/us/seattle"

    Given I authenticate using basic auth as "admin"
    When I delete the API "weather-api-v1.0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Deploy an HTTP API with labels and verify they are stored
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: labeled-api-v1.0
        labels:
          environment: production
          team: backend
          version: v1
      spec:
        displayName: Labeled-Deploy-API
        version: v1.0
        context: /labeled/$version
        upstream:
          main:
            url: http://testbench:3000/api/v2
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    Given I authenticate using basic auth as "admin"
    When I get the API "labeled-api-v1.0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "metadata.labels.environment" should be "production"
    And the JSON response field "metadata.labels.team" should be "backend"
    And the JSON response field "metadata.labels.version" should be "v1"
    
    Given I authenticate using basic auth as "admin"
    When I delete the API "labeled-api-v1.0"
    Then the response status code should be 200

  Scenario: Deploy an HTTP API with invalid labels (spaces in keys) should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-labels-api-v1.0
        labels:
          "My Label": value
          team: backend
      spec:
        displayName: Invalid-Labels-API
        version: v1.0
        context: /invalid/$version
        upstream:
          main:
            url: http://testbench:3000/api/v2
        operations:
          - method: GET
            path: /test
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"

  Scenario: Deploy an API with a query string in an upstreamDefinitions URL should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-upstream-query-api-v1.0
      spec:
        displayName: Invalid-Upstream-Query-API
        version: v1.0
        context: /invalid-upstream-query/$version
        vhosts:
          main: invalid-upstream-query.local
        upstreamDefinitions:
          - name: backend-default
            basePath: /api-main
            upstreams:
              - url: http://testbench:3000?region=eu
        upstream:
          main:
            ref: backend-default
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].message" should be "URL must not include a query string; only host[:port] is used"

  Scenario: Deploy an API with a bare query marker in an upstreamDefinitions URL should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-upstream-bare-query-api-v1.0
      spec:
        displayName: Invalid-Upstream-Bare-Query-API
        version: v1.0
        context: /invalid-upstream-bare-query/$version
        vhosts:
          main: invalid-upstream-bare-query.local
        upstreamDefinitions:
          - name: backend-default
            basePath: /api-main
            upstreams:
              - url: http://testbench:3000?
        upstream:
          main:
            ref: backend-default
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].message" should be "URL must not include a query string; only host[:port] is used"

  Scenario: Deploy an API with a fragment in an upstreamDefinitions URL should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-upstream-fragment-api-v1.0
      spec:
        displayName: Invalid-Upstream-Fragment-API
        version: v1.0
        context: /invalid-upstream-fragment/$version
        vhosts:
          main: invalid-upstream-fragment.local
        upstreamDefinitions:
          - name: backend-default
            basePath: /api-main
            upstreams:
              - url: http://testbench:3000#section
        upstream:
          main:
            ref: backend-default
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].message" should be "URL must not include a fragment; only host[:port] is used"

  Scenario: Deploy an API with both a query string and a fragment in an upstreamDefinitions URL should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: invalid-upstream-query-fragment-api-v1.0
      spec:
        displayName: Invalid-Upstream-Query-Fragment-API
        version: v1.0
        context: /invalid-upstream-query-fragment/$version
        vhosts:
          main: invalid-upstream-query-fragment.local
        upstreamDefinitions:
          - name: backend-default
            basePath: /api-main
            upstreams:
              - url: http://testbench:3000?a=1#top
        upstream:
          main:
            ref: backend-default
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the JSON response field "errors[0].message" should be "URL must not include a query string; only host[:port] is used"
    And the JSON response field "errors[1].message" should be "URL must not include a fragment; only host[:port] is used"
