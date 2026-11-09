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
Feature: Users sign in to the API Portal with file-based credentials
  As an API Portal user
  I want only correct credentials to open a session, and signing out to end it
  So that nobody reaches the protected API Portal REST API without a valid session

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API
  # level: the file-based login form backed by platform-api's file users. A session is proven usable
  # or unusable by calling an authenticated endpoint with the cookies the login returned; every
  # refused login is followed by that call, so a failed login can never leave a working session.
  # The session's controls follow: the CSRF token every session write needs, the session cookie's
  # attributes and rotation at login, the bearer tokens the Management API must refuse, and which
  # pages need a session or an administrator. Every refusal is paired with the same request
  # succeeding, so a broken route cannot pass as a refusal.

  @APIP-031 @type:security @type:smoke
  Scenario: A user signs in with a protected session, and failed logins and forged writes open nothing
    # A user signs in, uses the session and signs out
    When I submit an API Portal file login for actor "admin"
    Then the response status code should be 302
    And the response header "Location" should not contain "error="
    And the API Portal response should set a cookie named "connect.sid"
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 200
    When I send an API Portal session request to "/api-portal/default/views/default/logout"
    Then the response status code should be 302
    And the response header "Location" should contain "/login"
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

    # The session cookie is HttpOnly, Secure and scoped to the portal, and is replaced at sign-in
    When I open the API Portal login page
    Then the response status code should be 200
    And the API Portal response should set a cookie named "connect.sid"
    When I submit an API Portal file login for actor "admin" in the login page's session
    Then the response status code should be 302
    And the response header "Location" should not contain "error="
    And the API Portal response cookie "connect.sid" should have attribute "HttpOnly"
    And the API Portal response cookie "connect.sid" should have attribute "Secure"
    And the API Portal response cookie "connect.sid" should have attribute "Path=/api-portal"
    And the API Portal session cookie should differ from the login page's session cookie
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 200

    # A login with an incorrect password opens no session
    When I submit an API Portal file login for actor "admin" with an invalid password
    Then the response status code should be 302
    And the response header "Location" should contain "error="
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

    # A login for a user that does not exist opens no session
    When I submit an API Portal file login for a nonexistent user
    Then the response status code should be 302
    And the response header "Location" should contain "error="
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

    # A login without a password is refused with a reason and opens no session
    When I submit an API Portal file login with a missing password
    Then the response status code should be 302
    And the response header "Location" should contain "Username+and+password+are+required"
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

    # A session write with no CSRF token is refused and stores nothing
    Given I generate a unique resource name from "portal-csrf-label" and store it as "labelId"
    When I send an API Portal session "POST" request to "/api-portal/api/v0.9/labels" as "admin" with no CSRF token and JSON values:
      | id          | ${CTX:labelId} |
      | displayName | CSRF Label     |
    Then the response status code should be 403
    And the JSON response field "message" should be "CSRF token missing or invalid"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404
    When I send an API Portal session "POST" request to "/api-portal/api/v0.9/labels" as "admin" with JSON values:
      | id          | ${CTX:labelId} |
      | displayName | CSRF Label     |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:labelId}" at "/labels" for cleanup

    # A session write with an invalid CSRF token is refused and stores nothing
    Given I generate a unique resource name from "portal-csrf-label" and store it as "labelId"
    When I send an API Portal session "POST" request to "/api-portal/api/v0.9/labels" as "admin" with an invalid CSRF token and JSON values:
      | id          | ${CTX:labelId} |
      | displayName | CSRF Label     |
    Then the response status code should be 403
    And the JSON response field "message" should be "CSRF token missing or invalid"
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404
    When I send an API Portal session "POST" request to "/api-portal/api/v0.9/labels" as "admin" with JSON values:
      | id          | ${CTX:labelId} |
      | displayName | CSRF Label     |
    Then the response status code should be 201
    And I register API Portal resource "${CTX:labelId}" at "/labels" for cleanup

  @APIP-032 @type:security
  Scenario: Bearer tokens that are missing or forged are refused with the generic credential failure
    # A caller without a session or token is refused
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response status code should be 401

    # A bearer token that has expired is refused with the generic credential failure
    # The control re-signs the actor's real claims with the control plane's key, so the forging itself
    # is proven sound before each changed token is sent.
    When I send an API Portal "GET" request to "/organizations/default" with a "re-signed" bearer token of actor "admin"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/default" with a "expired" bearer token of actor "admin"
    Then the response status code should be 401
    And the JSON response field "code" should be 401
    And the JSON response field "message" should be "Authentication required"
    And the response body should not contain "expired"
    And the response body should not contain "signature"
    And the response body should not contain "alg"
    And the response body should not contain "stack"

    # A bearer token whose claims were changed after signing is refused with the generic credential failure
    # The control re-signs the actor's real claims with the control plane's key, so the forging itself
    # is proven sound before each changed token is sent.
    When I send an API Portal "GET" request to "/organizations/default" with a "re-signed" bearer token of actor "admin"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/default" with a "tampered" bearer token of actor "admin"
    Then the response status code should be 401
    And the JSON response field "code" should be 401
    And the JSON response field "message" should be "Authentication required"
    And the response body should not contain "expired"
    And the response body should not contain "signature"
    And the response body should not contain "alg"
    And the response body should not contain "stack"

    # A bearer token with algorithm none and no signature is refused with the generic credential failure
    # The control re-signs the actor's real claims with the control plane's key, so the forging itself
    # is proven sound before each changed token is sent.
    When I send an API Portal "GET" request to "/organizations/default" with a "re-signed" bearer token of actor "admin"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/default" with a "alg-none" bearer token of actor "admin"
    Then the response status code should be 401
    And the JSON response field "code" should be 401
    And the JSON response field "message" should be "Authentication required"
    And the response body should not contain "expired"
    And the response body should not contain "signature"
    And the response body should not contain "alg"
    And the response body should not contain "stack"

    # A bearer token signed with HS256 using the public key as secret is refused with the generic credential failure
    # The control re-signs the actor's real claims with the control plane's key, so the forging itself
    # is proven sound before each changed token is sent.
    When I send an API Portal "GET" request to "/organizations/default" with a "re-signed" bearer token of actor "admin"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/default" with a "hs256-public-key" bearer token of actor "admin"
    Then the response status code should be 401
    And the JSON response field "code" should be 401
    And the JSON response field "message" should be "Authentication required"
    And the response body should not contain "expired"
    And the response body should not contain "signature"
    And the response body should not contain "alg"
    And the response body should not contain "stack"

    # A bearer token signed by a key the control plane does not hold is refused with the generic credential failure
    # The control re-signs the actor's real claims with the control plane's key, so the forging itself
    # is proven sound before each changed token is sent.
    When I send an API Portal "GET" request to "/organizations/default" with a "re-signed" bearer token of actor "admin"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/default" with a "foreign-key" bearer token of actor "admin"
    Then the response status code should be 401
    And the JSON response field "code" should be 401
    And the JSON response field "message" should be "Authentication required"
    And the response body should not contain "expired"
    And the response body should not contain "signature"
    And the response body should not contain "alg"
    And the response body should not contain "stack"

  @APIP-033 @type:security
  Scenario: Pages follow sign-in and the administrator role
    # An anonymous visitor browses the catalogue but is sent to sign in for the applications page
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/apis"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/applications"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default/login"

    # An anonymous visitor browses the catalogue but is sent to sign in for the subscriptions page
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/apis"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/subscriptions"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default/login"

    # An anonymous visitor browses the catalogue but is sent to sign in for the api-keys page
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/apis"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/api-keys"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default/login"

    # An anonymous visitor to the settings page is sent to the default view's sign-in
    # The settings route has no view segment, so the sign-in redirect must still name a real view.
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/settings"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default/login"
    And the response header "Location" should not contain "undefined"

    # A developer opens their applications but is refused the settings page, which an administrator opens
    When I submit an API Portal file login for actor "developer"
    Then the response status code should be 302
    When I send an API Portal session request to "/api-portal/default/views/default/applications"
    Then the response status code should be 200
    When I send an API Portal session request to "/api-portal/default/settings"
    Then the response status code should be 403
    And the response body should contain "Access denied"
    When I submit an API Portal file login for actor "admin"
    Then the response status code should be 302
    When I send an API Portal session request to "/api-portal/default/settings"
    Then the response status code should be 200
