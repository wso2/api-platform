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
Feature: APIP cloud project lifecycle
  As an APIP cloud user
  I want to read the projects of the suite and their deployment pipeline
  So that the project resolution, its refusals and its pipeline binding are verified

  @TC_APIP_PROJ_001 @ID_APIP_PROJECT_001
  Scenario: The test project of the suite is resolved and active
    Given I obtain an APIP cloud console token
    When I read the APIP test project
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | id     | ${CTX:cloudScenarioProject} |
      | status | active                      |

  @skip
  @TC_APIP_PROJ_002 @TC_APIP_PROJ_003 @ID_APIP_PROJECT_002
  Scenario: A project can be created and retrieved by handle
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    Then the synthetic APIP project should be retrievable by handle

  @skip
  @TC_APIP_PROJ_006 @ID_APIP_PROJECT_003
  Scenario: A project whose handle is already in use is rejected
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    And I create the synthetic APIP project again under the same handle
    Then the cloud API response status code should be 409
    And the cloud API error code should be "PROJECT_EXISTS"

  @skip
  @TC_APIP_PROJ_007 @ID_APIP_PROJECT_004
  Scenario Outline: A project whose handle breaks the naming rules is rejected and leaves nothing behind
    Given I obtain an APIP cloud console token
    When I request a synthetic APIP project whose handle is "<defect>"
    Then the cloud API response status code should be 400
    And the rejected APIP project should not exist

    Examples:
      | defect                         |
      | uppercase                      |
      | containing an underscore       |
      | starting with a hyphen         |
      | ending with a hyphen           |
      | containing consecutive hyphens |
      | longer than 40 characters      |

  @TC_APIP_PROJ_009 @ID_APIP_PROJECT_006
  Scenario: Reading a project that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I read an APIP project that does not exist
    Then the cloud API response status code should be 404
    And the cloud API error code should be "PROJECT_NOT_FOUND"

  @skip
  @TC_APIP_PROJ_011 @ID_APIP_PROJECT_008
  Scenario: A new project is active without any manual intervention
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    Then the cloud API response should report:
      | status | active |
    And the synthetic APIP project should become active

  @TC_APIP_PROJ_012 @ID_APIP_PROJECT_009
  Scenario: The deployment pipeline a project is bound to is readable and listed
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    When I read the deployment pipeline of the APIP project
    Then the cloud API response status code should be 200
    And the deployment pipeline the APIP project is bound to should be listed by the platform

  @skip
  @TC_APIP_PROJ_014 @ID_APIP_PROJECT_011
  Scenario: A project that was never bound explicitly reads back the default pipeline
    Given I obtain an APIP cloud console token
    When I create a synthetic APIP project
    And I read the deployment pipeline of the APIP project
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | pipeline | default |
