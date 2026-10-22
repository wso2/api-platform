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

@platform-api-with-api-portal
Feature: Publishing a REST API to an API Portal through the control plane
  As an API publisher
  I want to publish a deployed REST API to an API Portal from the control plane
  So that API consumers can discover it and its subscription plans in the portal

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
    And I register the API Portal "${CTX:portalHandle}" with the control plane

  @smoke
  Scenario: A deployed REST API is published to the API Portal through the control plane
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"

  Scenario: Publishing creates the selected plans the API Portal does not have yet
    Given the subscription plan "${CTX:planHandle}" is synced to the API Portal
    And I generate a unique resource name from "publish-second-plan" and store it as "secondPlan"
    And I create a subscription plan "${CTX:secondPlan}" allowing 20 requests per hour via the control plane
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}                           |
      | version             | ${CTX:apiVersion}                          |
      | subscriptionPlanIds | ["${CTX:planHandle}", "${CTX:secondPlan}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:secondPlan}"

  Scenario: Republishing an edited draft replaces the listing in the API Portal
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}-renamed |
      | version             | ${CTX:apiVersion}        |
      | subscriptionPlanIds | ["${CTX:planHandle}"]    |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the API Portal should list the API "${CTX:apiHandle}" with display name "${CTX:apiHandle}-renamed"
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"

  Scenario: Deprecating a published API changes only its status
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    When I deprecate the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the JSON response field "status" should be "DEPRECATED"
    And the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" should have status "DEPRECATED"
    And the API Portal should list the API "${CTX:apiHandle}" with status "DEPRECATED"
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"

  @type:negative
  Scenario: Deprecating an already deprecated API is refused
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And I deprecate the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    When I deprecate the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_STATE_CONFLICT"

  Scenario: Unpublishing removes the listing from the API Portal and keeps a draft
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 204
    And the API Portal should not list the API "${CTX:apiHandle}"
    When I read the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 404
    When I read the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiHandle}"

  Scenario: Unpublishing keeps the draft that was saved after publishing
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}-next |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 204
    And the API Portal should not list the API "${CTX:apiHandle}"
    When I read the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the JSON response field "displayName" should be "${CTX:apiHandle}-next"

  @type:negative
  Scenario: Unpublishing an already unpublished API is refused
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_STATE_CONFLICT"

  @type:negative
  Scenario: An API that was never published cannot be deprecated or unpublished
    When I deprecate the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_STATE_CONFLICT"
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_STATE_CONFLICT"

  @type:negative
  Scenario: A draft without a definition cannot be published
    Given I save a publication draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the API Portal should not list the API "${CTX:apiHandle}"

  @type:negative
  Scenario: A draft with an invalid definition cannot be published
    Given I save a publication draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I send a "PUT" request to the control plane at "/api-portals/${CTX:portalHandle}/apis/rest-api/${CTX:apiHandle}/draft/definition" with body:
      """
      {"swagger": "not an OpenAPI document"}
      """
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the API Portal should not list the API "${CTX:apiHandle}"

  Scenario: An API that was never deployed to a gateway can be published
    Given I generate a unique resource name from "publish-idle-api" and store it as "idleHandle"
    And I generate a unique API context from "/publish-idle" and store it as "idleContext"
    And I create a secured REST API "${CTX:idleHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:idleContext}" offering plan "${CTX:planHandle}"
    And I save a publishable draft of the "RestApi" "${CTX:idleHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:idleHandle}     |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "RestApi" "${CTX:idleHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the API Portal should list the API "${CTX:idleHandle}" with status "PUBLISHED"
