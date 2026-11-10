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

  @TC_APIP_GWCFG_003 @ID_APIP_GATEWAY_CONFIG_002
  Scenario: A setting that is not in the allowlist is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                                                      | value                                           | message                            |
      | gateway.config.not.an.editable.setting                                    | 1                                               | is not an editable gateway setting |
      | gateway.config.controller.storage.postgres.host                           | "db.invalid"                                    | is not an editable gateway setting |
      | gatewayId                                                                 | "another-gateway"                               | is not an editable gateway setting |
      | gateway.config                                                            | {"controller": {"logging": {"level": "debug"}}} | is not an editable gateway setting |
      | gateway.config.policy_configurations.ratelimit_v1.memory.max_entries      | 30000                                           | is not an editable gateway setting |
      | gateway.config.policy_configurations.ratelimit_v1.memory.cleanup_interval | "10m"                                           | is not an editable gateway setting |
      | gateway.controller.deployment.replicaCount                                | 2                                               | is not an editable gateway setting |
      | gateway.gatewayRuntime.deployment.replicaCount                            | 2                                               | is not an editable gateway setting |
      | gateway.controller.deployment.resources.requests.cpu                      | "100m"                                          | is not an editable gateway setting |
      | gateway.controller.deployment.resources.requests.memory                   | "256Mi"                                         | is not an editable gateway setting |
      | gateway.controller.deployment.resources.limits.cpu                        | "500m"                                          | is not an editable gateway setting |
      | gateway.controller.deployment.resources.limits.memory                     | "512Mi"                                         | is not an editable gateway setting |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_004 @ID_APIP_GATEWAY_CONFIG_003
  Scenario: An enum setting refuses a value outside its value set
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                       | value   | message                                 |
      | gateway.config.controller.logging.level    | verbose | must be one of debug, info, warn, error |
      | gateway.config.controller.logging.level    | "DEBUG" | must be one of debug, info, warn, error |
      | gateway.config.controller.logging.level    | 3       | must be one of debug, info, warn, error |
      | gateway.config.policy_engine.logging.level | trace   | must be one of debug, info, warn, error |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_005 @ID_APIP_GATEWAY_CONFIG_004
  Scenario: A boolean setting refuses a value that is not a boolean
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                                                           | value   | message               |
      | gateway.config.router.access_logs.enabled                                      | yes     | must be true or false |
      | gateway.config.router.access_logs.enabled                                      | "true"  | must be true or false |
      | gateway.config.router.access_logs.enabled                                      | 1       | must be true or false |
      | gateway.config.mcp.append_resource_path_to_backend                             | "false" | must be true or false |
      | gateway.config.policy_configurations.ratelimit_v1.headers.include_x_rate_limit | yes     | must be true or false |
      | gateway.config.policy_configurations.ratelimit_v1.headers.include_ietf         | 0       | must be true or false |
      | gateway.config.policy_configurations.ratelimit_v1.headers.include_retry_after  | "true"  | must be true or false |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_024 @ID_APIP_GATEWAY_CONFIG_016
  Scenario: An enum with display labels refuses a label in place of its value
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                                             | value       | message                                               |
      | gateway.config.router.http_listener.server_header_transformation | REPLACE     | must be one of Overwrite, Add if absent, Pass through |
      | gateway.config.router.http_listener.server_header_transformation | "Overwrite" | must be one of Overwrite, Add if absent, Pass through |
      | gateway.config.router.upstream.tls.minimum_protocol_version      | TLS1_1      | must be one of TLS 1.2, TLS 1.3                       |
      | gateway.config.router.upstream.tls.minimum_protocol_version      | "TLS 1.3"   | must be one of TLS 1.2, TLS 1.3                       |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_025 @ID_APIP_GATEWAY_CONFIG_017
  Scenario: An integer setting refuses a value outside its bounds or not whole
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                             | value | message                   |
      | gateway.config.api_key.api_keys_per_user_per_api | 0     | must be between 1 and 100 |
      | gateway.config.api_key.api_keys_per_user_per_api | 101   | must be between 1 and 100 |
      | gateway.config.api_key.api_keys_per_user_per_api | 2.5   | must be a whole number    |
      | gateway.config.api_key.api_keys_per_user_per_api | "10"  | must be a whole number    |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_026 @ID_APIP_GATEWAY_CONFIG_018
  Scenario: The server header value refuses what a header cannot carry unchanged
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                                    | value        | message                             |
      | gateway.config.router.http_listener.server_header_value | ""           | must be between 1 and 64 characters |
      | gateway.config.router.http_listener.server_header_value | "Acm\u00e9"  | must be printable ASCII             |
      | gateway.config.router.http_listener.server_header_value | "acme\tedge" | must be printable ASCII             |
      | gateway.config.router.http_listener.server_header_value | " acme"      | must not start or end with a space  |
      | gateway.config.router.http_listener.server_header_value | 42           | must be a string                    |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_012 @ID_APIP_GATEWAY_CONFIG_011
  Scenario: A string setting refuses a value longer than its character maximum
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write the gateway configuration path "gateway.config_toml" with a string of 8193 characters
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "gateway.config_toml"
    And the cloud API response body should contain "must be between 0 and 8192 characters"

  @TC_APIP_GWCFG_016 @ID_APIP_GATEWAY_CONFIG_014
  Scenario: Raw TOML with control characters or line separators is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                | value      | message                                                |
      | gateway.config_toml | "a\u0000b" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u0007b" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u001bb" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u007fb" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u2028b" | must not contain control characters or line separators |
      | gateway.config_toml | "a\u2029b" | must not contain control characters or line separators |
    Then every one of those writes should have been refused as invalid

  @TC_APIP_GWCFG_023 @ID_APIP_GATEWAY_CONFIG_015
  Scenario: A setting the gateway does not read is refused rather than accepted and dropped
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And the active APIP gateway's configuration should be readable
    When I write each of these gateway configuration values separately:
      | path                                                                         | value | message                            |
      | gateway.controller.logging.level                                             | debug | is not an editable gateway setting |
      | gateway.gatewayRuntime.deployment.resources.limits.cpu                       | "1"   | is not an editable gateway setting |
      | gateway.config.router.upstream.timeouts.connect                              | "5s"  | is not an editable gateway setting |
      | gateway.config.policy_configurations.llm_cost_ratelimit_v1.cost_scale_factor | 1     | is not an editable gateway setting |
    Then every one of those writes should have been refused as invalid
