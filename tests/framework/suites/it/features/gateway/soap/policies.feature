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

# A policy that rejects a request on a SOAP API answers with a SOAP Fault instead of JSON:
# SOAP 1.2 for an application/soap+xml request, SOAP 1.1 otherwise. API-level policies
# apply to both the POST route and the GET route that serves the WSDL.
@soap-policies
Feature: SOAP API policies and SOAP Faults
  As an API publisher
  I want policy rejections on a SOAP API to be SOAP Faults
  So that SOAP clients can handle them like any other fault

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: API key authentication rejects requests with SOAP Faults and admits a valid key
    Given I generate a unique resource name from "soap-apikey" and store it as "soapName"
    And I generate a unique API version from "soap-apikey" and store it as "soapVersion"
    And I generate a unique API context from "/soap-apikey" and store it as "soapContext"
    And I generate a unique value from "soap-key" and store it as "apiKeyName"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP API Key                        |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}] |
    Then the response should be successful

    When I send a "POST" request to the "gateway-controller" service at "/soap-apis/${CTX:soapName}/api-keys" with body:
      """
      {"name": "${CTX:apiKeyName}"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "apiKey"

    # The WSDL route is protected by the same policy.
    When I send a "GET" request to "${CTX:soapContext}?wsdl" until status 401
    Then the response header "Content-Type" should contain "text/xml"
    And the response body should contain "<soapenv:Fault>"
    And the response body should contain "<faultcode>soapenv:Client</faultcode>"

    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap11" with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>7</cal:a><cal:b>5</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response status code should be 401
    And the response header "Content-Type" should contain "text/xml"
    And the response body should contain "<soapenv:Fault>"
    And the response body should contain "<faultcode>soapenv:Client</faultcode>"

    When I set header "Content-Type" to "application/soap+xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap12" with body:
      """
      <env:Envelope xmlns:env="http://www.w3.org/2003/05/soap-envelope" xmlns:cal="http://example.com/soap/calculator/v1"><env:Body><cal:Add><cal:a>7</cal:a><cal:b>5</cal:b></cal:Add></env:Body></env:Envelope>
      """
    Then the response status code should be 401
    And the response header "Content-Type" should contain "application/soap+xml"
    And the response body should contain "<env:Fault>"
    And the response body should contain "<env:Value>env:Sender</env:Value>"

    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I set header "X-API-Key" to "not-a-real-key"
    And I send a "POST" request to "${CTX:soapContext}/soap11" with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>7</cal:a><cal:b>5</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response status code should be 401
    And the response body should contain "<soapenv:Fault>"

    When I set header "X-API-Key" to "${CTX:apiKey}"
    And I send a "POST" request to "${CTX:soapContext}/soap11" until status 200 with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>7</cal:a><cal:b>5</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response body should contain "<cal:result>12</cal:result>"

    When I send a "GET" request to "${CTX:soapContext}?wsdl"
    Then the response status code should be 200
    And the response body should contain "${CTX:soapContext}/soap11"
    And the response body should not contain "testbench:3027"

  # The limit is a template expression. It renders to the string "2" and must reach the
  # policy as the integer its schema declares.
  Scenario: A rate limit from a templated parameter rejects excess requests with SOAP Faults
    Given I generate a unique resource name from "soap-ratelimit" and store it as "soapName"
    And I generate a unique API version from "soap-ratelimit" and store it as "soapVersion"
    And I generate a unique API context from "/soap-ratelimit" and store it as "soapContext"
    When I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP Rate Limit                     |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
      | spec.policies          | [{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":"{{ default \"2\" \"\" }}","duration":"1h"}]}}] |
    Then the response should be successful

    # The readiness wait is the 1st of the 2 allowed requests.
    When I set header "Content-Type" to "text/xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap11" until status 200 with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>1</cal:a><cal:b>1</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    And I send a "POST" request to "${CTX:soapContext}/soap11" with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>1</cal:a><cal:b>2</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "2"

    When I send a "POST" request to "${CTX:soapContext}/soap11" with body:
      """
      <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cal="http://example.com/soap/calculator/v1"><soapenv:Body><cal:Add><cal:a>1</cal:a><cal:b>3</cal:b></cal:Add></soapenv:Body></soapenv:Envelope>
      """
    Then the response status code should be 429
    And the response header "Content-Type" should contain "text/xml"
    And the response header "Retry-After" should exist
    And the response body should contain "<soapenv:Fault>"
    And the response body should contain "<code>900800</code>"

    When I set header "Content-Type" to "application/soap+xml; charset=utf-8"
    And I send a "POST" request to "${CTX:soapContext}/soap12" with body:
      """
      <env:Envelope xmlns:env="http://www.w3.org/2003/05/soap-envelope" xmlns:cal="http://example.com/soap/calculator/v1"><env:Body><cal:Add><cal:a>1</cal:a><cal:b>4</cal:b></cal:Add></env:Body></env:Envelope>
      """
    Then the response status code should be 429
    And the response header "Content-Type" should contain "application/soap+xml"
    And the response body should contain "<env:Fault>"
    And the response body should contain "<code>900800</code>"
