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

@platform-api-artifact-management
Feature: Artifacts keep their definition, project and authorship through their lifecycle
  As an API publisher
  I want a REST API, LLM provider or LLM proxy to keep what I gave it and record who changed it
  So that the control plane is a reliable record on every supported database

  Background:
    Given I generate a unique resource name from "art-proj-a" and store it as "projectA"
    And I generate a unique resource name from "art-proj-b" and store it as "projectB"
    And I create a project "${CTX:projectA}" on the control plane
    And I create a project "${CTX:projectB}" on the control plane

  Scenario: A REST API round-trips its definition and records who created and updated it
    Given I generate a unique resource name from "art-api" and store it as "apiName"
    And I generate a unique API context from "/art-api" and store it as "apiContext"
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:projectA}", "description": "created", "upstream": {"main": {"url": "http://backend.example.com"}}, "operations": [{"request": {"method": "GET", "path": "/health"}}]}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "apiHandle"
    And the response header "Location" should be "/api/v0.9/rest-apis/${CTX:apiHandle}"
    And the JSON response field "lifeCycleStatus" should be "CREATED"
    And the JSON response field "projectId" should be "${CTX:projectA}"
    And the JSON response field "createdBy" should be "admin"
    And the JSON response field "updatedBy" should be "admin"
    When I send a "PUT" request to the control plane at "/rest-apis/${CTX:apiHandle}" as "publisher" with body:
      """
      {"displayName": "${CTX:apiName} updated", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:projectA}", "description": "updated", "lifeCycleStatus": "PUBLISHED", "upstream": {"main": {"url": "http://backend.example.com"}}, "operations": [{"request": {"method": "GET", "path": "/health"}}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiName} updated"
    And the JSON response field "description" should be "updated"
    And the JSON response field "lifeCycleStatus" should be "PUBLISHED"
    And the JSON response field "createdBy" should be "admin"
    And the JSON response field "updatedBy" should be "publisher"

  Scenario: The REST API list is filtered by project
    Given I generate a unique resource name from "art-list-a" and store it as "apiA"
    And I generate a unique resource name from "art-list-b" and store it as "apiB"
    And I generate a unique API context from "/art-list-a" and store it as "contextA"
    And I generate a unique API context from "/art-list-b" and store it as "contextB"
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiA}", "context": "${CTX:contextA}", "version": "v1", "projectId": "${CTX:projectA}", "upstream": {"main": {"url": "http://backend.example.com"}}}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "handleA"
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiB}", "context": "${CTX:contextB}", "version": "v1", "projectId": "${CTX:projectB}", "upstream": {"main": {"url": "http://backend.example.com"}}}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "handleB"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:projectA}"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.total" should be 1
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:handleA}"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:projectB}"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:handleB}"
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:projectA}-missing"
    Then the response status code should be 404

  Scenario: LLM providers and proxies record their creator as their last updater
    Given I generate a unique resource name from "art-prov" and store it as "providerId"
    And I generate a unique resource name from "art-proxy" and store it as "proxyId"
    And I generate a unique API context from "/art-prov" and store it as "providerContext"
    And I generate a unique API context from "/art-proxy" and store it as "proxyContext"
    When I send a "POST" request to the control plane at "/llm-providers" with body:
      """
      {"id": "${CTX:providerId}", "displayName": "${CTX:providerId}", "version": "v1.0", "context": "${CTX:providerContext}", "template": "openai", "upstream": {"main": {"url": "http://backend.example.com"}}, "accessControl": {"mode": "allow_all"}}
      """
    Then the response status code should be 201
    And the JSON response field "createdBy" should be "admin"
    And the JSON response field "updatedBy" should be "admin"
    When I send a "POST" request to the control plane at "/llm-proxies" as "publisher" with body:
      """
      {"id": "${CTX:proxyId}", "displayName": "${CTX:proxyId}", "version": "v1.0", "projectId": "${CTX:projectA}", "context": "${CTX:proxyContext}", "provider": {"id": "${CTX:providerId}"}}
      """
    Then the response status code should be 201
    And the JSON response field "createdBy" should be "publisher"
    And the JSON response field "updatedBy" should be "publisher"
