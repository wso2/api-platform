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


@rest-api-versions
Feature: Listing the versions of a REST API through the control plane
  As an API platform operator
  I want GET /rest-apis/versions to report every version already taken by a display name
  So that a client can tell a brand-new API from a new version of an existing one before
  attempting the create

  # The over-the-wire half of this endpoint's coverage; the in-process half is
  # platform-api/internal/integration/api_versions_test.go, which this does not replace.
  # Not reproduced here: cross-org isolation (no second-org actor in this suite) and a version
  # stored without a context (create rejects an empty one). Both stay in the Go suite.

  Background:
    Given I generate a unique value from "papi-versions-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "reading-list" and store it as "displayName"

  @smoke
  Scenario: Every version of a display name is reported, in version order
    Given I generate a unique API context from "/papi-versions-v1" and store it as "contextV1"
    And I generate a unique API context from "/papi-versions-v2" and store it as "contextV2"
    And I generate a unique API context from "/papi-versions-v3" and store it as "contextV3"
    And I create a REST API "${CTX:displayName}-v1" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v1.0" and context "${CTX:contextV1}"
    And I create a REST API "${CTX:displayName}-v2" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v2.0" and context "${CTX:contextV2}"
    And I create a REST API "${CTX:displayName}-v3" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v3.0" and context "${CTX:contextV3}"

    When I send a "GET" request to the control plane at "/rest-apis/versions?displayName=${CTX:displayName}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:displayName}"
    And the JSON response field "versionsAvailable" should be true
    And the JSON response field "count" should be 3
    And the JSON response array field "list" should have 3 items

    # Asserted positionally, not as a set: an arbitrary order is a visible regression.
    And the JSON response field "list[0].version" should be "v1.0"
    And the JSON response field "list[1].version" should be "v2.0"
    And the JSON response field "list[2].version" should be "v3.0"

  Scenario: An unrelated API in the same project is not reported as a version
    Given I generate a unique resource name from "unrelated" and store it as "otherDisplayName"
    And I generate a unique API context from "/papi-versions-owned" and store it as "ownedContext"
    And I generate a unique API context from "/papi-versions-other" and store it as "otherContext"
    And I create a REST API "${CTX:displayName}-v1" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v1.0" and context "${CTX:ownedContext}"
    And I create a REST API "${CTX:otherDisplayName}-v1" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:otherDisplayName}" version "v1.0" and context "${CTX:otherContext}"

    When I send a "GET" request to the control plane at "/rest-apis/versions?displayName=${CTX:displayName}"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "list[0].id" should be "${CTX:displayName}-v1"
    And the response body should not contain "${CTX:otherDisplayName}"

  # projectId is documented as the project handle, not the UUID the row stores.
  Scenario: Each version reports the handle of the project it lives in
    Given I generate a unique API context from "/papi-versions-project-handle" and store it as "apiContext"
    And I create a REST API "${CTX:displayName}-v1" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v1.0" and context "${CTX:apiContext}"

    When I send a "GET" request to the control plane at "/rest-apis/versions?displayName=${CTX:displayName}"
    Then the response status code should be 200
    And the JSON response field "list[0].projectId" should be "${CTX:projectHandle}"
    And the JSON response field "list[0].context" should be "${CTX:apiContext}"
    And the JSON response field "list[0].lifeCycleStatus" should be "CREATED"
    And the JSON response should have field "list[0].createdAt"

  # A 404 here would make a client read "this name is free" as an error.
  Scenario: An unused display name is an empty result, not a 404
    Given I generate a unique resource name from "never-created" and store it as "missingDisplayName"
    When I send a "GET" request to the control plane at "/rest-apis/versions?displayName=${CTX:missingDisplayName}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:missingDisplayName}"
    And the JSON response field "versionsAvailable" should be false
    And the JSON response field "count" should be 0
    # "0 items" also rules out null - the step fails a field that is not an array.
    And the JSON response array field "list" should have 0 items

  Scenario: A missing displayName parameter is rejected rather than listing everything
    When I send a "GET" request to the control plane at "/rest-apis/versions"
    Then the response status code should be 400

  # Clients use this list to decide whether a create will be accepted, so the two endpoints
  # must agree. The Go suite proves the two SQL statements do.
  Scenario: What the list reports as taken is exactly what create refuses
    Given I generate a unique API context from "/papi-versions-agree-v1" and store it as "takenContext"
    And I generate a unique API context from "/papi-versions-agree-dup" and store it as "dupContext"
    And I generate a unique API context from "/papi-versions-agree-v2" and store it as "freeContext"
    And I create a REST API "${CTX:displayName}-v1" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v1.0" and context "${CTX:takenContext}"

    When I send a "GET" request to the control plane at "/rest-apis/versions?displayName=${CTX:displayName}"
    Then the JSON response array "list" should contain an item with "version" equal to "v1.0"
    And the JSON response array "list" should not contain an item with "version" equal to "v2.0"

    # Reported as taken: the create must conflict.
    When I attempt to create a REST API "${CTX:displayName}-dup" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v1.0" and context "${CTX:dupContext}"
    Then the response status code should be 409

    # Not reported: the create must succeed, and the list must then include it.
    When I attempt to create a REST API "${CTX:displayName}-v2" via the control plane in project "${CTX:projectHandle}" with display name "${CTX:displayName}" version "v2.0" and context "${CTX:freeContext}"
    Then the response status code should be 201

    When I send a "GET" request to the control plane at "/rest-apis/versions?displayName=${CTX:displayName}"
    Then the JSON response field "count" should be 2
    And the JSON response array "list" should contain an item with "version" equal to "v2.0"
