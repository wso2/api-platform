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

@model-endpoint-formats
Feature: Model endpoints in the Anthropic and Gemini formats
  As an API administrator
  I want model endpoints that speak the Anthropic and Gemini wire formats
  So that Claude and Gemini clients call Models without changing their SDKs

  An endpoint's inbound template decides the wire format its clients speak: its operations,
  where the model is read, and the shape of its errors and catalog. A provider on the endpoint's
  own format passes through; any other provider needs a transformer entry, and a Model naming an
  endpoint that has none for one of its providers is refused. Providers forward to the testbench
  echo service, which reflects the request.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: An Anthropic endpoint serves the Messages API
    Given I generate a unique resource name from "anthropic-native-provider" and store it as "nativeProvider"
    And I generate a unique resource name from "anthropic-openai-provider" and store it as "openaiProvider"
    And I generate a unique resource name from "anthropic-endpoint" and store it as "endpoint"
    And I generate a unique API context from "anthropic-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "anthropic-endpoint-key" and store it as "keyName"
    And I generate a unique resource name from "claude-native" and store it as "model"
    And I generate a unique resource name from "gpt-on-anthropic" and store it as "refusedModel"
    # The provider's base path is the endpoint's own context: the path the gateway builds for it
    # must not be mistaken for a request to rewrite again.
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:nativeProvider}                                                                                                                                                                                                                   |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                               |
      | displayName        | ${CTX:nativeProvider}                                                                                                                                                                                                                   |
      | version            | v1.0                                                                                                                                                                                                                                    |
      | template           | anthropic                                                                                                                                                                                                                               |
      | accessControl.mode | allow_all                                                                                                                                                                                                                               |
      | spec               | {"displayName": "${CTX:nativeProvider}", "version": "v1.0", "template": "anthropic", "exposure": "none", "upstream": {"url": "http://testbench:3002${CTX:endpointContext}", "auth": {"type": "api-key", "header": "x-api-key", "value": "anthropic-upstream-secret"}}} |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:openaiProvider}                                                                                                                               |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                           |
      | displayName        | ${CTX:openaiProvider}                                                                                                                               |
      | version            | v1.0                                                                                                                                                |
      | template           | openai                                                                                                                                              |
      | accessControl.mode | allow_all                                                                                                                                           |
      | spec               | {"displayName": "${CTX:openaiProvider}", "version": "v1.0", "template": "openai", "exposure": "none", "upstream": {"url": "http://testbench:3002/openai-upstream"}} |
    Then the response status code should be 201

    # Claude clients send their key in x-api-key.
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name                 | ${CTX:endpoint}                                                                          |
      | apiVersion           | ${CTX:gatewaySpecVersion}                                                                |
      | displayName          | ${CTX:endpoint}                                                                          |
      | version              | v1.0                                                                                     |
      | context              | ${CTX:endpointContext}                                                                   |
      | spec.inboundTemplate | anthropic                                                                                |
      | spec.policies        | [{"name": "api-key-auth", "version": "v1", "params": {"key": "x-api-key", "in": "header"}}] |
    Then the response status code should be 201

    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                                     |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                        |
      | displayName | ${CTX:model}                                                                                     |
      | version     | v1.0                                                                                             |
      | modelName   | ${CTX:model}                                                                                     |
      | endpoints   | ["${CTX:endpoint}"]                                                                              |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:nativeProvider}"}, "upstreamModel": "claude-haiku-4-5"}] |
    Then the response status code should be 201
    # An OpenAI provider needs a transformer on an Anthropic endpoint, and this one has none: the
    # Model is refused, naming the endpoint, its format and the provider.
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:refusedModel}                                                                    |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                              |
      | displayName | ${CTX:refusedModel}                                                                    |
      | version     | v1.0                                                                                   |
      | modelName   | ${CTX:refusedModel}                                                                    |
      | endpoints   | ["${CTX:endpoint}"]                                                                    |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:openaiProvider}"}, "upstreamModel": "gpt-4o"}] |
    Then the response status code should be 400
    And the response body should contain "endpoint '${CTX:endpoint}' (inbound format 'anthropic') has no transformer for provider '${CTX:openaiProvider}'"

    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    # Errors are in the Anthropic shape, authentication first.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "anthropic-version" to "2023-06-01"
    And I send a "POST" request to "${CTX:endpointContext}/v1/messages" until status 401 with body:
      """
      {"model": "${CTX:model}", "max_tokens": 16, "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "type" should be "error"
    And the JSON response field "error.type" should be "authentication_error"

    # An Anthropic provider speaks the endpoint's format and passes through, asked for the
    # target's upstream model with its own credential.
    When I set header "x-api-key" to "${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/v1/messages" until status 200 with body:
      """
      {"model": "${CTX:model}", "max_tokens": 16, "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "path" should be "${CTX:endpointContext}/v1/messages"
    And the JSON response field "json.model" should be "claude-haiku-4-5"
    And the response should contain echoed header "X-Api-Key" with value "anthropic-upstream-secret"

    When I send a "POST" request to "${CTX:endpointContext}/v1/messages" with body:
      """
      {"model": "${CTX:refusedModel}", "max_tokens": 16, "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 404
    And the JSON response field "type" should be "error"
    And the JSON response field "error.type" should be "not_found_error"

    # The catalog is the Anthropic model list, of the Models the endpoint serves.
    When I send a "GET" request to "${CTX:endpointContext}/v1/models"
    Then the response status code should be 200
    And the JSON response array field "data" should have 1 item
    And the JSON response field "data[0].type" should be "model"
    And the JSON response field "data[0].id" should be "${CTX:model}"
    And the JSON response field "has_more" should be false

    # An endpoint change that would stop it serving a Model it serves today is refused.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I update model endpoint "${CTX:endpoint}" from "resources/templates/model-endpoint.yaml" with values:
      | name                 | ${CTX:endpoint}                                                                          |
      | apiVersion           | ${CTX:gatewaySpecVersion}                                                                |
      | displayName          | ${CTX:endpoint}                                                                          |
      | version              | v1.0                                                                                     |
      | context              | ${CTX:endpointContext}                                                                   |
      | spec.inboundTemplate | gemini                                                                                   |
      | spec.policies        | [{"name": "api-key-auth", "version": "v1", "params": {"key": "x-api-key", "in": "header"}}] |
    Then the response status code should be 409
    And the response body should contain "${CTX:model}"

  Scenario: A Gemini endpoint reads the model from the path
    Given I generate a unique resource name from "gemini-native-provider" and store it as "provider"
    And I generate a unique resource name from "gemini-endpoint" and store it as "endpoint"
    And I generate a unique API context from "gemini-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "gemini-endpoint-key" and store it as "keyName"
    And I generate a unique resource name from "gemini-flash" and store it as "model"
    And I generate a unique resource name from "gemini-unknown" and store it as "unknownModel"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:provider}                                                                                                                                                                                                      |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                            |
      | displayName        | ${CTX:provider}                                                                                                                                                                                                      |
      | version            | v1.0                                                                                                                                                                                                                 |
      | template           | gemini                                                                                                                                                                                                               |
      | accessControl.mode | allow_all                                                                                                                                                                                                            |
      | spec               | {"displayName": "${CTX:provider}", "version": "v1.0", "template": "gemini", "exposure": "none", "upstream": {"url": "http://testbench:3002/gemini-upstream", "auth": {"type": "api-key", "header": "x-goog-api-key", "value": "gemini-upstream-secret"}}} |
    Then the response status code should be 201
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name                 | ${CTX:endpoint}                                                                               |
      | apiVersion           | ${CTX:gatewaySpecVersion}                                                                     |
      | displayName          | ${CTX:endpoint}                                                                               |
      | version              | v1.0                                                                                          |
      | context              | ${CTX:endpointContext}                                                                        |
      | spec.inboundTemplate | gemini                                                                                        |
      | spec.policies        | [{"name": "api-key-auth", "version": "v1", "params": {"key": "x-goog-api-key", "in": "header"}}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                               |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                  |
      | displayName | ${CTX:model}                                                                               |
      | version     | v1.0                                                                                       |
      | modelName   | ${CTX:model}                                                                               |
      | endpoints   | ["${CTX:endpoint}"]                                                                        |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "gemini-2.0-flash"}] |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:endpointContext}/v1beta/models/${CTX:model}:generateContent" until status 401 with body:
      """
      {"contents": [{"parts": [{"text": "Hello"}]}]}
      """
    Then the JSON response field "error.status" should be "UNAUTHENTICATED"

    # The model segment of the path is rewritten for the target.
    When I set header "x-goog-api-key" to "${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/v1beta/models/${CTX:model}:generateContent" until status 200 with body:
      """
      {"contents": [{"parts": [{"text": "Hello"}]}]}
      """
    Then the JSON response field "path" should be "/gemini-upstream/v1beta/models/gemini-2.0-flash:generateContent"
    And the response should contain echoed header "X-Goog-Api-Key" with value "gemini-upstream-secret"

    When I send a "POST" request to "${CTX:endpointContext}/v1/models/${CTX:model}:streamGenerateContent" with body:
      """
      {"contents": [{"parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/gemini-upstream/v1/models/gemini-2.0-flash:streamGenerateContent"

    When I send a "POST" request to "${CTX:endpointContext}/v1beta/models/${CTX:unknownModel}:generateContent" with body:
      """
      {"contents": [{"parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 404
    And the JSON response field "error.code" should be 404
    And the JSON response field "error.status" should be "NOT_FOUND"

    When I send a "GET" request to "${CTX:endpointContext}/v1beta/models"
    Then the response status code should be 200
    And the JSON response array field "models" should have 1 item
    And the JSON response field "models[0].name" should be "models/${CTX:model}"
