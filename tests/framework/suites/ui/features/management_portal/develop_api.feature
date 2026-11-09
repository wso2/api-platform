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

@CONSOLE
Feature: The Management Portal edits an API, attaches its policies and keeps its documents
  As an API publisher
  I want to change an API's details, endpoint, definition, policies and documents in the console
  So that the API and its gateway behaviour follow what I saved

  @CONSOLE-007
  Scenario: Journey: a publisher edits an API's definition, name, description and endpoint, and they stay saved after a reload
    Given I generate a unique handle from "console-edit-project" and store it as "project"
    And I generate a unique handle from "console-edit" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    And the user clicks "Import Definition" in the Management Portal
    And the user uploads spec fixture "petstore-v2.yaml" in the Management Portal
    And the user clicks "Save" in the Management Portal
    And the user confirms with "Save" in the Management Portal
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/edit"
    And the user enters "${CTX:api} edited" as the "Name" in the Management Portal
    And the user enters "Edited in the console." as the "Description" in the Management Portal
    And the user clicks "Save changes" in the Management Portal
    And the Management Portal shows "API updated."
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    And the user clicks "Edit endpoint" in the Management Portal
    And the user enters "http://testbench:3000/edited" as the "Endpoint URL" in the Management Portal
    And the user clicks "Save" in the Management Portal
    Then the Management Portal shows "Backend endpoint updated."
    # A fresh load of the page: everything below is read back from the control plane.
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    Then the API overview shows name "${CTX:api} edited"
    And the API overview shows description "Edited in the console."
    And the API overview shows endpoint "http://testbench:3000/edited"
    And the API overview lists exactly these resources:
      | GET  | /pets                |
      | POST | /pets                |
      | GET  | /pets/{petId}        |
      | GET  | /pets/{petId}/photos |
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    Then the Management Portal shows "4 resources"
    And the Management Portal shows "Pets and their photos."

  # The edit page offers Version and Context, but the control plane answers 200 to the update and
  # keeps the old values: https://github.com/wso2/api-platform/issues/3899
  @CONSOLE-007 @known-issue
  Scenario: Journey: a publisher changes an API's version and context, and they stay saved after a reload
    Given I generate a unique handle from "console-edit-project" and store it as "project"
    And I generate a unique handle from "console-edit-version" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/edit"
    And the user enters "v2.0" as the "Version" in the Management Portal
    And the user enters "/${CTX:api}-edited" as the "Context" in the Management Portal
    And the user clicks "Save changes" in the Management Portal
    And the Management Portal shows "API updated."
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    Then the API overview shows version "v2.0"
    And the API overview shows context "/${CTX:api}-edited"

  @CONSOLE-008 @CONSOLE-015
  Scenario: Journey: a publisher attaches an API-level and a resource-level policy, and the redeployed gateway applies the resource's only to it
    Given I generate a unique handle from "console-policy-project" and store it as "project"
    And I generate a unique handle from "console-policy" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "before"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/store/inventory"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/policies"
    And the user chooses "CORS" in the Management Portal
    Then the Management Portal shows "Applies to Global policies"
    When the user clicks "Attach policy" in the Management Portal
    And the user opens resource "GET /store/inventory" on the Policies page
    And the user chooses "Basic Auth" in the Management Portal
    Then the Management Portal shows "Applies to GET /store/inventory"
    When the user enters "console" as the "Username" in the Management Portal
    And the user enters "console-secret" as the "Password" in the Management Portal
    And the user clicks "Attach policy" in the Management Portal
    And the user clicks "Save changes" in the Management Portal
    # A fresh load of the page: the policies below are read back from the control plane.
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/policies"
    Then the Management Portal shows global policy "cors"
    And the Management Portal shows policy "basic-auth" on resource "GET /store/inventory"
    And the Management Portal shows no policy on resource "GET /pets"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "after"
    And the deployment status on gateway "it-gateway" becomes "Active"
    # The 401 shows the new deployment is live before the other resources are checked.
    Then the gateway answers 401 to a "GET" request to "/${CTX:api}/store/inventory"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/store/inventory" as "console" with password "console-secret"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/pets"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/pets/42"

  @CONSOLE-009
  Scenario: Journey: a publisher creates, edits and deletes an API's document
    Given I generate a unique handle from "console-docs-project" and store it as "project"
    And I generate a unique handle from "console-docs" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    And the user clicks "Create Document" in the Management Portal
    And the user selects "Samples & SDK" as the "Document type" in the Management Portal
    And the user enters "Quick start" as the "Name" in the Management Portal
    And the user enters "Call GET /pets first." as the "Content" in the Management Portal
    And the user clicks "Create" in the Management Portal
    Then the documents page lists "Quick start" under "Samples & SDK"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    Then the documents page lists "Quick start" under "Samples & SDK"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    Then the API overview's "Documents" card shows "Quick start"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    And the user clicks "Edit" in the Management Portal
    And the user enters "Call GET /store/inventory first." as the "Content" in the Management Portal
    And the user clicks "Save" in the Management Portal
    Then the Management Portal shows "Call GET /store/inventory first."
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    Then the Management Portal shows "Call GET /store/inventory first."
    And the Management Portal does not show "Call GET /pets first."
    When the user clicks "Delete" in the Management Portal
    And the user confirms with "Delete" in the Management Portal
    Then the Management Portal shows "No Documents available for this API"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    Then the API overview's "Documents" card shows "No Documents available for this API"
