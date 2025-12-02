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

# release.sh - Complete release workflow for a component
# Usage: ./scripts/release.sh <component> <version>

set -e

COMPONENT=$1  # gateway, platform-api, etc.
VERSION=$2

if [ -z "$COMPONENT" ] || [ -z "$VERSION" ]; then
    echo "Usage: $0 <component> <version>"
    echo "Example: $0 gateway 1.0.0"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

echo "========================================"
echo "Releasing $COMPONENT version $VERSION"
echo "========================================"

# Step 1: Validate version format
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    echo "❌ Error: Invalid version format: $VERSION"
    echo "Expected: X.Y.Z or X.Y.Z-prerelease"
    exit 1
fi

# Step 2: Update VERSION file
if [ "$COMPONENT" = "gateway" ]; then
    echo "$VERSION" > gateway/VERSION
    echo "✅ Updated gateway/VERSION"
elif [ "$COMPONENT" = "platform-api" ]; then
    echo "$VERSION" > platform-api/VERSION
    echo "✅ Updated platform-api/VERSION"
else
    echo "❌ Error: Unknown component: $COMPONENT"
    exit 1
fi

# Step 3: Update docker-compose.yaml
echo "Updating docker-compose.yaml..."
bash "$SCRIPT_DIR/update-docker-compose.sh" "$COMPONENT" "$VERSION"

# Step 4: Update Helm charts
echo "Updating Helm charts..."
bash "$SCRIPT_DIR/update-helm.sh" "$COMPONENT" "$VERSION"

# Step 5: Build Docker images
echo "Building Docker images..."
if [ "$COMPONENT" = "gateway" ]; then
    make build-gateway
elif [ "$COMPONENT" = "platform-api" ]; then
    make build-platform-api
fi

# Step 6: Run tests
echo "Running tests..."
if [ "$COMPONENT" = "gateway" ]; then
    make test-gateway || {
        echo "❌ Tests failed! Please fix the tests before releasing."
        exit 1
    }
elif [ "$COMPONENT" = "platform-api" ]; then
    make test-platform-api || {
        echo "❌ Tests failed! Please fix the tests before releasing."
        exit 1
    }
fi

# Step 7: Git operations
echo "Creating git commit..."
git add "${COMPONENT}/VERSION"
git add "gateway/docker-compose.yaml" 2>/dev/null || true
git add "kubernetes/helm/" 2>/dev/null || true

git commit -m "Release $COMPONENT version $VERSION

- Update VERSION to $VERSION
- Update Docker images to $VERSION
- Update Helm charts to $VERSION"

# Step 8: Create git tag
TAG_NAME="$COMPONENT/v$VERSION"
echo "Creating tag: $TAG_NAME"
git tag -a "$TAG_NAME" -m "Release $COMPONENT version $VERSION"

echo ""
echo "========================================"
echo "✅ Release $COMPONENT $VERSION completed"
echo "========================================"
echo ""
echo "Next steps:"
echo "  1. Review the changes: git show HEAD"
echo "  2. Push images: make push-$COMPONENT"
echo "  3. Push to git: git push origin $(git branch --show-current)"
echo "  4. Push tag: git push origin $TAG_NAME"
