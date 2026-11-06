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
Feature: API Portal liveness and readiness endpoints
  As a platform operator
  I want the API Portal to report whether it can reach its database
  So that an orchestrator stops routing to a portal that cannot serve pages

  Scenario Outline: The API Portal health path <path> reports its database up
    When I send a "GET" request to the API Portal health path "<path>"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the response header "Cache-Control" should be "no-store"
    And the JSON response field "status" should be "ok"
    And the JSON response should have field "timestamp"
    And the JSON response field "checks.database.status" should be "up"
    And the JSON response field "checks.database.error" should not exist

    Examples:
      | path               |
      | /health            |
      | /ready             |
      | /api-portal/health |
      | /api-portal/ready  |

  Scenario: A HEAD probe of API Portal readiness gets the status without a body
    When I send a "HEAD" request to the API Portal health path "/ready"
    Then the response status code should be 200
    And the response header "Cache-Control" should be "no-store"
    And the response body should be empty

  Scenario Outline: API Portal readiness refuses the <method> method and names the allowed ones
    When I send a "<method>" request to the API Portal health path "/ready"
    Then the response status code should be 405
    And the response header "Allow" should be "GET, HEAD"

    Examples:
      | method |
      | POST   |
      | PUT    |
      | DELETE |

  Scenario: An API Portal health probe starts no session
    When I send a "GET" request to the API Portal health path "/ready"
    Then the response status code should be 200
    And the response header "Set-Cookie" should not exist
