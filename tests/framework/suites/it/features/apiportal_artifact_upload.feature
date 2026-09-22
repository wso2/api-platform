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

Feature: API Portal artifact uploads

  Scenario: A publisher creates an API from an artifact ZIP
    Given I generate a unique resource name from "portal-artifact" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "name" should be "Artifact API ${CTX:apiId}"

  Scenario: An artifact ZIP requires metadata
    Given I generate a unique resource name from "portal-artifact-metadata" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "missing-metadata" request to "/apis" as "publisher"
    Then the response status code should be 400

  Scenario: An artifact ZIP requires a definition
    Given I generate a unique resource name from "portal-artifact-definition" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "missing-definition" request to "/apis" as "publisher"
    Then the response status code should be 400

  Scenario: A publisher updates an API from an artifact ZIP
    Given I generate a unique resource name from "portal-artifact-update" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" artifact upload "valid" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200

  Scenario: A publisher preserves the API identity when updating the same artifact ZIP
    Given I generate a unique resource name from "portal-artifact-stable" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "stable-create" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" artifact upload "stable-update" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:apiId}"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:apiId}"
    And the JSON response field "description" should be "artifact upload updated"

  Scenario: An artifact ZIP cannot escape its extraction root
    Given I generate a unique resource name from "portal-artifact-slip" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "zip-slip" request to "/apis" as "publisher"
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

  Scenario: An artifact ZIP cannot exceed the entry limit
    Given I generate a unique resource name from "portal-artifact-many" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "too-many-entries" request to "/apis" as "publisher"
    Then the response status code should be 400

  Scenario: An artifact upload cannot exceed the multipart size limit
    Given I generate a unique resource name from "portal-artifact-large" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "oversized" request to "/apis" as "publisher"
    Then the response status code should be 413

  Scenario: An artifact create without labels receives the default label
    Given I generate a unique resource name from "portal-artifact-default-label" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 1 item
    And the response body should contain "default"

  Scenario: An artifact create with an empty label list has no labels
    Given I generate a unique resource name from "portal-artifact-empty-label" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "labels-empty" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 0 items

  Scenario: An artifact update without labels preserves existing labels
    Given I generate a unique resource name from "portal-artifact-preserve-label" and store it as "apiId"
    And I generate a unique resource name from "portal-artifact-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Artifact Label"}
      """
    When I send an authenticated API Portal "POST" artifact upload "valid-with-label" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" artifact upload "valid" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 1 item
    And the response body should contain "${CTX:labelId}"

  Scenario: An artifact update with an empty label list clears existing labels
    Given I generate a unique resource name from "portal-artifact-clear-label" and store it as "apiId"
    And I generate a unique resource name from "portal-artifact-clear-label-value" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Cleared Artifact Label"}
      """
    When I send an authenticated API Portal "POST" artifact upload "valid-with-label" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" artifact upload "labels-update-empty" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until the top-level JSON response array field "labels" has 0 items
    Then the response status code should be 200
    And the JSON response array field "labels" should have 0 items
