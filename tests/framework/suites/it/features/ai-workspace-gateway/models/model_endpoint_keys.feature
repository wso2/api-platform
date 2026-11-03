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

@model-endpoint-keys
Feature: API keys on model endpoints and Models
  As an API administrator
  I want keys scoped to a model endpoint or to one Model
  So that a client reaches only the models it was given

  An endpoint key reaches every Model the endpoint serves, and its whole catalog. A Model key
  reaches that Model only, on every endpoint serving it, and the catalog lists only that Model.
  A key used out of its scope gets the same 401 as any invalid key, so it cannot tell which
  models exist.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: An endpoint key reaches every Model and a Model key reaches one
    Given I generate a unique resource name from "keys-provider" and store it as "provider"
    And I generate a unique resource name from "keys-endpoint" and store it as "endpoint"
    And I generate a unique API context from "keys-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "keys-endpoint-key" and store it as "endpointKeyName"
    And I generate a unique resource name from "key-alpha" and store it as "alpha"
    And I generate a unique resource name from "key-beta" and store it as "beta"
    And I generate a unique resource name from "keys-alpha-key" and store it as "alphaKeyName"
    And I generate a unique resource name from "keys-no-such-model" and store it as "unknownModel"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:provider}                                                                                                                                   |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                         |
      | displayName        | ${CTX:provider}                                                                                                                                   |
      | version            | v1.0                                                                                                                                              |
      | template           | openai                                                                                                                                            |
      | accessControl.mode | allow_all                                                                                                                                         |
      | spec               | {"displayName": "${CTX:provider}", "version": "v1.0", "template": "openai", "exposure": "none", "upstream": {"url": "http://testbench:3002/keys-upstream"}} |
    Then the response status code should be 201
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:alpha}                                                                             |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                |
      | displayName | ${CTX:alpha}                                                                             |
      | version     | v1.0                                                                                     |
      | modelName   | ${CTX:alpha}                                                                             |
      | endpoints   | ["${CTX:endpoint}"]                                                                      |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "alpha-upstream"}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:beta}                                                                             |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                               |
      | displayName | ${CTX:beta}                                                                             |
      | version     | v1.0                                                                                    |
      | modelName   | ${CTX:beta}                                                                             |
      | endpoints   | ["${CTX:endpoint}"]                                                                     |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "beta-upstream"}] |
    Then the response status code should be 201

    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:endpointKeyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"
    When I send a "POST" request to the "gateway-controller" service at "/models/${CTX:alpha}/api-keys" with body:
      """
      {"name": "${CTX:alphaKeyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "alphaKey"
    When I send a "GET" request to the "gateway-controller" service at "/models/${CTX:alpha}/api-keys"
    Then the response status code should be 200
    And the response body should contain "${CTX:alphaKeyName}"

    # The endpoint key reaches both Models, and the catalog lists both.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:alpha}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "json.model" should be "alpha-upstream"
    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:beta}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "json.model" should be "beta-upstream"
    When I send a "GET" request to "${CTX:endpointContext}/models"
    Then the response status code should be 200
    And the JSON response array field "data" should have 2 items
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:alpha}"
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:beta}"
    # The endpoint key on an unknown model gets the model error, after authentication.
    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:unknownModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 404
    And the JSON response field "error.code" should be "model_not_found"

    # The Model key reaches its own Model...
    When I set header "Authorization" to "Bearer ${CTX:alphaKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:alpha}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "json.model" should be "alpha-upstream"

    # ...and nothing else: another Model and an unknown model get the same 401.
    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:beta}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 401
    And the JSON response field "error.type" should be "authentication_error"
    And the JSON response field "error.message" should be "Valid API key required"
    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:unknownModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 401
    And the JSON response field "error.type" should be "authentication_error"
    And the JSON response field "error.message" should be "Valid API key required"

    # The catalog admits the Model key and lists only its Model.
    When I send a "GET" request to "${CTX:endpointContext}/models"
    Then the response status code should be 200
    And the JSON response array field "data" should have 1 item
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:alpha}"

    # Deleting the Model revokes its keys.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the Model "${CTX:alpha}"
    Then the response status code should be 200
    When I clear all headers
    And I set header "Authorization" to "Bearer ${CTX:alphaKey}"
    And I send a "GET" request to "${CTX:endpointContext}/models" until status 401
    Then the JSON response field "error.type" should be "authentication_error"
