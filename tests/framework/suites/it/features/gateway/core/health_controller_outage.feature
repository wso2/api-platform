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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# The scenario stops the gateway controller, so it runs in a block whose other runners never
# run at the same time.
@health
Feature: Runtime health during a short controller outage
  As a platform operator
  I want the runtime to stay ready while its controller is briefly unavailable
  So that a routine controller restart does not take the data plane out of rotation

  Scenario: The runtime stays ready and keeps serving its routes while the controller is down
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "health-xds-api" and store it as "apiName"
    And I generate a unique API version from "health-xds" and store it as "apiVersion"
    And I generate a unique API context from "/health-xds" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                          |
      | name                   | ${CTX:apiName}                                     |
      | spec.displayName       | ${CTX:apiName}                                     |
      | spec.version           | ${CTX:apiVersion}                                  |
      | spec.context           | ${CTX:apiContext}/$version                         |
      | spec.upstream.main.url | http://testbench:3000/api/v2                       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    When I stop the gateway service "gateway-controller"
    And I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200
    And the JSON response field "status" should be "ready"
    And the JSON response field "checks.xds_connection.status" should be "up"
    And the JSON response field "checks.policy_engine.status" should be "up"
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/healthy"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle"
    Then the response status code should be 200
    When I start the gateway service "gateway-controller"
    And I send a "GET" request to the gateway controller health path "/api/admin/v1/ready" until status 200
    And I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200
    And the JSON response field "checks.xds_connection.status" should be "up"
