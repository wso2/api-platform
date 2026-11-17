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

@CONSOLE
Feature: The Management Portal provisions a gateway that connects with the token it shows
  As an organization administrator
  I want to register a gateway in the console and start it with the token the console gives me
  So that the gateway joins the platform and the console shows it connected

  @CONSOLE-017
  Scenario: Journey: an administrator provisions a gateway, starts it with the token the console shows, and the console shows it connected
    Given I generate a unique handle from "console-gateway" and store it as "gateway"
    When the user signs in to the Management Portal as the administrator
    And the user opens the Management Portal page "/organizations/default/gateways/new"
    And the user enters "${CTX:gateway}" as the "Name" in the Management Portal
    And the user enters "http://gateway-runtime-console:8080" as the "URL" in the Management Portal
    And the gateway "${CTX:gateway}" provisioned in the Management Portal is deleted after the scenario
    And the user clicks "Provision gateway" in the Management Portal
    Then the Management Portal shows "Gateway “${CTX:gateway}” provisioned."
    And the gateway "${CTX:gateway}" shows as "Not connected" in the Management Portal's gateway list
    # The console shows a registration token only when it generates one, once.
    When the user opens the Management Portal page "/organizations/default/gateways/${CTX:gateway}"
    And the user clicks "Reconfigure" in the Management Portal
    And the user confirms with "Generate new token" in the Management Portal
    And a gateway is started as "-console" with the registration token the Management Portal shows
    Then the gateway "${CTX:gateway}" shows as "Connected" in the Management Portal's gateway list
