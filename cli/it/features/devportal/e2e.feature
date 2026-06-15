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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

Feature: DevPortal End-to-End Publish Flow
    As a CLI user
    I want to build an API project and publish it to the developer portal
    So that I can generate API keys, manage subscription plans, and subscribe

    Background:
        Given the CLI is available
        And the devportal is running
        And I have a devportal named "dp-e2e" configured
        And I target organization "1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"

    # One ordered scenario: the generated API ID is captured at publish time and
    # reused by the get / api-key / subscription steps. The subscription plan is
    # created BEFORE the API is published, because publish fails if a policy named
    # in the API manifest does not already exist in the org. The subscription then
    # uses that created plan (it-platinum) rather than a seeded one.
    @DP-E2E-001
    Scenario: Build, publish, key, plan, and subscribe end to end
        When I build the echo API project
        And I publish the subscription plan "resources/devportal/subscription-plan.yaml"
        And I publish the built API
        Then the output should contain "published"
        And the published API should be retrievable

        When I generate an API key named "echo-it-key" for the published API
        Then the command should succeed

        When I create a subscription for the published API with plan "it-platinum"
        Then the command should succeed
