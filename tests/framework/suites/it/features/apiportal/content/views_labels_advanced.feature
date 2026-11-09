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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API
  # level. Views and labels are administrator resources; every refused change is followed by a read
  # that shows the view is unchanged.

  @APIP-010 @type:negative @type:security
  Scenario: An administrator manages a view's labels and handle, within its deletion and access rules
    # An administrator creates views with and without labels
    Given I generate a unique resource name from "portal-view-label" and store it as "labelId"
    And I generate a unique resource name from "portal-labelled-view" and store it as "labelledViewId"
    And I generate a unique resource name from "portal-unlabelled-view" and store it as "unlabelledViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Partner APIs   |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "labelledViewId":
      | id          | ${CTX:labelledViewId} |
      | displayName | Partner View          |
      | labels      | ["${CTX:labelId}"]    |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "unlabelledViewId":
      | id          | ${CTX:unlabelledViewId} |
      | displayName | Empty View              |
    When I send an authenticated API Portal "GET" request to "/views/${CTX:labelledViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Partner View"
    And the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:labelId}"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:unlabelledViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 0 items

    # An administrator relabels and renames a view, and cannot rename it onto another view
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

    # A view is addressed by its handle, never by its display name
    Given I generate a unique resource name from "portal-handle-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}          |
      | displayName | Handle Resolution View |
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Handle Resolution View"
    When I send an authenticated API Portal "GET" request to "/views/Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis?view=Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/mcp-servers?view=${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers?view=Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/views/Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200

    # An administrator deletes a view, and deleting it again is not found
    Given I generate a unique resource name from "portal-deletable-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}  |
      | displayName | Deletable View |
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/default" as "admin"
    Then the response status code should be 200

    # A view that still has API workflows cannot be deleted until they are removed
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

    # A duplicate view, a view with an unknown label and a developer's change are refused
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

    # A developer can read labels and views but cannot create them
    Given I generate a unique resource name from "portal-readable-label" and store it as "labelId"
    And I generate a unique resource name from "portal-readable-view" and store it as "viewId"
    And I generate a unique resource name from "portal-denied-label" and store it as "deniedLabelId"
    And I generate a unique resource name from "portal-denied-view" and store it as "deniedViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Public APIs    |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Public View   |
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/labels" as "developer" with JSON values:
      | id          | ${CTX:deniedLabelId} |
      | displayName | Denied Label         |
    Then the response status code should be 403
    When I send an authenticated API Portal "POST" request to "/views" as "developer" with JSON values:
      | id          | ${CTX:deniedViewId} |
      | displayName | Denied View         |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:deniedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:deniedViewId}" as "admin"
    Then the response status code should be 404

  @APIP-010 @type:negative @known-issue
  Scenario: A view whose handle is invalid is refused and stores nothing
    # A view whose handle is not lowercase is refused and stores nothing
    # The portal stores a view handle as sent: one that is not lowercase, or holds a space or a slash,
    # is accepted (201), although the docs define handles as lowercase and a slash makes the view's
    # pages unreachable.
    Given I generate a unique resource name from "portal-bad-handle" and store it as "viewId"
    And I register API Portal resource "Upper-${CTX:viewId}" at "/views" for cleanup
    When I send an authenticated API Portal "POST" request to "/views" as "admin" with JSON values:
      | id          | Upper-${CTX:viewId}      |
      | displayName | Refused View  |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/Upper-${CTX:viewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/default" as "admin"
    Then the response status code should be 200

    # A view whose handle is containing a space is refused and stores nothing
    # The portal stores a view handle as sent: one that is not lowercase, or holds a space or a slash,
    # is accepted (201), although the docs define handles as lowercase and a slash makes the view's
    # pages unreachable.
    Given I generate a unique resource name from "portal-bad-handle" and store it as "viewId"
    And I register API Portal resource "${CTX:viewId} spaced" at "/views" for cleanup
    When I send an authenticated API Portal "POST" request to "/views" as "admin" with JSON values:
      | id          | ${CTX:viewId} spaced      |
      | displayName | Refused View  |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}%20spaced" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/default" as "admin"
    Then the response status code should be 200

    # A view whose handle is containing a slash is refused and stores nothing
    # The portal stores a view handle as sent: one that is not lowercase, or holds a space or a slash,
    # is accepted (201), although the docs define handles as lowercase and a slash makes the view's
    # pages unreachable.
    Given I generate a unique resource name from "portal-bad-handle" and store it as "viewId"
    And I register API Portal resource "${CTX:viewId}/apis" at "/views" for cleanup
    When I send an authenticated API Portal "POST" request to "/views" as "admin" with JSON values:
      | id          | ${CTX:viewId}/apis      |
      | displayName | Refused View  |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}%2Fapis" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/default" as "admin"
    Then the response status code should be 200

  @APIP-011 @type:negative
  Scenario: An administrator manages a label, and deleting it takes it off every API and view
    # An administrator manages a label from creation to deletion
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Premium APIs   |
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Premium APIs"
    When I send an authenticated API Portal "GET" request to "/labels" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:labelId}"
    When I send an authenticated API Portal "PUT" request to "/labels/${CTX:labelId}" as "admin" with JSON values:
      | id          | ${CTX:labelId} |
      | displayName | Partner APIs   |
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Partner APIs"
    When I send an authenticated API Portal "DELETE" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404

    # Deleting a label takes it off every API and view that carried it
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

    # A label that repeats an existing handle is refused and stores nothing
    Given I generate a unique resource name from "portal-guarded-label" and store it as "labelId"
    And I generate a unique resource name from "portal-refused-label" and store it as "refusedLabelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Guarded Label  |
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON body:
      """
      {"id": "${CTX:labelId}", "displayName": "Duplicate Label"}
      """
    Then the response status code should be 409
    And the response body should not contain "constraint"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:refusedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Guarded Label"

    # A label without a display name is refused and stores nothing
    Given I generate a unique resource name from "portal-guarded-label" and store it as "labelId"
    And I generate a unique resource name from "portal-refused-label" and store it as "refusedLabelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Guarded Label  |
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON body:
      """
      {"id": "${CTX:refusedLabelId}"}
      """
    Then the response status code should be 400
    And the response body should not contain "constraint"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:refusedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Guarded Label"

    # A label without a handle is refused and stores nothing
    Given I generate a unique resource name from "portal-guarded-label" and store it as "labelId"
    And I generate a unique resource name from "portal-refused-label" and store it as "refusedLabelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Guarded Label  |
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON body:
      """
      {"displayName": "Nameless Label"}
      """
    Then the response status code should be 400
    And the response body should not contain "constraint"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:refusedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Guarded Label"

    # A label with a field it does not know is refused and stores nothing
    Given I generate a unique resource name from "portal-guarded-label" and store it as "labelId"
    And I generate a unique resource name from "portal-refused-label" and store it as "refusedLabelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Guarded Label  |
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON body:
      """
      {"id": "${CTX:refusedLabelId}", "displayName": "X", "x": 1}
      """
    Then the response status code should be 400
    And the response body should not contain "constraint"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:refusedLabelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Guarded Label"
