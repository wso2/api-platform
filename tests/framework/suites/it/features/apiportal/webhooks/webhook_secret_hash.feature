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

Feature: API Portal webhook deliveries name the secret that signed them

  Scenario: A delivery to a subscriber with a secret carries the hash of that secret
    Given I generate a unique resource name from "portal-secret-hash-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-secret-hash-secret" and store it as "secret"
    And I store the hash of the secret "secret" as "expectedHash"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Secret Hash Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","secret":"${CTX:secret}","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-secret-hash-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Secret Hash Application","description":"created"}
      """
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.created"
    And the JSON response field "secret_hash" should be "${CTX:expectedHash}"
    And the response body should not contain "${CTX:secret}"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "application.created" should be "DELIVERED"

  Scenario: A delivery to a subscriber without a secret carries no hash
    Given I generate a unique resource name from "portal-no-secret-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"No Secret Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"]}
      """
    And I generate a unique resource name from "portal-no-secret-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"No Secret Application","description":"created"}
      """
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.created"
    And the JSON response field "secret_hash" should not exist
