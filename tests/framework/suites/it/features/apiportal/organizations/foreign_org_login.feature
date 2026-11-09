# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
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

@APIPORTAL @apiportal-foreign-org-login
Feature: API Portal organization-pinned login

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-FOREIGN-ORG-001..006, at the API level.
  # Runs in the api-portal-foreign-org block: a second portal ("api-portal-other-org") serves
  # "other-org" from the same platform-api, so each refusal is paired with the portal that accepts it.

  @APIP-FOREIGN-ORG-001
  Scenario: A portal serves its configured organization
    When I send an unauthenticated API Portal "GET" request to "/api-portal/other-org/views/default" using portal "api-portal-other-org"
    Then the response status code should be 200

  @APIP-FOREIGN-ORG-002 @type:security
  Scenario: A portal does not serve the platform API organization
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default" using portal "api-portal-other-org"
    Then the response status code should be 404

  @APIP-FOREIGN-ORG-003
  Scenario: A valid credential logs in to its matching organization portal
    When I submit an API Portal file login for actor "admin"
    Then the response status code should be 302
    And the response header "Location" should not contain "error="

  @APIP-FOREIGN-ORG-004 @type:security
  Scenario: A valid credential for another organization is rejected
    When I submit an API Portal file login for actor "admin" to organization "other-org" using portal "api-portal-other-org"
    Then the response status code should be 302
    And the response header "Location" should contain "error="

  @APIP-FOREIGN-ORG-005 @type:security
  Scenario: An organization mismatch is reported as a credential failure
    When I submit an API Portal file login for actor "admin" to organization "other-org" using portal "api-portal-other-org"
    Then the response status code should be 302
    And I store the API Portal response header "Location" as "mismatchLocation"
    When I submit an API Portal file login for actor "admin" with an invalid password to organization "other-org" using portal "api-portal-other-org"
    Then the response status code should be 302
    And the API Portal response header "Location" should equal stored "mismatchLocation"
    And the response header "Location" should contain "Invalid+username+or+password"

  @APIP-FOREIGN-ORG-006 @type:security
  Scenario: A rejected login does not establish a usable session
    When I submit an API Portal file login for actor "admin" to organization "other-org" using portal "api-portal-other-org"
    Then the response status code should be 302
    And the response header "Location" should contain "error="
    When I send an API Portal session request to "/organizations/other-org" using portal "api-portal-other-org"
    Then the response status code should be 401
