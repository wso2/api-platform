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

@template-functions
Feature: Typed template values in policy parameters
  As an API administrator
  I want environment templates in typed policy parameters to be converted to
  their declared policy types before the gateway enforces them.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @gateway-v1.2
  Scenario: env template in integer policy param is coerced and enforced at runtime
    Given I generate a unique resource name from "tpl-env-ratelimit-api" and store it as "apiName"
    And I generate a unique API version from "tpl-env-ratelimit-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-ratelimit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Env-Ratelimit-Api               |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.operations        | [{"method":"GET","path":"/probe","policies":[{"name":"advanced-ratelimit","version":"v1","params":{"quotas":[{"name":"request-limit","limits":[{"limit":"{{ env \"IT_RATE_LIMIT\" }}","duration":"1h"}]}]}}]}] |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_RATE_LIMIT" }}
      """
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ env "IT_RATE_LIMIT" }}
      """

    # The readiness probe uses ~1 request; send 4 more to reach the limit of 5.
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I send 4 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200

    # One more request must be rejected — limit exhausted.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  @gateway-v1.2
  Scenario: env template in boolean policy param is coerced and applied at runtime
    Given I generate a unique resource name from "tpl-env-cors-api" and store it as "apiName"
    And I generate a unique API version from "tpl-env-cors-api" and store it as "apiVersion"
    And I generate a unique API context from "/tpl-env-cors" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                     |
      | spec.displayName       | Tpl-Env-Cors-Api                    |
      | spec.version           | ${CTX:apiVersion}                  |
      | spec.context           | ${CTX:apiContext}/$version         |
      | spec.upstream.main.url | http://testbench:3000               |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET"],"allowCredentials":"{{ env \"IT_ALLOW_CREDENTIALS\" }}"}}] |
      | spec.operations        | [{"method":"GET","path":"/probe"}]  |
    Then the response status code should be 201
    And the response body should contain template literal:
      """
      {{ env "IT_ALLOW_CREDENTIALS" }}
      """
    And the stored RestApi configuration for "${CTX:apiName}" should contain:
      """
      {{ env "IT_ALLOW_CREDENTIALS" }}
      """

    # Runtime: allowCredentials=true must produce the credentials response header.
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe" until status 200
    When I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/probe"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Credentials" should be "true"
