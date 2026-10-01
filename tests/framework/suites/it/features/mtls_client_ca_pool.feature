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

@mtls @mtls-pool
Feature: Client certificate authority pool
  As a gateway administrator
  I want to curate the set of authorities whose client certificates the gateway trusts
  So that API developers can select from them when protecting an API with mutual TLS

  The pool is managed through the /certificates endpoint. A certificate uploaded with
  usage "downstream" is a client authority; one uploaded without usage (or with usage
  "upstream") is backend trust. Certificate fixtures are generated in memory once per run.

  Every scenario starts from an empty client authority pool.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "pool-partner-a" and store it as "partnerA"
    And I generate a unique resource name from "pool-partner-b" and store it as "partnerB"
    And I generate a unique resource name from "pool-candidate" and store it as "candidate"

  # ==================== UPLOADING A CLIENT AUTHORITY ====================

  Scenario: A CA certificate uploaded with usage downstream becomes a pooled client authority
    When I upload the certificate fixture "ca-a" as "${CTX:partnerA}" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "status" should be "success"
    And the JSON response field "name" should be "${CTX:partnerA}"
    And the JSON response field "usage" should be "downstream"
    And the JSON response field "role" should be "client"
    And the JSON response field "isLeaf" should be false
    And the JSON response field "count" should be 1
    And the JSON response should have field "id"
    And the JSON response should have field "subject"
    And the JSON response should have field "notAfter"
    And the JSON response field "warnings" should not exist
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the response status should be 200
    And the listed certificate "${CTX:partnerA}" should have "usage" equal to "downstream"
    And the listed certificate "${CTX:partnerA}" should have "role" equal to "client"
    And the listed certificate "${CTX:partnerA}" should have "referencedByApis" equal to 0
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=upstream"
    Then the certificate list should not contain "${CTX:partnerA}"

  Scenario: A certificate uploaded without usage is backend trust
    When I upload the certificate fixture "ca-a" as "${CTX:candidate}"
    Then the response status should be 201
    And the JSON response field "usage" should be "upstream"
    And the JSON response field "role" should not exist
    And the JSON response field "count" should be 1
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=upstream"
    Then the listed certificate "${CTX:candidate}" should have "usage" equal to "upstream"
    And the listed certificate "${CTX:candidate}" should not have field "referencedByApis"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "${CTX:candidate}"
    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the certificate list should contain "${CTX:candidate}"

  Scenario: A client authority can be marked as a relay for certificates carried in a header
    When I upload the certificate fixture "ca-a" as "${CTX:candidate}" with usage "downstream" and role "relay"
    Then the response status should be 201
    And the JSON response field "usage" should be "downstream"
    And the JSON response field "role" should be "relay"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "${CTX:candidate}" should have "role" equal to "relay"

  Scenario: An issuing CA uploaded together with its root is one entry identified by the issuing CA
    When I upload the certificate fixtures "ca-a-intermediate,ca-a" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "count" should be 2
    And the JSON response field "isLeaf" should be false
    And the JSON response field "subject" should be "${CTX:fixture.ca-a-intermediate.subject}"
    And the JSON response field "warnings" should not exist

  Scenario: A leaf certificate is accepted as a one-member authority and flagged
    When I upload the certificate fixture "client-selfsigned" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "isLeaf" should be true
    And the JSON response field "warnings[0].code" should be "CLIENT_CA_IS_LEAF"
    And the JSON response field "warnings[0].field" should be "certificate"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "${CTX:candidate}" should have "isLeaf" equal to true

  Scenario: A not-yet-valid authority is accepted with a warning
    When I upload the certificate fixture "ca-not-yet-valid" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "warnings[0].code" should be "CLIENT_CA_NOT_YET_VALID"
    And the JSON response field "warnings[0].field" should be "certificate"

  Scenario: The same certificate may be pooled under a second name
    Given the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I upload the certificate fixture "ca-b" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 201
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "${CTX:partnerB}"
    And the certificate list should contain "${CTX:candidate}"

  Scenario: A second authority with the same subject DN as a pooled one is accepted
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I upload the certificate fixture "ca-b-same-dn" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 201
    And the JSON response field "subject" should be "${CTX:fixture.ca-a.subject}"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "${CTX:partnerA}"
    And the certificate list should contain "${CTX:candidate}"

  Scenario: An authority expiring within thirty days is listed with an expiry warning
    Given the certificate fixture "ca-expires-soon" is pooled as "${CTX:candidate}" with usage "downstream"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "${CTX:candidate}" should have a warning with code "CERT_EXPIRES_SOON"
    And the listed certificate "${CTX:candidate}" should have a warning with field "notAfter"

  Scenario: An authority with more than thirty days left is listed without an expiry warning
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the listed certificate "${CTX:partnerA}" should have no warnings

  # ==================== REJECTED UPLOADS ====================

  Scenario: An expired certificate is rejected because nothing it signed can validate
    When I upload the certificate fixture "ca-expired" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 400
    And the JSON response field "status" should be "error"
    And the response should list a validation error for field "certificate" containing "the certificate expired on"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "${CTX:candidate}"

  Scenario: Two unrelated authorities in one PEM are rejected
    When I upload the certificate fixtures "ca-a,ca-b" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "this PEM contains more than one unrelated authority; upload each as its own entry"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "${CTX:candidate}"

  Scenario: A PEM that carries a private key is rejected and nothing is stored
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "${CTX:candidate}",
        "usage": "downstream",
        "certificate": "${CTX:fixture.ca-a.pem}\n${CTX:fixture.ca-a.key}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "the upload contains a private key; a client-CA entry accepts certificates only"
    And the response body should not contain "PRIVATE KEY"
    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the certificate list should not contain "${CTX:candidate}"

  Scenario: A backend trust certificate that carries a private key is rejected and nothing is stored
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "${CTX:candidate}",
        "usage": "upstream",
        "certificate": "${CTX:fixture.backend-ca.pem}\n${CTX:fixture.backend-ca.key}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "the certificate field takes certificates only; the private key belongs in privateKey"
    And the response body should not contain "PRIVATE KEY"
    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the certificate list should not contain "${CTX:candidate}"

  Scenario Outline: A relay upload with a narrowing the gateway does not understand is rejected
    When I upload to the certificates endpoint the body:
      """
      <body>
      """
    Then the response status should be 400
    And the response should list a validation error for field "<field>" with message "<message>"
    When I send a "GET" request to the "gateway-controller" service at "/certificates"
    Then the certificate list should not contain "${CTX:candidate}"

    Examples:
      | body                                                                                                                                   | field         | message                                               |
      | {"name":"${CTX:candidate}","usage":"downstream","role":"relay","certificate":"${CTX:fixture.edge-lb-ca.pem}","match":{"dnsSAN":["lb.example"]}} | match.dnsSAN  | unknown field dnsSAN; match takes uriSANs and dnsSANs |
      | {"name":"${CTX:candidate}","usage":"downstream","role":"relay","certificate":"${CTX:fixture.edge-lb-ca.pem}","match":{}}                        | match         | match must list uriSANs or dnsSANs, or be omitted     |
      | {"name":"${CTX:candidate}","usage":"downstream","role":"relay","certificate":"${CTX:fixture.edge-lb-ca.pem}","match":{"dnsSANs":"lb.example"}}  | match.dnsSANs | dnsSANs must be a list                                |
      | {"name":"${CTX:candidate}","usage":"downstream","roles":"relay","certificate":"${CTX:fixture.edge-lb-ca.pem}"}                                  | roles         | unknown field roles                                   |

  Scenario: A value that is not a PEM certificate is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "${CTX:candidate}",
        "usage": "downstream",
        "certificate": "this is not a certificate"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" with message "the value is not a PEM-encoded certificate"

  Scenario: An unknown usage is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "${CTX:candidate}",
        "usage": "backend",
        "certificate": "${CTX:fixture.ca-a.pem}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "usage" with message "usage must be upstream, downstream or identity"

  Scenario: An unknown role is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "${CTX:candidate}",
        "usage": "downstream",
        "role": "proxy",
        "certificate": "${CTX:fixture.ca-a.pem}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "role" with message "role must be client, relay or default"

  Scenario: A role given on an upstream certificate is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "${CTX:candidate}",
        "usage": "upstream",
        "role": "relay",
        "certificate": "${CTX:fixture.ca-a.pem}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "role" with message "role relay applies only to usage: downstream certificates"

  Scenario: A name with characters outside letters, digits, dot, underscore and hyphen is rejected
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool/partner a",
        "usage": "downstream",
        "certificate": "${CTX:fixture.ca-a.pem}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "name" with message "name may contain only letters, digits, ., _ and -"

  Scenario: Several problems in one upload are all reported at once
    When I upload to the certificates endpoint the body:
      """
      {
        "name": "pool/bad name",
        "usage": "upstream",
        "role": "relay",
        "certificate": "${CTX:fixture.ca-a.pem}"
      }
      """
    Then the response status should be 400
    And the response should list a validation error for field "name"
    And the response should list a validation error for field "role"

  Scenario: A body over the size limit is rejected without stating the limit
    When I upload a certificate body a tenth over the upload limit as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 413
    And the response body should not contain "1048576"
    And the response body should not contain "MiB"
    And the response body should not contain "bytes"

  # ==================== ONE NAME SPACE ====================

  Scenario: A client authority may not reuse the name of an upstream certificate
    Given the certificate fixture "ca-a" is pooled as "${CTX:candidate}"
    When I upload the certificate fixture "ca-b" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 409
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should contain "already exists"

  Scenario: A duplicate client authority name is a conflict
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I upload the certificate fixture "ca-b" as "${CTX:partnerA}" with usage "downstream"
    Then the response status should be 409
    And the JSON response field "message" should be "a client-CA authority named ${CTX:partnerA} already exists"

  # ==================== REMOVING AN AUTHORITY ====================

  Scenario: A client authority that no API references can be removed
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I delete the certificate named "${CTX:partnerA}"
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "${CTX:partnerA}"

  Scenario: Removing an unknown certificate is not found
    When I send a "DELETE" request to the "gateway-controller" service at "/certificates/no-such-certificate-id"
    Then the response status should be 404
    And the JSON response field "status" should be "error"

  # ==================== ROLES ====================

  Scenario: A developer can read the pool
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And I authenticate using basic auth as "developer"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the response status should be 200
    And the certificate list should contain "${CTX:partnerA}"

  Scenario: A developer cannot add to the pool
    Given I authenticate using basic auth as "developer"
    When I upload the certificate fixture "ca-a" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 403
    Given I authenticate using basic auth as "admin"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should not contain "${CTX:candidate}"

  Scenario: A developer cannot remove from the pool
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And I authenticate using basic auth as "developer"
    When I delete the certificate named "${CTX:partnerA}"
    Then the response status should be 403
    Given I authenticate using basic auth as "admin"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=downstream"
    Then the certificate list should contain "${CTX:partnerA}"

  Scenario: An unauthenticated caller cannot add to the pool
    Given I clear all headers
    When I upload the certificate fixture "ca-a" as "${CTX:candidate}" with usage "downstream"
    Then the response status should be 401
