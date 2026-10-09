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

@api-key-auth
Feature: API key authentication policy consistency across multiple gateways
  As an API developer running more than one gateway behind one control plane
  I want a key issued, revoked, or already in effect to be enforced identically by every gateway,
  including one that only joins after the key was issued
  So that which gateway happens to answer a request never changes whether a key works

  # Deployment and key lifecycle go through the control plane (platform-api), not the
  # gateway-controller's own REST API: an un-ordinaled platform-gateway lookup is rejected once
  # a block runs more than one replica, so these scenarios address "the first/second gateway"
  # explicitly throughout, the same way features/management-portal/deployments/multigateway.feature
  # does for its own deployment-consistency coverage.
  Background:
    Given the first gateway is running
    And the second gateway is running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "aka-multigateway-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @aka-g09 @aka-31
  Scenario: The same API key authenticates identically on every gateway
    Given I generate a unique resource name from "aka-s31" and store it as "apiHandle"
    And I generate a unique API context from "/aka-s31" and store it as "akaCtx"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:akaCtx}" and API key authentication
    And I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane
    And I deploy the "RestApi" "${CTX:apiHandle}" to the second gateway via the control plane
    Then I send a "GET" request to the first gateway "${CTX:akaCtx}/health" until status 401
    And I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 401
    When I issue an API key "${CTX:apiHandle}-multigateway-key-0123456789abcdef" for REST API "${CTX:apiHandle}" via the control plane
    And I set header "API-Key" to "${CTX:apiHandle}-multigateway-key-0123456789abcdef"
    Then I send a "GET" request to the first gateway "${CTX:akaCtx}/health" until status 200
    And I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 200
    When I set header "API-Key" to "invalid-${CTX:apiHandle}"
    Then I send a "GET" request to the first gateway "${CTX:akaCtx}/health" until status 401
    And I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 401

  @aka-g09 @aka-32
  Scenario: Revoking a key through the control plane is enforced on every gateway
    Given I generate a unique resource name from "aka-s32" and store it as "apiHandle"
    And I generate a unique API context from "/aka-s32" and store it as "akaCtx"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:akaCtx}" and API key authentication
    And I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane
    And I deploy the "RestApi" "${CTX:apiHandle}" to the second gateway via the control plane
    And I issue an API key "${CTX:apiHandle}-multigateway-key-0123456789abcdef" for REST API "${CTX:apiHandle}" via the control plane
    And I set header "API-Key" to "${CTX:apiHandle}-multigateway-key-0123456789abcdef"
    Then I send a "GET" request to the first gateway "${CTX:akaCtx}/health" until status 200
    And I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 200
    When I revoke the API key for REST API "${CTX:apiHandle}" via the control plane
    Then I send a "GET" request to the first gateway "${CTX:akaCtx}/health" until status 401
    And I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 401

  @aka-g09 @aka-33
  Scenario: A gateway that joins after the key was issued still enforces the same configuration
    Given I generate a unique resource name from "aka-s33" and store it as "apiHandle"
    And I generate a unique API context from "/aka-s33" and store it as "akaCtx"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:akaCtx}" and API key authentication
    And I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane
    And I issue an API key "${CTX:apiHandle}-multigateway-key-0123456789abcdef" for REST API "${CTX:apiHandle}" via the control plane
    And I set header "API-Key" to "${CTX:apiHandle}-multigateway-key-0123456789abcdef"
    Then I send a "GET" request to the first gateway "${CTX:akaCtx}/health" until status 200
    When I reset the request
    And I deploy the "RestApi" "${CTX:apiHandle}" to the second gateway via the control plane
    Then I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 401
    When I set header "API-Key" to "${CTX:apiHandle}-multigateway-key-0123456789abcdef"
    Then I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 200
    When I set header "API-Key" to "invalid-${CTX:apiHandle}"
    Then I send a "GET" request to the second gateway "${CTX:akaCtx}/health" until status 401
