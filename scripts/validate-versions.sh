#!/bin/bash
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

# validate-versions.sh - Validate version consistency across all files
# Usage: ./scripts/validate-versions.sh

set -e

echo "Validating version consistency..."
echo ""

ERRORS=0

# Read VERSION files
ROOT_VERSION=$(cat VERSION 2>/dev/null || echo "")
GATEWAY_VERSION=$(cat gateway/VERSION 2>/dev/null || echo "")
PLATFORM_API_VERSION=$(cat platform-api/VERSION 2>/dev/null || echo "")

echo "VERSION files:"
echo "  Root:         $ROOT_VERSION"
echo "  Gateway:      $GATEWAY_VERSION"
echo "  Platform API: $PLATFORM_API_VERSION"
echo ""

# Check docker-compose.yaml
if [ -f "gateway/docker-compose.yaml" ]; then
    echo "Checking docker-compose.yaml..."
    # Extract version tag after the image name (handles both wso2/* and ghcr.io/*)
    COMPOSE_CONTROLLER=$(grep "image:.*gateway-controller" gateway/docker-compose.yaml | sed 's/.*api-platform/gateway-controller://' | tr -d ' ')
    COMPOSE_POLICY=$(grep "image:.*policy-engine" gateway/docker-compose.yaml | sed 's/.*api-platform/policy-engine://' | tr -d ' ')
    COMPOSE_ROUTER=$(grep "image:.*gateway-router" gateway/docker-compose.yaml | sed 's/.*api-platform/gateway-router://' | tr -d ' ')

    echo "  Controller: $COMPOSE_CONTROLLER"
    echo "  Policy:     $COMPOSE_POLICY"
    echo "  Router:     $COMPOSE_ROUTER"

    if [ "$COMPOSE_CONTROLLER" != "v$GATEWAY_VERSION" ]; then
        echo "Controller version mismatch!"
        ERRORS=$((ERRORS + 1))
    fi

    if [ "$COMPOSE_POLICY" != "v$GATEWAY_VERSION" ]; then
        echo "Policy Engine version mismatch!"
        ERRORS=$((ERRORS + 1))
    fi

    if [ "$COMPOSE_ROUTER" != "v$GATEWAY_VERSION" ]; then
        echo "Router version mismatch!"
        ERRORS=$((ERRORS + 1))
    fi

    echo ""
fi

# Check Helm charts
if [ -f "kubernetes/helm/gateway-helm-chart/Chart.yaml" ]; then
    echo "Checking Helm charts..."
    HELM_APP_VERSION=$(grep "^appVersion:" kubernetes/helm/gateway-helm-chart/Chart.yaml | sed 's/appVersion: "//' | sed 's/"//')
    echo "  appVersion: $HELM_APP_VERSION"

    if [ "$HELM_APP_VERSION" != "$GATEWAY_VERSION" ]; then
        echo "Helm appVersion mismatch!"
        ERRORS=$((ERRORS + 1))
    fi

    echo ""
fi

if [ $ERRORS -eq 0 ]; then
    echo "All versions are consistent"
    exit 0
else
    echo "Found $ERRORS version inconsistencies"
    exit 1
fi
