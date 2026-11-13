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
Feature: The Management Portal's create wizard builds a REST API from scratch and refuses unusable details
  As an API publisher
  I want the console's create wizard to build my API from a backend endpoint and to stop me on bad details
  So that the API I create is the one I described

  @CONSOLE-001
  Scenario: Journey: a publisher creates a REST API from scratch with a backend endpoint
    Given I generate a unique handle from "console-create-project" and store it as "project"
    And I generate a unique handle from "console-scratch" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "http://testbench:3000/scratch" as the "Endpoint URL" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "${CTX:api}" as the "Name" in the Management Portal
    And the user enters "v2.5" as the "Version" in the Management Portal
    And the user enters "/${CTX:api}" as the "Context" in the Management Portal
    And the REST API "${CTX:api}" created in the Management Portal is deleted after the scenario
    And the user clicks "Create" in the Management Portal
    Then the API overview shows name "${CTX:api}"
    And the API overview shows version "v2.5"
    And the API overview shows context "/${CTX:api}"
    And the API overview shows endpoint "http://testbench:3000/scratch"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis"
    Then the Management Portal shows "${CTX:api}"

  @CONSOLE-004
  Scenario: Journey: the create wizard shows each invalid detail's error on Create and creates nothing
    Given I generate a unique handle from "console-invalid-project" and store it as "project"
    And a project "${CTX:project}" exists on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "http://testbench:3000" as the "Endpoint URL" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    # The identifier first: once edited, it no longer follows the name being cleared.
    And the user enters "Not_A_Handle" as the "Identifier" in the Management Portal
    And the user enters "" as the "Name" in the Management Portal
    And the user enters "v 1" as the "Version" in the Management Portal
    And the user enters "/bad context" as the "Context" in the Management Portal
    And the user enters "not-a-url" as the "Endpoint" in the Management Portal
    And the user clicks "Create" in the Management Portal
    Then the "Name" field in the Management Portal shows the error "Enter a name."
    And the "Identifier" field in the Management Portal shows the error "Use lowercase letters and numbers, separated by single hyphens."
    And the "Version" field in the Management Portal shows the error "Use letters, numbers, dots, hyphens and underscores — no spaces or slashes."
    And the "Context" field in the Management Portal shows the error "Start with / and use only letters, numbers, hyphens, dots and slashes."
    And the "Endpoint" field in the Management Portal shows the error "Enter a full URL, for example https://api.example.com."
    And the Management Portal shows "Configure and create"
    And project "${CTX:project}" has exactly these REST APIs on the control plane: ""

  # The REST wizard has no identifier check, and the control plane's refusal names no field, so
  # it lands in a banner: https://github.com/wso2/api-platform/issues/4139
  @CONSOLE-005 @known-issue
  Scenario: Journey: an identifier already used in the project is flagged on its field, and nothing is created until it changes
    Given I generate a unique handle from "console-taken-project" and store it as "project"
    And I generate a unique handle from "console-taken" and store it as "taken"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:taken}" exists in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "http://testbench:3000" as the "Endpoint URL" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    # Another name, so only the identifier collides.
    And the user enters "Another ${CTX:taken}" as the "Name" in the Management Portal
    And the user enters "${CTX:taken}" as the "Identifier" in the Management Portal
    Then the "Identifier" field in the Management Portal shows the error "This identifier is already in use."
    When the user clicks "Create" in the Management Portal
    Then the "Identifier" field in the Management Portal shows the error "This identifier is already in use."
    And project "${CTX:project}" has exactly these REST APIs on the control plane: "${CTX:taken}"
    When the user enters "${CTX:taken}-new" as the "Identifier" in the Management Portal
    And the REST API "${CTX:taken}-new" created in the Management Portal is deleted after the scenario
    And the user clicks "Create" in the Management Portal
    Then the API overview shows name "Another ${CTX:taken}"
    And project "${CTX:project}" has exactly these REST APIs on the control plane: "${CTX:taken},${CTX:taken}-new"

  @CONSOLE-006
  Scenario: Journey: going back in the wizard keeps the endpoint entered, and the API is created with the changed one
    Given I generate a unique handle from "console-back-project" and store it as "project"
    And I generate a unique handle from "console-back-endpoint" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user enters "http://testbench:3000/first" as the "Endpoint URL" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    Then the "Endpoint" field in the Management Portal holds "http://testbench:3000/first"
    When the user clicks "Back" in the Management Portal
    Then the "Endpoint URL" field in the Management Portal holds "http://testbench:3000/first"
    When the user enters "http://testbench:3000/second" as the "Endpoint URL" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    Then the "Endpoint" field in the Management Portal holds "http://testbench:3000/second"
    When the user enters "${CTX:api}" as the "Name" in the Management Portal
    And the REST API "${CTX:api}" created in the Management Portal is deleted after the scenario
    And the user clicks "Create" in the Management Portal
    Then the API overview shows name "${CTX:api}"
    And the API overview shows endpoint "http://testbench:3000/second"

  @CONSOLE-006
  Scenario: Journey: going back in the wizard keeps the contract uploaded, and the API is created with the changed one
    Given I generate a unique handle from "console-back-project" and store it as "project"
    And I generate a unique handle from "console-back-contract" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/new"
    And the user chooses "REST API" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    And the user chooses "Start with a contract" in the Management Portal
    And the user clicks "Upload" in the Management Portal
    And the user uploads spec fixture "petstore-v1.yaml" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    Then the "Description" field in the Management Portal holds "Pets and the store's inventory."
    When the user clicks "Back" in the Management Portal
    Then the Management Portal shows "/store/inventory"
    When the user uploads spec fixture "petstore-v2.yaml" in the Management Portal
    And the user clicks "Continue" in the Management Portal
    Then the "Description" field in the Management Portal holds "Pets and their photos."
    When the user enters "${CTX:api}" as the "Name" in the Management Portal
    And the user enters "http://testbench:3000" as the "Endpoint" in the Management Portal
    And the REST API "${CTX:api}" created in the Management Portal is deleted after the scenario
    And the user clicks "Create" in the Management Portal
    Then the API overview lists exactly these resources:
      | GET  | /pets                 |
      | POST | /pets                 |
      | GET  | /pets/{petId}         |
      | GET  | /pets/{petId}/photos  |
