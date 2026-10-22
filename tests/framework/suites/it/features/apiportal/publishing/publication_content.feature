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
Feature: Documents, a landing page and a thumbnail published with an API reach the API Portal
  As an API publisher
  I want the content I select for a publication to be what the API Portal shows
  So that consumers see the documentation I prepared and nothing I took out

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

  Scenario: Documents, a landing page and a thumbnail are published to the API Portal
    And I generate a unique resource name from "publish-doc" and store it as "docHandle"
    And I create a document "${CTX:docHandle}" of type "HowTo" with content "Step one: call the API." on the "RestApi" "${CTX:apiHandle}" via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle} |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
      | docIds              | ["${CTX:docHandle}"] |
    And I save the landing page "Welcome to the published API" on the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}"
    And I save a PNG thumbnail on the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}"
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=DOC_HowTo&fileName=${CTX:docHandle}.md" as "admin"
    Then the response status code should be 200
    And the response body should contain "Step one: call the API."
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=MARKETING&fileName=apiContent.md" as "admin"
    Then the response status code should be 200
    And the response body should contain "Welcome to the published API"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=IMAGE&fileName=api-icon.png" as "admin"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "image"

  @type:negative
  Scenario: Documents that together exceed the content size limit are refused before anything is sent to the API Portal
    And I generate a unique resource name from "publish-doc" and store it as "docHandle"
    And I create a document "${CTX:docHandle}" of type "HowTo" with 16384 bytes of incompressible content on the "RestApi" "${CTX:apiHandle}" via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle} |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
      | docIds              | ["${CTX:docHandle}"] |
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 400
    And the JSON response field "code" should be "PUBLICATION_VALIDATION_FAILED"
    And the JSON response field "message" should contain "too large"
    And the API Portal should not list the API "${CTX:apiHandle}"

  @known-issue
  Scenario: Unselecting every document, the landing page and the thumbnail removes them from the API Portal
    And I generate a unique resource name from "publish-doc" and store it as "docHandle"
    And I create a document "${CTX:docHandle}" of type "HowTo" with content "Step one: call the API." on the "RestApi" "${CTX:apiHandle}" via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle} |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
      | docIds              | ["${CTX:docHandle}"] |
    And I save the landing page "Welcome to the published API" on the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}"
    And I save a PNG thumbnail on the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}"
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}        |
      | version             | ${CTX:apiVersion}       |
      | subscriptionPlanIds | ["${CTX:planHandle}"]   |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=DOC_HowTo&fileName=${CTX:docHandle}.md" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=MARKETING&fileName=apiContent.md" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=IMAGE&fileName=api-icon.png" as "admin"
    Then the response status code should be 404

  @known-issue
  Scenario: Unselecting one of two documents removes only that document from the API Portal
    And I generate a unique resource name from "publish-doc" and store it as "docHandle"
    And I generate a unique resource name from "publish-doc-b" and store it as "docHandleB"
    And I create a document "${CTX:docHandle}" of type "HowTo" with content "Step one: call the API." on the "RestApi" "${CTX:apiHandle}" via the control plane
    And I create a document "${CTX:docHandleB}" of type "Samples" with content "Sample request." on the "RestApi" "${CTX:apiHandle}" via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle} |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
      | docIds              | ["${CTX:docHandle}", "${CTX:docHandleB}"] |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}        |
      | version             | ${CTX:apiVersion}       |
      | subscriptionPlanIds | ["${CTX:planHandle}"]   |
      | docIds              | ["${CTX:docHandle}"]    |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=DOC_HowTo&fileName=${CTX:docHandle}.md" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=DOC_Samples&fileName=${CTX:docHandleB}.md" as "admin"
    Then the response status code should be 404

  @known-issue
  Scenario: Unselecting the thumbnail removes it from the API Portal
    And I generate a unique resource name from "publish-doc" and store it as "docHandle"
    And I create a document "${CTX:docHandle}" of type "HowTo" with content "Step one: call the API." on the "RestApi" "${CTX:apiHandle}" via the control plane
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle} |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |
      | docIds              | ["${CTX:docHandle}"] |
    And I save a PNG thumbnail on the draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}"
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    And the response status code should be 201
    When I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}        |
      | version             | ${CTX:apiVersion}       |
      | subscriptionPlanIds | ["${CTX:planHandle}"]   |
      | docIds              | ["${CTX:docHandle}"]    |
    And I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiHandle}/assets?type=IMAGE&fileName=api-icon.png" as "admin"
    Then the response status code should be 404
