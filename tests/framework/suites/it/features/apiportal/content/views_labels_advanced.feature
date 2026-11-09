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


@APIPORTAL @apiportal-views-labels
Feature: Views are relabelled, renamed and protected through the API Portal REST API
  As an API Portal administrator
  I want to change which labels a view shows and rename it without losing them
  So that each audience's catalogue follows my changes, and no change can break another view

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-067..070 and 133..135, at the API
  # level. Views and labels are administrator resources; every refused change is followed by a read
  # that shows the view is unchanged.

  @APIP-CORE-067
  Scenario: An administrator relabels and renames a view, and cannot rename it onto another view
    Given I generate a unique resource name from "portal-first-label" and store it as "firstLabel"
    And I generate a unique resource name from "portal-second-label" and store it as "secondLabel"
    And I generate a unique resource name from "portal-view" and store it as "viewId"
    And I generate a unique resource name from "portal-renamed-view" and store it as "renamedViewId"
    And I generate a unique resource name from "portal-other-view" and store it as "otherViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "firstLabel":
      | id          | ${CTX:firstLabel} |
      | displayName | First Label |
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "secondLabel":
      | id          | ${CTX:secondLabel} |
      | displayName | Second Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}             |
      | displayName | Original View             |
      | labels      | ["${CTX:firstLabel}"]     |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "otherViewId":
      | id          | ${CTX:otherViewId} |
      | displayName | Other View         |
    When I send an authenticated API Portal "GET" request to "/labels" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:firstLabel}"

    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON values:
      | displayName | Updated View             |
      | labels      | ["${CTX:secondLabel}"]   |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the JSON response field "displayName" should be "Updated View"
    And the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:secondLabel}"

    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON values:
      | id          | ${CTX:renamedViewId} |
      | displayName | Updated View         |
    Then the response status code should be 200
    And I register API Portal resource "${CTX:renamedViewId}" at "/views" for cleanup
    When I send an authenticated API Portal "GET" request to "/views/${CTX:renamedViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "labels[0]" should be "${CTX:secondLabel}"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404

    When I send an authenticated API Portal "PUT" request to "/views/${CTX:otherViewId}" as "admin" with JSON values:
      | id          | ${CTX:renamedViewId} |
      | displayName | Other View           |
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/views/${CTX:otherViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Other View"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:renamedViewId}" as "admin"
    Then the JSON response field "displayName" should be "Updated View"

  @APIP-CORE-068
  Scenario: A view's API listing holds only the APIs that carry its label
    Given I generate a unique resource name from "portal-included-label" and store it as "includedLabel"
    And I generate a unique resource name from "portal-filtered-view" and store it as "viewId"
    And I generate a unique resource name from "portal-labelled-api" and store it as "apiId"
    And I generate a unique resource name from "portal-unlabelled-api" and store it as "otherApiId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "includedLabel":
      | id          | ${CTX:includedLabel} |
      | displayName | Included Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}            |
      | displayName | Filtered View            |
      | labels      | ["${CTX:includedLabel}"] |
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}             |
      | name   | ${CTX:apiId}             |
      | labels | ["${CTX:includedLabel}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:otherApiId} |
      | name   | ${CTX:otherApiId} |
      | labels | []                |
    And the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    And the response body should not contain "${CTX:otherApiId}"
    # Listing without a view covers the APIs every view shows, so an API with no labels is in none.
    When I send an authenticated API Portal "GET" request to "/apis" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    And the response body should not contain "${CTX:otherApiId}"
    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:otherApiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:otherApiId}"

  @APIP-CORE-069 @type:negative
  Scenario: A duplicate view, a view with an unknown label and a developer's change are refused
    Given I generate a unique resource name from "portal-guarded-view" and store it as "viewId"
    And I generate a unique resource name from "portal-unknown-label" and store it as "unknownLabel"
    And I generate a unique resource name from "portal-unlabelled-view" and store it as "unlabelledViewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Guarded View  |
    When I send an authenticated API Portal "POST" request to "/views" as "admin" with JSON values:
      | id          | ${CTX:viewId}  |
      | displayName | Duplicate View |
    Then the response status code should be 409
    And the response body should not contain "constraint"
    When I send an authenticated API Portal "POST" request to "/views" as "admin" with JSON values:
      | id          | ${CTX:unlabelledViewId} |
      | displayName | Unknown Label View      |
      | labels      | ["${CTX:unknownLabel}"] |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:unlabelledViewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "developer" with JSON values:
      | displayName | Changed by developer |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Guarded View"

  @APIP-CORE-070 @type:negative
  Scenario: A view that still has API workflows cannot be deleted until they are removed
    Given I generate a unique resource name from "portal-workflow-view" and store it as "viewId"
    And I generate a unique resource name from "portal-view-workflow" and store it as "workflowId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Workflow View |
    And a unique API Portal resource is created at "/views/${CTX:viewId}/api-workflows" as "publisher" with values and stored as "workflowId":
      | id                    | ${CTX:workflowId}        |
      | displayName           | Onboarding               |
      | description           | Guides a developer in    |
      | apiWorkflowDefinition | {"arazzo":"1.0.0","info":{"title":"Onboarding","version":"1.0.0"},"sourceDescriptions":[],"workflows":[]} |
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}/api-workflows/${CTX:workflowId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404

  @APIP-CORE-133
  Scenario: Deleting a label takes it off every API and view that carried it
    Given I generate a unique resource name from "portal-doomed-label" and store it as "labelId"
    And I generate a unique resource name from "portal-kept-label" and store it as "keptLabelId"
    And I generate a unique resource name from "portal-doomed-view" and store it as "viewId"
    And I generate a unique resource name from "portal-doomed-api" and store it as "apiId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Doomed Label   |
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "keptLabelId":
      | id          | ${CTX:keptLabelId} |
      | displayName | Kept Label         |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}                              |
      | displayName | Doomed Label View                          |
      | labels      | ["${CTX:labelId}", "${CTX:keptLabelId}"]   |
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "admin"
    Then the JSON response array "list" should contain an item with "id" equal to "${CTX:apiId}"

    When I send an authenticated API Portal "DELETE" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 0 items
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:keptLabelId}"
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:apiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 404

  @APIP-CORE-134 @type:negative
  Scenario Outline: A label <problem> is refused and stores nothing
    Given I generate a unique resource name from "portal-guarded-label" and store it as "labelId"
    And I generate a unique resource name from "portal-refused-label" and store it as "refusedLabelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Guarded Label  |
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON body:
      """
      <body>
      """
    Then the response status code should be <status>
    And the response body should not contain "constraint"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:refusedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Guarded Label"

    Examples:
      | problem                          | body                                                          | status |
      | that repeats an existing handle  | {"id": "${CTX:labelId}", "displayName": "Duplicate Label"}    | 409    |
      | without a display name           | {"id": "${CTX:refusedLabelId}"}                               | 400    |
      | without a handle                 | {"displayName": "Nameless Label"}                             | 400    |
      | with a field it does not know    | {"id": "${CTX:refusedLabelId}", "displayName": "X", "x": 1}   | 400    |

  # The portal stores a view handle as sent: one that is not lowercase, or holds a space or a slash,
  # is accepted (201), although the docs define handles as lowercase and a slash makes the view's
  # pages unreachable.
  @APIP-CORE-135 @type:negative @known-issue
  Scenario Outline: A view whose handle is <problem> is refused and stores nothing
    Given I generate a unique resource name from "portal-bad-handle" and store it as "viewId"
    And I register API Portal resource "<handle>" at "/views" for cleanup
    When I send an authenticated API Portal "POST" request to "/views" as "admin" with JSON values:
      | id          | <handle>      |
      | displayName | Refused View  |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/<encoded handle>" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/default" as "admin"
    Then the response status code should be 200

    Examples:
      | problem              | handle                 | encoded handle            |
      | not lowercase        | Upper-${CTX:viewId}    | Upper-${CTX:viewId}       |
      | containing a space   | ${CTX:viewId} spaced   | ${CTX:viewId}%20spaced    |
      | containing a slash   | ${CTX:viewId}/apis     | ${CTX:viewId}%2Fapis      |
