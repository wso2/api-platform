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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-126..127, at the HTTP level:
  # requests are sent to the portal host exactly as written ("root path"), with no session and no
  # Management API prefix. Every not-found answer is paired with a page of the configured organization that loads.

  @APIP-CORE-126 @type:smoke
  Scenario Outline: The health check at <path> answers without a session
    When I send an unauthenticated API Portal "GET" request to the root path "<path>"
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/json"
    And the JSON response field "status" should be "ok"

    Examples:
      | path               |
      | /health            |
      | /api-portal/health |

  @APIP-CORE-127 @type:negative
  Scenario Outline: A path <where> is not found, while the default view loads
    When I send an unauthenticated API Portal "GET" request to the root path "/api-portal/default/views/default"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to the root path "<path>"
    Then the response status code should be 404
    And the response header "Content-Type" should contain "<content type>"
    And the response body should contain "<marker>"
    And the response body should not contain "<absent>"

    Examples:
      | where                         | path                                        | content type | marker         | absent |
      | outside the portal prefix     | /not-the-portal/apis                        | text/plain   | Not Found      | <html  |
      | that the portal does not have | /api-portal/default/no-such-page            | text/html    | Page not found | stack  |
      | in a view that does not exist | /api-portal/default/views/no-such-view/apis | text/html    | Page not found | stack  |
