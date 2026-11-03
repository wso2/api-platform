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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// policyIntegrationDir is the directory inside a policy that holds its integration tests.
	policyIntegrationDir = "it"

	// policyDescriptorFile is the descriptor a policy keeps in its integration directory.
	policyDescriptorFile = "it.yaml"
)

var policyNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// PolicyDescriptor declares the integration runners a policy contributes to a block that
// takes its runners from the policy tree.
type PolicyDescriptor struct {
	// Policy is the policy directory name.
	Policy string `yaml:"policy"`

	// Runners are the runners the policy contributes.
	Runners []PolicyRunner `yaml:"runners"`
}

// PolicyRunner is one runner declared by a policy descriptor.
type PolicyRunner struct {
	// Name identifies the runner within the policy.
	Name string `yaml:"name"`

	// Tags has the same form as Runner.Tags, including the optional gateway-version selector.
	Tags string `yaml:"tags"`

	// Features lists feature files relative to the policy's integration directory.
	Features []string `yaml:"features"`
}

// ValidPolicyName reports whether name can identify a policy directory.
func ValidPolicyName(name string) bool {
	return policyNamePattern.MatchString(name)
}

// loadPolicyDescriptor reads <policyRoot>/<policy>/it/it.yaml. It returns nil without an
// error when the policy declares no descriptor.
func loadPolicyDescriptor(policyRoot, policy string) (*PolicyDescriptor, error) {
	path := filepath.Join(policyRoot, policy, policyIntegrationDir, policyDescriptorFile)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("policy %q: reading descriptor: %w", policy, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("policy %q: descriptor %s must be a regular file", policy, policyDescriptorFile)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy %q: reading descriptor: %w", policy, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var descriptor PolicyDescriptor
	if err := dec.Decode(&descriptor); err != nil {
		return nil, fmt.Errorf("policy %q: parsing descriptor: %w", policy, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("policy %q: the descriptor must hold a single YAML document", policy)
	}
	if err := descriptor.validate(policy, filepath.Join(policyRoot, policy, policyIntegrationDir)); err != nil {
		return nil, err
	}
	return &descriptor, nil
}

func (d *PolicyDescriptor) validate(policy, integrationDir string) error {
	var errs errorList
	if d.Policy != policy {
		errs.addf("policy %q: descriptor names policy %q", policy, d.Policy)
	}
	if len(d.Runners) == 0 {
		errs.addf("policy %q: the descriptor declares no runners", policy)
	}
	seen := map[string]bool{}
	for _, runner := range d.Runners {
		if strings.TrimSpace(runner.Name) == "" {
			errs.addf("policy %q: a runner has no name", policy)
			continue
		}
		if seen[runner.Name] {
			errs.addf("policy %q: duplicate runner name %q", policy, runner.Name)
		}
		seen[runner.Name] = true
		if len(runner.Features) == 0 {
			errs.addf("policy %q runner %q: declares no features", policy, runner.Name)
		}
		for _, feature := range runner.Features {
			if !filepath.IsLocal(filepath.FromSlash(feature)) {
				errs.addf("policy %q runner %q: feature %q must be a path inside the %s directory",
					policy, runner.Name, feature, policyIntegrationDir)
				continue
			}
			if !strings.HasSuffix(feature, ".feature") {
				errs.addf("policy %q runner %q: %q is not a .feature file", policy, runner.Name, feature)
				continue
			}
			info, err := os.Lstat(filepath.Join(integrationDir, filepath.FromSlash(feature)))
			switch {
			case err != nil:
				errs.addf("policy %q runner %q: feature %q does not exist", policy, runner.Name, feature)
			case !info.Mode().IsRegular():
				errs.addf("policy %q runner %q: feature %q must be a regular file", policy, runner.Name, feature)
			}
		}
	}
	return errs.err()
}

// runners converts the descriptor into suite runners. Runner names are prefixed with the
// policy name and feature paths are absolute, so they stay valid from any suite directory.
func (d *PolicyDescriptor) runners(policyRoot string) []Runner {
	out := make([]Runner, 0, len(d.Runners))
	for _, runner := range d.Runners {
		features := make([]string, 0, len(runner.Features))
		for _, feature := range runner.Features {
			features = append(features,
				filepath.Join(policyRoot, d.Policy, policyIntegrationDir, filepath.FromSlash(feature)))
		}
		out = append(out, Runner{Name: d.Policy + "/" + runner.Name, Features: features, Tags: runner.Tags})
	}
	return out
}

// listPolicies returns the policy directory names under policyRoot.
func listPolicies(policyRoot string) ([]string, error) {
	entries, err := os.ReadDir(policyRoot)
	if err != nil {
		return nil, fmt.Errorf("reading policy tree %q: %w", policyRoot, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && ValidPolicyName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// expandPolicyRunners resolves the runners of a block whose runners come from the policy
// tree. An empty requested list selects every policy that has a descriptor. A requested
// policy without a descriptor is reported as skipped; one missing from the tree is an error.
func expandPolicyRunners(block *ResolvedBlock, repoRoot string, requested []string) ([]Runner, []SkippedRunner, error) {
	source := platformGatewayPolicySource(block)
	if source == "" {
		return nil, nil, fmt.Errorf("no policy tree: platform-gateway has no addPoliciesFrom")
	}
	policyRoot := filepath.Join(repoRoot, source)
	available, err := listPolicies(policyRoot)
	if err != nil {
		return nil, nil, err
	}

	names := available
	explicit := len(requested) > 0
	if explicit {
		names = names[:0:0]
		seen := map[string]bool{}
		for _, name := range requested {
			if !ValidPolicyName(name) {
				return nil, nil, fmt.Errorf("invalid policy name %q", name)
			}
			if !slices.Contains(available, name) {
				return nil, nil, fmt.Errorf("policy %q is not in %s", name, source)
			}
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		sort.Strings(names)
	}

	var (
		runners []Runner
		skipped []SkippedRunner
	)
	for _, name := range names {
		descriptor, err := loadPolicyDescriptor(policyRoot, name)
		if err != nil {
			return nil, nil, err
		}
		if descriptor == nil {
			if explicit {
				skipped = append(skipped, SkippedRunner{
					Block: block.Name, Runner: name,
					Reason: "the policy declares no integration descriptor (" + policyIntegrationDir + "/" + policyDescriptorFile + ")",
				})
			}
			continue
		}
		runners = append(runners, descriptor.runners(policyRoot)...)
	}

	runners, err = parseRunnerTags(block.Name, runners)
	if err != nil {
		return nil, nil, err
	}
	return runners, skipped, nil
}
