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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@health
Feature: Gateway liveness and readiness endpoints
  As a platform operator
  I want the gateway controller and runtime to report their own health and their dependencies' health
  So that an orchestrator restarts or stops routing to a gateway that cannot serve

  Scenario Outline: The controller <role> endpoint reports every check up
    When I send a "GET" request to the gateway controller health path "<path>"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the response header "Cache-Control" should be "no-store"
    And the JSON response field "status" should be "healthy"
    And the JSON response should have field "timestamp"
    And the JSON response field "checks.database.status" should be "up"
    And the JSON response field "checks.database.error" should not exist
    And the JSON response field "checks.startup.status" should be "up"
    And the JSON response field "component" should not exist

    Examples:
      | role      | path                 |
      | liveness  | /api/admin/v1/health |
      | readiness | /api/admin/v1/ready  |

  Scenario: A HEAD probe of controller readiness gets the status without a body
    When I send a "HEAD" request to the gateway controller health path "/api/admin/v1/ready"
    Then the response status code should be 200
    And the response header "Cache-Control" should be "no-store"
    And the response body should be empty

  Scenario Outline: Controller readiness refuses the <method> method
    When I send a "<method>" request to the gateway controller health path "/api/admin/v1/ready"
    Then the response status code should be 405

    Examples:
      | method |
      | POST   |
      | DELETE |

  Scenario: The legacy controller liveness path still answers and points at its successor
    When I send a "GET" request to the gateway controller health path "/health"
    Then the response status code should be 200
    And the JSON response field "status" should be "healthy"
    And the response header "Deprecation" should be "true"
    And the response header "Link" should contain "</api/admin/v1/health>"
    And the response header "Link" should contain "successor-version"
    And the response header "Warning" should contain "Deprecated API"

  Scenario: The controller has no legacy readiness path
    When I send a "GET" request to the gateway controller health path "/ready"
    Then the response status code should be 404

  Scenario: Runtime readiness reports the policy engine and its control-plane connection
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the response header "Cache-Control" should be "no-store"
    And the JSON response field "status" should be "ready"
    And the JSON response should have field "timestamp"
    And the JSON response field "checks.policy_engine.status" should be "up"
    And the JSON response field "checks.xds_connection.status" should be "up"

  Scenario: Runtime liveness reports only the policy engine
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/healthy"
    Then the response status code should be 200
    And the JSON response field "status" should be "healthy"
    And the JSON response field "checks.policy_engine.status" should be "up"
    And the JSON response field "checks.xds_connection" should not exist

  Scenario: Runtime health routes answer on any virtual host
    Given I generate a unique value from "health-vhost" and store it as "virtualHost"
    And I set request host to "${CTX:virtualHost}.example.com"
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200
    And the JSON response field "status" should be "ready"

  Scenario: A path under the runtime health namespace that is not a health route is not answered as one
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready/extra"
    Then the response status code should be 404
    And the response body should not contain "policy_engine"

  Scenario Outline: An API cannot claim a route in the reserved health namespace (<case>)
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "health-reserved-api" and store it as "apiName"
    And I generate a unique API version from "health-reserved" and store it as "apiVersion"
    And I generate a unique API context from "/health-reserved" and store it as "apiContext"
    And I generate a unique value from "probe" and store it as "probeSegment"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}               |
      | name                   | ${CTX:apiName}                          |
      | spec.displayName       | ${CTX:apiName}                          |
      | spec.version           | ${CTX:apiVersion}                       |
      | spec.context           | <context>                               |
      | spec.upstream.main.url | http://testbench:3000/api/v2            |
      | spec.operations        | [{"method":"GET","path":"<operation>"}] |
    Then the response should be a client error
    And the response body should contain "reserved gateway health-check namespace"
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200
    And the JSON response field "status" should be "ready"

    Examples:
      | case                                 | context                              | operation                    |
      | context inside the namespace         | /_gateway-health/${CTX:probeSegment} | /status                      |
      | operation resolving to a health path | ${CTX:apiContext}/$version           | /../../_gateway-health/ready |
