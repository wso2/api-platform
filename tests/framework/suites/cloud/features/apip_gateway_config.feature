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
Feature: APIP cloud managed gateway configuration
  As an APIP cloud user
  I want to read and change the settings a managed gateway lets me edit
  So that only valid, allowlisted settings reach the gateway, and a change is neither lost nor misapplied

  @TC_APIP_GWCFG_001 @ID_APIP_GATEWAY_CONFIG_001
  Scenario: A gateway's configuration lists what it sets and what may be changed
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I read the configuration of the active APIP gateway
    Then the cloud API response status code should be 200
    And the gateway configuration should be a valid document

  @TC_APIP_GWCFG_002 @ID_APIP_GATEWAY_CONFIG_002
  Scenario: A written setting is reflected when the configuration is read
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
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

  @TC_APIP_GWCFG_003 @ID_APIP_GATEWAY_CONFIG_003
  Scenario: A setting that is not in the allowlist is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                            | value                                           | message                            |
      | gateway.config.not.an.editable.setting          | 1                                               | is not an editable gateway setting |
      | gateway.config.controller.storage.postgres.host | "db.invalid"                                    | is not an editable gateway setting |
      | gatewayId                                       | "another-gateway"                               | is not an editable gateway setting |
      | gateway.config                                  | {"controller": {"logging": {"level": "debug"}}} | is not an editable gateway setting |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_004 @ID_APIP_GATEWAY_CONFIG_004
  Scenario: An enum setting refuses a value outside its value set
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                       | value   | message                                 |
      | gateway.config.controller.logging.level    | verbose | must be one of debug, info, warn, error |
      | gateway.config.controller.logging.level    | "DEBUG" | must be one of debug, info, warn, error |
      | gateway.config.controller.logging.level    | 3       | must be one of debug, info, warn, error |
      | gateway.config.policy_engine.logging.level | trace   | must be one of debug, info, warn, error |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_005 @ID_APIP_GATEWAY_CONFIG_005
  Scenario: A boolean setting refuses a value that is not a boolean
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                      | value  | message               |
      | gateway.config.router.access_logs.enabled | yes    | must be true or false |
      | gateway.config.router.access_logs.enabled | "true" | must be true or false |
      | gateway.config.router.access_logs.enabled | 1      | must be true or false |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_006 @ID_APIP_GATEWAY_CONFIG_006
  Scenario: An integer setting refuses a value below its minimum
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                                 | value | message                          |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries | 999   | must be between 1000 and 1000000 |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries | 0     | must be between 1000 and 1000000 |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries | -1    | must be between 1000 and 1000000 |
      | gateway.controller.deployment.replicaCount                           | 0     | must be between 1 and 5          |
      | gateway.gatewayRuntime.deployment.replicaCount                       | 0     | must be between 1 and 5          |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_007 @ID_APIP_GATEWAY_CONFIG_007
  Scenario: An integer setting refuses a value above its maximum
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                                 | value   | message                          |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries | 1000001 | must be between 1000 and 1000000 |
      | gateway.controller.deployment.replicaCount                           | 6       | must be between 1 and 5          |
      | gateway.gatewayRuntime.deployment.replicaCount                       | 6       | must be between 1 and 5          |
      | gateway.gatewayRuntime.deployment.replicaCount                       | 100     | must be between 1 and 5          |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_008 @ID_APIP_GATEWAY_CONFIG_008
  Scenario: A duration setting refuses a value that is not a duration
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                                      | value    | message                         |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | soon     | must be a duration such as "5m" |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | "5"      | must be a duration such as "5m" |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | 5        | must be a duration such as "5m" |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | 1 minute | must be a duration such as "5m" |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_009 @ID_APIP_GATEWAY_CONFIG_009
  Scenario: A duration setting refuses a value outside its bounds
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                                      | value | message                    |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | 29s   | must be between 30s and 1h |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | 1h1s  | must be between 30s and 1h |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | 0s    | must be between 30s and 1h |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | -5m   | must be between 30s and 1h |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_010 @ID_APIP_GATEWAY_CONFIG_010
  Scenario: A quantity setting refuses a value that is not a Kubernetes quantity
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                    | value            | message                    |
      | gateway.controller.deployment.resources.requests.cpu    | lots             | must be a quantity such as |
      | gateway.controller.deployment.resources.requests.cpu    | 12 cores         | must be a quantity such as |
      | gateway.controller.deployment.resources.limits.cpu      | 500x             | must be a quantity such as |
      | gateway.controller.deployment.resources.requests.memory | 12               | must be a quantity such as |
      | gateway.controller.deployment.resources.limits.memory   | "lots of memory" | must be a quantity such as |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_011 @ID_APIP_GATEWAY_CONFIG_011
  Scenario: A quantity setting refuses a value outside its bounds
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                    | value  | message                       |
      | gateway.controller.deployment.resources.requests.cpu    | "10m"  | must be between 50m and 4     |
      | gateway.controller.deployment.resources.requests.cpu    | "5"    | must be between 50m and 4     |
      | gateway.controller.deployment.resources.requests.cpu    | "0"    | must be greater than zero     |
      | gateway.controller.deployment.resources.limits.cpu      | "-1"   | must be greater than zero     |
      | gateway.controller.deployment.resources.requests.memory | "64Mi" | must be between 128Mi and 8Gi |
      | gateway.controller.deployment.resources.limits.memory   | "9Gi"  | must be between 128Mi and 8Gi |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_012 @ID_APIP_GATEWAY_CONFIG_012
  Scenario: A string setting refuses a value longer than its character maximum
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write the gateway configuration path "gateway.config_toml" with a string of 8193 characters
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "gateway.config_toml"
    And the cloud API response body should contain "must be between 0 and 8192 characters"

  @TC_APIP_GWCFG_013 @ID_APIP_GATEWAY_CONFIG_013
  Scenario: A CPU request above the CPU limit is refused by the cross-field constraint
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write the gateway configuration with:
      | gateway.controller.deployment.resources.requests.cpu | "2" |
      | gateway.controller.deployment.resources.limits.cpu   | "1" |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "request"

  @TC_APIP_GWCFG_014 @ID_APIP_GATEWAY_CONFIG_014
  Scenario: A memory request above the memory limit is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write the gateway configuration with:
      | gateway.controller.deployment.resources.requests.memory | "2Gi" |
      | gateway.controller.deployment.resources.limits.memory   | "1Gi" |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "request"

  @TC_APIP_GWCFG_015 @ID_APIP_GATEWAY_CONFIG_015
  Scenario: A limit is judged against the request that is already stored
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I save the gateway configuration with:
      | gateway.controller.deployment.resources.requests.cpu | "1" |
      | gateway.controller.deployment.resources.limits.cpu   | "2" |
    Then the cloud API response status code should be 200
    When I write the gateway configuration with:
      | gateway.controller.deployment.resources.limits.cpu | "500m" |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "request"

  @TC_APIP_GWCFG_016 @ID_APIP_GATEWAY_CONFIG_016
  Scenario: Raw TOML with control characters or line separators is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                | value      | message                                                |
      | gateway.config_toml | "a\u0000b" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u0007b" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u001bb" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u007fb" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u2028b" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u2029b" | must not contain control characters or line separators |
    Then every one of those writes should have been refused as invalid

  # The platform does not parse the TOML it appends to the gateway's configuration (B2), so a section
  # the gateway already configures is accepted and the gateway no longer starts.
  @known-issue @TC_APIP_GWCFG_017 @ID_APIP_GATEWAY_CONFIG_017
  Scenario: Raw TOML that redefines a configured section is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write the gateway configuration with:
      | gateway.config_toml | [controller.logging]\nlevel = "debug" |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"

  # Same cause as @TC_APIP_GWCFG_017: nothing denies the controller's storage section (B2). The
  # host is one that cannot resolve, so an accepted write repoints the synthetic gateway at nothing.
  @known-issue @TC_APIP_GWCFG_018 @ID_APIP_GATEWAY_CONFIG_018
  Scenario: Raw TOML cannot repoint the gateway at another organisation's database
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write the gateway configuration with:
      | gateway.config_toml | [controller.storage.postgres]\nhost = "db.invalid" |
    Then the cloud API response status code should be a client error

  # The writers that lose a race are answered 500, not 409 (B28).
  @known-issue @TC_APIP_GWCFG_019 @ID_APIP_GATEWAY_CONFIG_019
  Scenario: Simultaneous writes to a gateway's configuration are not silently overwritten
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write the gateway configuration from 6 writers at once until one is rejected
    Then the cloud API response status code should be 409
    And the cloud API error code should be "conflict"

  @TC_APIP_GWCFG_022 @ID_APIP_GATEWAY_CONFIG_020
  Scenario: Clearing a setting returns the gateway to the platform default
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I save the gateway configuration with:
      | gateway.config.controller.logging.level | debug |
    Then the gateway configuration should report:
      | gateway.config.controller.logging.level | debug |
    When I save the gateway configuration with:
      | gateway.config.controller.logging.level | null |
    Then the cloud API response status code should be 200
    And the gateway configuration should no longer report "gateway.config.controller.logging.level"

  @TC_APIP_GWCFG_023 @ID_APIP_GATEWAY_CONFIG_021
  Scenario: A setting the gateway does not read is refused rather than accepted and dropped
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateway's configuration should become readable
    When I write each of these gateway configuration values separately:
      | path                                                                         | value | message                            |
      | gateway.controller.logging.level                                             | debug | is not an editable gateway setting |
      | gateway.gatewayRuntime.deployment.resources.limits.cpu                       | "1"   | is not an editable gateway setting |
      | gateway.config.router.upstream.timeouts.connect                              | "5s"  | is not an editable gateway setting |
      | gateway.config.policy_configurations.llm_cost_ratelimit_v1.cost_scale_factor | 1     | is not an editable gateway setting |
    Then every one of those writes should have been refused as invalid
