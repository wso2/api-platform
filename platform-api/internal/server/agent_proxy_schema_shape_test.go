/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package server

import (
	"os"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The Agent proxy oneOf schemas carry their properties twice: once on the parent,
// describing the shape common to every branch, and once inside each branch, where
// the mode-specific rules live. The parent copy is what lets the Go and
// TypeScript generators emit a usable object type instead of an opaque union; the
// branches are what actually validate.
//
// That duplication fails quietly in one direction. A property added to a branch
// but not to the parent still validates correctly — the branch governs — but it
// silently never appears in any generated client, which is the same class of bug
// that made these schemas worth changing in the first place. This test is the
// guard: the parent's property set must remain a superset of every branch's.
//
// It is deliberately one-directional. The parent may legitimately carry a
// property no single branch declares, because it holds the union across branches.

type oneOfSchemaShape struct {
	Properties map[string]yaml.Node `yaml:"properties"`
	OneOf      []struct {
		Ref string `yaml:"$ref"`
	} `yaml:"oneOf"`
}

func loadOneOfSchemas(t *testing.T) map[string]oneOfSchemaShape {
	t.Helper()

	data, err := os.ReadFile(realSpecPath)
	if err != nil {
		t.Fatalf("read %q: %v", realSpecPath, err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]oneOfSchemaShape `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %q: %v", realSpecPath, err)
	}
	return doc.Components.Schemas
}

func sortedPropertyNames(s oneOfSchemaShape) []string {
	names := make([]string, 0, len(s.Properties))
	for n := range s.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func TestAgentProxyOneOfParentsDeclareEveryBranchProperty(t *testing.T) {
	schemas := loadOneOfSchemas(t)

	const refPrefix = "#/components/schemas/"
	for _, parent := range []string{"PublicAgentCard", "ProtectedAgentCard", "FetchAgentCardRequest"} {
		t.Run(parent, func(t *testing.T) {
			schema, ok := schemas[parent]
			if !ok {
				t.Fatalf("schema %q not found in the spec", parent)
			}
			if len(schema.OneOf) == 0 {
				t.Fatalf("%s has no oneOf branches — this test is guarding the wrong schema", parent)
			}
			if len(schema.Properties) == 0 {
				t.Fatalf("%s declares no properties on the parent, so generators emit an opaque "+
					"union instead of a usable object type", parent)
			}

			declared := make(map[string]struct{}, len(schema.Properties))
			for _, n := range sortedPropertyNames(schema) {
				declared[n] = struct{}{}
			}

			for _, branchRef := range schema.OneOf {
				if !strings.HasPrefix(branchRef.Ref, refPrefix) {
					t.Fatalf("%s: branch $ref %q is not a local component reference", parent, branchRef.Ref)
				}
				branchName := strings.TrimPrefix(branchRef.Ref, refPrefix)
				branch, ok := schemas[branchName]
				if !ok {
					t.Fatalf("%s: branch schema %q not found", parent, branchName)
				}
				for _, prop := range sortedPropertyNames(branch) {
					if _, ok := declared[prop]; !ok {
						t.Errorf("%s.%s is declared on branch %s but not on the parent — it will "+
							"validate correctly and be missing from every generated client",
							parent, prop, branchName)
					}
				}
			}
		})
	}
}
