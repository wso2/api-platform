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
Feature: API Portal API workflows and Try with AI
  Journey APIPUI-007: the API Workflows gallery lists published workflows for developers and AI
  agents; an administrator creates, drafts, publishes, edits and deletes workflows in the
  settings; Try with AI and Discover with AI hand an agent a prompt to copy or download.

  @APIP-021
  Scenario: Journey: an administrator publishes API workflows, and a developer finds them and hands an API to an AI agent
    Given the user is signed in to the API Portal
    And the API Portal has a published Arazzo workflow, a published workflow hidden from agents and a draft workflow
    And the API Portal has an API visible to AI agents
    When the user signs out of the API Portal
    Then the API Workflows gallery lists only the published workflows, with AI Ready only on the visible one
    And the Arazzo workflow opens its flow and the Markdown workflow its page
    And the API's Try with AI prompt points agents at the API and downloads as a text file
    And the home page's Discover with AI prompt downloads as a text file
    When the administrator signs in to the API Portal
    Then the workflow form refuses to continue without a name or without content
    When the user creates an API workflow without a description
    And the user saves an API workflow as a draft, and the gallery does not list it until it is published
    And the user changes the description of that workflow and hides it from agents
    And the user deletes that workflow
