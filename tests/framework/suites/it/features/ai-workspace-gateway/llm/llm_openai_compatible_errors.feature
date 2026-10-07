# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
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

# Runs in the llm-openai-compatible-errors block, whose overlay sets
# [policy_engine.llm_openai_compatible_errors] enabled = true.

@llm @llm-openai-compatible-errors
Feature: OpenAI-compatible error responses for LLM APIs
  As an operator of an AI gateway
  I want every gateway-produced error on an LLM API to use the OpenAI error envelope
  So that clients built on OpenAI SDKs can parse the errors the gateway returns

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # The option is scoped by API kind, not by provider template. A guardrail written for REST APIs
  # writes its own JSON body without describing the failure; on an LLM route that body is
  # reshaped and the guardrail's status is kept.
  Scenario Outline: A policy rejection on a <template> provider is returned in the OpenAI envelope
    Given I generate a unique resource name from "oai-err-guard-<template>" and store it as "providerName"
    And I generate a unique value from "oai-err-guard-<template>-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-err-guard-<template>" and store it as "providerVersion"
    And I generate a unique API context from "/oai-err-guard-<template>" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                 |
      | name               | ${CTX:providerName}                                                                                       |
      | displayName        | ${CTX:providerDisplayName}                                                                                |
      | version            | ${CTX:providerVersion}                                                                                    |
      | template           | <template>                                                                                                |
      | spec.context       | ${CTX:providerContext}                                                                                    |
      | spec.upstream.url  | http://testbench:3000                                                                                     |
      | accessControl.mode | allow_all                                                                                                 |
      | spec.globalPolicies | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""}}}] |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:providerContext}/post" until status 200 with body:
      """
      hello
      """

    When I send a "POST" request to "${CTX:providerContext}/post" with body:
      """
      this request body has far more than five words in it
      """
    Then the response status code should be 422
    And the response header "Content-Type" should contain "application/json"
    And the JSON response field "error.type" should be "invalid_request_error"
    And the JSON response should have field "error.message"
    And the response body should match pattern "\Wparam\W\s*:\s*null"
    And the response body should match pattern "\Wcode\W\s*:"
    And the JSON response field "message" should not exist
    And the JSON response field "type" should not exist

    When I send a "POST" request to "${CTX:providerContext}/post" with body:
      """
      hello there
      """
    Then the response status code should be 200
    And the response body should not contain "invalid_request_error"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

    Examples:
      | template        |
      | openai          |
      | azure-openai    |
      | azureai-foundry |
      | anthropic       |
      | gemini          |
      | mistralai       |
      | awsbedrock      |

  Scenario: A policy rejection on an LLM proxy is returned in the OpenAI envelope
    Given I generate a unique resource name from "oai-err-proxy-backing" and store it as "providerName"
    And I generate a unique value from "oai-err-proxy-backing-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-err-proxy-backing" and store it as "providerVersion"
    And I generate a unique API context from "/oai-err-proxy-backing" and store it as "providerContext"
    And I generate a unique resource name from "oai-err-proxy" and store it as "proxyName"
    And I generate a unique value from "oai-err-proxy-display" and store it as "proxyDisplayName"
    And I generate a unique API version from "oai-err-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/oai-err-proxy" and store it as "proxyContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerDisplayName} |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3000      |
      | accessControl.mode | allow_all                  |
    Then the response status code should be 201
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                             |
      | name                | ${CTX:proxyName}                                                                                      |
      | displayName         | ${CTX:proxyDisplayName}                                                                               |
      | version             | ${CTX:proxyVersion}                                                                                   |
      | context             | ${CTX:proxyContext}                                                                                   |
      | provider.id         | ${CTX:providerName}                                                                                   |
      | spec.globalPolicies | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""}}}] |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:proxyContext}/post" until status 200 with body:
      """
      hello
      """

    When I send a "POST" request to "${CTX:proxyContext}/post" with body:
      """
      this request body has far more than five words in it
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "error.type" should be "invalid_request_error"
    And the JSON response should have field "error.message"
    And the response body should match pattern "\Wparam\W\s*:\s*null"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  # The upstream refuses connections, so the router produces the 503 itself. The route's only
  # policy never reads the response body, so this also covers buffering the body of an error
  # response alone: the router's plain-text reply must still be replaced.
  Scenario Outline: A router failure on a <template> provider is returned in the OpenAI envelope
    Given I generate a unique resource name from "oai-err-down-<template>" and store it as "providerName"
    And I generate a unique value from "oai-err-down-<template>-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-err-down-<template>" and store it as "providerVersion"
    And I generate a unique API context from "/oai-err-down-<template>" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                                           |
      | name                | ${CTX:providerName}                                                                                                 |
      | displayName         | ${CTX:providerDisplayName}                                                                                          |
      | version             | ${CTX:providerVersion}                                                                                              |
      | template            | <template>                                                                                                          |
      | spec.context        | ${CTX:providerContext}                                                                                              |
      | spec.upstream.url   | http://testbench:1                                                                                                  |
      | accessControl.mode  | allow_all                                                                                                           |
      | spec.globalPolicies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-oai-err-test","value":"down"}]}}}] |
    Then the response status code should be 201

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 503 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}
      """
    Then the response status code should be 503
    And the response header "Content-Type" should contain "application/json"
    And the response should be valid JSON
    And the JSON response field "error.type" should be "server_error"
    And the JSON response field "error.code" should be "101503"
    And the JSON response should have field "error.message"
    And the response body should match pattern "\Wparam\W\s*:\s*null"
    And the response body should not contain "upstream connect error"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

    Examples:
      | template        |
      | openai          |
      | azure-openai    |
      | azureai-foundry |
      | anthropic       |
      | gemini          |
      | mistralai       |
      | awsbedrock      |

  # A provider owns its complete error response, including the decision to send no body. Neither
  # form is replaced by the gateway's formatter.
  Scenario Outline: A <template> backend error is preserved with or without a body
    Given I generate a unique resource name from "oai-err-backend-<template>" and store it as "providerName"
    And I generate a unique value from "oai-err-backend-<template>-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-err-backend-<template>" and store it as "providerVersion"
    And I generate a unique API context from "/oai-err-backend-<template>" and store it as "providerContext"
    And I generate a unique resource name from "oai-err-empty-<template>" and store it as "emptyProviderName"
    And I generate a unique value from "oai-err-empty-<template>-display" and store it as "emptyProviderDisplayName"
    And I generate a unique API version from "oai-err-empty-<template>" and store it as "emptyProviderVersion"
    And I generate a unique API context from "/oai-err-empty-<template>" and store it as "emptyProviderContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                                              |
      | name                | ${CTX:providerName}                                                                                                    |
      | displayName         | ${CTX:providerDisplayName}                                                                                             |
      | version             | ${CTX:providerVersion}                                                                                                 |
      | template            | <template>                                                                                                             |
      | spec.context        | ${CTX:providerContext}                                                                                                 |
      | spec.upstream.url   | http://testbench:3000                                                                                                  |
      | accessControl.mode  | allow_all                                                                                                              |
      | spec.globalPolicies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-oai-err-test","value":"backend"}]}}}] |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                                            |
      | name                | ${CTX:emptyProviderName}                                                                                             |
      | displayName         | ${CTX:emptyProviderDisplayName}                                                                                      |
      | version             | ${CTX:emptyProviderVersion}                                                                                          |
      | template            | <template>                                                                                                           |
      | spec.context        | ${CTX:emptyProviderContext}                                                                                          |
      | spec.upstream.url   | http://testbench:3002                                                                                                |
      | accessControl.mode  | allow_all                                                                                                            |
      | spec.globalPolicies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-oai-err-test","value":"empty"}]}}}] |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/get" until status 200
    And I send a "GET" request to "${CTX:emptyProviderContext}/get" until status 200

    When I send a "GET" request to "${CTX:providerContext}/get?statusCode=418"
    Then the response status code should be 418
    And the JSON response field "method" should be "GET"
    And the JSON response field "error" should not exist
    And the response body should not contain "invalid_request_error"

    When I send a "GET" request to "${CTX:emptyProviderContext}/status/429"
    Then the response status code should be 429
    And the response body should be empty

    When I send a "GET" request to "${CTX:emptyProviderContext}/status/500"
    Then the response status code should be 500
    And the response body should be empty

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:emptyProviderName}"
    Then the response should be successful

    Examples:
      | template        |
      | openai          |
      | azure-openai    |
      | azureai-foundry |
      | anthropic       |
      | gemini          |
      | mistralai       |
      | awsbedrock      |

  # The option is scoped to LLM kinds: the same guardrail on a REST API keeps its own body.
  Scenario: A REST API rejection is not reshaped
    Given I generate a unique value from "oai-err-rest" and store it as "apiName"
    And I generate a unique API version from "oai-err-rest" and store it as "apiVersion"
    And I generate a unique API context from "/oai-err-rest" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}            |
      | spec.displayName       | ${CTX:apiName}            |
      | spec.version           | ${CTX:apiVersion}         |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3000     |
      | spec.operations        | [{"method":"GET","path":"/get"},{"method":"POST","path":"/validate","policies":[{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/get" until status 200

    When I send a "POST" request to "${CTX:apiContext}/${CTX:apiVersion}/validate" with body:
      """
      this request body has far more than five words in it
      """
    Then the response status code should be 422
    And the response body should not contain "invalid_request_error"
    And the JSON response field "error.type" should not exist

    When I delete the API "${CTX:apiName}"
    Then the response should be successful
