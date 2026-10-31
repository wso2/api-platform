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

@mtls @mtls-header-relay
Feature: Client certificates relayed in a header by a front proxy
  As a gateway administrator running the gateway behind a load balancer that terminates TLS
  I want the client certificate the proxy relays in a header to authenticate the client
  So that partners keep their certificates while the topology changes, without letting anyone
  who can write a header impersonate anyone else

  The same mtls-auth policy handles it and API definitions do not change. A header is text anyone
  can send, so it is believed only when the connection that carries it authenticated as a pool entry
  marked as a relay. Otherwise the header is ignored, never trusted, and the connection's own
  certificate is evaluated as itself. The relayed header never reaches a backend: when the policy
  believed it, the backend receives the certificate it carried as X-Forwarded-Client-Cert instead,
  unless the API sets forwardCertificate to false. The gateway's own default configuration applies
  here: no bypass.

  Every scenario starts from an empty client authority pool, and every HTTPS request waits until
  the gateway has applied the pool the scenario built.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "relay" and store it as "apiName"
    And I generate a unique API version from "relay" and store it as "apiVersion"
    And I generate a unique API context from "/relay" and store it as "apiContext"
    And I generate a unique resource name from "relay-partner-a" and store it as "caA"
    And I generate a unique resource name from "relay-partner-b" and store it as "caB"
    And the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "${CTX:caB}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                 |
      | name                   | ${CTX:apiName}                                                                            |
      | spec.displayName       | Relay API                                                                                 |
      | spec.version           | ${CTX:apiVersion}                                                                         |
      | spec.context           | ${CTX:apiContext}/$version                                                                |
      | spec.upstream.main.url | http://testbench:3002                                                                     |
      | spec.policies          | [{"name":"mtls-auth","version":"v0","params":{"accept":[{"ca":"${CTX:caA}"}]}}]         |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                                     |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401

  # ==================== HEADER MODE ON: A RELAY ENTRY VOUCHES ====================

  Scenario: A relay entry cannot be accepted as a client
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And I generate a unique value from "relay-refused" and store it as "refusedName"
    And I generate a unique API version from "relay-refused" and store it as "refusedVersion"
    And I generate a unique API context from "/relay-refused" and store it as "refusedContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                          |
      | name                   | ${CTX:refusedName}                                                                                 |
      | spec.displayName       | Relay Refused API                                                                                  |
      | spec.version           | ${CTX:refusedVersion}                                                                              |
      | spec.context           | ${CTX:refusedContext}/$version                                                                     |
      | spec.upstream.main.url | http://testbench:3002                                                                              |
      | spec.policies          | [{"name":"mtls-auth","version":"v0","params":{"accept":[{"ca":"${CTX:edgeLB}"}]}}]               |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                                              |
    Then the response status should be 400
    And the response should list a validation error for field "spec.policies[0].params.accept[0].ca" with message "${CTX:edgeLB} is a relay (front proxy) entry and cannot be accepted as a client"
