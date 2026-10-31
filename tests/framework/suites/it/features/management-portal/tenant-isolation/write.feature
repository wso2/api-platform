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

@TENANT
Feature: Another organization cannot change or remove an organization's resources
  As an organization administrator
  I want another organization's changes to my resources to be refused as not found
  So that my resources stay exactly as I left them

  Background:
    Given I generate a unique value from "tenant-write-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @TENANT-003
  Scenario: Another organization's update, delete, token rotation and secret delete are refused and change nothing
    Given I generate a unique resource name from "tenant-write-api" and store it as "apiHandle"
    And I generate a unique API context from "/tenant-write-api" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I generate a unique resource name from "tenant-write-gateway" and store it as "gatewayHandle"
    And I register a gateway "${CTX:gatewayHandle}" on the control plane
    And I generate a unique resource name from "tenant-write-secret" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    When I send a "PUT" request to the control plane at "/rest-apis/${CTX:apiHandle}" as "org B admin" with body:
      """
      {
        "id": "${CTX:apiHandle}",
        "displayName": "taken-over",
        "context": "${CTX:apiContext}",
        "version": "v1",
        "projectId": "${CTX:projectHandle}",
        "upstream": {"main": {"url": "http://testbench:3001"}}
      }
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "REST_API_NOT_FOUND"
    When I send a "DELETE" request to the control plane at "/rest-apis/${CTX:apiHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "REST_API_NOT_FOUND"
    When I send a "POST" request to the control plane at "/gateways/${CTX:gatewayHandle}/tokens" as "org B admin" with body:
      """
      {}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "GATEWAY_NOT_FOUND"
    When I send a "DELETE" request to the control plane at "/secrets/${CTX:secretHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "SECRET_NOT_FOUND"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiHandle}"
    And the JSON response field "context" should be "${CTX:apiContext}"
    When I send a "GET" request to the control plane at "/gateways/${CTX:gatewayHandle}/tokens"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    When I send a "GET" request to the control plane at "/secrets/${CTX:secretHandle}"
    Then the response status code should be 200
    And the JSON response field "status" should be "ACTIVE"
