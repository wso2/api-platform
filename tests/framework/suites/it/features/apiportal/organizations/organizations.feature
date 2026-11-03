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

Feature: API Portal organization boundaries

  Scenario: Organization creation is not supported
    When I send an authenticated API Portal "POST" request to "/organizations" as "admin" with JSON body:
      """
      {"id":"new-org","displayName":"New Organization","idpRefId":"new-org"}
      """
    Then the response status code should be 405
    And the JSON response field "code" should be "METHOD_NOT_ALLOWED"

  Scenario: Organization listing is not supported
    When I send an authenticated API Portal "GET" request to "/organizations" as "admin"
    Then the response status code should be 405
    And the JSON response field "code" should be "METHOD_NOT_ALLOWED"

  Scenario: The configured organization can be retrieved
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"

  Scenario: The configured organization can be updated
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON body:
      """
      {"id":"default","idpRefId":"default","displayName":"Updated Organization"}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Updated Organization"

  Scenario: Deleting the configured organization is not supported
    When I send an authenticated API Portal "DELETE" request to "/organizations/default" as "admin"
    Then the response status code should be 405
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200

  Scenario: A foreign organization cannot be read
    When I send an authenticated API Portal "GET" request to "/organizations/foreign-org" as "admin"
    Then the response status code should be 403

  Scenario: A foreign organization cannot be updated
    When I send an authenticated API Portal "PUT" request to "/organizations/foreign-org" as "admin" with JSON body:
      """
      {"id":"foreign-org","idpRefId":"foreign-org","displayName":"Hijacked"}
      """
    Then the response status code should be 403

  Scenario: The organization handle cannot be changed
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON body:
      """
      {"id":"renamed-org","idpRefId":"default","displayName":"Renamed"}
      """
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200

  Scenario: The organization identity-provider reference cannot be changed
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON body:
      """
      {"id":"default","idpRefId":"foreign-idp","displayName":"Re-pointed"}
      """
    Then the response status code should be 400

  Scenario: An unauthenticated caller cannot list organizations
    When I send an unauthenticated API Portal "GET" request to "/organizations"
    Then the response status code should be 401

  Scenario: A developer cannot create an organization
    When I send an authenticated API Portal "POST" request to "/organizations" as "developer" with JSON body:
      """
      {"id":"developer-org","displayName":"Should Be Forbidden","idpRefId":"developer-org"}
      """
    Then the response status code should be 403
