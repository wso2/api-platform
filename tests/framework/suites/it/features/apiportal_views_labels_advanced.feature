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

Feature: API Portal view and label resolution

  Scenario: An administrator lists labels for the organization
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Listed Label"}
      """
    When I send an authenticated API Portal "GET" request to "/labels" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:labelId}"

  Scenario: An administrator updates the labels associated with a view
    Given I generate a unique resource name from "portal-label" and store it as "firstLabel"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "firstLabel":
      """
      {"id":"${CTX:firstLabel}","displayName":"First Label"}
      """
    And I generate a unique resource name from "portal-label" and store it as "secondLabel"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "secondLabel":
      """
      {"id":"${CTX:secondLabel}","displayName":"Second Label"}
      """
    And I generate a unique resource name from "portal-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Original View","labels":["${CTX:firstLabel}"]}
      """
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON body:
      """
      {"displayName":"Updated View","labels":["${CTX:secondLabel}"]}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Updated View"
    And the JSON response array field "labels" should have 1 item

  Scenario: Renaming a view preserves its label associations
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Attached Label"}
      """
    And I generate a unique resource name from "portal-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Renameable View","labels":["${CTX:labelId}"]}
      """
    And I generate a unique resource name from "portal-renamed-view" and store it as "renamedViewId"
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON body:
      """
      {"id":"${CTX:renamedViewId}","displayName":"Renameable View"}
      """
    Then the response status code should be 200
    And I register API Portal resource "${CTX:renamedViewId}" at "/views" for cleanup
    When I send an authenticated API Portal "GET" request to "/views/${CTX:renamedViewId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 1 item
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404

  Scenario: An administrator cannot rename a view onto an existing handle
    Given I generate a unique resource name from "portal-view" and store it as "firstView"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "firstView":
      """
      {"id":"${CTX:firstView}","displayName":"First View"}
      """
    And I generate a unique resource name from "portal-view" and store it as "secondView"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "secondView":
      """
      {"id":"${CTX:secondView}","displayName":"Second View"}
      """
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:secondView}" as "admin" with JSON body:
      """
      {"id":"${CTX:firstView}","displayName":"Second View"}
      """
    Then the response status code should be 409

  Scenario: A view filter returns only APIs carrying an included label
    Given I generate a unique resource name from "portal-label" and store it as "includedLabel"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "includedLabel":
      """
      {"id":"${CTX:includedLabel}","displayName":"Included Label"}
      """
    And I generate a unique resource name from "portal-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Filtered View","labels":["${CTX:includedLabel}"]}
      """
    And I generate a unique resource name from "portal-view-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Filtered API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:includedLabel}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
