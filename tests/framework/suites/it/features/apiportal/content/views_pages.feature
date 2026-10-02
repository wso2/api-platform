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

Feature: API Portal view resolution and scoped pages

  Scenario: The organization root redirects to an existing fallback view
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

  Scenario: The organization root with a trailing slash redirects to a view
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

  Scenario: The portal root redirects to the configured organization view
    When I send an unauthenticated API Portal public "GET" request to "/api-portal"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

  Scenario: An administrator can delete a view while other views remain
    Given I generate a unique resource name from "portal-deletable-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Deletable View"}
      """
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404

  Scenario: Deleting an unknown view returns not found
    Given I generate a unique resource name from "portal-absent-view" and store it as "viewId"
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404

  Scenario: A view is resolved by its handle
    Given I generate a unique resource name from "portal-handle-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"View Handle Resolution Display Name"}
      """
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "View Handle Resolution Display Name"

  Scenario: A view display name is not accepted as its handle
    Given I generate a unique resource name from "portal-display-name-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Unresolvable View Display Name"}
      """
    When I send an authenticated API Portal "GET" request to "/views/Unresolvable%20View%20Display%20Name" as "admin"
    Then the response status code should be 404

  Scenario: A display name is not accepted by the API view filter
    Given I generate a unique resource name from "portal-filter-handle-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Filter Handle Display Name"}
      """
    When I send an authenticated API Portal "GET" request to "/apis?view=Filter%20Handle%20Display%20Name" as "admin"
    Then the response status code should be 404

  Scenario: A display name is not accepted by the MCP view filter
    Given I generate a unique resource name from "portal-mcp-filter-handle-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"MCP Filter Handle Display Name"}
      """
    When I send an authenticated API Portal "GET" request to "/mcp-servers?view=MCP%20Filter%20Handle%20Display%20Name" as "admin"
    Then the response status code should be 404

  Scenario: Deleting a view by display name does not delete the view
    Given I generate a unique resource name from "portal-delete-display-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Delete Display Name Only"}
      """
    When I send an authenticated API Portal "DELETE" request to "/views/Delete%20Display%20Name%20Only" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200

  Scenario: An omitted view filter leaves API listing unscoped
    When I send an authenticated API Portal "GET" request to "/apis" as "admin"
    Then the response status code should be 200

  Scenario: A view filter returns only APIs carrying its label
    Given I generate a unique resource name from "portal-page-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Page Filter Label"}
      """
    And I generate a unique resource name from "portal-page-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Page Filter View","labels":["${CTX:labelId}"]}
      """
    And I generate a unique resource name from "portal-page-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Page Filter API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"

  Scenario: Removing a view label hides an API from the view
    Given I generate a unique resource name from "portal-page-remove-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Removable Page Label"}
      """
    And I generate a unique resource name from "portal-page-remove-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Removable Page View","labels":["${CTX:labelId}"]}
      """
    And I generate a unique resource name from "portal-page-remove-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Removable Page API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "publisher"
    Then the response body should contain "${CTX:apiId}"
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON body:
      """
      {"labels":[]}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "publisher"
    Then the response body should not contain "${CTX:apiId}"

  Scenario: An API detail page is served inside a view containing its label
    Given I generate a unique resource name from "portal-detail-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Detail Label"}
      """
    And I generate a unique resource name from "portal-detail-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Detail View","labels":["${CTX:labelId}"]}
      """
    And I generate a unique resource name from "portal-detail-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Detail API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 200

  Scenario: An API detail page is hidden outside its view
    Given I generate a unique resource name from "portal-outside-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Outside Label"}
      """
    And I generate a unique resource name from "portal-outside-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Outside View"}
      """
    And I generate a unique resource name from "portal-outside-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Outside API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 404

  Scenario: An MCP detail page is served inside a view containing its label
    Given I generate a unique resource name from "portal-mcp-detail-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | MCP Detail     |
    And I generate a unique resource name from "portal-mcp-detail-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}        |
      | displayName | MCP Detail View      |
      | labels      | ["${CTX:labelId}"]  |
    And a unique API Portal MCP server with label "${CTX:labelId}" is created and stored as "mcpId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcp/${CTX:mcpId}"
    Then the response status code should be 200

  Scenario: An MCP detail page is hidden outside its view
    Given I generate a unique resource name from "portal-mcp-outside-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | MCP Outside    |
    And I generate a unique resource name from "portal-mcp-outside-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}   |
      | displayName | MCP Outside View |
    And a unique API Portal MCP server with label "${CTX:labelId}" is created and stored as "mcpId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcp/${CTX:mcpId}"
    Then the response status code should be 404

  Scenario: A view-scoped API markdown route is hidden outside its view
    Given I generate a unique resource name from "portal-markdown-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Markdown Label"}
      """
    And I generate a unique resource name from "portal-markdown-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Markdown View"}
      """
    And I generate a unique resource name from "portal-markdown-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Markdown Outside API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}.md"
    Then the response status code should be 404

  Scenario: A view-scoped API specification route is hidden outside its view
    Given I generate a unique resource name from "portal-spec-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Specification Label"}
      """
    And I generate a unique resource name from "portal-spec-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Specification View"}
      """
    And I generate a unique resource name from "portal-spec-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Specification Outside API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 404

  Scenario: An unknown API handle is hidden inside a valid view
    Given I generate a unique resource name from "portal-unknown-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Unknown API View"}
      """
    And I generate a unique resource name from "portal-unknown-api" and store it as "apiId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 404

  Scenario: Removing a label keeps an API hidden from a view-scoped page
    Given I generate a unique resource name from "portal-scoped-remove-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Scoped Removal Label"}
      """
    And I generate a unique resource name from "portal-scoped-remove-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id":"${CTX:viewId}","displayName":"Scoped Removal View","labels":["${CTX:labelId}"]}
      """
    And I generate a unique resource name from "portal-scoped-remove-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Scoped Removal API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 200
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON body:
      """
      {"labels":[]}
      """
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 404
