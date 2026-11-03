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

@APIDEF
Feature: The Management Portal creates an API from an OpenAPI definition and replaces its definition
  As an API publisher
  I want to create an API from my OpenAPI definition and upload a new one when it changes
  So that the console keeps an API's operations in step with its contract

  @APIDEF-010 @APIDEF-009 @APIDEF-012 @APIDEF-001
  Scenario: Journey: a publisher creates an API from an OpenAPI file after the console refuses unusable ones
    Given I generate a unique handle from "apidef-ui-project" and store it as "project"
    And I generate a unique handle from "apidef-ui-file" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user chooses "Start with a contract" in the Management Portal
    And the user clicks "Upload" in the Management Portal
    And the user uploads spec fixture "invalid.yaml" in the Management Portal
    Then the Management Portal shows "Validation failed:"
    When the user uploads spec fixture "swagger2.yaml" in the Management Portal
    Then the Management Portal shows "Swagger 2.x is not allowed"
    When the user uploads a generated spec of 65537 bytes in the Management Portal
    Then the Management Portal shows "The OpenAPI specification exceeds the maximum allowed size."
    When the user uploads spec fixture "petstore-v1.yaml" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "${CTX:api}" as the "Name" in the Management Portal
    And the user enters "v1.0" as the "Version" in the Management Portal
    And the user enters "/${CTX:api}" as the "Context" in the Management Portal
    And the user enters "Pets imported in the console" as the "Description" in the Management Portal
    And the user enters "http://testbench:3000" as the "Target URL" in the Management Portal
    And the user clicks "Create" in the Management Portal
    Then the Management Portal shows "/pets/{petId}"
    And the Management Portal shows "/store/inventory"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    Then the Management Portal shows "Pets and the store's inventory."

  @APIDEF-002
  Scenario: Journey: a publisher creates an API from an OpenAPI URL
    Given I generate a unique handle from "apidef-ui-project" and store it as "project"
    And I generate a unique handle from "apidef-ui-url" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And I store the https URL of spec fixture "petstore-v1.yaml" as "specUrl"
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user chooses "Start with a contract" in the Management Portal
    And the user clicks "URL" in the Management Portal
    And the user enters "${CTX:specUrl}" as the "URL for API Contract" in the Management Portal
    And the user submits the "URL for API Contract" field in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "${CTX:api}" as the "Name" in the Management Portal
    And the user enters "v1.0" as the "Version" in the Management Portal
    And the user enters "/${CTX:api}" as the "Context" in the Management Portal
    And the user enters "http://testbench:3000" as the "Target URL" in the Management Portal
    And the user clicks "Create" in the Management Portal
    Then the Management Portal shows "/pets/{petId}"
    And the Management Portal shows "/store/inventory"

  @APIDEF-004 @APIDEF-005
  Scenario: Journey: a publisher replaces a deployed API's definition and redeploys it
    Given I generate a unique handle from "apidef-ui-project" and store it as "project"
    And I generate a unique handle from "apidef-ui-update" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/store/inventory"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    And the user clicks "Import Definition" in the Management Portal
    And the user uploads spec fixture "petstore-v2.yaml" in the Management Portal
    And the user clicks "Save" in the Management Portal
    And the user confirms with "Save" in the Management Portal
    And the user clicks "View Resources" in the Management Portal
    Then the Management Portal shows "/pets/{petId}/photos"
    And the Management Portal does not show "/store/inventory"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "second"
    And the deployment status on gateway "it-gateway" becomes "Active"
    Then the gateway answers 200 to a "GET" request to "/${CTX:api}/pets/42/photos"
    And the gateway answers 404 to a "GET" request to "/${CTX:api}/store/inventory"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    And the user clicks "Import Definition" in the Management Portal
    And the user uploads spec fixture "petstore-renamed.yaml" in the Management Portal
    And the user clicks "Save" in the Management Portal
    And the user confirms with "Save" in the Management Portal
    And the user clicks "View Resources" in the Management Portal
    Then the Management Portal shows "/animals"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "third"
    And the deployment status on gateway "it-gateway" becomes "Active"
    Then the gateway answers 200 to a "GET" request to "/${CTX:api}/animals"
