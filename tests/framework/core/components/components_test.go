/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com)
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

package components

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func controllerLike() *Definition {
	return &Definition{
		Name:  "gateway-controller",
		Image: ImageRef{Ref: "gateway-controller:test"},
		Alias: "gateway-controller",
		Endpoints: []Endpoint{
			{Name: "rest", Port: 9090, Scheme: "http", AwaitListening: true},
			{Name: "admin", Port: 9092, Scheme: "http", AwaitListening: true},
			{Name: "xds", Port: 18000, Scheme: "grpc"},
		},
		Health: &HealthCheck{
			Endpoint: "admin", Path: "/api/admin/v1/health",
			ExpectStatus: 200, Timeout: 90 * time.Second, Interval: 2 * time.Second,
		},
		DB: &DBContract{
			Supported: []DBType{SQLite, Postgres, SQLServer},
			Schema: map[DBType][]string{
				Postgres:  {"gateway/gateway-controller/pkg/storage/gateway-controller-db.postgres.sql"},
				SQLServer: {"gateway/gateway-controller/pkg/storage/gateway-controller-db.sqlserver.sql"},
			},
			SelfMigrates: []DBType{SQLite},
			Env:          func(d DSN) map[string]string { return map[string]string{"STORAGE_TYPE": string(d.Type)} },
		},
	}
}

func TestDefinitionValidate(t *testing.T) {
	t.Run("a nil definition is rejected", func(t *testing.T) {
		var d *Definition
		require.ErrorContains(t, d.Validate(), "definition is required")
	})

	t.Run("a well-formed definition passes", func(t *testing.T) {
		require.NoError(t, controllerLike().Validate())
	})

	t.Run("name is required", func(t *testing.T) {
		d := controllerLike()
		d.Name = "  "
		require.ErrorContains(t, d.Validate(), "name is required")
	})

	t.Run("alias is required because everything addresses by it", func(t *testing.T) {
		d := controllerLike()
		d.Alias = ""
		require.ErrorContains(t, d.Validate(), "alias is required")
	})

	t.Run("image needs a ref or a build", func(t *testing.T) {
		d := controllerLike()
		d.Image = ImageRef{}
		require.ErrorContains(t, d.Validate(), "either a ref or a build")
	})

	t.Run("duplicate endpoint names and ports are rejected", func(t *testing.T) {
		d := controllerLike()
		d.Endpoints = append(d.Endpoints, Endpoint{Name: "rest", Port: 9090, Scheme: "http"})
		err := d.Validate()
		require.ErrorContains(t, err, `duplicate endpoint name "rest"`)
		require.ErrorContains(t, err, "both use port 9090")
	})

	t.Run("all problems are reported at once", func(t *testing.T) {
		d := controllerLike()
		d.Alias = ""
		d.Image = ImageRef{}
		d.Endpoints = []Endpoint{{Name: "", Port: 0, Scheme: ""}}
		err := d.Validate()
		require.ErrorContains(t, err, "alias is required")
		require.ErrorContains(t, err, "either a ref or a build")
		require.ErrorContains(t, err, "invalid port")
	})
}

func TestDefinitionValidationEdges(t *testing.T) {
	t.Run("endpoint fields are validated", func(t *testing.T) {
		d := controllerLike()
		d.Endpoints = []Endpoint{
			{Name: "", Port: 0, Scheme: "", PathPrefix: "relative"},
			{Name: "api", Port: 65536, Scheme: "http", PathPrefix: "relative"},
		}
		err := d.Validate()
		require.ErrorContains(t, err, "has no name")
		require.ErrorContains(t, err, "invalid port")
		require.ErrorContains(t, err, "has no scheme")
		require.ErrorContains(t, err, "pathPrefix")
	})

	t.Run("health fields are validated", func(t *testing.T) {
		d := controllerLike()
		d.Health = &HealthCheck{Path: "ready", ExpectStatus: 600}
		err := d.Validate()
		require.ErrorContains(t, err, "health check has no endpoint")
		require.ErrorContains(t, err, "health path")
		require.ErrorContains(t, err, "not a valid HTTP status")
		require.ErrorContains(t, err, "timeout must be positive")
		require.ErrorContains(t, err, "interval must be positive")
	})

	t.Run("a build-only image is valid", func(t *testing.T) {
		d := controllerLike()
		d.Image = ImageRef{Build: &ImageBuild{Context: ".", Dockerfile: "Dockerfile"}}
		require.NoError(t, d.Validate())
	})

	t.Run("compose definitions enforce their addressing contract", func(t *testing.T) {
		d := &Definition{
			Name: "stack", Alias: "stack",
			Compose: &ComposeSpec{Services: []string{"worker"}, PrimaryService: "api"},
		}
		err := d.Validate()
		require.ErrorContains(t, err, "no compose file")
		require.ErrorContains(t, err, "primary service")
		require.ErrorContains(t, err, "not in the services list")
		require.ErrorContains(t, err, "still needs endpoints")
	})
}

func TestHealthCheckValidation(t *testing.T) {
	t.Run("health must reference a declared endpoint", func(t *testing.T) {
		d := controllerLike()
		d.Health.Endpoint = "nope"
		require.ErrorContains(t, d.Validate(), `unknown endpoint "nope"`)
	})

	t.Run("a zero expected status is rejected", func(t *testing.T) {
		d := controllerLike()
		d.Health.ExpectStatus = 0
		require.ErrorContains(t, d.Validate(), "not a valid HTTP status")
	})

	t.Run("an interval longer than the timeout would probe once", func(t *testing.T) {
		d := controllerLike()
		d.Health.Interval = 2 * time.Minute
		d.Health.Timeout = 30 * time.Second
		require.ErrorContains(t, d.Validate(), "would probe at most once")
	})
}

func TestDBContractValidation(t *testing.T) {
	t.Run("an engine cannot both self-migrate and declare schema", func(t *testing.T) {
		d := controllerLike()
		d.DB.Schema[SQLite] = []string{"some.sql"}
		require.ErrorContains(t, d.Validate(), "in both schema and selfMigrates")
	})

	t.Run("schema for an unsupported engine is rejected", func(t *testing.T) {
		d := controllerLike()
		d.DB.Supported = []DBType{SQLite, Postgres}
		require.ErrorContains(t, d.Validate(), `schema declared for engine "sqlserver", which is not in supported`)
	})

	t.Run("owning a store without an Env mapping is rejected", func(t *testing.T) {
		d := controllerLike()
		d.DB.Env = nil
		require.ErrorContains(t, d.Validate(), "no Env mapping")
	})

	t.Run("a sharer must not declare its own schema", func(t *testing.T) {
		d := &Definition{
			Name: "mock-platform-api", Image: ImageRef{Ref: "mock:test"}, Alias: "mock-platform-api",
			DB: &DBContract{
				Supported:       []DBType{Postgres},
				SharesStoreWith: "gateway-controller",
				Schema:          map[DBType][]string{Postgres: {"other.sql"}},
			},
		}
		require.ErrorContains(t, d.Validate(), "also declares its own schema")
	})

	t.Run("a contract with no engines is rejected", func(t *testing.T) {
		d := controllerLike()
		d.DB.Supported = nil
		require.ErrorContains(t, d.Validate(), "supports no engine")
	})
}

func TestSchemaFor(t *testing.T) {
	c := controllerLike().DB

	t.Run("framework applies schema for external engines", func(t *testing.T) {
		ddl, must := c.SchemaFor(Postgres)
		require.True(t, must)
		require.Len(t, ddl, 1)
	})

	t.Run("no schema for a self-migrating engine", func(t *testing.T) {
		_, must := c.SchemaFor(SQLite)
		require.False(t, must)
	})

	t.Run("no schema for a sharer", func(t *testing.T) {
		sharer := &DBContract{
			Supported:       []DBType{Postgres},
			SharesStoreWith: "gateway-controller",
		}
		_, must := sharer.SchemaFor(Postgres)
		require.False(t, must)
	})
}

func TestResolveDBType(t *testing.T) {
	d := controllerLike()

	t.Run("explicit beats block default", func(t *testing.T) {
		got, err := d.ResolveDBType(Postgres, SQLite)
		require.NoError(t, err)
		require.Equal(t, Postgres, got)
	})

	t.Run("falls back to the block default", func(t *testing.T) {
		got, err := d.ResolveDBType("", SQLite)
		require.NoError(t, err)
		require.Equal(t, SQLite, got)
	})

	t.Run("an unsupported engine names the component and lists the alternatives", func(t *testing.T) {
		d := controllerLike()
		d.DB.Supported = []DBType{Postgres}
		_, err := d.ResolveDBType(SQLite, "")
		require.ErrorContains(t, err, `component "gateway-controller": does not support db "sqlite"`)
		require.ErrorContains(t, err, "supported: postgres")
	})

	t.Run("no type anywhere is an error, not a silent default", func(t *testing.T) {
		_, err := d.ResolveDBType("", "")
		require.ErrorContains(t, err, "neither the component nor its block specifies one")
	})

	t.Run("setting db on a storageless component is an error", func(t *testing.T) {
		stateless := &Definition{Name: "mock-jwks", Image: ImageRef{Ref: "jwks:test"}, Alias: "mock-jwks"}
		_, err := stateless.ResolveDBType(Postgres, "")
		require.ErrorContains(t, err, "has no storage")
	})

	t.Run("a storageless component resolves to no database when unset", func(t *testing.T) {
		stateless := &Definition{Name: "mock-jwks", Image: ImageRef{Ref: "jwks:test"}, Alias: "mock-jwks"}
		got, err := stateless.ResolveDBType("", Postgres)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("an invalid explicit engine is rejected before support lookup", func(t *testing.T) {
		_, err := d.ResolveDBType(DBType("oracle"), "")
		require.ErrorContains(t, err, `unknown db type "oracle"`)
	})
}

func TestRegistry(t *testing.T) {
	t.Run("register and look up", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register(controllerLike()))
		got, ok := r.Lookup("gateway-controller")
		require.True(t, ok)
		require.Equal(t, "gateway-controller", got.Name)
		require.Equal(t, 1, r.Len())
	})

	t.Run("duplicate registration is an error, not an overwrite", func(t *testing.T) {
		r := NewRegistry()
		require.NoError(t, r.Register(controllerLike()))
		require.ErrorContains(t, r.Register(controllerLike()), "already registered")
	})

	t.Run("an invalid definition is refused at registration", func(t *testing.T) {
		r := NewRegistry()
		require.Error(t, r.Register(&Definition{Name: "broken"}))
	})

	t.Run("names are sorted for stable messages", func(t *testing.T) {
		r := NewRegistry()
		for _, n := range []string{"redis", "gateway-runtime", "mock-jwks"} {
			require.NoError(t, r.Register(&Definition{Name: n, Image: ImageRef{Ref: n}, Alias: n}))
		}
		require.Equal(t, []string{"gateway-runtime", "mock-jwks", "redis"}, r.Names())
	})
}

func TestRegistryCrossValidation(t *testing.T) {
	newWith := func(defs ...*Definition) *Registry {
		r := NewRegistry()
		for _, d := range defs {
			require.NoError(t, r.Register(d))
		}
		return r
	}

	sharer := func(target string, supported ...DBType) *Definition {
		if len(supported) == 0 {
			supported = []DBType{Postgres}
		}
		return &Definition{
			Name: "mock-platform-api", Image: ImageRef{Ref: "mock:test"}, Alias: "mock-platform-api",
			DB: &DBContract{Supported: supported, SharesStoreWith: target},
		}
	}

	t.Run("a valid owner/sharer pair passes", func(t *testing.T) {
		r := newWith(controllerLike(), sharer("gateway-controller"))
		require.NoError(t, r.Validate())
	})

	t.Run("sharing an unknown component is caught", func(t *testing.T) {
		r := newWith(sharer("does-not-exist"))
		require.ErrorContains(t, r.Validate(), `sharesStoreWith unknown component "does-not-exist"`)
	})

	t.Run("sharing a sharer is caught", func(t *testing.T) {
		first := sharer("gateway-controller")
		second := &Definition{
			Name: "second-mock", Image: ImageRef{Ref: "m:test"}, Alias: "second-mock",
			DB: &DBContract{Supported: []DBType{Postgres}, SharesStoreWith: "mock-platform-api"},
		}
		r := newWith(controllerLike(), first, second)
		require.ErrorContains(t, r.Validate(), "which itself shares another store")
	})

	t.Run("sharing a storageless component is caught", func(t *testing.T) {
		stateless := &Definition{Name: "mock-jwks", Image: ImageRef{Ref: "j:test"}, Alias: "mock-jwks"}
		r := newWith(stateless, sharer("mock-jwks"))
		require.ErrorContains(t, r.Validate(), "which has no storage")
	})

	t.Run("a sharer with no engine in common with its owner is caught", func(t *testing.T) {
		owner := controllerLike()
		owner.DB.Supported = []DBType{SQLServer}
		owner.DB.Schema = map[DBType][]string{SQLServer: {"x.sql"}}
		owner.DB.SelfMigrates = nil
		r := newWith(owner, sharer("gateway-controller", Postgres))
		require.ErrorContains(t, r.Validate(), "supports no engine in common")
	})

	t.Run("dependsOn an unknown component is caught", func(t *testing.T) {
		d := controllerLike()
		d.DependsOn = []string{"ghost"}
		r := newWith(d)
		require.ErrorContains(t, r.Validate(), `dependsOn unknown component "ghost"`)
	})

	t.Run("a dependency cycle is caught rather than deadlocking boot", func(t *testing.T) {
		a := &Definition{Name: "a", Image: ImageRef{Ref: "a"}, Alias: "a", DependsOn: []string{"b"}}
		b := &Definition{Name: "b", Image: ImageRef{Ref: "b"}, Alias: "b", DependsOn: []string{"a"}}
		r := newWith(a, b)
		require.ErrorContains(t, r.Validate(), "dependency cycle")
	})
}

func TestImageRefResolve(t *testing.T) {
	img := ImageRef{
		Ref:    "mcr.microsoft.com/mssql/server:2022-latest",
		ByArch: map[string]string{"arm64": "mcr.microsoft.com/azure-sql-edge:latest"},
	}
	require.Equal(t, "mcr.microsoft.com/azure-sql-edge:latest", img.Resolve("arm64"))
	require.Equal(t, "mcr.microsoft.com/mssql/server:2022-latest", img.Resolve("amd64"))
	require.Equal(t, "mcr.microsoft.com/mssql/server:2022-latest", img.Resolve("s390x"))

	t.Run("an empty architecture override falls back to the default", func(t *testing.T) {
		img.ByArch["arm64"] = ""
		require.Equal(t, img.Ref, img.Resolve("arm64"))
	})
}

func TestRegistryZeroValue(t *testing.T) {
	var r Registry
	require.NoError(t, r.Register(&Definition{
		Name: "component", Image: ImageRef{Ref: "component:test"}, Alias: "component",
	}))
	require.Equal(t, 1, r.Len())
}

func TestTypedWiring(t *testing.T) {
	type gatewayWiring struct {
		ControlPlaneURL string `yaml:"controlPlaneURL"`
		SyncEnabled     bool   `yaml:"syncEnabled"`
	}
	spec := TypedWiring[gatewayWiring]()

	decode := func(t *testing.T, src string) (any, error) {
		t.Helper()
		var node yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(src), &node))
		return spec.Decode(node.Content[0])
	}

	t.Run("known keys decode", func(t *testing.T) {
		got, err := decode(t, "controlPlaneURL: http://platform-api:9443\nsyncEnabled: true\n")
		require.NoError(t, err)
		w := got.(*gatewayWiring)
		require.Equal(t, "http://platform-api:9443", w.ControlPlaneURL)
		require.True(t, w.SyncEnabled)
	})

	t.Run("an unknown key is rejected rather than silently ignored", func(t *testing.T) {
		_, err := decode(t, "controlPlainURL: http://platform-api:9443\n")
		require.ErrorContains(t, err, "controlPlainURL")
	})

	t.Run("a wiring type can validate itself at load", func(t *testing.T) {
		spec := TypedWiring[validatingWiring]()
		var node yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("mode: nonsense\n"), &node))
		_, err := spec.Decode(node.Content[0])
		require.ErrorContains(t, err, `mode must be "a" or "b"`)
	})
}

type validatingWiring struct {
	Mode string `yaml:"mode"`
}

func (w *validatingWiring) Validate() error {
	if w.Mode != "a" && w.Mode != "b" {
		return errModeInvalid
	}
	return nil
}

var errModeInvalid = errString(`mode must be "a" or "b"`)

type errString string

func (e errString) Error() string { return string(e) }

func runtimeLike() *Definition {
	return &Definition{
		Name:  "gateway-runtime",
		Image: ImageRef{Ref: "gateway-runtime:test"},
		Alias: "gateway-runtime",
		Endpoints: []Endpoint{
			{Name: "http", Port: 8080, Scheme: "http", AwaitListening: true},
			{Name: "https", Port: 8443, Scheme: "https"},
			{Name: "admin", Port: 9002, Scheme: "http", PathPrefix: "/api/admin"},
		},
	}
}

func mappedRuntime(t *testing.T, ordinal, replicas int) *Instance {
	t.Helper()
	inst, err := NewInstance(runtimeLike(), ordinal, replicas, "127.0.0.1", map[int]int{
		8080: 49153,
		8443: 49154,
		9002: 49155,
	})
	require.NoError(t, err)
	return inst
}

func TestAliasFor(t *testing.T) {
	d := runtimeLike()

	t.Run("a nil definition is rejected", func(t *testing.T) {
		_, err := AliasFor(nil, 0, 1)
		require.ErrorContains(t, err, "definition is required")
	})

	t.Run("a single instance keeps the declared alias verbatim", func(t *testing.T) {
		alias, err := AliasFor(d, 0, 1)
		require.NoError(t, err)
		require.Equal(t, "gateway-runtime", alias)
	})

	t.Run("replicas are suffixed by ordinal", func(t *testing.T) {
		first, err := AliasFor(d, 0, 2)
		require.NoError(t, err)
		second, err := AliasFor(d, 1, 2)
		require.NoError(t, err)
		require.Equal(t, "gateway-runtime-1", first)
		require.Equal(t, "gateway-runtime-2", second)
		require.NotEqual(t, first, second)
	})

	t.Run("a fixed alias cannot be replicated", func(t *testing.T) {
		fixed := runtimeLike()
		fixed.AliasIsFixed = true
		_, err := AliasFor(fixed, 0, 2)
		require.ErrorContains(t, err, "is fixed and cannot be suffixed")
		require.ErrorContains(t, err, "replicas must be 1")
	})

	t.Run("a fixed alias is fine at one replica", func(t *testing.T) {
		fixed := runtimeLike()
		fixed.AliasIsFixed = true
		alias, err := AliasFor(fixed, 0, 1)
		require.NoError(t, err)
		require.Equal(t, "gateway-runtime", alias)
	})

	t.Run("an out-of-range ordinal is rejected", func(t *testing.T) {
		_, err := AliasFor(d, 5, 2)
		require.ErrorContains(t, err, "out of range")
	})

	t.Run("a nonzero ordinal is rejected for one replica", func(t *testing.T) {
		_, err := AliasFor(d, 1, 1)
		require.ErrorContains(t, err, "out of range")
	})
}

func TestInstanceURLs(t *testing.T) {
	inst := mappedRuntime(t, 0, 1)

	t.Run("URL uses the mapped ephemeral host port", func(t *testing.T) {
		got, err := inst.URL("http")
		require.NoError(t, err)
		require.Equal(t, "http://127.0.0.1:49153", got)
	})

	t.Run("scheme comes from the endpoint", func(t *testing.T) {
		got, err := inst.URL("https")
		require.NoError(t, err)
		require.Equal(t, "https://127.0.0.1:49154", got)
	})

	t.Run("a path prefix is preserved", func(t *testing.T) {
		got, err := inst.URL("admin")
		require.NoError(t, err)
		require.Equal(t, "http://127.0.0.1:49155/api/admin", got)
	})

	t.Run("InternalURL uses alias and canonical port", func(t *testing.T) {
		got, err := inst.InternalURL("http")
		require.NoError(t, err)
		require.Equal(t, "http://gateway-runtime:8080", got)
	})

	t.Run("distinct listeners get distinct URLs", func(t *testing.T) {
		dataPlane, err := inst.URL("http")
		require.NoError(t, err)
		admin, err := inst.URL("admin")
		require.NoError(t, err)
		require.NotEqual(t, dataPlane, admin)
	})

	t.Run("an unknown endpoint lists what is available", func(t *testing.T) {
		_, err := inst.URL("nope")
		require.ErrorContains(t, err, `no endpoint "nope"`)
		require.ErrorContains(t, err, "available: admin, http, https")
	})

	t.Run("an unpublished port reports why rather than returning port zero", func(t *testing.T) {
		inst, err := NewInstance(runtimeLike(), 0, 1, "127.0.0.1", map[int]int{8080: 49153})
		require.NoError(t, err)
		_, err = inst.URL("admin")
		require.ErrorContains(t, err, "has no mapped host port")
	})
}

func TestInstanceIdentity(t *testing.T) {
	t.Run("a nil definition is rejected", func(t *testing.T) {
		_, err := NewInstance(nil, 0, 1, "127.0.0.1", nil)
		require.ErrorContains(t, err, "definition is required")
	})
	t.Run("a single instance labels plainly", func(t *testing.T) {
		require.Equal(t, "gateway-runtime", mappedRuntime(t, 0, 1).Label())
	})

	t.Run("replicas label by position", func(t *testing.T) {
		require.Equal(t, "gateway-runtime#2", mappedRuntime(t, 1, 2).Label())
	})

	t.Run("a host is required", func(t *testing.T) {
		_, err := NewInstance(runtimeLike(), 0, 1, "  ", nil)
		require.ErrorContains(t, err, "host is required")
	})

	t.Run("the mapping is copied, so a later caller mutation cannot corrupt it", func(t *testing.T) {
		mapped := map[int]int{8080: 49153}
		inst, err := NewInstance(runtimeLike(), 0, 1, "127.0.0.1", mapped)
		require.NoError(t, err)
		mapped[8080] = 1 // caller mutates its own map afterwards
		port, err := inst.MappedPort("http")
		require.NoError(t, err)
		require.Equal(t, 49153, port)
	})

	t.Run("refreshing ports replaces the mapping without aliasing the input", func(t *testing.T) {
		inst := mappedRuntime(t, 0, 1)
		mapped := map[int]int{8080: 50100}
		inst.RefreshPorts(mapped)
		mapped[8080] = 1
		port, err := inst.MappedPort("http")
		require.NoError(t, err)
		require.Equal(t, 50100, port)
	})
}

func TestSet(t *testing.T) {
	t.Run("the zero value is usable", func(t *testing.T) {
		var s Set
		require.NoError(t, s.Add(mappedRuntime(t, 0, 1)))
		require.Equal(t, 1, s.Len())
	})
	t.Run("a single instance resolves by name", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 0, 1)))
		inst, err := s.Get("gateway-runtime")
		require.NoError(t, err)
		require.Equal(t, "gateway-runtime", inst.Alias())
		require.Equal(t, 1, s.Len())
	})

	t.Run("replicas must be addressed by ordinal", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 0, 2)))
		require.NoError(t, s.Add(mappedRuntime(t, 1, 2)))

		_, err := s.Get("gateway-runtime")
		require.ErrorContains(t, err, "has 2 replicas")
		require.ErrorContains(t, err, "must be addressed by ordinal")

		second, err := s.At("gateway-runtime", 1)
		require.NoError(t, err)
		require.Equal(t, "gateway-runtime-2", second.Alias())
	})

	t.Run("All returns replicas in ordinal order regardless of insertion order", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 1, 2)))
		require.NoError(t, s.Add(mappedRuntime(t, 0, 2)))
		all := s.All("gateway-runtime")
		require.Len(t, all, 2)
		require.Equal(t, 0, all[0].Ordinal())
		require.Equal(t, 1, all[1].Ordinal())
	})

	t.Run("a duplicate ordinal is refused", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 0, 2)))
		require.ErrorContains(t, s.Add(mappedRuntime(t, 0, 2)), "already present")
	})

	t.Run("an unknown component lists what is present", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 0, 1)))
		_, err := s.Get("platform-api")
		require.ErrorContains(t, err, `no component "platform-api"`)
		require.ErrorContains(t, err, "present: gateway-runtime")
	})

	t.Run("an out-of-range ordinal reports the actual count", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 0, 1)))
		_, err := s.At("gateway-runtime", 3)
		require.ErrorContains(t, err, "has 1 instance(s)")
	})

	t.Run("At resolves the requested ordinal rather than the slice position", func(t *testing.T) {
		s := NewSet()
		require.NoError(t, s.Add(mappedRuntime(t, 1, 3)))
		inst, err := s.At("gateway-runtime", 1)
		require.NoError(t, err)
		require.Equal(t, 1, inst.Ordinal())
		_, err = s.At("gateway-runtime", 0)
		require.ErrorContains(t, err, "no instance with ordinal 0")
	})
}

func writeTOML(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func parse(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	tree := map[string]any{}
	require.NoError(t, toml.Unmarshal(raw, &tree), "merged output must be parseable TOML")
	return tree
}

func TestMergeTrees(t *testing.T) {
	t.Run("nested tables merge key by key", func(t *testing.T) {
		base := map[string]any{
			"router": map[string]any{
				"listenPort": int64(9090),
				"tls": map[string]any{
					"enabled":  true,
					"certPath": "/etc/certs/default.pem",
				},
			},
		}
		overlay := map[string]any{
			"router": map[string]any{
				"tls": map[string]any{"certPath": "/etc/certs/test.pem"},
			},
		}

		got, err := MergeTrees(base, overlay)
		require.NoError(t, err)
		require.Equal(t, map[string]any{
			"router": map[string]any{
				"listenPort": int64(9090),
				"tls": map[string]any{
					"enabled":  true,
					"certPath": "/etc/certs/test.pem",
				},
			},
		}, got)
	})

	t.Run("a scalar in an overlay replaces the base scalar", func(t *testing.T) {
		base := map[string]any{"logLevel": "info", "keepAlive": true}
		got, err := MergeTrees(base, map[string]any{"logLevel": "debug"})
		require.NoError(t, err)
		require.Equal(t, "debug", got["logLevel"])
		require.Equal(t, true, got["keepAlive"], "an untouched key must survive")
	})

	t.Run("an overlay may add keys the base never had", func(t *testing.T) {
		base := map[string]any{"router": map[string]any{"listenPort": int64(9090)}}
		got, err := MergeTrees(base, map[string]any{
			"router":  map[string]any{"adminPort": int64(9092)},
			"tracing": map[string]any{"enabled": true},
		})
		require.NoError(t, err)
		require.Equal(t, int64(9092), got["router"].(map[string]any)["adminPort"])
		require.Equal(t, true, got["tracing"].(map[string]any)["enabled"])
	})

	t.Run("arrays replace whole rather than concatenating", func(t *testing.T) {
		base := map[string]any{
			"upstreams": []any{
				map[string]any{"name": "one", "url": "http://one:8080"},
				map[string]any{"name": "two", "url": "http://two:8080"},
			},
			"ciphers": []any{"a", "b", "c"},
		}
		overlay := map[string]any{
			"upstreams": []any{map[string]any{"name": "mock", "url": "http://mock:8080"}},
			"ciphers":   []any{"z"},
		}

		got, err := MergeTrees(base, overlay)
		require.NoError(t, err)
		require.Equal(t, []any{map[string]any{"name": "mock", "url": "http://mock:8080"}}, got["upstreams"])
		require.Equal(t, []any{"z"}, got["ciphers"])
	})

	t.Run("three layers apply in order, the last winning", func(t *testing.T) {
		base := map[string]any{"a": "base", "b": "base", "c": "base"}
		shared := map[string]any{"b": "shared", "c": "shared"}
		block := map[string]any{"c": "block"}

		got, err := MergeTrees(base, shared, block)
		require.NoError(t, err)
		require.Equal(t, map[string]any{"a": "base", "b": "shared", "c": "block"}, got)
	})

	t.Run("a table meeting a scalar is an error naming the key path", func(t *testing.T) {
		base := map[string]any{
			"router": map[string]any{
				"tls": map[string]any{"enabled": true, "certPath": "/etc/certs/default.pem"},
			},
		}
		_, err := MergeTrees(base, map[string]any{
			"router": map[string]any{"tls": "enabled"},
		})
		require.ErrorContains(t, err, `key "router.tls"`)
		require.ErrorContains(t, err, "has type table in the base but string in the overlay")
	})

	t.Run("a scalar meeting a table is an error too", func(t *testing.T) {
		base := map[string]any{"router": map[string]any{"tls": true}}
		_, err := MergeTrees(base, map[string]any{
			"router": map[string]any{"tls": map[string]any{"enabled": true}},
		})
		require.ErrorContains(t, err, `key "router.tls" has type boolean in the base but table in the overlay`)
	})

	t.Run("an array meeting a table is an error, not a whole-value replace", func(t *testing.T) {
		base := map[string]any{"upstreams": []any{map[string]any{"name": "one"}}}
		_, err := MergeTrees(base, map[string]any{
			"upstreams": map[string]any{"name": "one"},
		})
		require.ErrorContains(t, err, `key "upstreams" has type array in the base but table in the overlay`)
	})

	t.Run("every conflict is reported at once", func(t *testing.T) {
		base := map[string]any{
			"router":  map[string]any{"tls": map[string]any{"enabled": true}},
			"tracing": map[string]any{"sampler": map[string]any{"rate": 0.1}},
		}
		_, err := MergeTrees(base, map[string]any{
			"router":  map[string]any{"tls": "on"},
			"tracing": map[string]any{"sampler": int64(1)},
		})
		require.ErrorContains(t, err, `key "router.tls"`)
		require.ErrorContains(t, err, `key "tracing.sampler"`)
	})

	t.Run("no overlays yields a copy of the base", func(t *testing.T) {
		base := map[string]any{"router": map[string]any{"listenPort": int64(9090)}}
		got, err := MergeTrees(base)
		require.NoError(t, err)
		require.Equal(t, base, got)
	})

	t.Run("inputs are never mutated", func(t *testing.T) {
		base := map[string]any{
			"router":  map[string]any{"tls": map[string]any{"enabled": true}},
			"ciphers": []any{"a"},
		}
		overlay := map[string]any{
			"router":  map[string]any{"tls": map[string]any{"enabled": false}},
			"ciphers": []any{"z"},
		}

		got, err := MergeTrees(base, overlay)
		require.NoError(t, err)
		require.Equal(t, true, base["router"].(map[string]any)["tls"].(map[string]any)["enabled"])
		require.Equal(t, []any{"a"}, base["ciphers"])

		got["ciphers"].([]any)[0] = "mutated"
		require.Equal(t, []any{"z"}, overlay["ciphers"])
	})
}

func TestMergeFiles(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "config.toml", `
logLevel = "info"
ciphers = ["a", "b"]

[router]
listenPort = 9090

[router.tls]
enabled = true
certPath = "/etc/certs/default.pem"
`)
	shared := writeTOML(t, dir, "shared.toml", `
logLevel = "debug"

[router.tls]
certPath = "/etc/certs/shared.pem"
`)
	block := writeTOML(t, dir, "block.toml", `
ciphers = ["z"]

[router]
listenPort = 19090
`)

	t.Run("base, shared and block overlay compose in precedence order", func(t *testing.T) {
		out, err := Merge(base, shared, block)
		require.NoError(t, err)

		got := parse(t, out)
		router := got["router"].(map[string]any)
		tls := router["tls"].(map[string]any)
		require.Equal(t, "debug", got["logLevel"], "shared overlay beats the shipped base")
		require.Equal(t, int64(19090), router["listenPort"], "block overlay beats the base")
		require.Equal(t, "/etc/certs/shared.pem", tls["certPath"])
		require.Equal(t, true, tls["enabled"], "a key no overlay mentions keeps the product default")
		require.Equal(t, []any{"z"}, got["ciphers"], "arrays replace whole")
	})

	t.Run("an empty overlay path is skipped", func(t *testing.T) {
		out, err := Merge(base, "", "")
		require.NoError(t, err)

		baseOnly, err := Merge(base)
		require.NoError(t, err)
		require.Equal(t, string(baseOnly), string(out))
		require.Equal(t, "info", parse(t, out)["logLevel"])
	})

	t.Run("a non-existent overlay path is an error naming the path", func(t *testing.T) {
		missing := filepath.Join(dir, "typo.toml")
		_, err := Merge(base, missing)
		require.ErrorContains(t, err, missing)
		require.ErrorContains(t, err, "overlay")
	})

	t.Run("a non-existent base is an error naming the path", func(t *testing.T) {
		missing := filepath.Join(dir, "gone.toml")
		_, err := Merge(missing, shared)
		require.ErrorContains(t, err, missing)
		require.ErrorContains(t, err, "base config")
	})

	t.Run("every unreadable layer is reported at once", func(t *testing.T) {
		_, err := Merge(base, filepath.Join(dir, "one.toml"), filepath.Join(dir, "two.toml"))
		require.ErrorContains(t, err, "one.toml")
		require.ErrorContains(t, err, "two.toml")
	})

	t.Run("an empty base path is rejected", func(t *testing.T) {
		_, err := Merge("", shared)
		require.ErrorContains(t, err, "no base config path")
	})

	t.Run("malformed TOML is reported against the layer that contains it", func(t *testing.T) {
		bad := writeTOML(t, dir, "bad.toml", "this is not = = toml")
		_, err := Merge(base, bad)
		require.ErrorContains(t, err, bad)
		require.ErrorContains(t, err, "parsing overlay")
	})

	t.Run("a type conflict across files names the key path", func(t *testing.T) {
		conflict := writeTOML(t, dir, "conflict.toml", "[router]\ntls = \"on\"\n")
		_, err := Merge(base, conflict)
		require.ErrorContains(t, err, `key "router.tls"`)
	})

	t.Run("output is deterministic", func(t *testing.T) {
		first, err := Merge(base, shared, block)
		require.NoError(t, err)
		second, err := Merge(base, shared, block)
		require.NoError(t, err)
		require.Equal(t, string(first), string(second))
	})
}

func TestAssemble(t *testing.T) {
	repoRoot := t.TempDir()
	writeTOML(t, repoRoot, "gateway/configs/config.toml", `
logLevel = "info"

[router]
listenPort = 9090
`)
	writeTOML(t, repoRoot, "tests/framework/overlays/shared.toml", `logLevel = "debug"`)
	writeTOML(t, repoRoot, "tests/framework/overlays/block.toml", "[router]\nlistenPort = 19090\n")

	injection := func() *ConfigInjection {
		return &ConfigInjection{
			BaseConfigPath:    "gateway/configs/config.toml",
			SharedOverlayPath: "tests/framework/overlays/shared.toml",
			ContainerPath:     "/home/wso2/conf/config.toml",
			Format:            TOML,
		}
	}

	t.Run("repo-relative paths resolve against the repo root", func(t *testing.T) {
		out, err := Assemble(injection(), repoRoot, "tests/framework/overlays/block.toml", nil)
		require.NoError(t, err)

		got := parse(t, out)
		require.Equal(t, "debug", got["logLevel"])
		require.Equal(t, int64(19090), got["router"].(map[string]any)["listenPort"])
	})

	t.Run("a block with no overlay of its own gets base plus shared", func(t *testing.T) {
		out, err := Assemble(injection(), repoRoot, "", nil)
		require.NoError(t, err)

		got := parse(t, out)
		require.Equal(t, "debug", got["logLevel"])
		require.Equal(t, int64(9090), got["router"].(map[string]any)["listenPort"])
	})

	t.Run("an absent shared overlay leaves the shipped base untouched", func(t *testing.T) {
		inj := injection()
		inj.SharedOverlayPath = ""
		out, err := Assemble(inj, repoRoot, "", nil)
		require.NoError(t, err)
		require.Equal(t, "info", parse(t, out)["logLevel"])
	})

	t.Run("an absolute overlay path is taken as given", func(t *testing.T) {
		absolute := writeTOML(t, t.TempDir(), "extra.toml", `logLevel = "trace"`)
		out, err := Assemble(injection(), repoRoot, absolute, nil)
		require.NoError(t, err)
		require.Equal(t, "trace", parse(t, out)["logLevel"])
	})

	t.Run("a nil injection is an error, not an empty config", func(t *testing.T) {
		_, err := Assemble(nil, repoRoot, "", nil)
		require.ErrorContains(t, err, "declares no config injection")
	})

	t.Run("a non-TOML format is rejected rather than guessed at", func(t *testing.T) {
		inj := injection()
		inj.Format = ConfigFormat("yaml")
		_, err := Assemble(inj, repoRoot, "", nil)
		require.ErrorContains(t, err, `unsupported config format "yaml"`)
	})

	t.Run("a missing shipped base names the resolved path", func(t *testing.T) {
		inj := injection()
		inj.BaseConfigPath = "gateway/configs/nope.toml"
		_, err := Assemble(inj, repoRoot, "", nil)
		require.ErrorContains(t, err, filepath.Join(repoRoot, "gateway/configs/nope.toml"))
	})
}

func TestOverlayVariableSubstitution(t *testing.T) {
	dir := t.TempDir()
	base := writeTOML(t, dir, "base.toml", "logLevel = \"info\"\n")

	t.Run("a supplied variable is substituted inside a value", func(t *testing.T) {
		overlay := writeTOML(t, dir, "analytics.toml",
			"[analytics]\nurl = \"http://testbench:3007/${BLOCK}\"\n")
		out, err := MergeWithVars(Vars{VarBlock: "gateway-analytics"}, base, overlay)
		require.NoError(t, err)
		got := parse(t, out)["analytics"].(map[string]any)
		require.Equal(t, "http://testbench:3007/gateway-analytics", got["url"])
	})

	t.Run("an unsupplied variable is an error naming it", func(t *testing.T) {
		overlay := writeTOML(t, dir, "unknown.toml", "url = \"http://x/${NOT_A_THING}\"\n")
		_, err := MergeWithVars(Vars{VarBlock: "b"}, base, overlay)
		require.ErrorContains(t, err, "${NOT_A_THING}")
		require.ErrorContains(t, err, "BLOCK") // says what WAS available
	})

	t.Run("an empty value is an error, not an empty path segment", func(t *testing.T) {
		overlay := writeTOML(t, dir, "empty.toml", "url = \"http://x/${BLOCK}\"\n")
		_, err := MergeWithVars(Vars{VarBlock: ""}, base, overlay)
		require.ErrorContains(t, err, "empty value")
	})

	t.Run("the shipped base is never substituted", func(t *testing.T) {
		productBase := writeTOML(t, dir, "product.toml", "secret = \"${BLOCK}\"\n")
		out, err := MergeWithVars(Vars{VarBlock: "gateway-analytics"}, productBase)
		require.NoError(t, err)
		require.Equal(t, "${BLOCK}", parse(t, out)["secret"])
	})

	t.Run("a non-framework placeholder is left alone", func(t *testing.T) {
		overlay := writeTOML(t, dir, "product-template.toml", "path = \"${env:HOME}/x\"\n")
		out, err := MergeWithVars(Vars{VarBlock: "b"}, base, overlay)
		require.NoError(t, err)
		require.Equal(t, "${env:HOME}/x", parse(t, out)["path"])
	})
}

func TestDefinitionAndDatabaseAccessors(t *testing.T) {
	t.Run("endpoint lookup", func(t *testing.T) {
		definition := Definition{Endpoints: []Endpoint{{Name: "api", Port: 8080}}}
		endpoint, ok := definition.Endpoint("api")
		require.True(t, ok)
		require.Equal(t, 8080, endpoint.Port)

		_, ok = definition.Endpoint("missing")
		require.False(t, ok)
	})

	t.Run("database type metadata", func(t *testing.T) {
		require.True(t, Postgres.Valid())
		require.False(t, Postgres.Embedded())
		require.True(t, SQLite.Embedded())
		require.False(t, DBType("unknown").Valid())
		require.False(t, DBType("unknown").Embedded())
	})

	t.Run("database contract support", func(t *testing.T) {
		contract := DBContract{Supported: []DBType{SQLite, Postgres}}
		require.True(t, contract.Supports(SQLite))
		require.False(t, contract.Supports(SQLServer))
		empty := DBContract{}
		require.False(t, empty.Supports(DBType("")))
	})

	t.Run("nil database contracts are safe", func(t *testing.T) {
		var contract *DBContract
		require.False(t, contract.Owns())
		require.False(t, contract.Supports(Postgres))
		ddl, apply := contract.SchemaFor(Postgres)
		require.Nil(t, ddl)
		require.False(t, apply)
	})
}

func TestComposeHelpers(t *testing.T) {
	t.Run("compose detection and staging name", func(t *testing.T) {
		empty := Definition{}
		require.False(t, empty.IsCompose())

		definition := Definition{Compose: &ComposeSpec{ComposeFile: "compose/gateway.yaml"}}
		require.True(t, definition.IsCompose())
		require.Equal(t, "gateway.yaml", definition.Compose.StagingName())
	})

	t.Run("generated files preserve the compose definition", func(t *testing.T) {
		definition := Definition{
			Name: "gateway",
			Compose: &ComposeSpec{
				StagedFiles:       map[string]string{"a": "b"},
				GeneratedFiles:    map[string][]byte{"existing": []byte("value")},
				Env:               map[string]string{"A": "B"},
				CoverageServices:  []string{"gateway"},
			},
		}
		updated := definition.Compose.WithGenerated(map[string][]byte{"generated": []byte("content")})
		require.Equal(t, map[string]string{"a": "b"}, updated.StagedFiles)
		require.Equal(t, map[string][]byte{"existing": []byte("value"), "generated": []byte("content")}, updated.GeneratedFiles)
		require.Equal(t, map[string]string{"A": "B"}, updated.Env)
		require.Equal(t, []string{"gateway"}, updated.CoverageServices)
		require.NotSame(t, definition.Compose, updated)
		updated.GeneratedFiles["existing"][0] = 'X'
		require.Equal(t, []byte("value"), definition.Compose.GeneratedFiles["existing"])
	})

	t.Run("environment content is deterministic and escapes dollar signs", func(t *testing.T) {
		content := string(EnvFileContent(map[string]string{"B": "two", "A": "one$1"}))
		require.Equal(t, "A=one$$1\nB=two\n", content)
	})
}

func TestDefinitionWithImageVersion(t *testing.T) {
	t.Run("replaces single-container image tags without mutating the definition", func(t *testing.T) {
		definition := Definition{Image: ImageRef{
			Ref:    "registry.example/app:current",
			ByArch: map[string]string{"arm64": "registry.example/app-arm64:current"},
		}}

		updated := definition.WithImageVersion("legacy")
		require.Equal(t, "registry.example/app:legacy", updated.Image.Ref)
		require.Equal(t, "registry.example/app-arm64:legacy", updated.Image.ByArch["arm64"])
		require.Equal(t, "registry.example/app:current", definition.Image.Ref)
	})

	t.Run("replaces every compose service image", func(t *testing.T) {
		definition := Definition{Compose: &ComposeSpec{Env: map[string]string{
			"CONTROLLER_IMAGE": "registry.example/controller:current",
			"RUNTIME_IMAGE":    "registry.example/runtime:current",
			"OTHER_VALUE":      "current",
		}}}

		updated := definition.WithImageVersion("legacy")
		require.Equal(t, "registry.example/controller:legacy", updated.Compose.Env["CONTROLLER_IMAGE"])
		require.Equal(t, "registry.example/runtime:legacy", updated.Compose.Env["RUNTIME_IMAGE"])
		require.Equal(t, "current", updated.Compose.Env["OTHER_VALUE"])
		require.Equal(t, "registry.example/controller:current", definition.Compose.Env["CONTROLLER_IMAGE"])
	})

	t.Run("does not alter references for an empty version", func(t *testing.T) {
		definition := &Definition{Image: ImageRef{Ref: "app:current"}}
		require.Same(t, definition, definition.WithImageVersion(" "))
	})
}

func TestConfigAndFileValidation(t *testing.T) {
	t.Run("valid configuration passes", func(t *testing.T) {
		d := controllerLike()
		d.Config = &ConfigInjection{
			BaseConfigPath: "config/base.toml",
			ContainerPath:  "/opt/app/config.toml",
			Format:         TOML,
		}
		d.Files = []FileMount{{HostPath: "certs/ca.pem", ContainerPath: "/etc/app/ca.pem"}}
		require.NoError(t, d.Validate())
	})

	t.Run("invalid configuration and mounts are reported", func(t *testing.T) {
		d := controllerLike()
		d.Config = &ConfigInjection{ContainerPath: "relative", Format: ConfigFormat("yaml")}
		d.Files = []FileMount{
			{ContainerPath: "relative"},
			{HostPath: "other.pem", ContainerPath: "relative"},
		}
		err := d.Validate()
		require.ErrorContains(t, err, "no baseConfigPath")
		require.ErrorContains(t, err, "unsupported config format")
		require.ErrorContains(t, err, "must be absolute")
		require.ErrorContains(t, err, "no hostPath")
		require.ErrorContains(t, err, "two file mounts target")
	})
}

func TestRegistryMustRegister(t *testing.T) {
	t.Run("registers all definitions", func(t *testing.T) {
		r := NewRegistry().MustRegister(controllerLike())
		require.Equal(t, 1, r.Len())
	})

	t.Run("panics when a definition is invalid", func(t *testing.T) {
		require.Panics(t, func() {
			NewRegistry().MustRegister(&Definition{Name: "broken"})
		})
	})
}
