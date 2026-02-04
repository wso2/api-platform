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

DOCKER_REGISTRY=${DOCKER_REGISTRY:-ghcr.io/wso2/api-platform}

COMPONENT=$1
VERSION=$2

if [ -z "$COMPONENT" ] || [ -z "$VERSION" ]; then
    echo "Usage: $0 <component> <version>"
    exit 1
fi

COMPOSE_FILE="gateway/docker-compose.yaml"
IT_COMPOSE_FILE="gateway/it/docker-compose.test.yaml"

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "Warning: docker-compose.yaml not found at $COMPOSE_FILE"
    exit 0
fi

if [ "$COMPONENT" = "gateway" ]; then
    # Update all gateway component images in main docker-compose.yaml
    # Use macOS-compatible sed syntax with pattern matching for any registry
    sed -i -i.bak \
        -e "s|image: .*/gateway-controller:.*|image: ${DOCKER_REGISTRY}/gateway-controller:$VERSION|" \
        -e "s|image: .*/policy-engine:.*|image: ${DOCKER_REGISTRY}/policy-engine:$VERSION|" \
        -e "s|image: .*/gateway-router:.*|image: ${DOCKER_REGISTRY}/gateway-router:$VERSION|" \
        "$COMPOSE_FILE"
    rm -f "$COMPOSE_FILE.bak"
    echo "Updated $COMPOSE_FILE with gateway version $VERSION"

    # Update integration test docker-compose.yaml if it exists
    if [ -f "$IT_COMPOSE_FILE" ]; then
        sed -i -i.bak \
            -e "s|image: .*/gateway-controller-coverage:.*|image: ${DOCKER_REGISTRY}/gateway-controller-coverage:$VERSION|" \
            -e "s|image: .*/policy-engine-coverage:.*|image: ${DOCKER_REGISTRY}/policy-engine-coverage:$VERSION|" \
            -e "s|image: .*/gateway-router:.*|image: ${DOCKER_REGISTRY}/gateway-router:$VERSION|" \
            "$IT_COMPOSE_FILE"
        rm -f "$IT_COMPOSE_FILE.bak"
        echo "Updated $IT_COMPOSE_FILE with gateway version $VERSION"
    fi
elif [ "$COMPONENT" = "platform-api" ]; then
    # Update platform-api image (if present in compose)
    sed -i -i.bak \
        -e "s|image: .*/platform-api:.*|image: ${DOCKER_REGISTRY}/platform-api:$VERSION|" \
        "$COMPOSE_FILE"
    rm -f "$COMPOSE_FILE.bak"
    echo "Updated $COMPOSE_FILE with platform-api version $VERSION"
fi
