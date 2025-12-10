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

DOCKER_REGISTRY=${DOCKER_REGISTRY:-ghcr.io/wso2/api-platform}

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
    exit 0
fi

if [ "$COMPONENT" = "gateway" ]; then
    # Update Chart.yaml - appVersion
    sed -i.bak "s/^appVersion:.*/appVersion: \"$VERSION\"/" "$CHART_FILE"

    # Update values.yaml - repository AND tags for gateway components
    # Use macOS-compatible sed syntax
    sed -i.bak \
        -e "s|repository: .*/gateway-controller|repository: ${DOCKER_REGISTRY}/gateway-controller|" \
        -e "s|repository: .*/policy-engine|repository: ${DOCKER_REGISTRY}/policy-engine|" \
        -e "s|repository: .*/gateway-router|repository: ${DOCKER_REGISTRY}/gateway-router|" \
        -e "s|tag: [0-9].*$|tag: $VERSION|g" \
        "$VALUES_FILE"
    rm -f "$VALUES_FILE.bak"

    echo "Updated Helm charts with gateway version $VERSION"
fi
