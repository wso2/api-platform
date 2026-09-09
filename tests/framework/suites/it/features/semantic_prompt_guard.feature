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

@semantic-prompt-guard
Feature: Semantic prompt guard policy
  As an API developer
  I want to block or allow prompts based on semantic similarity
  So that I can protect my LLM from harmful or off-topic requests
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A prompt matching a denied phrase is blocked
    Given I generate a unique value from "pg-deny-block" and store it as "apiName"
    And I generate a unique API version from "pg-deny-block" and store it as "apiVersion"
    And I generate a unique API context from "/pg-deny-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["hack the system","bypass security"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A prompt not matching any denied phrase is allowed
    Given I generate a unique value from "pg-deny-allow" and store it as "apiName"
    And I generate a unique API version from "pg-deny-allow" and store it as "apiVersion"
    And I generate a unique API context from "/pg-deny-allow" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["hack the system"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"what is the weather today"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A prompt matching an allowed phrase is permitted
    Given I generate a unique value from "pg-allow-match" and store it as "apiName"
    And I generate a unique API version from "pg-allow-match" and store it as "apiVersion"
    And I generate a unique API context from "/pg-allow-match" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","allowedPhrases":["customer support","product inquiry"],"allowSimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"customer support"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A prompt not matching any allowed phrase is blocked
    Given I generate a unique value from "pg-allow-block" and store it as "apiName"
    And I generate a unique API version from "pg-allow-block" and store it as "apiVersion"
    And I generate a unique API context from "/pg-allow-block" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","allowedPhrases":["customer support"],"allowSimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A denied phrase match takes priority over an allowed phrase match
    Given I generate a unique value from "pg-both-deny" and store it as "apiName"
    And I generate a unique API version from "pg-both-deny" and store it as "apiVersion"
    And I generate a unique API context from "/pg-both-deny" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","allowedPhrases":["general questions"],"deniedPhrases":["hack"],"allowSimilarityThreshold":0.5,"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A prompt matching an allowed phrase and no denied phrase is permitted
    Given I generate a unique value from "pg-both-allow" and store it as "apiName"
    And I generate a unique API version from "pg-both-allow" and store it as "apiVersion"
    And I generate a unique API context from "/pg-both-allow" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","allowedPhrases":["customer support question"],"deniedPhrases":["hack the system"],"allowSimilarityThreshold":0.9,"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"customer support question"}
      """
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Measured similarity between these two prompts is ~0.33 (via the embeddings mock's
  # /debug/similarity endpoint) - well below 0.99, so this is a genuine strict-match test.
  Scenario: A high allow threshold requires strict matching
    Given I generate a unique value from "pg-high-allow-threshold" and store it as "apiName"
    And I generate a unique API version from "pg-high-allow-threshold" and store it as "apiVersion"
    And I generate a unique API context from "/pg-high-allow-threshold" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","allowedPhrases":["hello world"],"allowSimilarityThreshold":0.99}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hello world"}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hi there world"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Measured similarity between "malicious attack" and "malicious attack 1" is ~0.85 -
  # above 0.5 but below 0.99, driving the low- and high-threshold scenarios below.
  Scenario: A low deny threshold provides broad blocking
    Given I generate a unique value from "pg-low-deny-threshold" and store it as "apiName"
    And I generate a unique API version from "pg-low-deny-threshold" and store it as "apiVersion"
    And I generate a unique API context from "/pg-low-deny-threshold" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["malicious attack"],"denySimilarityThreshold":0.5}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"malicious attack 1"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A high deny threshold requires a near-exact match
    Given I generate a unique value from "pg-high-deny-threshold" and store it as "apiName"
    And I generate a unique API version from "pg-high-deny-threshold" and store it as "apiVersion"
    And I generate a unique API context from "/pg-high-deny-threshold" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["malicious attack"],"denySimilarityThreshold":0.99}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"malicious attack 1"}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"malicious attack"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JSONPath extraction validates only the targeted field
    Given I generate a unique value from "pg-jsonpath" and store it as "apiName"
    And I generate a unique API version from "pg-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/pg-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.messages[0].content","deniedPhrases":["hack"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # "hack" in the system field should be ignored - only messages[0].content is validated
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"messages":[{"role":"user","content":"normal request"}],"system":"hack the server"}
      """
    Then the response status code should be 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"messages":[{"role":"user","content":"hack"}],"system":"safe content"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A JSONPath that does not resolve is rejected as a policy error
    Given I generate a unique value from "pg-invalid-jsonpath" and store it as "apiName"
    And I generate a unique API version from "pg-invalid-jsonpath" and store it as "apiVersion"
    And I generate a unique API context from "/pg-invalid-jsonpath" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.nonexistent.field","deniedPhrases":["test"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"message":"hello world"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: showAssessment true includes detailed similarity information
    Given I generate a unique value from "pg-assessment-true" and store it as "apiName"
    And I generate a unique API version from "pg-assessment-true" and store it as "apiVersion"
    And I generate a unique API context from "/pg-assessment-true" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","showAssessment":true,"deniedPhrases":["hack the system"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "assessments"
    And the response body should contain "similarity"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: showAssessment false returns minimal information
    Given I generate a unique value from "pg-assessment-false" and store it as "apiName"
    And I generate a unique API version from "pg-assessment-false" and store it as "apiVersion"
    And I generate a unique API context from "/pg-assessment-false" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","showAssessment":false,"deniedPhrases":["hack the system"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "GUARDRAIL_INTERVENED"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An empty request body is rejected
    Given I generate a unique value from "pg-empty-body" and store it as "apiName"
    And I generate a unique API version from "pg-empty-body" and store it as "apiVersion"
    And I generate a unique API context from "/pg-empty-body" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"deniedPhrases":["test"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # Unlike semantic-cache (which passes the request through unmodified on an embedding
  # failure), semantic-prompt-guard cannot verify a prompt is safe without an embedding, so
  # it fails closed - a deliberate, version-independent product behavior difference.
  Scenario: An embedding provider failure is rejected as a policy error
    Given I generate a unique value from "pg-embed-error" and store it as "apiName"
    And I generate a unique API version from "pg-embed-error" and store it as "apiVersion"
    And I generate a unique API context from "/pg-embed-error" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["test phrase"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    # The embeddings mock returns 500 for any input containing both "error" and "simulate"
    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"simulate error in embedding"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  # The embedding mock lowercases before hashing, so an uppercase denied phrase still
  # matches a lowercase request - confirmed identical (~1.0) via /debug/similarity.
  Scenario: Semantic matching is case-insensitive
    Given I generate a unique value from "pg-case-insensitive" and store it as "apiName"
    And I generate a unique API version from "pg-case-insensitive" and store it as "apiVersion"
    And I generate a unique API context from "/pg-case-insensitive" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"POST","path":"/chat","policies":[{"name":"semantic-prompt-guard","version":"v1","params":{"jsonPath":"$.prompt","deniedPhrases":["HACK THE SYSTEM"],"denySimilarityThreshold":0.9}}]},{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/chat" with body:
      """
      {"prompt":"hack the system"}
      """
    Then the response status code should be 422

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
