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

@NETAC
Feature: In deny mode the control plane refuses denied hosts and, when asked to, every private address
  As a platform operator
  I want to deny specific hosts and keep tenant- and user-supplied URLs off private networks
  So that the control plane can be used to reach neither a named service nor anything internal

  # The block's platform-api runs network access control in deny mode, denying 203.0.113.7, with
  # deny_private_networks on (overlays/platform-api-access-control-deny.toml). A spec URL stored
  # on an LLM provider template is checked when it is saved, without being fetched, so public
  # documentation addresses (RFC 5737) can stand in for public hosts. An import fetches its URL,
  # so a hostname that resolves to a private address is refused when it is dialed.
  Background:
    Given I authenticate using basic auth as "admin"
    And I generate a unique resource name from "netac-deny-proj" and store it as "project"
    And I create a project "${CTX:project}" on the control plane

  @NETAC-PA-008 @type:negative
  Scenario: A host matching a deny pattern is refused, while another public host is accepted
    Given I generate a unique resource name from "netac-deny-tpl" and store it as "refusedTemplate"
    And I generate a unique resource name from "netac-deny-tpl-ok" and store it as "allowedTemplate"
    When I create an LLM provider template via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:refusedTemplate}             |
      | endpointUrl    | https://llm.example.com/v1         |
      | openapiSpecUrl | https://203.0.113.7/openapi.yaml   |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "203.0.113.7"
    When I create an LLM provider template via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:allowedTemplate}             |
      | endpointUrl    | https://llm.example.com/v1         |
      | openapiSpecUrl | https://198.51.100.9/openapi.yaml  |
    Then the response status code should be 201

  @NETAC-PA-009 @type:negative
  Scenario: Private destinations are refused, as an address and as a hostname that resolves to one
    Given I generate a unique resource name from "netac-private-tpl" and store it as "refusedTemplate"
    And I generate a unique resource name from "netac-public-tpl" and store it as "allowedTemplate"
    And I generate a unique resource name from "netac-private-api" and store it as "api"
    And I generate a unique API context from "/netac-private" and store it as "apiContext"
    When I create an LLM provider template via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:refusedTemplate}             |
      | endpointUrl    | https://llm.example.com/v1         |
      | openapiSpecUrl | https://10.20.30.40/openapi.yaml   |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "10.20.30.40"
    When I create an LLM provider template via the control plane from "resources/templates/llm-provider-template-cp.yaml" with values:
      | displayName    | ${CTX:allowedTemplate}             |
      | endpointUrl    | https://llm.example.com/v1         |
      | openapiSpecUrl | https://198.51.100.9/openapi.yaml  |
    Then the response status code should be 201
    # A template family is identified by its display name, so the refused one can be
    # looked up by name to show it was never created.
    And the JSON response field "groupId" should be "${CTX:allowedTemplate}"
    When I send a "GET" request to the control plane at "/llm-provider-templates?query=groupId%3A${CTX:refusedTemplate}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/llm-provider-templates?query=groupId%3A${CTX:allowedTemplate}"
    Then the response status code should be 200
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                   |
      | displayName | ${CTX:api}                                   |
      | version     | v1.0                                         |
      | context     | ${CTX:apiContext}                            |
      | projectId   | ${CTX:project}                               |
      | upstream    | {"main": {"url": "http://testbench:3000"}}   |
      | url         | http://testbench:3024/specs/petstore-v1.yaml |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "testbench"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 200

  # An upstream URL is checked when it is saved, from the URL alone: the hostname rule and an
  # address written as a literal. A hostname is not resolved there, so testbench, which resolves
  # into the container network, is accepted; the gateway dials it, not the control plane.
  @NETAC-PA-018 @type:negative
  Scenario: A REST API upstream on a denied host or a private address is refused, while a hostname is checked by name only
    Given I generate a unique resource name from "netac-deny-rest" and store it as "deniedApi"
    And I generate a unique resource name from "netac-deny-private" and store it as "privateApi"
    And I generate a unique resource name from "netac-deny-named" and store it as "namedApi"
    And I generate a unique API context from "/netac-deny-rest" and store it as "deniedContext"
    And I generate a unique API context from "/netac-deny-private" and store it as "privateContext"
    And I generate a unique API context from "/netac-deny-named" and store it as "namedContext"
    When I create a REST API via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:deniedApi}      |
      | displayName | Denied Host REST API  |
      | projectId   | ${CTX:project}        |
      | context     | ${CTX:deniedContext}  |
      | upstreamUrl | https://203.0.113.7   |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    And the response body should not contain "203.0.113.7"
    When I create a REST API via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:privateApi}       |
      | displayName | Private Address REST API |
      | projectId   | ${CTX:project}          |
      | context     | ${CTX:privateContext}   |
      | upstreamUrl | http://10.20.30.40:8080 |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the JSON response field "errors[0].field" should be "upstream.main.url"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:deniedApi}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:privateApi}"
    Then the response status code should be 404
    When I create a REST API via the control plane from "resources/templates/rest-api-cp.yaml" with values:
      | id          | ${CTX:namedApi}       |
      | displayName | Named Host REST API   |
      | projectId   | ${CTX:project}        |
      | context     | ${CTX:namedContext}   |
      | upstreamUrl | http://testbench:3000 |
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:namedApi}"
    Then the response status code should be 200

