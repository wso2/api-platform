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
Feature: API Portal settings: APIs and their content
  Journey APIPUI-005: an administrator adds APIs in the step-by-step wizard, which checks its input,
  then edits, deprecates, publishes again and deletes them, and the catalogue follows each change.
  Journey APIPUI-010: an administrator imports an API's content package (landing page, icon and
  documents) and sets the LLM instructions AI agents read for a view.

  Background:
    Given the user is signed in to the API Portal

  @APIP-001 @APIP-002 @APIP-003
  Scenario: Journey: an administrator adds, edits, deprecates, republishes and deletes APIs in the settings
    When the user checks the details step of a new API in the wizard
    Then the wizard accepts only the spec files of the chosen API type
    And the wizard refuses to leave the details step while its required fields are empty
    When the user creates an API in the wizard with a thumbnail and documents
    Then the wizard API row shows its thumbnail
    And the wizard API shows in the catalogue with its type, tag, plan and documents
    And the wizard marks the current spec of the wizard API when a new one is chosen
    And the wizard keeps the thumbnail of the wizard API when a new file is rejected
    And switching between documents and import clears the pending changes of the wizard API
    When the user replaces the thumbnail and documents of the wizard API
    Then the wizard API shows the new thumbnail and documents
    When the user removes the thumbnail of the wizard API
    Then the wizard API row shows the first two letters of its name
    When the user deprecates the wizard API from its row menu
    Then the catalogue marks the wizard API as deprecated
    When the user publishes the wizard API from its row menu
    Then the catalogue no longer marks the wizard API as deprecated
    Given the API Portal has two APIs without subscriptions and one a developer subscribed to
    When the user deletes those three APIs together from the settings table
    Then the two APIs without subscriptions are deleted and the subscribed API is kept

  @APIP-009 @APIP-013
  Scenario: Journey: an administrator imports API content and sets the LLM instructions agents read
    When the user creates an API in the wizard with a rejected content package
    Then the wizard lists every rejected entry and continues as an edit
    And the user retries with a valid content package
    And the wizard shows a removed thumbnail as gone when the import after it is rejected
    Given the user creates an API in the wizard from a content package without a production URL
    And the user creates an API in the wizard with a thumbnail and documents
    When the user cancels replacing the content of the wizard API
    Then the user confirms replacing the content of the wizard API
    And the wizard API page shows the imported landing page and icon, and keeps its plans
    And a later documents-only edit keeps the imported content of the wizard API
    And a content package thumbnail replaces the thumbnail chosen in the same save
    And a new document replaces only the document of the same name in its own category
    Given the API Portal has a view of its own for LLM instructions
    When the user sets the LLM instructions of that view and previews them
    Then the llms.txt of that view uses the saved instructions
    When the user turns AI discovery off for that view
    Then that view serves no llms.txt and no longer offers AI Ready or Try with AI
