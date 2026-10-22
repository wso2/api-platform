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

@api-portal-publishing
Feature: Publishing a REST API to an API Portal through the control plane
  As an API publisher
  I want to publish a deployed REST API to an API Portal from the control plane
  So that API consumers can discover it and its subscription plans in the portal

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "publish-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @smoke
  Scenario: A deployed REST API is published to the API Portal through the control plane
    Given I generate a unique resource name from "publish-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "publish-api" and store it as "apiHandle"
    And I generate a unique API context from "/publish" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    Given I generate a unique resource name from "publish-portal" and store it as "portalHandle"
    When I register the API Portal "${CTX:portalHandle}" with the control plane
    And I save a publication draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
    And I save a minimal OpenAPI definition as the draft definition of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}"
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And the publication of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" should have status "PUBLISHED"
    And the API Portal should list the API "${CTX:apiHandle}" offering plan "${CTX:planHandle}"
