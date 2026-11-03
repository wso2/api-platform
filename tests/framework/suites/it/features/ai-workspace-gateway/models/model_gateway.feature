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

@model-gateway
Feature: Model gateway
  As an API administrator
  I want to serve Models on OpenAI-compatible model endpoints
  So that a client names a model and the gateway routes it to one of the Model's targets

  The gateway provisions the default model endpoint, "wso2", at /wso2/v1. A model endpoint
  dispatches each chat completions request on its "model" field to that Model's chain, which
  routes it to a target: directly, for a provider with exposure "none", or through the
  provider's own route. Providers forward to the testbench echo service, which reflects the
  request, so the reflected path shows which provider served it and the reflected body shows
  the model the provider was asked for.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: The default model endpoint is provisioned and protected
    When I send a "GET" request to the "gateway-controller" service at "/model-endpoints/wso2"
    Then the response status code should be 200
    And the JSON response field "kind" should be "ModelEndpoint"
    And the JSON response field "spec.context" should be "/wso2/v1"
    And the JSON response field "spec.policies[0].name" should be "api-key-auth"
    And the JSON response field "status.state" should be "deployed"
    And the response body should contain "openai-to-anthropic-transformer"
    And the response body should contain "openai-to-bedrock-transformer"

    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/wso2"
    Then the response status code should be 404

    When I delete the model endpoint "wso2"
    Then the response status code should be 409

    When I update model endpoint "wso2" from "resources/templates/model-endpoint.yaml" with values:
      | name        | wso2                      |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | WSO2 Model Endpoint       |
      | version     | v1.0                      |
      | context     | /elsewhere                |
    Then the response status code should be 409

  Scenario: Requests are authenticated before anything else
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/wso2/v1/chat/completions" with body:
      """
      {"model": "anything", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 401
    And the JSON response field "error.type" should be "authentication_error"
    And the JSON response field "error.message" should be "Valid API key required"

    When I send a "GET" request to "/wso2/v1/models"
    Then the response status code should be 401
    And the JSON response field "error.type" should be "authentication_error"

  Scenario: Only an admin may change a model endpoint or write Models
    Given I generate a unique resource name from "developer-endpoint" and store it as "endpoint"
    And I generate a unique API context from "developer-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "developer-model" and store it as "model"
    And I authenticate using basic auth as "developer"

    When I send a "GET" request to the "gateway-controller" service at "/model-endpoints/wso2"
    Then the response status code should be 200
    When I send a "GET" request to the "gateway-controller" service at "/model-endpoints"
    Then the response status code should be 200
    When I send a "GET" request to the "gateway-controller" service at "/models"
    Then the response status code should be 200

    When I update model endpoint "wso2" from "resources/templates/model-endpoint.yaml" with values:
      | name        | wso2                      |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | Renamed by a developer    |
      | version     | v1.0                      |
      | context     | /wso2/v1                  |
    Then the response status code should be 403
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
    Then the response status code should be 403
    When I delete the model endpoint "wso2"
    Then the response status code should be 403
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                     |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                        |
      | displayName | ${CTX:model}                                                                     |
      | version     | v1.0                                                                             |
      | modelName   | ${CTX:model}                                                                     |
      | endpoints   | ["wso2"]                                                                         |
      | targets     | [{"name": "a", "provider": {"id": "any-provider"}, "upstreamModel": "gpt-4o"}] |
    Then the response status code should be 403

    Given I authenticate using basic auth as "admin"
    When I send a "GET" request to the "gateway-controller" service at "/model-endpoints/wso2"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "WSO2 Model Endpoint"

  Scenario: A Model must name endpoints that exist
    Given I generate a unique resource name from "no-endpoints-model" and store it as "model"
    And I generate a unique resource name from "missing-endpoint" and store it as "missingEndpoint"
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                                                                                                       |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                                                                                          |
      | displayName | ${CTX:model}                                                                                                                                                       |
      | version     | v1.0                                                                                                                                                               |
      | modelName   | ${CTX:model}                                                                                                                                                       |
      | endpoints   | ["wso2"]                                                                                                                                                           |
      | targets     | []                                                                                                                                                                 |
      | spec        | {"displayName": "${CTX:model}", "version": "v1.0", "modelName": "${CTX:model}", "targets": [{"name": "a", "provider": {"id": "any-provider"}, "upstreamModel": "gpt-4o"}]} |
    Then the response status code should be 400
    And the response body should contain "spec.endpoints"

    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                     |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                        |
      | displayName | ${CTX:model}                                                                     |
      | version     | v1.0                                                                             |
      | modelName   | ${CTX:model}                                                                     |
      | endpoints   | ["wso2", "${CTX:missingEndpoint}"]                                               |
      | targets     | [{"name": "a", "provider": {"id": "any-provider"}, "upstreamModel": "gpt-4o"}] |
    Then the response status code should be 400
    And the response body should contain "spec.endpoints[1]: model endpoint '${CTX:missingEndpoint}' not found"
    When I send a "GET" request to the "gateway-controller" service at "/models/${CTX:model}"
    Then the response status code should be 404

  Scenario: A Model on a provider with exposure none calls the provider's upstream directly
    Given I generate a unique resource name from "direct-provider" and store it as "provider"
    And I generate a unique resource name from "direct-model" and store it as "model"
    And I generate a unique resource name from "direct-endpoint" and store it as "endpoint"
    And I generate a unique API context from "direct-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "direct-endpoint-key" and store it as "keyName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:provider}                                                                                                                                                                                                        |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                              |
      | displayName        | ${CTX:provider}                                                                                                                                                                                                        |
      | version            | v1.0                                                                                                                                                                                                                   |
      | template           | openai                                                                                                                                                                                                                 |
      | accessControl.mode | allow_all                                                                                                                                                                                                              |
      | spec               | {"displayName": "${CTX:provider}", "version": "v1.0", "template": "openai", "exposure": "none", "upstream": {"url": "http://testbench:3002/direct-upstream", "auth": {"type": "api-key", "header": "X-Direct-Upstream-Key", "value": "direct-upstream-secret"}}} |
    Then the response status code should be 201

    # A provider with exposure none serves no route, so it issues no API keys.
    When I send a "POST" request to the "gateway-controller" service at "/llm-providers/${CTX:provider}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 409

    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                                       |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                          |
      | displayName | ${CTX:model}                                                                                       |
      | version     | v1.0                                                                                               |
      | modelName   | ${CTX:model}                                                                                       |
      | endpoints   | ["${CTX:endpoint}"]                                                                                |
      | targets     | [{"name": "direct", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "direct-upstream-model"}] |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    # One hop to the provider's upstream, with the provider's own credential, asking for the
    # target's upstream model.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:model}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "path" should be "/direct-upstream/chat/completions"
    And the JSON response field "json.model" should be "direct-upstream-model"
    And the response should contain echoed header "X-Direct-Upstream-Key" with value "direct-upstream-secret"
    And the response should not contain echoed header "X-Wso2-Internal-Loopback"

    # The catalog lists the Models the endpoint serves.
    When I send a "GET" request to "${CTX:endpointContext}/models"
    Then the response status code should be 200
    And the JSON response field "object" should be "list"
    And the JSON response array field "data" should have 1 item
    And the JSON response array "data" should contain an item with "id" equal to "${CTX:model}"

    # A provider a Model targets cannot be deleted.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the LLM provider "${CTX:provider}"
    Then the response status code should be 409
    And the response body should contain "${CTX:model}"

  Scenario: A Model on a provider with its own route calls it through that route
    Given I generate a unique resource name from "routed-provider" and store it as "provider"
    And I generate a unique API context from "routed-provider" and store it as "providerContext"
    And I generate a unique resource name from "routed-provider-key" and store it as "providerKeyName"
    And I generate a unique resource name from "routed-model" and store it as "model"
    And I generate a unique resource name from "routed-endpoint" and store it as "endpoint"
    And I generate a unique API context from "routed-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "routed-endpoint-key" and store it as "keyName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name                | ${CTX:provider}                                                                       |
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                             |
      | displayName         | ${CTX:provider}                                                                       |
      | version             | v1.0                                                                                  |
      | template            | openai                                                                                |
      | accessControl.mode  | allow_all                                                                             |
      | spec.context        | ${CTX:providerContext}                                                                |
      | spec.upstream       | {"url": "http://testbench:3002/routed-upstream"}                                      |
      | spec.globalPolicies | [{"name": "api-key-auth", "version": "v1", "params": {"key": "X-Routed-Key", "in": "header"}}] |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/llm-providers/${CTX:provider}/api-keys" with body:
      """
      {"name": "${CTX:providerKeyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "providerKey"

    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                                                                                                                            |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                                                                                                               |
      | displayName | ${CTX:model}                                                                                                                                                                            |
      | version     | v1.0                                                                                                                                                                                    |
      | modelName   | ${CTX:model}                                                                                                                                                                            |
      | endpoints   | ["${CTX:endpoint}"]                                                                                                                                                                     |
      | targets     | [{"name": "routed", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "routed-upstream-model", "auth": {"type": "api-key", "header": "X-Routed-Key", "value": "${CTX:providerKey}"}}] |
    Then the response status code should be 201
    # The target credential is write-only.
    And the response body should not contain "${CTX:providerKey}"
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    # Through the provider's own route, which authenticates the target's key and forwards to
    # its upstream.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:model}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "path" should be "/routed-upstream/chat/completions"
    And the JSON response field "json.model" should be "routed-upstream-model"

    # Reported once, by the endpoint, naming the Model and the target that served it. The
    # provider route's own event for the loopback hop is dropped. The policy engine, not Envoy,
    # rewrites a model endpoint's path, so its events are selected by their API context.
    When I wait for an analytics event for API context "${CTX:endpointContext}" with response status 200
    Then the latest analytics event for API context "${CTX:endpointContext}" should have metadata field "apiType" with value "LlmProxy"
    And the latest analytics event for API context "${CTX:endpointContext}" should have metadata field "modelAnalytics.handle" with value "${CTX:model}"
    And the latest analytics event for API context "${CTX:endpointContext}" should have metadata field "modelAnalytics.name" with value "${CTX:model}"
    And the latest analytics event for API context "${CTX:endpointContext}" should have metadata field "modelAnalytics.target" with value "routed"
    And the latest analytics event for API context "${CTX:endpointContext}" should have metadata field "modelAnalytics.upstreamModel" with value "routed-upstream-model"
    And the latest analytics event for API context "${CTX:endpointContext}" should have metadata field "modelAnalytics.providerHandle" with value "${CTX:provider}"
    And the analytics collector should have no event for API context "${CTX:providerContext}"

  Scenario: A request naming no deployed model is answered in the OpenAI shape
    Given I generate a unique resource name from "errors-endpoint" and store it as "endpoint"
    And I generate a unique API context from "errors-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "errors-endpoint-key" and store it as "keyName"
    And I generate a unique resource name from "no-such-model" and store it as "unknownModel"
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    # A request with no model is answered by the endpoint itself, so it shows the endpoint is
    # routed; a request before then gets the router's own 404.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 400 with body:
      """
      {"messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "error.code" should be "missing_model"
    And the JSON response field "error.param" should be "model"

    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:unknownModel}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 404
    And the JSON response field "error.code" should be "model_not_found"
    And the JSON response field "error.type" should be "invalid_request_error"
    And the response body should not contain "${CTX:unknownModel}"

    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      not json
      """
    Then the response status code should be 400
    And the JSON response field "error.code" should be "invalid_json"

    # A model named twice is refused rather than guessed at: the gateway and the provider could
    # otherwise read different models.
    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "first", "model": "second", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 400
    And the JSON response field "error.code" should be "invalid_model"

  Scenario: A routing policy chooses between a Model's targets
    Given I generate a unique resource name from "rr-provider-a" and store it as "providerA"
    And I generate a unique resource name from "rr-provider-b" and store it as "providerB"
    And I generate a unique resource name from "rr-model" and store it as "model"
    And I generate a unique resource name from "unrouted-model" and store it as "unroutedModel"
    And I generate a unique resource name from "rr-endpoint" and store it as "endpoint"
    And I generate a unique API context from "rr-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "rr-endpoint-key" and store it as "keyName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:providerA}                                                                                                                         |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                |
      | displayName        | ${CTX:providerA}                                                                                                                         |
      | version            | v1.0                                                                                                                                     |
      | template           | openai                                                                                                                                   |
      | accessControl.mode | allow_all                                                                                                                                |
      | spec               | {"displayName": "${CTX:providerA}", "version": "v1.0", "template": "openai", "exposure": "none", "upstream": {"url": "http://testbench:3002/rr-upstream-a"}} |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:providerB}                                                                                                                         |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                |
      | displayName        | ${CTX:providerB}                                                                                                                         |
      | version            | v1.0                                                                                                                                     |
      | template           | openai                                                                                                                                   |
      | accessControl.mode | allow_all                                                                                                                                |
      | spec               | {"displayName": "${CTX:providerB}", "version": "v1.0", "template": "openai", "exposure": "none", "upstream": {"url": "http://testbench:3002/rr-upstream-b"}} |
    Then the response status code should be 201
    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name         | ${CTX:model}                                                                                                                                          |
      | apiVersion   | ${CTX:gatewaySpecVersion}                                                                                                                             |
      | displayName  | ${CTX:model}                                                                                                                                          |
      | version      | v1.0                                                                                                                                                  |
      | modelName    | ${CTX:model}                                                                                                                                          |
      | endpoints    | ["${CTX:endpoint}"]                                                                                                                                   |
      | targets      | [{"name": "a", "provider": {"id": "${CTX:providerA}"}, "upstreamModel": "model-a"}, {"name": "b", "provider": {"id": "${CTX:providerB}"}, "upstreamModel": "model-b"}] |
      | spec.routing | {"policy": {"name": "model-round-robin", "version": "v1"}, "params": {"models": [{"model": "a"}, {"model": "b"}], "suspendDuration": 0}}                    |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    # The router names a target by its name; each request goes to the next one.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:model}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the JSON response field "path" should be "/rr-upstream-a/chat/completions"
    And the JSON response field "json.model" should be "model-a"
    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" with body:
      """
      {"model": "${CTX:model}", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/rr-upstream-b/chat/completions"
    And the JSON response field "json.model" should be "model-b"

    # A Model with two targets and no router is refused.
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:unroutedModel}                                                                                                                                  |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                                                                             |
      | displayName | ${CTX:unroutedModel}                                                                                                                                  |
      | version     | v1.0                                                                                                                                                  |
      | modelName   | ${CTX:unroutedModel}                                                                                                                                  |
      | endpoints   | ["${CTX:endpoint}"]                                                                                                                                   |
      | targets     | [{"name": "a", "provider": {"id": "${CTX:providerA}"}, "upstreamModel": "model-a"}, {"name": "b", "provider": {"id": "${CTX:providerB}"}, "upstreamModel": "model-b"}] |
    Then the response status code should be 400
    And the response body should contain "routing"

  # The testbench Bedrock service checks each Converse call's SigV4 signature against the request
  # it received and answers 403 SignatureDoesNotMatch otherwise, so a 200 proves the gateway
  # signed the host, path and body it actually sent: for a direct target, the provider the route
  # job chose, not the endpoint's own default upstream.
  Scenario: Models on Bedrock are translated to Converse and signed for the provider
    Given I generate a unique resource name from "bedrock-direct" and store it as "directProvider"
    And I generate a unique resource name from "bedrock-routed" and store it as "routedProvider"
    And I generate a unique API context from "bedrock-routed" and store it as "routedProviderContext"
    And I generate a unique resource name from "bedrock-routed-key" and store it as "providerKeyName"
    And I generate a unique resource name from "bedrock-direct-model" and store it as "directModel"
    And I generate a unique resource name from "bedrock-routed-model" and store it as "routedModel"
    And I generate a unique resource name from "bedrock-endpoint" and store it as "endpoint"
    And I generate a unique API context from "bedrock-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "bedrock-endpoint-key" and store it as "keyName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:directProvider}                                                                                                                                                                                                                                                                                                                                                   |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                               |
      | displayName        | ${CTX:directProvider}                                                                                                                                                                                                                                                                                                                                                   |
      | version            | v1.0                                                                                                                                                                                                                                                                                                                                                                    |
      | template           | awsbedrock                                                                                                                                                                                                                                                                                                                                                              |
      | accessControl.mode | allow_all                                                                                                                                                                                                                                                                                                                                                               |
      | spec               | {"displayName": "${CTX:directProvider}", "version": "v1.0", "template": "awsbedrock", "exposure": "none", "upstream": {"url": "http://testbench:3006", "auth": {"type": "other", "policyName": "aws-authentication", "policyVersion": "v0", "policyParams": {"service": "bedrock", "region": "us-east-1", "authenticationType": "iam-user-access-key", "awsAccessKeyID": "AKIAITBEDROCKMOCK000", "awsSecretAccessKey": "itBedrockMockSecretKey00000000000000000000"}}}} |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name                | ${CTX:routedProvider}                                                                                                                                                                                                                                       |
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                   |
      | displayName         | ${CTX:routedProvider}                                                                                                                                                                                                                                       |
      | version             | v1.0                                                                                                                                                                                                                                                        |
      | template            | awsbedrock                                                                                                                                                                                                                                                  |
      | accessControl.mode  | allow_all                                                                                                                                                                                                                                                   |
      | spec.context        | ${CTX:routedProviderContext}                                                                                                                                                                                                                                |
      | spec.upstream       | {"url": "http://testbench:3006", "auth": {"type": "other", "policyName": "aws-authentication", "policyVersion": "v0", "policyParams": {"service": "bedrock", "region": "us-east-1", "authenticationType": "iam-user-access-key", "awsAccessKeyID": "AKIAITBEDROCKMOCK000", "awsSecretAccessKey": "itBedrockMockSecretKey00000000000000000000"}}} |
      | spec.globalPolicies | [{"name": "api-key-auth", "version": "v1", "params": {"key": "X-API-Key", "in": "header"}}]                                                                                                                                                                  |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/llm-providers/${CTX:routedProvider}/api-keys" with body:
      """
      {"name": "${CTX:providerKeyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "providerKey"

    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
      | spec.transformers | [{"template": "awsbedrock", "type": "openai-to-bedrock-transformer", "version": "v0", "upstreamPaths": ["/model/{upstreamModel}/converse", "/model/{upstreamModel}/converse-stream"]}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:directModel}                                                                                                     |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                                              |
      | displayName | ${CTX:directModel}                                                                                                     |
      | version     | v1.0                                                                                                                   |
      | modelName   | ${CTX:directModel}                                                                                                     |
      | endpoints   | ["${CTX:endpoint}"]                                                                                                    |
      | targets     | [{"name": "bedrock", "provider": {"id": "${CTX:directProvider}"}, "upstreamModel": "anthropic.claude-3-haiku-20240307-v1:0"}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:routedModel}                                                                                                                                                                        |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                                                                                                                 |
      | displayName | ${CTX:routedModel}                                                                                                                                                                        |
      | version     | v1.0                                                                                                                                                                                      |
      | modelName   | ${CTX:routedModel}                                                                                                                                                                        |
      | endpoints   | ["${CTX:endpoint}"]                                                                                                                                                                       |
      | targets     | [{"name": "bedrock", "provider": {"id": "${CTX:routedProvider}"}, "upstreamModel": "amazon.nova-lite-v1:0", "auth": {"type": "api-key", "header": "X-API-Key", "value": "${CTX:providerKey}"}}] |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:directModel}", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 50}
      """
    Then the JSON response field "object" should be "chat.completion"
    And the response body should contain "Hello from mock Bedrock (anthropic.claude-3-haiku-20240307-v1:0): Hello"

    When I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:routedModel}", "messages": [{"role": "user", "content": "Hi"}]}
      """
    Then the response body should contain "Hello from mock Bedrock (amazon.nova-lite-v1:0): Hi"

  # A Bedrock API key is a bearer token, so the provider sets it with api-key upstream auth instead
  # of signing. The testbench accepts a key only as the request's single Authorization header, so a
  # 200 proves the gateway replaced its caller's own `Authorization: Bearer <endpoint key>` with the
  # provider's rather than sending both.
  Scenario: Models on Bedrock can authenticate with a Bedrock API key
    Given I generate a unique resource name from "bedrock-api-key" and store it as "secretName"
    And I generate a unique resource name from "bedrock-apikey-provider" and store it as "provider"
    And I generate a unique resource name from "bedrock-apikey-model" and store it as "model"
    And I generate a unique resource name from "bedrock-apikey-endpoint" and store it as "endpoint"
    And I generate a unique API context from "bedrock-apikey-endpoint" and store it as "endpointContext"
    And I generate a unique resource name from "bedrock-apikey-endpoint-key" and store it as "keyName"
    When I send a "POST" request to the "gateway-controller" service at "/secrets" with body:
      """
      {
        "apiVersion": "${CTX:gatewaySpecVersion}",
        "kind": "Secret",
        "metadata": {"name": "${CTX:secretName}"},
        "spec": {"displayName": "${CTX:secretName}", "value": "itBedrockMockApiKey000000000000000000000"}
      }
      """
    Then the response status should be 201
    And I register the "secret" "${CTX:secretName}" for cleanup

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | name               | ${CTX:provider}                                                                                                                                                                                                                                                          |
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                |
      | displayName        | ${CTX:provider}                                                                                                                                                                                                                                                          |
      | version            | v1.0                                                                                                                                                                                                                                                                     |
      | template           | awsbedrock                                                                                                                                                                                                                                                               |
      | accessControl.mode | allow_all                                                                                                                                                                                                                                                                |
      | spec               | {"displayName": "${CTX:provider}", "version": "v1.0", "template": "awsbedrock", "exposure": "none", "upstream": {"url": "http://testbench:3006", "auth": {"type": "api-key", "policyParams": {"request": {"headers": [{"name": "Authorization", "value": "Bearer {{ secret \"${CTX:secretName}\" }}"}]}}}}} |
    Then the response status code should be 201
    And the response body should not contain "itBedrockMockApiKey"

    When I create model endpoint from "resources/templates/model-endpoint.yaml" with values:
      | name        | ${CTX:endpoint}           |
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | displayName | ${CTX:endpoint}           |
      | version     | v1.0                      |
      | context     | ${CTX:endpointContext}    |
      | spec.transformers | [{"template": "awsbedrock", "type": "openai-to-bedrock-transformer", "version": "v0", "upstreamPaths": ["/model/{upstreamModel}/converse", "/model/{upstreamModel}/converse-stream"]}] |
    Then the response status code should be 201
    When I create Model from "resources/templates/model.yaml" with values:
      | name        | ${CTX:model}                                                                                       |
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                          |
      | displayName | ${CTX:model}                                                                                       |
      | version     | v1.0                                                                                               |
      | modelName   | ${CTX:model}                                                                                       |
      | endpoints   | ["${CTX:endpoint}"]                                                                                |
      | targets     | [{"name": "bedrock", "provider": {"id": "${CTX:provider}"}, "upstreamModel": "amazon.nova-lite-v1:0"}] |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/model-endpoints/${CTX:endpoint}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "endpointKey"

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer ${CTX:endpointKey}"
    And I send a "POST" request to "${CTX:endpointContext}/chat/completions" until status 200 with body:
      """
      {"model": "${CTX:model}", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 50}
      """
    Then the JSON response field "object" should be "chat.completion"
    And the response body should contain "Hello from mock Bedrock (amazon.nova-lite-v1:0): Hello"
