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


@APIPORTAL @apiportal-organization-webhook
Feature: Webhook events name the organization by its control plane reference
  As a control plane that mirrors the API Portal's organization
  I want every webhook event to carry the organization's control plane reference ID
  So that I can resolve the organization an event belongs to, even after the reference changes

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level. It runs
  # in the api-portal-webhooks block, one runner at a time, because the reference is organization-wide
  # state that every event of every runner carries. The organization's settings are written back
  # when the scenario ends, pass or fail.

  @APIP-028
  Scenario: An event carries the organization's control plane reference ID as org.ref_id, and follows its change
    # An event carries the organization's control plane reference ID as org.ref_id, and follows its change
    Given I generate a unique resource name from "portal-cp-ref" and store it as "cpRefId"
    And I generate a unique resource name from "portal-org-ref-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-org-ref-app" and store it as "appId"
    And I generate a unique resource name from "portal-org-default-ref-app" and store it as "defaultRefAppId"
    And the API Portal organization "default" is restored to its current settings after the scenario
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.created"] |
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And I store the JSON response field "displayName" as "organizationName"

    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON values:
      | id          | default                 |
      | idpRefId    | default                 |
      | displayName | ${CTX:organizationName} |
      | cpRefId     | ${CTX:cpRefId}          |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the JSON response field "cpRefId" should be "${CTX:cpRefId}"
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}              |
      | displayName | Organization Ref App      |
      | description | Carries the reference set |
    And I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "org.ref_id" should be "${CTX:cpRefId}"
    And the JSON response field "data.handle" should be "${CTX:appId}"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "application.created" should be "DELIVERED"

    # Positive control: the reference the suite seeds ("default", the organization's handle) is
    # what an event carries once the reference is set back.
    When I send an authenticated API Portal "PUT" request to "/organizations/default" as "admin" with JSON values:
      | id          | default                 |
      | idpRefId    | default                 |
      | displayName | ${CTX:organizationName} |
      | cpRefId     | default                 |
    Then the response status code should be 200
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "defaultRefAppId":
      | id          | ${CTX:defaultRefAppId}        |
      | displayName | Default Ref App               |
      | description | Carries the seeded reference  |
    And I wait for an API Portal webhook event "application.created" containing "${CTX:defaultRefAppId}"
    Then the JSON response field "org.ref_id" should be "default"
    And the JSON response field "data.handle" should be "${CTX:defaultRefAppId}"
