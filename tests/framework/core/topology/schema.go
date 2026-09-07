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

package topology

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// Suite is the declarative description of a test run.
type Suite struct {

	// Name identifies the suite in reports and in --list output.
	Name string `yaml:"suite"`

	// Defaults apply to every block that does not override them.
	Defaults Defaults `yaml:"defaults"`

	// Blocks are the topologies this suite runs.
	Blocks []Block `yaml:"blocks"`
}

// Defaults are the suite-wide fallbacks.
type Defaults struct {
	// Parallel bounds the number of blocks that run concurrently.
	Parallel int `yaml:"parallel"`

	// Components contains component-specific suite fallbacks.
	Components map[string]ComponentDefaults `yaml:"components"`

	// DB is retained as a compatibility alias for older suite files. New files should use
	// Components so database and image-version defaults share one component entry.
	DB map[string]DBSpec `yaml:"db"`

	// Timeouts are the suite-wide time limits.
	Timeouts Timeouts `yaml:"timeouts"`

	// Cleanup controls resource cleanup retries. A zero value logs deletion failures and
	// moves on without retrying.
	Cleanup Cleanup `yaml:"cleanup"`
}

// ComponentDefaults are suite-wide fallbacks for one component.
type ComponentDefaults struct {
	// DB selects the component's default engine. Matrix values are not allowed here.
	DB DBSpec `yaml:"db"`

	// Version overrides the version read from the component's VERSION file.
	Version string `yaml:"version"`
}

// Timeouts are the suite's time limits.
type Timeouts struct {
	// Boot bounds bringing one block's topology up.
	Boot time.Duration `yaml:"boot"`

	// Propagation bounds asynchronous runtime propagation checks.
	Propagation time.Duration `yaml:"propagation"`
}

// Cleanup controls cleanup retry behavior.
type Cleanup struct {
	// MaxAttempts is the total number of cleanup attempts for one resource. Zero disables
	// retries and is the default.
	MaxAttempts int `yaml:"max-attempts"`
}

// Block describes one isolated topology and its runners.
type Block struct {
	// Name uniquely identifies the block and is used by block selection.
	Name string `yaml:"name"`

	// Parallel bounds concurrent runners for this block.
	Parallel int `yaml:"parallel"`

	// Components are the systems this block runs. A component may declare a database
	// matrix, which expands the block into one variant per engine.
	Components []Component `yaml:"components"`

	// Wiring configures components by name through their typed wiring specifications.
	Wiring map[string]yaml.Node `yaml:"wiring"`

	// Runners are the feature groups to execute.
	Runners []Runner `yaml:"runners"`
}

// Component selects and configures one registered component for a block.
type Component struct {
	// Name references a component in the registry.
	Name string `yaml:"name"`

	// Overlay is an optional component configuration overlay.
	Overlay string `yaml:"overlay"`

	// DB selects the component's engine or declares its matrix.
	DB DBSpec `yaml:"db"`

	// Version overrides the image version for this block's component instance.
	Version string `yaml:"version"`

	// Replicas is the number of instances to run.
	Replicas int `yaml:"replicas"`

	// DependsOn adds block-specific boot-order dependencies.
	DependsOn []string `yaml:"dependsOn"`
}

// Runner is a group of feature files executed against a block.
type Runner struct {
	// Name identifies the runner within its block.
	Name string `yaml:"name"`

	// Features lists feature files in execution order.
	Features []string `yaml:"features"`

	// Tags filters which scenarios in these features run, using Gherkin tag expressions.
	Tags string `yaml:"tags"`

	// Hook names an optional registered runner hook.
	Hook string `yaml:"hook"`
}

// EffectiveParallel returns the block's runner concurrency, falling back to 1.
func (b *Block) EffectiveParallel() int {
	if b == nil || b.Parallel <= 0 {
		return 1
	}
	return b.Parallel
}

// EffectiveReplicas returns the component's instance count, falling back to 1.
func (c *Component) EffectiveReplicas() int {
	if c == nil || c.Replicas <= 0 {
		return 1
	}
	return c.Replicas
}

// DBSpec selects one engine or a matrix of engines for a component.
type DBSpec struct {
	// One is the single engine, set by the scalar form.
	One components.DBType

	// Variant is the selected supported variant key for scalar values.
	Variant string

	// Matrix is the engine list, set by the mapping form. At most one component per block
	// may set it — see expandMatrix.
	Matrix []components.DBType

	// Variants contains versioned variant keys from YAML matrix values.
	Variants []string
}

// dbSpecMapping supports the mapping form without recursive decoding.
type dbSpecMapping struct {
	Matrix []string `yaml:"matrix"`
}

// UnmarshalYAML accepts a scalar engine or a {matrix: [...]} mapping.
func (d *DBSpec) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var raw string
		if err := value.Decode(&raw); err != nil {
			return err
		}
		key, variant, err := resolveDBVariant(raw)
		if err != nil {
			return fmt.Errorf("topology: %w", err)
		}
		d.One, d.Variant = variant.Engine, key
		return nil
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			key := value.Content[i]
			if key.Value != "matrix" {
				return fmt.Errorf("topology: unknown db field %q", key.Value)
			}
		}
		var m dbSpecMapping
		if err := value.Decode(&m); err != nil {
			return err
		}
		if len(m.Matrix) == 0 {
			return fmt.Errorf("topology: db.matrix is empty; name at least one engine")
		}
		d.Variants = make([]string, 0, len(m.Matrix))
		d.Matrix = make([]components.DBType, 0, len(m.Matrix))
		for _, raw := range m.Matrix {
			key, variant, err := resolveDBVariant(raw)
			if err != nil {
				return fmt.Errorf("topology: %w", err)
			}
			d.Variants = append(d.Variants, key)
			d.Matrix = append(d.Matrix, variant.Engine)
		}
		return nil
	case yaml.SequenceNode:
		return fmt.Errorf(
			"topology: db must be an engine or {matrix: [...]}; a bare list is not accepted " +
				"(write db: {matrix: [...]} to expand this block)")
	default:
		return fmt.Errorf("topology: db must be an engine or {matrix: [...]}")
	}
}

// IsMatrix reports whether this spec expands its block.
func (d DBSpec) IsMatrix() bool { return len(d.Matrix) > 0 || len(d.Variants) > 0 }

// IsZero reports whether no engine was declared at all, so the suite default applies.
func (d DBSpec) IsZero() bool { return d.One == "" && len(d.Matrix) == 0 && len(d.Variants) == 0 }
