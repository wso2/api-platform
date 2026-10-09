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

@analytics-header-filter
Feature: Analytics header filter policy configuration validation
  As an API developer
  I want invalid analytics header filter configurations to be rejected
  So that a misconfigured filter is never deployed

  # Rejection of invalid filter configurations and unsupported policy versions.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario Outline: An invalid policy configuration whose <filter> filter is missing the mode field is rejected
    Given I generate a unique value from "ahf-missing-mode-<filter>" and store it as "apiName"
    And I generate a unique API version from "ahf-missing-mode-<filter>" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-missing-mode-<filter>" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"<filter>":{"headers":["authorization"]}}}]}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "mode is required"

    Examples:
      | filter   |
      | request  |
      | response |

  Scenario: An invalid policy configuration with an invalid mode value is rejected
    Given I generate a unique value from "ahf-invalid-op" and store it as "apiName"
    And I generate a unique API version from "ahf-invalid-op" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-invalid-op" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"invalid","headers":["authorization"]}}}]}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"

  Scenario Outline: An invalid policy configuration with <case> is rejected
    Given I generate a unique value from "ahf-invalid-headers" and store it as "apiName"
    And I generate a unique API version from "ahf-invalid-headers" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-invalid-headers" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"v1","params":{"request":{"mode":"deny","headers":<headers>}}}]}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "<message>"

    Examples:
      | case                                       | headers | message |
      | an empty header name                       | [""] | String length must be greater than or equal to 1 |
      | a header name longer than 256 characters   | ["xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"] | String length must be less than or equal to 256 |
      | a duplicated header name                   | ["authorization","authorization"] | must be unique |

  # Policy references must be major-only (vN) and resolve to a major version built into the
  # gateway. v0 is a released but unbundled major version, v999 has never been released, and a
  # full semantic version is rejected even when it names the bundled release.
  Scenario Outline: A policy reference with <case> is rejected
    Given I generate a unique value from "ahf-bad-version" and store it as "apiName"
    And I generate a unique API version from "ahf-bad-version" and store it as "apiVersion"
    And I generate a unique API context from "/ahf-bad-version" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/test","policies":[{"name":"analytics-header-filter","version":"<version>","params":{"request":{"mode":"deny","headers":["authorization"]}}}]}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "<message>"

    Examples:
      | case                                      | version | message |
      | a major version not bundled in the gateway | v0      | major version 'v0' not found in loaded policy definitions |
      | a major version that does not exist        | v999    | major version 'v999' not found in loaded policy definitions |
      | a full semantic version                    | v1.0.1  | version must be major-only |
