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
	"flag"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
)

// Selection narrows the blocks, runners, and coverage mode used for a run.
type Selection struct {
	// Blocks names blocks to run. Empty means all. A matrix-generated variant is named
	// with its axis value, so "gateway-core/postgres" selects one variant.
	Blocks []string

	// SkipBlocks names blocks to exclude, applied after Blocks.
	SkipBlocks []string

	// Tags is a Gherkin tag expression applied on top of each runner's own tags, for
	// selecting a subset such as a smoke run.
	Tags string

	// Parallel overrides the suite's concurrent-block bound. Zero keeps the declared value.
	//
	Parallel int

	// RunnerParallel overrides every block's concurrent-runner bound. Zero keeps the
	// declared values.
	//
	RunnerParallel int

	// Coverage enables instrumented images and runtime coverage collection.
	Coverage bool

	// GatewayVersion overrides the image version for platform-gateway components.
	GatewayVersion string

	// CloudEnvironment selects the external cloud environment used by cloud-console.
	CloudEnvironment string
}

// Flags registers selection flags on fs.
func (s *Selection) Flags(fs *flag.FlagSet) {
	fs.Func("blocks", "comma-separated block names to run (default: all)", func(v string) error {
		s.Blocks = append(s.Blocks, splitList(v)...)
		return nil
	})
	fs.Func("skip-blocks", "comma-separated block names to exclude", func(v string) error {
		s.SkipBlocks = append(s.SkipBlocks, splitList(v)...)
		return nil
	})
	fs.StringVar(&s.Tags, "feature-tags", "",
		"godog tag filter applied to every runner: ',' is OR, '&&' is AND, '~' is NOT "+
			"(e.g. \"@metrics,@certificates\"). NOT cucumber's and/or/not words — those parse "+
			"as one literal tag name and silently match nothing")
	fs.IntVar(&s.Parallel, "block-parallel", 0, "override how many blocks run concurrently")
	fs.IntVar(&s.RunnerParallel, "runner-parallel", 0,
		"override how many runners run concurrently within each block")
	fs.BoolVar(&s.Coverage, "coverage", false,
		"build instrumented source images and collect runtime coverage")
	fs.StringVar(&s.GatewayVersion, "gateway-version", "",
		"override the platform-gateway image version for this run")
	fs.StringVar(&s.CloudEnvironment, "cloud-env", "",
		"select the cloud environment for external cloud-console components")
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Apply narrows a resolved suite and returns an independent copy.
func (s Selection) Apply(resolved *Resolved) (*Resolved, error) {
	if resolved == nil {
		return nil, fmt.Errorf("topology: resolved suite is required")
	}
	if s.Parallel < 0 {
		return nil, fmt.Errorf("topology: block parallelism cannot be negative")
	}
	if s.RunnerParallel < 0 {
		return nil, fmt.Errorf("topology: runner parallelism cannot be negative")
	}
	if strings.TrimSpace(s.GatewayVersion) != "" && s.Coverage {
		return nil, fmt.Errorf("topology: -gateway-version cannot be combined with -coverage; coverage requires framework-built gateway images")
	}
	out := &Resolved{
		Name:     resolved.Name,
		Parallel: resolved.Parallel,
		Timeouts: resolved.Timeouts,
		Cleanup:  resolved.Cleanup,
	}
	if s.Parallel > 0 {
		out.Parallel = s.Parallel
	}

	include := map[string]bool{}
	for _, b := range s.Blocks {
		include[b] = true
	}
	exclude := map[string]bool{}
	for _, b := range s.SkipBlocks {
		exclude[b] = true
	}

	matchedInclude := map[string]bool{}
	matchedExclude := map[string]bool{}

	for i := range resolved.Blocks {
		block := cloneBlock(resolved.Blocks[i])
		if version := strings.TrimSpace(s.GatewayVersion); version != "" {
			for j := range block.Components {
				component := &block.Components[j]
				if component.Def != nil && component.Def.Name == "platform-gateway" {
					var err error
					component.Def, err = component.Def.WithReleaseVersion(version)
					if err != nil {
						return nil, fmt.Errorf("topology: block %q: %w", block.Name, err)
					}
					component.Version = version
					component.BuildFromSource = false
				}
			}
		}
		if len(include) > 0 {
			_, byName := include[block.Name]
			_, bySource := include[block.Source]
			if !byName && !bySource {
				if exclude[block.Name] {
					matchedExclude[block.Name] = true
				}
				if exclude[block.Source] {
					matchedExclude[block.Source] = true
				}
				continue
			}
			if byName {
				matchedInclude[block.Name] = true
			}
			if bySource {
				matchedInclude[block.Source] = true
			}
		}

		if exclude[block.Name] || exclude[block.Source] {
			if exclude[block.Name] {
				matchedExclude[block.Name] = true
			}
			if exclude[block.Source] {
				matchedExclude[block.Source] = true
			}
			continue
		}

		skipped, err := selectGatewayDatabaseCompatibility(&block)
		if err != nil {
			return nil, err
		}
		if skipped != nil {
			if include[block.Name] {
				return nil, fmt.Errorf("topology: selected block %q is incompatible: %s", block.Name, skipped.Reason)
			}
			out.SkippedBlocks = append(out.SkippedBlocks, *skipped)
			continue
		}

		runners, skippedRunners, err := selectGatewayVersionRunners(&block)
		if err != nil {
			return nil, err
		}
		out.SkippedRunners = append(out.SkippedRunners, skippedRunners...)
		if len(block.Runners) > 0 && len(runners) == 0 {
			continue
		}
		block.Runners = runners

		skipExternalBlock := false
		for j := range block.Components {
			component := &block.Components[j]
			if component.Def == nil || !component.Def.IsExternal() {
				continue
			}
			parameters := make(map[string]string, len(component.ExternalParameters)+1)
			for key, value := range component.ExternalParameters {
				parameters[key] = value
			}
			if value := strings.TrimSpace(s.CloudEnvironment); value != "" {
				parameters["environment"] = value
			}
			for _, required := range component.Def.External.RequiredParameters {
				if strings.TrimSpace(parameters[required]) == "" {
					if len(include) == 0 {
						// External blocks are opt-in. A normal local suite run must not
						// require cloud credentials or a target environment merely because
						// the suite declares the cloud coverage block.
						skipExternalBlock = true
						break
					}
					return nil, fmt.Errorf("topology: external component %q in block %q requires -cloud-env",
						component.Def.Name, block.Name)
				}
			}
			component.ExternalParameters = parameters
		}
		if skipExternalBlock {
			continue
		}

		if s.RunnerParallel > 0 {
			block.Parallel = s.RunnerParallel
		}

		if s.Tags != "" {
			runners := make([]Runner, len(block.Runners))
			copy(runners, block.Runners)
			for j := range runners {
				runners[j].Tags = combineTags(runners[j].Tags, s.Tags)
			}
			block.Runners = runners
		}

		out.Blocks = append(out.Blocks, block)
	}

	var unmatched []string
	for name := range include {
		if !matchedInclude[name] {
			unmatched = append(unmatched, name)
		}
	}
	for name := range exclude {
		if !matchedExclude[name] {
			unmatched = append(unmatched, name)
		}
	}
	if len(unmatched) > 0 {
		sort.Strings(unmatched)
		available := make([]string, 0, len(resolved.Blocks))
		seen := map[string]bool{}
		for i := range resolved.Blocks {
			for _, n := range []string{resolved.Blocks[i].Name, resolved.Blocks[i].Source} {
				if !seen[n] {
					seen[n] = true
					available = append(available, n)
				}
			}
		}
		sort.Strings(available)
		return nil, fmt.Errorf("topology: no block matches %s (available: %s)",
			strings.Join(unmatched, ", "), strings.Join(available, ", "))
	}

	if len(out.Blocks) == 0 {
		if len(out.SkippedBlocks) > 0 {
			reasons := make([]string, 0, len(out.SkippedBlocks))
			for _, skipped := range out.SkippedBlocks {
				reasons = append(reasons, fmt.Sprintf("%s: %s", skipped.Block, skipped.Reason))
			}
			sort.Strings(reasons)
			return nil, fmt.Errorf("topology: the selection has no compatible blocks (%s)", strings.Join(reasons, "; "))
		}
		if len(out.SkippedRunners) > 0 {
			reasons := make([]string, 0, len(out.SkippedRunners))
			for _, skipped := range out.SkippedRunners {
				reasons = append(reasons, fmt.Sprintf("%s/%s: %s", skipped.Block, skipped.Runner, skipped.Reason))
			}
			sort.Strings(reasons)
			return nil, fmt.Errorf("topology: the selection has no compatible runners (%s)", strings.Join(reasons, "; "))
		}
		return nil, fmt.Errorf("topology: the selection matched no blocks, so the run would test nothing")
	}

	return out, nil
}

// combineTags combines runner and selection tag expressions.
func combineTags(runnerTags, selectionTags string) string {
	switch {
	case runnerTags == "":
		return selectionTags
	case selectionTags == "":
		return runnerTags
	default:
		return runnerTags + " && " + selectionTags
	}
}

func cloneBlock(block ResolvedBlock) ResolvedBlock {
	out := block
	out.Components = make([]ResolvedComponent, len(block.Components))
	for i, component := range block.Components {
		out.Components[i] = component
		out.Components[i].DependsOn = append([]string(nil), component.DependsOn...)
		out.Components[i].ExternalParameters = make(map[string]string, len(component.ExternalParameters))
		for key, value := range component.ExternalParameters {
			out.Components[i].ExternalParameters[key] = value
		}
		out.Components[i].StagedFiles = maps.Clone(component.StagedFiles)
		out.Components[i].DBCompatibility = maps.Clone(component.DBCompatibility)
	}
	out.Runners = make([]Runner, len(block.Runners))
	for i, runner := range block.Runners {
		out.Runners[i] = runner
		out.Runners[i].Features = append([]string(nil), runner.Features...)
		if runner.GatewayVersion != nil {
			constraint := *runner.GatewayVersion
			out.Runners[i].GatewayVersion = &constraint
		}
	}
	return out
}

// selectGatewayDatabaseCompatibility excludes a Gateway database variant when its
// configured release boundary does not include the Gateway selected for this block.
func selectGatewayDatabaseCompatibility(block *ResolvedBlock) (*SkippedBlock, error) {
	if block == nil {
		return nil, fmt.Errorf("topology: resolved block is required")
	}
	for _, component := range block.Components {
		if component.Def == nil || component.Def.Name != "platform-gateway" {
			continue
		}
		constraint, restricted := component.DBCompatibility[component.DB]
		if !restricted {
			return nil, nil
		}
		target, err := gatewayVersionFor(block)
		if err != nil {
			return nil, err
		}
		if target.matches(constraint) {
			return nil, nil
		}
		return &SkippedBlock{
			Block: block.Name,
			Reason: fmt.Sprintf("Gateway version %s does not satisfy platform-gateway dbCompatibility for %s (%s)",
				target, component.DB, constraint),
		}, nil
	}
	return nil, nil
}

// PrintList writes a human-readable summary without starting components.
func PrintList(resolved *Resolved, out *os.File) {
	if resolved == nil {
		return
	}
	if out == nil {
		out = os.Stdout
	}
	summaries := Summarize(resolved)

	_, _ = fmt.Fprintf(out, "suite: %s\n", resolved.Name)
	_, _ = fmt.Fprintf(out, "concurrent blocks: %d\n", resolved.Parallel)
	_, _ = fmt.Fprintf(out, "propagation ceiling: %s, boot timeout: %s\n\n",
		resolved.Timeouts.Propagation, resolved.Timeouts.Boot)

	totalFeatures := 0
	for _, b := range summaries {
		_, _ = fmt.Fprintf(out, "block %s  (db=%s, %d concurrent runner(s))\n", b.Name, b.DB, b.Parallel)
		_, _ = fmt.Fprintf(out, "  components: %s\n", strings.Join(b.Components, ", "))
		for _, r := range b.Runners {
			tags := ""
			if r.Tags != "" {
				tags = fmt.Sprintf("  tags=%s", r.Tags)
			}
			_, _ = fmt.Fprintf(out, "  runner %s%s\n", r.Name, tags)
			for _, f := range r.Features {
				_, _ = fmt.Fprintf(out, "    - %s\n", f)
				totalFeatures++
			}
		}
		_, _ = fmt.Fprintln(out)
	}
	for _, skipped := range resolved.SkippedBlocks {
		_, _ = fmt.Fprintf(out, "skipped block %s: %s\n", skipped.Block, skipped.Reason)
	}
	if len(resolved.SkippedBlocks) > 0 {
		_, _ = fmt.Fprintln(out)
	}

	_, _ = fmt.Fprintf(out, "%d block(s), %d feature binding(s)\n", len(summaries), totalFeatures)
}

// BlockSummary contains the resolved values shown by PrintList.
type BlockSummary struct {
	Name       string
	DB         string
	Parallel   int
	Components []string
	Runners    []RunnerSummary
}

// RunnerSummary contains the runner values shown by PrintList.
type RunnerSummary struct {
	Name     string
	Features []string
	Tags     string
}

// Summarize renders a resolved suite without starting components.
func Summarize(resolved *Resolved) []BlockSummary {
	if resolved == nil {
		return nil
	}
	out := make([]BlockSummary, 0, len(resolved.Blocks))
	for i := range resolved.Blocks {
		b := &resolved.Blocks[i]

		components := make([]string, 0, len(b.Components))
		for _, c := range b.Components {
			label := "<undefined>"
			if c.Def != nil {
				label = c.Def.Name
			}
			if c.Replicas > 1 {
				label = fmt.Sprintf("%s x%d", label, c.Replicas)
			}
			if c.DB != "" {
				label = fmt.Sprintf("%s [%s]", label, c.DB)
			}
			components = append(components, label)
		}

		runners := make([]RunnerSummary, 0, len(b.Runners))
		for j := range b.Runners {
			r := &b.Runners[j]
			runners = append(runners, RunnerSummary{
				Name: r.Name, Features: append([]string(nil), r.Features...), Tags: r.Tags,
			})
		}

		out = append(out, BlockSummary{
			Name: b.Name, DB: string(b.DB), Parallel: b.EffectiveParallel(),
			Components: components, Runners: runners,
		})
	}
	return out
}
