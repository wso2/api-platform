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

@GENAPP
Feature: LLM providers and LLM proxies are associated with applications
  As an application owner
  I want to associate my application with the LLM providers and LLM proxies it calls
  So that I can see which of its keys belong to each of them

  # Scenarios GENAPP-004 to 006 (scenarios/on-prem/GENAPP.yaml). Associations are stored by the control
  # plane alone, so no artifact is deployed.

  Background:
    Given I authenticate using basic auth as "admin"
    And I clear all headers
    And I generate a unique resource name from "genapp-project" and store it as "project"
    And I create a project "${CTX:project}" on the control plane
    And I generate a unique resource name from "genapp-app" and store it as "app"
    And I create an application "${CTX:app}" via the control plane in project "${CTX:project}"
    And I generate a unique resource name from "genapp-provider" and store it as "provider"
    And I generate a unique API context from "/genapp-provider" and store it as "providerContext"
    And I create an LLM provider "${CTX:provider}" via the control plane with context "${CTX:providerContext}" referencing template "openai"

  @GENAPP-004
  Scenario: An associated provider and proxy are listed once each, and a removed association is no longer listed
    Given I generate a unique resource name from "genapp-secret" and store it as "secret"
    And I create a secret "${CTX:secret}" with value "sk-genapp" via the control plane
    And I generate a unique resource name from "genapp-proxy" and store it as "proxy"
    And I generate a unique API context from "/genapp-proxy" and store it as "proxyContext"
    And I create an LLM proxy "${CTX:proxy}" via the control plane in project "${CTX:project}" with context "${CTX:proxyContext}" referencing provider "${CTX:provider}" and secret "${CTX:secret}"
    # The provider is named twice in the request.
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:provider}", "kind": "LlmProvider"}, {"id": "${CTX:proxy}", "kind": "LlmProxy"}, {"id": "${CTX:provider}", "kind": "LlmProvider"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/associations"
    Then the response status code should be 200
    And the JSON response field "count" should be 2
    And the JSON response array "list" item with "id" equal to "${CTX:provider}" should have "kind" equal to "LlmProvider"
    And the JSON response array "list" item with "id" equal to "${CTX:proxy}" should have "kind" equal to "LlmProxy"
    When I send a "DELETE" request to the control plane at "/applications/${CTX:app}/associations/${CTX:provider}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/associations"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:proxy}"
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:provider}"

  @GENAPP-005 @type:negative
  Scenario: A proxy in another project and a REST API cannot be associated with an application
    Given I generate a unique resource name from "genapp-other" and store it as "otherProject"
    And I create a project "${CTX:otherProject}" on the control plane
    And I generate a unique resource name from "genapp-secret" and store it as "secret"
    And I create a secret "${CTX:secret}" with value "sk-genapp" via the control plane
    And I generate a unique resource name from "genapp-proxy" and store it as "proxy"
    And I generate a unique API context from "/genapp-proxy" and store it as "proxyContext"
    And I create an LLM proxy "${CTX:proxy}" via the control plane in project "${CTX:otherProject}" with context "${CTX:proxyContext}" referencing provider "${CTX:provider}" and secret "${CTX:secret}"
    And I generate a unique resource name from "genapp-api" and store it as "api"
    And I generate a unique API context from "/genapp-api" and store it as "apiContext"
    And I create a REST API "${CTX:api}" via the control plane in project "${CTX:project}" with context "${CTX:apiContext}"
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:proxy}", "kind": "LlmProxy"}]}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should contain "different project"
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:api}", "kind": "RestApi"}]}
      """
    Then the response status code should be 400
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/associations"
    Then the response status code should be 200
    And the JSON response field "count" should be 0

  @GENAPP-006
  Scenario: The keys listed for an association are only the associated provider's keys
    Given I generate a unique resource name from "genapp-other-provider" and store it as "otherProvider"
    And I generate a unique API context from "/genapp-other" and store it as "otherContext"
    And I create an LLM provider "${CTX:otherProvider}" via the control plane with context "${CTX:otherContext}" referencing template "openai"
    And I send a "POST" request to the control plane at "/llm-providers/${CTX:provider}/api-keys" with body:
      """
      {"id": "x-key-1", "displayName": "X key 1"}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/llm-providers/${CTX:provider}/api-keys" with body:
      """
      {"id": "x-key-2", "displayName": "X key 2"}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/llm-providers/${CTX:otherProvider}/api-keys" with body:
      """
      {"id": "y-key-1", "displayName": "Y key 1"}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:provider}", "kind": "LlmProvider"}]}
      """
    And the response status code should be 200
    And I send a "POST" request to the control plane at "/applications/${CTX:app}/api-keys" with body:
      """
      {"apiKeys": [{"keyId": "x-key-1", "associatedEntity": {"id": "${CTX:provider}"}}, {"keyId": "x-key-2", "associatedEntity": {"id": "${CTX:provider}"}}, {"keyId": "y-key-1", "associatedEntity": {"id": "${CTX:otherProvider}"}}]}
      """
    And the response status code should be 200
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/associations/${CTX:provider}/api-keys"
    Then the response status code should be 200
    And the JSON response field "count" should be 2
    And the JSON response array "list" item with "keyId" equal to "x-key-1" should have "associatedEntity.id" equal to "${CTX:provider}"
    And the JSON response array "list" item with "keyId" equal to "x-key-2" should have "associatedEntity.id" equal to "${CTX:provider}"
    And the JSON response array "list" should not contain an item with "keyId" equal to "y-key-1"

  # The response to adding associations should list the application's associations, as the listing
  # does. It lists none, though its pagination counts them: the service windows the list with
  # limit -1, which the pagination helper treats as an empty window (#4117).
  @known-issue @type:regression
  Scenario: The response to associating a provider lists the application's associations
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:provider}", "kind": "LlmProvider"}]}
      """
    Then the response status code should be 200
    And the JSON response field "pagination.total" should be 1
    And the JSON response field "count" should be 1
    And the JSON response array "list" item with "id" equal to "${CTX:provider}" should have "kind" equal to "LlmProvider"

  # Adding an association the application already has should leave it listed once. On PostgreSQL it
  # answers 500: the repository skips the duplicate-key error inside its transaction, but PostgreSQL
  # has already aborted the transaction, so the commit fails (#4118).
  @known-issue @type:regression
  Scenario: Adding a provider the application is already associated with keeps it listed once
    Given I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:provider}", "kind": "LlmProvider"}]}
      """
    And the response status code should be 200
    When I send a "POST" request to the control plane at "/applications/${CTX:app}/associations" with body:
      """
      {"associations": [{"id": "${CTX:provider}", "kind": "LlmProvider"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/applications/${CTX:app}/associations"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:provider}"
