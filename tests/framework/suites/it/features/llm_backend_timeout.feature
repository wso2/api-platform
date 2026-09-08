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

@llm @llm-backend-timeout @resilience
Feature: LLM backend route timeouts
  As an API developer
  I want LLM provider and proxy timeout settings to control slow upstream requests
  So that the gateway applies the configured timeout contract

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: LLM provider timeout terminates a slow backend
    Given I generate a unique resource name from "llm-resilience-timeout-provider" and store it as "providerName"
    And I generate a unique value from "llm-resilience-timeout-display" and store it as "providerDisplayName"
    And I generate a unique API version from "llm-resilience-timeout" and store it as "providerVersion"
    And I generate a unique API context from "/llm-resilience-timeout" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:providerName}              |
      | displayName             | ${CTX:providerDisplayName}       |
      | version                 | ${CTX:providerVersion}           |
      | template                | openai                            |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3000            |
      | accessControl.mode      | allow_all                         |
      | spec.resilience.timeout | 2s                                |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/get" until status 200
    When I send a "GET" request to "${CTX:providerContext}/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: LLM provider timeout applies to deny-list exception routes
    Given I generate a unique resource name from "llm-deny-resilience-timeout-provider" and store it as "providerName"
    And I generate a unique value from "llm-deny-resilience-timeout-display" and store it as "providerDisplayName"
    And I generate a unique API version from "llm-deny-resilience-timeout" and store it as "providerVersion"
    And I generate a unique API context from "/llm-deny-resilience-timeout" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1                                      |
      | name                          | ${CTX:providerName}                                                   |
      | displayName                   | ${CTX:providerDisplayName}                                            |
      | version                       | ${CTX:providerVersion}                                                |
      | template                      | openai                                                                 |
      | spec.context                   | ${CTX:providerContext}                                                 |
      | spec.upstream.url              | http://testbench:3000                                                 |
      | accessControl.mode            | deny_all                                                                |
      | spec.accessControl.exceptions | [{"path":"/get","methods":["GET"]},{"path":"/delay/5","methods":["GET"]}] |
      | spec.resilience.timeout       | 2s                                                                     |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/get" until status 200
    When I send a "GET" request to "${CTX:providerContext}/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: LLM provider without resilience uses the gateway default timeout
    Given I generate a unique resource name from "llm-default-timeout-provider" and store it as "providerName"
    And I generate a unique value from "llm-default-timeout-display" and store it as "providerDisplayName"
    And I generate a unique API version from "llm-default-timeout" and store it as "providerVersion"
    And I generate a unique API context from "/llm-default-timeout" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}              |
      | displayName        | ${CTX:providerDisplayName}       |
      | version            | ${CTX:providerVersion}           |
      | template           | openai                            |
      | spec.context        | ${CTX:providerContext}            |
      | spec.upstream.url | http://testbench:3000            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/get" until status 200
    When I send a "GET" request to "${CTX:providerContext}/delay/2"
    Then the response status code should be 200
    And the response should be valid JSON
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: LLM proxy timeout terminates a slow backend
    Given I generate a unique resource name from "llm-proxy-timeout-provider" and store it as "providerName"
    And I generate a unique value from "llm-proxy-timeout-provider-display" and store it as "providerDisplayName"
    And I generate a unique API version from "llm-proxy-timeout-provider" and store it as "providerVersion"
    And I generate a unique API context from "/llm-proxy-timeout-provider" and store it as "providerContext"
    And I generate a unique resource name from "llm-proxy-timeout" and store it as "proxyName"
    And I generate a unique value from "llm-proxy-timeout-display" and store it as "proxyDisplayName"
    And I generate a unique API version from "llm-proxy-timeout" and store it as "proxyVersion"
    And I generate a unique API context from "/llm-proxy-timeout" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:providerName}              |
      | displayName        | ${CTX:providerDisplayName}       |
      | version            | ${CTX:providerVersion}           |
      | template           | openai                            |
      | spec.context        | ${CTX:providerContext}            |
      | spec.upstream.url | http://testbench:3000            |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:proxyName}                 |
      | displayName                | ${CTX:proxyDisplayName}          |
      | version                    | ${CTX:proxyVersion}              |
      | context                    | ${CTX:proxyContext}              |
      | provider.id                | ${CTX:providerName}              |
      | spec.resilience.timeout    | 2s                               |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:proxyContext}/get" until status 200
    When I send a "GET" request to "${CTX:proxyContext}/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: Provider timeout takes precedence over a longer proxy timeout
    Given I generate a unique resource name from "llm-both-timeout-provider" and store it as "providerName"
    And I generate a unique value from "llm-both-timeout-provider-display" and store it as "providerDisplayName"
    And I generate a unique API version from "llm-both-timeout-provider" and store it as "providerVersion"
    And I generate a unique API context from "/llm-both-timeout-provider" and store it as "providerContext"
    And I generate a unique resource name from "llm-both-timeout-proxy" and store it as "proxyName"
    And I generate a unique value from "llm-both-timeout-proxy-display" and store it as "proxyDisplayName"
    And I generate a unique API version from "llm-both-timeout-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/llm-both-timeout-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:providerName}              |
      | displayName             | ${CTX:providerDisplayName}       |
      | version                 | ${CTX:providerVersion}           |
      | template                | openai                            |
      | spec.context             | ${CTX:providerContext}            |
      | spec.upstream.url        | http://testbench:3000            |
      | accessControl.mode      | allow_all                         |
      | spec.resilience.timeout | 2s                                |
    Then the response status code should be 201
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1 |
      | name                       | ${CTX:proxyName}                 |
      | displayName                | ${CTX:proxyDisplayName}          |
      | version                    | ${CTX:proxyVersion}              |
      | context                    | ${CTX:proxyContext}              |
      | provider.id                | ${CTX:providerName}              |
      | spec.resilience.timeout    | 6s                               |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:proxyContext}/get" until status 200
    When I send a "GET" request to "${CTX:proxyContext}/delay/10"
    Then the gateway should have timed out after "2" seconds with status 504
    And the gateway should have responded within "4" seconds
    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
