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

  @TC_APIP_PORTAL_009 @TC_APIP_PORTAL_018 @ID_APIP_PORTAL_001 @layer:cloud-owned
  Scenario Outline: A portal request that breaks the naming rules is refused and leaves nothing behind
    Given I obtain an APIP cloud console token
    When I request a managed API Portal that has "<defect>"
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

    Examples:
      | defect                                    |
      | an uppercase handle                       |
      | a handle with an underscore               |
      | a handle shorter than 3 characters        |
      | a handle longer than 40 characters        |
      | no name                                   |
      | a name longer than 255 characters         |
      | a description longer than 1023 characters |

  @TC_APIP_PORTAL_017 @ID_APIP_PORTAL_002 @layer:cloud-owned
  Scenario: A portal whose consumers would sign in on an environment the organisation does not have is refused
    Given I obtain an APIP cloud console token
    When I request a managed API Portal whose login environment does not exist
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

  @TC_APIP_PORTAL_001 @TC_APIP_PORTAL_002 @TC_APIP_PORTAL_015 @ID_APIP_PORTAL_003 @layer:cloud-owned
  Scenario: A portal requested with a name and a handle is built and becomes active
    Given I obtain an APIP cloud console token
    When I request a synthetic managed API Portal for the runner
    Then the synthetic managed API Portal should be accepted for provisioning
    And the managed API Portal answer should locate the synthetic managed API Portal
    And the managed API Portal answer should expose no secret
    And the synthetic managed API Portal should become active
    And the cloud API response should report:
      | loginEnvironment | development |
      | selfSignup       | true        |
    And the synthetic managed API Portal should report an HTTPS URL
    And the managed API Portal list should include the synthetic managed API Portal

  @TC_APIP_PORTAL_008 @ID_APIP_PORTAL_004 @layer:cloud-owned
  Scenario: A portal whose handle is already in use is refused
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I request a managed API Portal under the handle of the synthetic managed API Portal
    Then the cloud API response status code should be 409
    And the synthetic managed API Portal should still be active

  @TC_APIP_PORTAL_013 @ID_APIP_PORTAL_005 @layer:cloud-owned
  Scenario: Reading a portal that does not exist returns not found
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I read the synthetic managed API Portal
    Then the cloud API response status code should be 200
    When I read a managed API Portal that does not exist
    Then the cloud API response status code should be 404

  @ID_APIP_PORTAL_036 @layer:cloud-owned
  Scenario Outline: A create body that breaks the request's shape is refused and leaves nothing behind
    Given I obtain an APIP cloud console token
    When I send a managed API Portal create request with <defect>
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

    Examples:
      | defect                  |
      | an unknown field        |
      | an organization field   |
      | a name that is not text |
      | malformed JSON          |

  @ID_APIP_PORTAL_037 @layer:cloud-owned
  Scenario: A portal's answers carry its details and never a secret
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I read the synthetic managed API Portal
    Then the cloud API response status code should be 200
    And the managed API Portal answer should carry its details
    And the managed API Portal answer should expose no secret
    And the managed API Portal list should include the synthetic managed API Portal
    And the managed API Portal answer should expose no secret

  @TC_APIP_PORTAL_003 @ID_APIP_PORTAL_012 @layer:cloud-owned
  Scenario: An active portal answers at its URL and sends a visitor to its view
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I request "/api-portal/health" from the synthetic managed API Portal
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | status | ok |
    When I follow the front door of the synthetic managed API Portal to its view
    And I request "/outside-the-portal" from the synthetic managed API Portal
    Then the cloud API response status code should be 404

  @ID_APIP_PORTAL_038 @layer:cloud-owned
  Scenario: The portal's management routes refuse a caller that does not hold the control plane's key
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I request "/api-portal/health" from the synthetic managed API Portal
    Then the cloud API response status code should be 200
    When I request "/api-portal/api/v0.9/webhook-subscribers" from the synthetic managed API Portal
    Then the cloud API response status code should be 401
    When I request "/api-portal/api/v0.9/webhook-subscribers" from the synthetic managed API Portal with a wrong shared key
    Then the cloud API response status code should be 401

  @TC_APIP_PORTAL_004 @ID_APIP_PORTAL_013 @layer:cloud-owned
  Scenario: A portal's name and description change in place
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I update the synthetic managed API Portal with:
      | name        | Renamed ${CTX:cloudAPIPortalID}  |
      | description | Changed by the APIP cloud test. |
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | id          | ${CTX:cloudAPIPortalID}          |
      | name        | Renamed ${CTX:cloudAPIPortalID}  |
      | description | Changed by the APIP cloud test. |
    And the synthetic managed API Portal should still be active
    When I read the synthetic managed API Portal
    Then the cloud API response should report:
      | name        | Renamed ${CTX:cloudAPIPortalID}  |
      | description | Changed by the APIP cloud test. |

  @ID_APIP_PORTAL_039 @layer:cloud-owned
  Scenario Outline: An update that breaks the rules is refused and leaves the portal as it was
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I try to update the synthetic managed API Portal with <defect>
    Then the cloud API response status code should be 400
    And the synthetic managed API Portal should still be active

    Examples:
      | defect                                    |
      | an empty name                             |
      | an empty login environment                |
      | a name longer than 255 characters         |
      | a description longer than 1023 characters |
      | a handle                                  |

  # apim-saas: Update and RotateSharedKey return the records 404 unmapped, so the platform answers 500 instead of 404.
  @known-issue
  @ID_APIP_PORTAL_040 @layer:cloud-owned
  Scenario Outline: Changing a portal that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I <change> a managed API Portal that does not exist
    Then the cloud API response status code should be 404
    And the cloud API error code should be "not_found"

    Examples:
      | change            |
      | update            |
      | rotate the key of |

  # apim-saas: the key-hash PATCH to secret-manager-api is sent as application/json, not application/merge-patch+json, and gets 415.
  @known-issue
  @TC_APIP_PORTAL_005 @ID_APIP_PORTAL_014 @layer:cloud-owned
  Scenario: After the shared key is rotated the control plane still publishes to the portal
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And the runner has an active synthetic managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    When I rotate the key of the synthetic managed API Portal
    Then the cloud API response status code should be 202
    And the synthetic managed API Portal should become active
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 201
    And the synthetic REST API should be PUBLISHED on the managed API Portal

  @ID_APIP_PORTAL_041 @layer:cloud-owned
  Scenario: Turning self sign-up off is accepted and finishes, it cannot be combined with a login environment change, and it can be turned on again
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I update the synthetic managed API Portal with:
      | selfSignup       | false          |
      | loginEnvironment | no-environment |
    Then the cloud API response status code should be 400
    And the synthetic managed API Portal should still be active
    When I update the synthetic managed API Portal with:
      | selfSignup | false |
    Then the cloud API response status code should be 202
    And the cloud API response should report:
      | status | pending |
    When the synthetic managed API Portal should become active
    And I read the synthetic managed API Portal
    Then the cloud API response should report:
      | selfSignup | false |
    When I update the synthetic managed API Portal with:
      | selfSignup | true |
    Then the cloud API response status code should be 202
    And the synthetic managed API Portal should become active
    When I read the synthetic managed API Portal
    Then the cloud API response should report:
      | selfSignup | true |

  @ID_APIP_PORTAL_054 @layer:core-smoke
  Scenario: A visitor to the default view of a provisioned portal is signed in silently and its AI discovery file answers
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And I follow the front door of the synthetic managed API Portal to its view
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}" from the synthetic managed API Portal
    Then the cloud API response status code should be 302
    And the portal answer should send the visitor to the identity provider for a silent sign-in
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/llms.txt" from the synthetic managed API Portal
    Then the cloud API response status code should be 200

  @ID_APIP_PORTAL_055 @layer:core-smoke
  Scenario: A path outside the portal and a page it does not have are not found while its AI discovery file loads
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And I follow the front door of the synthetic managed API Portal to its view
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/llms.txt" from the synthetic managed API Portal
    Then the cloud API response status code should be 200
    When I request "/not-the-portal/apis" from the synthetic managed API Portal
    Then the cloud API response status code should be 404
    And the portal answer header "Content-Type" should contain "text/plain"
    And the portal answer should contain "Not Found"
    When I request "/api-portal/${CTX:cloudPortalOrg}/no-such-page" from the synthetic managed API Portal
    Then the cloud API response status code should be 404
    And the portal answer header "Content-Type" should contain "text/html"
    And the portal answer should contain "Page not found"
    And the portal answer should not contain "stack"

  @ID_APIP_PORTAL_056 @layer:core-smoke
  Scenario: The portal's answers name no internals
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And I follow the front door of the synthetic managed API Portal to its view
    When I request "/api-portal/health" from the synthetic managed API Portal
    Then the cloud API response status code should be 200
    And the portal answer should carry no "X-Powered-By" header
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}" from the synthetic managed API Portal
    Then the cloud API response status code should be 302
    And the portal answer should carry no "X-Powered-By" header
    When I request "/api-portal/api/v0.9/webhook-subscribers" from the synthetic managed API Portal
    Then the cloud API response status code should be 401
    And the portal answer should carry no "X-Powered-By" header

  # Local cloud only: it has no CP Thunder, so the portal sign-in names no identity provider (consumer-login-not-configured.invalid).
  @known-issue
  @ID_APIP_PORTAL_047 @layer:cloud-owned
  Scenario: A portal with self sign-up off does not offer sign-up on its sign-in, and one with it on does
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And the sign-in of the synthetic managed API Portal should offer sign-up
    When I update the synthetic managed API Portal with:
      | selfSignup | false |
    Then the cloud API response status code should be 202
    And the synthetic managed API Portal should become active
    And the sign-in of the synthetic managed API Portal should not offer sign-up
    When I update the synthetic managed API Portal with:
      | selfSignup | true |
    Then the cloud API response status code should be 202
    And the synthetic managed API Portal should become active
    And the sign-in of the synthetic managed API Portal should offer sign-up

  @TC_APIP_PORTALRT_024 @ID_APIP_PORTAL_006 @layer:cloud-owned
  Scenario: A visitor who opens the portal's sign-in is sent to its identity provider
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I open the sign-in page of the synthetic managed API Portal as a new visitor
    Then the visitor should be sent to the portal's identity provider to sign in

  @TC_APIP_PORTALRT_025 @ID_APIP_PORTAL_007 @layer:cloud-owned
  Scenario: A consumer signs up from the portal's sign-in page and is signed in
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I sign up on the synthetic managed API Portal as a new consumer
    Then the consumer should be signed in to the synthetic managed API Portal
    And a visitor without the consumer's session should be sent to sign in

  @TC_APIP_PORTALRT_019 @ID_APIP_PORTAL_008 @layer:cloud-owned
  Scenario: A consumer who signed up signs in again with a username and password
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I sign in to the synthetic managed API Portal as the consumer
    Then the consumer should be signed in to the synthetic managed API Portal

  @TC_APIP_PORTALRT_026 @ID_APIP_PORTAL_009 @layer:cloud-owned
  Scenario: A wrong password does not sign the consumer in
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I try to sign in to the synthetic managed API Portal as the consumer with a wrong password
    Then the identity provider should refuse the sign-in
    And the consumer should not be signed in to the synthetic managed API Portal
    When I sign in to the synthetic managed API Portal as the consumer
    Then the consumer should be signed in to the synthetic managed API Portal

  @TC_APIP_PORTALRT_027 @ID_APIP_PORTAL_010 @layer:cloud-owned
  Scenario: Signing out ends the consumer's session
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And I sign in to the synthetic managed API Portal as the consumer
    And the consumer should be signed in to the synthetic managed API Portal
    When I sign out of the synthetic managed API Portal
    Then the consumer should not be signed in to the synthetic managed API Portal

  @TC_APIP_PORTAL_007 @ID_APIP_PORTAL_011 @layer:cloud-owned
  Scenario: A deleted portal is torn down and leaves the list
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I delete the synthetic managed API Portal
    Then the managed API Portal deletion should be accepted
    And the synthetic managed API Portal should eventually be gone

  @TC_APIP_PUB_001 @ID_APIP_PORTAL_022 @layer:inherited
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

  @TC_APIP_PUB_002 @ID_APIP_PORTAL_023 @layer:inherited
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
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the publication of the synthetic REST API should offer both synthetic subscription plans

  @TC_APIP_PUB_003 @ID_APIP_PORTAL_024 @layer:inherited
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
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the synthetic REST API should be PUBLISHED on the managed API Portal
    And the publication of the synthetic REST API should have the display name "Edited ${CTX:cloudAPIID}"

  @TC_APIP_PUB_004 @ID_APIP_PORTAL_025 @layer:inherited
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

  @TC_APIP_PUB_005 @ID_APIP_PORTAL_026 @layer:inherited
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
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    And the publication of the synthetic REST API should not include the synthetic document

  @TC_APIP_PUB_006 @ID_APIP_PORTAL_027 @layer:inherited
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

  @TC_APIP_PUBV_001 @ID_APIP_PORTAL_028 @layer:inherited
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

  @TC_APIP_PUBV_002 @ID_APIP_PORTAL_029 @layer:inherited
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

  @TC_APIP_PUBV_003 @ID_APIP_PORTAL_030 @layer:inherited
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

  @TC_APIP_PUBV_004 @ID_APIP_PORTAL_031 @layer:inherited
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

  @TC_APIP_PUBV_005 @ID_APIP_PORTAL_032 @layer:inherited
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

  @TC_APIP_PUBV_006 @ID_APIP_PORTAL_033 @layer:inherited
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

  @TC_APIP_PUBV_007 @ID_APIP_PORTAL_034 @layer:inherited
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

  @TC_APIP_PUBV_008 @ID_APIP_PORTAL_035 @layer:cloud-owned
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

  @ID_APIP_PORTAL_049 @layer:inherited
  Scenario Outline: A draft whose <field> is at its limit is saved
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I try to save a publication draft of the synthetic REST API with a <field> of <length> characters
    Then the cloud API response status code should be 200

    Examples:
      | field       | length |
      | displayName | 255    |
      | version     | 30     |
      | description | 1023   |

  @ID_APIP_PORTAL_050 @layer:inherited
  Scenario Outline: A draft whose <field> is empty or over its limit is refused
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I try to save a publication draft of the synthetic REST API with a <field> of <length> characters
    Then the cloud API response status code should be 400
    And the cloud API error code should be "PUBLICATION_VALIDATION_FAILED"
    When I save a publication draft of the synthetic REST API
    Then the cloud API response status code should be 200

    Examples:
      | field       | length |
      | displayName | 0      |
      | displayName | 256    |
      | version     | 0      |
      | version     | 31     |
      | description | 1024   |

  @ID_APIP_PORTAL_051 @layer:inherited
  Scenario: A saved draft keeps its description, visibility, endpoints and owners
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    When I save a publication draft of the synthetic REST API with:
      | description     | Metadata round trip.                                                                                                                            |
      | agentVisibility | HIDDEN                                                                                                                                          |
      | tags            | ["alpha", "beta"]                                                                                                                               |
      | endpoints       | {"productionUrl": "https://prod.example.com", "sandboxUrl": "https://sandbox.example.com"}                                                    |
      | owners          | {"businessOwner": "Pat Doe", "businessOwnerEmail": "pat@example.com", "technicalOwner": "Sam Roe", "technicalOwnerEmail": "sam@example.com"} |
    Then the publication draft of the synthetic REST API should report:
      | description                | Metadata round trip.        |
      | agentVisibility            | HIDDEN                      |
      | endpoints.productionUrl    | https://prod.example.com    |
      | endpoints.sandboxUrl       | https://sandbox.example.com |
      | owners.businessOwner       | Pat Doe                     |
      | owners.businessOwnerEmail  | pat@example.com             |
      | owners.technicalOwner      | Sam Roe                     |
      | owners.technicalOwnerEmail | sam@example.com             |

  @ID_APIP_PORTAL_052 @layer:inherited
  Scenario: A draft refuses a definition of an unsupported type and a thumbnail that is not an image or is too large
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a PNG thumbnail on the publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    When I save a definition of type "text/plain" on the publication draft of the synthetic REST API
    Then the cloud API response status code should be 400
    When I save a thumbnail that is text declared as PNG on the publication draft of the synthetic REST API
    Then the cloud API response status code should be 400
    When I save a thumbnail that is larger than 2 MiB on the publication draft of the synthetic REST API
    Then the cloud API response status code should be 413

  @ID_APIP_PORTAL_053 @layer:inherited
  Scenario: A saved draft returns its definition and thumbnail unchanged
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I save a PNG thumbnail on the publication draft of the synthetic REST API
    Then the publication draft of the synthetic REST API should return the definition and the thumbnail it was saved with

  @ID_APIP_PORTAL_042 @layer:cloud-owned
  Scenario: A published API is served by the portal and leaves it when unpublished
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I follow the front door of the synthetic managed API Portal to its view
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal
    Then the cloud API response status code should be 404
    When I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 201
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}/docs/specification.json" from the synthetic managed API Portal until status 200
    Then the cloud API response should report:
      | openapi | 3.0.3 |
    And the portal answer should contain "/ping"
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 200
    Then the portal answer should contain "${CTX:cloudAPIID}"
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/apis.md" from the synthetic managed API Portal until status 200
    Then the portal answer should contain "${CTX:cloudAPIID}"
    When I unpublish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 204
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 404
    And I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}/docs/specification.json" from the synthetic managed API Portal until status 404
    And I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/apis.md" from the synthetic managed API Portal until status 200
    Then the portal answer should not contain "${CTX:cloudAPIID}"

  @ID_APIP_PORTAL_057 @layer:inherited
  Scenario: A republished edit is what the portal serves
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I follow the front door of the synthetic managed API Portal to its view
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    And I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 200
    When I save a publication draft of the synthetic REST API with:
      | displayName | Edited ${CTX:cloudAPIID} |
    And I save a valid definition on the publication draft of the synthetic REST API
    And I publish the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 200
    Then the portal answer should contain "Edited ${CTX:cloudAPIID}"

  @ID_APIP_PORTAL_058 @layer:inherited
  Scenario: A deprecated API is still served by the portal
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I follow the front door of the synthetic managed API Portal to its view
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    And I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 200
    When I deprecate the synthetic REST API on the managed API Portal
    Then the cloud API response status code should be 200
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 200
    Then the portal answer should contain "${CTX:cloudAPIID}"
    When I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}/docs/specification.json" from the synthetic managed API Portal until status 200
    Then the portal answer should contain "/ping"

  # api-platform: DeleteAPI never unpublishes the API from the portal, so the page stays served.
  @known-issue
  @ID_APIP_PORTAL_043 @layer:cloud-owned
  Scenario: Deleting a published API removes its publication and its page on the portal
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active managed API Portal
    And I create a synthetic subscription plan
    And I create a synthetic REST API in the default project
    And I save a publication draft of the synthetic REST API
    And I save a valid definition on the publication draft of the synthetic REST API
    And I follow the front door of the synthetic managed API Portal to its view
    And I publish the synthetic REST API on the managed API Portal
    And the cloud API response status code should be 201
    And I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 200
    When I delete the synthetic REST API
    Then the synthetic REST API should not be published on the managed API Portal
    And I request "/api-portal/${CTX:cloudPortalOrg}/views/${CTX:cloudPortalView}/api/${CTX:cloudAPIID}.md" from the synthetic managed API Portal until status 404

  @ID_APIP_PORTAL_045 @layer:cloud-owned
  Scenario Outline: Managed API Portal routes refuse a request <credential>
    Given I obtain an APIP cloud console token
    When I send a "<method>" request <credential> to the managed API Portal path "<path>"
    Then the cloud API response status code should be 401

    Examples:
      | method | credential          | path                               |
      | POST   | without a token     | /managed-api-portals               |
      | GET    | without a token     | /managed-api-portals/absent-portal |
      | PUT    | without a token     | /managed-api-portals/absent-portal |
      | DELETE | without a token     | /managed-api-portals/absent-portal |
      | GET    | with a forged token | /managed-api-portals               |
      | POST   | with a forged token | /managed-api-portals               |
      | GET    | with a forged token | /managed-api-portals/absent-portal |
      | PUT    | with a forged token | /managed-api-portals/absent-portal |
      | DELETE | with a forged token | /managed-api-portals/absent-portal |

  @ID_APIP_PORTAL_046 @layer:cloud-owned
  Scenario Outline: The rotate-key route refuses a request <credential>
    Given I obtain an APIP cloud console token
    When I send a "<method>" request <credential> to the managed API Portal path "<path>"
    Then the cloud API response status code should be 401

    Examples:
      | method | credential          | path                                          |
      | POST   | without a token     | /managed-api-portals/absent-portal/rotate-key |
      | POST   | with a forged token | /managed-api-portals/absent-portal/rotate-key |

  @ID_APIP_PORTAL_044 @layer:cloud-owned
  Scenario: A deleted portal stops serving
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I request "/api-portal/health" from the synthetic managed API Portal
    Then the cloud API response status code should be 200
    When I delete the synthetic managed API Portal
    Then the managed API Portal deletion should be accepted
    And the synthetic managed API Portal should eventually be gone
    And the synthetic managed API Portal should no longer serve

  @ID_APIP_PORTAL_048 @layer:cloud-owned
  Scenario: The handle of a deleted portal can be used again
    Given I obtain an APIP cloud console token
    When I request a managed API Portal under the handle of the deleted synthetic managed API Portal
    Then the synthetic managed API Portal should be accepted for provisioning
