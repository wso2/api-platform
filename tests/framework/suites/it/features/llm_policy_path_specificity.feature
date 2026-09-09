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

@policy-path-specificity @llm
Feature: LLM policy path and method specificity
  As an API developer
  I want a policy attached to overlapping paths and methods on an LLM provider or proxy
  to apply only the most specific match (path first, then method) to each request
  So that overlapping path/method policies do not stack on the same route
  # Note: this applies to LlmProvider and LlmProxy (both go through the LLM transformer);
  # RestApi operations use explicit per-operation policies and are unaffected.
  #
  # Each distinct path/method pattern under test (e.g. /chat/completions vs /chat/* vs /*)
  # can resolve to its own Envoy route, so route readiness is folded into the FIRST request
  # against each distinct pattern via "until status 200" rather than assumed from a single
  # provider-wide probe. A 404 while a route is still propagating happens before the request
  # reaches the policy engine, so retries during that fold never consume a rate-limit quota.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ------------------------------------------------------------------
  # Regex syntax must not contribute to path specificity: otherwise the
  # (?:/.*)? suffix makes /a/* look longer than the narrower /a/b route,
  # causing Envoy to select the wildcard route and its policy chain.
  # ------------------------------------------------------------------
  Scenario: A shallow exact path selects its own policy chain ahead of a wildcard
    Given I generate a unique resource name from "pps-shallow-template" and store it as "templateName"
    And I generate a unique value from "pps-shallow-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-shallow-provider" and store it as "providerName"
    And I generate a unique value from "pps-shallow-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-shallow" and store it as "providerVersion"
    And I generate a unique API context from "/pps-shallow" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"set-headers","version":"v1","paths":[{"path":"/a/*","methods":["POST"],"params":{"response":{"headers":[{"name":"X-Matched-Policy","value":"wildcard"}]}}},{"path":"/a/b","methods":["POST"],"params":{"response":{"headers":[{"name":"X-Matched-Policy","value":"exact"}]}}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/a/b" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-Matched-Policy" should be "exact"

    When I send a "POST" request to "${CTX:providerContext}/a/c" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-Matched-Policy" should be "wildcard"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Reproduces a path-specificity bug using an LLM provider with the
  # advanced-ratelimit policy attached to TWO overlapping paths:
  #
  #   - POST /chat/completions -> 4 requests / hour   (specific)
  #   - /* (all methods)       -> 1 request  / hour   (wildcard catch-all)
  #
  # EXPECTED: a POST to /chat/completions is governed ONLY by the specific
  #           4/hour quota; the 1/hour wildcard quota governs every OTHER path.
  # ------------------------------------------------------------------
  Scenario: Overlapping specific and wildcard advanced-ratelimit paths apply only the most specific match
    Given I generate a unique resource name from "pps-advrl-template" and store it as "templateName"
    And I generate a unique value from "pps-advrl-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-advrl-provider" and store it as "providerName"
    And I generate a unique value from "pps-advrl-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-advrl" and store it as "providerVersion"
    And I generate a unique API context from "/pps-advrl" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"quotas":[{"name":"chat-quota","limits":[{"limit":4,"duration":"1h"}]}]}},{"path":"/*","methods":["*"],"params":{"quotas":[{"name":"wildcard-quota","limits":[{"limit":1,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # ----- /chat/completions must be governed ONLY by the specific 4/hour quota -----

    # Request 1 - allowed (chat-quota 1/4); folds route readiness for this specific path
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """

    # Request 2 - EXPECTED allowed (chat-quota 2/4).
    # A confirmed product bug fans the /* wildcard quota (1/hour) onto /chat/completions
    # too, so this request wrongly returns 429 once the wildcard bucket is exhausted -
    # see the @known-issue tag/exclusion for this scenario in it-suite.yaml.
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # Request 3 - allowed (chat-quota 3/4)
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # Request 4 - allowed (chat-quota 4/4)
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # Request 5 - blocked: the specific 4/hour chat-quota is now exhausted
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # ----- every OTHER path is governed by the 1/hour wildcard quota (intended) -----

    # Request 1 to /embeddings - allowed (wildcard-quota 1/1); folds route readiness for /*
    When I send a "POST" request to "${CTX:providerContext}/embeddings" until status 200 with body:
      """
      {"model":"text-embedding-3-small","input":"Hello"}
      """

    # Request 2 to /embeddings - blocked by the 1/hour wildcard quota (intended)
    When I send a "POST" request to "${CTX:providerContext}/embeddings" with body:
      """
      {"model":"text-embedding-3-small","input":"Hello"}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Generalises the fix beyond the root /*: the SAME policy attached to three
  # overlapping paths of decreasing specificity must apply only the most specific
  # match to each request -
  #   - /chat/completions -> 4 / hour   (exact, most specific)
  #   - /chat/*           -> 2 / hour   (nested wildcard)
  #   - /*                -> 1 / hour   (root catch-all)
  # Distinct quota names keep the per-path buckets isolated, and the X-RateLimit-Limit
  # header proves which quota governs each path.
  # ------------------------------------------------------------------
  Scenario: Overlapping nested paths each apply only their most specific advanced-ratelimit
    Given I generate a unique resource name from "pps-nested-template" and store it as "templateName"
    And I generate a unique value from "pps-nested-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-nested-provider" and store it as "providerName"
    And I generate a unique value from "pps-nested-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-nested" and store it as "providerVersion"
    And I generate a unique API context from "/pps-nested" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["*"],"params":{"quotas":[{"name":"chat-exact-quota","limits":[{"limit":4,"duration":"1h"}]}]}},{"path":"/chat/*","methods":["*"],"params":{"quotas":[{"name":"chat-wild-quota","limits":[{"limit":2,"duration":"1h"}]}]}},{"path":"/*","methods":["*"],"params":{"quotas":[{"name":"root-wild-quota","limits":[{"limit":1,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # /chat/completions -> governed ONLY by the exact-path quota (4/hour)
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "4"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 5th request exceeds the exact-path limit of 4
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # /chat/<other> -> governed ONLY by the nested wildcard /chat/* quota (2/hour)
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "2"
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 3rd request exceeds the /chat/* limit of 2
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # any other path -> governed ONLY by the root /* quota (1/hour)
    When I send a "POST" request to "${CTX:providerContext}/models" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "1"
    # 2nd request exceeds the /* limit of 1
    When I send a "POST" request to "${CTX:providerContext}/models" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Same path with different methods + order independence. The wildcard paths are
  # declared FIRST (before the specific /chat/completions entries) to prove the
  # outcome does not depend on declaration order:
  #   - POST /chat/completions -> 4  / hour
  #   - GET  /chat/completions -> 10 / hour
  #   - /chat/* (any method)   -> 2  / hour
  #   - /*      (any method)   -> 1  / hour
  # ------------------------------------------------------------------
  Scenario: Per-method limits on the same path win over wildcards regardless of declaration order
    Given I generate a unique resource name from "pps-method-template" and store it as "templateName"
    And I generate a unique value from "pps-method-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-method-provider" and store it as "providerName"
    And I generate a unique value from "pps-method-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-method" and store it as "providerVersion"
    And I generate a unique API context from "/pps-method" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"quotas":[{"name":"root-quota","limits":[{"limit":1,"duration":"1h"}]}]}},{"path":"/chat/*","methods":["*"],"params":{"quotas":[{"name":"chatwild-quota","limits":[{"limit":2,"duration":"1h"}]}]}},{"path":"/chat/completions","methods":["GET"],"params":{"quotas":[{"name":"cc-get-quota","limits":[{"limit":10,"duration":"1h"}]}]}},{"path":"/chat/completions","methods":["POST"],"params":{"quotas":[{"name":"cc-post-quota","limits":[{"limit":4,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # POST /chat/completions -> the POST-specific quota (4/hour)
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "4"
    And the response header "X-RateLimit-Remaining" should be "3"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 5th POST exceeds the POST limit of 4
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # GET /chat/completions -> the GET-specific quota (10/hour), a SEPARATE bucket from POST.
    # These succeed even though the POST quota is exhausted, proving method-level isolation.
    When I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200
    Then the response header "X-RateLimit-Limit" should be "10"
    And the response header "X-RateLimit-Remaining" should be "9"
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200

    # /chat/<other> -> the /chat/* quota (2/hour)
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "2"
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 3rd exceeds the /chat/* limit of 2
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # any other path -> the root /* quota (1/hour)
    When I send a "POST" request to "${CTX:providerContext}/models" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "1"
    # 2nd exceeds the /* limit of 1
    When I send a "POST" request to "${CTX:providerContext}/models" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Method specificity on the SAME path: a concrete method beats the '*' wildcard method,
  # mirroring how a specific path beats a wildcard path.
  #   - /chat/completions '*'  -> 4  / hour  (all methods)
  #   - /chat/completions GET  -> 10 / hour  (GET wins over '*' for GET requests)
  #   - /chat/*           '*'  -> 2  / hour
  #   - /*                '*'  -> 1  / hour
  # So GET /chat/completions = 10, every other method on /chat/completions = 4.
  # ------------------------------------------------------------------
  Scenario: A concrete method beats the wildcard method on the same path
    Given I generate a unique resource name from "pps-methodspec-template" and store it as "templateName"
    And I generate a unique value from "pps-methodspec-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-methodspec-provider" and store it as "providerName"
    And I generate a unique value from "pps-methodspec-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-methodspec" and store it as "providerVersion"
    And I generate a unique API context from "/pps-methodspec" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["*"],"params":{"quotas":[{"name":"cc-all-quota","limits":[{"limit":4,"duration":"1h"}]}]}},{"path":"/chat/completions","methods":["GET"],"params":{"quotas":[{"name":"cc-get-quota","limits":[{"limit":10,"duration":"1h"}]}]}},{"path":"/*","methods":["*"],"params":{"quotas":[{"name":"root-quota","limits":[{"limit":1,"duration":"1h"}]}]}},{"path":"/chat/*","methods":["*"],"params":{"quotas":[{"name":"chatwild-quota","limits":[{"limit":2,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # GET /chat/completions -> the GET-specific quota (10/hour), winning over the '*' entry (4)
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200
    Then the response header "X-RateLimit-Limit" should be "10"
    And the response header "X-RateLimit-Remaining" should be "9"
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    # 5th GET still succeeds (limit is 10, not 4) - proves GET is NOT governed by the '*' entry
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200

    # POST /chat/completions -> the '*' entry (4/hour); GET's usage did not touch this bucket
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "4"
    And the response header "X-RateLimit-Remaining" should be "3"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 5th POST exceeds the '*' entry limit of 4
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # /chat/<other> -> the /chat/* quota (2/hour)
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "2"
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 3rd exceeds the /chat/* limit of 2
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # any other path -> the root /* quota (1/hour)
    When I send a "POST" request to "${CTX:providerContext}/models" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "1"
    When I send a "POST" request to "${CTX:providerContext}/models" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Narrower method set wins on the same path: [POST] is more specific than [GET, POST].
  #   - /chat/completions [GET, POST] -> 3   / hour
  #   - /chat/completions [POST]      -> 100 / hour
  # So GET = 3 (only the [GET, POST] entry covers it), POST = 100 (the narrower [POST] entry).
  # ------------------------------------------------------------------
  Scenario: A narrower method set wins over a broader one on the same path
    Given I generate a unique resource name from "pps-narrow-template" and store it as "templateName"
    And I generate a unique value from "pps-narrow-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-narrow-provider" and store it as "providerName"
    And I generate a unique value from "pps-narrow-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-narrow" and store it as "providerVersion"
    And I generate a unique API context from "/pps-narrow" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET","POST"],"params":{"quotas":[{"name":"cc-readwrite-quota","limits":[{"limit":3,"duration":"1h"}]}]}},{"path":"/chat/completions","methods":["POST"],"params":{"quotas":[{"name":"cc-write-quota","limits":[{"limit":100,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # GET is covered only by the [GET, POST] entry -> 3/hour
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200
    Then the response header "X-RateLimit-Limit" should be "3"
    And the response header "X-RateLimit-Remaining" should be "2"
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 200
    # 4th GET exceeds the [GET, POST] limit of 3
    When I send a "GET" request to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # POST is covered by the narrower [POST] entry -> 100/hour (NOT capped at 3)
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should be "99"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 5th POST still succeeds (limit is 100, not 3) - proves POST uses the narrower [POST] entry
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # The same most-specific-wins logic applies to an LlmProxy's own policies.
  #   proxy policies: /chat/completions -> 4 / hour, /* -> 1 / hour
  # So /chat/completions = 4 (not also limited by the /* entry), other paths = 1.
  # ------------------------------------------------------------------
  Scenario: Most specific wins for an LLM proxy's policies
    Given I generate a unique resource name from "pps-proxyspec-template" and store it as "templateName"
    And I generate a unique value from "pps-proxyspec-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-proxyspec-provider" and store it as "providerName"
    And I generate a unique value from "pps-proxyspec-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-proxyspec-backend" and store it as "providerVersion"
    And I generate a unique API context from "/pps-proxyspec-backend" and store it as "providerContext"
    And I generate a unique resource name from "pps-proxyspec-proxy" and store it as "proxyName"
    And I generate a unique value from "pps-proxyspec-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "pps-proxyspec-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/pps-proxyspec-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    # Backing provider (no policies, just forwards to the echo backend)
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    # Proxy carrying the overlapping-path advanced-ratelimit policy
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:proxyName}                  |
      | displayName            | ${CTX:proxyDisplayName}           |
      | version                | ${CTX:proxyVersion}               |
      | context                | ${CTX:proxyContext}               |
      | provider.id            | ${CTX:providerName}                |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["*"],"params":{"quotas":[{"name":"proxy-cc-quota","limits":[{"limit":4,"duration":"1h"}]}]}},{"path":"/*","methods":["*"],"params":{"quotas":[{"name":"proxy-root-quota","limits":[{"limit":1,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # POST /chat/completions on the proxy -> the specific quota (4/hour)
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "4"
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    # 5th exceeds the specific limit of 4
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    # any other path on the proxy -> the /* quota (1/hour)
    When I send a "POST" request to "${CTX:proxyContext}/embeddings" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-RateLimit-Limit" should be "1"
    When I send a "POST" request to "${CTX:proxyContext}/embeddings" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429

    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Two separate policy blocks of the SAME name both apply. Each block is resolved
  # most-specific-within-itself, and every block layers onto the route. set-headers is used
  # (two blocks setting DIFFERENT response headers) so "both applied" is directly assertable:
  #   Block 1 (set-headers, X-Tier):  /chat/completions [*]->chat-all, [GET]->chat-get,
  #                                   /chat/* [*]->chat-wild, /* [*]->root
  #   Block 2 (set-headers, X-Global): /* [*]->global-applied
  # Every response carries the block-1 X-Tier for its most specific path/method AND the
  # block-2 X-Global, proving both same-name blocks apply.
  # ------------------------------------------------------------------
  Scenario: Two policy blocks of the same name both apply, each most-specific within itself
    Given I generate a unique resource name from "pps-twoblocks-template" and store it as "templateName"
    And I generate a unique value from "pps-twoblocks-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "pps-twoblocks-provider" and store it as "providerName"
    And I generate a unique value from "pps-twoblocks-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "pps-twoblocks" and store it as "providerVersion"
    And I generate a unique API context from "/pps-twoblocks" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}            |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:providerContext}            |
      | spec.upstream.url  | http://testbench:3002              |
      | accessControl.mode | allow_all                          |
      | spec.operationPolicies | [{"name":"set-headers","version":"v1","paths":[{"path":"/chat/completions","methods":["*"],"params":{"response":{"headers":[{"name":"X-Tier","value":"chat-all"}]}}},{"path":"/chat/completions","methods":["GET"],"params":{"response":{"headers":[{"name":"X-Tier","value":"chat-get"}]}}},{"path":"/chat/*","methods":["*"],"params":{"response":{"headers":[{"name":"X-Tier","value":"chat-wild"}]}}},{"path":"/*","methods":["*"],"params":{"response":{"headers":[{"name":"X-Tier","value":"root"}]}}}]},{"name":"set-headers","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"response":{"headers":[{"name":"X-Global","value":"global-applied"}]}}}]}] |
    Then the response status code should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"

    # POST /chat/completions -> block 1 most-specific is the [*] entry (chat-all); block 2 also applies
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-Tier" should be "chat-all"
    And the response header "X-Global" should be "global-applied"

    # GET /chat/completions -> block 1 most-specific is the [GET] entry (chat-get); block 2 also applies
    When I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200
    Then the response header "X-Tier" should be "chat-get"
    And the response header "X-Global" should be "global-applied"

    # /chat/<other> -> block 1 /chat/* (chat-wild); block 2 also applies
    When I send a "POST" request to "${CTX:providerContext}/chat/embeddings" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-Tier" should be "chat-wild"
    And the response header "X-Global" should be "global-applied"

    # any other path -> block 1 /* (root); block 2 also applies
    When I send a "POST" request to "${CTX:providerContext}/models" until status 200 with body:
      """
      {"model":"gpt-4"}
      """
    Then the response header "X-Tier" should be "root"
    And the response header "X-Global" should be "global-applied"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200
