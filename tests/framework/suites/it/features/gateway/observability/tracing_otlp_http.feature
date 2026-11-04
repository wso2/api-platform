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

@tracing-otlp-http
Feature: Request tracing over OTLP/HTTP
  As an operator
  I want to export traces to a collector that accepts only OTLP/HTTP
  And go from an analytics record to the trace of its request
  So that I can trace requests with any OpenTelemetry backend

  # The block exports over OTLP/HTTP, and the OTel analytics publisher sends its log records to
  # the same collector.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "tracing-http" and store it as "apiName"
    And I generate a unique API version from "tracing-http" and store it as "apiVersion"
    And I generate a unique API context from "/tracing-http" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}             |
      | name                   | ${CTX:apiName}                        |
      | spec.displayName       | ${CTX:apiName}                        |
      | spec.version           | ${CTX:apiVersion}                     |
      | spec.context           | ${CTX:apiContext}/$version            |
      | spec.upstream.main.url | http://testbench:3002                 |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 200

  Scenario: The router's and the policy engine's spans both arrive over OTLP/HTTP as one trace
    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything"
    Then the response status code should be 200
    And the span "external_processing.process" should be a child of a span from service "router"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: The OTel analytics record of a request carries its trace id
    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything"
    Then the response status code should be 200
    And the otel-collector should have an analytics log record for the trace

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
