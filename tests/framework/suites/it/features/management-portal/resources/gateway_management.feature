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

@platform-api-gateway-management
Feature: Registered gateways keep their definition and rotate their tokens
  As an API platform administrator
  I want a registered gateway to keep its endpoints and properties and to rotate its tokens
  So that a gateway can be onboarded and re-keyed on every supported database

  Scenario: A registered gateway round-trips its endpoints and properties and starts inactive
    Given I generate a unique resource name from "mgmt-gw" and store it as "gateway"
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:gateway}", "displayName": "${CTX:gateway}", "endpoints": ["http://a.example.com", "http://b.example.com"], "functionalityType": "regular", "properties": {"region": "eu", "tier": "gold"}}
      """
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/gateways/${CTX:gateway}"
    When I send a "GET" request to the control plane at "/gateways/${CTX:gateway}"
    Then the response status code should be 200
    And the JSON response field "isActive" should be false
    And the JSON response field "functionalityType" should be "regular"
    And the JSON response array field "endpoints" should have 2 items
    And the JSON response field "endpoints[0]" should be "http://a.example.com"
    And the JSON response field "endpoints[1]" should be "http://b.example.com"
    And the JSON response field "properties.region" should be "eu"
    And the JSON response field "properties.tier" should be "gold"
    And the JSON response field "createdBy" should be "admin"
    When I send a "GET" request to the control plane at "/gateways?query=${CTX:gateway}"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:gateway}"

  Scenario: Gateway tokens are rotated and revoked independently
    Given I generate a unique resource name from "tok-gw" and store it as "gateway"
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:gateway}", "displayName": "${CTX:gateway}", "endpoints": ["http://tok.example.com"], "functionalityType": "regular"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the response status code should be 201
    And the JSON response should have field "token"
    And I store the JSON response field "id" as "firstToken"
    And the response header "Location" should be "/api/v0.9/gateways/${CTX:gateway}/tokens/${CTX:firstToken}"
    When I send a "POST" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the response status code should be 201
    And I store the JSON response field "id" as "secondToken"
    When I send a "GET" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response array "list" item with "id" equal to "${CTX:firstToken}" should have "status" equal to "active"
    When I send a "DELETE" request to the control plane at "/gateways/${CTX:gateway}/tokens/${CTX:firstToken}"
    Then the response status code should be 200
    When I send a "DELETE" request to the control plane at "/gateways/${CTX:gateway}/tokens/${CTX:firstToken}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:firstToken}"
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:secondToken}"
