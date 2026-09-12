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

@basic-ratelimit
Feature: Basic rate limiting policy
  As an API developer
  I want a simple rate limiting policy
  So that I can protect my APIs without complex configuration

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce basic rate limit on an operation
    Given I generate a unique value from "brl-basic" and store it as "apiName"
    And I generate a unique API version from "brl-basic" and store it as "apiVersion"
    And I generate a unique API context from "/brl-basic" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/limited","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":5,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Rate limit headers are returned on a successful response
    Given I generate a unique value from "brl-headers" and store it as "apiName"
    And I generate a unique API version from "brl-headers" and store it as "apiVersion"
    And I generate a unique API context from "/brl-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/check","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":100,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/check"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Multiple limits enforce the most restrictive one
    Given I generate a unique value from "brl-multi-limit" and store it as "apiName"
    And I generate a unique API version from "brl-multi-limit" and store it as "apiVersion"
    And I generate a unique API context from "/brl-multi-limit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/resource","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":10,"duration":"1h"},{"requests":5,"duration":"24h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Each route enforces its own quota
    Given I generate a unique value from "brl-per-route" and store it as "apiName"
    And I generate a unique API version from "brl-per-route" and store it as "apiVersion"
    And I generate a unique API context from "/brl-per-route" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/route1","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]},{"method":"GET","path":"/route2","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/route1"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/route1"
    Then the response status code should be 429

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/route2"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/route2"
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A 429 response includes a Retry-After header
    Given I generate a unique value from "brl-retry-after" and store it as "apiName"
    And I generate a unique API version from "brl-retry-after" and store it as "apiVersion"
    And I generate a unique API context from "/brl-retry-after" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/resource","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429
    And the response header "Retry-After" should exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A route without its own policy is not throttled when no API-level policy exists
    Given I generate a unique value from "brl-route-isolation" and store it as "apiName"
    And I generate a unique API version from "brl-route-isolation" and store it as "apiVersion"
    And I generate a unique API context from "/brl-route-isolation" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"GET","path":"/limited","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]},{"method":"GET","path":"/open"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/limited"
    Then the response status code should be 429

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/open"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should not exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An API-level policy scopes a shared bucket across sibling operations
    Given I generate a unique value from "brl-scope" and store it as "apiName"
    And I generate a unique API version from "brl-scope" and store it as "apiVersion"
    And I generate a unique API context from "/brl-scope" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":5,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":100,"duration":"1h"}]}}]},{"method":"GET","path":"/resource-a"},{"method":"GET","path":"/resource-b","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]},{"method":"GET","path":"/resource-c"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 429

    # The API-level bucket (5) is already exhausted by the readiness poll plus resource-b's
    # traffic (every request against an operation under this API counts toward the API-level
    # bucket, whether or not that operation also has its own route-level policy).
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource-a"
    Then the response status code should be 429

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource-c"
    Then the response status code should be 429

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 429

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An API-level quota is shared across operations without route-level policies
    Given I generate a unique value from "brl-api-shared" and store it as "apiName"
    And I generate a unique API version from "brl-api-shared" and store it as "apiVersion"
    And I generate a unique API context from "/brl-api-shared" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":20,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/route-a"},{"method":"GET","path":"/route-b"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 6 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/route-a"
    Then the response status code should be 200

    When I send 6 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/route-b"
    Then the response status code should be 200

    When I send 12 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/route-a"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A lower API-level limit still blocks a route with a higher route-level limit
    Given I generate a unique value from "brl-additive" and store it as "apiName"
    And I generate a unique API version from "brl-additive" and store it as "apiVersion"
    And I generate a unique API context from "/brl-additive" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":10,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":100,"duration":"1h"}]}}]},{"method":"GET","path":"/resource-b","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":100,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 8 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "10"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Mixed attachment returns the scope-correct limit header on a 429
    Given I generate a unique value from "brl-mixed-headers" and store it as "apiName"
    And I generate a unique API version from "brl-mixed-headers" and store it as "apiVersion"
    And I generate a unique API context from "/brl-mixed-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":5,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":100,"duration":"1h"}]}}]},{"method":"GET","path":"/resource-a"},{"method":"GET","path":"/resource-b","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource-b"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "3"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource-a"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "5"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Updating an API adds then removes a route-level policy for the same route
    Given I generate a unique value from "brl-update-route" and store it as "apiName"
    And I generate a unique API version from "brl-update-route" and store it as "apiVersion"
    And I generate a unique API context from "/brl-update-route" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":50,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":50,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":2,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    # The confirmation poll above is itself a request against the new 2-request bucket, so the
    # bucket is already exhausted by the time the poll condition is satisfied.
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" until header "X-RateLimit-Limit" is "2"

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 429

    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":50,"duration":"1h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/resource"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/resource" until header "X-RateLimit-Limit" is "50"

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/resource"
    Then the response status code should be 200

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An API-level quota is consumed across routes when one route also has a route-level policy
    Given I generate a unique value from "brl-reading-list" and store it as "apiName"
    And I generate a unique API version from "brl-reading-list" and store it as "apiVersion"
    And I generate a unique API context from "/brl-reading-list" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":15,"duration":"24h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":100,"duration":"24h"}]}}]},{"method":"GET","path":"/books","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"24h"}]}}]},{"method":"GET","path":"/authors"},{"method":"GET","path":"/categories"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I send 3 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/books"
    Then the response status code should be 200

    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/books"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "3"
    And the response header "X-RateLimit-Remaining" should be "0"

    When I send 8 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/authors"
    Then the response status code should be 200

    When I send 8 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/categories"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "15"
    And the response header "X-RateLimit-Remaining" should be "0"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Route-level traffic on one operation also consumes the API-level bucket used by a sibling operation
    Given I generate a unique value from "brl-books-siblings" and store it as "apiName"
    And I generate a unique API version from "brl-books-siblings" and store it as "apiVersion"
    And I generate a unique API context from "/brl-books-siblings" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                   |
      | spec.displayName       | ${CTX:apiName}                   |
      | spec.version           | ${CTX:apiVersion}                |
      | spec.context           | ${CTX:apiContext}/$version       |
      | spec.upstream.main.url | http://testbench:3000            |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":15,"duration":"24h"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/books","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":50,"duration":"24h"}]}}]},{"method":"POST","path":"/books"},{"method":"GET","path":"/books/{id}"},{"method":"PUT","path":"/books/{id}"},{"method":"DELETE","path":"/books/{id}"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/books" until status 200

    When I send 13 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/books"
    Then the response status code should be 200

    When I send 2 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/books/1d4c9647-5e62-4f1d-9c30-e1f25c6d0e73"
    Then the response status code should be 429
    And the response header "X-RateLimit-Limit" should be "15"
    And the response header "X-RateLimit-Remaining" should be "0"

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
