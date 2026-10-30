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
	"bufio"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/sheet"
)

var sheetCheckLine = regexp.MustCompile(`^(?:\*|Given|When|Then|And|But)\s+sheet check "([^"]*)"\s*$`)
var scenarioKeyword = regexp.MustCompile(`^(?:Scenario|Scenario Outline|Scenario Template|Example):`)

// featureScenario is the part of a parsed scenario the sheet rules need.
type featureScenario struct {
	name   string
	line   int
	tags   []string // scenario tags only, without the leading "@"
	checks []string
}

// parsedFeature is one feature file reduced to its sheet-relevant structure.
type parsedFeature struct {
	path         string
	featureTags  []string // feature and rule tags
	scenarios    []featureScenario
	backgroundCk []int // lines of sheet checks inside a Background
}

func tagsOf(line string) []string {
	if i := strings.Index(line, " #"); i >= 0 {
		line = line[:i]
	}
	var tags []string
	for field := range strings.FieldsSeq(line) {
		tags = append(tags, strings.TrimPrefix(field, "@"))
	}
	return tags
}

func parseFeatureFile(path string) (parsedFeature, error) {
	file, err := os.Open(path)
	if err != nil {
		return parsedFeature{}, err
	}
	defer file.Close()

	parsed := parsedFeature{path: filepath.ToSlash(path)}
	var pending []string
	inBackground := false
	current := -1
	docString := "" // the open doc-string delimiter, while inside one
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		// Doc-string content is step payload, never Gherkin.
		if docString != "" {
			if strings.HasPrefix(line, docString) {
				docString = ""
			}
			continue
		}
		if strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, "```") {
			docString = line[:3]
			continue
		}
		switch {
		case strings.HasPrefix(line, "@"):
			pending = append(pending, tagsOf(line)...)
		case strings.HasPrefix(line, "Feature:"), strings.HasPrefix(line, "Rule:"):
			parsed.featureTags = append(parsed.featureTags, pending...)
			pending, inBackground, current = nil, false, -1
		case strings.HasPrefix(line, "Background:"):
			pending, inBackground, current = nil, true, -1
		case scenarioKeyword.MatchString(line):
			name := strings.TrimSpace(line[strings.Index(line, ":")+1:])
			parsed.scenarios = append(parsed.scenarios, featureScenario{name: name, line: lineNo, tags: pending})
			pending, inBackground, current = nil, false, len(parsed.scenarios)-1
		case strings.HasPrefix(line, "Examples:"), strings.HasPrefix(line, "Scenarios:"):
			// Tags above Examples select the outline's rows, so they count as the scenario's.
			if current >= 0 {
				parsed.scenarios[current].tags = append(parsed.scenarios[current].tags, pending...)
			}
			pending = nil
		default:
			match := sheetCheckLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			if inBackground {
				parsed.backgroundCk = append(parsed.backgroundCk, lineNo)
			} else if current >= 0 {
				parsed.scenarios[current].checks = append(parsed.scenarios[current].checks, match[1])
			}
		}
	}
	return parsed, scanner.Err()
}

// featureKey identifies a feature file by its absolute, slash-separated path, so roots and
// manifest refs given relative or absolute name the same file.
func featureKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.ToSlash(filepath.Clean(path))
}

// layerOf derives the suite layer from a feature path, or "" outside suites/ui and suites/it.
func layerOf(path string) sheet.Layer {
	path = filepath.ToSlash(path)
	switch {
	case strings.Contains("/"+path, "/suites/ui/"):
		return sheet.LayerUI
	case strings.Contains("/"+path, "/suites/it/"):
		return sheet.LayerIT
	}
	return ""
}

// checkSheets verifies that the scenario-sheet manifests in manifestDir and the sheet
// tags and checkpoints in the feature roots agree with each other.
func checkSheets(manifestDir, featureRoots, suites string) []string {
	if strings.TrimSpace(manifestDir) == "" {
		return nil
	}
	manifests, err := sheet.LoadManifests(manifestDir)
	if err != nil {
		return []string{err.Error()}
	}
	enrolled, issues := enrolledFeatures(suites)

	features := map[string]parsedFeature{}
	for root := range strings.SplitSeq(featureRoots, ",") {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".feature") {
				return nil
			}
			parsed, parseErr := parseFeatureFile(path)
			if parseErr != nil {
				issues = append(issues, fmt.Sprintf("%s: %v", path, parseErr))
				return nil
			}
			features[featureKey(path)] = parsed
			return nil
		})
		if walkErr != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", root, walkErr))
		}
	}

	// referenced[layer|feature|scenario] lists the IDs whose manifest refs name it.
	referenced := map[string][]string{}
	entries := map[string]sheet.Entry{}
	for _, m := range manifests {
		for _, e := range m.Scenarios {
			entries[e.ID] = e
			for _, layer := range sheet.Layers {
				cov := e.For(layer)
				anyKnownIssue := false
				for _, ref := range cov.Refs {
					refIssues, knownIssue := checkSheetRef(m.Path, e.ID, layer, cov.Status, ref, features, enrolled)
					issues = append(issues, refIssues...)
					anyKnownIssue = anyKnownIssue || knownIssue
					key := string(layer) + "|" + featureKey(ref.Feature) + "|" + ref.Scenario
					referenced[key] = append(referenced[key], e.ID)
				}
				if cov.Status == sheet.KnownIssue && !anyKnownIssue {
					issues = append(issues, fmt.Sprintf("%s: %s %s status known-issue needs at least one referenced scenario tagged @known-issue", m.Path, e.ID, layer))
				}
			}
		}
	}

	sheetOf := map[string]string{}
	for _, m := range manifests {
		for _, e := range m.Scenarios {
			sheetOf[e.ID] = m.Sheet
		}
	}

	for path, f := range features {
		for _, code := range sheetsTagged(f, sheetOf) {
			if !slices.Contains(f.featureTags, code) {
				issues = append(issues, fmt.Sprintf("%s: the feature carries sheet IDs of %s, so it must be tagged @%s", path, code, code))
			}
		}
		for _, tag := range f.featureTags {
			if sheet.IsID(tag) {
				issues = append(issues, fmt.Sprintf("%s: sheet ID @%s must be tagged on each scenario, not on the feature or rule", path, tag))
			}
		}
		for _, line := range f.backgroundCk {
			issues = append(issues, fmt.Sprintf("%s:%d: `sheet check` belongs in a scenario, not in a Background", path, line))
		}
		layer := layerOf(path)
		for _, s := range f.scenarios {
			ids := sheet.IDsFromTags(s.tags)
			at := fmt.Sprintf("%s:%d: scenario %q", path, s.line, s.name)
			for _, id := range ids {
				if _, known := entries[id]; !known {
					issues = append(issues, fmt.Sprintf("%s is tagged @%s, which no sheet manifest lists", at, id))
					continue
				}
				if !slices.Contains(referenced[string(layer)+"|"+path+"|"+s.name], id) {
					issues = append(issues, fmt.Sprintf("%s is tagged @%s but the manifest's %s refs for %s do not name it", at, id, layer, id))
				}
			}
			if len(s.checks) == 0 {
				// A journey's checks are what attribute a failure to one of its IDs.
				if layer == sheet.LayerUI && len(ids) > 1 {
					issues = append(issues, fmt.Sprintf("%s covers %d sheet IDs but has no `sheet check` steps", at, len(ids)))
				}
				continue
			}
			if len(ids) == 0 {
				issues = append(issues, fmt.Sprintf("%s uses `sheet check` but carries no sheet ID tags", at))
				continue
			}
			seen := map[string]bool{}
			for _, id := range s.checks {
				switch {
				case seen[id]:
					issues = append(issues, fmt.Sprintf("%s checks %s more than once", at, id))
				case !slices.Contains(ids, id):
					issues = append(issues, fmt.Sprintf("%s checks %s but is not tagged @%s", at, id, id))
				}
				seen[id] = true
			}
			for _, id := range ids {
				if !seen[id] {
					issues = append(issues, fmt.Sprintf("%s is tagged @%s but has no `sheet check %q` step", at, id, id))
				}
			}
		}
	}
	return issues
}

// checkSheetRef validates one manifest ref and reports whether its scenario is tagged
// @known-issue.
func checkSheetRef(manifest, id string, layer sheet.Layer, status sheet.CoverageStatus, ref sheet.Ref, features map[string]parsedFeature, enrolled map[string]bool) ([]string, bool) {
	at := fmt.Sprintf("%s: %s %s ref %s / %q", manifest, id, layer, ref.Feature, ref.Scenario)
	path := featureKey(ref.Feature)
	if layerOf(path) != layer {
		return []string{fmt.Sprintf("%s: a %s ref must point into suites/%s", at, layer, layer)}, false
	}
	f, ok := features[path]
	if !ok {
		return []string{fmt.Sprintf("%s: feature file not found in the checked feature roots", at)}, false
	}
	if enrolled != nil && !enrolled[path] {
		return []string{fmt.Sprintf("%s: no runner in the checked suites runs this feature", at)}, false
	}
	for _, s := range f.scenarios {
		if s.name != ref.Scenario {
			continue
		}
		tags := append(append([]string(nil), f.featureTags...), s.tags...)
		var issues []string
		if !slices.Contains(s.tags, id) {
			issues = append(issues, fmt.Sprintf("%s: the scenario is not tagged @%s", at, id))
		}
		knownIssue := slices.Contains(tags, "known-issue")
		if status == sheet.Automated && knownIssue {
			issues = append(issues, fmt.Sprintf("%s: the scenario is tagged @known-issue, so its status must be known-issue", at))
		}
		return issues, knownIssue
	}
	return []string{fmt.Sprintf("%s: no scenario with that name in the feature", at)}, false
}

// sheetsTagged returns the sheet codes whose IDs the feature's scenarios carry.
func sheetsTagged(f parsedFeature, sheetOf map[string]string) []string {
	var codes []string
	for _, s := range f.scenarios {
		for _, id := range sheet.IDsFromTags(s.tags) {
			if code, ok := sheetOf[id]; ok && !slices.Contains(codes, code) {
				codes = append(codes, code)
			}
		}
	}
	return codes
}

// enrolledFeatures returns the features the suites' runners run, keyed like featureKey, or
// nil when no suites are given. A runner's feature paths are relative to its suite file.
func enrolledFeatures(suites string) (map[string]bool, []string) {
	if strings.TrimSpace(suites) == "" {
		return nil, nil
	}
	enrolled := map[string]bool{}
	var issues []string
	for raw := range strings.SplitSeq(suites, ",") {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: read suite YAML: %v", path, err))
			continue
		}
		var suite struct {
			Blocks []struct {
				Runners []struct {
					Features []string `yaml:"features"`
				} `yaml:"runners"`
			} `yaml:"blocks"`
		}
		if err := yaml.Unmarshal(data, &suite); err != nil {
			issues = append(issues, fmt.Sprintf("%s: parse suite YAML: %v", path, err))
			continue
		}
		for _, block := range suite.Blocks {
			for _, runner := range block.Runners {
				for _, feature := range runner.Features {
					enrolled[featureKey(filepath.Join(filepath.Dir(path), feature))] = true
				}
			}
		}
	}
	return enrolled, issues
}
