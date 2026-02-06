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

@cors
Feature: CORS Policy
  As an API developer
  I want to configure CORS policies on my API
  So that cross-origin requests are correctly allowed and preflighted

  Background:
    Given the gateway services are running

  Scenario: Preflight request allows configured origin, methods, and headers
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: cors-preflight-api
      spec:
        displayName: CORS Preflight API
        version: v1.0
        context: /cors-preflight/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        policies:
          - name: cors
            version: v0
            params:
              allowedOrigins:
                - "http://example.com"
                - '^https://[^.]+\.example\.com$'
                - "http://localhost:5000"
              allowedMethods:
                - "GET"
                - "POST"
              allowedHeaders:
                - "Content-Type"
              exposedHeaders:
                - X-Content-Type-Options
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: OPTIONS
            path: /{country_code}/{city}
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/cors-preflight/v1.0/test/test" to be ready

    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    And the response header "Access-Control-Allow-Methods" should contain "GET"
    And the response header "Access-Control-Allow-Methods" should contain "POST"
    And the response header "Access-Control-Allow-Headers" should contain "Content-Type"

  Scenario: Preflight request allows configured origin based on regex
    When I set header "Origin" to "https://app.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "https://app.example.com"
    And the response header "Access-Control-Allow-Methods" should contain "GET"
    And the response header "Access-Control-Allow-Methods" should contain "POST"
    And the response header "Access-Control-Allow-Headers" should contain "Content-Type"

  Scenario: Preflight request fails for disallowed origin
    When I set header "Origin" to "http://evil.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist

  Scenario: Preflight request fails for disallowed method
    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "PUT"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist
  
  Scenario: Preflight request fails for disallowed header
    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Authorization"
    And I send an OPTIONS request to "http://localhost:8080/cors-preflight/v1.0/us/seattle"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist
    And the response header "Access-Control-Allow-Methods" should not exist
    And the response header "Access-Control-Allow-Headers" should not exist

    Given I authenticate using basic auth as "admin"
    When I delete the API "cors-preflight-api"
    Then the response should be successful
