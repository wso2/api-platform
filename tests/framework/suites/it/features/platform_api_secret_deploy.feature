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

@platform-api-secret-deploy
Feature: Platform-API-driven deployment resolves secret references on demand
  As an API platform operator
  I want a resource created via platform-api and referencing a secret to be deployed to the
  gateway
  So that the gateway controller fetches the secret value on demand when the deployment event
  arrives, confirming that a secret created after gateway startup is resolved correctly at
  deploy time

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "papi-secret-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  Scenario: An LLM provider with a secret-backed API key is deployed and active on the gateway
    Given I generate a unique resource name from "papi-secret-provider-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "papi-secret-provider" and store it as "providerId"
    When I create an LLM provider "${CTX:providerId}" via the control plane referencing template "openai" and secret "${CTX:secretHandle}"
    And I deploy the "LlmProvider" "${CTX:providerId}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:providerId}" until status 200

  Scenario: An LLM proxy with a secret-backed auth override is deployed and active on the gateway
    Given I generate a unique resource name from "papi-secret-proxy-base" and store it as "baseProviderId"
    And I create an LLM provider "${CTX:baseProviderId}" via the control plane referencing template "openai"
    And I deploy the "LlmProvider" "${CTX:baseProviderId}" to the gateway via the control plane
    And I generate a unique resource name from "papi-secret-proxy-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "papi-secret-proxy" and store it as "proxyId"
    When I create an LLM proxy "${CTX:proxyId}" via the control plane in project "${CTX:projectHandle}" referencing provider "${CTX:baseProviderId}" and secret "${CTX:secretHandle}"
    And I deploy the "LlmProxy" "${CTX:proxyId}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyId}" until status 200

  Scenario: An MCP proxy with a secret-backed upstream API key is deployed and active on the gateway
    Given I generate a unique resource name from "papi-secret-mcp-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "papi-secret-mcp" and store it as "mcpId"
    When I create an MCP proxy "${CTX:mcpId}" via the control plane referencing secret "${CTX:secretHandle}"
    And I deploy the "Mcp" "${CTX:mcpId}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/${CTX:mcpId}" until status 200

  Scenario: A REST API with a secret-backed upstream credential is deployed and active on the gateway
    Given I generate a unique resource name from "papi-secret-restapi-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "papi-secret-restapi" and store it as "apiHandle"
    And I generate a unique API context from "/papi-secret-restapi" and store it as "apiContext"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and an upstream auth secret "${CTX:secretHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiHandle}" until status 200

  Scenario: A REST API with a secret-backed policy header value is deployed and active on the gateway
    Given I generate a unique resource name from "papi-policy-secret-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "papi-policy-secret-restapi" and store it as "apiHandle"
    And I generate a unique API context from "/papi-policy-secret-restapi" and store it as "apiContext"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and a policy header secret "${CTX:secretHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiHandle}" until status 200
