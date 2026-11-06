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

# The Platform API, the API Portal and the gateway controller in this block share one Postgres
# server, so a scenario that takes it down takes down the database of all three. The runtime
# uses no database.
@health
Feature: Health during a database outage
  As a platform operator
  I want every component that needs the database to report itself not ready while the database is unusable
  So that an orchestrator stops routing to it, and routes to it again as soon as the database is back

  Scenario: Every component is ready while the database is reachable
    When I send a "GET" request to the platform-api health path "/ready"
    Then the response status code should be 200
    And the JSON response field "checks.database.status" should be "up"
    When I send a "GET" request to the API Portal health path "/ready"
    Then the response status code should be 200
    And the JSON response field "checks.database.status" should be "up"
    When I send a "GET" request to the gateway controller health path "/api/admin/v1/ready"
    Then the response status code should be 200
    And the JSON response field "checks.database.status" should be "up"
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200

  Scenario: A stopped database fails every component that uses it, and each recovers when it returns
    When I stop the "postgres" database server
    And I send a "GET" request to the platform-api health path "/ready" until status 503
    Then the JSON response field "status" should be "unhealthy"
    And the JSON response field "checks.database.status" should be "down"
    And the JSON response field "checks.database.error" should be "database unreachable"
    And the JSON response should have field "checks.database.since"
    And the JSON response field "checks.startup.status" should be "up"
    And the response header "Cache-Control" should be "no-store"
    And the response body should not contain "it-postgres"
    And the response body should not contain "dial tcp"
    And the response should have arrived within 3 seconds
    When I send a "GET" request to the platform-api health path "/health"
    Then the response status code should be 503
    And the JSON response field "checks.database.status" should be "down"
    When I send a "HEAD" request to the platform-api health path "/ready"
    Then the response status code should be 503
    And the response body should be empty
    When I send a "GET" request to the API Portal health path "/ready" until status 503
    Then the JSON response field "status" should be "unhealthy"
    And the JSON response field "checks.database.error" should be "database unreachable"
    And the JSON response should have field "checks.database.since"
    And the response body should not contain "it-postgres"
    When I send a "GET" request to the gateway controller health path "/api/admin/v1/ready" until status 503
    Then the JSON response field "status" should be "unhealthy"
    And the JSON response field "checks.database.error" should be "database unreachable"
    And the response body should not contain "it-postgres"
    When I send a "GET" request to the gateway controller health path "/api/admin/v1/health"
    Then the response status code should be 503
    When I send a "GET" request to the gateway runtime health path "/_gateway-health/ready"
    Then the response status code should be 200
    And the JSON response field "checks.xds_connection.status" should be "up"
    When I start the "postgres" database server
    And I send a "GET" request to the platform-api health path "/ready" until status 200
    Then the JSON response field "status" should be "ok"
    And the JSON response field "checks.database.status" should be "up"
    And the JSON response field "checks.database.error" should not exist
    And the JSON response field "checks.database.since" should not exist
    When I send a "GET" request to the API Portal health path "/ready" until status 200
    Then the JSON response field "checks.database.status" should be "up"
    When I send a "GET" request to the gateway controller health path "/api/admin/v1/ready" until status 200
    Then the JSON response field "checks.database.status" should be "up"

  Scenario: A hung database fails readiness within the check timeout instead of hanging the probe
    When I pause the "postgres" database server
    And I send a "GET" request to the platform-api health path "/ready" until status 503
    Then the JSON response field "checks.database.status" should be "down"
    And the response body should match pattern "error.:.(timed out after [0-9.]+m?s|database connection pool exhausted)"
    And the response should have arrived within 3 seconds
    When I send a "GET" request to the API Portal health path "/ready" until status 503
    Then the JSON response field "checks.database.status" should be "down"
    And the response body should match pattern "error.:.(timed out after [0-9.]+m?s|database connection pool exhausted)"
    And the response should have arrived within 3 seconds
    When I send a "GET" request to the gateway controller health path "/api/admin/v1/ready" until status 503
    Then the JSON response field "checks.database.status" should be "down"
    And the response body should match pattern "error.:.(timed out after [0-9.]+m?s|database connection pool exhausted)"
    And the response should have arrived within 3 seconds
    When I unpause the "postgres" database server
    And I send a "GET" request to the platform-api health path "/ready" until status 200
    And I send a "GET" request to the API Portal health path "/ready" until status 200
    And I send a "GET" request to the gateway controller health path "/api/admin/v1/ready" until status 200
    Then the JSON response field "checks.database.status" should be "up"
