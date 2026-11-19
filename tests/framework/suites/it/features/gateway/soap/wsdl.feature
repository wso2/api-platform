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

# The gateway rewrites every endpoint address in a service description it serves on a SOAP
# API's GET route so that it points at the gateway, using the authority the client asked
# for. The testbench soap-backend's WSDL advertises http://testbench:3027<base>/soap11 and
# /soap12; charset=iso-8859-1 serves it in ISO-8859-1, doctype=true adds an internal DTD,
# and a GET without ?wsdl returns an HTML page.
@soap-wsdl
Feature: SOAP API service description rewrite
  As an API consumer
  I want the WSDL a SOAP API serves to advertise the gateway's endpoints
  So that SOAP tooling generated from it calls the gateway, not the backend

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "soap-wsdl" and store it as "soapName"
    And I generate a unique API version from "soap-wsdl" and store it as "soapVersion"
    And I generate a unique API context from "/soap-wsdl" and store it as "soapContext"
    And I create SOAP API from "resources/templates/soap-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:soapName}                     |
      | spec.displayName       | SOAP WSDL                           |
      | spec.version           | ${CTX:soapVersion}                  |
      | spec.context           | ${CTX:soapContext}                  |
      | spec.upstream.main.url | http://testbench:3027/calculator/v1 |
    And the response should be successful

  Scenario: The WSDL's endpoint addresses point at the gateway
    When I set request host to "soap.example.com:8080"
    And I send a "GET" request to "${CTX:soapContext}?wsdl" until status 200
    Then the response header "Content-Type" should contain "text/xml"
    And the response header "Cache-Control" should be "no-store"
    And the response header "ETag" should not exist
    And the response body should contain "http://soap.example.com:8080${CTX:soapContext}/soap11"
    And the response body should contain "http://soap.example.com:8080${CTX:soapContext}/soap12"
    And the response body should not contain "testbench:3027"
    And the response body should contain "targetNamespace"

  Scenario: The addresses follow the authority the client requested
    When I set request host to "other.example.com"
    And I send a "GET" request to "${CTX:soapContext}?wsdl" until status 200
    Then the response body should contain "http://other.example.com${CTX:soapContext}/soap11"
    And the response body should not contain "soap.example.com"
    And the response body should not contain "testbench:3027"

  Scenario: A WSDL in ISO-8859-1 is rewritten and served in its declared encoding
    When I set request host to "soap.example.com:8080"
    And I send a "GET" request to "${CTX:soapContext}?wsdl&charset=iso-8859-1" until status 200
    Then the response header "Content-Type" should contain "iso-8859-1"
    And the response body should contain "encoding="
    And the response body should contain "ISO-8859-1"
    And the response body should contain "http://soap.example.com:8080${CTX:soapContext}/soap11"
    And the response body should not contain "testbench:3027"

  Scenario: A WSDL with a DOCTYPE is refused instead of being served with the backend's addresses
    When I send a "GET" request to "${CTX:soapContext}?wsdl&doctype=true" until status 502
    Then the response header "Content-Type" should contain "text/xml"
    And the response body should contain "<soapenv:Fault>"
    And the response body should not contain "testbench:3027"

    When I send a "GET" request to "${CTX:soapContext}?wsdl"
    Then the response status code should be 200
    And the response body should not contain "testbench:3027"

  Scenario: A GET that is not a service description passes through unchanged
    When I send a "GET" request to "${CTX:soapContext}/docs" until status 200
    Then the response header "Content-Type" should contain "text/html"
    And the response header "X-Backend-Path" should be "/calculator/v1/docs"
    And the response body should contain "<!DOCTYPE html><html lang=en><body>Calculator service</body></html>"
