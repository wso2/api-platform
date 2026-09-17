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

Feature: API Portal API listing

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has API listing fixtures

  Scenario: The API listing shows REST and GraphQL APIs but not MCP
    When the API listing shows REST and GraphQL APIs but not MCP

  Scenario: Searching the API listing returns only the GraphQL API
    When the API listing search returns only the GraphQL API

  Scenario: Searching the API listing excludes the MCP server
    When the API listing search excludes the MCP server
