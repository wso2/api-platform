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


@APIPORTAL @apiportal-security
Feature: API Portal hardening: headers, error bodies, upload limits and outbound calls
  As an API Portal operator
  I want responses to disclose nothing about the portal's internals, uploads to stay within their
  limit, and the try-it proxy to call only an API's registered public endpoints
  So that the portal neither advertises what to attack nor becomes a way into the internal network

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level.
  # Each route's status is asserted first, so a broken route cannot pass a header or body check by
  # returning nothing. Every refusal is paired with the same request succeeding.

  @APIP-035 @type:security
  Scenario: Responses name no internals, and uploads are limited without naming the limit
    # The health route does not expose the X-Powered-By header
    When I send an unauthenticated API Portal "GET" request to "/health"
    Then the response status code should be 200
    And the response header "X-Powered-By" should not exist

    # The API route does not expose the X-Powered-By header
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response status code should be 401
    And the response header "X-Powered-By" should not exist

    # The HTML page route does not expose the X-Powered-By header
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/apis"
    Then the response status code should be 200
    And the response header "X-Powered-By" should not exist

    # A refused request is answered with a generic error that names no internals
    Given I generate a unique resource name from "portal-error-label" and store it as "labelId"
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON values:
      | displayName | Label without an id |
    Then the response status code should be 400
    And the JSON response field "code" should be 400
    And the JSON response should have field "message"
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    And the response body should not contain "/app/"
    And the response body should not contain "SELECT"
    And the response body should not contain "    at "
    When I send an authenticated API Portal "POST" request to "/labels" as "admin" with JSON values:
      | id          | ${CTX:labelId}      |
      | displayName | Label without an id |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:labelId}" at "/labels" for cleanup

    # A definition, content archive or theme just over the upload limit is refused without naming the limit, and one just under it is accepted
    Given I generate a unique resource name from "portal-upload-limit" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest oversized" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 413
    And the response body should not contain "10485760"
    And the response body should not contain "10 MB"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "rest near-limit" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
    Then the response status code should be 201
    When I upload API Portal content "oversized" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 413
    And the response body should not contain "10485760"
    And the response body should not contain "10 MB"
    When I upload API Portal content "near-limit" for API "${CTX:apiId}" as "publisher"
    Then the response status code should be 201
    Given I generate a unique resource name from "portal-upload-limit-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}     |
      | displayName | Upload Limit View |
    When I upload API Portal theme "oversized" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 413
    And the response body should not contain "10485760"
    And the response body should not contain "10 MB"
    When I upload API Portal theme "near-limit" for view "${CTX:viewId}" as "admin"
    Then the response status code should be 200

  @APIP-035 @type:security
  Scenario: The try-it proxy refuses internal addresses and calls the API's registered public endpoint
    # The try-it proxy refuses the metadata address registered as its endpoint and calls the API's registered public endpoint
    # A request without browser fetch metadata is refused as cross-site, and a registered public
    # endpoint is called (its .invalid name never resolves, so the call fails upstream with 502):
    # together they show the route works and that only the address decides the refusal.
    Given I generate a unique resource name from "portal-tryout" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                   | ${CTX:apiId}                  |
      | name                 | ${CTX:apiId}                  |
      | endPoints.sandboxURL | http://169.254.169.254/latest |
    Then the response status code should be 201
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://backend.example.invalid/ping"
    Then the response status code should be 403
    And the JSON response field "message" should be "Request origin is not allowed."
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://backend.example.invalid/ping" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 502
    And the JSON response field "error" should be "bad_gateway"
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/http://169.254.169.254/latest/meta-data/" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 422
    And the JSON response field "error" should be "invalid_request"
    And the JSON response field "message" should be "The requested target could not be called."
    And the response body should not contain "169.254.169.254"

    # The try-it proxy refuses a loopback address and calls the API's registered public endpoint
    # A request without browser fetch metadata is refused as cross-site, and a registered public
    # endpoint is called (its .invalid name never resolves, so the call fails upstream with 502):
    # together they show the route works and that only the address decides the refusal.
    Given I generate a unique resource name from "portal-tryout" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                   | ${CTX:apiId}                             |
      | name                 | ${CTX:apiId}                             |
      | endPoints.sandboxURL | http://127.0.0.1:9243/api-portal/health |
    Then the response status code should be 201
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://backend.example.invalid/ping"
    Then the response status code should be 403
    And the JSON response field "message" should be "Request origin is not allowed."
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://backend.example.invalid/ping" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 502
    And the JSON response field "error" should be "bad_gateway"
    # A public target the API never registered is refused, whatever its address
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://unregistered.example.invalid/ping" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 422
    And the JSON response field "error" should be "invalid_request"
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/http://127.0.0.1:9243/api-portal/health" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 422
    And the JSON response field "error" should be "invalid_request"
    And the JSON response field "message" should be "The requested target could not be called."
    And the response body should not contain "127.0.0.1"

    # The try-it proxy refuses an internal host name it never registered and calls the API's registered public endpoint
    # A request without browser fetch metadata is refused as cross-site, and a registered public
    # endpoint is called (its .invalid name never resolves, so the call fails upstream with 502):
    # together they show the route works and that only the address decides the refusal.
    Given I generate a unique resource name from "portal-tryout" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id                   | ${CTX:apiId}                  |
      | name                 | ${CTX:apiId}                  |
      | endPoints.sandboxURL | http://169.254.169.254/latest |
    Then the response status code should be 201
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://backend.example.invalid/ping"
    Then the response status code should be 403
    And the JSON response field "message" should be "Request origin is not allowed."
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://backend.example.invalid/ping" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 502
    And the JSON response field "error" should be "bad_gateway"
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api/${CTX:apiId}/tryout-proxy/https://platform-api:9243/health" with header "Sec-Fetch-Site" set to "same-origin"
    Then the response status code should be 422
    And the JSON response field "error" should be "invalid_request"
    And the JSON response field "message" should be "The requested target could not be called."
    And the response body should not contain "platform-api"
