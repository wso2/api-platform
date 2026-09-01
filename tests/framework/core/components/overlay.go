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
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	roleBase    = "base config"
	roleOverlay = "overlay"
)

type mergeErrors struct{ errs []error }

func (l *mergeErrors) add(err error) {
	if err != nil {
		l.errs = append(l.errs, err)
	}
}

func (l *mergeErrors) addf(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

func (l *mergeErrors) err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return errors.Join(l.errs...)
}

// Vars contains values substituted into an overlay before parsing.
type Vars map[string]string

// VarBlock is the overlay variable name for the current block.
const VarBlock = "BLOCK"

// Merge loads a TOML base and overlays, merges them in order, and returns TOML output.
func Merge(base string, overlays ...string) ([]byte, error) {
	return MergeWithVars(nil, base, overlays...)
}

// MergeWithVars merges TOML layers after expanding variables in overlays.
func MergeWithVars(vars Vars, base string, overlays ...string) ([]byte, error) {
	if base == "" {
		return nil, errors.New("config merge: no base config path")
	}

	var errs mergeErrors
	baseTree, err := load(roleBase, base, nil)
	errs.add(err)

	trees := make([]map[string]any, 0, len(overlays))
	for _, path := range overlays {
		if path == "" {
			continue // an absent optional layer, not a mistake
		}
		tree, err := load(roleOverlay, path, vars)
		if err != nil {
			errs.add(err)
			continue
		}
		trees = append(trees, tree)
	}
	if err := errs.err(); err != nil {
		return nil, err
	}

	merged, err := MergeTrees(baseTree, trees...)
	if err != nil {
		return nil, err
	}

	out, err := toml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("config merge: encoding merged config: %w", err)
	}
	return out, nil
}

// Assemble merges a component's base, shared, and block overlays.
func Assemble(inj *ConfigInjection, repoRoot string, blockOverlay string, vars Vars) ([]byte, error) {
	if inj == nil {
		return nil, errors.New("config assembly: component declares no config injection")
	}
	if inj.Format != TOML {
		return nil, fmt.Errorf("config assembly: unsupported config format %q", inj.Format)
	}
	overlays := make([]string, 0, len(inj.ExtraOverlays)+2)
	overlays = append(overlays, resolve(repoRoot, inj.SharedOverlayPath))
	for _, extra := range inj.ExtraOverlays {
		overlays = append(overlays, resolve(repoRoot, extra))
	}
	overlays = append(overlays, resolve(repoRoot, blockOverlay))
	return MergeWithVars(vars, resolve(repoRoot, inj.BaseConfigPath), overlays...)
}

// MergeTrees deep-merges maps in order without modifying the inputs.
func MergeTrees(base map[string]any, overlays ...map[string]any) (map[string]any, error) {
	var errs mergeErrors
	out := cloneTree(base)
	for _, overlay := range overlays {
		mergeInto(out, overlay, "", &errs)
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	return out, nil
}

// mergeInto merges src onto dst in place and records type conflicts with their paths.
func mergeInto(dst, src map[string]any, path string, errs *mergeErrors) {
	for _, key := range slices.Sorted(maps.Keys(src)) {
		srcVal := src[key]
		keyPath := key
		if path != "" {
			keyPath = path + "." + key
		}

		dstVal, present := dst[key]
		if !present {
			dst[key] = cloneValue(srcVal)
			continue
		}

		dstTable, dstIsTable := dstVal.(map[string]any)
		srcTable, srcIsTable := srcVal.(map[string]any)
		switch {
		case dstIsTable && srcIsTable:
			mergeInto(dstTable, srcTable, keyPath, errs)
		case dstIsTable != srcIsTable:
			errs.addf("config overlay: key %q has type %s in the base but %s in the overlay",
				keyPath, typeName(dstVal), typeName(srcVal))
		default:
			dst[key] = cloneValue(srcVal)
		}
	}
}

// load reads and parses one TOML layer, expanding variables in overlays.
func load(role, path string, vars Vars) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config merge: reading %s %q: %w", role, path, err)
	}
	text := string(raw)
	if role == roleOverlay {
		text, err = expand(path, text, vars)
		if err != nil {
			return nil, err
		}
	}
	tree := map[string]any{}
	if err := toml.Unmarshal([]byte(text), &tree); err != nil {
		return nil, fmt.Errorf("config merge: parsing %s %q: %w", role, path, err)
	}
	return tree, nil
}

// expand replaces framework variables in overlay text.
func expand(path, text string, vars Vars) (string, error) {
	var b strings.Builder
	rest := text

	for {
		start := strings.Index(rest, "${")
		if start < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		b.WriteString(rest[:start])

		after := rest[start+2:]
		end := strings.Index(after, "}")
		if end < 0 {
			b.WriteString(rest[start:])
			return b.String(), nil
		}

		name := after[:end]
		if !isVarName(name) {
			b.WriteString(rest[start : start+2+end+1])
			rest = after[end+1:]
			continue
		}

		value, supplied := vars[name]
		switch {
		case !supplied:
			return "", fmt.Errorf("config merge: overlay %q references ${%s}, which the "+
				"framework does not supply here (it supplies: %v)", path, name, varNames(vars))
		case value == "":
			return "", fmt.Errorf("config merge: overlay %q references ${%s} and the framework "+
				"supplied an empty value for it", path, name)
		}
		b.WriteString(value)
		rest = after[end+1:]
	}
}

// isVarName reports whether name matches the framework variable syntax.
func isVarName(name string) bool {
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}

// varNames returns variable names in sorted order.
func varNames(vars Vars) []string {
	out := make([]string, 0, len(vars))
	for name := range vars {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// resolve maps a relative path beneath repoRoot and leaves empty paths unchanged.
func resolve(repoRoot, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(repoRoot, path)
}

// cloneTree deep-copies a TOML value tree.
func cloneTree(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, val := range src {
		out[key] = cloneValue(val)
	}
	return out
}

func cloneValue(val any) any {
	switch typed := val.(type) {
	case map[string]any:
		return cloneTree(typed)
	case []any:
		out := make([]any, len(typed))
		for i, elem := range typed {
			out[i] = cloneValue(elem)
		}
		return out
	default:
		return val
	}
}

// typeName returns a TOML-oriented name for a value.
func typeName(val any) string {
	switch val.(type) {
	case map[string]any:
		return "table"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int32, int64:
		return "integer"
	case float32, float64:
		return "float"
	case time.Time:
		return "datetime"
	case nil:
		return "nothing"
	default:
		return fmt.Sprintf("%T", val)
	}
}
