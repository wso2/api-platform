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

@mtls @mtls-pool-references
Feature: Pool entries referenced by APIs, and the levers that revoke a client
  As a gateway administrator and an API developer
  I want removing trust to be safe and immediate
  So that a pool entry an API depends on cannot vanish underneath it, and cutting off one
  client is an edit that applies on the next request

  Every scenario starts from an empty client authority pool, and every HTTPS request waits until
  the gateway has applied the pool the scenario built.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "ref-api" and store it as "apiName"
    And I generate a unique API version from "ref-api" and store it as "apiVersion"
    And I generate a unique API context from "/ref-api" and store it as "apiContext"
    And I generate a unique value from "ref-other-api" and store it as "otherApiName"
    And I generate a unique API version from "ref-other-api" and store it as "otherApiVersion"
    And I generate a unique API context from "/ref-other-api" and store it as "otherApiContext"
    And I generate a unique resource name from "ref-partner-a" and store it as "partnerA"
    And I generate a unique resource name from "ref-partner-b" and store it as "partnerB"
    And I generate a unique resource name from "ref-entry" and store it as "entry"

  # ==================== WHO REFERENCES WHAT ====================

  Scenario: The listing counts the APIs that name an authority, not the ones that inherit the pool
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:otherApiName} |
      | spec.displayName       | ${CTX:otherApiName} |
      | spec.version           | ${CTX:otherApiVersion} |
      | spec.context           | ${CTX:otherApiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "${CTX:partnerA}" should have "referencedByApis" equal to 1
    And the listed certificate "${CTX:partnerB}" should have "referencedByApis" equal to 0

  Scenario: An authority named in an API's accept list cannot be removed while the reference stands
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I delete the certificate named "${CTX:partnerA}"
    Then the response status should be 409
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "is named by 1 deployed API"
    And the response should list a validation error for field "spec.policies[0].params.accept[0].ca" containing "${CTX:apiName}"
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I delete the API "${CTX:apiName}"
    And I delete the certificate named "${CTX:partnerA}" once no API references it
    Then the response should be successful

  Scenario: An authority referenced only by inheriting APIs can be removed, and they stop accepting it on the next request
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 200
    When I delete the certificate named "${CTX:partnerB}"
    Then the response should be successful
    And the gateway has applied the client authority pool
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: The last client authority cannot be removed while any API attaches mtls-auth
    Given the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "${CTX:entry}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I delete the certificate named "${CTX:entry}"
    Then the response status should be 409
    And the JSON response field "message" should contain "cannot remove the last client-CA authority while 1 deployed API"
    And the response should list a validation error for field "spec.policies[0]" containing "${CTX:apiName}"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "${CTX:entry}"
    Given the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I delete the certificate named "${CTX:entry}"
    Then the response should be successful

  Scenario: A relay entry can be removed even when it is the last one, since header mode simply turns off
    Given the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And I upload the certificate fixture "edge-lb-ca" as "${CTX:entry}" with usage "downstream" and role "relay"
    And the response status should be 201
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I delete the certificate named "${CTX:entry}"
    Then the response should be successful

  Scenario: An unreferenced upstream trust certificate can be deleted
    Given the certificate fixture "backend-ca" is pooled as "${CTX:entry}"
    When I delete the certificate named "${CTX:entry}"
    Then the response should be successful

  # ==================== REVOKING ONE CLIENT IS AN EDIT ====================

  Scenario: Removing one SAN from the list cuts off exactly the clients carrying it
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","match":{"uriSANs":["urn:partner-a:payments","urn:partner-a:first"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-multi-san"
    Then the response status code should be 200
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-multi-san"
    Then the response status code should be 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: Removing one partner's entry leaves the other partner untouched
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"},{"ca":"${CTX:partnerB}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 200
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: A thumbprint cut-over lists old and new, then drops the old
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","thumbprints":["${CTX:fixture.client-valid.thumbprint}","${CTX:fixture.client-renewed.thumbprint}"]}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-renewed"
    Then the response status code should be 200
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","thumbprints":["${CTX:fixture.client-renewed.thumbprint}"]}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-renewed"
    Then the response status code should be 200

  # ==================== EDGE CASES IN THE ACCEPT LIST ====================

  Scenario: Listing the same authority twice is accepted and evaluates once
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","match":{"uriSANs":["urn:partner-a:nothing"]}},{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: The same authority pooled under two names is usable by either name
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the certificate fixture "ca-a" is pooled as "${CTX:entry}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | ${CTX:apiName} |
      | spec.version           | ${CTX:apiVersion} |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:entry}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
