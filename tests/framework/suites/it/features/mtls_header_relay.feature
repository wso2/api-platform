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
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}"}]}}]         |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                                     |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401

  # ==================== HEADER MODE OFF: THE HEADER IS TEXT ====================

  Scenario: Without a relay entry the header is ignored and deleted, and the connection decides
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 401

  # ==================== HEADER MODE ON: A RELAY ENTRY VOUCHES ====================

  Scenario Outline: With a relay entry, only a connection authenticated as the relay can make the header believed
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                                                                                      | status |
      | with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"                            | 401    |
      | with client certificate "client-expired" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"              | 401    |
      | with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"             | 200    |
      | with client certificate "client-wrong-ca" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"             | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"                     | 200    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"                  | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-expired"                   | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-selfsigned-b"              | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "pem"    | 200    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "base64" | 200    |
      | with client certificate "edge-lb"                                                                                               | 401    |

  Scenario: A header that is not a certificate is rejected when the relay vouched for it
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    And I set header "X-WSO2-CLIENT-CERTIFICATE" to "this-is-not-a-certificate"
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """

  Scenario: A relay entry narrowed by SAN vouches only for the proxy carrying that SAN
    Given I generate a unique resource name from "relay-corp" and store it as "corpRelay"
    And I upload the certificate fixture "corp-ca" as "${CTX:corpRelay}" with usage "downstream" and role "relay" and dns SAN "lb.corp.test"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb-corp" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "corp-other-service" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 401

  Scenario: A believed header reaches the backend as X-Forwarded-Client-Cert where the policy evaluated it, and never on a public route
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"
    And the backend's X-Forwarded-Client-Cert should not name certificate "edge-lb"
    Given I generate a unique value from "relay-public" and store it as "publicName"
    And I generate a unique API version from "relay-public" and store it as "publicVersion"
    And I generate a unique API context from "/relay-public" and store it as "publicContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}              |
      | name                   | ${CTX:publicName}                      |
      | spec.displayName       | Relay Public API                       |
      | spec.version           | ${CTX:publicVersion}                   |
      | spec.context           | ${CTX:publicContext}/$version          |
      | spec.upstream.main.url | http://testbench:3002                  |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicVersion}/anything" until the route answers 200
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicVersion}/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"

  Scenario: A header from a connection that is not the relay never reaches a backend
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    And I generate a unique value from "relay-public" and store it as "publicName"
    And I generate a unique API version from "relay-public" and store it as "publicVersion"
    And I generate a unique API context from "/relay-public" and store it as "publicContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}              |
      | name                   | ${CTX:publicName}                      |
      | spec.displayName       | Relay Public API                       |
      | spec.version           | ${CTX:publicVersion}                   |
      | spec.context           | ${CTX:publicContext}/$version          |
      | spec.upstream.main.url | http://testbench:3002                  |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicVersion}/anything" until the route answers 200
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicVersion}/anything" with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicVersion}/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"
    And the backend's X-Forwarded-Client-Cert should not name certificate "client-wrong-ca"

  Scenario: An API that opts out of the certificate header receives neither header, even one it believed
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    And I generate a unique value from "relay-nofwd" and store it as "nofwdName"
    And I generate a unique API version from "relay-nofwd" and store it as "nofwdVersion"
    And I generate a unique API context from "/relay-nofwd" and store it as "nofwdContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                              |
      | name                   | ${CTX:nofwdName}                                                                                                       |
      | spec.displayName       | Relay Opt-out API                                                                                                      |
      | spec.version           | ${CTX:nofwdVersion}                                                                                                    |
      | spec.context           | ${CTX:nofwdContext}/$version                                                                                           |
      | spec.upstream.main.url | http://testbench:3002                                                                                                  |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}"}],"forwardCertificate":false}}]           |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                                                                  |
    Then the response should be successful
    And I send a "GET" request to "${CTX:nofwdContext}/${CTX:nofwdVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:nofwdContext}/${CTX:nofwdVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the response should not contain echoed header "x-forwarded-client-cert"

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
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:edgeLB}"}]}}]               |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                                              |
    Then the response status should be 400
    And the response should list a validation error for field "spec.policies[0].params.accept[0].ca" with message "${CTX:edgeLB} is a relay (front proxy) entry and cannot be accepted as a client"

  Scenario: One load balancer serves an API that accepts it and an API that accepts the clients it relays
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And I generate a unique resource name from "relay-edge-lb-client" and store it as "edgeLBClient"
    And the certificate fixture "edge-lb-ca" is pooled as "${CTX:edgeLBClient}" with usage "downstream"
    And I generate a unique value from "relay-lb" and store it as "lbName"
    And I generate a unique API version from "relay-lb" and store it as "lbVersion"
    And I generate a unique API context from "/relay-lb" and store it as "lbContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                              |
      | name                   | ${CTX:lbName}                                                                                          |
      | spec.displayName       | Relay LB API                                                                                           |
      | spec.version           | ${CTX:lbVersion}                                                                                       |
      | spec.context           | ${CTX:lbContext}/$version                                                                              |
      | spec.upstream.main.url | http://testbench:3002                                                                                  |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:edgeLBClient}"}]}}]             |
      | spec.operations        | [{"method":"GET","path":"/anything"}]                                                                  |
    Then the response should be successful
    And the response should include a warning with code "MTLS_ACCEPT_NAMES_RELAY_AUTHORITY" for field "spec.policies[0].params.accept[0].ca"
    And I send a "GET" request to "${CTX:lbContext}/${CTX:lbVersion}/anything" until the route answers 401
    And I wait for the analytics collector to settle
    And I reset the analytics collector
    When I send a "GET" request over HTTPS to "${CTX:lbContext}/${CTX:lbVersion}/anything" with client certificate "edge-lb"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:lbContext}/${CTX:lbVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the analytics collector should have received at least 2 events
    And I wait for the analytics collector to settle
    And the latest analytics event for path "${CTX:lbContext}/${CTX:lbVersion}/anything" should have the user id of fixture "edge-lb"
    And I wait for the analytics collector to settle
    And I reset the analytics collector
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the analytics collector should have received at least 1 event
    And I wait for the analytics collector to settle
    And the latest analytics event for path "${CTX:apiContext}/${CTX:apiVersion}/anything" should have the user id of fixture "client-valid"
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb"
    Then the response status code should be 401

  Scenario: Removing the last relay entry turns header mode off again
    Given I generate a unique resource name from "relay-edge-lb" and store it as "edgeLB"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLB}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    When I delete the certificate named "${CTX:edgeLB}"
    Then the response should be successful
    And the gateway has applied the client authority pool
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 401
