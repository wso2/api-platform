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

# The block restricts api_portal.health.allowed_clients to loopback, and the suite calls the
# portal from outside its container, so every health probe below comes from a refused client.
@health
Feature: API Portal health access restriction
  As a platform operator
  I want the API Portal health endpoints to answer only the clients I allow
  So that the portal's dependency state is not readable by any caller

  Scenario Outline: A client outside the allowed list is refused on <path> without any check result
    When I send a "GET" request to the API Portal health path "<path>"
    Then the response status code should be 403
    And the response header "Content-Type" should contain "application/json"
    And the response header "Cache-Control" should be "no-store"
    And the JSON response field "status" should be "forbidden"
    And the JSON response field "checks" should not exist
    And the JSON response field "timestamp" should not exist

    Examples:
      | path               |
      | /health            |
      | /ready             |
      | /api-portal/health |
      | /api-portal/ready  |

  Scenario Outline: A forged <header> header does not admit a refused client
    When I send a "GET" request to the API Portal health path "/ready" with header "<header>" set to "<value>"
    Then the response status code should be 403
    And the JSON response field "status" should be "forbidden"

    Examples:
      | header          | value         |
      | X-Forwarded-For | 127.0.0.1     |
      | X-Real-IP       | 127.0.0.1     |
      | Forwarded       | for=127.0.0.1 |

  Scenario: A refused client is refused before its method is considered
    When I send a "POST" request to the API Portal health path "/ready"
    Then the response status code should be 403
    And the response header "Allow" should not exist

  Scenario: A HEAD probe from a refused client is refused too
    When I send a "HEAD" request to the API Portal health path "/ready"
    Then the response status code should be 403

  Scenario: The Platform API health endpoints keep their own allowed list
    When I send a "GET" request to the platform-api health path "/ready"
    Then the response status code should be 200
    And the JSON response field "status" should be "ok"
