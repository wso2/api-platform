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

@model-weighted-round-robin
Feature: Model weighted round-robin load balancing policy
  As an API developer
  I want the model-weighted-round-robin policy to distribute AI model requests
  according to configurable weights in a repeating sequence
  So that traffic can be proportioned across models and unhealthy models are
  automatically suspended and recovered

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Basic weighted distribution with payload location
    Given I generate a unique value from "wrr-basic" and store it as "apiName"
    And I generate a unique API version from "wrr-basic" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-basic" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"gpt-3.5-turbo","weight":3},{"model":"gpt-4","weight":1}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Weight sequence: [gpt-3.5-turbo, gpt-3.5-turbo, gpt-3.5-turbo, gpt-4]
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-4"

    # Cycle wraps back to the start of the weighted sequence
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original-model","prompt":"Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Equal weight distribution
    Given I generate a unique value from "wrr-equal" and store it as "apiName"
    And I generate a unique API version from "wrr-equal" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-equal" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"model-a","weight":1},{"model":"model-b","weight":1}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original","data":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original","data":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"

    # Cycle wraps back to model-a
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"original","data":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Three models with different weights
    Given I generate a unique value from "wrr-three" and store it as "apiName"
    And I generate a unique API version from "wrr-three" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-three" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"fast-model","weight":5},{"model":"balanced-model","weight":3},{"model":"premium-model","weight":2}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Total weight = 10, so sequence is: [fast x5, balanced x3, premium x2]
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"

    # Weight boundary: 6th request transitions to balanced-model
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "balanced-model"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "balanced-model"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "balanced-model"

    # Weight boundary: 9th request transitions to premium-model
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "premium-model"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with header location
    Given I generate a unique value from "wrr-header" and store it as "apiName"
    And I generate a unique API version from "wrr-header" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"model-a","weight":1},{"model":"model-b","weight":1}],"requestModel":{"location":"header","identifier":"X-Model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "X-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"

    When I set header "X-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with query parameter location
    Given I generate a unique value from "wrr-query" and store it as "apiName"
    And I generate a unique API version from "wrr-query" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-query" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"model-alpha","weight":2},{"model":"model-beta","weight":1}],"requestModel":{"location":"queryParam","identifier":"model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Sequence: [model-alpha, model-alpha, model-beta]
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=model-alpha"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=model-alpha"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=model-beta"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Model selection with path parameter location
    Given I generate a unique value from "wrr-path" and store it as "apiName"
    And I generate a unique API version from "wrr-path" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/models/*","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"new-model-1","weight":1},{"model":"new-model-2","weight":1}],"requestModel":{"location":"pathParam","identifier":"/models/([^/]+)/chat"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/models/old-model/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/new-model-1/chat"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/models/old-model/chat" with body:
      """
      {"prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/new-model-2/chat"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Suspend model on 5xx error with recovery
    Given I generate a unique value from "wrr-susp-5xx" and store it as "apiName"
    And I generate a unique API version from "wrr-susp-5xx" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-susp-5xx" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"failing-model","weight":1},{"model":"working-model","weight":1}],"suspendDuration":30,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # First request goes to failing-model, returns 500 from backend
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Next request should skip suspended failing-model and use working-model
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "working-model"

    # Another request should still use working-model (failing-model is suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "working-model"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Suspend model on 429 rate limit error
    Given I generate a unique value from "wrr-susp-429" and store it as "apiName"
    And I generate a unique API version from "wrr-susp-429" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-susp-429" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"rate-limited-model","weight":1},{"model":"available-model","weight":1}],"suspendDuration":30,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # First request returns 429 from backend
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=429" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 429

    # Next request should use available-model (rate-limited-model is suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "available-model"

    # Another request should still use available-model (rate-limited-model remains suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "available-model"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: All models suspended returns 503
    Given I generate a unique value from "wrr-all-susp" and store it as "apiName"
    And I generate a unique API version from "wrr-all-susp" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-all-susp" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"model-1","weight":1},{"model":"model-2","weight":1}],"suspendDuration":5,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Trigger 500 error for model-1
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Trigger 500 error for model-2
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Now all models are suspended, should return 503
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 503
    And the response body should contain "All models are currently unavailable"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: No suspension when suspendDuration is 0
    Given I generate a unique value from "wrr-no-susp" and store it as "apiName"
    And I generate a unique API version from "wrr-no-susp" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-no-susp" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"model-a","weight":1},{"model":"model-b","weight":1}],"suspendDuration":0,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # First request fails with 500
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Next request still rotates to model-b (no suspension)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"

    # Next request rotates back to model-a (not suspended)
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle empty request body
    Given I generate a unique value from "wrr-empty" and store it as "apiName"
    And I generate a unique API version from "wrr-empty" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-empty" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"selected-model","weight":1}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
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
    Given I generate a unique value from "wrr-inv-json" and store it as "apiName"
    And I generate a unique API version from "wrr-inv-json" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-inv-json" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"selected-model","weight":1}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      invalid json {
      """
    Then the response status code should be 400
    And the response body should contain "Invalid JSON in request body"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    Given I generate a unique value from "wrr-inv-path" and store it as "apiName"
    And I generate a unique API version from "wrr-inv-path" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-inv-path" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"selected-model","weight":1}],"requestModel":{"location":"payload","identifier":"$.nonexistent.field"}}}]},{"method":"GET","path":"/health"}] |
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
    Given I generate a unique value from "wrr-missing" and store it as "apiName"
    And I generate a unique API version from "wrr-missing" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-missing" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"selected-model","weight":1}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
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

  Scenario: Fallback to secondary models on primary failure
    Given I generate a unique value from "wrr-fallback" and store it as "apiName"
    And I generate a unique API version from "wrr-fallback" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-fallback" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"primary-model","weight":4},{"model":"secondary-model-1","weight":3},{"model":"secondary-model-2","weight":3}],"suspendDuration":10,"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Primary model fails
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat?statusCode=500" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 500

    # Subsequent requests use secondary models
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any"}
      """
    Then the response status code should be 200
    And the response body should match pattern "(secondary-model-1|secondary-model-2)"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Canary deployment with small weight for new model
    Given I generate a unique value from "wrr-canary" and store it as "apiName"
    And I generate a unique API version from "wrr-canary" and store it as "apiVersion"
    And I generate a unique API context from "/wrr-canary" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"model-weighted-round-robin","version":"v1","params":{"models":[{"model":"stable-model-v1","weight":9},{"model":"canary-model-v2","weight":1}],"requestModel":{"location":"payload","identifier":"$.model"}}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # Requests 1-9 go to stable-model-v1, request 10 goes to canary-model-v2
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"

    # Weight boundary: 10th request transitions to canary-model-v2
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"model":"any","prompt":"test"}
      """
    Then the response status code should be 200
    And the response body should contain "canary-model-v2"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
