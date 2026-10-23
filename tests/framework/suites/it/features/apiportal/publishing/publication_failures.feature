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

@platform-api-gateway-api-portal
Feature: Publishing is refused, and explained, when the control plane or the API Portal cannot accept it
  As an API publisher
  I want a rejected publish, deprecate or unpublish to say why and to leave everything as it was
  So that I can fix the cause and repeat the action

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "publish-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "publish-api" and store it as "apiHandle"
    And I generate a unique API context from "/publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    And I send a "GET" request to "${CTX:apiContext}/" until status 401
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"

  @type:negative
  Scenario: Publishing without a saved draft is refused
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 404
    And the JSON response field "code" should be "DRAFT_NOT_FOUND"
    And the API Portal should not list the API "${CTX:apiHandle}"

  @type:negative
  Scenario: A draft cannot select a subscription plan that is not active
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    And I generate a unique resource name from "publish-inactive-plan" and store it as "inactivePlan"
    And I create an inactive subscription plan "${CTX:inactivePlan}" allowing 10 requests per hour via the control plane
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portalHandle}/apis/rest-api/${CTX:apiHandle}/draft" with body:
      """
      {"displayName": "${CTX:apiHandle}", "version": "${CTX:apiVersion}", "subscriptionPlanIds": ["${CTX:inactivePlan}"]}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the JSON response field "message" should contain "not active"

  @type:negative
  Scenario: A plan deactivated after the draft was saved blocks the publish
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    And I generate a unique resource name from "publish-later-plan" and store it as "laterPlan"
    And I create a subscription plan "${CTX:laterPlan}" allowing 10 requests per hour via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}   |
      | version             | ${CTX:apiVersion}  |
      | subscriptionPlanIds | ["${CTX:laterPlan}"] |
    And I send a "PUT" request to the control plane at "/subscription-plans/${CTX:laterPlan}" with body:
      """
      {"id": "${CTX:laterPlan}", "displayName": "${CTX:laterPlan}", "status": "INACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 10}]}
      """
    And the response status code should be 200
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the API Portal should not list the API "${CTX:apiHandle}"

  @type:negative
  Scenario: Unpublishing is refused while the API Portal still has subscriptions to the API
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And an application subscribes to the published API "${CTX:apiHandle}" with plan "${CTX:planHandle}" in the API Portal and the subscription is stored as "subscriptionId"
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_PORTAL_CONFLICT"
    And the JSON response field "message" should contain "active subscriptions are removed"
    And the response body should not contain "ERR_SUB_EXIST"
    And the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"

  @type:negative
  Scenario: Unpublishing is refused while the API Portal still has API keys for the API
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And an API key is generated in the API Portal for the published API "${CTX:apiHandle}" and the key handle is stored as "keyHandle"
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_PORTAL_CONFLICT"
    And the JSON response field "message" should contain "active API keys are removed"
    And the response body should not contain "ERR_KEY_EXIST"
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"

  Scenario: Unpublishing succeeds once the subscriptions and API keys are removed
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And an application subscribes to the published API "${CTX:apiHandle}" with plan "${CTX:planHandle}" in the API Portal and the subscription is stored as "subscriptionId"
    And an API key is generated in the API Portal for the published API "${CTX:apiHandle}" and the key handle is stored as "keyHandle"
    When the subscription "${CTX:subscriptionId}" is deleted from the API Portal
    And the API key "${CTX:keyHandle}" of the published API "${CTX:apiHandle}" is revoked in the API Portal
    And I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 204
    And the API Portal should not list the API "${CTX:apiHandle}"

  @type:negative
  Scenario: A wrong shared key is reported as an unavailable portal, not as a conflict
    Given I register the API Portal "${CTX:portalHandle}" with the control plane using a wrong shared key
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 503
    And the JSON response field "code" should be "PUBLICATION_PORTAL_UNAVAILABLE"
    And the API Portal should not list the API "${CTX:apiHandle}"
    When I read the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 404
    When I read the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200

  Scenario: Publishing works once the shared key is rotated to the one the portal accepts
    Given I register the API Portal "${CTX:portalHandle}" with the control plane using a wrong shared key
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 503
    When I rotate the shared key of the API Portal "${CTX:portalHandle}" to the key the portal accepts
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"

  @type:negative
  Scenario: A user without the publish scope cannot publish
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane as "developer"
    Then the response status code should be 403
    And the API Portal should not list the API "${CTX:apiHandle}"

  @type:negative
  Scenario: A user without the draft scope cannot save a draft
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    When I send a "PUT" request to the control plane at "/api-portals/${CTX:portalHandle}/apis/rest-api/${CTX:apiHandle}/draft" as "developer" with body:
      """
      {"displayName": "${CTX:apiHandle}", "version": "${CTX:apiVersion}"}
      """
    Then the response status code should be 403

  @type:negative
  Scenario: Publishing an API or to a portal that does not exist is refused
    Given I register the API Portal "${CTX:portalHandle}" with the control plane
    When I send a "POST" request to the control plane at "/api-portals/${CTX:portalHandle}/apis/rest-api/no-such-api/publish"
    Then the response status code should be 404
    And the JSON response field "code" should be "API_NOT_FOUND"
    When I send a "POST" request to the control plane at "/api-portals/no-such-portal/apis/rest-api/${CTX:apiHandle}/publish"
    Then the response status code should be 404
    And the JSON response field "code" should be "API_PORTAL_NOT_FOUND"
