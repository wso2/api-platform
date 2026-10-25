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

@platform-api-api-portal-register
Feature: An operator registers an API Portal through platform-api
  As an API platform operator
  I want to register an already-running API Portal against my organization through platform-api
  So that platform-api can publish APIs to that portal using the stored shared key, with the key
  itself never readable back from any response

  Background:
    Given the gateway services are running

  @smoke
  Scenario: Registering a valid API Portal returns 201 with Location, and the shared key is never echoed
    Given I generate a unique resource name from "mp-portal" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle} display                                                 |
      | handle    | ${CTX:portalHandle}                                                         |
      | url       | https://${CTX:portalHandle}.example.com                                     |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef            |
    Then the response status code should be 201
    And the response header "Location" should contain "/api-portals/${CTX:portalHandle}"
    And the JSON response field "id" should be "${CTX:portalHandle}"
    And the JSON response field "name" should be "${CTX:portalHandle} display"
    And the JSON response field "url" should be "https://${CTX:portalHandle}.example.com"
    And the JSON response field "sharedKey" should not exist

  @negative
  Scenario: Registering an API Portal with a non-HTTPS URL is rejected
    Given I generate a unique resource name from "mp-portal-http" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle}                                              |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | http://${CTX:portalHandle}.example.com                           |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 400

  @negative
  Scenario Outline: Registering an API Portal with a malformed shared key is rejected
    Given I generate a unique resource name from "mp-portal-key" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle}                     |
      | handle    | ${CTX:portalHandle}                     |
      | url       | https://${CTX:portalHandle}.example.com |
      | sharedKey | <sharedKey>                             |
    Then the response status code should be 400

    Examples:
      | sharedKey                                                         |
      | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde   |
      | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0 |
      | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeZ  |
      |                                                                   |

  @negative
  Scenario Outline: Registering an API Portal with an invalid handle is rejected
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | sample                                                           |
      | handle    | <handle>                                                         |
      | url       | https://sample.example.com                                       |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 400

    Examples:
      | handle                                     |
      | AB                                         |
      | Portal                                     |
      | portal with spaces                         |
      | mp-portal-01234567890123456789012345678901 |

  @negative
  Scenario: Registering the same handle twice in one organization returns 409
    Given I generate a unique resource name from "mp-portal-dup" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle}                                              |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | https://${CTX:portalHandle}.example.com                          |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 201
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle}                                              |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | https://${CTX:portalHandle}-two.example.com                      |
      | sharedKey | fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210 |
    Then the response status code should be 409

  @regression
  Scenario: Updating an API Portal mutates allowed fields, keeps the handle immutable, and never echoes the rotated key
    Given I generate a unique resource name from "mp-portal-upd" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle} original                                     |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | https://${CTX:portalHandle}.example.com                          |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 201
    When I update the API Portal "${CTX:portalHandle}" via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle} updated                                      |
      | handle    | mp-portal-rename-attempt                                         |
      | url       | https://${CTX:portalHandle}-v2.example.com                       |
      | sharedKey | fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210 |
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:portalHandle}"
    And the JSON response field "name" should be "${CTX:portalHandle} updated"
    And the JSON response field "url" should be "https://${CTX:portalHandle}-v2.example.com"
    And the JSON response field "sharedKey" should not exist

  @regression
  Scenario: An API Portal walks the full lifecycle: register, read, list, delete, and 404 after
    Given I generate a unique resource name from "mp-portal-lc" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" with values:
      | name      | ${CTX:portalHandle}                                              |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | https://${CTX:portalHandle}.example.com                          |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 201
    When I get the API Portal "${CTX:portalHandle}" via the control plane
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:portalHandle}"
    And the JSON response field "sharedKey" should not exist
    When I list API Portals via the control plane
    Then the response status code should be 200
    And the JSON response field "list" should contain "${CTX:portalHandle}"
    When I delete the API Portal "${CTX:portalHandle}" via the control plane
    Then the response status code should be 204
    When I get the API Portal "${CTX:portalHandle}" via the control plane
    Then the response status code should be 404

  @negative
  Scenario: A caller without ap:api_portal:* scopes is refused create and read
    Given I generate a unique resource name from "mp-portal-dev" and store it as "portalHandle"
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" as "developer" with values:
      | name      | ${CTX:portalHandle}                                              |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | https://${CTX:portalHandle}.example.com                          |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 403
    When I list API Portals via the control plane as "developer"
    Then the response status code should be 403

  @negative
  Scenario: A caller holding only ap:api_portal:read may list but cannot create
    Given I generate a unique resource name from "mp-portal-pub" and store it as "portalHandle"
    When I list API Portals via the control plane as "publisher"
    Then the response status code should be 200
    When I create an API Portal via the control plane from "resources/templates/api-portal.yaml" as "publisher" with values:
      | name      | ${CTX:portalHandle}                                              |
      | handle    | ${CTX:portalHandle}                                              |
      | url       | https://${CTX:portalHandle}.example.com                          |
      | sharedKey | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
    Then the response status code should be 403
