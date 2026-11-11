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

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/topology"
)

// productPaths maps product source trees, relative to the repository root, to the components
// that run them. A change under one runs every block that starts one of its components. This
// is the only hand-kept list: it changes when a product is added, not when a test is.
var productPaths = []struct {
	prefix     string
	components []string
}{
	{"gateway/", []string{"platform-gateway"}},
	{"platform-api/", []string{"platform-api"}},
	{"portals/api-portal/", apiPortalComponents},
	{"tests/framework/core/catalog/platformgateway/", []string{"platform-gateway"}},
	{"tests/framework/core/catalog/platformapi/", []string{"platform-api"}},
	{"tests/framework/core/catalog/apiportal/", apiPortalComponents},
}

var apiPortalComponents = []string{"api-portal", "api-portal-other-org", "api-portal-multi-organization"}

// sharedStepPackage holds steps every product package builds on. A change to it, or to the
// steps package itself, runs every block.
const sharedStepPackage = "common"

// selection is the outcome for a set of changed files.
type selection struct {
	// all is true when a change can affect any block.
	all bool
	// sources are the blocks selected by name, each with every variant.
	sources map[string]bool
	// samples are the one-variant blocks added to check that a step package change does not
	// break the blocks that do not use it.
	samples map[string]bool
}

// index holds what the selection rules read from the suite.
type index struct {
	// suiteDir is the suite directory relative to the repository root, in slash form.
	suiteDir string
	blocks   []topology.ResolvedBlock
	// featureSources maps a feature path, relative to the suite directory, to the blocks
	// whose runners list it.
	featureSources map[string]map[string]bool
	// featureText holds each feature's content, for finding the resources it names.
	featureText map[string]string
	// stepPackageSources maps a product step package to the blocks that run a feature using
	// one of its steps, or a step of a package that imports it, since that package runs its
	// code.
	stepPackageSources map[string]map[string]bool
	// samples is the first block of each block file, in its first variant.
	samples []string
}

// buildIndex reads the features and step packages of the suite in suiteAbs, which is the
// directory suiteDir (relative to the repository root) on disk.
func buildIndex(resolved *topology.Resolved, suiteAbs, suiteDir string) (*index, error) {
	idx := &index{
		suiteDir:           strings.TrimSuffix(filepath.ToSlash(suiteDir), "/"),
		blocks:             resolved.Blocks,
		featureSources:     map[string]map[string]bool{},
		featureText:        map[string]string{},
		stepPackageSources: map[string]map[string]bool{},
	}

	seenFile := map[string]bool{}
	for _, block := range resolved.Blocks {
		if !seenFile[block.File] {
			seenFile[block.File] = true
			idx.samples = append(idx.samples, block.Name)
		}
		for _, runner := range block.Runners {
			for _, feature := range runner.Features {
				feature = pathpkg.Clean(filepath.ToSlash(feature))
				addTo(idx.featureSources, feature, block.Source)
			}
		}
	}

	packages, shared, err := readStepPackages(filepath.Join(suiteAbs, "steps"))
	if err != nil {
		return nil, err
	}
	for feature := range idx.featureSources {
		raw, err := os.ReadFile(filepath.Join(suiteAbs, filepath.FromSlash(feature)))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", feature, err)
		}
		idx.featureText[feature] = string(raw)
		used := map[string]bool{}
		unmatched := false
		for _, step := range featureSteps(string(raw)) {
			// A shared step (steps/*.go, steps/common) belongs to no product package.
			matched := shared.matches(step)
			for name, pkg := range packages {
				if pkg.matches(step) {
					used[name] = true
					matched = true
				}
			}
			unmatched = unmatched || !matched
		}
		// A step no pattern matches (an outline placeholder, say) may belong to any package,
		// so the feature counts as using all of them. A pattern built at run time matches
		// every step, so its package counts as used by every feature.
		for name := range packages {
			if !unmatched && !used[name] {
				continue
			}
			for _, dependency := range packages.dependenciesOf(name) {
				for source := range idx.featureSources[feature] {
					addTo(idx.stepPackageSources, dependency, source)
				}
			}
		}
	}
	// A block whose runners come from a policy tree has features this index cannot read, so
	// every step package change runs it.
	for _, block := range resolved.Blocks {
		if block.RunnersFrom == "" {
			continue
		}
		for name := range packages {
			addTo(idx.stepPackageSources, name, block.Source)
		}
	}
	return idx, nil
}

// affected selects the blocks the changed files, relative to the repository root, can affect.
func (idx *index) affected(changed []string) selection {
	sel := selection{sources: map[string]bool{}, samples: map[string]bool{}}
	for _, file := range changed {
		file = strings.TrimSpace(filepath.ToSlash(file))
		if file == "" {
			continue
		}
		if !idx.addFile(&sel, file) {
			return selection{all: true}
		}
	}
	if len(sel.sources) == 0 && len(sel.samples) == 0 {
		// No rule selected anything (only step unit tests changed, say): run everything
		// rather than nothing.
		return selection{all: true}
	}
	for sample := range sel.samples {
		if sel.sources[idx.sourceOf(sample)] {
			delete(sel.samples, sample)
		}
	}
	return sel
}

// addFile adds the blocks file can affect to sel. It reports false when the file can affect
// every block.
func (idx *index) addFile(sel *selection, file string) bool {
	for _, product := range productPaths {
		if strings.HasPrefix(file, product.prefix) {
			idx.addComponents(sel, product.components)
			return true
		}
	}
	rel, ok := strings.CutPrefix(file, idx.suiteDir+"/")
	if !ok {
		return false
	}
	switch {
	case strings.HasSuffix(rel, "_test.go"):
		// Unit tests of the steps run outside the suite.
		return true
	case strings.HasPrefix(rel, "features/"):
		for feature, sources := range idx.featureSources {
			if feature == rel || strings.HasPrefix(rel, strings.TrimSuffix(feature, "/")+"/") {
				for source := range sources {
					sel.sources[source] = true
				}
			}
		}
		return true
	case strings.HasPrefix(rel, "blocks/"):
		for _, block := range idx.blocks {
			if block.File == rel {
				sel.sources[block.Source] = true
			}
		}
		return true
	case strings.HasPrefix(rel, "resources/"):
		return idx.addResource(sel, rel)
	case strings.HasPrefix(rel, "steps/"):
		parts := strings.Split(rel, "/")
		if len(parts) < 3 || parts[1] == sharedStepPackage {
			return false
		}
		for source := range idx.stepPackageSources[parts[1]] {
			sel.sources[source] = true
		}
		for _, sample := range idx.samples {
			sel.samples[sample] = true
		}
		return true
	}
	return false
}

// addResource selects the blocks that run a feature naming the resource. A resource no
// feature names may be read by step code, so it runs every block.
func (idx *index) addResource(sel *selection, rel string) bool {
	found := false
	for feature, text := range idx.featureText {
		if strings.Contains(text, rel) {
			found = true
			for source := range idx.featureSources[feature] {
				sel.sources[source] = true
			}
		}
	}
	return found
}

func (idx *index) addComponents(sel *selection, names []string) {
	for _, block := range idx.blocks {
		for _, component := range block.Components {
			if component.Def != nil && slices.Contains(names, component.Def.Name) {
				sel.sources[block.Source] = true
				break
			}
		}
	}
}

func (idx *index) sourceOf(name string) string {
	for _, block := range idx.blocks {
		if block.Name == name {
			return block.Source
		}
	}
	return name
}

// blocks returns the -blocks value for sel, or empty for every block.
func (sel selection) blocks() string {
	if sel.all {
		return ""
	}
	names := make([]string, 0, len(sel.sources)+len(sel.samples))
	for name := range sel.sources {
		names = append(names, name)
	}
	for name := range sel.samples {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// label names what runs, for the integration jobs' titles.
func (idx *index) label(sel selection) string {
	if sel.all {
		return "all blocks"
	}
	label := fmt.Sprintf("%d of %d blocks", len(sel.sources), idx.sourceCount())
	switch n := len(sel.samples); {
	case n == 1:
		label += " + 1 sample block"
	case n > 1:
		label += fmt.Sprintf(" + %d sample blocks", n)
	}
	return label
}

func (idx *index) sourceCount() int {
	seen := map[string]bool{}
	for _, block := range idx.blocks {
		seen[block.Source] = true
	}
	return len(seen)
}

// stepPackage is one directory under steps/ and the step patterns it registers.
type stepPackage struct {
	importPath string
	imports    []string
	patterns   []*regexp.Regexp
}

func (p *stepPackage) matches(step string) bool {
	for _, pattern := range p.patterns {
		if pattern.MatchString(step) {
			return true
		}
	}
	return false
}

type stepPackages map[string]*stepPackage

// dependenciesOf returns name and every product step package it imports, directly or not:
// a feature using name's steps also runs their code.
func (pkgs stepPackages) dependenciesOf(name string) []string {
	out := []string{name}
	for i := 0; i < len(out); i++ {
		for other, pkg := range pkgs {
			if slices.Contains(pkgs[out[i]].imports, pkg.importPath) && !slices.Contains(out, other) {
				out = append(out, other)
			}
		}
	}
	return out
}

// readStepPackages parses the product step packages under dir, collecting the literal
// patterns passed to Step, and returns the patterns of the shared steps (dir itself and its
// common package) separately.
func readStepPackages(dir string) (stepPackages, *stepPackage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	shared := &stepPackage{}
	if err := shared.readDir(dir); err != nil {
		return nil, nil, err
	}
	pkgs := stepPackages{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == sharedStepPackage {
			if err := shared.readDir(filepath.Join(dir, entry.Name())); err != nil {
				return nil, nil, err
			}
			continue
		}
		pkg := &stepPackage{importPath: "/steps/" + entry.Name()}
		if err := pkg.readDir(filepath.Join(dir, entry.Name())); err != nil {
			return nil, nil, err
		}
		pkgs[entry.Name()] = pkg
	}
	return pkgs, shared, nil
}

func (p *stepPackage) readDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return err
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		if err := p.read(file); err != nil {
			return err
		}
	}
	return nil
}

func (p *stepPackage) read(file string) error {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil && strings.Contains(path, "/suites/it/steps/") {
			p.imports = append(p.imports, path[strings.LastIndex(path, "/steps/"):])
		}
	}
	var badPattern error
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Step" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			// A pattern built at run time cannot be matched here, so it matches every step.
			p.patterns = append(p.patterns, regexp.MustCompile(""))
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			badPattern = fmt.Errorf("%s: %w", file, err)
			return false
		}
		pattern, err := regexp.Compile(value)
		if err != nil {
			badPattern = fmt.Errorf("%s: step pattern %q: %w", file, value, err)
			return false
		}
		p.patterns = append(p.patterns, pattern)
		return true
	})
	return badPattern
}

var stepKeyword = regexp.MustCompile(`^(?:Given|When|Then|And|But|\*)\s+(.+)$`)

// featureSteps returns the step texts of a feature, skipping doc strings and tables.
func featureSteps(text string) []string {
	var steps []string
	inDocString := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if inDocString != "" {
			if strings.HasPrefix(line, inDocString) {
				inDocString = ""
			}
			continue
		}
		if strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, "```") {
			inDocString = line[:3]
			continue
		}
		if m := stepKeyword.FindStringSubmatch(line); m != nil {
			steps = append(steps, strings.TrimSpace(m[1]))
		}
	}
	return steps
}

func addTo(m map[string]map[string]bool, key, value string) {
	if m[key] == nil {
		m[key] = map[string]bool{}
	}
	m[key][value] = true
}
