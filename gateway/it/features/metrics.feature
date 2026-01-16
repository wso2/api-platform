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

Feature: Gateway Metrics
  As an operator
  I want to verify that gateway components expose Prometheus metrics
  So that I can monitor the health and performance of the gateway

  Background:
    Given the gateway services are running

  Scenario: Gateway controller metrics endpoint is accessible
    When I send a GET request to the gateway controller metrics endpoint
    Then the response status code should be 200
    And the response should contain Prometheus metrics

  Scenario: Policy engine metrics endpoint is accessible
    When I send a GET request to the policy engine metrics endpoint
    Then the response status code should be 200
    And the response should contain Prometheus metrics

  Scenario: Gateway controller metrics reflect API operations
    Given I am authenticated as "admin" with password "admin"
    When I create a new API with name "metrics-test-api"
    And I send a GET request to the gateway controller metrics endpoint
    Then the response should contain metric "gateway_controller_api_operations_total"
    And the response should contain metric "gateway_controller_apis_total"

  Scenario: Policy engine metrics reflect request processing
    Given I am authenticated as "admin" with password "admin"
    And I create a new API with the following configuration:
      """
      {
        "name": "metrics-api",
        "version": "1.0",
        "basePath": "/metrics-api",
        "backend": {
          "url": "http://sample-backend:9080"
        },
        "routes": [
          {
            "path": "/test",
            "methods": ["GET"]
          }
        ]
      }
      """
    And I wait for API deployment to complete
    When I send a GET request to "/metrics-api/test" through the router
    Then the response status code should be 200
    When I send a GET request to the policy engine metrics endpoint
    Then the response should contain metric "policy_engine_requests_total"
