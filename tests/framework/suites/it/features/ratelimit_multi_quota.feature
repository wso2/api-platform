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

@ratelimit-multi-quota
Feature: Advanced rate limit multi-quota policies and state persistence
  As an API developer
  I want a single policy to track several independent quotas or limits at once
  So that I can enforce per-route and per-API budgets together, surviving unrelated config updates

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A per-route quota and a per-api quota are enforced together
    Given I generate a unique value from "arl-multi-dim" and store it as "apiName"
    And I generate a unique API version from "arl-multi-dim" and store it as "apiVersion"
    And I generate a unique API context from "/arl-multi-dim" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/multi1","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"per-route","limits":[{"limit":5,"duration":"1h"}],"keyExtraction":[{"type":"routename"}]},{"name":"per-api","limits":[{"limit":8,"duration":"1h"}],"keyExtraction":[{"type":"apiname"}]}]}}]},{"method":"GET","path":"/multi2","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"per-route","limits":[{"limit":5,"duration":"1h"}],"keyExtraction":[{"type":"routename"}]},{"name":"per-api","limits":[{"limit":8,"duration":"1h"}],"keyExtraction":[{"type":"apiname"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/multi1"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi1"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/multi2"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/multi2"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: The most restrictive limit within a quota wins, before and after an update
    Given I generate a unique value from "arl-multi-limit" and store it as "apiName"
    And I generate a unique API version from "arl-multi-limit" and store it as "apiVersion"
    And I generate a unique API context from "/arl-multi-limit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-quota","limits":[{"limit":10,"duration":"1h"},{"limit":8,"duration":"24h"}],"keyExtraction":[{"type":"routename"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 8 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-quota","limits":[{"limit":12,"duration":"1h"},{"limit":10000,"duration":"24h"}],"keyExtraction":[{"type":"routename"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    # The new limits replace the quota's state entirely, so the confirmation poll below observes
    # a fresh bucket. That poll is itself a request against the new 12-request window.
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" until header "X-RateLimit-Limit" is "12"

    When I send 11 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Updating an API for an unrelated change does not reset rate limit state
    Given I generate a unique value from "arl-update-preserves" and store it as "apiName"
    And I generate a unique API version from "arl-update-preserves" and store it as "apiVersion"
    And I generate a unique API context from "/arl-update-preserves" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":5,"duration":"1h"}]}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/resource","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":5,"duration":"1h"}]}]}}]},{"method":"PUT","path":"/handle"}] |
    Then the resource creation response should indicate successful deployment
    # /resource's own policy is unchanged by this update, so its response header would show the
    # same limit before and after — poll the newly added sibling operation instead, which is only
    # reachable once the whole updated configuration (including /resource's untouched quota) is
    # live, per the same atomic per-API delivery guarantee every readiness check in this suite
    # relies on.
    And I send a "PUT" request to "${CTX:apiContext}/${CTX:apiVersion}/handle" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
