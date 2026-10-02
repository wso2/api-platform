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

@gateway-controller-admin-mcp
Feature: Gateway controller admin MCP endpoint
  As a gateway administrator using an MCP client
  I want read-only runtime status from the controller's admin MCP endpoint
  So that I can check gateway health without management privileges being exposed there

  Background:
    Given the gateway services are running

  Scenario: tools/list advertises the read-only admin tools
    Given I authenticate using basic auth as "admin"
    When I send an MCP "tools/list" request to the "gateway-controller-admin" service
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response array field "result.tools" should have 2 items
    And the response body should contain "wso2_apip_gw_get_gateway_status"
    And the response body should contain "wso2_apip_gw_get_config_dump"

  Scenario: The gateway status tool reports a healthy controller
    Given I authenticate using basic auth as "admin"
    When I call the MCP tool "wso2_apip_gw_get_gateway_status" on the "gateway-controller-admin" service with arguments:
      """
      {}
      """
    Then the response status code should be 200
    And the JSON response field "result.isError" should not exist
    And the JSON response field "result.structuredContent.health.status" should be "healthy"
    And the JSON response should have field "result.structuredContent.xds_sync"

  Scenario: A non-admin caller is refused the admin MCP endpoint
    Given I authenticate using basic auth as "developer"
    When I send an MCP "tools/list" request to the "gateway-controller-admin" service
    Then the response status code should be 403
    And the JSON response field "code" should be "forbidden"
