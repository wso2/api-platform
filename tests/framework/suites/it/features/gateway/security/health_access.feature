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

# The block restricts router.health.allowed_clients to loopback, and the suite calls the
# runtime from outside the container, so every health probe below comes from a refused client.
@health
Feature: Gateway runtime health access restriction
  As a platform operator
  I want the runtime health routes to answer only the clients I allow
  So that the dependency state they expose is not readable from the traffic listener

  Scenario Outline: A client outside the allowed list is refused on <path>
    When I send a "GET" request to the gateway runtime health path "<path>"
    Then the response status code should be 403
    And the response body should contain "RBAC: access denied"
    And the response body should not contain "policy_engine"
    And the response body should not contain "xds_connection"

    Examples:
      | path                     |
      | /_gateway-health/ready   |
      | /_gateway-health/healthy |

  Scenario Outline: A forged <header> header does not admit a refused client
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready" with header "<header>" set to "<value>"
    Then the response status code should be 403
    And the response body should not contain "policy_engine"

    Examples:
      | header                   | value         |
      | X-Forwarded-For          | 127.0.0.1     |
      | X-Real-IP                | 127.0.0.1     |
      | Forwarded                | for=127.0.0.1 |
      | X-Envoy-External-Address | 127.0.0.1     |

  Scenario: A refused client is refused on every virtual host
    Given I generate a unique value from "health-access-vhost" and store it as "virtualHost"
    And I set request host to "${CTX:virtualHost}.example.com"
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 403

  Scenario: API traffic is not affected by the health restriction
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "health-access-api" and store it as "apiName"
    And I generate a unique API version from "health-access" and store it as "apiVersion"
    And I generate a unique API context from "/health-access" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                          |
      | name                   | ${CTX:apiName}                                     |
      | spec.displayName       | ${CTX:apiName}                                     |
      | spec.version           | ${CTX:apiVersion}                                  |
      | spec.context           | ${CTX:apiContext}/$version                         |
      | spec.upstream.main.url | http://testbench:3000/api/v2                       |
      | spec.operations        | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/us/seattle" until status 200
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 403

  Scenario: The controller health endpoints keep their own allowed list
    When I send a "GET" request to the gateway controller health path "/api/admin/v1/ready"
    Then the response status code should be 200
    And the JSON response field "status" should be "healthy"
