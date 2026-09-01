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

  # Migrated WITHOUT the analytics collector, and that is the whole point of why this
  # file — unlike analytics_basic — can run on the baseline topology. None of these
  # scenarios inspect a published event: they assert only on the gateway's OWN responses.
  # The valid cases prove the policy config is accepted and the request still flows
  # (200 + valid JSON); the invalid cases prove deploy-time validation rejects a bad
  # policy (400 + "Configuration validation failed"). What the policy does to analytics
  # DATA is never observed here, so no collector, no analytics overlay — nothing beyond
  # the shared gateway + testbench.
  #
  # Mechanical migration differences:
  #   - Upstream http://echo-backend:80/anything -> http://testbench:3000/anything. The
  #     appended path is kept. testbench:3000 (not :3002) because nothing asserts on the
  #     reflected $.json/$.args/url/data — only that the body is valid JSON, which the
  #     backend reflector returns on any path.
  #   - Request and readiness URLs are relative.
  #   - One authentication in Background instead of once per scenario.
  #   - The legacy "Headers field omitted" scenario ended with a manual delete + re-auth;
  #     createAPI registers per-scenario cleanup, so the API is removed automatically. The
  #     delete's own assertion is kept as an explicit step (deleteAPI deregisters it from
  #     cleanup, so there is no double-delete).

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Both request and response headers filtering configured
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-both-api
      spec:
        displayName: Analytics Header Filter Both API
        version: v1.0
        context: /analytics-both/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  request:
                    mode: deny
                    headers:
                      - "authorization"
                      - "x-api-key"
                  response:
                    mode: allow
                    headers:
                      - "content-type"
                      - "x-custom-header"
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/analytics-both/v1.0/test" until status 200

    When I set header "Authorization" to "Bearer test-token"
    And I set header "X-API-Key" to "secret-key"
    And I set header "User-Agent" to "test-client"
    And I send a "GET" request to "/analytics-both/v1.0/test"
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Only request headers filtering configured
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-request-api
      spec:
        displayName: Analytics Header Filter Request API
        version: v1.0
        context: /analytics-request/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /data
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  request:
                    mode: allow
                    headers:
                      - "content-type"
                      - "user-agent"
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/analytics-request/v1.0/data" until status 200

    When I set header "Content-Type" to "application/json"
    And I set header "User-Agent" to "test-client"
    And I set header "Authorization" to "Bearer secret-token"
    And I send a "GET" request to "/analytics-request/v1.0/data"
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Only response headers filtering configured
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-response-api
      spec:
        displayName: Analytics Header Filter Response API
        version: v1.0
        context: /analytics-response/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /headers
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  response:
                    mode: deny
                    headers:
                      - "server"
                      - "x-powered-by"
                      - "x-internal-debug"
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/analytics-response/v1.0/headers" until status 200

    When I send a "GET" request to "/analytics-response/v1.0/headers"
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Invalid policy configuration - missing mode field
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-invalid-api
      spec:
        displayName: Analytics Header Filter Invalid API
        version: v1.0
        context: /analytics-invalid/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  request:
                    headers:
                      - "authorization"
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"

  Scenario: Invalid policy configuration - invalid operation value
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-invalid-op-api
      spec:
        displayName: Analytics Header Filter Invalid Op API
        version: v1.0
        context: /analytics-invalid-op/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  request:
                    mode: invalid
                    headers:
                      - "authorization"
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"

  Scenario: Headers field omitted defaults to empty array
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-no-headers-api
      spec:
        displayName: Analytics Header Filter No Headers API
        version: v1.0
        context: /analytics-no-headers/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /test
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  response:
                    mode: allow
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/analytics-no-headers/v1.0/test" until status 200

    When I send a "GET" request to "/analytics-no-headers/v1.0/test"
    Then the response status code should be 200
    And the response should be valid JSON

    When I delete the API "analytics-header-filter-no-headers-api"
    Then the response status code should be 200

  Scenario: Case-insensitive header matching with allow operation
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-case-api
      spec:
        displayName: Analytics Header Filter Case API
        version: v1.0
        context: /analytics-case/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /case-test
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  request:
                    mode: allow
                    headers:
                      - "Content-Type"
                      - "USER-AGENT"
                      - "x-custom-header"
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/analytics-case/v1.0/case-test" until status 200

    When I set header "content-type" to "application/json"
    And I set header "user-agent" to "test-client"
    And I set header "X-Custom-Header" to "test-value"
    And I set header "Authorization" to "Bearer secret"
    And I send a "GET" request to "/analytics-case/v1.0/case-test"
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Empty headers array with deny operation
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: analytics-header-filter-empty-api
      spec:
        displayName: Analytics Header Filter Empty API
        version: v1.0
        context: /analytics-empty/$version
        upstream:
          main:
            url: http://testbench:3000/anything
        operations:
          - method: GET
            path: /empty-test
            policies:
              - name: analytics-header-filter
                version: v1
                params:
                  request:
                    mode: deny
                    headers: []
                  response:
                    mode: allow
                    headers: []
      """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/analytics-empty/v1.0/empty-test" until status 200

    When I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer token"
    And I send a "GET" request to "/analytics-empty/v1.0/empty-test"
    Then the response status code should be 200
    And the response should be valid JSON
