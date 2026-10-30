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

# A policy that reads the whole request body (content-length-guardrail here) makes Envoy send the
# body to the policy engine as one ext_proc gRPC message. The block's overlay raises
# router.http_listener.per_connection_buffer_limit_bytes and
# policy_engine.request_body.max_decompressed_bytes to 16 MiB.
@request-body-size
Feature: Request body size limits
  As an API developer
  I want large request bodies to pass through policies that buffer the body
  So that payloads such as base64-encoded images are not capped below the configured limits

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A buffered request body above 4 MiB reaches the backend
    Given I generate a unique value from "body-size-large" and store it as "apiName"
    And I generate a unique API version from "body-size-large" and store it as "apiVersion"
    And I generate a unique API context from "/body-size-large" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | ${CTX:apiVersion}          |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000      |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/upload","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":1,"max":16777216,"jsonPath":""}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # 6 MiB: above gRPC's 4 MiB default receive size, below the configured limits.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/upload" with a JSON body of 6291456 bytes
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: A buffered request body above the listener buffer limit is rejected
    Given I generate a unique value from "body-size-over-limit" and store it as "apiName"
    And I generate a unique API version from "body-size-over-limit" and store it as "apiVersion"
    And I generate a unique API context from "/body-size-over-limit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | ${CTX:apiVersion}          |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000      |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/upload","policies":[{"name":"content-length-guardrail","version":"v1","params":{"request":{"min":1,"max":33554432,"jsonPath":""}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # 17 MiB: above the 16 MiB per_connection_buffer_limit_bytes. The testbench refuses bodies over
    # 10 MiB with its own 413 ("Request Entity Too Large"), so the body tells the gateway's apart.
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/upload" with a JSON body of 17825792 bytes
    Then the response status code should be 413
    And the response body should contain "Payload Too Large"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404
