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

@APIPORTAL
Feature: API Portal applications, subscriptions and API keys
  Journey APIPUI-015: a developer creates, edits and deletes applications and associates API keys
  with them.
  Journey APIPUI-013: a developer subscribes to an API and manages the subscription: its token,
  suspension, plan and unsubscription.
  Journey APIPUI-014: a developer generates and manages API keys for an API.
  Calling an API with the token or key needs a gateway, which this stack does not run.

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has an API with two free plans and API key security

  @APIP-014
  Scenario: Journey: a developer creates, edits and deletes an application and associates an API key with it
    When the developer signs in to the API Portal
    Then the developer's Applications page invites them to create their first application
    And the application form refuses an empty name and stops a name at 100 characters
    Given the API Portal has an application fixture
    And the developer has an API key for that API
    When the developer associates that API key with the application, then removes it
    Then the API key is still listed on the API's API Keys page
    And the application detail shows the key association sections
    And the user creates edits and deletes the application

  @APIP-025
  Scenario: Journey: a developer subscribes to an API and manages the subscription
    When a signed-out visitor who clicks Subscribe on that API is sent to sign in
    And the developer subscribes to the Bronze plan on the API page and sees the token once
    Then the API page offers to switch to the Silver plan
    When the developer reveals, regenerates, suspends and resumes the subscription in Subscriptions
    And the developer switches the subscription to the Silver plan, which keeps its token
    And the developer unsubscribes in Subscriptions
    Then the developer's Subscriptions page has no subscriptions
    And subscribing again gives the developer a new token

  @APIP-026
  Scenario: Journey: a developer generates, associates, regenerates and revokes an API key
    When the developer signs in to the API Portal
    And the API Portal has an application fixture
    When the developer generates an API key with an expiry on the API's API Keys page and sees it once
    Then the API key is listed with its name, status, expiry and no application
    When the developer associates the API key with the application from the API Keys page
    Then filtering the API Keys page by that application lists the key
    When the developer regenerates the API key and sees a new key once
    Then the Keys overview lists the API key with its API
    When the developer revokes the API key
    Then the API key is listed as revoked, without regenerate or revoke actions
