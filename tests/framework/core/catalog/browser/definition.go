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

package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// PlaywrightVersion is the ONE pin for the browser stack. Three things must carry the same
// version or remote connections misbehave in ways that name nothing useful: the playwright-go
// module (its minor encodes this: v0.6201.x == 1.62.1), the browser image tag, and the
// run-server the container starts. PlaywrightVersionMatchesModule enforces the first;
// deriving image and command from this constant enforces the rest.
const PlaywrightVersion = "1.62.1"

// EnvImageBrowser overrides the browser image, for CI to pin a mirror.
const EnvImageBrowser = "IT_IMAGE_BROWSER"

const svcBrowser = "browser"

// Browser is the Playwright server the UI suite's scenarios drive, ON the block's network.
//
// In-network rather than a host browser, deliberately: portals and the control plane address
// each other by alias, and only a browser on the same network resolves those names the way a
// deployed user's browser resolves the product's public hostnames. It also removes every
// host-mapped-port dependency except the one ws endpoint, and makes macOS dev and Linux CI
// byte-identical. The suite connects with playwright-go over ws:// and runs one
// BrowserContext per scenario.
func Browser() *components.Definition {
	return &components.Definition{
		Name:  svcBrowser,
		Image: shared.Image(EnvImageBrowser, imgPlaywright()),
		Alias: svcBrowser,
		Cmd: []string{
			"npx", "-y", "playwright@" + PlaywrightVersion,
			"run-server", "--port", "3000", "--host", "0.0.0.0",
		},
		Endpoints: []components.Endpoint{
			{Name: "ws", Port: 3000, Scheme: "http", AwaitListening: true},
		},
		// Sized for a shared browser plus a handful of concurrent contexts (~400MB + 50MB
		// per context); raise alongside runner parallelism, not instead of measuring.
		Limits: components.ResourceLimits{CPUs: 2, MemoryMB: 1500},
	}
}

func imgPlaywright() string {
	return "mcr.microsoft.com/playwright:v" + PlaywrightVersion + "-noble"
}

// PlaywrightWSPath is the ws endpoint path run-server listens on.
const PlaywrightWSPath = "/"

// PlaywrightVersionMatchesModule verifies the playwright-go requirement in this module's
// go.mod encodes PlaywrightVersion. The module's versioning scheme is v0.<MMmm>.<patch>
// where MM is the upstream minor and mm the upstream patch: v0.6201.x == Playwright 1.62.1.
// Read from go.mod rather than build info, which omits test-only imports.
func PlaywrightVersionMatchesModule() error {
	root, ok := shared.RepoRootFromCallerFile()
	if !ok {
		return fmt.Errorf("catalog: cannot locate the repository root to verify the playwright pin")
	}
	raw, err := os.ReadFile(filepath.Join(root, "tests", "framework", "go.mod"))
	if err != nil {
		return fmt.Errorf("catalog: reading go.mod to verify the playwright pin: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "github.com/mxschmitt/playwright-go" {
			continue
		}
		derived, derr := playwrightVersionFromModule(fields[1])
		if derr != nil {
			return derr
		}
		if derived != PlaywrightVersion {
			return fmt.Errorf(
				"catalog: playwright-go %s encodes Playwright %s but PlaywrightVersion pins %s — "+
					"update the constant and the browser image together",
				fields[1], derived, PlaywrightVersion)
		}
		return nil
	}
	return fmt.Errorf("catalog: playwright-go is not required by go.mod; the browser component cannot be driven")
}

// playwrightVersionFromModule turns v0.6201.1 into 1.62.1.
func playwrightVersionFromModule(module string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(module, "v"), ".")
	if len(parts) < 2 || len(parts[1]) != 4 {
		return "", fmt.Errorf("catalog: unexpected playwright-go version %q", module)
	}
	minor, err1 := strconv.Atoi(parts[1][:2])
	patch, err2 := strconv.Atoi(parts[1][2:])
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("catalog: unexpected playwright-go version %q", module)
	}
	return fmt.Sprintf("1.%d.%d", minor, patch), nil
}
