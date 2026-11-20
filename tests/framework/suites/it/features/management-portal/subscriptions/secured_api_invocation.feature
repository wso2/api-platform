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

@platform-api-secured-invocation
Feature: Platform-API-issued subscription credentials authorize gateway invocation
  As an API platform operator
  I want an API key and subscription created entirely through platform-api to authorize a
  real request through the gateway data plane
  So that platform-api's own subscription-plan -> application -> subscription -> API-key
  issuance pipeline is proven to produce a genuinely working, enforced credential

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "papi-secured-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @secured
  Scenario: A published, secured API is invocable through the gateway only with valid credentials
    Given I generate a unique resource name from "papi-secured-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "papi-secured-api" and store it as "apiHandle"
    And I generate a unique API context from "/papi-secured" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When I generate a unique resource name from "papi-secured-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I generate a unique value from "papi-secured-key-api-key-0123456789" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane

    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401

  @secured
  Scenario: A weak custom API key is refused by platform-api and cannot invoke the API
    Given I generate a unique resource name from "papi-weak-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "papi-weak-api" and store it as "apiHandle"
    And I generate a unique API context from "/papi-weak" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When I generate a unique resource name from "papi-weak-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/api-keys" with body:
      """
      {"displayName": "Weak key", "apiKey": "aaaa"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    # The strong key reaching 200 shows the API and subscription work, so the 401 below is the
    # weak key's refusal and not a setup fault.
    When I generate a unique value from "papi-strong-key-api-key-0123456789" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I set header "API-Key" to "aaaa"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401

  @secured @known-issue
  Scenario: A subscription plan limit is enforced on a platform-api-issued credential
    Given I generate a unique resource name from "papi-secured-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 3 requests per minute via the control plane
    And I generate a unique resource name from "papi-secured-api" and store it as "apiHandle"
    And I generate a unique API context from "/papi-secured" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When I generate a unique resource name from "papi-secured-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I generate a unique value from "papi-secured-key-api-key-0123456789" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane

    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I send 2 "GET" requests to "${CTX:apiContext}/"
    And I send a "GET" request to "${CTX:apiContext}/" until status 429
