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

Feature: API Portal AI discovery resources

  Scenario: The portal-wide LLM index is publicly available
    When I send an unauthenticated API Portal public "GET" request to "/llms.txt"
    Then the response status code should be 200
    And the response body should contain "AI Agent Entry Point"

  Scenario: The view LLM index includes a visible API
    Given I generate a unique resource name from "portal-discovery" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Discoverable API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["default"],"endPoints":{"productionURL":"https://discovery.invalid","sandboxURL":"https://discovery.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/llms.txt"
    Then the response status code should be 200
    And the response body should contain "Discoverable API"

  Scenario: The API markdown catalog includes a visible API
    Given I generate a unique resource name from "portal-discovery" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Markdown Catalog API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["default"],"endPoints":{"productionURL":"https://discovery.invalid","sandboxURL":"https://discovery.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should contain "Markdown Catalog API"

  Scenario: The API markdown catalog includes a visible WebSub API
    Given I generate a unique resource name from "portal-websub-discovery" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websub" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSub Catalog API","version":"v1.0","type":"WEBSUB","status":"PUBLISHED","labels":["default"],"endPoints":{"productionURL":"https://websub-discovery.invalid","sandboxURL":"https://websub-discovery.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should contain "WebSub Catalog API"

  Scenario: Hidden APIs are excluded from the API markdown catalog
    Given I generate a unique resource name from "portal-hidden-markdown" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Hidden Markdown API","version":"v1.0","type":"REST","status":"PUBLISHED","agentVisibility":"HIDDEN","labels":["default"],"endPoints":{"productionURL":"https://hidden-markdown.invalid","sandboxURL":"https://hidden-markdown.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis.md"
    Then the response status code should be 200
    And the response body should not contain "Hidden Markdown API"

  Scenario: Hidden APIs are excluded from the public LLM index
    Given I generate a unique resource name from "portal-hidden-discovery" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Hidden Discovery API","version":"v1.0","type":"REST","status":"PUBLISHED","agentVisibility":"HIDDEN","labels":["default"],"endPoints":{"productionURL":"https://discovery.invalid","sandboxURL":"https://discovery.invalid"}}
      """
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/llms.txt"
    Then the response status code should be 200
    And the response body should not contain "Hidden Discovery API"

  Scenario: The view LLM index is unavailable when AI discovery is disabled
    When I verify API Portal view "default" returns not found when AI discovery is disabled as "publisher"
    Then the response status code should be 404
