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

@DEPLC
Feature: The Management Portal stops, redeploys, restores and deletes an API's deployments
  As an API publisher
  I want to manage an API's deployments from its deploy page
  So that I can take it off a gateway, bring it back and clear its history without the API

  # The console has deploy pages for REST and GraphQL APIs only; LLM providers, LLM proxies and
  # MCP proxies are covered by the integration suite.
  @DEPLC-004 @DEPLC-003
  Scenario: Journey: a REST API's live deployment can be neither deleted nor restored over itself
    Given I generate a unique handle from "deplc-ui-rest-project" and store it as "project"
    And I generate a unique handle from "deplc-ui-rest" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" exists in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    # Four deployments, so the full history, where deletion lives, is offered.
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "second"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "third"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "live"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/health"
    * sheet check "DEPLC-004"
    When the user clicks "View More" in the Management Portal
    Then the Management Portal offers a "Delete ${CTX:first}" button
    And the Management Portal offers no "Delete ${CTX:live}" button
    When the user closes the drawer in the Management Portal
    * sheet check "DEPLC-003"
    And the user clicks "Change deployment on it-gateway" on gateway "it-gateway" in the Management Portal
    And the user selects deployment "${CTX:live}" in the Management Portal
    Then the Management Portal's "Restore" button is disabled
    When the user selects deployment "${CTX:first}" in the Management Portal
    Then the Management Portal's "Restore" button is enabled
    When the user clicks "Cancel" in the Management Portal
    Then the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/health"

  # The console sends undeploy and restore without the gatewayId the control plane requires:
  # https://github.com/wso2/api-platform/issues/3900
  @DEPLC-001 @DEPLC-002 @DEPLC-005 @known-issue
  Scenario: Journey: a REST API's deployment is stopped, redeployed and deleted from its deploy page
    Given I generate a unique handle from "deplc-ui-rest-actions-project" and store it as "project"
    And I generate a unique handle from "deplc-ui-rest-actions" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" exists in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/deploy"
    # Four deployments, so the full history, where deletion lives, is offered.
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "second"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "third"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "live"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/health"
    * sheet check "DEPLC-001"
    When the user clicks "Stop" on gateway "it-gateway" in the Management Portal
    Then the Management Portal confirms stopping deployment "${CTX:live}"
    And the deployment status on gateway "it-gateway" becomes "Suspended"
    And the gateway answers 404 to a "GET" request to "/${CTX:api}/health"
    * sheet check "DEPLC-002"
    When the user clicks "Redeploy" on gateway "it-gateway" in the Management Portal
    Then the Management Portal confirms redeploying deployment "${CTX:live}"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "GET" request to "/${CTX:api}/health"
    * sheet check "DEPLC-005"
    When the user clicks "Stop" on gateway "it-gateway" in the Management Portal
    Then the Management Portal confirms stopping deployment "${CTX:live}"
    And the deployment status on gateway "it-gateway" becomes "Suspended"
    When the user clicks "View More" in the Management Portal
    And the user clicks "Delete ${CTX:live}" in the Management Portal
    Then the Management Portal confirms deleting deployment "${CTX:live}"
    And the Management Portal offers no "Delete ${CTX:live}" button
    And the Management Portal offers a "Delete ${CTX:first}" button

  @DEPLC-004 @DEPLC-003
  Scenario: Journey: a GraphQL API's live deployment can be neither deleted nor restored over itself
    Given I generate a unique handle from "deplc-ui-graphql-project" and store it as "project"
    And I generate a unique handle from "deplc-ui-graphql" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a GraphQL API "${CTX:api}" exists in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/graphql-apis/${CTX:api}/deploy"
    # Four deployments, so the full history, where deletion lives, is offered.
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "second"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "third"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "live"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "POST" request to "/${CTX:api}" with body:
      """
      {"query":"{ hello }"}
      """
    * sheet check "DEPLC-004"
    When the user clicks "View More" in the Management Portal
    Then the Management Portal offers a "Delete ${CTX:first}" button
    And the Management Portal offers no "Delete ${CTX:live}" button
    When the user closes the drawer in the Management Portal
    * sheet check "DEPLC-003"
    And the user clicks "Change deployment on it-gateway" on gateway "it-gateway" in the Management Portal
    And the user selects deployment "${CTX:live}" in the Management Portal
    Then the Management Portal's "Restore" button is disabled
    When the user selects deployment "${CTX:first}" in the Management Portal
    Then the Management Portal's "Restore" button is enabled
    When the user clicks "Cancel" in the Management Portal
    Then the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "POST" request to "/${CTX:api}" with body:
      """
      {"query":"{ hello }"}
      """

  # The console sends undeploy and restore without the gatewayId the control plane requires:
  # https://github.com/wso2/api-platform/issues/3900
  @DEPLC-001 @DEPLC-002 @DEPLC-005 @known-issue
  Scenario: Journey: a GraphQL API's deployment is stopped, redeployed and deleted from its deploy page
    Given I generate a unique handle from "deplc-ui-graphql-actions-project" and store it as "project"
    And I generate a unique handle from "deplc-ui-graphql-actions" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a GraphQL API "${CTX:api}" exists in project "${CTX:project}" on the control plane
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/graphql-apis/${CTX:api}/deploy"
    # Four deployments, so the full history, where deletion lives, is offered.
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "first"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "second"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "third"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the user clicks "Deploy" on gateway "it-gateway" in the Management Portal
    And the Management Portal confirms a deployment started and stores its name as "live"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "POST" request to "/${CTX:api}" with body:
      """
      {"query":"{ hello }"}
      """
    * sheet check "DEPLC-001"
    When the user clicks "Stop" on gateway "it-gateway" in the Management Portal
    Then the Management Portal confirms stopping deployment "${CTX:live}"
    And the deployment status on gateway "it-gateway" becomes "Suspended"
    And the gateway answers 404 to a "POST" request to "/${CTX:api}" with body:
      """
      {"query":"{ hello }"}
      """
    * sheet check "DEPLC-002"
    When the user clicks "Redeploy" on gateway "it-gateway" in the Management Portal
    Then the Management Portal confirms redeploying deployment "${CTX:live}"
    And the deployment status on gateway "it-gateway" becomes "Active"
    And the gateway answers 200 to a "POST" request to "/${CTX:api}" with body:
      """
      {"query":"{ hello }"}
      """
    * sheet check "DEPLC-005"
    When the user clicks "Stop" on gateway "it-gateway" in the Management Portal
    Then the Management Portal confirms stopping deployment "${CTX:live}"
    And the deployment status on gateway "it-gateway" becomes "Suspended"
    When the user clicks "View More" in the Management Portal
    And the user clicks "Delete ${CTX:live}" in the Management Portal
    Then the Management Portal confirms deleting deployment "${CTX:live}"
    And the Management Portal offers no "Delete ${CTX:live}" button
    And the Management Portal offers a "Delete ${CTX:first}" button
