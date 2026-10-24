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

# Runs only with tracing and the otel analytics publisher on:
# cd gateway/it && make test-tracing-analytics
Feature: Analytics records carry their trace
  As an operator
  I want each analytics record to name the trace of its request
  So that I can go from a record to the request's trace

  Background:
    Given the gateway services are running
    And I reset the trace collector

  Scenario: The OTel analytics record of a request carries its trace id
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tracing-analytics-api
      spec:
        displayName: Tracing Analytics API
        version: v1.0
        context: /tracing-analytics/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/tracing-analytics/v1.0/health" to be ready

    When I send the following requests in a new trace
    And I send a GET request to "http://localhost:8080/tracing-analytics/v1.0/health"
    Then the response status code should be 200
    And I wait for the span "external_processing.process"
    And an analytics log record should carry the trace

    Given I authenticate using basic auth as "admin"
    When I delete the API "tracing-analytics-api"
    Then the response should be successful
