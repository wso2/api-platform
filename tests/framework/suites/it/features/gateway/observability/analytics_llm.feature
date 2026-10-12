# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
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

@analytics-llm
Feature: Analytics LLM proxy event capture
  As a platform administrator
  I want LLM proxy analytics to avoid duplicate events
  So that one client request produces one analytics event

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  # An LLM proxy forwards to its provider over the gateway's internal loopback, so one client
  # call traverses the listener twice and the access-log service delivers two entries. Only the
  # proxy's own event may be published: it carries the user identity, and the loopback provider
  # hop is its anonymous duplicate. Asserts an EXACT count - "at least 1" would pass while
  # double-counting, which is the bug this guards.
  Scenario: An LLM proxy invocation generates exactly one analytics event
    Given I generate a unique resource name from "analytics-dedup-provider" and store it as "providerName"
    And I generate a unique value from "analytics-dedup-provider" and store it as "providerDisplayName"
    And I generate a unique API version from "analytics-dedup" and store it as "providerVersion"
    And I generate a unique API context from "/analytics-dedup-provider" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}         |
      | name               | ${CTX:providerName}               |
      | displayName        | ${CTX:providerDisplayName}        |
      | version            | ${CTX:providerVersion}             |
      | template           | openai                             |
      | spec.context       | ${CTX:providerContext}             |
      | spec.upstream.url  | http://testbench:3008/anything    |
      | accessControl.mode | allow_all                          |
    Then the response should be successful

    Given I generate a unique resource name from "analytics-dedup-proxy" and store it as "proxyName"
    And I generate a unique value from "analytics-dedup-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "analytics-dedup-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/analytics-dedup-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion   | ${CTX:gatewaySpecVersion} |
      | name         | ${CTX:proxyName}          |
      | displayName  | ${CTX:proxyDisplayName}  |
      | version      | ${CTX:proxyVersion}      |
      | context      | ${CTX:proxyContext}      |
      | provider.id  | ${CTX:providerName}      |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}
      """
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have metadata field "apiType" with value "LlmProxy"
    And I wait for the analytics collector to settle

    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    # The surviving event must be the proxy's own, identified two ways: its request URI is the
    # proxy context (not the provider's loopback context) and its api kind is LlmProxy.
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have metadata field "apiType" with value "LlmProxy"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
