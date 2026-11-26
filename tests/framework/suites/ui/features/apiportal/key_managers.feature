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
Feature: API Portal settings: key managers, organization and webhooks
  Journey APIPUI-016: an administrator adds a key manager, which is offered to developers creating
  an OAuth2 key while it is enabled, and deletes it.
  Journey APIPUI-011: an administrator edits the organization's details.
  Journey APIPUI-017: an administrator adds, edits, disables and deletes a webhook.
  These change portal-wide state, so they run on a stack of their own, one at a time.

  Background:
    Given the user is signed in to the API Portal

  @APIP-015
  Scenario: Journey: an administrator adds a key manager, disables and enables it, and deletes it
    When the administrator creates a key manager
    Then the developer can choose that key manager when creating an OAuth2 key
    When the administrator disables that key manager, and it is no longer offered to the developer
    And the administrator enables that key manager, and it is again offered to the developer
    And the administrator deletes that key manager

  @APIP-028
  Scenario: Journey: an administrator edits the organization
    When the organization form refuses an invalid business owner email
    And the administrator changes the organization's name and business owner, and they are kept after a reload
    Then the organization's handle and IdP reference cannot be changed

  @APIP-050
  Scenario: Journey: an administrator adds, edits, disables and deletes a webhook
    When the webhook form refuses a webhook without a target URL or secret, or with an invalid URL
    And the administrator adds a webhook with a URL, secret, timeout and two events
    Then the webhook is listed with its events, its secret and enabled
    When the administrator edits the webhook leaving the secret blank, which keeps it
    And the administrator disables the webhook
    And the administrator deletes the webhook
