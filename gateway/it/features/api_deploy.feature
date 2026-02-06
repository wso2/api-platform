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

Feature: API Deployment and Invocation
  As an API developer
  I want to deploy an API configuration and invoke it
  So that I can verify the gateway routes requests correctly

  Background:
    Given the gateway services are running

  Scenario: Deploy a simple HTTP API and invoke it successfully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: weather-api-v1.0
      spec:
        displayName: Weather-API
        version: v1.0
        context: /weather/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: POST
            path: /alerts/active
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for 2 seconds
    When I send a GET request to "http://localhost:8080/weather/v1.0/us/seattle"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "/api/v2/us/seattle"

    Given I authenticate using basic auth as "admin"
    When I delete the API "weather-api-v1.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Deploy an HTTP API with labels and verify they are stored
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: labeled-api-v1.0
        labels:
          environment: production
          team: backend
          version: v1
      spec:
        displayName: Labeled-API
        version: v1.0
        context: /labeled/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for 2 seconds
    
    Given I authenticate using basic auth as "admin"
    When I get the API "labeled-api-v1.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "api.configuration.metadata.labels.environment" should be "production"
    And the JSON response field "api.configuration.metadata.labels.team" should be "backend"
    And the JSON response field "api.configuration.metadata.labels.version" should be "v1"
    
    Given I authenticate using basic auth as "admin"
    When I delete the API "labeled-api-v1.0"
    Then the response should be successful

  Scenario: Deploy an HTTP API with invalid labels (spaces in keys) should fail
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /test
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
