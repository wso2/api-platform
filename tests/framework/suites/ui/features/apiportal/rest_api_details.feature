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
Feature: API Portal API pages, documentation and consoles
  Journey APIPUI-003: an API page shows the header buttons, endpoints, resources, scopes and plans
  that fit its type, and its documentation renders the specification.
  Journey APIPUI-004: the built-in consoles: Try It for REST, GraphiQL for GraphQL and the AsyncAPI
  tryout for WebSocket and WebSub. Sending a request needs a gateway, which this stack does not run.

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has API page fixtures

  @APIP-004
  Scenario: Journey: a developer reads API pages of every type and their documentation
    When the user opens the seeded REST API overview and sees its details
    Then the REST API page header offers Subscribe, Documentation, Try with AI and API Keys
    And the GraphQL API page shows the sections of its type
    And the WebSocket API page shows the sections of its type
    And the WebSub API page shows the sections of its type
    And the SOAP API page shows the sections of its type
    And the page of the API without plans has no plans panel and no Subscribe button, and offers API Keys
    When the user opens the seeded REST API specification and sees its OpenAPI document

  @APIP-009 @known-issue
  Scenario: The REST API documentation lists the specification and additional document
    When the user opens the seeded REST API documentation and sees the additional document

  @APIP-004
  Scenario: Journey: a developer opens the Try It, GraphiQL and AsyncAPI consoles
    When the seeded REST API specification exposes the Try It console
    Then the GraphQL API Tryout offers the GraphiQL console with its production and sandbox endpoints
    And the WebSocket API Tryout opens the AsyncAPI tryout
    And the WebSub API Tryout opens the AsyncAPI tryout
