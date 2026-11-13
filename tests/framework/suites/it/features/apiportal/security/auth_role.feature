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


@APIPORTAL @apiportal-security
Feature: API Portal role authorization
  As an API Portal administrator running the portal in role mode
  I want each role to grant exactly the operations its role-to-scope mapping lists
  So that administrators run the portal, developers manage their own applications, and a narrow
  role only reads

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level, in
  # the api-portal-auth-role block (authorization.mode = role). The token's scope claim is ignored;
  # the portal expands the token's roles through its own mapping, where ap_admin grants every
  # operation, ap_subscriber manages applications, keys and subscriptions and reads the rest, and
  # dp_narrow_it only reads. Every refusal is paired with the same operation succeeding for a role
  # that grants it.

  @APIP-033 @type:security
  Scenario: Operations are authorized by portal role grants
    # Role authorization mode is active
    When I send an authenticated API Portal "GET" request to "/key-managers" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/key-managers" as "narrow"
    Then the response status code should be 403

    # An admin operation is authorized by portal role grants
    Given I generate a unique resource name from "portal-role-admin-label" and store it as "labelId"
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON values:
      | id          | ${CTX:labelId}     |
      | displayName | Role Admin Label   |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:labelId}" at "/labels" for cleanup

    # An unauthenticated caller is rejected
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response status code should be 401

    # A narrow actor is restricted by portal role grants
    When I send an authenticated API Portal "GET" request to "/apis" as "narrow"
    Then the response status code should be 200
    And I generate a unique resource name from "portal-role-narrow-app" and store it as "appId"
    When I send an authenticated API Portal "POST" request to "/applications" as "narrow" with JSON values:
      | id          | ${CTX:appId}         |
      | displayName | Role Narrow Application |
      | description | role                  |
    Then the response status code should be 403

    # A developer manages their own application but cannot change the organization's labels
    Given I generate a unique resource name from "portal-role-developer-app" and store it as "appId"
    And I generate a unique resource name from "portal-role-developer-label" and store it as "labelId"
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}               |
      | displayName | Role Developer Application |
      | description | role                       |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/labels" as "developer" with JSON values:
      | id          | ${CTX:labelId}       |
      | displayName | Role Developer Label |
    Then the response status code should be 403
    And the JSON response field "message" should be "Forbidden"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON values:
      | id          | ${CTX:labelId}       |
      | displayName | Role Developer Label |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:labelId}" at "/labels" for cleanup
