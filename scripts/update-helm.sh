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

# update-helm.sh - Update Helm chart versions
# Usage: ./scripts/update-helm.sh <component> <version>

set -e

COMPONENT=$1
VERSION=$2

if [ -z "$COMPONENT" ] || [ -z "$VERSION" ]; then
    echo "Usage: $0 <component> <version>"
    exit 1
fi

CHART_FILE="kubernetes/helm/gateway-helm-chart/Chart.yaml"
VALUES_FILE="kubernetes/helm/gateway-helm-chart/values.yaml"

if [ ! -f "$CHART_FILE" ] || [ ! -f "$VALUES_FILE" ]; then
    echo "Warning: Helm chart files not found"
    return 0
fi

if [ "$COMPONENT" = "gateway" ]; then
    # Update Chart.yaml - appVersion
    sed -i '' "s/^appVersion:.*/appVersion: \"$VERSION\"/" "$CHART_FILE"

    # Update values.yaml - repository AND tags for gateway components
    # Use macOS-compatible sed syntax
    sed -i '' \
        -e "s|repository: .*/api-platform-gateway-controller|repository: ghcr.io/renuka-fernando/api-platform-gateway-controller|" \
        -e "s|repository: .*/api-platform-policy-engine|repository: ghcr.io/renuka-fernando/api-platform-policy-engine|" \
        -e "s|repository: .*/api-platform-gateway-router|repository: ghcr.io/renuka-fernando/api-platform-gateway-router|" \
        -e "s|tag: v[0-9].*$|tag: v$VERSION|g" \
        "$VALUES_FILE"

    echo "✅ Updated Helm charts with gateway version $VERSION (GHCR)"
fi
