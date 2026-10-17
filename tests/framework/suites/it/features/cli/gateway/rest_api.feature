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

@ap-cli-gateway
Feature: ap CLI REST API commands
  As a CLI user
  I want to manage REST APIs on the gateway
  So that I can list, view, and delete deployed APIs

  Background:
    Given the "ap" CLI is available
    And the CLI uses the platform gateway as "api-gw"

  @legacy:GW-API-001
  Scenario: List APIs
    Given I generate a unique resource name from "cli-rest-api" and store it as "apiName"
    And I generate a unique API context from "/cli-rest-api" and store it as "apiContext"
    And I apply API from "resources/templates/rest-api.yaml" through the CLI with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name       | ${CTX:apiName}            |
      | spec       | {"displayName":"${CTX:apiName}","version":"v1.0","context":"${CTX:apiContext}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    When I run ap with arguments "gateway rest-api list"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:apiName}"

  # Other runners deploy APIs to the same gateway, so the list is never empty. The scenario
  # checks that an API the list showed is gone from it once deleted.
  @legacy:GW-API-002
  Scenario: List APIs no longer shows a deleted API
    Given I generate a unique resource name from "cli-rest-api" and store it as "apiName"
    And I generate a unique API context from "/cli-rest-api" and store it as "apiContext"
    And I apply API from "resources/templates/rest-api.yaml" through the CLI with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name       | ${CTX:apiName}            |
      | spec       | {"displayName":"${CTX:apiName}","version":"v1.0","context":"${CTX:apiContext}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    And I run ap with arguments "gateway rest-api delete --id ${CTX:apiName}"
    And the CLI command should succeed
    When I run ap with arguments "gateway rest-api list"
    Then the CLI command should succeed
    And the CLI output should not contain "${CTX:apiName}"

  @legacy:GW-API-003
  Scenario: Get an existing API
    Given I generate a unique resource name from "cli-rest-api" and store it as "apiName"
    And I generate a unique API context from "/cli-rest-api" and store it as "apiContext"
    And I apply API from "resources/templates/rest-api.yaml" through the CLI with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name       | ${CTX:apiName}            |
      | spec       | {"displayName":"${CTX:apiName}","version":"v1.0","context":"${CTX:apiContext}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    When I run ap with arguments "gateway rest-api get --id ${CTX:apiName}"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:apiName}"

  @legacy:GW-API-004
  Scenario: Getting an API that does not exist is rejected
    Given I generate a unique resource name from "cli-missing-api" and store it as "missingApi"
    When I run ap with arguments "gateway rest-api get --id ${CTX:missingApi}"
    Then the CLI command should fail
    And the CLI output should contain "not found"

  @legacy:GW-API-005
  Scenario: Delete an existing API
    Given I generate a unique resource name from "cli-rest-api" and store it as "apiName"
    And I generate a unique API context from "/cli-rest-api" and store it as "apiContext"
    And I apply API from "resources/templates/rest-api.yaml" through the CLI with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name       | ${CTX:apiName}            |
      | spec       | {"displayName":"${CTX:apiName}","version":"v1.0","context":"${CTX:apiContext}","upstream":{"main":{"url":"http://testbench:3000"}},"operations":[{"method":"GET","path":"/data"}]} |
    When I run ap with arguments "gateway rest-api delete --id ${CTX:apiName}"
    Then the CLI command should succeed
    When I run ap with arguments "gateway rest-api get --id ${CTX:apiName}"
    Then the CLI command should fail
    And the CLI output should contain "not found"

  @legacy:GW-API-006
  Scenario: Deleting an API that does not exist is rejected
    Given I generate a unique resource name from "cli-missing-api" and store it as "missingApi"
    When I run ap with arguments "gateway rest-api delete --id ${CTX:missingApi}"
    Then the CLI command should fail
    And the CLI output should contain "not found"
