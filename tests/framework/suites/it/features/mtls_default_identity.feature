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

@mtls @mtls-default-identity
Feature: Presenting a default client certificate to backends
  As a platform administrator
  I want the gateway to present one certificate to every HTTPS backend whose definition names no identity
  So that a fleet of backends requiring mutual TLS needs no identity named in each API

  With router.upstream.tls.present_default_identity on, an HTTPS backend whose upstream
  definition names no tls.identity is presented the gateway identity with role default, else the
  HTTPS listener certificate. A tls.identity always wins. Only one identity holds role default,
  and uploading, rotating or deleting it takes effect without a redeploy. The scenarios with the
  switch off are in mtls_default_identity_off.feature.

  The optional backend answers every request and reports the subject of whatever client
  certificate it was presented, or an empty subject. The required backend trusts the issuer of
  partner A and answers 400 to a request without a certificate it trusts.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I store the URL of the "optional" TLS backend as "optionalBackend"
    And I store the URL of the "required" TLS backend as "requiredBackend"
    And I generate a unique value from "default-identity" and store it as "apiName"
    And I generate a unique API version from "default-identity" and store it as "apiVersion"
    And I generate a unique API context from "/default-identity" and store it as "apiContext"
    And I generate a unique resource name from "backend-trust" and store it as "backendTrust"
    And I generate a unique resource name from "default-identity-cert" and store it as "defaultIdentity"
    And I generate a unique resource name from "other-identity-cert" and store it as "otherIdentity"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendTrust}" with usage "upstream"

  # ==================== SWITCH ON: WHICH CERTIFICATE ====================

  Scenario: Without a default identity the backend sees the HTTPS listener certificate
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the gateway listener certificate
    And the response header "X-Client-Verified" should be "false"

  Scenario: Uploading a default identity changes what the backend sees without a redeploy
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the gateway listener certificate
    And the response header "X-Client-Verified" should be "false"
    When the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    Then I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a" while tolerating the gateway listener certificate
    And the response header "X-Client-Verified" should be "true"

  Scenario: Deleting the default identity falls back to the HTTPS listener certificate
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"
    When I remove the gateway identity "${CTX:defaultIdentity}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the gateway listener certificate while tolerating the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "false"

  Scenario: Rotating the default identity changes what the backend sees and keeps its role
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"
    When I rotate the gateway identity "${CTX:defaultIdentity}" from fixture "gw-identity-b"
    Then the response status code should be 200
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-b" while tolerating the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "false"
    And the gateway identity listing should show role default only on "${CTX:defaultIdentity}"

  Scenario: Deleting the default identity and uploading another default presents the new one without a redeploy
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a"
    When I remove the gateway identity "${CTX:defaultIdentity}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the gateway listener certificate while tolerating the client certificate of "gw-identity-a"
    When the gateway identity "${CTX:otherIdentity}" is uploaded from fixture "gw-identity-b" with role default
    Then I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-b" while tolerating the gateway listener certificate
    And the response header "X-Client-Verified" should be "false"
    And the gateway identity listing should show role default only on "${CTX:otherIdentity}"

  Scenario: A required backend accepts the default identity when it trusts the identity's issuer
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:requiredBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a"
    And the response status code should be 200

  Scenario: A backend that does not trust the default identity's issuer answers when it does not require a certificate and refuses when it does
    Given I generate a unique value from "default-identity-required" and store it as "requiredApiName"
    And I generate a unique API context from "/default-identity-required" and store it as "requiredContext"
    And the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "corp-other-service" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "corp-other-service"
    And the response header "X-Client-Verified" should be "false"
    And the response status code should be 200
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:requiredApiName}     |
      | spec.displayName        | ${CTX:requiredApiName}     |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:requiredContext}/$version |
      | spec.upstream.main.url  | ${CTX:requiredBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:requiredContext}/${CTX:apiVersion}/anything" until the backend sees a refusal
    And the response status code should be 400
    And the JSON response field "backend" should be "a"
    And the JSON response field "error" should be "the SSL certificate error: x509: certificate signed by unknown authority"
    And the JSON response field "client" should not exist

  # ==================== SWITCH ON: WHICH DEFINITIONS ====================

  Scenario: A definition naming an identity presents it and not the default
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    And the gateway identity "${CTX:otherIdentity}" is uploaded from fixture "gw-identity-b"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                | ${CTX:gatewaySpecVersion}  |
      | name                      | ${CTX:apiName}             |
      | spec.displayName          | ${CTX:apiName}             |
      | spec.version              | ${CTX:apiVersion}          |
      | spec.context              | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions  | [{"name":"partner","upstreams":[{"url":"${CTX:optionalBackend}"}],"tls":{"identity":"${CTX:otherIdentity}"}}] |
      | spec.upstream.main.ref    | partner                    |
      | spec.operations           | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-b"
    And the response header "X-Client-Verified" should be "false"

  Scenario Outline: A tls block that names no identity presents the default
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                | ${CTX:gatewaySpecVersion}  |
      | name                      | ${CTX:apiName}             |
      | spec.displayName          | ${CTX:apiName}             |
      | spec.version              | ${CTX:apiVersion}          |
      | spec.context              | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions  | [{"name":"partner","upstreams":[{"url":"${CTX:optionalBackend}"}],"tls":<tls>}] |
      | spec.upstream.main.ref    | partner                    |
      | spec.operations           | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"

    Examples:
      | tls                                           |
      | {"trustedCAs":["${CTX:backendTrust}"]}        |
      | {"verifyHostName":true}                       |

  Scenario: An inline sandbox upstream presents the default identity
    Given I generate a unique resource name from "default-identity-sandbox" and store it as "sandboxLabel"
    And the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                | ${CTX:gatewaySpecVersion}  |
      | name                      | ${CTX:apiName}             |
      | spec.displayName          | ${CTX:apiName}             |
      | spec.version              | ${CTX:apiVersion}          |
      | spec.context              | ${CTX:apiContext}/$version |
      | spec.vhosts.sandbox       | ${CTX:sandboxLabel}.example |
      | spec.upstream.main.url    | http://testbench:3002      |
      | spec.upstream.sandbox.url | ${CTX:optionalBackend}     |
      | spec.operations           | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "${CTX:sandboxLabel}.example"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"

  Scenario: An LLM provider upstream presents the default identity
    Given I generate a unique resource name from "default-identity-provider" and store it as "providerName"
    And I generate a unique API version from "default-identity-provider" and store it as "providerVersion"
    And I generate a unique API context from "/default-identity-provider" and store it as "providerContext"
    And the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | ${CTX:optionalBackend}     |
      | accessControl.mode | allow_all                  |
    Then the response status code should be 201
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"

  @agent
  Scenario: An Agent upstream presents the default identity
    Given I generate a unique resource name from "default-identity-agent" and store it as "agentName"
    And I generate a unique API context from "/default-identity-agent" and store it as "agentContext"
    And the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}  |
      | name              | ${CTX:agentName}           |
      | spec.displayName  | ${CTX:agentName}           |
      | spec.version      | v1.0                       |
      | spec.context      | ${CTX:agentContext}        |
      | spec.upstream.url | ${CTX:optionalBackend}     |
      | spec.a2a          | {"protocolVersion":"1.0","operationConfigs":{"transports":[{"protocolBinding":"JSONRPC"}]},"agentCard":{"public":{"mode":"passthrough","rewriteUrls":false}}} |
    Then the response should be successful
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"

  Scenario: An MCP proxy upstream presents the default identity
    Given I generate a unique resource name from "default-identity-mcp" and store it as "mcpName"
    And I generate a unique API version from "default-identity-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/default-identity-mcp" and store it as "mcpContext"
    And the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}  |
      | name              | ${CTX:mcpName}             |
      | displayName       | ${CTX:mcpName}             |
      | version           | ${CTX:mcpVersion}          |
      | context           | ${CTX:mcpContext}          |
      | specVersion       | 2025-06-18                 |
      | spec.upstream.url | ${CTX:optionalBackend}     |
    Then the response should be successful
    And I send a "GET" request to "${CTX:mcpContext}/mcp" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"

  Scenario: An LLM proxy reaches the backend of its provider with the default identity
    Given I generate a unique resource name from "default-identity-llm-provider" and store it as "providerName"
    And I generate a unique API version from "default-identity-llm-provider" and store it as "providerVersion"
    And I generate a unique API context from "/default-identity-llm-provider" and store it as "providerContext"
    And I generate a unique resource name from "default-identity-llm-proxy" and store it as "proxyName"
    And I generate a unique API version from "default-identity-llm-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/default-identity-llm-proxy" and store it as "proxyContext"
    And the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | ${CTX:optionalBackend}     |
      | accessControl.mode | allow_all                  |
    Then the response status code should be 201
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:proxyName}          |
      | displayName | ${CTX:proxyName}          |
      | version     | ${CTX:proxyVersion}       |
      | context     | ${CTX:proxyContext}       |
      | provider.id | ${CTX:providerName}       |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until the backend sees the client certificate of "gw-identity-a"
    And the response header "X-Client-Verified" should be "true"

  # ==================== SWITCH ON: THE CERTIFICATE POOL ====================

  Scenario: A second default identity is refused
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    When I upload the gateway identity "${CTX:otherIdentity}" from fixture "gw-identity-b" with role default
    Then the response status code should be 409
    And the JSON response field "message" should be "gateway identity ${CTX:defaultIdentity} already has role: default; delete it before uploading another default identity"

  Scenario Outline: Role default is refused for a usage other than identity
    When I upload the certificate fixture "<fixture>" as "${CTX:otherIdentity}" with usage "<usage>" and role "default"
    Then the response status code should be 400
    And the response should list a validation error for field "role" with message "role default applies only to usage: identity certificates"

    Examples:
      | fixture    | usage      |
      | ca-a       | downstream |
      | ca-b       | upstream   |

  Scenario: The listing shows role default only on the default identity
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    And the gateway identity "${CTX:otherIdentity}" is uploaded from fixture "gw-identity-b"
    Then the gateway identity listing should show role default only on "${CTX:defaultIdentity}"
