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
Feature: APIP cloud managed gateway lifecycle
  As an APIP cloud user
  I want to request, read, update, deploy to and delete managed gateways
  So that the gateways an environment offers are the ones that were registered for it, and a refused request registers nothing

  @TC_APIP_GW_001 @ID_APIP_GATEWAY_001
  Scenario: The organisation's managed gateways are listed with their status
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I list the managed APIP gateways
    Then the cloud API response status code should be 200
    And the listed managed gateways should each report an identity, an environment and a status
    And the listed managed gateways should include the active gateway

  @TC_APIP_GW_002 @ID_APIP_GATEWAY_002
  Scenario: An existing environment can be selected to host a managed gateway
    Given I obtain an APIP cloud console token
    When I select an existing APIP environment for a managed gateway
    Then the selected APIP environment should be listed by the platform

  @TC_APIP_GW_009 @ID_APIP_GATEWAY_003
  Scenario: A managed gateway cannot be created in an environment that does not exist
    Given I obtain an APIP cloud console token
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | syn-absent-environment |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "syn-absent-environment"

  # Product issue B26: the platform refuses the version, but the integrator's 502 reaches the console as 500, not 400.
  @known-issue @TC_APIP_GW_010 @ID_APIP_GATEWAY_004
  Scenario Outline: A version that is neither major.minor nor CalVer is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | version     | <version>                      |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"

    Examples:
      | version     |
      | latest      |
      | sha-3f2a9c1 |
      | v1.2        |
      | 1.2.3       |
      | 1           |
      | 2026-05-13  |

  @TC_APIP_GW_019 @ID_APIP_GATEWAY_005
  Scenario: Reading a managed gateway that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I send an authenticated "GET" request to the APIP cloud path "/managed-gateways/no-such-gateway"
    Then the cloud API response status code should be 404
    And the cloud API error code should be "not_found"

  # The gateways are requested with a name and an environment only, so nothing optional is sent.
  @TC_APIP_GW_003 @TC_APIP_GW_004 @TC_APIP_GW_008 @TC_APIP_GW_011 @TC_APIP_GW_014 @TC_APIP_GW_015 @TC_APIP_GW_016 @TC_APIP_GW_017 @TC_APIP_PROV_001 @TC_APIP_PROV_003 @TC_APIP_PROV_011 @ID_APIP_GATEWAY_006
  Scenario: A gateway requested with only a name and an environment is built, connects and reports its version, and the name can be used in another environment
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And I create a synthetic non-production APIP environment that lasts for the runner
    And I choose a synthetic managed APIP gateway name at the handle limit of the active gateway's environment and the synthetic environment
    When I request a synthetic managed APIP gateway that lasts for the runner with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | displayName | ${CTX:cloudManagedGatewayName} |
      | handle      | ${CTX:cloudManagedGatewayName} |
    Then the managed gateway request should be accepted
    When I request a synthetic managed APIP gateway that lasts for the runner with:
      | environment | ${CTX:cloudEnvironmentName}    |
      | displayName | ${CTX:cloudManagedGatewayName} |
      | handle      | ${CTX:cloudManagedGatewayName} |
    Then the managed gateway request should be accepted
    Given I work with synthetic managed APIP gateway 1
    Then the managed APIP gateway should report a recorded provisioning state on its first read
    And the managed APIP gateway should not report its controller as connected yet
    And the managed APIP gateway should report no version
    And the managed APIP gateway should be retrievable through the managed-gateway endpoint
    And the managed APIP gateway should be bound to the environment it was created in
    And the managed APIP gateway status should progress from provisioning to built without failing
    And synthetic managed APIP gateway 1 should become active
    And the managed APIP gateway should report its controller as connected
    And the managed APIP gateway should report a version
    And the managed APIP gateway endpoint should answer requests

  @TC_APIP_PROV_005 @TC_APIP_PROV_007 @ID_APIP_GATEWAY_007
  Scenario: A gateway created in a new environment is built and connects, and handles at the length limit still receive a routable service
    Given I obtain an APIP cloud console token
    And I work with synthetic managed APIP gateway 2
    Then synthetic managed APIP gateway 2 should become active
    And the managed APIP gateway should be bound to the environment it was created in
    And the managed APIP gateway should report its controller as connected
    And the managed APIP gateway endpoint should answer requests
    When I work with synthetic managed APIP gateway 1
    Then the managed APIP gateway endpoint should answer requests

  # Product issue B27: a repeated name is answered 201 with a second gateway under a suffixed id, not 409.
  @known-issue @TC_APIP_GW_007 @TC_APIP_PROV_002 @ID_APIP_GATEWAY_008
  Scenario: A gateway name already used in the environment is refused
    Given I obtain an APIP cloud console token
    And I work with synthetic managed APIP gateway 1
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudManagedGatewayEnvironment} |
    Then the cloud API response status code should be 409
    And the cloud API error code should be "conflict"
    And the environment should hold exactly one managed gateway with the chosen name
    And the managed APIP gateway status should progress from provisioning to built without failing

  @TC_APIP_GW_013 @ID_APIP_GATEWAY_009
  Scenario: Updating a gateway's metadata is reflected when it is read
    Given I obtain an APIP cloud console token
    And I work with synthetic managed APIP gateway 1
    When I update the managed APIP gateway with:
      | description | Updated by the APIP cloud integration test |
      | isCritical  | true                                       |
      | properties  | {"owner": "cloud-it"}                      |
    Then the cloud API response status code should be 200
    And the managed APIP gateway should report:
      | description      | Updated by the APIP cloud integration test |
      | isCritical       | true                                       |
      | properties.owner | cloud-it                                   |
    And the update should have left the managed APIP gateway's identity unchanged
    When I update the managed APIP gateway with:
      | description | Restored by the APIP cloud integration test |
      | isCritical  | false                                       |
    Then the cloud API response status code should be 200

  @TC_APIP_DEP_010 @ID_APIP_GATEWAY_010
  Scenario: A deployment that names fewer gateways than the environment already runs the API on is refused
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And I work with synthetic managed APIP gateway 1
    And I create a synthetic REST API in the default project
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                              |
      | gatewayIds  | ${CTX:cloudGatewayID},${CTX:cloudSyntheticManagedGateway1} |
    Then the cloud API response status code should be 201
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 409
    And the cloud API error code should be "gateway_set_incomplete"
    And the cloud API response body should contain "${CTX:cloudSyntheticManagedGateway1}"

  # The default of the environment is handed to the gateway the runner built, so the refusal is
  # asked of a gateway that can be given up, and handed back afterwards.
  @TC_APIP_GW_012 @ID_APIP_GATEWAY_011
  Scenario: The default gateway of an environment cannot be deleted while others exist
    Given I obtain an APIP cloud console token
    And I work with synthetic managed APIP gateway 1
    When I make the managed APIP gateway the default of its environment
    Then the cloud API response status code should be 200
    And the managed APIP gateway should be the default of its environment
    When I request the deletion of the managed APIP gateway
    Then the cloud API response status code should be 409
    And the cloud API error code should be "gateway_is_default"
    And the cloud API response body should contain "${CTX:cloudManagedGatewayEnvironment}"
    And the managed APIP gateway should be the default of its environment
    When I hand the default of the environment back to the gateway that held it
    Then synthetic managed APIP gateway 1 should still be retrievable

  # Product issue: the recreated gateway gets a suffixed id (`-xxxx`) because the deleted gateway's name is not released yet.
  @known-issue @TC_APIP_GW_005 @TC_APIP_GW_006 @TC_APIP_PROV_004 @TC_APIP_PROV_006 @ID_APIP_GATEWAY_012
  Scenario: A gateway is deployed to and deleted on its own, and its name is free again once it is gone
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And I work with synthetic managed APIP gateway 1
    And I create a synthetic REST API in the default project
    When I deploy a build of the synthetic REST API to synthetic managed APIP gateway 1
    Then the synthetic REST API should be deployed on synthetic managed APIP gateway 1
    And the synthetic REST API should not be deployed on the active gateway
    When I undeploy the synthetic REST API from synthetic managed APIP gateway 1
    Then the synthetic REST API should not be deployed on synthetic managed APIP gateway 1
    When I delete the managed APIP gateway
    Then the managed APIP gateway should eventually be absent
    And the active APIP gateway should still be active
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudManagedGatewayEnvironment} |
    Then the managed gateway request should be accepted
    And the managed APIP gateway should be registered under the id its name gives it
    And the environment should hold exactly one managed gateway with the chosen name

  # The settings below are written to gateway 2, the gateway of the runner's own environment, which
  # nothing else uses once the scenarios above have run: a write makes the gateway apply its release
  # again, so it is never made to the shared gateway.
  @TC_APIP_GWCFG_002 @ID_APIP_GATEWAY_013
  Scenario: A written setting is reflected when the configuration is read
    Given I obtain an APIP cloud console token
    And I work with the configuration of synthetic managed APIP gateway 2
    And the configuration of the selected gateway should become readable
    When I save the gateway configuration with:
      | gateway.config.controller.logging.level                              | debug |
      | gateway.config.router.access_logs.enabled                            | false |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries | 5000  |
    Then the cloud API response status code should be 200
    And the gateway configuration phase should be "applying"
    And the gateway configuration should report:
      | gateway.config.controller.logging.level                              | debug |
      | gateway.config.router.access_logs.enabled                            | false |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries | 5000  |

  @TC_APIP_GWCFG_015 @ID_APIP_GATEWAY_014
  Scenario: A limit is judged against the request that is already stored
    Given I obtain an APIP cloud console token
    And I work with the configuration of synthetic managed APIP gateway 2
    And the configuration of the selected gateway should become readable
    When I save the gateway configuration with:
      | gateway.controller.deployment.resources.requests.cpu | "1" |
      | gateway.controller.deployment.resources.limits.cpu   | "2" |
    Then the cloud API response status code should be 200
    When I write the gateway configuration with:
      | gateway.controller.deployment.resources.limits.cpu | "500m" |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "request"

  @TC_APIP_GWCFG_022 @ID_APIP_GATEWAY_015
  Scenario: Clearing a setting returns the gateway to the platform default
    Given I obtain an APIP cloud console token
    And I work with the configuration of synthetic managed APIP gateway 2
    And the configuration of the selected gateway should become readable
    When I save the gateway configuration with:
      | gateway.config.controller.logging.level | debug |
    Then the gateway configuration should report:
      | gateway.config.controller.logging.level | debug |
    When I save the gateway configuration with:
      | gateway.config.controller.logging.level | null |
    Then the cloud API response status code should be 200
    And the gateway configuration should no longer report "gateway.config.controller.logging.level"

  # Product issue B2: the platform does not parse the raw TOML, so a redefined section is accepted (200), not refused (400).
  @known-issue @TC_APIP_GWCFG_017 @ID_APIP_GATEWAY_016
  Scenario: Raw TOML that redefines a configured section is refused
    Given I obtain an APIP cloud console token
    And I work with the configuration of synthetic managed APIP gateway 2
    And the configuration of the selected gateway should become readable
    When I write the gateway configuration with:
      | gateway.config_toml | [controller.logging]\nlevel = "debug" |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"

  # Product issue B2: nothing denies the controller's storage section, so the write is accepted (200), not refused.
  @known-issue @TC_APIP_GWCFG_018 @ID_APIP_GATEWAY_017
  Scenario: Raw TOML cannot repoint the gateway at another organisation's database
    Given I obtain an APIP cloud console token
    And I work with the configuration of synthetic managed APIP gateway 2
    And the configuration of the selected gateway should become readable
    When I write the gateway configuration with:
      | gateway.config_toml | [controller.storage.postgres]\nhost = "db.invalid" |
    Then the cloud API response status code should be a client error

  # Product issue B28: the writers that lose the race are answered 500, not 409.
  @known-issue @TC_APIP_GWCFG_019 @ID_APIP_GATEWAY_018
  Scenario: Simultaneous writes to a gateway's configuration are not silently overwritten
    Given I obtain an APIP cloud console token
    And I work with the configuration of synthetic managed APIP gateway 2
    And the configuration of the selected gateway should become readable
    When I write the gateway configuration from 6 writers at once until one is rejected
    Then the cloud API response status code should be 409
    And the cloud API error code should be "conflict"
