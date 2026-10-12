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

Feature: API Portal organization content

  Scenario: An administrator applies a theme and retrieves an image asset
    Given I generate a unique resource name from "portal-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image/png"
    And the API Portal response body should equal hex "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"

  Scenario: An administrator applies a favicon theme
    Given I generate a unique resource name from "portal-favicon-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Favicon View"}
      """
    When I upload API Portal theme "favicon" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=favicon.ico" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image/x-icon"
    And the API Portal response body should equal hex "00000100010010100000010020002801000016000000"

  Scenario: An administrator retrieves a themed stylesheet
    Given I generate a unique resource name from "portal-style-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Style View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=style&fileName=main.css" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/css"
    And the response body should contain "#123456"

  Scenario: Reapplying a theme replaces its previous assets
    Given I generate a unique resource name from "portal-reapply-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Reapply View"}
      """
    When I upload API Portal theme "old" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I upload API Portal theme "new" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=old.png" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 200

  Scenario: Theme upload rejects unsupported file types
    Given I generate a unique resource name from "portal-invalid-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Invalid Theme View"}
      """
    When I upload API Portal theme "unsupported" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 400

  Scenario: An anonymous caller can retrieve a themed image
    Given I generate a unique resource name from "portal-public-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Public Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image/png"

  Scenario: A foreign organization identifier does not alter public asset resolution
    Given I generate a unique resource name from "portal-foreign-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Foreign Identifier View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png&orgId=00000000-0000-0000-0000-000000000000"
    Then the response status code should be 200

  Scenario: A missing themed asset falls back to a not-found response
    Given I generate a unique resource name from "portal-fallback-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Fallback Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=no-such-asset.png"
    Then the response status code should be 404

  Scenario: An administrator exports the applied theme
    Given I generate a unique resource name from "portal-export-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Export Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/export-theme" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/zip"

  Scenario: An administrator resets a theme to defaults
    Given I generate a unique resource name from "portal-reset-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Reset Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/reset-theme" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 404

  Scenario: A publisher cannot apply organization content
    Given I generate a unique resource name from "portal-publisher-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Publisher Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "publisher"
    Then the response status code should be 403

  Scenario: A developer cannot apply organization content
    Given I generate a unique resource name from "portal-developer-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Developer Theme View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "developer"
    Then the response status code should be 403

  Scenario: A developer cannot reset organization content
    Given I generate a unique resource name from "portal-developer-reset-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Developer Reset View"}
      """
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/reset-theme" as "developer"
    Then the response status code should be 403
