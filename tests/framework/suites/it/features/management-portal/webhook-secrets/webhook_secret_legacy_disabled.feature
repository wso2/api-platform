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

@platform-api-webhook-legacy-disabled
Feature: Platform API refuses a hash-less event when the legacy shared-secret fallback is off
  As an operator who has not opted into the legacy hash-less webhook delivery path
  I want an event the control plane cannot trace to a specific secret refused outright
  So that a portal's events are never silently accepted against the shared secret by default

  Scenario: An event with no secret_hash is rejected when legacy_shared_secret is left at its default
    Given I generate a unique resource name from "wsec-legacy-off" and store it as "meterId"
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Should not be created","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404
