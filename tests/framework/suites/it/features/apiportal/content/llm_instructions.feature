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

@APIPORTAL @apiportal-llm-instructions
Feature: A view's LLM instructions and AI discovery shape what agents read
  As an API Portal administrator
  I want to name and describe each view for AI agents, preview the result and switch discovery per view
  So that agents read an accurate llms.txt, and a view I keep from agents exposes nothing to them

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-139..141. LLM instructions are a
  # view's settings page data: they are read and written with an administrator's portal session and
  # its CSRF token at /api-portal/{org}/views/{view}/llms-config, and previewed at
  # /api-portal/{org}/views/{view}/llms.txt/preview. Every scenario uses its own views, so no other
  # runner's view changes. Every page hidden from agents is paired with the same page in a view that
  # still serves it.

  @APIP-CORE-139
  Scenario: LLM instructions name and describe a view in its llms.txt, and empty fields fall back
    Given I generate a unique resource name from "portal-llm-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | LLM View      |
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "API Portal"
    And the response body should contain "Empower your development with our APIs"

    When I send an API Portal session "PUT" request to "/api-portal/default/views/${CTX:viewId}/llms-config" as "admin" with JSON values:
      | aiEnabled         | true                                 |
      | portalName        | Partner <Portal>                     |
      | portalDescription | Partner APIs for settlement ledgers. |
    Then the response status code should be 200
    When I send an API Portal session "GET" request to "/api-portal/default/views/${CTX:viewId}/llms-config" as "admin"
    Then the response status code should be 200
    And the JSON response field "portalName" should be "Partner Portal"
    And the JSON response field "portalDescription" should be "Partner APIs for settlement ledgers."
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "# Partner Portal"
    And the response body should contain "Partner APIs for settlement ledgers."
    And the response body should not contain "<Portal>"
    And the response body should not contain "Empower your development with our APIs"

    When I send an API Portal session "PUT" request to "/api-portal/default/views/${CTX:viewId}/llms-config" as "admin" with JSON values:
      | aiEnabled         | true |
      | portalName        |      |
      | portalDescription |      |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "Partner Portal"
    And the response body should contain "API Portal"
    And the response body should contain "Empower your development with our APIs"

  @APIP-CORE-140 @type:security
  Scenario: The llms.txt preview renders draft instructions without saving them, for administrators only
    Given I generate a unique resource name from "portal-preview-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId} |
      | displayName | Preview View  |
    When I send an API Portal session "POST" request to "/api-portal/default/views/${CTX:viewId}/llms.txt/preview" as "admin" with JSON values:
      | portalName        | Draft Portal Name         |
      | portalDescription | Draft portal description. |
    Then the response status code should be 200
    And the response header "Content-Type" should contain "text/plain"
    And the response body should contain "# Draft Portal Name"
    And the response body should contain "Draft portal description."
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "Draft Portal Name"

    When I send an API Portal session "POST" request to "/api-portal/default/views/${CTX:viewId}/llms.txt/preview" as "developer" with JSON values:
      | portalName | Developer Draft |
    Then the response status code should be 403
    And the response body should not contain "Developer Draft"
    When I send an API Portal session "PUT" request to "/api-portal/default/views/${CTX:viewId}/llms-config" as "developer" with JSON values:
      | aiEnabled  | false           |
      | portalName | Developer Draft |
    Then the response status code should be 403
    When I send an unauthenticated API Portal "POST" request to the root path "/api-portal/default/views/${CTX:viewId}/llms.txt/preview"
    Then the response status code should be 302
    And the response header "Location" should contain "/login"
    And the response body should not contain "API Portal"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "Developer Draft"

  @APIP-CORE-141
  Scenario: Turning AI discovery off for one view hides it from agents while other views keep serving them
    Given I generate a unique resource name from "portal-ai-label" and store it as "labelId"
    And I generate a unique resource name from "portal-ai-off-view" and store it as "viewId"
    And I generate a unique resource name from "portal-ai-on-view" and store it as "otherViewId"
    And I generate a unique resource name from "portal-ai-api" and store it as "apiId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | AI Label       |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | AI Off View        |
      | labels      | ["${CTX:labelId}"] |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "otherViewId":
      | id          | ${CTX:otherViewId} |
      | displayName | AI On View         |
      | labels      | ["${CTX:labelId}"] |
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "dp-badge--ai"

    When I send an API Portal session "PUT" request to "/api-portal/default/views/${CTX:viewId}/llms-config" as "admin" with JSON values:
      | aiEnabled         | false |
      | portalName        |       |
      | portalDescription |       |
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/llms.txt"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/apis.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should not contain "dp-badge--ai"

    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherViewId}/llms.txt"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherViewId}/apis.md"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherViewId}/api/${CTX:apiId}.md"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherViewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherViewId}/api/${CTX:apiId}"
    Then the response status code should be 200
    And the response body should contain "dp-badge--ai"
