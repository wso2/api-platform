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

# update-docker-compose.sh - Update image tags in docker-compose files
# Usage: ./scripts/update-docker-compose.sh <component> <version>

set -e

COMPONENT=$1
VERSION=$2

if [ -z "$COMPONENT" ] || [ -z "$VERSION" ]; then
    echo "Usage: $0 <component> <version>"
    exit 1
fi

COMPOSE_FILE="gateway/docker-compose.yaml"

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "Warning: docker-compose.yaml not found at $COMPOSE_FILE"
    return 0
fi

if [ "$COMPONENT" = "gateway" ]; then
    # Update all gateway component images
    # Use macOS-compatible sed syntax with pattern matching for any registry
    sed -i '' \
        -e "s|image: .*/api-platform-gateway-controller:.*|image: ghcr.io/renuka-fernando/api-platform-gateway-controller:v$VERSION|" \
        -e "s|image: .*/api-platform-policy-engine:.*|image: ghcr.io/renuka-fernando/api-platform-policy-engine:v$VERSION|" \
        -e "s|image: .*/api-platform-gateway-router:.*|image: ghcr.io/renuka-fernando/api-platform-gateway-router:v$VERSION|" \
        "$COMPOSE_FILE"
    echo "✅ Updated docker-compose.yaml with gateway version v$VERSION (GHCR)"
elif [ "$COMPONENT" = "platform-api" ]; then
    # Update platform-api image (if present in compose)
    sed -i '' \
        -e "s|image: .*/api-platform-platform-api:.*|image: ghcr.io/renuka-fernando/api-platform-platform-api:v$VERSION|" \
        "$COMPOSE_FILE"
    echo "✅ Updated docker-compose.yaml with platform-api version v$VERSION (GHCR)"
fi
