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

Feature: Event Gateway Health Checks
  As an operator
  I want to verify that event gateway services are healthy
  So that I can ensure the gateway is operational

  Background:
    Given the event gateway services are running

  Scenario: Event gateway liveness probe returns UP
    When I send a GET request to the event gateway health endpoint
    Then the response status code should be 200
    And the response should be valid JSON
    And the response should indicate UP status

  Scenario: Event gateway readiness probe returns READY
    When I send a GET request to the event gateway ready endpoint
    Then the response status code should be 200
    And the response should be valid JSON
    And the response should indicate READY status
