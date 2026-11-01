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

@platform-api-publication-drafts
Feature: API publication drafts are prepared and validated before anything reaches a portal
  As an API publisher
  I want a publication draft to keep what I prepared, to be validated, and to stay intact when
  the API Portal cannot be reached
  So that publishing is safe to retry on every supported database

  # The registered portal's address never resolves, so every call that would leave the control
  # plane is observably refused, while everything the control plane decides alone still runs.
  Background:
    Given I generate a unique resource name from "pub-proj" and store it as "project"
    And I generate a unique resource name from "pub-plan" and store it as "plan"
    And I generate a unique resource name from "pub-api" and store it as "apiName"
    And I generate a unique API context from "/pub-api" and store it as "apiContext"
    And I generate a unique resource name from "pub-portal" and store it as "portal"
    And I create a project "${CTX:project}" on the control plane
    And I create a subscription plan "${CTX:plan}" allowing 10 requests per minute via the control plane
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": ["${CTX:plan}"]}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "api"
    When I send a "POST" request to the control plane at "/api-portals" with body:
      """
      {"name": "${CTX:portal}", "handle": "${CTX:portal}", "url": "https://${CTX:portal}.example.invalid", "sharedKey": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
      """
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/api-portals/${CTX:portal}"

  Scenario: A saved draft round-trips its details, definition and sanitized landing page
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft"
    Then the response status code should be 404
    And the JSON response field "code" should be "DRAFT_NOT_FOUND"
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft" with body:
      """
      {"displayName": "${CTX:apiName} listing", "version": "v1", "subscriptionPlanIds": ["${CTX:plan}"], "endpoints": {"productionUrl": "http://backend.example.com"}}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiName} listing"
    And the JSON response field "subscriptionPlans[0].id" should be "${CTX:plan}"
    And the JSON response field "hasLandingPage" should be false
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft/definition" with body:
      """
      {"openapi": "3.0.3", "info": {"title": "${CTX:apiName}", "version": "v1"}, "paths": {"/health": {"get": {"responses": {"200": {"description": "ok"}}}}}}
      """
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft/definition"
    Then the response status code should be 200
    And the JSON response field "info.title" should be "${CTX:apiName}"
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft/landing-page" with header "Content-Type" set to "text/markdown" with body:
      """
      # Welcome

      <script>alert(1)</script>

      Use this API.
      """
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft/landing-page"
    Then the response status code should be 200
    And the response body should contain "Use this API."
    And the response body should not contain "<script>"
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft" with body:
      """
      {"displayName": "${CTX:apiName} renamed", "version": "v1", "subscriptionPlanIds": []}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiName} renamed"
    And the JSON response array field "subscriptionPlans" should have 0 items
    And the JSON response field "hasLandingPage" should be true

  Scenario Outline: A draft naming something that does not exist is rejected
    When I send a "<method>" request to the control plane at "<path>" with body:
      """
      {"displayName": "${CTX:apiName}", "version": "v1", "subscriptionPlanIds": [<plans>]}
      """
    Then the response status code should be <status>
    And the JSON response field "code" should be "<code>"

    Examples:
      | method | path                                                               | plans                        | status | code                          |
      | PUT    | /api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft          | "${CTX:plan}-missing"        | 400    | PUBLICATION_VALIDATION_FAILED |
      | PUT    | /api-portals/${CTX:portal}-missing/apis/rest-api/${CTX:api}/draft  | "${CTX:plan}"                | 404    | API_PORTAL_NOT_FOUND          |
      | PUT    | /api-portals/${CTX:portal}/apis/rest-api/${CTX:api}-missing/draft  | "${CTX:plan}"                | 404    | API_NOT_FOUND                 |

  Scenario: An inactive subscription plan cannot be offered, even when it was active at save time
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft" with body:
      """
      {"displayName": "${CTX:apiName}", "version": "v1", "subscriptionPlanIds": ["${CTX:plan}"]}
      """
    Then the response status code should be 200
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft/definition" with body:
      """
      {"openapi": "3.0.3", "info": {"title": "${CTX:apiName}", "version": "v1"}, "paths": {"/health": {"get": {"responses": {"200": {"description": "ok"}}}}}}
      """
    Then the response status code should be 204
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "${CTX:plan}", "status": "INACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "limitCount": 10}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/publish"
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the JSON response field "message" should contain "${CTX:plan}"
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft" with body:
      """
      {"displayName": "${CTX:apiName}", "version": "v1", "subscriptionPlanIds": ["${CTX:plan}"]}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlans[0].id" should be "${CTX:plan}"

  Scenario: Publishing needs a definition, and an unreachable portal leaves the draft unpublished
    When I send a "POST" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/publish"
    Then the response status code should be 404
    And the JSON response field "code" should be "DRAFT_NOT_FOUND"
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft" with body:
      """
      {"displayName": "${CTX:apiName}", "version": "v1", "subscriptionPlanIds": ["${CTX:plan}"]}
      """
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/publish"
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft/definition" with body:
      """
      {"openapi": "3.0.3", "info": {"title": "${CTX:apiName}", "version": "v1"}, "paths": {"/health": {"get": {"responses": {"200": {"description": "ok"}}}}}}
      """
    Then the response status code should be 204
    When I send a "POST" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/publish"
    Then the response status code should be 503
    And the JSON response field "code" should be "PUBLICATION_PORTAL_UNAVAILABLE"
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/publication"
    Then the response status code should be 404
    And the JSON response field "code" should be "PUBLICATION_NOT_FOUND"
    When I send a "GET" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/draft"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiName}"

  Scenario Outline: Only a live listing can be unpublished or deprecated
    When I send a "POST" request to the control plane at "/api-portals/${CTX:portal}/apis/rest-api/${CTX:api}/<action>"
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_STATE_CONFLICT"

    Examples:
      | action    |
      | unpublish |
      | deprecate |
