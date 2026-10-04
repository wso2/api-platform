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

Feature: API Portal applications

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has an application fixture

  Scenario: An administrator creates edits and deletes an application
    When the user creates edits and deletes the application

  Scenario: An application with a key manager shows key controls
    Given the API Portal has a key manager fixture
    When the application detail shows key manager controls

  Scenario: Application credentials can be added generated and revoked
    Given the API Portal has a key manager fixture
    When the user adds generates and revokes application credentials

  Scenario: Multiple key managers render isolated controls
    Given the API Portal has two key manager fixtures
    When the application page shows isolated controls for both key managers

  Scenario: Multiple key managers keep separate client mappings
    Given the API Portal has two key manager fixtures
    When the user links separate clients to both key managers

  Scenario: The second key manager generates a token in its own modal
    Given the API Portal has two key manager fixtures
    When the user generates a token from the second key manager

  Scenario: The first key manager generates a token in its own modal
    Given the API Portal has two key manager fixtures
    When the user generates a token from the first key manager

  Scenario: A key manager token failure stays in its own card
    Given the API Portal has two key manager fixtures
    When the user generates a token with an invalid secret from the second key manager

  Scenario: Revoking one key manager does not affect the other
    Given the API Portal has two key manager fixtures
    When the user revokes only the second key manager credentials
