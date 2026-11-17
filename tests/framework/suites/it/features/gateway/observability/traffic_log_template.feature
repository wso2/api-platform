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
  every run except the latency, so each scenario compares the whole line, with <number> standing
  for the latency in milliseconds. The template also holds a few mistakes, which the gateway
  reports once at startup.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "traffic-log-template" and store it as "apiName"
    And I generate a unique API version from "traffic-log-template" and store it as "apiVersion"
    And I generate a unique API context from "/traffic-log-template" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                                                                       |
      | name                   | ${CTX:apiName}                                                                                                                                                                                  |
      | spec.displayName       | ${CTX:apiName}                                                                                                                                                                                  |
      | spec.version           | ${CTX:apiVersion}                                                                                                                                                                               |
      | spec.context           | ${CTX:apiContext}/$version                                                                                                                                                                      |
      | spec.upstream.main.url | http://testbench:3002                                                                                                                                                                           |
      | spec.operations        | [{"method":"GET","path":"/info"},{"method":"GET","path":"/status/{code}"},{"method":"GET","path":"/secure","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

  Scenario: A request is logged as one line in the template's layout
    Given I clear all headers
    And I set header "Authorization" to "Bearer not-a-real-token"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 200
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":200},"auth":"****","user":null,"claims":null,"typo":null,"fixed":[1.5,true,null,"gw"],"resBody":null,"error":{"category":null,"code":null,"message":null},"durationMs":<number>,"badUnit":null}
      """

  Scenario: A value the request does not have is written as null
    Given I clear all headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 200
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":200},"auth":null,"user":null,"claims":null,"typo":null,"fixed":[1.5,true,null,"gw"],"resBody":null,"error":{"category":null,"code":null,"message":null},"durationMs":<number>,"badUnit":null}
      """

  Scenario Outline: An error status from the backend writes its error details
    Given I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/info" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/status/<status>" until status <status>
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":<status>},"auth":null,"user":null,"claims":null,"typo":null,"fixed":[1.5,true,null,"gw"],"resBody":null,"error":{"category":"OTHER","code":<status>,"message":"UNCLASSIFIED"},"durationMs":<number>,"badUnit":null}
      """

    Examples:
      | status |
      | 404    |
      | 500    |
      | 503    |

  Scenario: An authenticated request writes its subject and claims, without an excluded claim
    Given I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "tenant=acme,region=eu" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/secure" until status 200
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":200},"auth":"****","user":"test-user","claims":{"region":"eu"},"typo":null,"fixed":[1.5,true,null,"gw"],"resBody":null,"error":{"category":null,"code":null,"message":null},"durationMs":<number>,"badUnit":null}
      """

  Scenario: A request the gateway refuses writes its error details and no latency
    Given I clear all headers
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/secure" until status 401
    Then the "gateway-runtime" log since the latest request should contain:
      """
      {"source":"it-traffic-log-template","api":"${CTX:apiName}","http":{"method":"GET","status":401},"auth":null,"user":null,"claims":null,"typo":null,"fixed":[1.5,true,null,"gw"],"resBody":null,"error":{"category":"AUTH","code":401,"message":"AUTHENTICATION_FAILURE"},"durationMs":null,"badUnit":null}
      """

  Scenario: Mistakes in the template are reported at startup
    Then the "gateway-runtime" service logs should contain:
      """
      traffic_logging.template: field \"typo\" is written as null
      """
    And the "gateway-runtime" service logs should contain:
      """
      traffic_logging.template: field \"badUnit\" is written as null: convertUnit: unsupported unit \"rs\"
      """
    And the "gateway-runtime" service logs should contain:
      """
      traffic_logging.template: field \"resBody\" reads response.body but collector.response_body is false; no response body will be logged
      """
    And the "gateway-runtime" service logs should contain:
      """
      traffic_logging.template: exclude_fields entry \"nosuch\" matches no field in the template and has no effect
      """
    And the "gateway-runtime" service logs should contain:
      """
      traffic_logging.properties is set but traffic_logging.template is also set; properties are ignored while a template is used
      """
