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
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// Default timeouts, applied when a suite names none.
const (
	DefaultBootTimeout        = 5 * time.Minute
	DefaultPropagationTimeout = 180 * time.Second
	DefaultParallel           = 2
)

// Resolved is a suite with defaults applied and matrices expanded.
type Resolved struct {
	Name     string
	Parallel int
	Timeouts Timeouts
	Cleanup  Cleanup

	// Blocks are fully expanded matrix variants.
	Blocks []ResolvedBlock
}

// ResolvedBlock is one concrete topology variant.
type ResolvedBlock struct {
	// Name is the unique block identifier, including any matrix variant.
	Name string

	// Source is the block name as written, before matrix expansion.
	Source string

	// DB is the matrix engine for this variant, or empty when no matrix is declared.
	DB components.DBType

	Parallel   int
	Components []ResolvedComponent
	Runners    []Runner
}

// ResolvedComponent is one component with its engine and wiring resolved.
type ResolvedComponent struct {
	// Def is the registered definition.
	Def *components.Definition

	// Version is the resolved image version, if one was configured.
	Version string

	// DB is the resolved component engine, or empty for a stateless component.
	DB components.DBType

	// Image is the database server image selected for this component.
	Image components.ImageRef

	// Overlay is the block-supplied configuration overlay, if any.
	Overlay string

	// Replicas is the instance count.
	Replicas int

	// Wiring is the decoded, validated wiring value, or nil when the block supplied none.
	Wiring any

	// DependsOn are block-specific boot-order dependencies.
	DependsOn []string
}

// AllDependencies returns definition and block-specific dependencies without duplicates.
func (c ResolvedComponent) AllDependencies() []string {
	var out []string
	if c.Def != nil {
		out = append(out, c.Def.DependsOn...)
	}
	for _, d := range c.DependsOn {
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

// EffectiveParallel returns the block concurrency, defaulting to one.
func (b *ResolvedBlock) EffectiveParallel() int {
	if b == nil || b.Parallel <= 0 {
		return 1
	}
	return b.Parallel
}

// PartitionKey returns a lowercase, URL-safe key derived from the block name.
func (b *ResolvedBlock) PartitionKey() string {
	if b == nil {
		return ""
	}
	var out strings.Builder
	for _, r := range strings.ToLower(b.Name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}
	return strings.Trim(out.String(), "-")
}

// LoadFile reads and resolves a suite file.
func LoadFile(path string, registry *components.Registry) (*Resolved, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("topology: reading suite file %q: %w", path, err)
	}
	resolved, err := Load(raw, registry)
	if err != nil {
		return nil, fmt.Errorf("topology: in suite file %q: %w", path, err)
	}
	return resolved, nil
}

// Load parses and resolves a suite from YAML.
//
// Parsing is strict and rejects unknown fields and multiple YAML documents.
func Load(raw []byte, registry *components.Registry) (*Resolved, error) {
	if registry == nil {
		return nil, fmt.Errorf("topology: a component registry is required")
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var suite Suite
	if err := dec.Decode(&suite); err != nil {
		return nil, fmt.Errorf("topology: parsing suite: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("topology: parsing suite: multiple YAML documents are not allowed")
		}
		return nil, fmt.Errorf("topology: parsing suite: %w", err)
	}

	resolved, err := resolve(&suite, registry)
	if err != nil {
		return nil, err
	}
	if err := Validate(resolved, registry); err != nil {
		return nil, err
	}
	return resolved, nil
}

// resolve applies defaults and expands database matrices.
func resolve(suite *Suite, registry *components.Registry) (*Resolved, error) {
	defaults := suite.Defaults.Components
	if len(suite.Defaults.DB) > 0 {
		if defaults == nil {
			defaults = make(map[string]ComponentDefaults, len(suite.Defaults.DB))
		} else {
			defaults = maps.Clone(defaults)
		}
		for name, db := range suite.Defaults.DB {
			if _, exists := defaults[name]; !exists {
				defaults[name] = ComponentDefaults{DB: db}
			}
		}
	}
	out := &Resolved{
		Name:     suite.Name,
		Parallel: suite.Defaults.Parallel,
		Timeouts: suite.Defaults.Timeouts,
		Cleanup:  suite.Defaults.Cleanup,
	}
	if out.Parallel <= 0 {
		out.Parallel = DefaultParallel
	}
	if out.Timeouts.Boot <= 0 {
		out.Timeouts.Boot = DefaultBootTimeout
	}
	if out.Timeouts.Propagation <= 0 {
		out.Timeouts.Propagation = DefaultPropagationTimeout
	}

	var errs errorList

	if err := validateDefaults(defaults, registry); err != nil {
		errs.add(err)
	}

	for i := range suite.Blocks {
		block := &suite.Blocks[i]
		variants, err := expandMatrix(block)
		if err != nil {
			errs.add(err)
			continue
		}
		for _, variant := range variants {
			rb, err := resolveBlock(block, variant, defaults, registry)
			if err != nil {
				errs.add(err)
				continue
			}
			out.Blocks = append(out.Blocks, *rb)
		}
	}

	if err := errs.err(); err != nil {
		return nil, err
	}
	return out, nil
}

// variant describes one matrix expansion.
type variant struct {
	// name is the matrix suffix appended to the block name.
	name string

	// component is the component whose engine varies.
	component string

	// db is the selected database engine.
	db components.DBType

	// image is the selected database variant.
	image DBVariant
}

// expandMatrix returns the concrete variants for a block.
func expandMatrix(block *Block) ([]variant, error) {
	var (
		matrixComponent string
		variants        []string
	)
	for i := range block.Components {
		c := &block.Components[i]
		if !c.DB.IsMatrix() {
			continue
		}
		if matrixComponent != "" {
			return nil, fmt.Errorf(
				"block %q: components %q and %q both declare db.matrix; at most one component "+
					"per block may be a matrix", block.Name, matrixComponent, c.Name)
		}
		matrixComponent = c.Name
		variants = c.DB.Variants
		if len(variants) == 0 {
			for _, engine := range c.DB.Matrix {
				key, _, ok := defaultVariant(engine)
				if !ok {
					return nil, fmt.Errorf("block %q: unsupported database engine %q", block.Name, engine)
				}
				variants = append(variants, key)
			}
		}
	}

	if matrixComponent == "" {
		return []variant{{}}, nil
	}

	out := make([]variant, 0, len(variants))
	seen := make(map[string]bool, len(variants))
	for _, key := range variants {
		if seen[key] {
			return nil, fmt.Errorf("block %q: db.matrix contains duplicate engine %q or variant", block.Name, dbEngineName(key))
		}
		seen[key] = true
		db, ok := SupportedDBVariants[key]
		if !ok {
			return nil, fmt.Errorf("block %q: unsupported database variant %q", block.Name, key)
		}
		suffix := strings.ReplaceAll(key, ":", "-")
		if defaultKey, ok := DefaultDBVariants[db.Engine]; ok && key == defaultKey {
			suffix = string(db.Engine)
		}
		out = append(out, variant{name: suffix, component: matrixComponent, db: db.Engine, image: db})
	}
	return out, nil
}

func dbEngineName(key string) string {
	if variant, ok := SupportedDBVariants[key]; ok {
		return string(variant.Engine)
	}
	return key
}

// componentEngine selects the component-specific or default engine.
func componentEngine(c *Component, v variant, defaults map[string]ComponentDefaults) components.DBType {
	if v.component != "" && c.Name == v.component {
		return v.db
	}
	if c.DB.One != "" {
		return c.DB.One
	}
	if spec, ok := defaults[c.Name]; ok {
		return spec.DB.One
	}
	return ""
}

func componentVariant(c *Component, v variant, defaults map[string]ComponentDefaults) (string, DBVariant, bool) {
	if v.component != "" && c.Name == v.component {
		if v.image.Engine == "" {
			key, image, ok := defaultVariant(v.db)
			return key, image, ok
		}
		return v.name, v.image, true
	}
	if c.DB.Variant != "" {
		variant, ok := SupportedDBVariants[c.DB.Variant]
		return c.DB.Variant, variant, ok
	}
	if c.DB.One != "" {
		key, variant, ok := defaultVariant(c.DB.One)
		return key, variant, ok
	}
	if spec, ok := defaults[c.Name]; ok {
		key := spec.DB.Variant
		if key == "" {
			key, _, ok = defaultVariant(spec.DB.One)
			if !ok {
				return "", DBVariant{}, false
			}
		}
		variant, ok := SupportedDBVariants[key]
		return key, variant, ok
	}
	return "", DBVariant{}, false
}

// engineFor resolves an engine, following store-sharing components to their owner.
func engineFor(
	c *Component, v variant, defaults map[string]ComponentDefaults, block *Block, registry *components.Registry,
) components.DBType {
	def, ok := registry.Lookup(c.Name)
	if !ok || def.DB == nil || def.DB.Owns() {
		return componentEngine(c, v, defaults)
	}

	ownerName := def.DB.SharesStoreWith
	for i := range block.Components {
		if owner := &block.Components[i]; owner.Name == ownerName {
			return componentEngine(owner, v, defaults)
		}
	}

	return componentEngine(&Component{Name: ownerName}, v, defaults)
}

// validateDefaults checks suite component defaults.
func validateDefaults(defaults map[string]ComponentDefaults, registry *components.Registry) error {
	var errs errorList
	for _, name := range sortedKeys(defaults) {
		spec := defaults[name]
		db := spec.DB
		if db.IsMatrix() {
			errs.addf("defaults.components[%q].db: a matrix is not allowed here; defaults are a fallback "+
				"and must name a single engine (declare the matrix on the component in the "+
				"block that should expand)", name)
			continue
		}
		def, ok := registry.Lookup(name)
		if !ok {
			errs.addf("defaults.components[%q]: unknown component (registered: %v)", name, registry.Names())
			continue
		}
		if db.IsZero() {
			continue
		}
		if def.DB == nil {
			errs.addf("defaults.components[%q].db: component has no storage, so it cannot be given an engine", name)
			continue
		}
		if !def.DB.Owns() {
			errs.addf("defaults.components[%q].db: component shares %q's store, so its engine follows %q "+
				"and cannot be set here", name, def.DB.SharesStoreWith, def.DB.SharesStoreWith)
			continue
		}
		if !db.One.Valid() {
			errs.addf("defaults.components[%q].db: unknown db type %q", name, db.One)
			continue
		}
		if !def.DB.Supports(db.One) {
			errs.addf("defaults.components[%q].db: component does not support db %q", name, db.One)
		}
	}
	return errs.err()
}

func sortedKeys(m map[string]ComponentDefaults) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func resolveBlock(
	block *Block, v variant, defaults map[string]ComponentDefaults, registry *components.Registry,
) (*ResolvedBlock, error) {
	name := block.Name
	if v.name != "" {
		name = block.Name + "/" + v.name
	}

	rb := &ResolvedBlock{
		Name:     name,
		Source:   block.Name,
		DB:       v.db,
		Parallel: block.EffectiveParallel(),
		Runners:  block.Runners,
	}

	var errs errorList

	seen := map[string]bool{}
	for i := range block.Components {
		c := &block.Components[i]

		if seen[c.Name] {
			errs.addf("block %q: component %q listed twice (use replicas instead)", name, c.Name)
			continue
		}
		seen[c.Name] = true

		def, ok := registry.Lookup(c.Name)
		if !ok {
			errs.addf("block %q: unknown component %q (registered: %v)", name, c.Name, registry.Names())
			continue
		}

		version := c.Version
		if version == "" {
			version = defaults[c.Name].Version
		}
		if version != "" {
			def = def.WithImageVersion(version)
		}

		dbType, err := def.ResolveDBType(engineFor(c, v, defaults, block, registry), "")
		if err != nil {
			errs.addf("block %q: %v", name, err)
			continue
		}

		wiring, err := resolveWiring(block, name, c.Name, def)
		if err != nil {
			errs.add(err)
			continue
		}

		replicas := c.EffectiveReplicas()
		if replicas > 1 && def.AliasIsFixed {
			errs.addf("block %q: component %q has a fixed alias and cannot have replicas (%d requested)",
				name, c.Name, replicas)
			continue
		}

		_, dbVariant, _ := componentVariant(c, v, defaults)
		rb.Components = append(rb.Components, ResolvedComponent{
			Def: def, Version: version, DB: dbType, Image: dbVariant.Image, Overlay: c.Overlay, Replicas: replicas, Wiring: wiring,
			DependsOn: append([]string(nil), c.DependsOn...),
		})
	}

	for target := range block.Wiring {
		if !seen[target] {
			errs.addf("block %q: wiring supplied for %q, which this block does not run", name, target)
		}
	}

	if err := errs.err(); err != nil {
		return nil, err
	}
	return rb, nil
}

// resolveWiring decodes and validates one component's block wiring.
func resolveWiring(
	block *Block, blockName, component string, def *components.Definition,
) (any, error) {
	node, present := block.Wiring[component]
	if !present {
		return nil, nil
	}
	if def.Wiring == nil {
		return nil, fmt.Errorf("block %q: component %q accepts no wiring", blockName, component)
	}
	value, err := def.Wiring.Decode(&node)
	if err != nil {
		return nil, fmt.Errorf("block %q: wiring for %q: %w", blockName, component, err)
	}
	return value, nil
}

// Block returns a resolved block by name.
func (r *Resolved) Block(name string) (*ResolvedBlock, bool) {
	if r == nil {
		return nil, false
	}
	for i := range r.Blocks {
		if r.Blocks[i].Name == name {
			return &r.Blocks[i], true
		}
	}
	return nil, false
}

// BlockNames returns every resolved block name, in declaration order.
func (r *Resolved) BlockNames() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.Blocks))
	for i := range r.Blocks {
		out = append(out, r.Blocks[i].Name)
	}
	return out
}
