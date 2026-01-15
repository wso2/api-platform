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

Feature: Gateway Metrics Endpoints
  As an operator
  I want to verify that metrics endpoints are working
  So that I can monitor the gateway components

  Background:
    Given the gateway services are running

  Scenario: Gateway controller metrics endpoint is accessible
    When I send a GET request to the gateway controller metrics endpoint
    Then the response status code should be 200
    And the response should contain prometheus metrics

  Scenario: Policy engine metrics endpoint is accessible
    When I send a GET request to the policy engine metrics endpoint
    Then the response status code should be 200
    And the response should contain prometheus metrics

  Scenario: Gateway controller metrics include API count
    When I send a GET request to the gateway controller metrics endpoint
    Then the response status code should be 200
    And the metrics should contain "gateway_controller_apis_total"

  Scenario: API count metric updates after API creation
    Given I send a GET request to the gateway controller metrics endpoint
    And I extract the current API count from metrics
    When I create a new API via the gateway controller
    And I send a GET request to the gateway controller metrics endpoint
    Then the API count metric should have increased
