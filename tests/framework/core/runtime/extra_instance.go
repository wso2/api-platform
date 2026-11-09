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
	overlay, replicas := "", 0
	for _, rc := range t.Block.Components {
		if rc.Def != nil && rc.Def.Name == component {
			def, overlay, replicas = rc.Def, rc.Overlay, rc.Replicas
			break
		}
	}
	if def == nil || !def.IsCompose() || def.Compose == nil {
		return nil, fmt.Errorf("runtime: block %q runs no compose component %q", blockName(t.Block), component)
	}
	if err := t.claimSuffix(component, suffix, replicas); err != nil {
		return nil, err
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
	spec := def.Compose.WithGenerated(generated)
	opts := Options{Network: t.network, RepoRoot: t.repoRoot, Env: merged, LogWriter: t.logWriter, InstanceSuffix: suffix}
	stack, err := launchComposeWithRetry(ctx, component+suffix, spec.BootAttempts,
		func(ctx context.Context) (*ComposeStack, error) { return LaunchCompose(ctx, def, spec, opts) })
	if stack != nil {
		t.extraMu.Lock()
		t.stops = append(t.stops, stack.Stop)
		t.extraMu.Unlock()
	}
	if err != nil {
		if stack != nil {
			return nil, fmt.Errorf("starting %s%s: %w\nservice logs:\n%s", component, suffix, err, stack.Logs(ctx))
		}
		return nil, fmt.Errorf("starting %s%s: %w", component, suffix, err)
	}
	return stack, nil
}

// claimSuffix reserves suffix for one extra instance of component. Its services are named with
// it on the block's network, so a suffix the block's own replicas or an earlier extra instance
// use would put two services behind one name.
func (t *Topology) claimSuffix(component, suffix string, replicas int) error {
	for ordinal := 0; ordinal < replicas; ordinal++ {
		if instanceSuffix(ordinal, replicas) == suffix {
			return fmt.Errorf("runtime: %s replica %d already uses the suffix %q", component, ordinal+1, suffix)
		}
	}
	t.extraMu.Lock()
	defer t.extraMu.Unlock()
	if t.extraSuffixes[component+suffix] {
		return fmt.Errorf("runtime: another instance of %s already uses the suffix %q", component, suffix)
	}
	if t.extraSuffixes == nil {
		t.extraSuffixes = map[string]bool{}
	}
	t.extraSuffixes[component+suffix] = true
	return nil
}
