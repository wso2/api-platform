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
Feature: The Management Portal offers a read-only user no way to change an API
  As an organization's read-only member
  I want to open an API's pages without being offered changes I may not make
  So that the console never invites me into an action the control plane would refuse

  @CONSOLE-018
  Scenario: Journey: a read-only viewer opens an API's pages and is offered no create, edit, deploy or delete action
    Given I generate a unique handle from "console-viewer-project" and store it as "project"
    And I generate a unique handle from "console-viewer" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    # The administrator first: the same controls are there and usable, so their absence below is
    # the viewer's role and not a selector that matches nothing.
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis"
    Then the Management Portal shows "${CTX:api}"
    And the Management Portal offers a "Create API" button
    And the Management Portal offers a "Delete ${CTX:api}" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    Then the Management Portal shows "/store/inventory"
    And the Management Portal offers a "Edit API details" link
    And the Management Portal offers a "Deploy to Gateway" link
    And the Management Portal's "Edit endpoint" button is enabled
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    Then the Management Portal's "Import Definition" button is enabled
    And the Management Portal offers a "Edit" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/policies"
    Then the Management Portal shows "Drag and drop policies here to apply across all resources"
    And the policy catalog's "CORS" entry in the Management Portal can be attached
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    Then the Management Portal's "Create Document" button is enabled
    When the user clicks "Create Document" in the Management Portal
    And the user enters "Viewer guide" as the "Name" in the Management Portal
    And the user enters "Read before calling the API." as the "Content" in the Management Portal
    And the user clicks "Create" in the Management Portal
    Then the documents page lists "Viewer guide" under "How To"
    And the Management Portal offers a "Create Document" button
    And the Management Portal offers a "Edit" button
    And the Management Portal offers a "Delete" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    Then the "Deploy" button on gateway "it-gateway" in the Management Portal is enabled
    # The viewer.
    When the user signs in to the Management Portal as the read-only viewer
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis"
    Then the Management Portal shows "${CTX:api}"
    And the Management Portal offers no "Create API" button
    And the Management Portal offers no "Delete ${CTX:api}" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}"
    Then the Management Portal shows "/store/inventory"
    And the Management Portal offers no "Edit API details" link
    And the Management Portal's "Deploy to Gateway" link is disabled
    And the Management Portal's "Edit endpoint" button is disabled
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/definition"
    Then the Management Portal's "Import Definition" button is disabled
    And the Management Portal offers no "Edit" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/policies"
    # The catalog stays on show, but picking a policy from it opens nothing to attach.
    Then the Management Portal shows "No policies attached."
    And the Management Portal does not show "Drag and drop policies here to apply across all resources"
    And the policy catalog's "CORS" entry in the Management Portal can not be attached
    When the user chooses "CORS" in the Management Portal
    Then the Management Portal offers no "Attach policy" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/documents"
    # With a document listed, the page's Create Document action is hidden rather than disabled.
    Then the documents page lists "Viewer guide" under "How To"
    And the Management Portal offers no "Create Document" button
    And the Management Portal offers no "Edit" button
    And the Management Portal offers no "Delete" button
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    Then the "Deploy" button on gateway "it-gateway" in the Management Portal is disabled
