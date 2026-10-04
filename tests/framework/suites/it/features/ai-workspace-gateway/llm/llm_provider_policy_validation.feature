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
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
# either express or implied.  See the License for the specific
# language governing permissions and limitations under the License.
# --------------------------------------------------------------------

@llm-provider-policy-validation
Feature: LLM provider policy validation
  As an API administrator
  I want invalid LLM provider policy references to be rejected
  So that only installed policy versions can be deployed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Create LLM provider referencing a non-existent policy is rejected
    Given I generate a unique resource name from "lpm-bad-policy" and store it as "providerName"
    And I generate a unique value from "lpm-bad-policy" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-bad-policy" and store it as "providerVersion"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:providerName}               |
      | displayName         | ${CTX:providerDisplayName}        |
      | version             | ${CTX:providerVersion}            |
      | template            | openai                              |
      | spec.upstream.url   | http://testbench:3008/openai/v1   |
      | accessControl.mode  | allow_all                          |
      | spec.globalPolicies | [{"name":"this-policy-does-not-exist","version":"v1"}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create LLM provider referencing a non-existent policy version is rejected
    Given I generate a unique resource name from "lpm-bad-version" and store it as "providerName"
    And I generate a unique value from "lpm-bad-version" and store it as "providerDisplayName"
    And I generate a unique API version from "lpm-bad-version" and store it as "providerVersion"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:providerName}               |
      | displayName         | ${CTX:providerDisplayName}        |
      | version             | ${CTX:providerVersion}            |
      | template            | openai                              |
      | spec.upstream.url   | http://testbench:3008/openai/v1   |
      | accessControl.mode  | allow_all                          |
      | spec.globalPolicies | [{"name":"basic-ratelimit","version":"v999"}] |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
