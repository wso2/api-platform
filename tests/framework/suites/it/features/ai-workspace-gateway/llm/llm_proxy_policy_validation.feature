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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@llm-proxy-policy-validation
Feature: LLM proxy policy validation
  As an API administrator
  I want invalid LLM proxy policy references to be rejected
  So that only installed policy versions can be deployed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Create LLM proxy referencing a non-existent policy version is rejected
    Given I generate a unique resource name from "lpx-badver-template" and store it as "templateName"
    And I generate a unique resource name from "lpx-badver-provider" and store it as "providerName"
    And I generate a unique value from "lpx-badver-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "lpx-badver-provider" and store it as "providerVersion"
    And I generate a unique API context from "/lpx-badver-provider" and store it as "providerContext"
    And I generate a unique resource name from "lpx-badver-proxy" and store it as "proxyName"
    And I generate a unique value from "lpx-badver-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "lpx-badver-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/lpx-badver-proxy" and store it as "proxyContext"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:templateName}       |
      | displayName | ${CTX:templateName}       |
    Then the response status code should be 201

    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:providerName}       |
      | displayName        | ${CTX:providerDisplayName} |
      | version            | ${CTX:providerVersion}    |
      | template           | ${CTX:templateName}       |
      | spec.context       | ${CTX:providerContext}    |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                 |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:proxyName}          |
      | displayName         | ${CTX:proxyDisplayName}  |
      | version             | ${CTX:proxyVersion}      |
      | context             | ${CTX:proxyContext}      |
      | provider.id         | ${CTX:providerName}       |
      | spec.globalPolicies | [{"name":"basic-ratelimit","version":"v999"}] |
    Then the response should be a client error
    And the response should be valid JSON

    When I delete the LLM provider "${CTX:providerName}"
    Then the response status code should be 200
    And I send a "GET" request to "${CTX:providerContext}/chat/completions" until status 404
    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200
