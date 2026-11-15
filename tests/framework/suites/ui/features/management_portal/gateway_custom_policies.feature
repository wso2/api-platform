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

@GWCP
Feature: The Management Portal syncs a gateway's custom policies and attaches them to an API
  As an organization administrator
  I want to sync the custom policies my gateway reports and attach one to an API
  So that the organization's APIs can use the policies its gateways provide

  # The block's gateway reports the integration-test definitions under
  # core/catalog/platformgateway/resources/custom-policies; it-builtin is not listed as custom.
  @GWCP-001 @GWCP-002 @GWCP-010 @GWCP-011
  Scenario: Journey: an administrator syncs a gateway's custom policy and its next minor version, and attaches it to an API
    Given I generate a unique handle from "console-gwcp-project" and store it as "project"
    And I generate a unique handle from "console-gwcp" and store it as "api"
    And a project "${CTX:project}" exists on the control plane
    And a REST API "${CTX:api}" is imported from spec fixture "petstore-v1.yaml" in project "${CTX:project}" on the control plane
    And the custom policy "it-custom-minor" synced in the Management Portal is deleted after the scenario
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/gateways/it-gateway"
    And the user opens the "Policies" tab in the Management Portal
    And the user searches for "it-builtin" in the "Search policies" search field in the Management Portal
    Then gateway policy "it-builtin" version "v1.0.0" in the Management Portal shows "Policy Hub"
    And gateway policy "it-builtin" version "v1.0.0" in the Management Portal shows "Built-in"
    When the user searches for "it-custom-minor" in the "Search policies" search field in the Management Portal
    Then gateway policy "it-custom-minor" version "v1.0.0" in the Management Portal shows "Custom"
    When the user clicks "Sync" on gateway policy "it-custom-minor" version "v1.0.0" in the Management Portal
    Then the Management Portal shows "Synced it-custom-minor v1.0.0 to the organization."
    And gateway policy "it-custom-minor" version "v1.0.0" in the Management Portal shows "Synced"
    And gateway policy "it-custom-minor" version "v1.1.0" in the Management Portal shows "Replaces v1.0.0"
    When the user clicks "Sync" on gateway policy "it-custom-minor" version "v1.1.0" in the Management Portal
    Then the Management Portal shows "Synced it-custom-minor v1.1.0 to the organization."
    And gateway policy "it-custom-minor" version "v1.1.0" in the Management Portal shows "Synced"
    And gateway policy "it-custom-minor" version "v1.0.0" in the Management Portal shows "Organization version: v1.1.0"
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/policies"
    Then the policy catalog's "IT Custom Minor" entry in the Management Portal shows "Custom"
    When the user chooses "IT Custom Minor" in the Management Portal
    Then the Management Portal shows "Applies to Global policies"
    And the Management Portal's "Attach policy" button is disabled
    When the user enters "console" as the "Header value" in the Management Portal
    And the user enters "X-Console" as the "Header name" in the Management Portal
    And the user clicks "Attach policy" in the Management Portal
    And the user clicks "Save changes" in the Management Portal
    Then the Management Portal shows "Policies saved."
    # A fresh load of the page: the policy below is read back from the control plane.
    When the user opens the Management Portal page "/organizations/default/projects/${CTX:project}/apis/${CTX:api}/develop/policies"
    Then the Management Portal shows global policy "it-custom-minor"
