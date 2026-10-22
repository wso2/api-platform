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
Feature: Publishing while the API Portal is unavailable
  As an API publisher
  I want a publish or deprecate that cannot reach the API Portal to fail without changing anything
  So that I can repeat it once the portal is back

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

  @type:negative
  Scenario: Publishing fails and promotes nothing while the API Portal is down
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I stop the API Portal
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 503
    And the JSON response field "code" should be "PUBLICATION_PORTAL_UNAVAILABLE"
    When I read the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 404
    When I read the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200

  @type:negative
  Scenario: Deprecating fails and leaves the API published while the API Portal is down
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And I stop the API Portal
    When I deprecate the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 503
    And the JSON response field "code" should be "PUBLICATION_PORTAL_UNAVAILABLE"
    And the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    When I start the API Portal and wait until it is ready
    Then the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"

  Scenario: The same publish succeeds once the API Portal is back
    Given I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I stop the API Portal
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 503
    When I start the API Portal and wait until it is ready
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the API Portal should list the API "${CTX:apiHandle}" with status "PUBLISHED"
