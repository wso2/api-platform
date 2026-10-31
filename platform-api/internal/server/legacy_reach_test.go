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

// legacy_reach_test.go — a pre-tree scope name must not reach more than it did.
//
// testdata/pre-tree-scopes.yaml is every operation's accepted list as the specs
// declared it before the permission tree. For every scope name that appears
// there — renamed ones that legacy-scope-mapping.yaml now translates, and ones
// whose spelling never changed — this file compares the operations it reached
// then with the operations it reaches in the generated specs now, and allows
// exactly two kinds of growth: a read operation (the tree lets a parent's :read
// flow down to its sub-resources' reads) and the entries of allowedReachGrowth,
// each with its reason. Anything else is a silent privilege expansion for a
// token or IDP role that still carries the old name, and fails here.

package server

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

const preTreeScopesPath = "testdata/pre-tree-scopes.yaml"

// allowedReachGrowth lists, per pre-tree name, the non-read operations it may
// reach now although it did not before, and why. Keep it short: every entry is
// a conscious widening for every holder of that name.
var allowedReachGrowth = map[string]map[string]string{
	// LLM provider templates were a top-level resource (ap:llm_template:*); the
	// tree places them under the provider, so the provider's :manage covers them.
	"ap:llm_provider:manage": {
		"createLLMProviderTemplate":            "templates now live under ap:llm_provider",
		"copyLLMProviderTemplateVersion":       "templates now live under ap:llm_provider",
		"updateLLMProviderTemplate":            "templates now live under ap:llm_provider",
		"setLLMProviderTemplateVersionEnabled": "templates now live under ap:llm_provider",
		"deleteLLMProviderTemplateVersion":     "templates now live under ap:llm_provider",
	},
}

func loadPreTreeScopes(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(preTreeScopesPath)
	if err != nil {
		t.Fatalf("read %q: %v", preTreeScopesPath, err)
	}
	var doc struct {
		Operations map[string][]string `yaml:"operations"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %q: %v", preTreeScopesPath, err)
	}
	if len(doc.Operations) == 0 {
		t.Fatalf("%q lists no operations", preTreeScopesPath)
	}
	return doc.Operations
}

func TestLegacyNamesReachNoMoreThanBefore(t *testing.T) {
	before := loadPreTreeScopes(t)
	legacy := loadShippedLegacyScopeMap(t)

	now := map[string]specOperation{}
	for _, op := range loadSpecOperations(t) {
		now[op.key] = op
	}
	// The baseline is only meaningful while it describes the same operations.
	for key := range before {
		if _, ok := now[key]; !ok {
			t.Errorf("%s is in %s but no shipped spec declares it any more — update the baseline deliberately", key, preTreeScopesPath)
		}
	}
	for key := range now {
		if _, ok := before[key]; !ok {
			t.Logf("%s is new since the pre-tree spec; it has no baseline", key)
		}
	}

	names := map[string]bool{}
	for _, scopes := range before {
		for _, s := range scopes {
			names[s] = true
		}
	}

	checked, allowedReads, allowedListed := 0, 0, 0
	for _, name := range sortedKeys(names) {
		grants := append([]string{name}, legacy[name]...)
		for key, op := range now {
			reachedBefore := false
			for _, s := range before[key] {
				if s == name {
					reachedBefore = true
				}
			}
			reachesNow := false
			accepted := setOf(op.scopes)
			for _, g := range grants {
				if accepted[g] {
					reachesNow = true
				}
			}
			checked++
			if reachedBefore && !reachesNow {
				t.Errorf("%s reached %s before the tree and no longer does", name, key)
			}
			if !reachedBefore && reachesNow {
				switch {
				case isReadOperation(middleware.Operation{Method: op.method, Path: op.path}):
					allowedReads++ // a parent's :read flowing down to a sub-resource read
				case allowedReachGrowth[name][key] != "":
					allowedListed++
				default:
					t.Errorf("%s now reaches %s %s (%s), which it did not before the tree — a widening for every token still carrying that name; narrow the mapping or list it in allowedReachGrowth with a reason",
						name, op.method, op.path, key)
				}
			}
		}
	}
	for name, ops := range allowedReachGrowth {
		for key := range ops {
			if _, ok := now[key]; !ok {
				t.Errorf("allowedReachGrowth lists %s for %s, which no shipped spec declares", key, name)
			}
		}
	}
	t.Logf("%d pre-tree names × operations checked; %d read widenings, %d listed widenings allowed", checked, allowedReads, allowedListed)
}
