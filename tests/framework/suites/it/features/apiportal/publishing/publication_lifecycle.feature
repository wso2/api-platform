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

@platform-api-gateway-api-portal @APIPORTAL
Feature: Publishing an API to an API Portal through the control plane
  As an API publisher
  I want to publish a REST or GraphQL API to an API Portal from the control plane
  So that API consumers can discover it and its subscription plans in the portal

  @APIP-044 @smoke
  Scenario: A REST API is published, republished, deprecated and unpublished through the control plane
    # A deployed REST API is published to the API Portal through the control plane
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "publish-project" and store it as "projectHandle"
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
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"

    # Publishing creates the selected plans the API Portal does not have yet
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
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

    # An API that was never deployed to a gateway can be published
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
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

    # Republishing an edited draft replaces the listing in the API Portal
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
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

    # Republishing keeps the API Portal's subscriptions and shows the new description
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | description         | First description     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    And an application subscribes to the published API "${CTX:apiHandle}" with plan "${CTX:planHandle}" in the API Portal and the subscription is stored as "subscriptionId"
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}-renamed |
      | version             | ${CTX:apiVersion}        |
      | description         | Second description       |
      | subscriptionPlanIds | ["${CTX:planHandle}"]    |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the API Portal should list the API "${CTX:apiHandle}" with display name "${CTX:apiHandle}-renamed"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}" as "admin"
    Then the response status code should be 200
    And the JSON response field "description" should be "Second description"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:planHandle}"

    # Deprecating a published API changes only its status
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
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

    # Deprecating keeps the plan on offer and the existing subscription in the API Portal
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And an application subscribes to the published API "${CTX:apiHandle}" with plan "${CTX:planHandle}" in the API Portal and the subscription is stored as "subscriptionId"
    When I deprecate the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the API Portal should list the API "${CTX:apiHandle}" with status "DEPRECATED"
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:planHandle}"

    # Unpublishing removes the listing from the API Portal and keeps a draft
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
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

    # Unpublishing keeps the draft that was saved after publishing
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
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

  @APIP-044
  Scenario: A published API is served by its pages and labelled views, and leaves them when unpublished
    # A published API's page, specification and Markdown are served by the API Portal
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}/docs/specification.json"
    Then the response status code should be 200
    And the JSON response field "openapi" should be "3.0.3"
    And the response body should contain "/ping"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiHandle}"

    # An API published with a label appears only in the views of that label
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-partners" and store it as "partnersLabel"
    And I generate a unique resource name from "publish-internal" and store it as "otherLabel"
    And I generate a unique resource name from "publish-partners-view" and store it as "partnersView"
    And I generate a unique resource name from "publish-internal-view" and store it as "otherView"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "partnersLabel":
      | id          | ${CTX:partnersLabel} |
      | displayName | Partners             |
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "otherLabel":
      | id          | ${CTX:otherLabel} |
      | displayName | Internal          |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "partnersView":
      | id          | ${CTX:partnersView}       |
      | displayName | Partners View             |
      | labels      | ["${CTX:partnersLabel}"]  |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "otherView":
      | id          | ${CTX:otherView}      |
      | displayName | Internal View         |
      | labels      | ["${CTX:otherLabel}"] |
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}         |
      | version             | ${CTX:apiVersion}        |
      | labels              | ["${CTX:partnersLabel}"] |
      | subscriptionPlanIds | ["${CTX:planHandle}"]    |
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:partnersView}/apis.md"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:otherView}/apis.md"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiHandle}"

    # Unpublishing removes the API's page, Markdown and agent listings from the API Portal
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response body should contain "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/llms.txt"
    Then the response body should contain "${CTX:apiHandle}"
    When I unpublish the "RestApi" "${CTX:apiHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 204
    And the API Portal should not list the API "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}/docs/specification.json"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:apiHandle}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiHandle}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "${CTX:apiHandle}"

  @APIP-049 @type:negative
  Scenario: A GraphQL API is published, deprecated and unpublished, only through its own type and definition
    # A GraphQL API is published, deprecated and unpublished through the control plane
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-graphql" and store it as "graphqlHandle"
    And I generate a unique API context from "/publish-graphql" and store it as "graphqlContext"
    And I create a GraphQL API "${CTX:graphqlHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:graphqlContext}"
    And I save a publishable draft of the "GraphQLApi" "${CTX:graphqlHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:graphqlHandle}  |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "GraphQLApi" "${CTX:graphqlHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the publication of the "GraphQLApi" "${CTX:graphqlHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    And the API Portal should list the API "${CTX:graphqlHandle}" with type "GRAPHQL"
    And the API Portal should list the API "${CTX:graphqlHandle}" with status "PUBLISHED"
    And the API Portal should list the API "${CTX:graphqlHandle}" offering plan "${CTX:planHandle}"
    When I deprecate the "GraphQLApi" "${CTX:graphqlHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the API Portal should list the API "${CTX:graphqlHandle}" with status "DEPRECATED"
    When I unpublish the "GraphQLApi" "${CTX:graphqlHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 204
    And the API Portal should not list the API "${CTX:graphqlHandle}"
    When I read the draft of the "GraphQLApi" "${CTX:graphqlHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:graphqlHandle}"

    # A GraphQL draft is published only through its own type, and only with a GraphQL definition
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-graphql" and store it as "graphqlHandle"
    And I generate a unique API context from "/publish-graphql" and store it as "graphqlContext"
    And I create a GraphQL API "${CTX:graphqlHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:graphqlContext}"
    And I save a publication draft of the "GraphQLApi" "${CTX:graphqlHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:graphqlHandle}  |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I send a "PUT" request to the control plane at "/api-portals/${CTX:portalHandle}/apis/graphql-api/${CTX:graphqlHandle}/draft/definition" with body:
      """
      {"openapi": "3.0.3", "info": {"title": "${CTX:graphqlHandle}", "version": "1.0.0"}, "paths": {}}
      """
    When I publish the "RestApi" "${CTX:graphqlHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 404
    And the JSON response field "code" should be "API_NOT_FOUND"
    When I publish the "GraphQLApi" "${CTX:graphqlHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the API Portal should not list the API "${CTX:graphqlHandle}"

  @APIP-049 @type:negative
  Scenario: An MCP proxy is published, republished, deprecated and unpublished, only through its own type and definition
    # An MCP proxy is published, deprecated and unpublished through the control plane
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-mcp" and store it as "mcpHandle"
    And I generate a unique API context from "/publish-mcp" and store it as "mcpContext"
    And I create an MCP proxy "${CTX:mcpHandle}" via the control plane with context "${CTX:mcpContext}" and spec versions "2025-06-18"
    And I save a publishable draft of the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:mcpHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    When I publish the "Mcp" "${CTX:mcpHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the publication of the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    And the API Portal should list the MCP server "${CTX:mcpHandle}" with status "PUBLISHED"
    And the API Portal should list the MCP server "${CTX:mcpHandle}" offering plan "${CTX:planHandle}"
    And the API Portal should not list the API "${CTX:mcpHandle}"
    When I deprecate the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the API Portal should list the MCP server "${CTX:mcpHandle}" with status "DEPRECATED"
    When I unpublish the "Mcp" "${CTX:mcpHandle}" from the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 204
    And the API Portal should not list the MCP server "${CTX:mcpHandle}"
    When I read the draft of the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:mcpHandle}"

    # Republishing an edited MCP proxy draft replaces its listing in the API Portal
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-mcp" and store it as "mcpHandle"
    And I generate a unique API context from "/publish-mcp" and store it as "mcpContext"
    And I create an MCP proxy "${CTX:mcpHandle}" via the control plane with context "${CTX:mcpContext}" and spec versions "2025-06-18"
    And I save a publishable draft of the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:mcpHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "Mcp" "${CTX:mcpHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I save a publishable draft of the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:mcpHandle}-renamed |
      | version             | ${CTX:apiVersion}        |
      | subscriptionPlanIds | ["${CTX:planHandle}"]    |
    And I publish the "Mcp" "${CTX:mcpHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    And the API Portal should list the MCP server "${CTX:mcpHandle}" with display name "${CTX:mcpHandle}-renamed"
    And the API Portal should list the MCP server "${CTX:mcpHandle}" with status "PUBLISHED"

    # An MCP proxy draft is published only through its own type, and only with an MCP definition
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
    And I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    Given I generate a unique resource name from "publish-mcp" and store it as "mcpHandle"
    And I generate a unique API context from "/publish-mcp" and store it as "mcpContext"
    And I create an MCP proxy "${CTX:mcpHandle}" via the control plane with context "${CTX:mcpContext}" and spec versions "2025-06-18"
    And I save a publication draft of the "Mcp" "${CTX:mcpHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:mcpHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I send a "PUT" request to the control plane at "/api-portals/${CTX:portalHandle}/apis/mcp-proxy/${CTX:mcpHandle}/draft/definition" with body:
      """
      {"openapi": "3.0.3", "info": {"title": "${CTX:mcpHandle}", "version": "1.0.0"}, "paths": {}}
      """
    When I publish the "RestApi" "${CTX:mcpHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 404
    And the JSON response field "code" should be "API_NOT_FOUND"
    When I publish the "Mcp" "${CTX:mcpHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the API Portal should not list the MCP server "${CTX:mcpHandle}"
