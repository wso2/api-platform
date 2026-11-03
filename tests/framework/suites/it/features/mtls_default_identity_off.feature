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

@mtls @mtls-default-identity
Feature: Not presenting a default client certificate while the switch is off
  As a platform administrator
  I want the gateway to present no client certificate unless an API names an identity
  So that uploading a default identity changes nothing until I switch presenting it on

  With router.upstream.tls.present_default_identity off, no client certificate is presented to
  an HTTPS backend whose upstream definition names no tls.identity, even when a gateway identity
  with role default exists, and it stays that way across requests. The scenarios with the switch
  on are in mtls_default_identity.feature.

  The optional backend answers every request and reports the subject of whatever client
  certificate it was presented, or an empty subject. The required backend trusts the issuer of
  partner A and answers 400 to a request without a certificate it trusts.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I store the URL of the "optional" TLS backend as "optionalBackend"
    And I store the URL of the "required" TLS backend as "requiredBackend"
    And I generate a unique value from "default-identity" and store it as "apiName"
    And I generate a unique API version from "default-identity" and store it as "apiVersion"
    And I generate a unique API context from "/default-identity" and store it as "apiContext"
    And I generate a unique resource name from "backend-trust" and store it as "backendTrust"
    And I generate a unique resource name from "default-identity-cert" and store it as "defaultIdentity"
    And I generate a unique resource name from "other-identity-cert" and store it as "otherIdentity"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendTrust}" with usage "upstream"

  Scenario: A default identity is not presented while the switch is off
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    And the gateway has applied its configuration
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:optionalBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees no client certificate
    And the gateway has applied its configuration
    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/anything" and the backend sees no client certificate in every response
    Then the response header "X-Client-Verified" should be "false"
    And the JSON response field "client" should be ""
    And the JSON response field "verified" should be "false"
    And the response status code should be 200

  Scenario: A backend requiring a client certificate refuses an API that names no identity while the switch is off
    Given the gateway identity "${CTX:defaultIdentity}" is uploaded from fixture "gw-identity-a" with role default
    And the gateway has applied its configuration
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}  |
      | name                    | ${CTX:apiName}             |
      | spec.displayName        | ${CTX:apiName}             |
      | spec.version            | ${CTX:apiVersion}          |
      | spec.context            | ${CTX:apiContext}/$version |
      | spec.upstream.main.url  | ${CTX:requiredBackend}     |
      | spec.operations         | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the backend sees a refusal
    And the gateway has applied its configuration
    When I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/anything" and the backend sees a refusal in every response
    Then the response status code should be 400
    And the JSON response field "backend" should be "a"
    And the JSON response field "error" should be "no required SSL certificate was sent"
    And the response header "X-Client-Subject" should not exist
