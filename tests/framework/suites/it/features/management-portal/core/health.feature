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
Feature: Platform API liveness and readiness endpoints
  As a platform operator
  I want the Platform API to report whether it has started and can reach its database
  So that an orchestrator stops routing to a control plane that cannot serve requests

  Scenario Outline: The Platform API <role> endpoint reports every check up
    When I send a "GET" request to the platform-api health path "<path>"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the response header "Cache-Control" should be "no-store"
    And the JSON response field "status" should be "ok"
    And the JSON response should have field "timestamp"
    And the JSON response field "checks.database.status" should be "up"
    And the JSON response field "checks.database.error" should not exist
    And the JSON response field "checks.startup.status" should be "up"
    And the JSON response field "component" should not exist

    Examples:
      | role      | path    |
      | liveness  | /health |
      | readiness | /ready  |

  Scenario: A HEAD probe of Platform API readiness gets the status without a body
    When I send a "HEAD" request to the platform-api health path "/ready"
    Then the response status code should be 200
    And the response header "Cache-Control" should be "no-store"
    And the response body should be empty

  Scenario Outline: Platform API readiness refuses the <method> method and names the allowed ones
    When I send a "<method>" request to the platform-api health path "/ready"
    Then the response status code should be 405
    And the response header "Allow" should be "GET, HEAD"

    Examples:
      | method |
      | POST   |
      | DELETE |

  Scenario: Platform API health ignores credentials it cannot verify
    When I send a "GET" request to the platform-api health path "/ready" with header "Authorization" set to "Bearer not-a-valid-token"
    Then the response status code should be 200
    And the JSON response field "status" should be "ok"
