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

@model-endpoint-multiple
Feature: Several model endpoints on one gateway
  As an API administrator
  I want several model endpoints on one gateway
  So that different consumers get their own context, keys and catalog

  A gateway serves any number of model endpoints beside the provisioned "wso2". A Model names
  the endpoints that serve it; each endpoint has its own context, policies, keys and catalog, and
  serves only the Models that name it.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Each endpoint serves the Models that name it
    Given I generate a unique resource name from "multi-provider" and store it as "provider"
    And I generate a unique resource name from "partners-endpoint" and store it as "partners"
    And I generate a unique API context from "partners-endpoint" and store it as "partnersContext"
    And I generate a unique resource name from "partners-endpoint-key" and store it as "partnersKeyName"
    And I generate a unique resource name from "public-endpoint" and store it as "public"
    And I generate a unique API context from "public-endpoint" and store it as "publicContext"
    And I generate a unique resource name from "public-endpoint-key" and store it as "publicKeyName"
    And I generate a unique resource name from "partners-copy" and store it as "partnersCopy"
    And I generate a unique API context from "partners-copy" and store it as "partnersCopyContext"
    And I generate a unique resource name from "partners-clash" and store it as "partnersClash"
    And I generate a unique resource name from "shared-model" and store it as "sharedModel"
    And I generate a unique resource name from "partner-only-model" and store it as "partnerOnlyModel"
    And I generate a unique resource name from "public-only-model" and store it as "publicOnlyModel"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:provider}                                                                                                                                    |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                          |
      | displayName        | ${CTX:provider}                                                                                                                                    |
      | version            | v1.0                                                                                                                                               |
      | template           | openai                                                                                                                                             |
      | accessControl.mode | allow_all                                                                                                                                          |
      | spec               | {"displayName": "${CTX:provider}", "version": "v1.0", "template": "openai", "exposure": "none", "upstream": {"url": "http://testbench:3002/multi-upstream"}} |
    Then the response status code should be 201

    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:partners}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:partners}           |
      | version     | v1.0                      |
      | context     | ${CTX:partnersContext}    |
    Then the response status code should be 201
    # With no policies of its own, an endpoint authenticates with api-key-auth.
    And the JSON response field "spec.policies[0].name" should be "api-key-auth"
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:public}             |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:public}             |
      | version     | v1.0                      |
      | context     | ${CTX:publicContext}      |
    Then the response status code should be 201

    # Another endpoint cannot take its display name, nor its context.
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:partnersCopy}        |
      | apiVersion  | ${CTX:gatewaySpecVersion}  |
      | displayName | ${CTX:partners}            |
      | version     | v1.0                       |
      | context     | ${CTX:partnersCopyContext} |
    Then the response status code should be 409
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:partnersClash}      |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:partnersClash}      |
      | version     | v1.0                      |
      | context     | ${CTX:partnersContext}    |
    Then the response status code should be 409

    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:sharedModel}                                                                        |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                 |
      | displayName | ${CTX:sharedModel}                                                                        |
      | version     | v1.0                                                                                      |
      | modelName   | ${CTX:sharedModel}                                                                        |
      | endpoints   | ["${CTX:partners}", "${CTX:public}"]                                                      |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "shared-upstream"}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:partnerOnlyModel}                                                                    |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                  |
      | displayName | ${CTX:partnerOnlyModel}                                                                    |
      | version     | v1.0                                                                                       |
      | modelName   | ${CTX:partnerOnlyModel}                                                                    |
      | endpoints   | ["${CTX:partners}"]                                                                        |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "partner-upstream"}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:publicOnlyModel}                                                                    |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                 |
      | displayName | ${CTX:publicOnlyModel}                                                                    |
      | version     | v1.0                                                                                      |
      | modelName   | ${CTX:publicOnlyModel}                                                                    |
      | endpoints   | ["${CTX:public}"]                                                                         |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "public-upstream"}] |
    Then the response status code should be 201

    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:partners}/api-keys" with body:
      """
      {"name": "${CTX:partnersKeyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "partnersKey"
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:public}/api-keys" with body:
      """
      {"name": "${CTX:publicKeyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "publicKey"

    # The partner endpoint serves the Models that name it, on its own context.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:partnersKey}"
    And I send a "POST" request to "${CTX:partnersContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:sharedModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "path" should be "/multi-upstream/chat/completions"
    And the JSON response field "json.model" should be "shared-upstream"
    When I send a "POST" request to "${CTX:partnersContext}/chat/completions" with body:
      """
      {"model": "${CTX:partnerOnlyModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "json.model" should be "partner-upstream"

    # A Model that does not name it is unknown there.
    When I send a "POST" request to "${CTX:partnersContext}/chat/completions" with body:
      """
      {"model": "${CTX:publicOnlyModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 404
    And the JSON response field "error.code" should be "model_not_found"

    # An endpoint's key works on that endpoint only.
    When I set header "Authorization" to "Bearer ${CTX:publicKey}"
    And I send a "POST" request to "${CTX:partnersContext}/chat/completions" with body:
      """
      {"model": "${CTX:sharedModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 401
    When I send a "POST" request to "${CTX:publicContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:sharedModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "json.model" should be "shared-upstream"
    When I send a "POST" request to "${CTX:publicContext}/chat/completions" with body:
      """
      {"model": "${CTX:partnerOnlyModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 404

    # Each catalog lists its own endpoint's Models.
    When I set header "Authorization" to "Bearer ${CTX:partnersKey}"
    And I send a "GET" request to "${CTX:partnersContext}/models"
    Then the response status code should be 200
    And the JSON response array field "data" should have 2 items
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:sharedModel}"
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:partnerOnlyModel}"
    When I set header "Authorization" to "Bearer ${CTX:publicKey}"
    And I send a "GET" request to "${CTX:publicContext}/models"
    Then the response status code should be 200
    And the JSON response array field "data" should have 2 items
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:sharedModel}"
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:publicOnlyModel}"

    # An endpoint a deployed Model names cannot be deleted.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the model endpoint "${CTX:partners}"
    Then the response status code should be 409
    And the response body should contain "${CTX:partnerOnlyModel}"

    # Undeploying one endpoint leaves the others serving.
    When I update model endpoint "${CTX:partners}" from "resources/templates/model-endpoint.yaml" with values:
      | name                 | ${CTX:partners}           |
      | apiVersion           | ${CTX:gatewaySpecVersion} |
      | displayName          | ${CTX:partners}           |
      | version              | v1.0                      |
      | context              | ${CTX:partnersContext}    |
      | spec.deploymentState | undeployed                |
    Then the response status code should be 200
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:partnersKey}"
    And I send a "POST" request to "${CTX:partnersContext}/chat/completions" until status 404 with body:
      """
      {"model": "${CTX:sharedModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    When I set header "Authorization" to "Bearer ${CTX:publicKey}"
    And I send a "POST" request to "${CTX:publicContext}/chat/completions" with body:
      """
      {"model": "${CTX:sharedModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "json.model" should be "shared-upstream"
