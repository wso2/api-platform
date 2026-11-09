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

Feature: API Portal MCP server listing

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has MCP listing fixtures

  Scenario: The MCP listing shows both servers but not the REST API
    When the MCP listing shows both servers but not the REST API

  Scenario: Searching the MCP listing returns only the first server
    When the MCP listing search returns only the first server

  Scenario: Searching the MCP listing excludes the REST API
    When the MCP listing search excludes the REST API
