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

# A SOAP API is a passthrough: one POST and one GET route under its context, both
# forwarded to the upstream with the context replaced by the upstream base path. The
# testbench soap-backend (port 3027) answers <base>/soap11 and <base>/soap12 and echoes
# the path it received in X-Backend-Path.
@soap-deploy
Feature: SOAP API CRUD and connectivity
  As a gateway operator
  I want to deploy SOAP APIs against the gateway-controller
  So that SOAP clients can reach a SOAP backend through the gateway

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy a SOAP API and invoke it with SOAP 1.1 and SOAP 1.2
    Given I generate a unique resource name from "soap-e2e" and store it as "soapName"
    And I generate a unique value from "soap-e2e" and store it as "soapDisplayName"
    And I generate a unique API version from "soap-e2e" and store it as "soapVersion"
    And I generate a unique API context from "/soap-e2e" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                 |
      | name                   | ${CTX:soapName}                           |
      | spec.displayName       | ${CTX:soapDisplayName}                    |
      | spec.version           | ${CTX:soapVersion}                        |
      | spec.context           | ${CTX:soapContext}                        |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1       |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I set header "SOAPAction" to "Add"
    And I send a "POST" request to "${CTX:soapContext}/soap11" until status 200 with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>7</cal:a><cal:b>5</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response header "Content-Type" should contain "text/xml"
    And the response header "X-Backend-Path" should be "/calculator/v1/soap11"
    And the response body should contain "<cal:result>12</cal:result>"

    When I set header "Content-Type" to "application/soap+xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap12" with body:
      """
      <env:Envelope xmlns:env="http://www.w3.org/2003/05/soap-envelope" xmlns:cal="http://example.com/soap/calculator/v1"><env:Body><cal:Subtract><cal:a>7</cal:a><cal:b>5</cal:b></cal:Subtract></env:Body></env:Envelope>
      """
    Then the response status code should be 200
    And the response header "Content-Type" should contain "application/soap+xml"
    And the response header "X-Backend-Path" should be "/calculator/v1/soap12"
    And the response body should contain "<cal:result>2</cal:result>"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: A SOAP Fault from the backend passes through unchanged
    Given I generate a unique resource name from "soap-backend-fault" and store it as "soapName"
    And I generate a unique API version from "soap-backend-fault" and store it as "soapVersion"
    And I generate a unique API context from "/soap-backend-fault" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP Backend Fault                  |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    Then the response should be successful

    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap11" until status 500 with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Divide><cal:a>1</cal:a><cal:b>0</cal:b></cal:Divide></soapenv:Body></soapenv:Envelope>
      """
    Then the response header "Content-Type" should contain "text/xml"
    And the response header "X-Backend-Path" should be "/calculator/v1/soap11"
    And the response body should contain "<faultstring>division by zero</faultstring>"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful

  Scenario: Update a deployed SOAP API's upstream, and verify the change takes effect
    Given I generate a unique resource name from "soap-update" and store it as "soapName"
    And I generate a unique value from "soap-update" and store it as "soapDisplayName"
    And I generate a unique API version from "soap-update" and store it as "soapVersion"
    And I generate a unique API context from "/soap-update" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | ${CTX:soapDisplayName}              |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    Then the response should be successful

    When I update SOAP API "${CTX:soapName}" from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | ${CTX:soapDisplayName} v2           |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v2 |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "spec.displayName" should be "${CTX:soapDisplayName} v2"

    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap11" until header "X-Backend-Path" is "/calculator/v2/soap11" with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>1</cal:a><cal:b>2</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response status code should be 200
    And the response body should contain "<cal:result>3</cal:result>"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful

  # The context is replaced by the upstream base path. An upstream URL that ends in "/"
  # must not double the slash in front of the operation path.
  Scenario: An upstream URL with a trailing slash reaches the backend without a double slash
    Given I generate a unique resource name from "soap-trailing-slash" and store it as "soapName"
    And I generate a unique API version from "soap-trailing-slash" and store it as "soapVersion"
    And I generate a unique API context from "/soap-trailing-slash" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}            |
      | name                   | ${CTX:soapName}                      |
      | spec.displayName       | SOAP Trailing Slash                  |
      | spec.version           | ${CTX:soapVersion}                   |
      | spec.context           | ${CTX:soapContext}                   |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1/ |
    Then the response should be successful

    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap11" until status 200 with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>7</cal:a><cal:b>5</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response header "X-Backend-Path" should be "/calculator/v1/soap11"
    And the response body should contain "<cal:result>12</cal:result>"

    When I send a "GET" request to "${CTX:soapContext}"
    Then the response status code should be 200
    And the response header "X-Backend-Path" should be "/calculator/v1/"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful

  Scenario: An undeployed SOAP API keeps its configuration and serves no traffic
    Given I generate a unique resource name from "soap-undeployed" and store it as "soapName"
    And I generate a unique API version from "soap-undeployed" and store it as "soapVersion"
    And I generate a unique API context from "/soap-undeployed" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP Undeployed                     |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.deploymentState   | undeployed                          |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    Then the response should be successful
    And the JSON response field "status.state" should be "undeployed"

    When I get the SOAP API "${CTX:soapName}"
    Then the response should be successful
    And the JSON response field "spec.deploymentState" should be "undeployed"

    When I update SOAP API "${CTX:soapName}" from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP Undeployed                     |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.deploymentState   | deployed                            |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    Then the response should be successful
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "${CTX:soapContext}/page" until status 200

    When I update SOAP API "${CTX:soapName}" from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP Undeployed                     |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.deploymentState   | undeployed                          |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    Then the response should be successful
    And the JSON response field "status.state" should be "undeployed"
    And I send a "GET" request to "${CTX:soapContext}/page" until status 404

    When I get the SOAP API "${CTX:soapName}"
    Then the response should be successful
    And the JSON response field "spec.upstream.main.url" should be "http://testbench:3027/calculator/v1"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful

  Scenario: Deploy a SOAP API with labels and verify they are stored
    Given I generate a unique resource name from "soap-labeled" and store it as "soapName"
    And I generate a unique API version from "soap-labeled" and store it as "soapVersion"
    And I generate a unique API context from "/soap-labeled" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                       |
      | name                   | ${CTX:soapName}                                 |
      | spec.displayName       | SOAP Labeled                                    |
      | spec.version           | ${CTX:soapVersion}                              |
      | spec.context           | ${CTX:soapContext}                              |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1             |
      | metadata.labels        | {"environment":"production","team":"soap-team"} |
    Then the response should be successful

    When I get the SOAP API "${CTX:soapName}"
    Then the response should be successful
    And the JSON response field "metadata.labels.environment" should be "production"
    And the JSON response field "metadata.labels.team" should be "soap-team"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful

  Scenario: Deploy a SOAP API without an upstream fails validation
    Given I generate a unique resource name from "soap-no-upstream" and store it as "soapName"
    And I generate a unique API version from "soap-no-upstream" and store it as "soapVersion"
    And I generate a unique API context from "/soap-no-upstream" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion | ${CTX:gatewaySpecVersion} |
      | name       | ${CTX:soapName}           |
      | spec       | {"displayName":"SOAP No Upstream","version":"${CTX:soapVersion}","context":"${CTX:soapContext}"} |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== LIST / GET ====================

  Scenario: List SOAP APIs filtered by displayName
    Given I generate a unique resource name from "soap-filter" and store it as "soapName"
    And I generate a unique value from "UniqueSoapFilterTest" and store it as "soapDisplayName"
    And I generate a unique API version from "soap-filter" and store it as "soapVersion"
    And I generate a unique API context from "/soap-filter" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | ${CTX:soapDisplayName}              |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/soap-apis?displayName=${CTX:soapDisplayName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 1
    And the response body should contain "${CTX:soapName}"

    When I delete the SOAP API "${CTX:soapName}"
    Then the response should be successful

  Scenario: List SOAP APIs when none match a filter
    Given I generate a unique value from "no-such-soap-api" and store it as "missingDisplayName"
    When I send a "GET" request to the "gateway-controller" service at "/soap-apis?displayName=${CTX:missingDisplayName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "count" should be 0

  Scenario: Get a non-existent SOAP API returns 404
    When I send a "GET" request to the "gateway-controller" service at "/soap-apis/non-existent-soap-id"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
