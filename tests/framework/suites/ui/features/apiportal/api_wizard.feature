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

Feature: API Portal settings API wizard

  Background:
    Given the user is signed in to the API Portal

  Scenario: An administrator creates an API with a thumbnail, a spec and documents
    When the user creates an API in the wizard with a thumbnail and documents
    Then the wizard API row shows its thumbnail

  Scenario: An administrator changes the thumbnail and documents of an API
    Given the user creates an API in the wizard with a thumbnail and documents
    When the user replaces the thumbnail and documents of the wizard API
    Then the wizard API shows the new thumbnail and documents

  Scenario: A rejected content package keeps the API and can be retried
    When the user creates an API in the wizard with a rejected content package
    Then the wizard lists every rejected entry and continues as an edit
    And the user retries with a valid content package

  Scenario: Importing content into an existing API asks first
    Given the user creates an API in the wizard with a thumbnail and documents
    When the user cancels replacing the content of the wizard API
    Then the user confirms replacing the content of the wizard API

  Scenario: An administrator creates an MCP server with a tools schema
    When the user creates an MCP server in the wizard
    Then the wizard MCP server row shows its thumbnail
