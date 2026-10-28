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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package sheet

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Layer names the test layer a manifest entry describes.
type Layer string

const (
	// LayerUI is the browser-driven suite under suites/ui.
	LayerUI Layer = "ui"
	// LayerIT is the API-level integration suite under suites/it.
	LayerIT Layer = "it"
)

// Layers lists every layer a sheet ID must be accounted for in.
var Layers = []Layer{LayerUI, LayerIT}

// CoverageStatus says how one layer accounts for a sheet ID.
type CoverageStatus string

const (
	// Automated means the referenced scenarios run in CI and assert the sheet's expectation.
	Automated CoverageStatus = "automated"
	// KnownIssue means the referenced scenarios assert the expectation but are excluded
	// with @known-issue until the linked product issue is fixed.
	KnownIssue CoverageStatus = "known-issue"
	// Exempt means the layer deliberately does not test the ID, for the recorded reason.
	Exempt CoverageStatus = "exempt"
	// Planned means the ID is not automated in the layer yet.
	Planned CoverageStatus = "planned"
)

// Ref points at one scenario, by feature path relative to the framework root.
type Ref struct {
	Feature  string `yaml:"feature"`
	Scenario string `yaml:"scenario"`
}

// Coverage is how one layer covers one sheet ID.
type Coverage struct {
	Status CoverageStatus `yaml:"status"`
	Refs   []Ref          `yaml:"refs,omitempty"`
	Issue  string         `yaml:"issue,omitempty"`
	Reason string         `yaml:"reason,omitempty"`
}

// Entry is one row of the scenario sheet.
type Entry struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Priority string `yaml:"priority,omitempty"`
	// SheetUpdate is the corrected expected result the feature owner decided on, to be
	// copied back into the sheet, when the sheet's own wording did not match the product.
	SheetUpdate string   `yaml:"sheetUpdate,omitempty"`
	UI          Coverage `yaml:"ui"`
	IT          Coverage `yaml:"it"`
}

// For returns the entry's coverage in layer.
func (e Entry) For(layer Layer) Coverage {
	if layer == LayerUI {
		return e.UI
	}
	return e.IT
}

// Manifest mirrors one feature's scenario sheet.
type Manifest struct {
	Sheet     string  `yaml:"sheet"`
	Title     string  `yaml:"title"`
	Scenarios []Entry `yaml:"scenarios"`

	// Path is where the manifest was loaded from.
	Path string `yaml:"-"`
}

// LoadManifests loads and validates every *.yaml manifest in dir, sorted by sheet code.
func LoadManifests(dir string) ([]Manifest, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("sheet: listing manifests in %q: %w", dir, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("sheet: no manifests found in %q", dir)
	}
	manifests := make([]Manifest, 0, len(paths))
	seen := map[string]string{}
	for _, path := range paths {
		m, err := LoadManifest(path)
		if err != nil {
			return nil, err
		}
		for _, e := range m.Scenarios {
			if other, dup := seen[e.ID]; dup {
				return nil, fmt.Errorf("sheet: %s is listed in both %s and %s", e.ID, other, path)
			}
			seen[e.ID] = path
		}
		manifests = append(manifests, m)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Sheet < manifests[j].Sheet })
	return manifests, nil
}

// LoadManifest loads and validates one manifest file.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("sheet: reading manifest %q: %w", path, err)
	}
	var m Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("sheet: parsing manifest %q: %w", path, err)
	}
	m.Path = path
	if err := m.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("sheet: manifest %q: %w", path, err)
	}
	return m, nil
}

// Validate checks the manifest's own consistency; it does not look at feature files.
func (m Manifest) Validate() error {
	if strings.TrimSpace(m.Sheet) == "" {
		return fmt.Errorf("sheet code is required")
	}
	if len(m.Scenarios) == 0 {
		return fmt.Errorf("sheet %s lists no scenarios", m.Sheet)
	}
	prefix := strings.SplitN(m.Sheet, "-", 2)[0] + "-"
	seen := map[string]bool{}
	for _, e := range m.Scenarios {
		if !IsID(e.ID) {
			return fmt.Errorf("%q is not a sheet scenario ID", e.ID)
		}
		if !strings.HasPrefix(e.ID, prefix) {
			return fmt.Errorf("%s does not belong to sheet %s (IDs must start with %s)", e.ID, m.Sheet, prefix)
		}
		if seen[e.ID] {
			return fmt.Errorf("%s is listed more than once", e.ID)
		}
		seen[e.ID] = true
		if strings.TrimSpace(e.Title) == "" {
			return fmt.Errorf("%s needs a title", e.ID)
		}
		for _, layer := range Layers {
			if err := e.For(layer).validate(); err != nil {
				return fmt.Errorf("%s %s: %w", e.ID, layer, err)
			}
		}
	}
	return nil
}

func (c Coverage) validate() error {
	switch c.Status {
	case Automated, KnownIssue:
		if len(c.Refs) == 0 {
			return fmt.Errorf("status %s needs at least one scenario ref", c.Status)
		}
		for _, ref := range c.Refs {
			if strings.TrimSpace(ref.Feature) == "" || strings.TrimSpace(ref.Scenario) == "" {
				return fmt.Errorf("every ref needs a feature and a scenario")
			}
		}
		if c.Status == KnownIssue && strings.TrimSpace(c.Issue) == "" {
			return fmt.Errorf("status known-issue needs the issue link")
		}
	case Exempt:
		if strings.TrimSpace(c.Reason) == "" {
			return fmt.Errorf("status exempt needs a reason")
		}
		if len(c.Refs) > 0 {
			return fmt.Errorf("status exempt must not reference scenarios")
		}
	case Planned:
		if len(c.Refs) > 0 {
			return fmt.Errorf("status planned must not reference scenarios; mark it automated or known-issue")
		}
	case "":
		return fmt.Errorf("status is required (automated, known-issue, exempt or planned)")
	default:
		return fmt.Errorf("unknown status %q (want automated, known-issue, exempt or planned)", c.Status)
	}
	return nil
}
