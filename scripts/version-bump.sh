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

# version-bump.sh - Bump version numbers in VERSION files
# Usage: ./scripts/version-bump.sh <major|minor|patch> <component>

set -e

BUMP_TYPE=$1  # major, minor, patch
COMPONENT=$2  # gateway, platform-api, or root (empty)

if [ -z "$BUMP_TYPE" ]; then
    echo "Error: Bump type required (major, minor, patch)"
    echo "Usage: $0 <major|minor|patch> <component>"
    exit 1
fi

# Determine VERSION file location
if [ -z "$COMPONENT" ] || [ "$COMPONENT" = "root" ]; then
    VERSION_FILE="VERSION"
else
    VERSION_FILE="$COMPONENT/VERSION"
fi

if [ ! -f "$VERSION_FILE" ]; then
    echo "Error: VERSION file not found: $VERSION_FILE"
    exit 1
fi

# Read current version
CURRENT_VERSION=$(cat "$VERSION_FILE" | tr -d '[:space:]')

# Strip prerelease/build metadata
BASE_VERSION=$(echo "$CURRENT_VERSION" | sed 's/-.*$//')

# Parse version components
IFS='.' read -r MAJOR MINOR PATCH <<< "$BASE_VERSION"

# Bump version
case "$BUMP_TYPE" in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    patch)
        PATCH=$((PATCH + 1))
        ;;
    next-dev)
        # Bump minor version and always add SNAPSHOT
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    *)
        echo "Error: Invalid bump type: $BUMP_TYPE"
        echo "Valid types: major, minor, patch, next-dev"
        exit 1
        ;;
esac

NEW_VERSION="$MAJOR.$MINOR.$PATCH"

# Handle SNAPSHOT suffix
if [ "$BUMP_TYPE" = "next-dev" ]; then
    # Always add SNAPSHOT for next-dev
    NEW_VERSION="$NEW_VERSION-SNAPSHOT"
elif [[ "$CURRENT_VERSION" == *"-SNAPSHOT" ]]; then
    # Preserve SNAPSHOT for other bump types if it existed
    NEW_VERSION="$NEW_VERSION-SNAPSHOT"
fi

# Write new version
echo "$NEW_VERSION" > "$VERSION_FILE"
echo "✅ Bumped $VERSION_FILE: $CURRENT_VERSION → $NEW_VERSION"
