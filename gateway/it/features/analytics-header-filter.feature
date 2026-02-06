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

@analytics-header-filter
Feature: Analytics Header Filter Policy
  As an API developer
  I want to control which headers are included in analytics data
  So that I can prevent sensitive or noisy headers from being collected

  Background:
    Given the gateway services are running

  Scenario: Both request and response headers filtering configured
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-both-api
      spec:
        displayName: Analytics Header Filter Both API
        version: v1.0
        context: /analytics-both/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  requestHeadersToFilter:
                    operation: deny
                    headers:
                      - "authorization"
                      - "x-api-key"
                  responseHeadersToFilter:
                    operation: allow
                    headers:
                      - "content-type"
                      - "x-custom-header"
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/analytics-both/v1.0/test" to be ready

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "User-Agent" to "test-client"
    And I send a GET request to "http://localhost:8080/analytics-both/v1.0/test"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: Only request headers filtering configured
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-request-api
      spec:
        displayName: Analytics Header Filter Request API
        version: v1.0
        context: /analytics-request/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /data
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  requestHeadersToFilter:
                    operation: allow
                    headers:
                      - "content-type"
                      - "user-agent"
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/analytics-request/v1.0/data" to be ready

    When I set header "Content-Type" to "application/json"
    And I set header "User-Agent" to "test-client"
    And I set header "Authorization" to "Bearer secret-token"
    And I send a GET request to "http://localhost:8080/analytics-request/v1.0/data"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: Only response headers filtering configured
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-response-api
      spec:
        displayName: Analytics Header Filter Response API
        version: v1.0
        context: /analytics-response/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /headers
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  responseHeadersToFilter:
                    operation: deny
                    headers:
                      - "server"
                      - "x-powered-by"
                      - "x-internal-debug"
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/analytics-response/v1.0/headers" to be ready

    When I send a GET request to "http://localhost:8080/analytics-response/v1.0/headers"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: Invalid policy configuration - missing operation field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-invalid-api
      spec:
        displayName: Analytics Header Filter Invalid API
        version: v1.0
        context: /analytics-invalid/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  requestHeadersToFilter:
                    headers:
                      - "authorization"
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "configuration validation failed"

  Scenario: Invalid policy configuration - invalid operation value
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-invalid-op-api
      spec:
        displayName: Analytics Header Filter Invalid Op API
        version: v1.0
        context: /analytics-invalid-op/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  requestHeadersToFilter:
                    operation: invalid
                    headers:
                      - "authorization"
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "configuration validation failed"

  Scenario: Invalid policy configuration - missing headers field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-no-headers-api
      spec:
        displayName: Analytics Header Filter No Headers API
        version: v1.0
        context: /analytics-no-headers/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  responseHeadersToFilter:
                    operation: allow
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "configuration validation failed"

  Scenario: Case-insensitive header matching with allow operation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-case-api
      spec:
        displayName: Analytics Header Filter Case API
        version: v1.0
        context: /analytics-case/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /case-test
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  requestHeadersToFilter:
                    operation: allow
                    headers:
                      - "Content-Type"
                      - "USER-AGENT"
                      - "x-custom-header"
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/analytics-case/v1.0/case-test" to be ready

    When I set header "content-type" to "application/json"
    And I set header "user-agent" to "test-client"
    And I set header "X-Custom-Header" to "test-value"
    And I set header "Authorization" to "Bearer secret"
    And I send a GET request to "http://localhost:8080/analytics-case/v1.0/case-test"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: Empty headers array with deny operation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: analytics-header-filter-empty-api
      spec:
        displayName: Analytics Header Filter Empty API
        version: v1.0
        context: /analytics-empty/$version
        upstream:
          main:
            url: http://echo-backend:80/anything
        operations:
          - method: GET
            path: /empty-test
            policies:
              - name: analytics-header-filter
                version: v0
                params:
                  requestHeadersToFilter:
                    operation: deny
                    headers: []
                  responseHeadersToFilter:
                    operation: allow
                    headers: []
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/analytics-empty/v1.0/empty-test" to be ready

    When I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer token"
    And I send a GET request to "http://localhost:8080/analytics-empty/v1.0/empty-test"
    Then the response should be successful
    And the response should be valid JSON
