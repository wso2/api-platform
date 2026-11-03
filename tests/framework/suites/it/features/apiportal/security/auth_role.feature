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

Feature: API Portal role authorization

  Scenario: Role authorization mode is active
    When I send an authenticated API Portal "GET" request to "/key-managers" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/key-managers" as "narrow"
    Then the response status code should be 403

  Scenario: An admin operation is authorized by portal role grants
    And I generate a unique resource name from "portal-role-admin-label" and store it as "labelId"
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON values:
      | id          | ${CTX:labelId}     |
      | displayName | Role Admin Label   |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:labelId}" at "/labels" for cleanup

  Scenario: An unauthenticated caller is rejected
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response status code should be 401

  Scenario: A narrow actor is restricted by portal role grants
    When I send an authenticated API Portal "GET" request to "/apis" as "narrow"
    Then the response status code should be 200
    And I generate a unique resource name from "portal-role-narrow-app" and store it as "appId"
    When I send an authenticated API Portal "POST" request to "/applications" as "narrow" with JSON values:
      | id          | ${CTX:appId}         |
      | displayName | Role Narrow Application |
      | description | role                  |
    Then the response status code should be 403
