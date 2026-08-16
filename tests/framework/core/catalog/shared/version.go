/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	gatewayVersionFallback     = "1.2.0-SNAPSHOT"
	platformAPIVersionFallback = "0.17.0-SNAPSHOT"
	apiPortalVersionFallback   = "1.0.0-SNAPSHOT"
	aiWorkspaceVersionFallback = "1.1.0-SNAPSHOT"
)

var gatewayVersion = sync.OnceValue(func() string { return versionFile("gateway", gatewayVersionFallback) })
var platformAPIVersion = sync.OnceValue(func() string { return versionFile("platform-api", platformAPIVersionFallback) })
var apiPortalVersion = sync.OnceValue(func() string {
	return versionFile(filepath.Join("portals", "api-portal"), apiPortalVersionFallback)
})
var aiWorkspaceVersion = sync.OnceValue(func() string {
	return versionFile(filepath.Join("portals", "ai-workspace"), aiWorkspaceVersionFallback)
})

// SourceVersion returns the version declared by a catalog product's VERSION file.
func SourceVersion(component string) (string, bool) {
	switch component {
	case "platform-gateway":
		return gatewayVersion(), true
	case "platform-api":
		return platformAPIVersion(), true
	case "api-portal":
		return apiPortalVersion(), true
	case "ai-workspace":
		return aiWorkspaceVersion(), true
	default:
		return "", false
	}
}

func versionFile(dir, fallback string) string {
	root, ok := RepoRootFromCallerFile()
	if !ok {
		return fallback
	}
	raw, err := os.ReadFile(filepath.Join(root, dir, "VERSION"))
	if err != nil {
		return fallback
	}
	if value := strings.TrimSpace(string(raw)); value != "" {
		return value
	}
	return fallback
}

// GatewayControllerImage returns the controller image for the current source version.
func GatewayControllerImage() string {
	return "ghcr.io/wso2/api-platform/gateway-controller:" + gatewayVersion()
}

// GatewayRuntimeImage returns the runtime image for the current source version.
func GatewayRuntimeImage() string {
	return "ghcr.io/wso2/api-platform/gateway-runtime:" + gatewayVersion()
}

// GatewayControllerRunImage returns the controller image used by the suite.
func GatewayControllerRunImage() string { return GatewayControllerImage() }

// GatewayRuntimeRunImage returns the runtime image used by the suite.
func GatewayRuntimeRunImage() string { return GatewayRuntimeImage() }

// PlatformAPIImage returns the Platform API image for its source version.
func PlatformAPIImage() string {
	return "ghcr.io/wso2/api-platform/platform-api:" + platformAPIVersion()
}

// APIPortalImage returns the API Portal image for its source version.
func APIPortalImage() string {
	return "ghcr.io/wso2/api-platform/api-portal:" + apiPortalVersion()
}

// AIWorkspaceImage returns the AI Workspace image for its source version.
func AIWorkspaceImage() string {
	return "ghcr.io/wso2/api-platform/ai-workspace:" + aiWorkspaceVersion()
}

// RepoRootFromCallerFile locates the checkout containing gateway/VERSION.
func RepoRootFromCallerFile() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "gateway", "VERSION")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
	}
}
