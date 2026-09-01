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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

func testRegistry(t *testing.T) *components.Registry {
	t.Helper()

	type gwWiring struct {
		ControlPlaneHost string `yaml:"controlPlaneHost"`
		LogLevel         string `yaml:"logLevel"`
	}

	owner := &components.Definition{
		Name: "gateway-controller", Image: components.ImageRef{Ref: "gc:test"}, Alias: "gateway-controller",
		Endpoints: []components.Endpoint{{Name: "admin", Port: 9092, Scheme: "http"}},
		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			Schema: map[components.DBType][]string{
				components.Postgres:  {"gc.postgres.sql"},
				components.SQLServer: {"gc.sqlserver.sql"},
			},
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          func(components.DSN) map[string]string { return nil },
		},
		Wiring: components.TypedWiring[gwWiring](),
	}
	runtime := &components.Definition{
		Name: "gateway-runtime", Image: components.ImageRef{Ref: "rt:test"}, Alias: "gateway-runtime",
		Endpoints: []components.Endpoint{{Name: "http", Port: 8080, Scheme: "http"}},
		DependsOn: []string{"gateway-controller"},
	}
	sharer := &components.Definition{
		Name: "mock-platform-api", Image: components.ImageRef{Ref: "m:test"}, Alias: "mock-platform-api",
		Endpoints: []components.Endpoint{{Name: "https", Port: 9243, Scheme: "https"}},
		DB: &components.DBContract{
			Supported:       []components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			SharesStoreWith: "gateway-controller",
			Env:             func(components.DSN) map[string]string { return nil },
		},
	}
	pgOnly := &components.Definition{
		Name: "api-portal", Image: components.ImageRef{Ref: "ap:test"}, Alias: "api-portal",
		Endpoints: []components.Endpoint{{Name: "http", Port: 9080, Scheme: "http"}},
		DB: &components.DBContract{
			Supported: []components.DBType{components.Postgres},
			Schema:    map[components.DBType][]string{components.Postgres: {"ap.postgres.sql"}},
			Env:       func(components.DSN) map[string]string { return nil },
		},
	}
	stateless := &components.Definition{
		Name: "mock-jwks", Image: components.ImageRef{Ref: "j:test"}, Alias: "mock-jwks",
		Endpoints: []components.Endpoint{{Name: "http", Port: 8080, Scheme: "http"}},
	}
	fixed := &components.Definition{
		Name: "kafka", Image: components.ImageRef{Ref: "k:test"}, Alias: "kafka",
		AliasIsFixed: true,
		Endpoints:    []components.Endpoint{{Name: "tcp", Port: 9092, Scheme: "tcp"}},
	}

	r := components.NewRegistry()
	require.NoError(t, r.Register(owner))
	require.NoError(t, r.Register(runtime))
	require.NoError(t, r.Register(sharer))
	require.NoError(t, r.Register(pgOnly))
	require.NoError(t, r.Register(stateless))
	require.NoError(t, r.Register(fixed))
	require.NoError(t, r.Validate())
	return r
}

const minimalSuite = `
suite: api-platform-it
defaults:
  parallel: 3
  db:
    gateway-controller: sqlite
    api-portal: postgres
blocks:
  - name: gateway-core
    parallel: 2
    components:
      - name: gateway-controller
      - name: gateway-runtime
      - name: mock-jwks
    runners:
      - name: api-deploy
        features: [features/api_deploy.feature]
`

func load(t *testing.T, src string) (*Resolved, error) {
	t.Helper()
	return Load([]byte(src), testRegistry(t))
}

func TestLoadMinimal(t *testing.T) {
	r, err := load(t, minimalSuite)
	require.NoError(t, err)

	require.Equal(t, "api-platform-it", r.Name)
	require.Equal(t, 3, r.Parallel)
	require.Len(t, r.Blocks, 1)

	b := r.Blocks[0]
	require.Equal(t, "gateway-core", b.Name)
	require.Equal(t, 2, b.Parallel)
	require.Len(t, b.Components, 3)
}

func TestDefaultsAreApplied(t *testing.T) {
	t.Run("timeouts fall back when unstated", func(t *testing.T) {
		r, err := load(t, minimalSuite)
		require.NoError(t, err)
		require.Equal(t, DefaultBootTimeout, r.Timeouts.Boot)
		require.Equal(t, DefaultPropagationTimeout, r.Timeouts.Propagation)
	})

	t.Run("declared timeouts win", func(t *testing.T) {
		src := `
suite: s
defaults:
  db:
    gateway-controller: sqlite
    api-portal: postgres
  timeouts:
    boot: 90s
    propagation: 240s
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [f.feature]}]
`
		r, err := load(t, src)
		require.NoError(t, err)
		require.Equal(t, 90*time.Second, r.Timeouts.Boot)
		require.Equal(t, 240*time.Second, r.Timeouts.Propagation)
	})

	t.Run("cleanup retries are optional and resolved", func(t *testing.T) {
		src := `
suite: s
defaults:
  cleanup:
    max-attempts: 3
  db:
    gateway-controller: sqlite
blocks:
  - name: b
    components: [{name: gateway-controller}]
    runners: [{name: r, features: [f.feature]}]
`
		r, err := load(t, src)
		require.NoError(t, err)
		require.Equal(t, 3, r.Cleanup.MaxAttempts)

		defaultResolved, err := load(t, minimalSuite)
		require.NoError(t, err)
		require.Zero(t, defaultResolved.Cleanup.MaxAttempts)
	})

	t.Run("negative cleanup attempts are rejected", func(t *testing.T) {
		_, err := load(t, `
suite: s
defaults:
  cleanup:
    max-attempts: -1
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [f.feature]}]
`)
		require.ErrorContains(t, err, "cleanup max-attempts cannot be negative")
	})

	t.Run("a runner may not declare scenario concurrency", func(t *testing.T) {
		_, err := load(t, `
suite: s
blocks:
  - name: b
    components:
      - name: gateway-controller
    runners:
      - name: r
        concurrency: 4
        features: [features/api_deploy.feature]
`)
		require.Error(t, err)
		require.Contains(t, err.Error(), "concurrency")
	})
}

func TestComponentDefaultsResolveDatabaseAndVersion(t *testing.T) {
	src := `
suite: s
defaults:
  components:
    gateway-controller:
      db: sqlite
      version: legacy
blocks:
  - name: b
    components:
      - name: gateway-controller
        version: block-version
    runners: [{name: r, features: [f.feature]}]
`
	r, err := load(t, src)
	require.NoError(t, err)
	require.Len(t, r.Blocks, 1)
	require.Equal(t, components.SQLite, r.Blocks[0].Components[0].DB)
	require.Equal(t, "block-version", r.Blocks[0].Components[0].Version)
	require.Equal(t, "gc:block-version", r.Blocks[0].Components[0].Def.Image.Ref)
}

func TestComponentDefaultVersionAppliesWhenBlockOmitsIt(t *testing.T) {
	src := `
suite: s
defaults:
  components:
    gateway-controller:
      db: sqlite
      version: legacy
blocks:
  - name: b
    components: [{name: gateway-controller}]
    runners: [{name: r, features: [f.feature]}]
`
	r, err := load(t, src)
	require.NoError(t, err)
	require.Equal(t, "legacy", r.Blocks[0].Components[0].Version)
	require.Equal(t, "gc:legacy", r.Blocks[0].Components[0].Def.Image.Ref)
}

func TestDBResolutionOrder(t *testing.T) {
	src := `
suite: s
defaults:
  db:
    gateway-controller: sqlite
    api-portal: postgres
blocks:
  - name: mixed
    components:
      - name: gateway-controller
        db: sqlite
      - name: api-portal
      - name: mock-jwks
    runners: [{name: r, features: [f.feature]}]
`
	r, err := load(t, src)
	require.NoError(t, err)
	b := r.Blocks[0]

	byName := map[string]ResolvedComponent{}
	for _, c := range b.Components {
		byName[c.Def.Name] = c
	}

	require.Equal(t, components.SQLite, byName["gateway-controller"].DB)
	require.Equal(t, components.Postgres, byName["api-portal"].DB)
	require.Empty(t, byName["mock-jwks"].DB)
}

func TestMatrixExpansion(t *testing.T) {
	src := `
suite: s
defaults:
  db:
    gateway-controller: sqlite
    api-portal: postgres
blocks:
  - name: gateway-core
    components:
      - name: gateway-controller
        db:
          matrix: [sqlite, postgres, sqlserver]
      - name: mock-platform-api
      - name: api-portal
    runners: [{name: r, features: [f.feature]}]
`
	r, err := load(t, src)
	require.NoError(t, err)

	require.Len(t, r.Blocks, 3)
	require.Equal(t, []string{"gateway-core/sqlite", "gateway-core/postgres", "gateway-core/sqlserver"},
		r.BlockNames())

	for _, b := range r.Blocks {
		require.Equal(t, "gateway-core", b.Source)
	}

	lite, ok := r.Block("gateway-core/sqlite")
	require.True(t, ok)
	require.Equal(t, components.SQLite, lite.DB)

	byName := map[string]ResolvedComponent{}
	for _, c := range lite.Components {
		byName[c.Def.Name] = c
	}

	require.Equal(t, components.SQLite, byName["gateway-controller"].DB)

	require.Equal(t, components.Postgres, byName["api-portal"].DB,
		"an independent sibling must keep its own engine across the matrix component's variants")

	require.Equal(t, components.SQLite, byName["mock-platform-api"].DB,
		"a sharer must follow its owner's engine through the matrix")
}

func TestStrictParsing(t *testing.T) {
	t.Run("an unknown key is rejected", func(t *testing.T) {
		src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    paralell: 4
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [f.feature]}]
`
		_, err := load(t, src)
		require.ErrorContains(t, err, "paralell")
	})

	t.Run("a stale version key is rejected", func(t *testing.T) {
		_, err := load(t, "version: 1\nsuite: s\nblocks: []\n")
		require.ErrorContains(t, err, "field version not found")
	})

	t.Run("a second YAML document is rejected", func(t *testing.T) {
		_, err := load(t, "suite: s\nblocks: []\n---\nsuite: ignored\n")
		require.ErrorContains(t, err, "multiple YAML documents")
	})

	t.Run("unknown fields in a database mapping are rejected", func(t *testing.T) {
		_, err := load(t, `
suite: s
blocks:
  - name: b
    components:
      - name: gateway-controller
        db: {matrix: [sqlite], matrx: [postgres]}
    runners: [{name: r, features: [f.feature]}]
`)
		require.ErrorContains(t, err, `unknown db field "matrx"`)
	})
}

func TestWiring(t *testing.T) {
	t.Run("typed wiring decodes", func(t *testing.T) {
		src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: gateway-controller}]
    wiring:
      gateway-controller:
        controlPlaneHost: platform-api:9443
        logLevel: debug
    runners: [{name: r, features: [f.feature]}]
`
		r, err := load(t, src)
		require.NoError(t, err)
		require.NotNil(t, r.Blocks[0].Components[0].Wiring)
	})

	t.Run("an unknown wiring key is rejected", func(t *testing.T) {
		src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: gateway-controller}]
    wiring:
      gateway-controller:
        controlPlainHost: oops
    runners: [{name: r, features: [f.feature]}]
`
		_, err := load(t, src)
		require.ErrorContains(t, err, "controlPlainHost")
	})

	t.Run("wiring for a component that accepts none is rejected", func(t *testing.T) {
		src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    wiring:
      mock-jwks: {anything: 1}
    runners: [{name: r, features: [f.feature]}]
`
		_, err := load(t, src)
		require.ErrorContains(t, err, "accepts no wiring")
	})

	t.Run("wiring for an absent component is rejected", func(t *testing.T) {
		src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    wiring:
      gateway-controller: {logLevel: debug}
    runners: [{name: r, features: [f.feature]}]
`
		_, err := load(t, src)
		require.ErrorContains(t, err, "which this block does not run")
	})
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name, src, wants string
	}{
		{
			name: "unknown component",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: no-such-thing}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: `unknown component "no-such-thing"`,
		},
		{
			name: "component listed twice",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}, {name: mock-jwks}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: "listed twice (use replicas instead)",
		},
		{
			name: "unsupported engine for a component",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: api-portal, db: sqlite}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: `does not support db "sqlite"`,
		},
		{
			name: "replicas on a fixed alias",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: kafka, replicas: 2}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: "fixed alias and cannot have replicas",
		},
		{
			name: "duplicate block name",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: dup
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [a.feature]}]
  - name: dup
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [b.feature]}]
`,
			wants: `duplicate block name "dup"`,
		},
		{
			name: "block with no runners",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners: []
`,
			wants: "boot and do nothing",
		},
		{
			name: "missing dependency in block",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: gateway-runtime}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: "which this block does not run",
		},
		{
			name: "sharer without its owner",
			src: `
suite: s
defaults: {db: {gateway-controller: postgres, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-platform-api}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: "is not in this block",
		},
		{
			name: "replicated sharer",
			src: `
suite: s
defaults: {db: {gateway-controller: postgres, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: gateway-controller}, {name: mock-platform-api, replicas: 2}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: "cannot be replicated",
		},
		{
			name: "duplicate runner name",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners:
      - {name: r, features: [a.feature]}
      - {name: r, features: [b.feature]}
`,
			wants: `duplicate runner name "r"`,
		},
		{
			name: "feature bound to two runners",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners:
      - {name: one, features: [shared.feature]}
      - {name: two, features: [shared.feature]}
`,
			wants: "would run more than once",
		},
		{
			name: "absolute overlay path",
			src: `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks, overlay: /etc/whatever.toml}]
    runners: [{name: r, features: [f.feature]}]
`,
			wants: "must be repo-relative",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(t, tc.src)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wants)
		})
	}
}

func TestAllProblemsReportedAtOnce(t *testing.T) {
	src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: nope}, {name: also-nope}]
    runners: [{name: r, features: [f.feature]}]
`
	_, err := load(t, src)
	require.ErrorContains(t, err, `unknown component "nope"`)
	require.ErrorContains(t, err, `unknown component "also-nope"`)
}

func TestValidateFeatureFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "features"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "features", "api_deploy.feature"),
		[]byte("Feature: x\n"), 0o644))

	r, err := load(t, minimalSuite)
	require.NoError(t, err)

	t.Run("an existing feature passes", func(t *testing.T) {
		require.NoError(t, ValidateFeatureFiles(r, root))
	})

	t.Run("a missing feature is caught", func(t *testing.T) {
		require.ErrorContains(t, ValidateFeatureFiles(r, t.TempDir()), "does not exist")
	})

	t.Run("a non-feature path is caught", func(t *testing.T) {
		other := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(other, "features"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(other, "features", "api_deploy.feature"),
			[]byte("x"), 0o644))

		bad := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [features/notes.txt]}]
`
		rb, err := load(t, bad)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(other, "features", "notes.txt"), []byte("x"), 0o644))
		require.ErrorContains(t, ValidateFeatureFiles(rb, other), "is not a .feature file")
	})
}

func TestValidateHooks(t *testing.T) {
	src := `
suite: s
defaults: {db: {gateway-controller: sqlite, api-portal: postgres}}
blocks:
  - name: b
    components: [{name: mock-jwks}]
    runners: [{name: r, features: [f.feature], hook: restartBetweenScenarios}]
`
	r, err := load(t, src)
	require.NoError(t, err)

	t.Run("a registered hook passes", func(t *testing.T) {
		require.NoError(t, ValidateHooks(r, map[string]bool{"restartBetweenScenarios": true}))
	})

	t.Run("an unknown hook is caught and lists what is registered", func(t *testing.T) {
		err := ValidateHooks(r, map[string]bool{"somethingElse": true})
		require.ErrorContains(t, err, `unknown hook "restartBetweenScenarios"`)
		require.ErrorContains(t, err, "registered: somethingElse")
	})
}

func TestNilInputs(t *testing.T) {
	t.Run("validation functions reject a nil resolved suite", func(t *testing.T) {
		require.ErrorContains(t, Validate(nil, nil), "resolved suite is required")
		require.ErrorContains(t, ValidateFeatureFiles(nil, ""), "resolved suite is required")
		require.ErrorContains(t, ValidateHooks(nil, nil), "resolved suite is required")
	})

	t.Run("selection rejects a nil resolved suite", func(t *testing.T) {
		_, err := (Selection{}).Apply(nil)
		require.ErrorContains(t, err, "resolved suite is required")
	})

	t.Run("summary helpers handle nil values", func(t *testing.T) {
		require.Nil(t, Summarize(nil))
		require.Empty(t, (*ResolvedBlock)(nil).PartitionKey())
		require.Equal(t, 1, (*ResolvedBlock)(nil).EffectiveParallel())
		block, ok := (*Resolved)(nil).Block("missing")
		require.Nil(t, block)
		require.False(t, ok)
		require.Empty(t, (*Resolved)(nil).BlockNames())
		require.Equal(t, 1, (*Block)(nil).EffectiveParallel())
		require.Equal(t, 1, (*Component)(nil).EffectiveReplicas())
		require.NotPanics(t, func() { PrintList(nil, nil) })
	})

	t.Run("resolved components without definitions are reported", func(t *testing.T) {
		r := &Resolved{Name: "suite", Blocks: []ResolvedBlock{{
			Name: "block", Components: []ResolvedComponent{{}},
		}}}
		require.ErrorContains(t, Validate(r, nil), "component has no definition")
		require.Empty(t, r.Blocks[0].Components[0].AllDependencies())
		require.NotPanics(t, func() { Summarize(r) })
	})
}

func TestPartitionKey(t *testing.T) {
	cases := map[string]string{
		"gateway-analytics":     "gateway-analytics",
		"gateway-core/postgres": "gateway-core-postgres", // matrix variants must not share one
		"gateway-core/sqlite":   "gateway-core-sqlite",
		"Gateway Core":          "gateway-core", // URL-safe: lowercased, spaces dashed
		"-gateway-":             "gateway",      // no leading or trailing dash to double up
	}
	for name, want := range cases {
		b := &ResolvedBlock{Name: name}
		require.Equal(t, want, b.PartitionKey(), "partition key for block %q", name)
	}
}

func TestPartitionKeysMustNotCollide(t *testing.T) {
	const src = `
suite: api-platform-it
defaults:
  parallel: 2
  db:
    gateway-controller: sqlite
    api-portal: postgres
blocks:
  - name: gateway.analytics
    components:
      - name: mock-jwks
    runners:
      - name: one
        features: [features/api_deploy.feature]
  - name: gateway-analytics
    components:
      - name: mock-jwks
    runners:
      - name: two
        features: [features/api_errors.feature]
`
	_, err := load(t, src)
	require.ErrorContains(t, err, "partition key")
	require.ErrorContains(t, err, "gateway.analytics")
	require.ErrorContains(t, err, "gateway-analytics")
}

func TestDBSpecGrammar(t *testing.T) {
	decode := func(t *testing.T, src string) (DBSpec, error) {
		t.Helper()
		var holder struct {
			DB DBSpec `yaml:"db"`
		}
		err := yaml.Unmarshal([]byte(src), &holder)
		return holder.DB, err
	}

	t.Run("scalar is a single engine", func(t *testing.T) {
		got, err := decode(t, "db: postgres")
		require.NoError(t, err)
		require.Equal(t, components.Postgres, got.One)
		require.Equal(t, "postgres:16-alpine", got.Variant)
		require.False(t, got.IsMatrix())
		require.False(t, got.IsZero())
	})

	t.Run("versioned scalar selects a supported variant", func(t *testing.T) {
		got, err := decode(t, "db: postgres:17-alpine")
		require.NoError(t, err)
		require.Equal(t, components.Postgres, got.One)
		require.Equal(t, "postgres:17-alpine", got.Variant)
	})

	t.Run("mapping form declares a matrix", func(t *testing.T) {
		got, err := decode(t, "db: {matrix: [sqlite, postgres, sqlserver]}")
		require.NoError(t, err)
		require.True(t, got.IsMatrix())
		require.Equal(t,
			[]components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			got.Matrix)
		require.Empty(t, got.One)
	})

	t.Run("absent means fall back to defaults.components", func(t *testing.T) {
		got, err := decode(t, "other: value")
		require.NoError(t, err)
		require.True(t, got.IsZero())
	})

	t.Run("a bare list is refused and says what to write instead", func(t *testing.T) {
		_, err := decode(t, "db: [sqlite, postgres]")
		require.Error(t, err)
		require.ErrorContains(t, err, "bare list is not accepted")
		require.ErrorContains(t, err, "matrix")
	})

	t.Run("an empty matrix is an error, not a silent single engine", func(t *testing.T) {
		_, err := decode(t, "db: {matrix: []}")
		require.Error(t, err)
		require.ErrorContains(t, err, "empty")
	})

	t.Run("mapping form retains versioned variant keys", func(t *testing.T) {
		got, err := decode(t, "db: {matrix: [postgres:16-alpine, postgres:17-alpine]}")
		require.NoError(t, err)
		require.Equal(t, []string{"postgres:16-alpine", "postgres:17-alpine"}, got.Variants)
		require.Equal(t, []components.DBType{components.Postgres, components.Postgres}, got.Matrix)
	})

	t.Run("unsupported variants are rejected", func(t *testing.T) {
		_, err := decode(t, "db: postgres:15-alpine")
		require.ErrorContains(t, err, "unsupported database variant")
	})
}

func TestBlocksCanSelectDifferentDatabaseVariants(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
suite: variants
blocks:
  - name: postgres-16
    components:
      - name: gateway-controller
        db: postgres:16-alpine
    runners:
      - name: smoke-16
        features: [smoke.feature]
  - name: postgres-17
    components:
      - name: gateway-controller
        db: postgres:17-alpine
    runners:
      - name: smoke-17
        features: [smoke-17.feature]
`)
	resolved, err := Load(raw, registry)
	require.NoError(t, err)
	require.Len(t, resolved.Blocks, 2)
	require.Equal(t, "postgres:16-alpine", resolved.Blocks[0].Components[0].Image.Ref)
	require.Equal(t, "postgres:17-alpine", resolved.Blocks[1].Components[0].Image.Ref)
}

func TestExpandMatrixFindsTheVaryingComponent(t *testing.T) {
	t.Run("no matrix yields one unsuffixed variant", func(t *testing.T) {
		got, err := expandMatrix(&Block{
			Name: "gateway-jwt",
			Components: []Component{
				{Name: "platform-gateway"},
				{Name: "testbench"},
			},
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Empty(t, got[0].name, "a non-expanding block keeps its unsuffixed name")
		require.Empty(t, got[0].component)
	})

	t.Run("one matrix yields one variant per engine, naming the component", func(t *testing.T) {
		got, err := expandMatrix(&Block{
			Name: "gateway-core",
			Components: []Component{
				{Name: "platform-gateway", DB: DBSpec{
					Matrix: []components.DBType{components.SQLite, components.Postgres},
				}},
				{Name: "platform-api", DB: DBSpec{One: components.Postgres}},
			},
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for i, want := range []components.DBType{components.SQLite, components.Postgres} {
			require.Equal(t, string(want), got[i].name)
			require.Equal(t, want, got[i].db)
			require.Equal(t, "platform-gateway", got[i].component,
				"the variant must record which component it varies")
		}
	})

	t.Run("two matrices in one block are refused, naming both", func(t *testing.T) {
		_, err := expandMatrix(&Block{
			Name: "gateway-crossplane",
			Components: []Component{
				{Name: "platform-gateway", DB: DBSpec{Matrix: []components.DBType{components.SQLite}}},
				{Name: "platform-api", DB: DBSpec{Matrix: []components.DBType{components.Postgres}}},
			},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "gateway-crossplane")
		require.ErrorContains(t, err, "platform-gateway")
		require.ErrorContains(t, err, "platform-api")
		require.ErrorContains(t, err, "at most one component")
	})

	t.Run("duplicate engines in a matrix are refused", func(t *testing.T) {
		_, err := expandMatrix(&Block{
			Name: "gateway-duplicate",
			Components: []Component{{Name: "gateway-controller", DB: DBSpec{
				Matrix: []components.DBType{components.SQLite, components.SQLite},
			}}},
		})
		require.ErrorContains(t, err, `duplicate engine "sqlite"`)
	})
}

func TestComponentEngineResolution(t *testing.T) {
	defaults := map[string]ComponentDefaults{
		"platform-gateway": {DB: DBSpec{One: components.SQLite}},
		"platform-api":     {DB: DBSpec{One: components.Postgres}},
	}
	sweep := variant{name: "sqlserver", component: "platform-gateway", db: components.SQLServer}

	t.Run("the matrix component takes this variant's engine", func(t *testing.T) {
		got := componentEngine(&Component{Name: "platform-gateway"}, sweep, defaults)
		require.Equal(t, components.SQLServer, got)
	})

	t.Run("a non-matrix component is untouched by the sweep", func(t *testing.T) {
		got := componentEngine(&Component{Name: "platform-api"}, sweep, defaults)
		require.Equal(t, components.Postgres, got,
			"defaults.components must hold platform-api steady across every gateway variant")
	})

	t.Run("an explicit component db beats defaults", func(t *testing.T) {
		got := componentEngine(
			&Component{Name: "platform-api", DB: DBSpec{One: components.SQLite}}, sweep, defaults)
		require.Equal(t, components.SQLite, got)
	})

	t.Run("nothing anywhere resolves empty, for the caller to reject", func(t *testing.T) {
		got := componentEngine(&Component{Name: "unlisted"}, sweep, defaults)
		require.Empty(t, got)
	})
}

func TestValidateDefaultsRejectsAMatrix(t *testing.T) {
	registry := testRegistry(t)

	t.Run("a matrix in defaults is refused and names the component", func(t *testing.T) {
		err := validateDefaults(map[string]ComponentDefaults{
			"gateway-controller": {DB: DBSpec{Matrix: []components.DBType{components.SQLite, components.Postgres}}},
		}, registry)
		require.Error(t, err)
		require.ErrorContains(t, err, "gateway-controller")
		require.ErrorContains(t, err, "matrix is not allowed")
	})

	t.Run("a scalar is accepted", func(t *testing.T) {
		require.NoError(t, validateDefaults(map[string]ComponentDefaults{
			"gateway-controller": {DB: DBSpec{One: components.SQLite}},
		}, registry))
	})

	t.Run("an unknown component is refused", func(t *testing.T) {
		err := validateDefaults(map[string]ComponentDefaults{
			"gateway-controllr": {DB: DBSpec{One: components.SQLite}},
		}, registry)
		require.Error(t, err)
		require.ErrorContains(t, err, "gateway-controllr")
		require.ErrorContains(t, err, "unknown component")
	})

	t.Run("an engine the component does not support is refused", func(t *testing.T) {
		err := validateDefaults(map[string]ComponentDefaults{
			"api-portal": {DB: DBSpec{One: components.SQLite}},
		}, registry)
		require.Error(t, err)
		require.ErrorContains(t, err, "api-portal")
		require.ErrorContains(t, err, "does not support")
	})
}

func matrixSuite() *Resolved {
	block := func(name, source string, db components.DBType, tags string) ResolvedBlock {
		return ResolvedBlock{
			Name: name, Source: source, DB: db, Parallel: 2,
			Runners: []Runner{
				{Name: "api-deploy", Features: []string{"features/api_deploy.feature"}, Tags: tags},
			},
		}
	}
	return &Resolved{
		Name:     "api-platform-it",
		Parallel: 3,
		Timeouts: Timeouts{Boot: DefaultBootTimeout, Propagation: DefaultPropagationTimeout},
		Blocks: []ResolvedBlock{
			block("gateway-core/sqlite", "gateway-core", components.SQLite, ""),
			block("gateway-core/postgres", "gateway-core", components.Postgres, ""),
			block("gateway-core/sqlserver", "gateway-core", components.SQLServer, ""),
			block("cp-dp-e2e", "cp-dp-e2e", components.Postgres, "@multigateway"),
		},
	}
}

func TestSelectionByBlockName(t *testing.T) {
	t.Run("one variant selects exactly that variant", func(t *testing.T) {
		got, err := Selection{Blocks: []string{"gateway-core/postgres"}}.Apply(matrixSuite())
		require.NoError(t, err)
		require.Equal(t, []string{"gateway-core/postgres"}, got.BlockNames())
	})

	t.Run("a bare source name selects every variant it generated", func(t *testing.T) {
		got, err := Selection{Blocks: []string{"gateway-core"}}.Apply(matrixSuite())
		require.NoError(t, err)
		require.Len(t, got.Blocks, 3)
	})

	t.Run("several names combine", func(t *testing.T) {
		got, err := Selection{Blocks: []string{"gateway-core/sqlite", "cp-dp-e2e"}}.Apply(matrixSuite())
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"gateway-core/sqlite", "cp-dp-e2e"}, got.BlockNames())
	})

	t.Run("no selection runs everything", func(t *testing.T) {
		got, err := Selection{}.Apply(matrixSuite())
		require.NoError(t, err)
		require.Len(t, got.Blocks, 4)
	})
}

func TestUnmatchedSelectionIsAnError(t *testing.T) {
	_, err := Selection{Blocks: []string{"gateway-core/mysql"}}.Apply(matrixSuite())
	require.ErrorContains(t, err, "no block matches gateway-core/mysql")
	require.ErrorContains(t, err, "available:")
	require.ErrorContains(t, err, "gateway-core/postgres")
}

func TestSkipBlocks(t *testing.T) {
	t.Run("excludes a variant", func(t *testing.T) {
		got, err := Selection{SkipBlocks: []string{"gateway-core/sqlserver"}}.Apply(matrixSuite())
		require.NoError(t, err)
		require.NotContains(t, got.BlockNames(), "gateway-core/sqlserver")
		require.Len(t, got.Blocks, 3)
	})

	t.Run("a source name excludes all its variants", func(t *testing.T) {
		got, err := Selection{SkipBlocks: []string{"gateway-core"}}.Apply(matrixSuite())
		require.NoError(t, err)
		require.Equal(t, []string{"cp-dp-e2e"}, got.BlockNames())
	})

	t.Run("an unmatched skip is also an error", func(t *testing.T) {
		_, err := Selection{SkipBlocks: []string{"never-existed"}}.Apply(matrixSuite())
		require.ErrorContains(t, err, "no block matches never-existed")
	})

	t.Run("excluding everything is refused", func(t *testing.T) {
		_, err := Selection{SkipBlocks: []string{"gateway-core", "cp-dp-e2e"}}.Apply(matrixSuite())
		require.ErrorContains(t, err, "matched no blocks")
	})
}

func TestTagsAreCombinedNotReplaced(t *testing.T) {
	got, err := Selection{Tags: "@smoke"}.Apply(matrixSuite())
	require.NoError(t, err)

	byName := map[string]string{}
	for i := range got.Blocks {
		byName[got.Blocks[i].Name] = got.Blocks[i].Runners[0].Tags
	}

	require.Equal(t, "@smoke", byName["gateway-core/sqlite"], "a runner with no tags takes the selection")
	require.Equal(t, "@multigateway && @smoke", byName["cp-dp-e2e"],
		"a runner with tags must satisfy BOTH")
}

func TestParallelOverride(t *testing.T) {
	suite := matrixSuite()
	require.Equal(t, 3, suite.Parallel)

	got, err := Selection{Parallel: 1}.Apply(suite)
	require.NoError(t, err)
	require.Equal(t, 1, got.Parallel)

	unchanged, err := Selection{}.Apply(suite)
	require.NoError(t, err)
	require.Equal(t, 3, unchanged.Parallel, "zero must keep the declared value")
}

func TestSelectionParallelValidation(t *testing.T) {
	suite := matrixSuite()
	_, err := Selection{Parallel: -1}.Apply(suite)
	require.ErrorContains(t, err, "block parallelism cannot be negative")

	_, err = Selection{RunnerParallel: -1}.Apply(suite)
	require.ErrorContains(t, err, "runner parallelism cannot be negative")
}

func TestGatewayVersionSelectionOverride(t *testing.T) {
	var flags Selection
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.Flags(fs)
	require.NoError(t, fs.Parse([]string{"-gateway-version=1.1.0"}))
	require.Equal(t, "1.1.0", flags.GatewayVersion)

	original := &components.Definition{
		Name: "platform-gateway",
		Compose: &components.ComposeSpec{Env: map[string]string{
			"PG_CONTROLLER_IMAGE": "gateway-controller:current",
			"PG_RUNTIME_IMAGE":    "gateway-runtime:current",
		}},
	}
	suite := &Resolved{Blocks: []ResolvedBlock{{
		Name:       "gateway-core",
		Components: []ResolvedComponent{{Def: original, Version: "current"}},
	}}}

	got, err := flags.Apply(suite)
	require.NoError(t, err)
	component := got.Blocks[0].Components[0]
	require.Equal(t, "1.1.0", component.Version)
	require.Equal(t, "gateway-controller:1.1.0", component.Def.Compose.Env["PG_CONTROLLER_IMAGE"])
	require.Equal(t, "gateway-runtime:1.1.0", component.Def.Compose.Env["PG_RUNTIME_IMAGE"])
	require.Equal(t, "gateway-controller:current", original.Compose.Env["PG_CONTROLLER_IMAGE"])
}

func TestGatewayVersionCannotBeCombinedWithCoverage(t *testing.T) {
	_, err := (Selection{GatewayVersion: "1.1.0", Coverage: true}).Apply(matrixSuite())
	require.ErrorContains(t, err, "cannot be combined with -coverage")
}

func TestApplyDoesNotMutateTheInput(t *testing.T) {
	suite := matrixSuite()

	_, err := Selection{Tags: "@smoke", Parallel: 1}.Apply(suite)
	require.NoError(t, err)

	require.Equal(t, 3, suite.Parallel, "the original suite's parallelism must be untouched")
	require.Len(t, suite.Blocks, 4)
	require.Equal(t, "@multigateway", suite.Blocks[3].Runners[0].Tags,
		"the original runner tags must be untouched")
}

func TestApplyCopiesNestedTopologyData(t *testing.T) {
	suite := matrixSuite()
	suite.Blocks[0].Components = []ResolvedComponent{{
		Def:       &components.Definition{Name: "component", DependsOn: []string{"dependency"}},
		DependsOn: []string{"block-dependency"},
	}}

	got, err := (Selection{}).Apply(suite)
	require.NoError(t, err)
	got.Blocks[0].Runners[0].Features[0] = "changed.feature"
	got.Blocks[0].Components[0].DependsOn[0] = "changed-dependency"

	require.Equal(t, "features/api_deploy.feature", suite.Blocks[0].Runners[0].Features[0])
	require.Equal(t, "block-dependency", suite.Blocks[0].Components[0].DependsOn[0])
}

func TestSummarize(t *testing.T) {
	suite := matrixSuite()
	suite.Blocks[0].Components = []ResolvedComponent{
		{Def: &components.Definition{Name: "platform-gateway"}, DB: components.SQLite, Replicas: 1},
		{Def: &components.Definition{Name: "testbench"}, Replicas: 2},
	}

	got := Summarize(suite)
	require.Len(t, got, 4)

	first := got[0]
	require.Equal(t, "gateway-core/sqlite", first.Name)
	require.Equal(t, "sqlite", first.DB)
	require.Contains(t, first.Components, "platform-gateway [sqlite]")
	require.Contains(t, first.Components, "testbench x2",
		"replica count belongs in the summary — it is a real cost")
	require.Len(t, first.Runners, 1)
}

func TestEffectiveParallelGuardsAgainstZero(t *testing.T) {
	b := ResolvedBlock{Parallel: 0}
	require.Equal(t, 1, b.EffectiveParallel())

	b.Parallel = 4
	require.Equal(t, 4, b.EffectiveParallel())
}

func matchesGodogTags(filter string, scenarioTags []string) bool {
	has := func(tag string) bool {
		for _, t := range scenarioTags {
			if strings.TrimPrefix(t, "@") == tag {
				return true
			}
		}
		return false
	}

	ok := true
	for _, group := range strings.Split(filter, "&&") {
		matchedInGroup := false
		for _, tag := range strings.Split(group, ",") {
			tag = strings.ReplaceAll(strings.TrimSpace(tag), "@", "")
			if tag == "" {
				continue
			}
			if tag[0] == '~' {
				matchedInGroup = !has(tag[1:]) || matchedInGroup
				continue
			}
			matchedInGroup = has(tag) || matchedInGroup
		}
		ok = ok && matchedInGroup
	}
	return ok
}

func TestCombinedTagsSelectSomething(t *testing.T) {
	filter := combineTags("~@needs-settle-primitive", "@basic-ratelimit,@ratelimit")

	require.NotContains(t, filter, "(",
		"godog has no grouping operator, so a parenthesis becomes part of the tag literal and "+
			"the expression silently matches nothing")

	require.True(t, matchesGodogTags(filter, []string{"@basic-ratelimit"}),
		"a scenario in a selected feature, not excluded by the runner, must run")
	require.True(t, matchesGodogTags(filter, []string{"@ratelimit"}),
		"the selection is an OR across features")
	require.False(t, matchesGodogTags(filter, []string{"@basic-ratelimit", "@needs-settle-primitive"}),
		"the runner's exclusion must still apply inside a selected feature")
	require.False(t, matchesGodogTags(filter, []string{"@cors"}),
		"a feature outside the selection must not run")
}

func TestUncombinedRunnerTagsStillExclude(t *testing.T) {
	filter := combineTags("~@needs-settle-primitive", "")

	require.True(t, matchesGodogTags(filter, []string{"@basic-ratelimit"}))
	require.False(t, matchesGodogTags(filter, []string{"@needs-settle-primitive"}))
}
