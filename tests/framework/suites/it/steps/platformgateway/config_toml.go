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

package platformgateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

func (g *Gateway) registerConfigSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I change "([^"]*)" from "([^"]*)" to "([^"]*)" in the gateway's config\.toml$`, g.changeConfigValue)
}

// changeConfigValue rewrites one string setting in the block gateway's staged config.toml, the
// file gateway-controller reads when it starts; a restart step applies the change.
func (g *Gateway) changeConfigValue(ctx context.Context, key, from, to string) error {
	resolvedTo, err := stepscommon.Expand(ctx, to)
	if err != nil {
		return err
	}
	stack, _, err := g.topo.ServiceControl("gateway-controller")
	if err != nil {
		return err
	}
	path := filepath.Join(stack.StageDir(), "config.toml")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading the gateway's config.toml: %w", err)
	}
	updated, err := replaceTOMLString(string(content), key, from, resolvedTo)
	if err != nil {
		return err
	}
	// Written in place: the file is bind-mounted, so the container sees it on restart.
	return os.WriteFile(path, []byte(updated), 0o644)
}

// replaceTOMLString changes a dotted key's string value from one value to another, refusing
// unless exactly one line in the key's section sets it to the expected value.
func replaceTOMLString(content, key, from, to string) (string, error) {
	dot := strings.LastIndex(key, ".")
	if dot < 0 {
		return "", fmt.Errorf("key %q names no section", key)
	}
	section, leaf := "["+key[:dot]+"]", key[dot+1:]
	lines := strings.Split(content, "\n")
	inSection, found := false, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inSection = trimmed == section
			continue
		}
		name, value, ok := strings.Cut(trimmed, "=")
		if !inSection || !ok || strings.TrimSpace(name) != leaf {
			continue
		}
		if strings.Trim(strings.TrimSpace(value), `"'`) != from {
			return "", fmt.Errorf("%s is %s, not %q", key, strings.TrimSpace(value), from)
		}
		if found >= 0 {
			return "", fmt.Errorf("%s is set more than once", key)
		}
		found = i
	}
	if found < 0 {
		return "", fmt.Errorf("%s is not set in the config", key)
	}
	lines[found] = leaf + " = " + strconv.Quote(to)
	return strings.Join(lines, "\n"), nil
}
