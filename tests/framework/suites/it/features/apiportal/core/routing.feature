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

@APIPORTAL @apiportal-routing
Feature: The API Portal answers health checks and unknown paths predictably
  As an operator running the API Portal behind a load balancer
  I want health checks to answer without a session and unknown paths to be refused cleanly
  So that probes see a live portal and nothing outside the portal is ever served by it

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the HTTP level:
  # requests are sent to the portal host exactly as written ("root path"), with no session and no
  # Management API prefix. Every not-found answer is paired with a page of the configured organization that loads.

  @APIP-008 @type:negative @type:smoke
  Scenario: The portal's roots redirect to a view, health checks answer, and unknown paths are not found
    # The portal root redirects to a view of the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/api-portal"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

    # The organization root redirects to a view of the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

    # The organization root with a slash redirects to a view of the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

    # The portal root redirects to the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/api-portal"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default"

    # The health check at /health answers without a session
    When I send an unauthenticated API Portal "GET" request to the root path "/health"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the JSON response field "status" should be "ok"

    # The health check at /api-portal/health answers without a session
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/health"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the JSON response field "status" should be "ok"

    # A path outside the portal prefix is not found, while the default view loads
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/default/views/default"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to the root path "/not-the-portal/apis"
    Then the response status code should be 404
    And the response header "Content-Type" should contain "text/plain"
    And the response body should contain "Not Found"
    And the response body should not contain "<html"

    # A path that the portal does not have is not found, while the default view loads
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/default/views/default"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/default/no-such-page"
    Then the response status code should be 404
    And the response header "Content-Type" should contain "text/html"
    And the response body should contain "Page not found"
    And the response body should not contain "stack"

    # A path in a view that does not exist is not found, while the default view loads
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/default/views/default"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/default/views/no-such-view/apis"
    Then the response status code should be 404
    And the response header "Content-Type" should contain "text/html"
    And the response body should contain "Page not found"
    And the response body should not contain "stack"
