//go:build integration

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

package runtime

import (
	"context"
	"strconv"
	"testing"
	"time"

	"database/sql"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

// These tests exercise network creation, container startup, port resolution, and readiness.
const probeImage = "nginx:alpine"

func newTestNetwork(t *testing.T, ctx context.Context, name string) *Network {
	t.Helper()
	network, err := NewNetwork(ctx, name)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "docker provider") {
		t.Skipf("Docker is unavailable: %v", err)
	}
	require.NoError(t, err)
	return network
}

func probeDef(name string) *components.Definition {
	return &components.Definition{
		Name:  name,
		Image: components.ImageRef{Ref: probeImage},
		Alias: name,
		Endpoints: []components.Endpoint{
			{Name: "http", Port: 80, Scheme: "http", AwaitListening: true},
		},
		Health: &components.HealthCheck{
			Endpoint: "http", Path: "/", ExpectStatus: 200,
			Timeout: 60 * time.Second, Interval: time.Second,
		},
		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 128},
	}
}

func TestLaunchAgainstDocker(t *testing.T) {
	ctx := context.Background()

	nw := newTestNetwork(t, ctx, "launcher-probe")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	def := probeDef("probe")
	c, err := Launch(ctx, def, Options{Network: nw, Replicas: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	t.Run("the mapped port is ephemeral, not the canonical one", func(t *testing.T) {
		port, err := c.Instance.MappedPort("http")
		require.NoError(t, err)
		require.NotEqual(t, 80, port, "host port must not be the canonical container port")
		require.Greater(t, port, 1024)
	})

	t.Run("the health gate passes against a real server", func(t *testing.T) {
		require.NoError(t, AwaitHealthy(ctx, c.Instance, nil))
	})

	t.Run("internal addressing uses alias and canonical port", func(t *testing.T) {
		internal, err := c.Instance.InternalURL("http")
		require.NoError(t, err)
		require.Equal(t, "http://probe:80", internal)
	})

	t.Run("logs are retrievable for diagnostics", func(t *testing.T) {
		logs, err := c.Logs(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, logs)
	})

	t.Run("stop is idempotent", func(t *testing.T) {
		other, err := Launch(ctx, probeDef("probe-stop"), Options{Network: nw})
		require.NoError(t, err)
		require.NoError(t, other.Stop(ctx))
		require.NoError(t, other.Stop(ctx))
	})
}

func TestRunnerCleanupScopeWithDocker(t *testing.T) {
	ctx := context.Background()
	nw := newTestNetwork(t, ctx, "cleanup-scope")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	container, err := Launch(ctx, probeDef("cleanup-probe"), Options{Network: nw})
	require.NoError(t, err)

	registry := cleanup.NewRegistry(nil)
	var deletions atomic.Int32
	require.NoError(t, registry.RegisterDeleter(cleanup.KindAPI, func(ctx context.Context, _ cleanup.Resource) error {
		deletions.Add(1)
		return container.Stop(ctx)
	}))
	require.NoError(t, registry.RegisterRunner(cleanup.Resource{
		Kind: cleanup.KindAPI, ID: "cleanup-probe", Actor: "test", Description: "docker lifecycle probe",
	}))

	require.NoError(t, registry.Sweep(ctx))
	require.EqualValues(t, 0, deletions.Load())
	require.Equal(t, 1, registry.Count())

	require.NoError(t, registry.SweepRunner(ctx))
	require.EqualValues(t, 1, deletions.Load())
	require.Zero(t, registry.Count())
}

func TestPerBlockNetworkIsolation(t *testing.T) {
	ctx := context.Background()

	nwA := newTestNetwork(t, ctx, "block-a")
	t.Cleanup(func() { _ = nwA.Remove(context.Background()) })

	nwB := newTestNetwork(t, ctx, "block-b")
	t.Cleanup(func() { _ = nwB.Remove(context.Background()) })

	require.NotEqual(t, nwA.Name(), nwB.Name())

	def := probeDef("gateway-runtime") // identical alias in both blocks

	a, err := Launch(ctx, def, Options{Network: nwA})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Stop(context.Background()) })

	b, err := Launch(ctx, def, Options{Network: nwB})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Stop(context.Background()) })

	require.Equal(t, "gateway-runtime", a.Instance.Alias())
	require.Equal(t, "gateway-runtime", b.Instance.Alias(), "same alias must be usable in a second block")

	portA, err := a.Instance.MappedPort("http")
	require.NoError(t, err)
	portB, err := b.Instance.MappedPort("http")
	require.NoError(t, err)
	require.NotEqual(t, portA, portB, "concurrent instances must not share a host port")

	require.NoError(t, AwaitHealthy(ctx, a.Instance, nil))
	require.NoError(t, AwaitHealthy(ctx, b.Instance, nil))
}

func TestReplicaAliasesWithinABlock(t *testing.T) {
	ctx := context.Background()

	nw := newTestNetwork(t, ctx, "replicas")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	def := probeDef("gateway-controller")
	set := components.NewSet()

	for i := range 2 {
		c, err := Launch(ctx, def, Options{Network: nw, Ordinal: i, Replicas: 2})
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Stop(context.Background()) })
		require.NoError(t, set.Add(c.Instance))
	}

	first, err := set.At("gateway-controller", 0)
	require.NoError(t, err)
	second, err := set.At("gateway-controller", 1)
	require.NoError(t, err)
	require.Equal(t, "gateway-controller-1", first.Alias())
	require.Equal(t, "gateway-controller-2", second.Alias())

	_, err = set.Get("gateway-controller")
	require.ErrorContains(t, err, "must be addressed by ordinal")
}

func TestLaunchFailureIsReportedWithDiagnostics(t *testing.T) {
	ctx := context.Background()

	nw := newTestNetwork(t, ctx, "bad-image")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	def := probeDef("nonexistent")
	def.Image = components.ImageRef{Ref: "wso2/definitely-not-a-real-image:" + strconv.Itoa(int(time.Now().Unix()))}

	_, err := Launch(ctx, def, Options{Network: nw, StartTimeout: 30 * time.Second})
	require.Error(t, err)
	require.ErrorContains(t, err, "nonexistent")
}

// writeDDL stages a schema file for the integration test.
func writeDDL(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return name // repo-relative, resolved against RepoRoot
}

// psql runs a query inside the server container and returns trimmed stdout.
func psql(t *testing.T, ctx context.Context, c *Container, creds Credentials, db, query string) string {
	t.Helper()
	// Exec does not accept environment overrides, so pass the password through a shell.
	script := fmt.Sprintf("PGPASSWORD=%s psql -U %s -d %s -t -A -c %s",
		shellQuote(creds.Password), shellQuote(creds.User), shellQuote(db), shellQuote(query))

	out, err := c.Exec(ctx, []string{"sh", "-c", script})
	require.NoError(t, err, "psql failed: %s", out)
	return strings.TrimSpace(out)
}

// shellQuote wraps a value in single quotes, escaping any single quote it contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func TestProvisionPostgres(t *testing.T) {
	ctx := context.Background()

	repoRoot := t.TempDir()
	gcDDL := writeDDL(t, repoRoot, "gc.sql",
		"CREATE TABLE artifacts (id TEXT PRIMARY KEY, payload TEXT);")
	paDDL := writeDDL(t, repoRoot, "pa.sql",
		"CREATE TABLE artifacts (id TEXT PRIMARY KEY, note TEXT);\nCREATE TABLE apis (id TEXT PRIMARY KEY);")

	gc := &components.Definition{
		Name: "gateway-controller", Image: components.ImageRef{Ref: "gc:test"}, Alias: "gateway-controller",
		DB: &components.DBContract{
			Supported: []components.DBType{components.Postgres},
			Schema:    map[components.DBType][]string{components.Postgres: {gcDDL}},
			Env: func(d components.DSN) map[string]string {
				return map[string]string{
					"APIP_GW_CONTROLLER_STORAGE_TYPE":              string(d.Type),
					"APIP_GW_CONTROLLER_STORAGE_POSTGRES_HOST":     d.Host,
					"APIP_GW_CONTROLLER_STORAGE_POSTGRES_DATABASE": d.Database,
				}
			},
		},
	}
	pa := &components.Definition{
		Name: "platform-api", Image: components.ImageRef{Ref: "pa:test"}, Alias: "platform-api",
		DB: &components.DBContract{
			Supported: []components.DBType{components.Postgres},
			Schema:    map[components.DBType][]string{components.Postgres: {paDDL}},
			Env: func(d components.DSN) map[string]string {
				return map[string]string{
					"APIP_CP_DATABASE_DRIVER": string(d.Type),
					"APIP_CP_DATABASE_HOST":   d.Host,
					"APIP_CP_DATABASE_NAME":   d.Database,
				}
			},
		},
	}
	mock := &components.Definition{
		Name: "mock-platform-api", Image: components.ImageRef{Ref: "m:test"}, Alias: "mock-platform-api",
		DB: &components.DBContract{
			Supported:       []components.DBType{components.Postgres},
			SharesStoreWith: "gateway-controller",
			Env: func(d components.DSN) map[string]string {
				return map[string]string{"DB_NAME": d.Database, "DB_HOST": d.Host}
			},
		},
	}

	nw := newTestNetwork(t, ctx, "db-provision")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	p, err := Provision(ctx, DatabaseOptions{
		Network:  nw,
		RepoRoot: repoRoot,
		Requests: []Request{
			{Def: gc, Type: components.Postgres, Replicas: 1},
			{Def: pa, Type: components.Postgres, Replicas: 1},
			{Def: mock, Type: components.Postgres, Replicas: 1},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Stop(context.Background()) })

	server := p.Servers[components.Postgres]
	require.NotNil(t, server, "one postgres server should have been started")

	t.Run("one server serves every store", func(t *testing.T) {
		require.Len(t, p.Servers, 1)
	})

	t.Run("each owning component got its own database", func(t *testing.T) {
		list := psql(t, ctx, server, p.Credentials, "bootstrap",
			"SELECT datname FROM pg_database WHERE datname IN ('gateway_controller','platform_api') ORDER BY datname;")
		require.Equal(t, "gateway_controller\nplatform_api", list)
	})

	t.Run("schema was applied to each database independently", func(t *testing.T) {
		gcTables := psql(t, ctx, server, p.Credentials, "gateway_controller",
			"SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;")
		require.Equal(t, "artifacts", gcTables)

		paTables := psql(t, ctx, server, p.Credentials, "platform_api",
			"SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;")
		require.Equal(t, "apis\nartifacts", paTables)
	})

	t.Run("schema is applied before the port accepts connections", func(t *testing.T) {
		count := psql(t, ctx, server, p.Credentials, "platform_api",
			"SELECT count(*) FROM apis;")
		require.Equal(t, "0", count)
	})

	t.Run("the sharer receives the owner's database, not its own", func(t *testing.T) {
		require.Equal(t, "gateway_controller", p.Env[KeyFor("mock-platform-api", 0)]["DB_NAME"])
		require.Equal(t, "gateway_controller", p.Env[KeyFor("gateway-controller", 0)]["APIP_GW_CONTROLLER_STORAGE_POSTGRES_DATABASE"])
	})

	t.Run("each component gets the DSN in its own vocabulary", func(t *testing.T) {
		gcEnv := p.Env[KeyFor("gateway-controller", 0)]
		paEnv := p.Env[KeyFor("platform-api", 0)]
		require.Equal(t, "postgres", gcEnv["APIP_GW_CONTROLLER_STORAGE_TYPE"])
		require.Equal(t, "postgres", paEnv["APIP_CP_DATABASE_DRIVER"])
		require.Equal(t, "platform_api", paEnv["APIP_CP_DATABASE_NAME"])
	})

	t.Run("components address the server by alias, never a mapped port", func(t *testing.T) {
		require.Equal(t, "it-postgres", p.Env[KeyFor("platform-api", 0)]["APIP_CP_DATABASE_HOST"])
	})

	t.Run("credentials are generated, not hardcoded", func(t *testing.T) {
		require.NotEmpty(t, p.Credentials.Password)
		require.NotContains(t, []string{"postgres", "gateway", "apip"}, p.Credentials.Password)
		require.Greater(t, len(p.Credentials.Password), 20)
	})
}

func TestProvisionReplicasGetSeparateDatabases(t *testing.T) {
	ctx := context.Background()

	repoRoot := t.TempDir()
	ddl := writeDDL(t, repoRoot, "gc.sql", "CREATE TABLE artifacts (id TEXT PRIMARY KEY);")

	gc := &components.Definition{
		Name: "gateway-controller", Image: components.ImageRef{Ref: "gc:test"}, Alias: "gateway-controller",
		DB: &components.DBContract{
			Supported: []components.DBType{components.Postgres},
			Schema:    map[components.DBType][]string{components.Postgres: {ddl}},
			Env: func(d components.DSN) map[string]string {
				return map[string]string{"DB": d.Database}
			},
		},
	}

	nw := newTestNetwork(t, ctx, "db-replicas")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	p, err := Provision(ctx, DatabaseOptions{
		Network: nw, RepoRoot: repoRoot,
		Requests: []Request{{Def: gc, Type: components.Postgres, Replicas: 2}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Stop(context.Background()) })

	require.Equal(t, "gateway_controller", p.Env[KeyFor("gateway-controller", 0)]["DB"])
	require.Equal(t, "gateway_controller_2", p.Env[KeyFor("gateway-controller", 1)]["DB"])

	server := p.Servers[components.Postgres]
	both := psql(t, ctx, server, p.Credentials, "bootstrap",
		"SELECT count(*) FROM pg_database WHERE datname LIKE 'gateway_controller%';")
	require.Equal(t, "2", both)

	for _, db := range []string{"gateway_controller", "gateway_controller_2"} {
		tables := psql(t, ctx, server, p.Credentials, db,
			"SELECT table_name FROM information_schema.tables WHERE table_schema='public';")
		require.Equal(t, "artifacts", tables, "database %s should carry the owner's schema", db)
	}
}

// TestSharedHostPortSurvivesRehoming verifies that a shared component keeps its host port.
func TestSharedHostPortSurvivesRehoming(t *testing.T) {
	ctx := context.Background()

	home := newTestNetwork(t, ctx, "sharedport-home")
	t.Cleanup(func() { _ = home.Remove(context.Background()) })

	def := probeDef("sharedport")
	c, err := Launch(ctx, def, Options{Network: home, Replicas: 1, StableHostPorts: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	atLaunch, err := c.Instance.MappedPort("http")
	require.NoError(t, err)

	live := func(t *testing.T) int {
		t.Helper()
		p, perr := c.container.MappedPort(ctx, "80/tcp")
		require.NoError(t, perr)
		return int(p.Num())
	}

	require.Equal(t, atLaunch, live(t), "the launch snapshot must match what docker publishes")

	for range 3 {
		nw := newTestNetwork(t, ctx, "sharedport-block")

		require.NoError(t, c.AttachTo(ctx, nw, "sharedport"))
		require.Equal(t, atLaunch, live(t), "host port changed after attaching to a block network")

		require.NoError(t, c.DetachFrom(ctx, nw))
		require.Equal(t, atLaunch, live(t), "host port changed after detaching from a block network")

		require.NoError(t, nw.Remove(ctx))
	}

	require.Equal(t, atLaunch, live(t), "host port must be stable for the container's whole life")
}

func TestProvisionSQLServer(t *testing.T) {
	if runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64" {
		t.Skipf("no sql server image for %s", runtime.GOARCH)
	}
	ctx := context.Background()

	repoRoot := t.TempDir()
	gcDDL := writeDDL(t, repoRoot, "gc.sqlserver.sql",
		"CREATE TABLE artifacts (id NVARCHAR(64) PRIMARY KEY, payload NVARCHAR(MAX));")
	paDDL := writeDDL(t, repoRoot, "pa.sqlserver.sql",
		"CREATE TABLE artifacts (id NVARCHAR(64) PRIMARY KEY);\nCREATE TABLE apis (id NVARCHAR(64) PRIMARY KEY);")

	gc := &components.Definition{
		Name: "gateway-controller", Image: components.ImageRef{Ref: "gc:test"}, Alias: "gateway-controller",
		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLServer},
			Schema:    map[components.DBType][]string{components.SQLServer: {gcDDL}},
			Env:       func(d components.DSN) map[string]string { return map[string]string{"DB": d.Database} },
		},
	}
	pa := &components.Definition{
		Name: "platform-api", Image: components.ImageRef{Ref: "pa:test"}, Alias: "platform-api",
		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLServer},
			Schema:    map[components.DBType][]string{components.SQLServer: {paDDL}},
			Env:       func(d components.DSN) map[string]string { return map[string]string{"DB": d.Database} },
		},
	}

	nw := newTestNetwork(t, ctx, "db-sqlserver")
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	p, err := Provision(ctx, DatabaseOptions{
		Network: nw, RepoRoot: repoRoot,
		Requests: []Request{
			{Def: gc, Type: components.SQLServer},
			{Def: pa, Type: components.SQLServer},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Stop(context.Background()) })

	server := p.Servers[components.SQLServer]
	require.NotNil(t, server)

	port, err := server.Instance.MappedPort("sql")
	require.NoError(t, err)
	host := server.Instance.Host()

	query := func(t *testing.T, database, q string) string {
		t.Helper()
		conn, err := sql.Open("sqlserver", sqlServerDSN(host, port, p.Credentials, database))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		rows, err := conn.QueryContext(ctx, q)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		var out []string
		for rows.Next() {
			var v string
			require.NoError(t, rows.Scan(&v))
			out = append(out, v)
		}
		require.NoError(t, rows.Err())
		return strings.Join(out, "\n")
	}

	t.Run("both databases were created", func(t *testing.T) {
		got := query(t, "master", "SELECT name FROM sys.databases "+
			"WHERE name IN ('gateway_controller','platform_api') ORDER BY name")
		require.Contains(t, got, "gateway_controller")
		require.Contains(t, got, "platform_api")
	})

	t.Run("schema landed in each database independently", func(t *testing.T) {
		gcTables := query(t, "gateway_controller",
			"SELECT table_name FROM information_schema.tables ORDER BY table_name")
		require.Contains(t, gcTables, "artifacts")
		require.NotContains(t, gcTables, "apis")

		paTables := query(t, "platform_api",
			"SELECT table_name FROM information_schema.tables ORDER BY table_name")
		require.Contains(t, paTables, "apis")
		require.Contains(t, paTables, "artifacts")
	})

	t.Run("Provision returns only after schema is applied", func(t *testing.T) {
		count := query(t, "platform_api", "SELECT COUNT(*) FROM apis")
		require.Equal(t, "0", count)
	})

	t.Run("each component got its own database in the DSN", func(t *testing.T) {
		require.Equal(t, "gateway_controller", p.Env[KeyFor("gateway-controller", 0)]["DB"])
		require.Equal(t, "platform_api", p.Env[KeyFor("platform-api", 0)]["DB"])
	})
}
