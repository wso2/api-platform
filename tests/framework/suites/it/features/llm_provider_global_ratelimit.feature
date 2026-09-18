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

@llm-provider-wide-ratelimit
Feature: Provider-wide rate limiting for LLM providers and proxies
  As an API developer
  I want a single rate-limit bucket shared across every resource of an LLM provider or proxy
  So that I can enforce one provider-wide quota regardless of which resource is called,
  or scope a quota to a single resource when that is what I need instead

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: globalPolicies shares one rate-limit bucket across all resources
    # Exhausting /chat/completions also limits /embeddings because they share a single
    # provider-level bucket.
    Given I generate a unique resource name from "gplr-global-template" and store it as "templateName"
    And I generate a unique value from "gplr-global-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-global-provider" and store it as "providerName"
    And I generate a unique value from "gplr-global-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-global" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-global" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:providerName}               |
      | displayName             | ${CTX:providerDisplayName}        |
      | version                 | ${CTX:providerVersion}            |
      | template                | ${CTX:templateName}               |
      | spec.context            | ${CTX:providerContext}            |
      | spec.upstream.url       | http://testbench:3002             |
      | accessControl.mode      | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.globalPolicies     | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":10,"duration":"1h"}]}}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    # Exhaust the shared provider-level bucket via /chat/completions
    When I send 20 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: a separate resource must ALSO be limited - it shares the same bucket
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: operationPolicies keeps independent rate-limit buckets per resource
    # Exhausting /chat/completions does NOT affect /embeddings - it has its own bucket.
    Given I generate a unique resource name from "gplr-op-template" and store it as "templateName"
    And I generate a unique value from "gplr-op-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-op-provider" and store it as "providerName"
    And I generate a unique value from "gplr-op-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-op" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-op" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:providerName}               |
      | displayName              | ${CTX:providerDisplayName}        |
      | version                  | ${CTX:providerVersion}            |
      | template                 | ${CTX:templateName}               |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3002             |
      | accessControl.mode       | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.operationPolicies   | [{"name":"basic-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"limits":[{"requests":10,"duration":"1h"}]}},{"path":"/embeddings","methods":["GET"],"params":{"limits":[{"requests":10,"duration":"1h"}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send 20 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # Separate resource has its OWN bucket - still served
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: globalPolicies and operationPolicies coexist - per-resource cap blocks one resource without affecting another
    # /chat/completions is blocked by its tighter per-resource bucket; /embeddings is isolated
    # from that exhaustion but is eventually capped by the shared global bucket.
    Given I generate a unique resource name from "gplr-combined-template" and store it as "templateName"
    And I generate a unique value from "gplr-combined-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-combined-provider" and store it as "providerName"
    And I generate a unique value from "gplr-combined-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-combined" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-combined" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:providerName}               |
      | displayName              | ${CTX:providerDisplayName}        |
      | version                  | ${CTX:providerVersion}            |
      | template                 | ${CTX:templateName}               |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3002             |
      | accessControl.mode       | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.globalPolicies      | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":20,"duration":"1h"}]}}] |
      | spec.operationPolicies   | [{"name":"basic-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"limits":[{"requests":5,"duration":"1h"}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    # Per-resource op bucket for /chat/completions is exhausted (limit: 5)
    When I send 10 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no op policy - its independent bucket is untouched
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 200

    # Exhaust the shared global bucket via /embeddings (global limit: 20; at least 5 already consumed)
    When I send 25 "GET" requests to "${CTX:providerContext}/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: deprecated policies field still enforces independent per-resource rate limits
    Given I generate a unique resource name from "gplr-legacy-template" and store it as "templateName"
    And I generate a unique value from "gplr-legacy-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-legacy-provider" and store it as "providerName"
    And I generate a unique value from "gplr-legacy-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-legacy" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-legacy" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:providerName}               |
      | displayName              | ${CTX:providerDisplayName}        |
      | version                  | ${CTX:providerVersion}            |
      | template                 | ${CTX:templateName}               |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3002             |
      | accessControl.mode       | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.policies            | [{"name":"basic-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"limits":[{"requests":10,"duration":"1h"}]}},{"path":"/embeddings","methods":["GET"],"params":{"limits":[{"requests":10,"duration":"1h"}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send 20 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # Separate resource has its OWN bucket - still served
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: advanced-ratelimit globalPolicies with keyExtraction=apiname shares one bucket across all provider resources
    # All operations share ONE counter. Exhausting /chat/completions also limits /embeddings
    # because the counter key is the API name, not the route.
    Given I generate a unique resource name from "gplr-adv-gl-template" and store it as "templateName"
    And I generate a unique value from "gplr-adv-gl-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-adv-gl-provider" and store it as "providerName"
    And I generate a unique value from "gplr-adv-gl-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-adv-gl" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-adv-gl" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:providerName}               |
      | displayName              | ${CTX:providerDisplayName}        |
      | version                  | ${CTX:providerVersion}            |
      | template                 | ${CTX:templateName}               |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3002             |
      | accessControl.mode       | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.globalPolicies      | [{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":10,"duration":"1h"}]}],"keyExtraction":[{"type":"apiname"}]}}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send 20 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings shares the same apiname-keyed bucket - must also be limited
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: advanced-ratelimit operationPolicies without keyExtraction keeps independent buckets per provider resource
    # Default key is routename, so each operation gets its own isolated counter. Exhausting
    # /chat/completions leaves /embeddings unaffected.
    Given I generate a unique resource name from "gplr-adv-op-template" and store it as "templateName"
    And I generate a unique value from "gplr-adv-op-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-adv-op-provider" and store it as "providerName"
    And I generate a unique value from "gplr-adv-op-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-adv-op" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-adv-op" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:providerName}               |
      | displayName              | ${CTX:providerDisplayName}        |
      | version                  | ${CTX:providerVersion}            |
      | template                 | ${CTX:templateName}               |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3002             |
      | accessControl.mode       | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.operationPolicies   | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"quotas":[{"name":"request-limit","limits":[{"limit":10,"duration":"1h"}]}]}},{"path":"/embeddings","methods":["GET"],"params":{"quotas":[{"name":"request-limit","limits":[{"limit":10,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send 20 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # /embeddings has its own routename-keyed bucket - completely unaffected
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 200

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: advanced-ratelimit globalPolicies with keyExtraction=apiname shares one bucket across all proxy resources
    # The proxy exposes multiple operations; the shared apiname counter spans all of them.
    Given I generate a unique resource name from "gplr-adv-gl-px-template" and store it as "templateName"
    And I generate a unique value from "gplr-adv-gl-px-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-adv-gl-px-backend" and store it as "backendName"
    And I generate a unique value from "gplr-adv-gl-px-backend" and store it as "backendDisplayName"
    And I generate a unique API version from "gplr-adv-gl-px-backend" and store it as "backendVersion"
    And I generate a unique API context from "/gplr-adv-gl-px-backend" and store it as "backendContext"
    And I generate a unique resource name from "gplr-adv-gl-proxy" and store it as "proxyName"
    And I generate a unique value from "gplr-adv-gl-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "gplr-adv-gl-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/gplr-adv-gl-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:backendName}                |
      | displayName        | ${CTX:backendDisplayName}         |
      | version            | ${CTX:backendVersion}             |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:backendContext}             |
      | spec.upstream.url  | http://testbench:3002             |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion          | gateway.api-platform.wso2.com/v1 |
      | name                | ${CTX:proxyName}                  |
      | displayName         | ${CTX:proxyDisplayName}           |
      | version             | ${CTX:proxyVersion}               |
      | context             | ${CTX:proxyContext}               |
      | provider.id         | ${CTX:backendName}                |
      | spec.globalPolicies | [{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":10,"duration":"1h"}]}],"keyExtraction":[{"type":"apiname"}]}}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:proxyContext}/chat/completions" until status 200

    # Exhaust the shared api-level proxy bucket via /chat/completions
    When I send 20 "GET" requests to "${CTX:proxyContext}/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings shares the same apiname-keyed bucket - must also be limited
    When I send a "GET" request to "${CTX:proxyContext}/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:backendName}"
    Then the response should be successful

  Scenario: advanced-ratelimit operationPolicies without keyExtraction keeps independent buckets per proxy resource
    # Default routename key gives each operation its own isolated counter. Exhausting
    # /chat/completions leaves /embeddings unaffected.
    Given I generate a unique resource name from "gplr-adv-op-px-template" and store it as "templateName"
    And I generate a unique value from "gplr-adv-op-px-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-adv-op-px-backend" and store it as "backendName"
    And I generate a unique value from "gplr-adv-op-px-backend" and store it as "backendDisplayName"
    And I generate a unique API version from "gplr-adv-op-px-backend" and store it as "backendVersion"
    And I generate a unique API context from "/gplr-adv-op-px-backend" and store it as "backendContext"
    And I generate a unique resource name from "gplr-adv-op-proxy" and store it as "proxyName"
    And I generate a unique value from "gplr-adv-op-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "gplr-adv-op-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/gplr-adv-op-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:backendName}                |
      | displayName        | ${CTX:backendDisplayName}         |
      | version            | ${CTX:backendVersion}             |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:backendContext}             |
      | spec.upstream.url  | http://testbench:3002             |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:proxyName}                  |
      | displayName            | ${CTX:proxyDisplayName}           |
      | version                | ${CTX:proxyVersion}               |
      | context                | ${CTX:proxyContext}               |
      | provider.id            | ${CTX:backendName}                |
      | spec.operationPolicies | [{"name":"advanced-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"quotas":[{"name":"request-limit","limits":[{"limit":10,"duration":"1h"}]}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:proxyContext}/chat/completions" until status 200

    When I send 20 "GET" requests to "${CTX:proxyContext}/chat/completions"
    Then the response status code should be 429

    # /embeddings has its own routename-keyed bucket - completely unaffected
    When I send a "GET" request to "${CTX:proxyContext}/embeddings"
    Then the response status code should be 200

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:backendName}"
    Then the response should be successful

  Scenario: mixed advanced-ratelimit global and basic-ratelimit operation on provider - global bucket exhausted by rejected operation traffic
    # Operation policy (3/hr) fires before global (5/hr) for /chat/completions. Global still
    # increments on every attempt - including those the operation policy rejects - so
    # /embeddings (no operation policy of its own) eventually hits the global cap too.
    Given I generate a unique resource name from "gplr-mix-template" and store it as "templateName"
    And I generate a unique value from "gplr-mix-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-mix-provider" and store it as "providerName"
    And I generate a unique value from "gplr-mix-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "gplr-mix" and store it as "providerVersion"
    And I generate a unique API context from "/gplr-mix" and store it as "providerContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion               | gateway.api-platform.wso2.com/v1 |
      | name                     | ${CTX:providerName}               |
      | displayName              | ${CTX:providerDisplayName}        |
      | version                  | ${CTX:providerVersion}            |
      | template                 | ${CTX:templateName}               |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3002             |
      | accessControl.mode       | deny_all                            |
      | spec.accessControl.exceptions | [{"path":"/chat/completions","methods":["GET"]},{"path":"/embeddings","methods":["GET"]}] |
      | spec.globalPolicies      | [{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":5,"duration":"1h"}]}],"keyExtraction":[{"type":"apiname"}]}}] |
      | spec.operationPolicies   | [{"name":"basic-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"limits":[{"requests":3,"duration":"1h"}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 200

    When I send 10 "GET" requests to "${CTX:providerContext}/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no operation policy but shares the global apiname bucket,
    # already exhausted by /chat/completions traffic
    When I send a "GET" request to "${CTX:providerContext}/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: mixed advanced-ratelimit global and basic-ratelimit operation on proxy - global bucket exhausted by rejected operation traffic
    # Same as the provider case above, but the policies are attached to the proxy instead.
    Given I generate a unique resource name from "gplr-mix-px-template" and store it as "templateName"
    And I generate a unique value from "gplr-mix-px-template" and store it as "templateDisplayName"
    And I generate a unique resource name from "gplr-mix-px-backend" and store it as "backendName"
    And I generate a unique value from "gplr-mix-px-backend" and store it as "backendDisplayName"
    And I generate a unique API version from "gplr-mix-px-backend" and store it as "backendVersion"
    And I generate a unique API context from "/gplr-mix-px-backend" and store it as "backendContext"
    And I generate a unique resource name from "gplr-mix-proxy" and store it as "proxyName"
    And I generate a unique value from "gplr-mix-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "gplr-mix-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/gplr-mix-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | ${CTX:templateDisplayName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:backendName}                |
      | displayName        | ${CTX:backendDisplayName}         |
      | version            | ${CTX:backendVersion}             |
      | template           | ${CTX:templateName}               |
      | spec.context       | ${CTX:backendContext}             |
      | spec.upstream.url  | http://testbench:3002             |
      | accessControl.mode | allow_all                          |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:proxyName}                  |
      | displayName            | ${CTX:proxyDisplayName}           |
      | version                | ${CTX:proxyVersion}               |
      | context                | ${CTX:proxyContext}               |
      | provider.id            | ${CTX:backendName}                |
      | spec.globalPolicies    | [{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":5,"duration":"1h"}]}],"keyExtraction":[{"type":"apiname"}]}}] |
      | spec.operationPolicies | [{"name":"basic-ratelimit","version":"v1","paths":[{"path":"/chat/completions","methods":["GET"],"params":{"limits":[{"requests":3,"duration":"1h"}]}}]}] |
    Then the response status code should be 201

    And I send a "GET" request to "${CTX:proxyContext}/chat/completions" until status 200

    When I send 10 "GET" requests to "${CTX:proxyContext}/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no operation policy but shares the global apiname bucket,
    # already exhausted by /chat/completions traffic
    When I send a "GET" request to "${CTX:proxyContext}/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful

    When I delete the LLM provider "${CTX:backendName}"
    Then the response should be successful
