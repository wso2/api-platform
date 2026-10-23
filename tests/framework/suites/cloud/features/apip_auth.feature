# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@apip-cloud
Feature: APIP cloud access and identity
  As an APIP cloud operator
  I want only a valid console token for my organisation to reach the cloud API
  So that a missing, forged or malformed credential never reads or changes a tenant's data

  @TC_APIP_AUTH_001 @ID_APIP_AUTH_001
  Scenario: The console token is a live JWT that the cloud API accepts
    Given I obtain an APIP cloud console token
    Then the APIP cloud console token should be a JWT that has not expired
    When I send an authenticated "GET" request to the APIP cloud path "/projects/default"
    Then the cloud API response status code should be 200

  @TC_APIP_AUTH_002 @ID_APIP_AUTH_002
  Scenario: The cloud API resolves the caller's organisation from the token alone
    Given I obtain an APIP cloud console token
    Then a "GET" request to the APIP cloud path "/projects/default" should give the same result when the "X-Org-Id" header names another organisation

  @TC_APIP_AUTH_003 @ID_APIP_AUTH_003
  Scenario: The console token carries the organisation claim the control plane requires
    Given I obtain an APIP cloud console token
    Then the APIP cloud console token should carry an organisation claim

  @TC_APIP_AUTH_004 @ID_APIP_AUTH_004
  Scenario: A request without an Authorization header is refused
    When I send a "GET" request to the APIP cloud path "/projects" without an Authorization header
    Then the cloud API response status code should be 401

  @TC_APIP_AUTH_005 @ID_APIP_AUTH_005
  Scenario: A request bearing an expired token is refused
    Given I obtain an APIP cloud console token
    When I send a "GET" request to the APIP cloud path "/projects" bearing a token that expired an hour ago
    Then the cloud API response status code should be 401

  @TC_APIP_AUTH_006 @ID_APIP_AUTH_006
  Scenario Outline: A token signed by an unknown issuer or key is refused
    When I send a "GET" request to the APIP cloud path "/projects" bearing a token <forgery>
    Then the cloud API response status code should be 401

    Examples:
      | forgery                              |
      | signed with an unknown key           |
      | with no signature                    |
      | signed with an unknown shared secret |

  @TC_APIP_AUTH_007 @ID_APIP_AUTH_007
  Scenario Outline: A malformed bearer value is refused without leaking internal detail
    When I send a "GET" request to the APIP cloud path "/projects" with the Authorization header "<header>"
    Then the cloud API response status code should be 401
    And the cloud API error response should not expose internal detail

    Examples:
      | header             |
      | Bearer x           |
      | Bearer a.b.c       |
      | Bearer a.b.        |
      | Bearer             |
      | Basic dXNlcjpwYXNz |

  @TC_APIP_AUTH_008 @ID_APIP_AUTH_008
  Scenario Outline: Every cloud control-plane route requires authentication
    When I send a "<method>" request to the APIP cloud path "<path>" without an Authorization header
    Then the cloud API response status code should be 401

    Examples:
      | method | path                                                                  |
      | GET    | /projects                                                             |
      | GET    | /projects/default                                                     |
      | GET    | /projects/default/deployment-pipeline                                 |
      | GET    | /projects/default/apis/no-such-api/builds                             |
      | GET    | /projects/default/apis/no-such-api/deployments                        |
      | GET    | /rest-apis                                                            |
      | GET    | /environments                                                         |
      | GET    | /pipelines                                                            |
      | GET    | /managed-gateways                                                     |
      | GET    | /managed-gateways/no-such-gateway                                     |
      | GET    | /managed-gateways/no-such-gateway/configuration                       |
      | GET    | /managed-api-portals                                                  |
      | GET    | /llm-providers/no-such-provider/environment-deployments               |
      | GET    | /logs                                                                 |
      | POST   | /projects                                                             |
      | POST   | /rest-apis                                                            |
      | POST   | /environments                                                         |
      | POST   | /pipelines                                                            |
      | POST   | /managed-gateways                                                     |
      | POST   | /projects/default/apis/no-such-api/builds                             |
      | POST   | /projects/default/apis/no-such-api/deployments                        |
      | POST   | /projects/default/apis/no-such-api/deployments/no-such-dep/undeploy   |
      | DELETE | /environments/no-such-environment                                     |
      | DELETE | /managed-gateways/no-such-gateway                                     |

