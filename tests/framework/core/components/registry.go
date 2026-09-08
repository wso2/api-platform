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

package components

import (
	"fmt"
	"sort"
)

// Registry stores component definitions by name.
type Registry struct {
	byName map[string]*Definition
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]*Definition)}
}

// Register validates and adds a definition.
func (r *Registry) Register(d *Definition) error {
	if d == nil {
		return fmt.Errorf("registry: cannot register a nil definition")
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if r.byName == nil {
		r.byName = make(map[string]*Definition)
	}
	if _, exists := r.byName[d.Name]; exists {
		return fmt.Errorf("registry: %s is already registered", d)
	}
	r.byName[d.Name] = d
	return nil
}

// MustRegister registers definitions and panics on failure.
func (r *Registry) MustRegister(defs ...*Definition) *Registry {
	for _, d := range defs {
		if err := r.Register(d); err != nil {
			panic(err)
		}
	}
	return r
}

// Lookup returns the named definition.
func (r *Registry) Lookup(name string) (*Definition, bool) {
	d, ok := r.byName[name]
	return d, ok
}

// Names returns every registered name, sorted, for error messages and --list.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Len returns the number of registered components.
func (r *Registry) Len() int { return len(r.byName) }

// Validate checks relationships between registered definitions.
func (r *Registry) Validate() error {
	var errs errorList

	for _, name := range r.Names() {
		d := r.byName[name]

		for _, dep := range d.DependsOn {
			if _, ok := r.byName[dep]; !ok {
				errs.addf("%s: dependsOn unknown component %q", d, dep)
			}
			if dep == d.Name {
				errs.addf("%s: dependsOn itself", d)
			}
		}

		if d.DB != nil && d.DB.SharesStoreWith != "" {
			owner, ok := r.byName[d.DB.SharesStoreWith]
			switch {
			case !ok:
				errs.addf("%s: sharesStoreWith unknown component %q", d, d.DB.SharesStoreWith)
			case owner.Name == d.Name:
				errs.addf("%s: sharesStoreWith itself", d)
			case owner.DB == nil:
				errs.addf("%s: sharesStoreWith %q, which has no storage", d, owner.Name)
			case !owner.DB.Owns():
				errs.addf("%s: sharesStoreWith %q, which itself shares another store", d, owner.Name)
			default:
				if !hasCommonDBType(d.DB.Supported, owner.DB.Supported) {
					errs.addf("%s: shares %q's store but supports no engine in common with it", d, owner.Name)
				}
			}
		}
	}

	if err := r.validateCycles(); err != nil {
		errs.add(err)
	}

	return errs.err()
}

// validateCycles checks the dependency graph for cycles.
func (r *Registry) validateCycles() error {
	const (
		unvisited = 0
		active    = 1
		done      = 2
	)
	state := make(map[string]int, len(r.byName))

	var walk func(name string, path []string) error
	walk = func(name string, path []string) error {
		switch state[name] {
		case active:
			return fmt.Errorf("registry: dependency cycle: %s -> %s", joinPath(path), name)
		case done:
			return nil
		}
		state[name] = active
		if d, ok := r.byName[name]; ok {
			for _, dep := range d.DependsOn {
				if _, known := r.byName[dep]; !known {
					continue // already reported by Validate
				}
				if err := walk(dep, append(path, name)); err != nil {
					return err
				}
			}
		}
		state[name] = done
		return nil
	}

	for _, name := range r.Names() {
		if err := walk(name, nil); err != nil {
			return err
		}
	}
	return nil
}

func hasCommonDBType(a, b []DBType) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func joinPath(path []string) string {
	if len(path) == 0 {
		return "(root)"
	}
	out := path[0]
	for _, p := range path[1:] {
		out += " -> " + p
	}
	return out
}
