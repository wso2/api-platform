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
Feature: Gateway custom policies are synced into the organization by version
  As an API platform administrator
  I want to sync the custom policies a gateway reports into my organization
  So that APIs in the organization can use them, one record per policy and major version

  # The block's gateway loads the integration-test definitions under
  # core/catalog/platformgateway/resources/custom-policies, and reports each one its build
  # manifest lists as custom; it-builtin is not listed.
  Background:
    Given I store the registered gateway id as "gateway"
    And I send a "GET" request to the control plane at "/gateways/${CTX:gateway}/manifest" until the JSON array "policies" contains an item with "name" equal to "it-custom-basic"

  @GWCP-001
  Scenario: The gateway manifest marks the policies its build manifest lists as custom and the others as built in
    When I send a "GET" request to the control plane at "/gateways/${CTX:gateway}/manifest"
    Then the response status code should be 200
    And the JSON response array "policies" item with "name" equal to "it-custom-basic" should have "isCustomPolicy" equal to "true"
    And the JSON response array "policies" item with "name" equal to "it-custom-basic" should have "version" equal to "v1.0.0"
    And the JSON response array "policies" item with "name" equal to "it-custom-basic" should have "policyDefinition.parameters.properties.headerValue.type" equal to "string"
    And the JSON response array "policies" item with "name" equal to "it-builtin" should have "isCustomPolicy" equal to "false"
    And the JSON response array "policies" item with "name" equal to "it-builtin" should not have field "policyDefinition"

  @GWCP-002
  Scenario: Syncing a custom policy adds it to the organization with its display name and definition
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-basic&policyVersion=v1.0.0"
    Then the response status code should be 200
    And the JSON response field "name" should be "it-custom-basic"
    And the JSON response field "version" should be "v1.0.0"
    And the JSON response field "displayName" should be "IT Custom Basic"
    And the JSON response field "policyDefinition.parameters.properties.headerValue.type" should be "string"
    And I store the JSON response field "uuid" as "policyId"
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the response status code should be 200
    And the JSON response array "list" item with "uuid" equal to "${CTX:policyId}" should have "name" equal to "it-custom-basic"
    And the JSON response array "list" item with "uuid" equal to "${CTX:policyId}" should have "version" equal to "v1.0.0"
    When I send a "GET" request to the control plane at "/gateway-custom-policies/${CTX:policyId}/versions/v1.0.0"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "IT Custom Basic"

  @GWCP-003 @type:negative
  Scenario: Syncing a version the organization already holds is a conflict and keeps the record
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-basic&policyVersion=v1.0.0"
    Then the response status code should be 200
    And I store the JSON response field "uuid" as "policyId"
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-basic&policyVersion=v1.0.0"
    Then the response status code should be 409
    And the JSON response field "code" should be "POLICY_VERSION_CONFLICT"
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" item with "name" equal to "it-custom-basic" should have "uuid" equal to "${CTX:policyId}"

  @GWCP-004
  Scenario: Syncing a higher minor version replaces the organization's record of that major version
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-minor&policyVersion=v1.0.0"
    Then the response status code should be 200
    And I store the JSON response field "uuid" as "policyId"
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-minor&policyVersion=v1.1.0"
    Then the response status code should be 200
    And the JSON response field "uuid" should be "${CTX:policyId}"
    And the JSON response field "version" should be "v1.1.0"
    And the JSON response field "policyDefinition.parameters.properties.headerName.type" should be "string"
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" item with "name" equal to "it-custom-minor" should have "version" equal to "v1.1.0"
    When I send a "GET" request to the control plane at "/gateway-custom-policies/${CTX:policyId}/versions/v1.0.0"
    Then the response status code should be 404

  @GWCP-005 @type:negative
  Scenario: Syncing a lower minor version than the organization holds is refused
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-minor&policyVersion=v1.1.0"
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-minor&policyVersion=v1.0.0"
    Then the response status code should be 409
    And the JSON response field "code" should be "POLICY_VERSION_CONFLICT"
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" item with "name" equal to "it-custom-minor" should have "version" equal to "v1.1.0"

  @GWCP-006 @type:negative
  Scenario: Syncing a version that changes only the patch number is refused
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-patch&policyVersion=v1.0.0"
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-patch&policyVersion=v1.0.1"
    Then the response status code should be 409
    And the JSON response field "code" should be "POLICY_VERSION_CONFLICT"
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" item with "name" equal to "it-custom-patch" should have "version" equal to "v1.0.0"

  @GWCP-007
  Scenario: Two major versions of a custom policy are kept as two records
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-major&policyVersion=v1.0.0"
    Then the response status code should be 200
    And I store the JSON response field "uuid" as "majorOneId"
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-major&policyVersion=v2.0.0"
    Then the response status code should be 200
    And the JSON response field "uuid" should not equal "${CTX:majorOneId}"
    And the JSON response field "policyDefinition.parameters.properties.tagValue.type" should be "string"
    And I store the JSON response field "uuid" as "majorTwoId"
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" item with "uuid" equal to "${CTX:majorOneId}" should have "version" equal to "v1.0.0"
    And the JSON response array "list" item with "uuid" equal to "${CTX:majorTwoId}" should have "version" equal to "v2.0.0"

  @GWCP-008 @type:negative
  Scenario: Syncing a policy the gateway does not report as custom is refused
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-builtin&policyVersion=v1.0.0"
    Then the response status code should be 409
    And the JSON response field "code" should be "POLICY_INVALID_STATE"
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-basic&policyVersion=v1.0.0"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" should contain an item with "name" equal to "it-custom-basic"
    And the JSON response array "list" should not contain an item with "name" equal to "it-builtin"

  @GWCP-009 @type:negative
  Scenario: Syncing a policy version the gateway does not report is refused
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-basic&policyVersion=v1.0.9"
    Then the response status code should be 404
    And the JSON response field "code" should be "CUSTOM_POLICY_VERSION_NOT_FOUND"
    When I send a "POST" request to the control plane at "/gateway-custom-policies/sync?gatewayId=${CTX:gateway}&policyName=it-custom-basic&policyVersion=v1.0.0"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/gateway-custom-policies?limit=100"
    Then the JSON response array "list" item with "name" equal to "it-custom-basic" should have "version" equal to "v1.0.0"
