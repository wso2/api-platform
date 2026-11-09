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


@APIPORTAL @apiportal-theme
Feature: A view's theme is applied, exported and reset through the API Portal REST API
  As an API Portal administrator
  I want to give a view its own theme, download it and return it to the default
  So that each audience sees a branded portal, and only administrators can change how it looks

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API
  # level. A theme archive holds one wrapper folder; only templates, stylesheets and images are kept.
  # Each scenario themes its own view. Themed images are public. Every refused change is followed
  # by a read showing the view's theme is unchanged.

  @APIP-012 @type:smoke
  Scenario: An administrator applies, exports and resets a theme on one view
    # An administrator applies a theme, exports it, replaces it and resets it
    Given I generate a unique resource name from "portal-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Themed View   |
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image/png"
    And the API Portal response body should equal hex "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=style&fileName=main.css" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/css"
    And the response body should contain "#123456"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/export-theme" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/zip"
    And the response body should contain "brand-mark.png"

    When I upload API Portal theme "old" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I upload API Portal theme "new" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=old.png" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 200

    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/reset-theme" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=style&fileName=main.css" as "admin"
    Then the response status code should be 200
    And the response body should not contain "#123456"

    # A favicon theme serves its icon
    Given I generate a unique resource name from "portal-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Themed View   |
    When I upload API Portal theme "favicon" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=favicon.ico" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image/x-icon"
    And the API Portal response body should equal hex "00000100010010100000010020002801000016000000"

    # Exporting a view with no theme of its own returns the default theme
    Given I generate a unique resource name from "portal-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Themed View   |
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/export-theme" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/zip"
    And the response body should not contain "brand-mark.png"

    # A theme applies only to the view it was uploaded for
    Given I generate a unique resource name from "portal-themed-view" and store it as "viewId"
    And I generate a unique resource name from "portal-plain-view" and store it as "otherViewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Themed View   |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "otherViewId":
      | id          | ${CTX:otherViewId} |
      | displayName | Plain View         |
    When I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:otherViewId}/asset?fileType=image&fileName=brand-mark.png"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:otherViewId}/asset?fileType=style&fileName=main.css" as "admin"
    Then the response body should not contain "#123456"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:otherViewId}/export-theme" as "admin"
    Then the response status code should be 200
    And the response body should not contain "brand-mark.png"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherViewId}"
    Then the response status code should be 200

  @APIP-012 @type:negative @type:security
  Scenario: Invalid themes are refused, scripts are dropped, and only an administrator can change a theme
    # A theme with an entry outside its extraction root is refused and the view keeps its current theme
    Given I generate a unique resource name from "portal-guarded-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}       |
      | displayName | Guarded Theme View  |
    And I upload API Portal theme "new" for view "${CTX:viewId}" as "admin"
    And the response status code should be 200
    When I upload API Portal theme "zip-slip" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "/tmp/"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 404

    # A theme larger than the upload limit is refused and the view keeps its current theme
    Given I generate a unique resource name from "portal-guarded-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}       |
      | displayName | Guarded Theme View  |
    And I upload API Portal theme "new" for view "${CTX:viewId}" as "admin"
    And the response status code should be 200
    When I upload API Portal theme "oversized" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 413
    And the response body should not contain "stack"
    And the response body should not contain "/tmp/"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 404

    # A theme that holds only scripts is refused and the view keeps its current theme
    Given I generate a unique resource name from "portal-guarded-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}       |
      | displayName | Guarded Theme View  |
    And I upload API Portal theme "new" for view "${CTX:viewId}" as "admin"
    And the response status code should be 200
    When I upload API Portal theme "scripts-only" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "/tmp/"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 404

    # A theme whose template embeds a script element is refused and the view keeps its current theme
    Given I generate a unique resource name from "portal-guarded-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}       |
      | displayName | Guarded Theme View  |
    And I upload API Portal theme "new" for view "${CTX:viewId}" as "admin"
    And the response status code should be 200
    When I upload API Portal theme "script-in-template" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "/tmp/"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 404

    # A theme's script files are never stored or served
    Given I generate a unique resource name from "portal-script-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}     |
      | displayName | Script Theme View |
    When I upload API Portal theme "with-script" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=style&fileName=main.css" as "admin"
    Then the response status code should be 200
    And the response body should contain "#123456"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/export-theme" as "admin"
    Then the response status code should be 200
    And the response body should contain "brand-mark.png"
    And the response body should not contain "app.js"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}"
    Then the response status code should be 200
    And the response body should not contain "window.themeScript"

    # Theme requests without theme files, for an unknown view, or without asset details are refused
    Given I generate a unique resource name from "portal-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Themed View   |
    And I generate a unique resource name from "portal-missing-view" and store it as "missingViewId"
    When I upload API Portal theme "unsupported" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 404
    When I upload API Portal theme "default" for view "${CTX:missingViewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:missingViewId}/export-theme" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image" as "admin"
    Then the response status code should be 400

    # Themed images are public, and only an administrator can change a theme
    Given I generate a unique resource name from "portal-theme-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Themed View   |
    And I upload API Portal theme "default" for view "${CTX:viewId}" as "admin"
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image/png"
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png&orgId=00000000-0000-0000-0000-000000000000"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=no-such-asset.png"
    Then the response status code should be 404

    When I upload API Portal theme "new" for view "${CTX:viewId}" as "publisher"
    Then the response status code should be 403
    When I upload API Portal theme "new" for view "${CTX:viewId}" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "POST" request to "/views/${CTX:viewId}/reset-theme" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=new.png" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}/asset?fileType=image&fileName=brand-mark.png" as "admin"
    Then the response status code should be 200
    And the API Portal response body should equal hex "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"
