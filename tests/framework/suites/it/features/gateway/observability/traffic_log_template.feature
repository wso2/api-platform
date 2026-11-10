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


@traffic-log-template
Feature: Traffic log lines from a JSON template
  As a gateway operator
  I want traffic log lines written in the layout my log tools expect
  So that I can read them without converting the default line

  The gateway in this block writes traffic log lines to stdout with the inline template in
  core/catalog/overlays/traffic-log-template.toml. Every value in that template is the same on
  every run, so each scenario compares the whole line.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "traffic-log-template" and store it as "apiName"
    And I generate a unique API version from "traffic-log-template" and store it as "apiVersion"
    And I generate a unique API context from "/traffic-log-template" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/info"}] |
    Then the response should be successful

  Scenario: A request is logged as one line in the template's layout
    Given I clear all headers
    And I set header "Authorization" to "Bearer not-a-real-token"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 200
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":200},"auth":"****","user":null,"typo":null,"fixed":[1.5,true,null,"gw"]}
      """

  Scenario: A value the request does not have is written as null
    Given I clear all headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 200
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":200},"auth":null,"user":null,"typo":null,"fixed":[1.5,true,null,"gw"]}
      """
