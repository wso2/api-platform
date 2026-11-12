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
Feature: APIP cloud managed API Portal and consumer sign-in
  As an APIP cloud user
  I want to create a managed API Portal that my consumers can sign up and sign in to
  So that portal creation, its refusals, consumer sign-in and portal deletion are verified

  # The refusals run first, while the runner holds no portal: the organisation's plan may allow a
  # single portal, and the plan is checked before the request is.
  # The creation scenario then builds one portal for the runner, because a build takes minutes.
  # Every later scenario uses that portal and the last one deletes it. When the platform fails the
  # build, the creation scenario reports it and the later scenarios build one replacement. A
  # consumer is a user of the organisation's identity provider that the cloud API cannot remove,
  # so the runner signs up one consumer and the later sign-in scenarios reuse it.

  @TC_APIP_PORTAL_009 @TC_APIP_PORTAL_018 @ID_APIP_PORTAL_001
  Scenario Outline: A portal request that breaks the naming rules is refused and leaves nothing behind
    Given I obtain an APIP cloud console token
    When I request a managed API Portal that has "<defect>"
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

    Examples:
      | defect                             |
      | an uppercase handle                |
      | a handle with an underscore        |
      | a handle shorter than 3 characters |
      | a handle longer than 40 characters |
      | no name                            |

  @TC_APIP_PORTAL_017 @ID_APIP_PORTAL_002
  Scenario: A portal whose consumers would sign in on an environment the organisation does not have is refused
    Given I obtain an APIP cloud console token
    When I request a managed API Portal whose login environment does not exist
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

  @TC_APIP_PORTAL_001 @TC_APIP_PORTAL_002 @TC_APIP_PORTAL_015 @ID_APIP_PORTAL_003
  Scenario: A portal requested with a name and a handle is built and becomes active
    Given I obtain an APIP cloud console token
    When I request a synthetic managed API Portal for the runner
    Then the synthetic managed API Portal should be accepted for provisioning
    And the synthetic managed API Portal should become active
    And the cloud API response should report:
      | loginEnvironment | development |
      | selfSignup       | true        |
    And the synthetic managed API Portal should report an HTTPS URL
    And the managed API Portal list should include the synthetic managed API Portal

  @TC_APIP_PORTAL_008 @ID_APIP_PORTAL_004
  Scenario: A portal whose handle is already in use is refused
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I request a managed API Portal under the handle of the synthetic managed API Portal
    Then the cloud API response status code should be 409
    And the synthetic managed API Portal should still be active

  @TC_APIP_PORTAL_013 @ID_APIP_PORTAL_005
  Scenario: Reading a portal that does not exist returns not found
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I read the synthetic managed API Portal
    Then the cloud API response status code should be 200
    When I read a managed API Portal that does not exist
    Then the cloud API response status code should be 404

  @skip
  @TC_APIP_PORTALRT_024 @ID_APIP_PORTAL_006
  Scenario: A visitor who opens the portal's sign-in is sent to its identity provider
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I open the sign-in page of the synthetic managed API Portal as a new visitor
    Then the visitor should be sent to the portal's identity provider to sign in

  @skip
  @TC_APIP_PORTALRT_025 @ID_APIP_PORTAL_007
  Scenario: A consumer signs up from the portal's sign-in page and is signed in
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I sign up on the synthetic managed API Portal as a new consumer
    Then the consumer should be signed in to the synthetic managed API Portal
    And a visitor without the consumer's session should be sent to sign in

  @skip
  @TC_APIP_PORTALRT_019 @ID_APIP_PORTAL_008
  Scenario: A consumer who signed up signs in again with a username and password
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I sign in to the synthetic managed API Portal as the consumer
    Then the consumer should be signed in to the synthetic managed API Portal

  @skip
  @TC_APIP_PORTALRT_026 @ID_APIP_PORTAL_009
  Scenario: A wrong password does not sign the consumer in
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I try to sign in to the synthetic managed API Portal as the consumer with a wrong password
    Then the identity provider should refuse the sign-in
    And the consumer should not be signed in to the synthetic managed API Portal
    When I sign in to the synthetic managed API Portal as the consumer
    Then the consumer should be signed in to the synthetic managed API Portal

  @skip
  @TC_APIP_PORTALRT_027 @ID_APIP_PORTAL_010
  Scenario: Signing out ends the consumer's session
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And I sign in to the synthetic managed API Portal as the consumer
    And the consumer should be signed in to the synthetic managed API Portal
    When I sign out of the synthetic managed API Portal
    Then the consumer should not be signed in to the synthetic managed API Portal

  @TC_APIP_PORTAL_007 @ID_APIP_PORTAL_011
  Scenario: A deleted portal is torn down and leaves the list
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I delete the synthetic managed API Portal
    Then the managed API Portal deletion should be accepted
    And the synthetic managed API Portal should eventually be gone

  @known-issue
  @TC_APIP_PUB_001 @ID_APIP_PORTAL_022
  Scenario: A REST API is published and unpublished, and its draft is kept
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 201
    And the synthetic REST API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic REST API should offer the synthetic subscription plan
    And the API publications of the synthetic REST API should show the managed API Portal as PUBLISHED
    When I unpublish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 204
    And the synthetic REST API should not be published on the managed API Portal
    And the API publications of the synthetic REST API should show the managed API Portal as NOT_PUBLISHED
    And the publication draft of the synthetic REST API should still exist
    When I unpublish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PUBLICATION_STATE_CONFLICT"

  @known-issue
  @TC_APIP_PUB_002 @ID_APIP_PORTAL_023
  Scenario: Republishing with a newly created plan offers it alongside the first
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    And I create another synthetic subscription plan
    When I save a publication draft of the synthetic REST API with:
      | subscriptionPlanIds | ["${CTX:cloudSubscriptionPlanID}", "${CTX:cloudSecondSubscriptionPlanID}"] |
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the publication of the synthetic REST API should offer both synthetic subscription plans

  @known-issue
  @TC_APIP_PUB_003 @ID_APIP_PORTAL_024
  Scenario: Republishing an edited draft replaces the listing
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    When I save a publication draft of the synthetic REST API with:
      | displayName | Edited ${CTX:cloudAPIID}       |
      | description | Edited by the APIP cloud test. |
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic REST API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic REST API should have the display name "Edited ${CTX:cloudAPIID}"

  @known-issue
  @TC_APIP_PUB_004 @ID_APIP_PORTAL_025
  Scenario: Deprecating a published API changes only its status, and only once
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    When I deprecate the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic REST API should be DEPRECATED on the managed API Portal
    And the API publications of the synthetic REST API should show the managed API Portal as DEPRECATED
    And the publication of the synthetic REST API should offer the synthetic subscription plan
    When I deprecate the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PUBLICATION_STATE_CONFLICT"

  @known-issue
  @TC_APIP_PUB_005 @ID_APIP_PORTAL_026
  Scenario: A document, a landing page and a thumbnail are published, and an unselected document is removed
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I create a synthetic document for the synthetic REST API
    And I save a publication draft of the synthetic REST API with:
      | docIds | ["${CTX:cloudDocumentID}"] |
    And I save a valid definition on the publication draft of the synthetic REST API
    And I save a landing page on the publication draft of the synthetic REST API
    And I save a PNG thumbnail on the publication draft of the synthetic REST API
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 201
    And the publication of the synthetic REST API should include the synthetic document
    And the publication of the synthetic REST API should have a landing page and a thumbnail
    When I save a publication draft of the synthetic REST API with:
      | docIds | [] |
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the publication of the synthetic REST API should not include the synthetic document

  @known-issue
  @TC_APIP_PUB_006 @ID_APIP_PORTAL_027
  Scenario: A GraphQL API is published, deprecated and unpublished
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I create a synthetic GraphQL API in the default project
    And I save a publication draft of the synthetic GraphQL API
    And I save a valid definition on the publication draft of the synthetic GraphQL API
    When I publish the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 201
    And the synthetic GraphQL API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic GraphQL API should offer the synthetic subscription plan
    When I deprecate the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic GraphQL API should be DEPRECATED on the managed API Portal
    When I unpublish the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 204
    And the synthetic GraphQL API should not be published on the managed API Portal
    And the publication draft of the synthetic GraphQL API should still exist


  # Drafts and the checks made before publishing. Nothing below reaches the managed API Portal: a
  # draft stays on the control plane, and every publish is refused before the portal is called.

  @known-issue
  @TC_APIP_PUBV_001 @ID_APIP_PORTAL_028
  Scenario: A saved draft keeps its details, its definition and a landing page without raw HTML
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I save a landing page on the publication draft of the synthetic REST API
    Then the publication draft of the synthetic REST API should keep its details and definition
    And the landing page of the publication draft of the synthetic REST API should have no raw HTML

  @TC_APIP_PUBV_002 @ID_APIP_PORTAL_029
  Scenario Outline: A draft naming something that does not exist is refused
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I try to save a publication draft naming an unknown <reference>
    Then the cloud API response status code should be <status>
    And the cloud API error code should be "<code>"

    Examples:
      | reference         | status | code                          |
      | subscription plan | 400    | PUBLICATION_VALIDATION_FAILED |
      | API               | 404    | API_NOT_FOUND                 |
      | API Portal        | 404    | API_PORTAL_NOT_FOUND          |

  @TC_APIP_PUBV_003 @ID_APIP_PORTAL_030
  Scenario: Publishing needs a saved draft with a valid definition
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 404
    And the cloud API error code should be "DRAFT_NOT_FOUND"
    When I save a publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    When I save an invalid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    And the synthetic REST API should not be published on the managed API Portal

  @TC_APIP_PUBV_004 @ID_APIP_PORTAL_031
  Scenario Outline: An API that was never published can be neither unpublished nor deprecated
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I <action> the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PUBLICATION_STATE_CONFLICT"

    Examples:
      | action    |
      | unpublish |
      | deprecate |

  @TC_APIP_PUBV_005 @ID_APIP_PORTAL_032
  Scenario: An inactive subscription plan cannot be offered, even when it was active when the draft was saved
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    When I deactivate the synthetic subscription plan
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    And the synthetic REST API should not be published on the managed API Portal
    When I try to save a publication draft of the synthetic REST API
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"

  @TC_APIP_PUBV_006 @ID_APIP_PORTAL_033
  Scenario: A subscription plan offered by a draft cannot be deleted
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    When I try to delete the synthetic subscription plan
    Then the cloud API response status code should be 409
    And the cloud API error code should be "SUBSCRIPTION_PLAN_IN_USE"

  @TC_APIP_PUBV_007 @ID_APIP_PORTAL_034
  Scenario: A GraphQL draft is published only through its own route, and only with a GraphQL definition
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I create a synthetic GraphQL API in the default project
    And I save a publication draft of the synthetic GraphQL API
    When I save an OpenAPI definition on the publication draft of the synthetic GraphQL API
    And I publish the synthetic GraphQL API through the REST API route
    Then the cloud API response status code should be 404
    And the cloud API error code should be "API_NOT_FOUND"
    When I publish the synthetic GraphQL API on the managed API Portal
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    And the synthetic GraphQL API should not be published on the managed API Portal

  @TC_APIP_PUBV_008 @ID_APIP_PORTAL_035
  Scenario Outline: Publication routes refuse a request <credential>
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I send a "<method>" request <credential> to the publication path "<path>"
    Then the cloud API response status code should be 401

    Examples:
      | method | credential          | path                                                                                    |
      | GET    | without a token     | /api-publications?apiType=rest-api&apiId=${CTX:cloudAPIID}                              |
      | PUT    | without a token     | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/draft              |
      | POST   | without a token     | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/publish            |
      | POST   | without a token     | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/unpublish          |
      | GET    | with a forged token | /api-publications?apiType=rest-api&apiId=${CTX:cloudAPIID}                              |
      | PUT    | with a forged token | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/draft              |
      | POST   | with a forged token | /api-portals/${CTX:cloudAPIPortalID}/apis/graphql-api/${CTX:cloudAPIID}/publish         |
      | POST   | with a forged token | /api-portals/${CTX:cloudAPIPortalID}/apis/rest-api/${CTX:cloudAPIID}/deprecate          |
