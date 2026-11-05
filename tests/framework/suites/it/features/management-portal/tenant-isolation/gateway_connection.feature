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
Feature: A gateway's connection reaches only its own organization's resources
  As an organization administrator
  I want another organization's gateway to get nothing of mine from the gateway internal API
  So that a gateway connection can never be used to read my APIs, subscriptions or secrets

  Background:
    Given the first gateway is running
    And I generate a unique value from "tenant-conn-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @TENANT-005
  Scenario: Another organization's gateway token gets not found for the API and its subscriptions, and none of the secrets
    Given I generate a unique resource name from "tenant-conn-secret" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "tenant-conn-api" and store it as "apiHandle"
    And I generate a unique API context from "/tenant-conn-api" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and a policy header secret "${CTX:secretHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I obtain an API key for the first gateway via the control plane
    And I resolve the gateway internal artifact id of REST API deployment "${CTX:deploymentId}" and store it as "apiUuid"
    When I send a "GET" request to the gateway internal API at "/apis/${CTX:apiUuid}"
    Then the response status code should be 200
    When I send a "GET" request to the gateway internal API at "/apis/${CTX:apiUuid}/subscriptions"
    Then the response status code should be 200
    When I send a "GET" request to the gateway internal API at "/secrets"
    Then the response status code should be 200
    And the response body should contain "${CTX:secretHandle}"
    When I obtain an API key for the second gateway via the control plane as "org B admin"
    And I send a "GET" request to the gateway internal API at "/apis/${CTX:apiUuid}"
    Then the response status code should be 404
    When I send a "GET" request to the gateway internal API at "/apis/${CTX:apiUuid}/subscriptions"
    Then the response status code should be 404
    When I send a "GET" request to the gateway internal API at "/secrets"
    Then the response status code should be 200
    And the response body should not contain "${CTX:secretHandle}"
