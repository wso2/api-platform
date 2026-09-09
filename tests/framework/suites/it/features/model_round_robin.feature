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

@model-round-robin
Feature: Model round-robin load balancing policy
  As an API developer
  I want the model-round-robin policy to distribute AI model requests evenly across
  multiple configured models in a cyclic pattern
  So that load is balanced and unhealthy models are automatically suspended and recovered

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Basic round-robin with two models
    Given I generate a unique value from "mrr-two" and store it as "apiName"
    And I generate a unique API version from "mrr-two" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-two" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"model-a"},{"model":"model-b"}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Round-robin with three models
    Given I generate a unique value from "mrr-three" and store it as "apiName"
    And I generate a unique API version from "mrr-three" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-three" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"model-alpha"},{"model":"model-beta"},{"model":"model-gamma"}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-alpha"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-beta"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-gamma"

    # Cycle back to the first model
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-alpha"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Round-robin with four models
    Given I generate a unique value from "mrr-four" and store it as "apiName"
    And I generate a unique API version from "mrr-four" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-four" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"gpt-3.5-turbo"},{"model":"gpt-4"},{"model":"claude-3-sonnet"},{"model":"gemini-pro"}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-4"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "claude-3-sonnet"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "gemini-pro"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with header location
    Given I generate a unique value from "mrr-header" and store it as "apiName"
    And I generate a unique API version from "mrr-header" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"header-model-1"},{"model":"header-model-2"}],"requestModel":{"location":"header","identifier":"X-AI-Model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-AI-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "header-model-1"

    When I set header "X-AI-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "header-model-2"

    # Cycle wraps back to header-model-1
    When I set header "X-AI-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "header-model-1"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with query parameter location
    Given I generate a unique value from "mrr-query" and store it as "apiName"
    And I generate a unique API version from "mrr-query" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-query" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"query-model-1"},{"model":"query-model-2"},{"model":"query-model-3"}],"requestModel":{"location":"queryParam","identifier":"model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-1"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-2"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-3"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-1"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with path parameter location
    Given I generate a unique value from "mrr-path" and store it as "apiName"
    And I generate a unique API version from "mrr-path" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/models/*","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"path-model-x"},{"model":"path-model-y"}],"requestModel":{"location":"pathParam","identifier":"/models/([^/]+)/chat"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/models/original-model/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/path-model-x/chat"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/models/original-model/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/path-model-y/chat"

    # Cycle wraps back to path-model-x
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/models/original-model/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/path-model-x/chat"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with nested JSONPath
    Given I generate a unique value from "mrr-nested" and store it as "apiName"
    And I generate a unique API version from "mrr-nested" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-nested" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"nested-model-1"},{"model":"nested-model-2"}],"requestModel":{"location":"payload","identifier":"$.settings.ai.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"settings":{"ai":{"model":"original"}},"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "nested-model-1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"settings":{"ai":{"model":"original"}},"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "nested-model-2"

    # Cycle wraps back to nested-model-1
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"settings":{"ai":{"model":"original"}},"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "nested-model-1"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Suspend model on 5xx error with recovery
    Given I generate a unique value from "mrr-suspend-5xx" and store it as "apiName"
    And I generate a unique API version from "mrr-suspend-5xx" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-suspend-5xx" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"first-model"},{"model":"second-model"},{"model":"third-model"}],"suspendDuration":3,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # First request goes to first-model, forced to fail with 500
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Next request skips the suspended first-model and uses second-model
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "second-model"

    # Next request uses third-model
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "third-model"

    # Next request uses second-model again (first-model still suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "second-model"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Suspend model on 429 rate limit error
    Given I generate a unique value from "mrr-suspend-429" and store it as "apiName"
    And I generate a unique API version from "mrr-suspend-429" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-suspend-429" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"model-1"},{"model":"model-2"}],"suspendDuration":3,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=429" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 429

    # Next request uses model-2 (model-1 is suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-2"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-2"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: All models suspended returns 503
    Given I generate a unique value from "mrr-all-suspended" and store it as "apiName"
    And I generate a unique API version from "mrr-all-suspended" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-all-suspended" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"model-x"},{"model":"model-y"}],"suspendDuration":5,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Both models are now suspended
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 503
    And the response body should contain "All models are currently unavailable"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: No suspension when suspendDuration is 0
    Given I generate a unique value from "mrr-no-suspend" and store it as "apiName"
    And I generate a unique API version from "mrr-no-suspend" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-no-suspend" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"model-one"},{"model":"model-two"}],"suspendDuration":0,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Rotation continues to model-two with no suspension in effect
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-two"

    # Rotation continues back to model-one (never suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-one"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle empty request body
    Given I generate a unique value from "mrr-empty-body" and store it as "apiName"
    And I generate a unique API version from "mrr-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"selected-model"}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      """
    Then the response status code should be 400
    And the response body should contain "Request body is empty"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle invalid JSON in request body
    Given I generate a unique value from "mrr-invalid-json" and store it as "apiName"
    And I generate a unique API version from "mrr-invalid-json" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-invalid-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"selected-model"}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      not valid json [
      """
    Then the response status code should be 400
    And the response body should contain "Invalid JSON in request body"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    Given I generate a unique value from "mrr-invalid-jsonpath" and store it as "apiName"
    And I generate a unique API version from "mrr-invalid-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-invalid-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"selected-model"}],"requestModel":{"location":"payload","identifier":"$.does.not.exist"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"test"}
      """
    Then the response status code should be 400
    And the response body should contain "Invalid or missing model"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle missing model field in payload
    Given I generate a unique value from "mrr-missing-model" and store it as "apiName"
    And I generate a unique API version from "mrr-missing-model" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-missing-model" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"selected-model"}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"test without model field"}
      """
    Then the response status code should be 200
    And the response body should contain "selected-model"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: High availability with automatic failover
    Given I generate a unique value from "mrr-ha-failover" and store it as "apiName"
    And I generate a unique API version from "mrr-ha-failover" and store it as "apiVersion"
    And I generate a unique API context from "/mrr-ha-failover" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-round-robin","version":"v1","params":{"models":[{"model":"primary-instance"},{"model":"backup-instance-1"},{"model":"backup-instance-2"}],"suspendDuration":10,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Primary instance fails
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Automatic failover to a backup instance
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should match pattern "(backup-instance-1|backup-instance-2)"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
