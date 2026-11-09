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

@APIPORTAL @apiportal-artifact-upload
Feature: APIs are created and updated from artifact ZIPs through the API Portal REST API
  As an API publisher
  I want to create and update an API by uploading one artifact ZIP with its metadata and definition
  So that a GitOps pipeline can publish an API in a single request, safely

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level.
  # Each artifact fixture is built by the "artifact upload" step from the scenario's "apiId" and
  # carries that id in its metadata. Unsafe uploads (a zip-slip or absolute entry, too many entries,
  # an oversized body, an entry that inflates past the limit, a file that is not a ZIP) and invalid
  # metadata (an MCP type, an unknown status) are refused before anything is stored, and the refusal
  # names no server internals.

  @APIP-005
  Scenario: An artifact ZIP creates and updates an API, keeps its identity and applies its labels
    # A publisher creates an API from an artifact ZIP and updates it from another
    Given I generate a unique resource name from "portal-artifact" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "name" should be "Artifact API ${CTX:apiId}"
    And the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "default"
    When I send an authenticated API Portal "PUT" artifact upload "valid" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200

    # Updating an API from an artifact keeps its identity
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

    # An artifact's labels are kept unless the artifact sets them
    Given I generate a unique resource name from "portal-artifact-labelled" and store it as "apiId"
    And I generate a unique resource name from "portal-artifact-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Artifact Label |
    When I send an authenticated API Portal "POST" artifact upload "valid-with-label" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "PUT" artifact upload "valid" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:labelId}"
    When I send an authenticated API Portal "PUT" artifact upload "labels-update-empty" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until the top-level JSON response array field "labels" has 0 items
    Then the JSON response array field "labels" should have 0 items

    # An artifact created with an empty label list has no labels
    Given I generate a unique resource name from "portal-artifact-unlabelled" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "labels-empty" request to "/apis" as "publisher"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response array field "labels" should have 0 items

  @APIP-005 @type:security
  Scenario: Unsafe artifacts are refused, and only a publisher can upload to an API that exists
    # An artifact without metadata is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "missing-metadata" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact without a definition is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "missing-definition" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact with an entry outside its extraction root is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "zip-slip" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact with more entries than allowed is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "too-many-entries" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact larger than the upload limit is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "oversized" request to "/apis" as "publisher"
    Then the response status code should be 413
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact with an absolute entry path is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "absolute-path" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact that is not a ZIP archive is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "not-a-zip" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact for an MCP server is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "mcp-type" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # An artifact with an invalid status is refused and stores nothing
    Given I generate a unique resource name from "portal-artifact-refused" and store it as "apiId"
    When I send an authenticated API Portal "POST" artifact upload "invalid-status" request to "/apis" as "publisher"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "apip-absolute-entry"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201

    # Only a publisher can upload artifacts, and only to an API that exists
    Given I generate a unique resource name from "portal-artifact-guarded" and store it as "apiId"
    And I generate a unique resource name from "portal-artifact-missing" and store it as "missingId"
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "PUT" artifact upload "valid" request to "/apis/${CTX:missingId}" as "publisher"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" artifact upload "valid" request to "/apis" as "publisher"
    Then the response status code should be 201
