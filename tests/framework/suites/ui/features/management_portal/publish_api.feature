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
Feature: The Management Portal publishes an API to an API Portal
  As an API publisher
  I want to publish my deployed API from the console
  So that developers find it, its definition and its documents in the API Portal

  @CONSOLE-016
  Scenario: Journey: a publisher publishes a deployed API, and the API Portal lists it with its definition and documents
    Given I generate a unique handle from "console-publish-project" and store it as "project"
    And I generate a unique handle from "console-publish" and store it as "api"
    And I generate a unique handle from "console-portal" and store it as "portal"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    And the block's API Portal is registered on the control plane as "${CTX:portal}"
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    And the user clicks "Create Document" in the Management Portal
    And the user enters "Quick start" as the "Name" in the Management Portal
    And the user enters "Call GET /pets first." as the "Content" in the Management Portal
    And the user clicks "Create" in the Management Portal
    And the documents page lists "Quick start" under "How To"
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/portals"
    And the user clicks "Publish" on portal "${CTX:portal}" in the Management Portal
    Then the publication status on portal "${CTX:portal}" becomes "Published"
    When the user follows "View in Portal" on portal "${CTX:portal}"
    Then the API Portal's API page lists exactly these resources:
      | GET  | /pets            |
      | POST | /pets            |
      | GET  | /pets/{petId}    |
      | GET  | /store/inventory |
    # The portal names a document by its handle and type, the path the control plane publishes it at.
    When the user follows the "Documentation" link in the API Portal
    Then the API Portal's documentation lists "API Definition" under "SPECIFICATION"
    And the API Portal's documentation lists "quick-start" under "HowTo"
    When the user follows the "APIs" link in the API Portal
    Then the Management Portal shows "${CTX:api}"
