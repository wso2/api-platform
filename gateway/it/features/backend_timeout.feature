# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@backend-timeout
Feature: Backend timeout
  As an API developer
  I want backend timeout (upstreamDefinitions) to be enforced by the gateway
  So that requests to slow or unreachable backends fail within the configured timeout

  Background:
    Given the gateway services are running

  Scenario: RestApi backend timeout using upstreamDefinitions
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: timeout-api-v1.0
      spec:
        displayName: Timeout-API
        version: v1.0
        context: /timeout-api/$version
        upstreamDefinitions:
          - name: my-timeout-upstream
            timeout:
              request: 2s
              idle: 5s
            upstreams:
              - urls:
                  - https://www.google.com:81
        upstream:
          main:
            ref: my-timeout-upstream
        operations:
          - method: GET
            path: /timeout-test
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for 2 seconds
    When I send a GET request to "http://localhost:8080/timeout-api/v1.0/timeout-test"
    Then the response should be a server error
    Given I authenticate using basic auth as "admin"
    When I delete the API "timeout-api-v1.0"
    Then the response should be successful

  Scenario: RestApi without upstream timeout uses global defaults
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: timeout-api-global-v1.0
      spec:
        displayName: Timeout-API-Global
        version: v1.0
        context: /timeout-global/$version
        upstreamDefinitions:
          - name: my-timeout-upstream-global
            upstreams:
              - urls:
                  - https://www.google.com:81
        upstream:
          main:
            ref: my-timeout-upstream-global
        operations:
          - method: GET
            path: /timeout-test
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for 2 seconds
    When I send a GET request to "http://localhost:8080/timeout-global/v1.0/timeout-test"
    Then the response should be a server error
    Given I authenticate using basic auth as "admin"
    When I delete the API "timeout-api-global-v1.0"
    Then the response should be successful

