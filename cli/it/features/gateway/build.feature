# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: Gateway Build Command
    As a CLI user
    I want to build custom gateway images
    So that I can create gateways with custom policies

    Background:
        Given the CLI is available
        And Docker is available

    # =========================================
    # Build Tests
    # =========================================

    @BUILD-001
    Scenario: Build gateway with valid policy manifest but no PolicyHub access
        # Note: This test validates manifest parsing works, but PolicyHub is not accessible in the test environment
        When I run ap with arguments "gateway build -f resources/gateway/policy-manifest.yaml --docker-registry localhost:5000 --image-tag test --gateway-builder ghcr.io/wso2/api-platform/gateway-builder:0.3.0-SNAPSHOT"
        Then the command should fail
        And the output should contain "Validating Policy Manifest"

    @BUILD-002
    Scenario: Build gateway with invalid manifest
        When I run ap with arguments "gateway build -f resources/gateway/invalid.yaml --docker-registry localhost:5000 --image-tag test --gateway-builder ghcr.io/wso2/api-platform/gateway-builder:0.3.0-SNAPSHOT"
        Then the command should fail

    @BUILD-003
    Scenario: Build gateway with missing manifest file
        When I run ap with arguments "gateway build -f non-existent-manifest.yaml --docker-registry localhost:5000 --image-tag test --gateway-builder ghcr.io/wso2/api-platform/gateway-builder:0.3.0-SNAPSHOT"
        Then the command should fail
        And the output should contain "no such file"
