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

# test-config.toml raises router.http_listener.per_connection_buffer_limit_bytes and
# policy_engine.request_body.max_decompressed_bytes to 16 MiB for these scenarios.
@request-body-size
Feature: Request Body Size Limits
  As an API developer
  I want large request bodies to pass through policies that buffer the body
  So that payloads such as base64-encoded images are not capped below the configured limits

  Background:
    Given the gateway services are running

  Scenario: A buffered request body above 4 MiB reaches the backend
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: body-size-large-api
      spec:
        displayName: Request Body Size - Large Body
        version: v1.0
        context: /body-size-large/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /upload
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 1
                    max: 16777216
                    jsonPath: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/body-size-large/v1.0/health" to be ready

    # 6 MiB: above gRPC's 4 MiB default receive size, below the configured limits.
    When I send a POST request to "http://localhost:8080/body-size-large/v1.0/upload" with a JSON body of 6291456 bytes
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the API "body-size-large-api"
    Then the response should be successful

  Scenario: A buffered request body above the listener buffer limit is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: body-size-over-limit-api
      spec:
        displayName: Request Body Size - Over Limit
        version: v1.0
        context: /body-size-over-limit/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /upload
            policies:
              - name: content-length-guardrail
                version: v1
                params:
                  request:
                    min: 1
                    max: 33554432
                    jsonPath: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/body-size-over-limit/v1.0/health" to be ready

    # 17 MiB: above the 16 MiB per_connection_buffer_limit_bytes.
    When I send a POST request to "http://localhost:8080/body-size-over-limit/v1.0/upload" with a JSON body of 17825792 bytes
    Then the response status code should be 413

    Given I authenticate using basic auth as "admin"
    When I delete the API "body-size-over-limit-api"
    Then the response should be successful
