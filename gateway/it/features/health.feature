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

Feature: Gateway Health Check
  As an operator
  I want to verify that gateway services are healthy
  So that I can ensure the gateway is operational

  Background:
    Given the gateway services are running

  Scenario: Gateway controller health endpoint returns OK
    When I send a GET request to the gateway controller health endpoint
    Then the response status code should be 200
    And the response should indicate healthy status

  Scenario: Router is ready to accept traffic
    When I send a GET request to the router ready endpoint
    Then the response status code should be 200

  Scenario: All gateway services are healthy
    When I check the health of all gateway services
    Then all services should report healthy status

  # ==================== HEALTH ENDPOINT RESPONSE VALIDATION ====================

  Scenario: Gateway controller health endpoint returns valid JSON
    When I send a GET request to the gateway controller health endpoint
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Gateway controller health endpoint is accessible without authentication
    When I send a GET request to the gateway controller health endpoint
    Then the response status code should be 200
