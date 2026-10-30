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

@TENANT
Feature: The Management Portal keeps each organization's APIs and gateways to itself
  As an organization administrator
  I want another organization's console to neither list nor open my APIs and gateways
  So that what my organization publishes stays inside it

  @TENANT-002 @TENANT-001 @TENANT-003
  Scenario: Journey: another organization can neither find nor open an organization's API and gateway
    Given I generate a unique handle from "tenant-ui-project" and store it as "project"
    And I generate a unique handle from "tenant-ui-api" and store it as "apiA"
    And I generate a unique handle from "tenant-ui-gateway" and store it as "gatewayA"
    And I generate a unique handle from "tenant-ui-own-api" and store it as "apiB"
    And I generate a unique handle from "tenant-ui-own-gateway" and store it as "gatewayB"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:apiA}" exists in project "${CTX:project}" on the control plane
    And a gateway "${CTX:gatewayA}" is registered on the control plane
    And a REST API "${CTX:apiB}" exists in project "default" on the control plane for "org B admin"
    And a gateway "${CTX:gatewayB}" is registered on the control plane for "org B admin"
    When the user signs in to the Management Portal as "org B admin"
    * sheet check "TENANT-002"
    And the user opens the Management Portal page "/organizations/it-second-org/projects/default/apis"
    Then the Management Portal shows "${CTX:apiB}"
    And the Management Portal does not show "${CTX:apiA}"
    When the user opens the Management Portal page "/organizations/it-second-org/gateways"
    Then the Management Portal shows "${CTX:gatewayB}"
    And the Management Portal does not show "${CTX:gatewayA}"
    * sheet check "TENANT-001"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:apiA}"
    Then the Management Portal is not on a page under "/organizations/default"
    And the Management Portal does not show "${CTX:apiA}"
    When the user opens the Management Portal page "/organizations/it-second-org/projects/default/apis/${CTX:apiA}"
    Then the Management Portal shows "API not found"
    When the user opens the Management Portal page "/organizations/it-second-org/gateways/${CTX:gatewayA}-absent"
    Then the Management Portal shows "Unable to load this gateway."
    When the user opens the Management Portal page "/organizations/it-second-org/gateways/${CTX:gatewayA}"
    Then the Management Portal shows "Unable to load this gateway."
    And the Management Portal does not show "${CTX:gatewayA}"
    * sheet check "TENANT-003"
    When the user opens the Management Portal page "/organizations/it-second-org/projects/default/apis/${CTX:apiA}/deploy"
    Then the Management Portal shows "API not found"
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:apiA}"
    Then the Management Portal shows "${CTX:apiA}"
    When the user opens the Management Portal page "/organizations/default/gateways/${CTX:gatewayA}"
    Then the Management Portal shows "${CTX:gatewayA}"

  @TENANT-004
  Scenario: Journey: an organization's deploy screen offers only its own gateways
    Given I generate a unique handle from "tenant-ui-deploy-project" and store it as "deployProject"
    And I generate a unique handle from "tenant-ui-deploy-api" and store it as "deployApi"
    And I generate a unique handle from "tenant-ui-deploy-gateway" and store it as "deployGatewayA"
    And I generate a unique handle from "tenant-ui-other-gateway" and store it as "deployGatewayB"
    And a project "${CTX:deployProject}" exists on the control plane
    And a REST API "${CTX:deployApi}" exists in project "${CTX:deployProject}" on the control plane
    And a gateway "${CTX:deployGatewayA}" is registered on the control plane
    And a gateway "${CTX:deployGatewayB}" is registered on the control plane for "org B admin"
    When the user signs in to the Management Portal as the administrator
    * sheet check "TENANT-004"
    And the user opens the Management Portal page "/organizations/default/projects/${CTX:deployProject}/apis/${CTX:deployApi}/deploy"
    Then the Management Portal shows "${CTX:deployGatewayA}"
    And the Management Portal does not show "${CTX:deployGatewayB}"
