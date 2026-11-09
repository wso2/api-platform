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


@APIPORTAL @apiportal-organization
Feature: The API Portal's own organization is managed through the API Portal REST API
  As an API Portal administrator
  I want to read and rename the organization the portal serves, and nothing else
  So that the organization cannot be created, removed, re-pointed or reached from another tenant

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-099..103, 198, 199 and 201,
  # at the API level. The portal serves exactly one organization ("default" in this suite), shared
  # by every runner in the block. An organization update replaces every settable field, so each
  # scenario that changes it first registers the organization's current settings to be written back
  # when it ends, pass or fail. Organizations that do not exist ("foreign-org") are refused before
  # any lookup, by design.

  @APIP-CORE-099
  Scenario: An administrator updates the organization's name and contacts, and its identity cannot change
    Given I generate a unique value from "Default organization" and store it as "organizationName"
    And I generate a unique resource name from "portal-org-view" and store it as "viewId"
    And the API Portal organization "default" is restored to its current settings after the scenario
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Organization View |
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    And the JSON response field "cpRefId" should be "default"

    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON values:
      | id                   | default                 |
      | idpRefId             | default                 |
      | displayName          | ${CTX:organizationName} |
      | businessOwner        | Organization Owner      |
      | businessOwnerContact | +94 11 000 0000         |
      | businessOwnerEmail   | owner@example.com       |
      | cpRefId              | default                 |
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:organizationName}"
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:organizationName}"
    And the JSON response field "businessOwner" should be "Organization Owner"
    And the JSON response field "businessOwnerContact" should be "+94 11 000 0000"
    And the JSON response field "businessOwnerEmail" should be "owner@example.com"
    And the JSON response field "cpRefId" should be "default"
    # A view with no llms.txt settings of its own titles its index with the organization's name.
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "# ${CTX:organizationName} API Portal"

    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON values:
      | id          | renamed-org             |
      | idpRefId    | default                 |
      | displayName | ${CTX:organizationName} |
      | cpRefId     | default                 |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON values:
      | id          | default                 |
      | idpRefId    | foreign-idp             |
      | displayName | ${CTX:organizationName} |
      | cpRefId     | default                 |
    Then the response status code should be 400
    When I send an authenticated API Portal "DELETE" request to "/organizations/default" as "admin"
    Then the response status code should be 405
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    And the JSON response field "idpRefId" should be "default"
    And the JSON response field "businessOwner" should be "Organization Owner"

  @APIP-CORE-100 @type:security
  Scenario: The organization collection cannot be listed or added to
    When I send an authenticated API Portal "GET" request to "/organizations" as "admin"
    Then the response status code should be 405
    And the JSON response field "code" should be "METHOD_NOT_ALLOWED"
    When I send an authenticated API Portal "POST" request to "/organizations" as "admin" with JSON values:
      | id          | new-org          |
      | displayName | New Organization |
      | idpRefId    | new-org          |
    Then the response status code should be 405
    And the JSON response field "code" should be "METHOD_NOT_ALLOWED"
    When I send an authenticated API Portal "POST" request to "/organizations" as "developer" with JSON values:
      | id          | developer-org          |
      | displayName | Developer Organization |
      | idpRefId    | developer-org          |
    Then the response status code should be 403
    When I send an unauthenticated API Portal "GET" request to "/organizations"
    Then the response status code should be 401

  @APIP-CORE-101 @type:security
  Scenario: Another organization cannot be read or changed
    When I send an authenticated API Portal "GET" request to "/organizations/foreign-org" as "admin"
    Then the response status code should be 403
    When I send an authenticated API Portal "PUT" request to "/organizations/foreign-org" as "admin" with JSON values:
      | id          | foreign-org |
      | idpRefId    | foreign-org |
      | displayName | Hijacked    |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    And the response body should not contain "Hijacked"

  @APIP-CORE-102 @type:security
  Scenario Outline: A <role> can read the organization but not change it
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "<role>"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "<role>" with JSON values:
      | id          | default            |
      | idpRefId    | default            |
      | displayName | Changed by <role>  |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response body should not contain "Changed by <role>"

    Examples:
      | role      |
      | publisher |
      | developer |

  @APIP-CORE-103 @type:negative
  Scenario: An organization update without its required fields is refused
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON values:
      | displayName | Incomplete Organization |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the response body should not contain "Incomplete Organization"

  @APIP-CORE-198
  Scenario: An Organization YAML file updates the organization and upserts its labels by id and views by id
    Given I generate a unique value from "Manifest organization" and store it as "organizationName"
    And I generate a unique resource name from "portal-manifest-label" and store it as "existingLabel"
    And I generate a unique resource name from "portal-manifest-new-label" and store it as "newLabel"
    And I generate a unique resource name from "portal-manifest-view" and store it as "viewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "existingLabel":
      | id          | ${CTX:existingLabel} |
      | displayName | Original Label       |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}           |
      | displayName | Original View           |
      | labels      | ["${CTX:existingLabel}"] |
    And I register API Portal resource "${CTX:newLabel}" at "/labels" for cleanup
    And the API Portal organization "default" is restored to its current settings after the scenario
    When I send an authenticated API Portal "PUT" multipart request to "/organizations/default" as "admin" with file field "organization" from "resources/templates/api-portal-organization.yaml" with values:
      | name                    | default                    |
      | displayName             | ${CTX:organizationName}    |
      | spec.idpRefId           | default                    |
      | spec.cpRefId            | default                    |
      | spec.businessOwner      | Manifest Owner             |
      | spec.businessOwnerEmail | manifest-owner@example.com |
      | spec.configuration      | {}                         |
      | spec.labels             | [{"id": "${CTX:existingLabel}", "displayName": "Renamed Label"}, {"id": "${CTX:newLabel}", "displayName": "New Label"}] |
      | spec.views              | [{"id": "${CTX:viewId}", "displayName": "Renamed View", "labels": ["${CTX:existingLabel}", "${CTX:newLabel}"]}] |
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:organizationName}"
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    And the JSON response field "displayName" should be "${CTX:organizationName}"
    And the JSON response field "businessOwner" should be "Manifest Owner"
    And the JSON response field "businessOwnerEmail" should be "manifest-owner@example.com"
    And the JSON response field "cpRefId" should be "default"
    # The existing label is updated in place, the new one is created.
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:existingLabel}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed Label"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:newLabel}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "New Label"
    # The view keeps its id, takes the new name, and its labels are replaced by the manifest's.
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed View"
    And the JSON response array field "labels" should have 2 items
    And the response body should contain "${CTX:existingLabel}"
    And the response body should contain "${CTX:newLabel}"

  @APIP-CORE-199 @type:negative
  Scenario Outline: An Organization YAML file with <problem> is refused and changes nothing
    Given I generate a unique value from "Refused organization" and store it as "organizationName"
    And the API Portal organization "default" is restored to its current settings after the scenario
    When I send an authenticated API Portal "PUT" multipart request to "/organizations/default" as "admin" with file field "organization" from "resources/templates/api-portal-organization.yaml" with values:
      | kind        | <kind>                  |
      | name        | <handle>                |
      | displayName | ${CTX:organizationName} |
      | spec        | <spec>                  |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    And the response body should not contain "${CTX:organizationName}"

    Examples:
      | problem                 | kind         | handle      | spec                                                                                                  |
      | another kind            | Application  | default     | {"displayName": "${CTX:organizationName}", "idpRefId": "default", "cpRefId": "default"}               |
      | another handle          | Organization | renamed-org | {"displayName": "${CTX:organizationName}", "idpRefId": "default", "cpRefId": "default"}               |
      | another IDP reference   | Organization | default     | {"displayName": "${CTX:organizationName}", "idpRefId": "foreign-idp", "cpRefId": "default"}           |
      | no display name         | Organization | default     | {"idpRefId": "default", "cpRefId": "default", "businessOwner": "${CTX:organizationName}"}             |
      | a spec that is no map   | Organization | default     | "${CTX:organizationName}"                                                                             |
      | a label without an id   | Organization | default     | {"displayName": "${CTX:organizationName}", "idpRefId": "default", "labels": [{"displayName": "L"}]}   |
      | labels that are no list | Organization | default     | {"displayName": "${CTX:organizationName}", "idpRefId": "default", "labels": "label"}                  |
      | a view without an id    | Organization | default     | {"displayName": "${CTX:organizationName}", "idpRefId": "default", "views": [{"displayName": "V"}]}    |

  @APIP-CORE-201 @type:security
  Scenario Outline: A <role> cannot update the organization with a YAML file
    Given the API Portal organization "default" is restored to its current settings after the scenario
    When I send an authenticated API Portal "PUT" multipart request to "/organizations/default" as "<role>" with file field "organization" from "resources/templates/api-portal-organization.yaml" with values:
      | name          | default           |
      | displayName   | Changed by <role> |
      | spec.idpRefId | default           |
      | spec.cpRefId  | default           |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the response body should not contain "Changed by <role>"

    Examples:
      | role      |
      | publisher |
      | developer |
