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
Feature: Another organization cannot read an organization's resources
  As an organization administrator
  I want another organization to get the same answer for my resources as for ones that do not exist
  So that it cannot read them or even learn that they exist

  Background:
    Given I generate a unique value from "tenant-read-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @TENANT-001
  Scenario: Another organization gets not found for a REST API, gateway, application and secret
    Given I generate a unique resource name from "tenant-read-api" and store it as "apiHandle"
    And I generate a unique API context from "/tenant-read-api" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I generate a unique resource name from "tenant-read-gateway" and store it as "gatewayHandle"
    And I register a gateway "${CTX:gatewayHandle}" on the control plane
    And I generate a unique resource name from "tenant-read-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I generate a unique resource name from "tenant-read-secret" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "REST_API_NOT_FOUND"
    And the JSON response field "message" should be "The specified REST API could not be found."
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}-absent" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "REST_API_NOT_FOUND"
    And the JSON response field "message" should be "The specified REST API could not be found."
    When I send a "GET" request to the control plane at "/gateways/${CTX:gatewayHandle}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/gateways/${CTX:gatewayHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "GATEWAY_NOT_FOUND"
    And the JSON response field "message" should be "The specified gateway could not be found."
    When I send a "GET" request to the control plane at "/gateways/${CTX:gatewayHandle}-absent" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "GATEWAY_NOT_FOUND"
    And the JSON response field "message" should be "The specified gateway could not be found."
    When I send a "GET" request to the control plane at "/applications/${CTX:appHandle}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/applications/${CTX:appHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "APPLICATION_NOT_FOUND"
    And the JSON response field "message" should be "The specified application could not be found."
    When I send a "GET" request to the control plane at "/applications/${CTX:appHandle}-absent" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "APPLICATION_NOT_FOUND"
    And the JSON response field "message" should be "The specified application could not be found."
    When I send a "GET" request to the control plane at "/secrets/${CTX:secretHandle}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/secrets/${CTX:secretHandle}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "SECRET_NOT_FOUND"
    And the JSON response field "message" should be "The specified secret could not be found."
    When I send a "GET" request to the control plane at "/secrets/${CTX:secretHandle}-absent" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "SECRET_NOT_FOUND"
    And the JSON response field "message" should be "The specified secret could not be found."
