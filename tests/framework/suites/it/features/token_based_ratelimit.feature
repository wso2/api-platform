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

@token-based-ratelimit
Feature: Token-based rate limiting for LLM providers
  As an API developer
  I want to rate limit LLM traffic by prompt, completion, and total token usage
  So that I can protect upstream LLM budgets rather than just counting requests

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce a prompt-token and a total-token quota together
    Given I generate a unique resource name from "tbrl-basic-template" and store it as "templateName"
    And I generate a unique value from "tbrl-basic-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-basic-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-basic-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-basic" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-basic" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1                                    |
      | name                   | ${CTX:templateName}                                                 |
      | displayName            | ${CTX:templateDisplayName}                                          |
      | spec.promptTokens      | {"location":"payload","identifier":"$.json.usage.prompt_tokens"}     |
      | spec.totalTokens       | {"location":"payload","identifier":"$.json.usage.total_tokens"}      |
      | spec.requestModel      | {"location":"payload","identifier":"$.json.model"}                    |
      | spec.responseModel     | {"location":"payload","identifier":"$.json.model"}                    |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"promptTokenLimits":[{"count":10,"duration":"1m"}],"totalTokenLimits":[{"count":20,"duration":"1m"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":5}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":5}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":1}}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Prompt, completion, and total quotas are tracked and enforced independently
    Given I generate a unique resource name from "tbrl-multi-template" and store it as "templateName"
    And I generate a unique value from "tbrl-multi-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-multi-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-multi-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-multi" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-multi" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1                                        |
      | name                   | ${CTX:templateName}                                                     |
      | displayName            | ${CTX:templateDisplayName}                                              |
      | spec.promptTokens      | {"location":"payload","identifier":"$.json.usage.prompt_tokens"}         |
      | spec.completionTokens  | {"location":"payload","identifier":"$.json.usage.completion_tokens"}     |
      | spec.totalTokens       | {"location":"payload","identifier":"$.json.usage.total_tokens"}          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"promptTokenLimits":[{"count":5,"duration":"1m"}],"completionTokenLimits":[{"count":10,"duration":"1m"}],"totalTokenLimits":[{"count":15,"duration":"1m"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":5,"completion_tokens":5,"total_tokens":10}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Token extraction works against a gzip-compressed backend response
    Given I generate a unique resource name from "tbrl-gzip-template" and store it as "templateName"
    And I generate a unique value from "tbrl-gzip-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-gzip-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-gzip-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-gzip" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-gzip" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1                                |
      | name               | ${CTX:templateName}                                             |
      | displayName        | ${CTX:templateDisplayName}                                      |
      | spec.totalTokens   | {"location":"payload","identifier":"$.args.total_tokens[0]"}     |
      | spec.requestModel  | {"location":"payload","identifier":"$.args.model[0]"}            |
      | spec.responseModel | {"location":"payload","identifier":"$.args.model[0]"}            |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"request-rewrite","version":"v1","paths":[{"path":"/chat/completions","methods":["POST","GET"],"params":{"pathRewrite":{"type":"ReplaceFullPath","replaceFullPath":"/gzip"}}}]},{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":2,"duration":"1m"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I set header "Content-Type" to "application/json"
    And I set header "Accept-Encoding" to "gzip"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions?model=gpt-4&total_tokens=1" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "Content-Encoding" should contain "gzip"
    And the response header "X-RateLimit-Remaining" should be "1"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions?model=gpt-4&total_tokens=1" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions?model=gpt-4&total_tokens=1" with body:
      """
      {}
      """
    Then the response status code should be 429
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: A successful response reports rate limit headers
    Given I generate a unique resource name from "tbrl-headers-template" and store it as "templateName"
    And I generate a unique value from "tbrl-headers-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-headers-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-headers-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-headers" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-headers" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1                                     |
      | name               | ${CTX:templateName}                                                  |
      | displayName        | ${CTX:templateDisplayName}                                           |
      | spec.totalTokens   | {"location":"payload","identifier":"$.json.usage.total_tokens"}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":100,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should exist
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Separate providers with separate templates have isolated quotas
    Given I generate a unique resource name from "tbrl-iso-template-a" and store it as "templateNameA"
    And I generate a unique value from "tbrl-iso-template-a" and store it as "templateDisplayNameA"
    And I generate a unique resource name from "tbrl-iso-provider-a" and store it as "providerNameA"
    And I generate a unique value from "tbrl-iso-provider-a" and store it as "providerDisplayNameA"
    And I generate a unique API version from "tbrl-iso-a" and store it as "providerVersionA"
    And I generate a unique API context from "/tbrl-iso-a" and store it as "providerContextA"
    And I generate a unique resource name from "tbrl-iso-template-b" and store it as "templateNameB"
    And I generate a unique value from "tbrl-iso-template-b" and store it as "templateDisplayNameB"
    And I generate a unique resource name from "tbrl-iso-provider-b" and store it as "providerNameB"
    And I generate a unique value from "tbrl-iso-provider-b" and store it as "providerDisplayNameB"
    And I generate a unique API version from "tbrl-iso-b" and store it as "providerVersionB"
    And I generate a unique API context from "/tbrl-iso-b" and store it as "providerContextB"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                  |
      | name             | ${CTX:templateNameA}                                              |
      | displayName      | ${CTX:templateDisplayNameA}                                       |
      | spec.totalTokens | {"location":"payload","identifier":"$.json.usage.total_tokens"}    |
    Then the response status code should be 201

    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                  |
      | name             | ${CTX:templateNameB}                                              |
      | displayName      | ${CTX:templateDisplayNameB}                                       |
      | spec.totalTokens | {"location":"payload","identifier":"$.json.usage.total_tokens"}    |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerNameA}              |
      | displayName            | ${CTX:providerDisplayNameA}       |
      | version                | ${CTX:providerVersionA}           |
      | template               | ${CTX:templateNameA}              |
      | spec.context           | ${CTX:providerContextA}           |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":5,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContextA}/chat/completions" until status 200

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerNameB}              |
      | displayName            | ${CTX:providerDisplayNameB}       |
      | version                | ${CTX:providerVersionB}           |
      | template               | ${CTX:templateNameB}              |
      | spec.context           | ${CTX:providerContextB}           |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":5,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContextB}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContextA}/chat/completions" with body:
      """
      {"usage":{"total_tokens":3}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContextA}/chat/completions" with body:
      """
      {"usage":{"total_tokens":2}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContextA}/chat/completions" with body:
      """
      {"usage":{"total_tokens":1}}
      """
    Then the response status code should be 429

    When I send a "POST" request to "${CTX:providerContextB}/chat/completions" with body:
      """
      {"usage":{"total_tokens":6}}
      """
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerNameA}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerNameB}"
    Then the response should be successful

  Scenario: Multiple quotas are enforced independently and the violated quota is identified
    Given I generate a unique resource name from "tbrl-detail-template" and store it as "templateName"
    And I generate a unique value from "tbrl-detail-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-detail-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-detail-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-detail" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-detail" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                        |
      | name                  | ${CTX:templateName}                                                     |
      | displayName           | ${CTX:templateDisplayName}                                              |
      | spec.promptTokens     | {"location":"payload","identifier":"$.json.usage.prompt_tokens"}         |
      | spec.completionTokens | {"location":"payload","identifier":"$.json.usage.completion_tokens"}     |
      | spec.totalTokens      | {"location":"payload","identifier":"$.json.usage.total_tokens"}          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"promptTokenLimits":[{"count":10,"duration":"1m"}],"completionTokenLimits":[{"count":20,"duration":"1m"}],"totalTokenLimits":[{"count":25,"duration":"1m"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":8,"completion_tokens":15,"total_tokens":23}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And the response header "X-RateLimit-Quota" should exist

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: The rate limit window resets once its duration elapses
    Given I generate a unique resource name from "tbrl-window-template" and store it as "templateName"
    And I generate a unique value from "tbrl-window-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-window-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-window-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-window" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-window" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                  |
      | name             | ${CTX:templateName}                                               |
      | displayName      | ${CTX:templateDisplayName}                                        |
      | spec.totalTokens | {"location":"payload","identifier":"$.json.usage.total_tokens"}    |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":5,"duration":"10s"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":5}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":1}}
      """
    Then the response status code should be 429

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until header "X-RateLimit-Remaining" is "5" with body:
      """
      {"usage":{"total_tokens":0}}
      """
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Zero token usage does not consume quota
    Given I generate a unique resource name from "tbrl-zero-template" and store it as "templateName"
    And I generate a unique value from "tbrl-zero-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-zero-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-zero-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-zero" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-zero" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                  |
      | name             | ${CTX:templateName}                                               |
      | displayName      | ${CTX:templateDisplayName}                                        |
      | spec.totalTokens | {"location":"payload","identifier":"$.json.usage.total_tokens"}    |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":10,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":5}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "5"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":0}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "5"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":5}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":1}}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Cost extracted from a request header blocks before reaching the upstream
    Given I generate a unique resource name from "tbrl-header-cost-template" and store it as "templateName"
    And I generate a unique value from "tbrl-header-cost-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-header-cost-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-header-cost-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-header-cost" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-header-cost" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                |
      | name             | ${CTX:templateName}                                             |
      | displayName      | ${CTX:templateDisplayName}                                      |
      | spec.totalTokens | {"location":"header","identifier":"X-Token-Cost"}                 |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":10,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I set header "Content-Type" to "application/json"
    And I set header "X-Token-Cost" to "6"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "4"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4"}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Deleting and recreating a provider starts with a fresh quota
    Given I generate a unique resource name from "tbrl-recreate-template" and store it as "templateName"
    And I generate a unique value from "tbrl-recreate-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-recreate-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-recreate-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-recreate" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-recreate" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                 |
      | name             | ${CTX:templateName}                                              |
      | displayName      | ${CTX:templateDisplayName}                                       |
      | spec.totalTokens | {"location":"payload","identifier":"$.json.usage.total_tokens"}   |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":5,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":5}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":1}}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":10,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200
    # A deleted provider's own quota state is not guaranteed to invalidate synchronously with the
    # delete call, so recreating under the same name can still observe the old provider's counter
    # for a moment. Poll (at zero cost, so it can never itself consume quota) until the new limit
    # is actually being served before trusting the counter it reports.
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until header "X-RateLimit-Limit" is "10" with body:
      """
      {"usage":{"total_tokens":0}}
      """

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"total_tokens":5}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "5"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Different providers sharing the same template have isolated quotas
    Given I generate a unique resource name from "tbrl-shared-template" and store it as "templateName"
    And I generate a unique value from "tbrl-shared-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-shared-alpha" and store it as "providerNameAlpha"
    And I generate a unique value from "tbrl-shared-alpha" and store it as "providerDisplayNameAlpha"
    And I generate a unique API version from "tbrl-shared-alpha" and store it as "providerVersionAlpha"
    And I generate a unique API context from "/tbrl-shared-alpha" and store it as "providerContextAlpha"
    And I generate a unique resource name from "tbrl-shared-beta" and store it as "providerNameBeta"
    And I generate a unique value from "tbrl-shared-beta" and store it as "providerDisplayNameBeta"
    And I generate a unique API version from "tbrl-shared-beta" and store it as "providerVersionBeta"
    And I generate a unique API context from "/tbrl-shared-beta" and store it as "providerContextBeta"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion       | gateway.api-platform.wso2.com/v1                                 |
      | name             | ${CTX:templateName}                                              |
      | displayName      | ${CTX:templateDisplayName}                                       |
      | spec.totalTokens | {"location":"payload","identifier":"$.json.usage.total_tokens"}   |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerNameAlpha}          |
      | displayName            | ${CTX:providerDisplayNameAlpha}   |
      | version                | ${CTX:providerVersionAlpha}       |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContextAlpha}       |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":5,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContextAlpha}/chat/completions" until status 200

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerNameBeta}           |
      | displayName            | ${CTX:providerDisplayNameBeta}    |
      | version                | ${CTX:providerVersionBeta}        |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContextBeta}        |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"totalTokenLimits":[{"count":5,"duration":"1h"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContextBeta}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContextAlpha}/chat/completions" with body:
      """
      {"usage":{"total_tokens":5}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send a "POST" request to "${CTX:providerContextAlpha}/chat/completions" with body:
      """
      {"usage":{"total_tokens":1}}
      """
    Then the response status code should be 429

    When I send a "POST" request to "${CTX:providerContextBeta}/chat/completions" with body:
      """
      {"usage":{"total_tokens":3}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "2"

    When I delete the LLM provider "${CTX:providerNameAlpha}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:providerNameBeta}"
    Then the response should be successful

  Scenario: An explicit empty prompt and completion limit still enforces the total-token limit
    Given I generate a unique resource name from "tbrl-empty-both-template" and store it as "templateName"
    And I generate a unique value from "tbrl-empty-both-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-empty-both-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-empty-both-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-empty-both" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-empty-both" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                        |
      | name                  | ${CTX:templateName}                                                     |
      | displayName           | ${CTX:templateDisplayName}                                              |
      | spec.promptTokens     | {"location":"payload","identifier":"$.json.usage.prompt_tokens"}         |
      | spec.completionTokens | {"location":"payload","identifier":"$.json.usage.completion_tokens"}     |
      | spec.totalTokens      | {"location":"payload","identifier":"$.json.usage.total_tokens"}          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"promptTokenLimits":[],"completionTokenLimits":[],"totalTokenLimits":[{"count":5,"duration":"1m"}],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":0,"completion_tokens":5,"total_tokens":5}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":0,"completion_tokens":1,"total_tokens":1}}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: An explicit empty completion and total limit still enforces the prompt-token limit
    Given I generate a unique resource name from "tbrl-prompt-only-template" and store it as "templateName"
    And I generate a unique value from "tbrl-prompt-only-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-prompt-only-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-prompt-only-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-prompt-only" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-prompt-only" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                        |
      | name                  | ${CTX:templateName}                                                     |
      | displayName           | ${CTX:templateDisplayName}                                              |
      | spec.promptTokens     | {"location":"payload","identifier":"$.json.usage.prompt_tokens"}         |
      | spec.completionTokens | {"location":"payload","identifier":"$.json.usage.completion_tokens"}     |
      | spec.totalTokens      | {"location":"payload","identifier":"$.json.usage.total_tokens"}          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"promptTokenLimits":[{"count":5,"duration":"1m"}],"completionTokenLimits":[],"totalTokenLimits":[],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":5}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":1,"completion_tokens":0,"total_tokens":1}}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: An explicit empty prompt and total limit still enforces the completion-token limit
    Given I generate a unique resource name from "tbrl-completion-only-template" and store it as "templateName"
    And I generate a unique value from "tbrl-completion-only-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "tbrl-completion-only-provider" and store it as "providerName"
    And I generate a unique value from "tbrl-completion-only-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "tbrl-completion-only" and store it as "providerVersion"
    And I generate a unique API context from "/tbrl-completion-only" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1                                        |
      | name                  | ${CTX:templateName}                                                     |
      | displayName           | ${CTX:templateDisplayName}                                              |
      | spec.promptTokens     | {"location":"payload","identifier":"$.json.usage.prompt_tokens"}         |
      | spec.completionTokens | {"location":"payload","identifier":"$.json.usage.completion_tokens"}     |
      | spec.totalTokens      | {"location":"payload","identifier":"$.json.usage.total_tokens"}          |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:providerName}               |
      | displayName            | ${CTX:providerDisplayName}        |
      | version                | ${CTX:providerVersion}            |
      | template               | ${CTX:templateName}               |
      | spec.context           | ${CTX:providerContext}            |
      | spec.upstream.url      | http://testbench:3002             |
      | accessControl.mode     | allow_all                          |
      | spec.policies          | [{"name":"token-based-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"promptTokenLimits":[],"completionTokenLimits":[{"count":5,"duration":"1m"}],"totalTokenLimits":[],"algorithm":"fixed-window","backend":"memory"}}]}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":0,"completion_tokens":5,"total_tokens":5}}
      """
    Then the response status code should be 200

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"usage":{"prompt_tokens":0,"completion_tokens":1,"total_tokens":1}}
      """
    Then the response status code should be 429

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
