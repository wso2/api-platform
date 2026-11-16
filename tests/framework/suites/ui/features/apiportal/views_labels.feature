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
Feature: API Portal settings: views, labels and themes
  Journey APIPUI-008: an administrator creates, edits, renames and deletes views and labels, and
  each view serves only the APIs that carry its labels.
  Journey APIPUI-009: an administrator themes one view, sees the new look only there, downloads the
  theme and resets it.

  Background:
    Given the user is signed in to the API Portal

  @APIP-010 @APIP-011
  Scenario: Journey: an administrator manages views and labels, and each view serves the APIs of its labels
    When the administrator adds a label and a view with that label from their display names
    And the API Portal has an API with the settings label and an API with only the default label
    Then the settings view lists only the API with its label
    When the user adds the default label to the settings view
    Then the settings view lists both APIs
    When the user renames the settings view
    Then the old address of the renamed view is not found
    When the user renames the settings label
    And the user deletes the settings label, which an API uses
    Then the API that used the deleted label drops out of the settings view
    When the user deletes the settings view
    Then the deleted view's address is not found

  @APIP-012
  Scenario: Journey: an administrator themes a view, downloads the theme and resets it
    Given the API Portal has two views to theme
    When the user applies a theme to the first view in the settings
    Then the first view shows the theme, and the second view keeps the default look
    And the downloaded theme of the first view holds the applied stylesheet
    And a theme that holds only a script is refused, and the first view keeps its theme
    When the user resets the theme of the first view
    Then the first view is back to the default look
