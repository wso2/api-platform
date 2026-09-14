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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// errorList accumulates validation failures.
type errorList struct{ errs []error }

func (l *errorList) add(err error) {
	if err != nil {
		l.errs = append(l.errs, err)
	}
}

func (l *errorList) addf(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

func (l *errorList) err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return errors.Join(l.errs...)
}

// Validate checks a resolved suite without accessing the filesystem or runtime.
func Validate(r *Resolved, registry *components.Registry) error {
	var errs errorList
	if r == nil {
		return fmt.Errorf("topology: resolved suite is required")
	}

	if strings.TrimSpace(r.Name) == "" {
		errs.addf("topology: the suite needs a name")
	}
	if len(r.Blocks) == 0 {
		errs.addf("topology: the suite declares no blocks")
	}
	if r.Cleanup.MaxAttempts < 0 {
		errs.addf("topology: cleanup max-attempts cannot be negative")
	}
	seenBlock := map[string]bool{}
	partitionOwner := map[string]string{}
	featureOwners := map[string]map[string]bool{}

	for i := range r.Blocks {
		b := &r.Blocks[i]

		if strings.TrimSpace(b.Name) == "" {
			errs.addf("topology: a block has no name")
			continue
		}
		if seenBlock[b.Name] {
			errs.addf("topology: duplicate block name %q", b.Name)
		}
		seenBlock[b.Name] = true

		key := b.PartitionKey()
		if key == "" {
			errs.addf("topology: block %q has no usable partition key: a block name needs at "+
				"least one letter or digit", b.Name)
		} else if owner, taken := partitionOwner[key]; taken {
			errs.addf("topology: blocks %q and %q both resolve to partition key %q; they would "+
				"share one bucket in every partitioned testbench service", owner, b.Name, key)
		} else {
			partitionOwner[key] = b.Name
		}

		errs.add(validateBlockComponents(b))
		errs.add(validateBlockRunners(b, featureOwners))
	}

	for feature, owners := range featureOwners {
		if len(owners) > 1 {
			names := make([]string, 0, len(owners))
			for owner := range owners {
				names = append(names, owner)
			}
			sort.Strings(names)
			errs.addf("topology: feature %q is bound to %d runners (%s); it would run more than once",
				feature, len(names), strings.Join(names, ", "))
		}
	}

	return errs.err()
}

func validateBlockComponents(b *ResolvedBlock) error {
	var errs errorList

	if len(b.Components) == 0 {
		errs.addf("block %q: declares no components", b.Name)
	}

	present := map[string]bool{}
	for _, c := range b.Components {
		if c.Def == nil {
			errs.addf("block %q: component has no definition", b.Name)
			continue
		}
		present[c.Def.Name] = true
	}

	for _, c := range b.Components {
		if c.Def == nil {
			continue
		}
		if c.Def.DB != nil && !c.Def.DB.Owns() && !present[c.Def.DB.SharesStoreWith] {
			errs.addf("block %q: component %q shares %q, which is not in this block",
				b.Name, c.Def.Name, c.Def.DB.SharesStoreWith)
		}

		for _, dep := range c.AllDependencies() {
			if !present[dep] {
				errs.addf("block %q: component %q depends on %q, which this block does not run",
					b.Name, c.Def.Name, dep)
			}
		}

		if filepath.IsAbs(c.Overlay) {
			errs.addf("block %q: component %q overlay %q must be repo-relative, not absolute",
				b.Name, c.Def.Name, c.Overlay)
		}

		if c.Def.Shared {
			if c.Def.DB != nil && c.Def.DB.Owns() {
				errs.addf("block %q: component %q owns storage and cannot be shared across blocks",
					b.Name, c.Def.Name)
			}
			if c.Replicas > 1 {
				errs.addf("block %q: component %q is shared and cannot have %d replicas",
					b.Name, c.Def.Name, c.Replicas)
			}
		}

		if c.Def.IsCompose() && c.Replicas > 1 {
			errs.addf("block %q: compose component %q cannot have %d replicas",
				b.Name, c.Def.Name, c.Replicas)
		}

		if c.Replicas > 1 && c.Def.DB != nil && !c.Def.DB.Owns() {
			errs.addf("block %q: component %q shares another component's store and cannot be replicated",
				b.Name, c.Def.Name)
		}
	}

	return errs.err()
}

func validateBlockRunners(b *ResolvedBlock, featureOwners map[string]map[string]bool) error {
	var errs errorList

	if len(b.Runners) == 0 {
		errs.addf("block %q: declares no runners, so its topology would boot and do nothing", b.Name)
	}

	seenRunner := map[string]bool{}
	for i := range b.Runners {
		run := &b.Runners[i]

		if strings.TrimSpace(run.Name) == "" {
			errs.addf("block %q: a runner has no name", b.Name)
			continue
		}
		if seenRunner[run.Name] {
			errs.addf("block %q: duplicate runner name %q", b.Name, run.Name)
		}
		seenRunner[run.Name] = true

		if len(run.Features) == 0 {
			errs.addf("block %q runner %q: declares no features", b.Name, run.Name)
		}

		seenFeature := map[string]bool{}
		for _, f := range run.Features {
			if strings.TrimSpace(f) == "" {
				errs.addf("block %q runner %q: has an empty feature path", b.Name, run.Name)
				continue
			}
			if seenFeature[f] {
				errs.addf("block %q runner %q: feature %q listed twice", b.Name, run.Name, f)
				continue
			}
			seenFeature[f] = true
			owner := b.Source + "/" + run.Name
			if featureOwners[f] == nil {
				featureOwners[f] = map[string]bool{}
			}
			featureOwners[f][owner] = true
		}
	}

	return errs.err()
}

// ValidateFeatureFiles checks that referenced feature files exist and are files.
func ValidateFeatureFiles(r *Resolved, root string) error {
	var errs errorList
	if r == nil {
		return fmt.Errorf("topology: resolved suite is required")
	}
	checked := map[string]bool{}

	for i := range r.Blocks {
		b := &r.Blocks[i]
		for j := range b.Runners {
			run := &b.Runners[j]
			for _, f := range run.Features {
				if checked[f] {
					continue
				}
				checked[f] = true

				path := f
				if !filepath.IsAbs(path) && root != "" {
					path = filepath.Join(root, f)
				}
				info, err := os.Stat(path)
				switch {
				case err != nil:
					errs.addf("block %q runner %q: feature %q does not exist", b.Name, run.Name, f)
				case info.IsDir():
					errs.addf("block %q runner %q: feature %q is a directory", b.Name, run.Name, f)
				case !strings.HasSuffix(f, ".feature"):
					errs.addf("block %q runner %q: %q is not a .feature file", b.Name, run.Name, f)
				}
			}
		}
	}

	return errs.err()
}

// ValidateHooks checks that runner hooks are registered.
func ValidateHooks(r *Resolved, registered map[string]bool) error {
	var errs errorList
	if r == nil {
		return fmt.Errorf("topology: resolved suite is required")
	}
	for i := range r.Blocks {
		b := &r.Blocks[i]
		for j := range b.Runners {
			run := &b.Runners[j]
			if run.Hook == "" {
				continue
			}
			if !registered[run.Hook] {
				known := make([]string, 0, len(registered))
				for name := range registered {
					known = append(known, name)
				}
				sort.Strings(known)
				errs.addf("block %q runner %q: unknown hook %q (registered: %s)",
					b.Name, run.Name, run.Hook, strings.Join(known, ", "))
			}
		}
	}
	return errs.err()
}
