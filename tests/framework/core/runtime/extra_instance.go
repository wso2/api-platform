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

package runtime

import (
	"context"
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// LaunchComposeInstance starts one more instance of a compose component the block runs, with
// suffix after its service names and env over the component's own environment. The block's
// dependencies provision nothing for it, so env carries what they would, such as a gateway's
// registration token. The block stops it at teardown.
func (t *Topology) LaunchComposeInstance(
	ctx context.Context, component, suffix string, env map[string]string,
) (*ComposeStack, error) {
	if t == nil || t.Block == nil {
		return nil, fmt.Errorf("runtime: topology is required")
	}
	if suffix == "" {
		return nil, fmt.Errorf("runtime: another instance of %s needs a suffix to tell it apart", component)
	}
	var def *components.Definition
	overlay := ""
	for _, rc := range t.Block.Components {
		if rc.Def != nil && rc.Def.Name == component {
			def, overlay = rc.Def, rc.Overlay
			break
		}
	}
	if def == nil || !def.IsCompose() || def.Compose == nil {
		return nil, fmt.Errorf("runtime: block %q runs no compose component %q", blockName(t.Block), component)
	}

	merged := map[string]string{}
	for k, v := range def.Compose.Env {
		merged[k] = v
	}
	for k, v := range env {
		merged[k] = v
	}
	generated := map[string][]byte{"api-platform.env": components.EnvFileContent(merged)}
	if def.Config != nil {
		vars := components.Vars{components.VarBlock: t.Block.PartitionKey()}
		content, err := components.Assemble(def.Config, t.repoRoot, overlay, vars)
		if err != nil {
			return nil, fmt.Errorf("assembling config for %s%s: %w", component, suffix, err)
		}
		generated["config.toml"] = content
	}
	stack, err := LaunchCompose(ctx, def, def.Compose.WithGenerated(generated), Options{
		Network: t.network, RepoRoot: t.repoRoot, Env: merged, LogWriter: t.logWriter, InstanceSuffix: suffix,
	})
	if stack != nil {
		t.stops = append(t.stops, stack.Stop)
	}
	if err != nil {
		if stack != nil {
			return nil, fmt.Errorf("starting %s%s: %w\nservice logs:\n%s", component, suffix, err, stack.Logs(ctx))
		}
		return nil, fmt.Errorf("starting %s%s: %w", component, suffix, err)
	}
	return stack, nil
}
