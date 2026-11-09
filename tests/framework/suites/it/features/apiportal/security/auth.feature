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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-002..006 and 202..210, at the API
  # level: the file-based login form backed by platform-api's file users. A session is proven usable
  # or unusable by calling an authenticated endpoint with the cookies the login returned; every
  # refused login is followed by that call, so a failed login can never leave a working session.
  # The session's controls follow: the CSRF token every session write needs, the session cookie's
  # attributes and rotation at login, the bearer tokens the Management API must refuse, and which
  # pages need a session or an administrator. Every refusal is paired with the same request
  # succeeding, so a broken route cannot pass as a refusal.

  @APIP-CORE-002 @type:smoke
  Scenario: A user signs in, uses the session and signs out
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

  @APIP-CORE-003 @type:security
  Scenario: A login with an incorrect password opens no session
    When I submit an API Portal file login for actor "admin" with an invalid password
    Then the response status code should be 302
    And the response header "Location" should contain "error="
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

  @APIP-CORE-004 @type:security
  Scenario: A login for a user that does not exist opens no session
    When I submit an API Portal file login for a nonexistent user
    Then the response status code should be 302
    And the response header "Location" should contain "error="
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

  @APIP-CORE-005 @type:security
  Scenario: A login without a password is refused with a reason and opens no session
    When I submit an API Portal file login with a missing password
    Then the response status code should be 302
    And the response header "Location" should contain "Username+and+password+are+required"
    When I send an API Portal session request to "/organizations/default"
    Then the response status code should be 401

  @APIP-CORE-006 @type:security
  Scenario: A caller without a session or token is refused
    When I send an unauthenticated API Portal "GET" request to "/organizations/default"
    Then the response status code should be 401

  @APIP-CORE-202 @type:security
  Scenario Outline: A session write with <token> CSRF token is refused and stores nothing
    Given I generate a unique resource name from "portal-csrf-label" and store it as "labelId"
    When I send an API Portal session "POST" request to "/api-portal/api/v0.9/labels" as "admin" with <token> CSRF token and JSON values:
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

    Examples:
      | token      |
      | no         |
      | an invalid |

  @APIP-CORE-204 @type:security
  Scenario: The session cookie is HttpOnly, Secure and scoped to the portal, and is replaced at sign-in
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

  # The control re-signs the actor's real claims with the control plane's key, so the forging itself
  # is proven sound before each changed token is sent.
  @APIP-CORE-206 @type:security
  Scenario Outline: A bearer token <problem> is refused with the generic credential failure
    When I send an API Portal "GET" request to "/organizations/default" with a "re-signed" bearer token of actor "admin"
    Then the response status code should be 200
    When I send an API Portal "GET" request to "/organizations/default" with a "<variant>" bearer token of actor "admin"
    Then the response status code should be 401
    And the JSON response field "code" should be 401
    And the JSON response field "message" should be "Authentication required"
    And the response body should not contain "expired"
    And the response body should not contain "signature"
    And the response body should not contain "alg"
    And the response body should not contain "stack"

    Examples:
      | problem                                             | variant          |
      | that has expired                                    | expired          |
      | whose claims were changed after signing             | tampered         |
      | with algorithm none and no signature                | alg-none         |
      | signed with HS256 using the public key as secret    | hs256-public-key |
      | signed by a key the control plane does not hold     | foreign-key      |

  @APIP-CORE-208 @type:security
  Scenario Outline: An anonymous visitor browses the catalogue but is sent to sign in for the <page> page
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/apis"
    Then the response status code should be 200
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/views/default/<page>"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default/login"

    Examples:
      | page          |
      | applications  |
      | subscriptions |
      | api-keys      |

  @APIP-CORE-209 @type:security
  Scenario: A developer opens their applications but is refused the settings page, which an administrator opens
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

  # The settings route has no view segment, so the sign-in redirect must still name a real view.
  @APIP-CORE-210 @type:security
  Scenario: An anonymous visitor to the settings page is sent to the default view's sign-in
    When I send an unauthenticated API Portal "GET" request to "/api-portal/default/settings"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default/login"
    And the response header "Location" should not contain "undefined"
