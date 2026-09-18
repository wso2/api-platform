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

Feature: API Portal file-based authentication

  Scenario: A valid file-based login establishes a session
    When I submit an API Portal file login for actor "admin"
    Then the response status code should be 302
    And the response header "Location" should not contain "error="
    And the API Portal response should set a cookie named "connect.sid"

  Scenario: An incorrect password is rejected by file-based login
    When I submit an API Portal file login for actor "admin" with an invalid password
    Then the response status code should be 302
    And the response header "Location" should contain "error="

  Scenario: A nonexistent username is rejected by file-based login
    When I submit an API Portal file login for a nonexistent user
    Then the response status code should be 302
    And the response header "Location" should contain "error="

  Scenario: A file-based login with a missing password is rejected
    When I submit an API Portal file login with a missing password
    Then the response status code should be 302
    And the response header "Location" should contain "Username+and+password+are+required"

  Scenario: A file-based login session grants access to an authenticated endpoint
    When I submit an API Portal file login for actor "admin"
    Then the response status code should be 302
    And the API Portal response should set a cookie named "connect.sid"
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 200

  Scenario: An unauthenticated caller cannot access an authenticated endpoint
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response should be a client error
