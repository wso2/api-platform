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

@policy-engine-admin
Feature: Policy engine admin API
  As a gateway administrator
  I want to access the policy engine admin API
  So that I can inspect the current configuration and debug issues
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Config dump endpoint returns valid JSON
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the response header "Content-Type" should be "application/json"
    And the response should be valid JSON

  Scenario: Config dump contains a policy registry section
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the JSON response should have field "policy_registry"
    And the JSON response should have field "policy_registry.total_policies"
    And the JSON response should have field "policy_registry.policies"

  Scenario: Config dump contains a policy chains section
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the JSON response should have field "policy_chains"
    And the JSON response should have field "policy_chains.total_policy_chains"
    And the JSON response should have field "policy_chains.policy_chains"

  Scenario: Config dump contains a route metadata section
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the JSON response should have field "route_metadata"
    And the JSON response should have field "route_metadata.total_routes"
    And the JSON response should have field "route_metadata.routes"

  Scenario: Config dump contains a lazy resources section
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the JSON response should have field "lazy_resources"
    And the JSON response should have field "lazy_resources.total_resources"

  Scenario: Config dump reflects a deployed API's route
    Given I generate a unique value from "policy-admin-route" and store it as "apiName"
    And I generate a unique API version from "policy-admin-route" and store it as "apiVersion"
    And I generate a unique API context from "/policy-admin-route" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/info","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Test-Header","value":"test-value"}]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the JSON response field "policy_chains.total_policy_chains" should be greater than 0
    And the config dump should contain route with base path "${CTX:apiContext}/${CTX:apiVersion}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  @known-issue
  Scenario: Config dump reflects an API's deletion
    Given I generate a unique value from "policy-admin-delete" and store it as "apiName"
    And I generate a unique API version from "policy-admin-delete" and store it as "apiVersion"
    And I generate a unique API context from "/policy-admin-delete" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/info"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the config dump should contain route with base path "${CTX:apiContext}/${CTX:apiVersion}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

    # The policy engine's own route_metadata rebuild after a deletion has been measured taking
    # longer than the generic dump-consistency wait's ceiling, so this polls with its own,
    # longer-patience timeout rather than a fixed sleep.
    Then I wait for the config dump to stop containing a route with base path "${CTX:apiContext}/${CTX:apiVersion}"

  Scenario: Config dump shows a policy's parameters
    Given I generate a unique value from "policy-admin-params" and store it as "apiName"
    And I generate a unique API version from "policy-admin-params" and store it as "apiVersion"
    And I generate a unique API context from "/policy-admin-params" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/test","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Custom-Header","value":"custom-value"}]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the config dump should contain policy "set-headers" for route "${CTX:apiContext}/${CTX:apiVersion}/test"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A POST request to the config dump endpoint returns 405
    When I send a "POST" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 405

  Scenario: Multiple APIs sync correctly via xDS
    Given I generate a unique value from "policy-admin-xds-1" and store it as "api1Name"
    And I generate a unique API version from "policy-admin-xds-1" and store it as "api1Version"
    And I generate a unique API context from "/policy-admin-xds-1" and store it as "api1Context"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:api1Name}                   |
      | spec.displayName       | ${CTX:api1Name}                   |
      | spec.version           | ${CTX:api1Version}                |
      | spec.context           | ${CTX:api1Context}/$version       |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/test"}] |
    Then the response should be successful

    Given I generate a unique value from "policy-admin-xds-2" and store it as "api2Name"
    And I generate a unique API version from "policy-admin-xds-2" and store it as "api2Version"
    And I generate a unique API context from "/policy-admin-xds-2" and store it as "api2Context"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:api2Name}                   |
      | spec.displayName       | ${CTX:api2Name}                   |
      | spec.version           | ${CTX:api2Version}                |
      | spec.context           | ${CTX:api2Context}/$version       |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"POST","path":"/data"}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:api1Context}/${CTX:api1Version}/health" until status 200
    And I send a "GET" request to "${CTX:api2Context}/${CTX:api2Version}/health" until status 200

    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the response status code should be 200
    And the config dump should contain route with base path "${CTX:api1Context}/${CTX:api1Version}"
    And the config dump should contain route with base path "${CTX:api2Context}/${CTX:api2Version}"

    When I delete the API "${CTX:api1Name}"
    Then the response should be successful
    When I delete the API "${CTX:api2Name}"
    Then the response should be successful

  Scenario: An API update syncs via xDS
    Given I generate a unique value from "policy-admin-xds-update" and store it as "apiName"
    And I generate a unique API version from "policy-admin-xds-update" and store it as "apiVersion"
    And I generate a unique API context from "/policy-admin-xds-update" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/original"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the config dump should contain route with base path "${CTX:apiContext}/${CTX:apiVersion}"

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/original"},{"method":"POST","path":"/new-endpoint"}] |
    Then the response should be successful

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/new-endpoint" until status 200 with body:
      """
      """
    And I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the config dump should contain route with base path "${CTX:apiContext}/${CTX:apiVersion}"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
