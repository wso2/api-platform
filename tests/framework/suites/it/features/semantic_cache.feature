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

@semantic-cache
Feature: Semantic cache policy
  As an API developer
  I want to cache LLM responses based on semantic similarity
  So that I can reduce latency and costs for similar requests
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: First request is a cache miss
    Given I generate a unique value from "sc-miss" and store it as "apiName"
    And I generate a unique API version from "sc-miss" and store it as "apiVersion"
    And I generate a unique API context from "/sc-miss" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"unique first-contact prompt about lighthouse keepers"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should not exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An identical repeated request is a cache hit
    Given I generate a unique value from "sc-hit" and store it as "apiName"
    And I generate a unique API version from "sc-hit" and store it as "apiVersion"
    And I generate a unique API context from "/sc-hit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"What is the capital of Germany?"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until header "X-Cache-Status" is "HIT" with body:
      """
      {"prompt":"What is the capital of Germany?"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # The mock embedding lowercases and word-splits before hashing, so a case-only variant
  # produces a cosine similarity of ~1.0 against the original - a genuine test of semantic
  # (not exact-string) matching, confirmed via the embeddings service's own /debug/similarity
  # endpoint before writing this scenario.
  Scenario: A semantically similar request (case variant) returns the cached response
    Given I generate a unique value from "sc-similar" and store it as "apiName"
    And I generate a unique API version from "sc-similar" and store it as "apiVersion"
    And I generate a unique API context from "/sc-similar" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"capital of italy"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until header "X-Cache-Status" is "HIT" with body:
      """
      {"prompt":"Capital of Italy"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Measured similarity between these two prompts is ~0.83 (via /debug/similarity) - below a
  # 0.99 threshold, so this is a genuine near-exact-match requirement, not a coincidence.
  Scenario: A high similarity threshold rejects a related but distinct request
    Given I generate a unique value from "sc-high-threshold" and store it as "apiName"
    And I generate a unique API version from "sc-high-threshold" and store it as "apiVersion"
    And I generate a unique API context from "/sc-high-threshold" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.99,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"What is the capital of France?"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"What is the capital of Germany?"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should not exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Same ~0.83-similarity prompt pair as the high-threshold scenario above, but with a low
  # threshold that admits it - demonstrating the threshold, not the prompt pair, is what changed.
  Scenario: A low similarity threshold matches a broader range of requests
    Given I generate a unique value from "sc-low-threshold" and store it as "apiName"
    And I generate a unique API version from "sc-low-threshold" and store it as "apiVersion"
    And I generate a unique API context from "/sc-low-threshold" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.7,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"What is the capital of France?"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until header "X-Cache-Status" is "HIT" with body:
      """
      {"prompt":"What is the capital of Germany?"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction embeds only the targeted field
    Given I generate a unique value from "sc-jsonpath" and store it as "apiName"
    And I generate a unique API version from "sc-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/sc-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"jsonPath":"$.messages[0].content","cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello AI assistant"}],"metadata":"request-1"}
      """

    # Same content field, different metadata - should still hit since only the content field is embedded
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until header "X-Cache-Status" is "HIT" with body:
      """
      {"messages":[{"role":"user","content":"Hello AI assistant"}],"metadata":"request-2-different"}
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A JSONPath that does not resolve is rejected as a policy error
    Given I generate a unique value from "sc-invalid-jsonpath" and store it as "apiName"
    And I generate a unique API version from "sc-invalid-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/sc-invalid-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"jsonPath":"$.nonexistent.field"}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"message":"This field exists but not the expected path"}
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the response body should contain "SEMANTIC_CACHE"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An empty request body is handled gracefully
    Given I generate a unique value from "sc-empty-body" and store it as "apiName"
    And I generate a unique API version from "sc-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/sc-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      """

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A non-200 upstream response is not cached
    Given I generate a unique value from "sc-non-200" and store it as "apiName"
    And I generate a unique API version from "sc-non-200" and store it as "apiVersion"
    And I generate a unique API context from "/sc-non-200" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/status/500","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/status/500" with body:
      """
      {"prompt":"unique-non-200-test-prompt"}
      """
    Then the response status code should be 500

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/status/500" with body:
      """
      {"prompt":"unique-non-200-test-prompt"}
      """
    Then the response status code should be 500
    And the response header "X-Cache-Status" should not exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An embedding provider failure allows the request to proceed uncached
    Given I generate a unique value from "sc-embed-error" and store it as "apiName"
    And I generate a unique API version from "sc-embed-error" and store it as "apiVersion"
    And I generate a unique API context from "/sc-embed-error" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9,"cacheUnauthenticated":true}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # The embeddings mock returns 500 for any input containing both "error" and "simulate"
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"error simulate embedding failure"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A different authenticated caller does not receive another caller's cached response
    Given I generate a unique value from "sc-isolation" and store it as "apiName"
    And I generate a unique API version from "sc-isolation" and store it as "apiVersion"
    And I generate a unique API context from "/sc-isolation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"semantic-cache","version":"v1","params":{"similarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys" with body:
      """
      {"name":"isolation-caller-a"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "callerKey"
    And I set header "API-Key" to "${CTX:callerKey}"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"isolation test prompt about coral reefs"}
      """

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until header "X-Cache-Status" is "HIT" with body:
      """
      {"prompt":"isolation test prompt about coral reefs"}
      """

    Given I authenticate using basic auth as "admin"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys" with body:
      """
      {"name":"isolation-caller-b"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "callerKey"
    And I set header "API-Key" to "${CTX:callerKey}"

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" until status 200 with body:
      """
      {"prompt":"isolation test prompt about coral reefs"}
      """
    Then the response header "X-Cache-Status" should not exist

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
