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

package it_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/topology"
)

// subject is the component this suite exists to test. Its engine is the axis every block sweeps.
const subject = "platform-gateway"

// sweptEngines is the coverage this suite must match: the three CI workflows
// (gateway-integration-test.yml and its -postgres/-sqlserver siblings) each run the WHOLE
// feature set once per engine.
var sweptEngines = []components.DBType{components.SQLite, components.Postgres, components.SQLServer}

func loadSuite(t *testing.T) *topology.Resolved {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	registry, err := catalog.Registry()
	require.NoError(t, err)
	resolved, err := topology.LoadFile(filepath.Join(dir, "it-suite.yaml"), registry)
	require.NoError(t, err)
	return resolved
}

// TestEveryBlockSweepsEveryEngine is the invariant that the declarative form deliberately does
// NOT encode.
//
// Expansion is declared per block, on the component that varies, so that a reader of a block can
// see that it runs three times. The cost of that choice is repetition: sixteen blocks each state
// the same matrix, and a block that quietly loses its matrix would simply run one engine and
// still pass. That is precisely the silent-coverage-loss failure the whole design is meant to
// prevent, so it is enforced here instead — loudly, by block name, at build time.
//
// A default that swept every block would have removed the repetition, but it could not tell
// "this block deliberately runs one engine" from "someone deleted the matrix". This test can:
// a deliberate exception goes in singleEngineBlocks below, with a reason.
func TestEveryBlockSweepsEveryEngine(t *testing.T) {
	// Blocks that deliberately do NOT sweep. Empty today. Add a block here only with a reason
	// it genuinely cannot run on every engine — not to silence a failure.
	singleEngineBlocks := map[string]string{}

	resolved := loadSuite(t)

	// Group resolved variants by the block they were declared as.
	variants := map[string][]components.DBType{}
	for i := range resolved.Blocks {
		b := &resolved.Blocks[i]
		if !runsSubject(b) {
			continue
		}
		variants[b.Source] = append(variants[b.Source], engineOf(b, subject))
	}
	require.NotEmpty(t, variants, "no block runs %q — has the suite been restructured?", subject)

	for _, source := range sortedBlockNames(variants) {
		got := variants[source]
		if reason, exempt := singleEngineBlocks[source]; exempt {
			require.Len(t, got, 1, "block %q is listed as single-engine (%s) but expanded to %d "+
				"variants; remove it from singleEngineBlocks", source, reason, len(got))
			continue
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		want := append([]components.DBType(nil), sweptEngines...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		require.Equal(t, want, got,
			"block %q does not cover every engine for %q. Add\n"+
				"    db:\n      matrix: [sqlite, postgres, sqlserver]\n"+
				"to its %q component, or list it in singleEngineBlocks with a reason.",
			source, subject, subject)
	}
}

func runsSubject(b *topology.ResolvedBlock) bool {
	for _, c := range b.Components {
		if c.Def.Name == subject {
			return true
		}
	}
	return false
}

func engineOf(b *topology.ResolvedBlock, component string) components.DBType {
	for _, c := range b.Components {
		if c.Def.Name == component {
			return c.DB
		}
	}
	return ""
}

func sortedBlockNames(m map[string][]components.DBType) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
