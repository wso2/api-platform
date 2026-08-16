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
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type slowWriter struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func newTestNetwork(t *testing.T, ctx context.Context, name string) *Network {
	t.Helper()
	network, err := NewNetwork(ctx, name)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "docker provider") {
		t.Skipf("Docker is unavailable: %v", err)
	}
	require.NoError(t, err)
	return network
}

func newTestHTTPServer(t *testing.T, handler http.Handler, tls bool) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
		t.Skipf("loopback listeners are unavailable: %v", err)
	}
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	if tls {
		server.StartTLS()
	} else {
		server.Start()
	}
	t.Cleanup(server.Close)
	return server
}

func (w *slowWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.b.Write(p)
	w.mu.Unlock()
	runtime.Gosched()
	return n, err
}

func writeReportLikeGodog(dst io.Writer, runner string, scenarios int) {
	fmt.Fprintf(dst, "%d scenarios", scenarios)
	runtime.Gosched()
	fmt.Fprintf(dst, " (%d passed)", scenarios)
	runtime.Gosched()
	fmt.Fprintf(dst, " [%s]\n", runner)
}

func TestRunnerOutputIsNotInterleaved(t *testing.T) {
	const runners, scenariosEach = 12, 7

	shared := &slowWriter{b: &bytes.Buffer{}}

	var wg sync.WaitGroup
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var own bytes.Buffer
			writeReportLikeGodog(&own, fmt.Sprintf("runner-%02d", i), scenariosEach)
			flushRunnerOutput(shared, fmt.Sprintf("block/runner-%02d", i), &own)
		}(i)
	}
	wg.Wait()

	got := shared.b.String()

	for i := 0; i < runners; i++ {
		want := fmt.Sprintf("%d scenarios (%d passed) [runner-%02d]", scenariosEach, scenariosEach, i)
		require.Contains(t, got, want,
			"runner-%02d's summary was not emitted intact — output was interleaved", i)
	}

	for i := 0; i < runners; i++ {
		header := fmt.Sprintf("=== block/runner-%02d ===\n", i)
		idx := strings.Index(got, header)
		require.GreaterOrEqual(t, idx, 0, "missing header for runner-%02d", i)
		rest := got[idx+len(header):]
		require.True(t, strings.HasPrefix(rest, fmt.Sprintf("%d scenarios", scenariosEach)),
			"runner-%02d's header is not followed by its own report; another runner's output "+
				"was flushed in between", i)
	}
}

func TestSharedWriterCorrupts(t *testing.T) {
	const runners, scenariosEach = 12, 7

	shared := &slowWriter{b: &bytes.Buffer{}}

	var wg sync.WaitGroup
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			writeReportLikeGodog(shared, fmt.Sprintf("runner-%02d", i), scenariosEach)
		}(i)
	}
	wg.Wait()

	got := shared.b.String()
	intact := 0
	for i := 0; i < runners; i++ {
		if strings.Contains(got, fmt.Sprintf("%d scenarios (%d passed) [runner-%02d]",
			scenariosEach, scenariosEach, i)) {
			intact++
		}
	}
	if intact == runners {
		t.Skip("no interleaving occurred this run; the guarded failure mode could not be " +
			"demonstrated, which does not mean it cannot happen")
	}
	t.Logf("shared writer corrupted %d of %d summaries — this is what flushRunnerOutput prevents",
		runners-intact, runners)
}

func owner(name string, types ...components.DBType) *components.Definition {
	if len(types) == 0 {
		types = []components.DBType{components.SQLite, components.Postgres, components.SQLServer}
	}
	return &components.Definition{
		Name: name, Image: components.ImageRef{Ref: name + ":test"}, Alias: name,
		DB: &components.DBContract{
			Supported: types,
			Schema: map[components.DBType][]string{
				components.Postgres:  {name + ".postgres.sql"},
				components.SQLServer: {name + ".sqlserver.sql"},
			},
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          func(components.DSN) map[string]string { return nil },
		},
	}
}

func sharer(name, target string, types ...components.DBType) *components.Definition {
	if len(types) == 0 {
		types = []components.DBType{components.Postgres}
	}
	return &components.Definition{
		Name: name, Image: components.ImageRef{Ref: name + ":test"}, Alias: name,
		DB: &components.DBContract{Supported: types, SharesStoreWith: target},
	}
}

func TestBuildPlanBasics(t *testing.T) {
	t.Run("each owning component gets its own store", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres, Replicas: 1},
			{Def: owner("platform-api"), Type: components.Postgres, Replicas: 1},
		})
		require.NoError(t, err)
		require.Len(t, plan.Stores, 2)

		gc, ok := plan.StoreFor("gateway-controller", 0)
		require.True(t, ok)
		pa, ok := plan.StoreFor("platform-api", 0)
		require.True(t, ok)
		require.NotEqual(t, gc.Database, pa.Database, "different components must not share a database")
	})

	t.Run("one server per engine, not per component", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres},
			{Def: owner("platform-api"), Type: components.Postgres},
			{Def: owner("api-portal"), Type: components.Postgres},
		})
		require.NoError(t, err)
		require.Len(t, plan.Servers, 1)
		require.Equal(t, components.Postgres, plan.Servers[0].Type)
		require.Len(t, plan.Servers[0].Stores, 3)
	})

	t.Run("mixed engines produce one server each", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("platform-api"), Type: components.Postgres},
			{Def: owner("gateway-controller"), Type: components.SQLite},
		})
		require.NoError(t, err)
		require.Len(t, plan.Servers, 1, "sqlite is embedded and needs no server")
		require.True(t, plan.NeedsServer(components.Postgres))
		require.False(t, plan.NeedsServer(components.SQLite))
	})

	t.Run("an embedded engine still gets a store, just no server", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.SQLite},
		})
		require.NoError(t, err)
		require.Empty(t, plan.Servers)
		s, ok := plan.StoreFor("gateway-controller", 0)
		require.True(t, ok)
		require.True(t, s.Embedded)
	})

	t.Run("a component without storage is skipped entirely", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: &components.Definition{Name: "mock-jwks", Image: components.ImageRef{Ref: "j"}, Alias: "j"}},
		})
		require.NoError(t, err)
		require.Empty(t, plan.Stores)
		require.Empty(t, plan.Servers)
	})
}

func TestBuildPlanCarriesDatabaseImage(t *testing.T) {
	image := components.ImageRef{Ref: "postgres:17-alpine"}
	plan, err := BuildPlan([]Request{{
		Def: owner("gateway-controller"), Type: components.Postgres, Image: image,
	}})
	require.NoError(t, err)
	require.Len(t, plan.Servers, 1)
	require.Equal(t, image, plan.Servers[0].Image)
}

func TestServerDefinitionsUseConfiguredImages(t *testing.T) {
	postgresDefault, err := (postgresEngine{}).serverDefinition(Credentials{}, nil, "")
	require.NoError(t, err)
	require.Equal(t, "postgres:16-alpine", postgresDefault.Image.Ref)

	postgres, err := (postgresEngine{}).serverDefinition(Credentials{}, nil, "", components.ImageRef{
		Ref: "postgres:17-alpine",
	})
	require.NoError(t, err)
	require.Equal(t, "postgres:17-alpine", postgres.Image.Ref)

	sqlserver, err := (sqlServerEngine{}).serverDefinition(Credentials{}, nil, "", components.ImageRef{
		Ref: "mssql/server:custom",
	})
	require.NoError(t, err)
	require.Equal(t, "mssql/server:custom", sqlserver.Image.Ref)
	require.Empty(t, sqlserver.Image.ByArch)

	sqlserverDefault, err := (sqlServerEngine{}).serverDefinition(Credentials{}, nil, "")
	require.NoError(t, err)
	require.Equal(t, "mcr.microsoft.com/mssql/server:2022-latest", sqlserverDefault.Image.Ref)
	require.Equal(t, "mcr.microsoft.com/azure-sql-edge:latest", sqlserverDefault.Image.ByArch["arm64"])
}

func TestBuildPlanSchema(t *testing.T) {
	t.Run("the framework applies schema for an external engine", func(t *testing.T) {
		plan, err := BuildPlan([]Request{{Def: owner("gateway-controller"), Type: components.Postgres}})
		require.NoError(t, err)
		s, _ := plan.StoreFor("gateway-controller", 0)
		require.Equal(t, []string{"gateway-controller.postgres.sql"}, s.Schema)
	})

	t.Run("no schema is planned for a self-migrating engine", func(t *testing.T) {
		plan, err := BuildPlan([]Request{{Def: owner("gateway-controller"), Type: components.SQLite}})
		require.NoError(t, err)
		s, _ := plan.StoreFor("gateway-controller", 0)
		require.Empty(t, s.Schema)
	})

	t.Run("a sharer's store carries only the owner's schema", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres},
			{Def: sharer("mock-platform-api", "gateway-controller"), Type: components.Postgres},
		})
		require.NoError(t, err)
		require.Len(t, plan.Stores, 1, "a sharer must not create a second store")

		gc, _ := plan.StoreFor("gateway-controller", 0)
		mock, _ := plan.StoreFor("mock-platform-api", 0)
		require.Equal(t, gc.ID, mock.ID)
		require.Equal(t, "gateway-controller", gc.Owner)
		require.Equal(t, []string{"gateway-controller.postgres.sql"}, gc.Schema)
	})
}

func TestBuildPlanReplicas(t *testing.T) {
	t.Run("replicas get independent stores by default", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres, Replicas: 2},
		})
		require.NoError(t, err)
		require.Len(t, plan.Stores, 2)

		first, ok := plan.StoreFor("gateway-controller", 0)
		require.True(t, ok)
		second, ok := plan.StoreFor("gateway-controller", 1)
		require.True(t, ok)
		require.NotEqual(t, first.Database, second.Database)
		require.Equal(t, "gateway_controller", first.Database)
		require.Equal(t, "gateway_controller_2", second.Database)
	})

	t.Run("a single-instance owner is shared by all sharer instances", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres, Replicas: 1},
			{Def: sharer("mock-platform-api", "gateway-controller"), Type: components.Postgres, Replicas: 3},
		})
		require.NoError(t, err)
		require.Len(t, plan.Stores, 1)
		for i := range 3 {
			s, ok := plan.StoreFor("mock-platform-api", i)
			require.True(t, ok)
			require.Equal(t, StoreID("gateway-controller"), s.ID)
		}
	})

	t.Run("matching replica counts pair by position", func(t *testing.T) {
		plan, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres, Replicas: 2},
			{Def: sharer("mock-platform-api", "gateway-controller"), Type: components.Postgres, Replicas: 2},
		})
		require.NoError(t, err)
		second, _ := plan.StoreFor("mock-platform-api", 1)
		ownerSecond, _ := plan.StoreFor("gateway-controller", 1)
		require.Equal(t, ownerSecond.ID, second.ID, "sharer #2 must read owner #2's store")
	})

	t.Run("an ambiguous replica pairing is refused rather than guessed", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres, Replicas: 3},
			{Def: sharer("mock-platform-api", "gateway-controller"), Type: components.Postgres, Replicas: 2},
		})
		require.ErrorContains(t, err, "replica counts must match")
	})
}

func TestBuildPlanErrors(t *testing.T) {
	t.Run("a sharer whose owner is absent from the block is caught", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: sharer("mock-platform-api", "gateway-controller"), Type: components.Postgres},
		})
		require.ErrorContains(t, err, "is not in this block")
	})

	t.Run("an engine mismatch between sharer and owner is caught", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.SQLServer},
			{Def: sharer("mock-platform-api", "gateway-controller",
				components.Postgres, components.SQLServer), Type: components.Postgres},
		})
		require.ErrorContains(t, err, "shares")
		require.ErrorContains(t, err, "which is")
	})

	t.Run("an unresolved engine is caught", func(t *testing.T) {
		_, err := BuildPlan([]Request{{Def: owner("gateway-controller"), Type: ""}})
		require.ErrorContains(t, err, "no engine was resolved")
	})

	t.Run("an unsupported engine is caught", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: owner("gateway-controller", components.Postgres), Type: components.SQLite},
		})
		require.ErrorContains(t, err, `does not support engine "sqlite"`)
	})

	t.Run("a duplicate component in one block is caught", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: owner("gateway-controller"), Type: components.Postgres},
			{Def: owner("gateway-controller"), Type: components.Postgres},
		})
		require.ErrorContains(t, err, "appears twice")
	})

	t.Run("all problems are reported at once", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: owner("a", components.Postgres), Type: components.SQLite},
			{Def: sharer("b", "missing"), Type: components.Postgres},
		})
		require.ErrorContains(t, err, "does not support engine")
		require.ErrorContains(t, err, "is not in this block")
	})

	t.Run("database name collisions are rejected", func(t *testing.T) {
		_, err := BuildPlan([]Request{
			{Def: owner("a-b"), Type: components.Postgres},
			{Def: owner("a_b"), Type: components.Postgres},
		})
		require.ErrorContains(t, err, "same database name")
	})
}

func TestRuntimeInputValidation(t *testing.T) {
	t.Run("compose replicas are rejected before Docker access", func(t *testing.T) {
		def := &components.Definition{Name: "stack", Compose: &components.ComposeSpec{}}
		_, err := LaunchCompose(context.Background(), def, &components.ComposeSpec{}, Options{
			Network: &Network{name: "test", block: "block"}, Replicas: 2,
		})
		require.ErrorContains(t, err, "does not support replicas")
	})

	t.Run("staged paths cannot escape their directory", func(t *testing.T) {
		for _, path := range []string{"../secret", "../../secret", "/absolute/path"} {
			require.Error(t, validateStagePath(path), path)
		}
		require.NoError(t, validateStagePath("nested/config.toml"))
	})
}

func TestSQLServerDefinition(t *testing.T) {
	def, err := sqlServerEngine{}.serverDefinition(Credentials{User: "sa", Password: "Aa1!x"}, nil, "")
	require.NoError(t, err)
	require.Equal(t, "mcr.microsoft.com/azure-sql-edge:latest", def.Image.Resolve("arm64"))
	require.Equal(t, "mcr.microsoft.com/mssql/server:2022-latest", def.Image.Resolve("amd64"))
	require.Equal(t, "Aa1!x", def.Env["MSSQL_SA_PASSWORD"])
	require.Equal(t, "Aa1!x", def.Env["SA_PASSWORD"])
}

func TestDatabaseEngineCapabilities(t *testing.T) {
	require.False(t, sqlServerEngine{}.canBootstrapAtStart())
	require.True(t, postgresEngine{}.canBootstrapAtStart())
	require.Equal(t, sqlServerBootAttempts, (sqlServerEngine{}).bootAttempts())
	require.Equal(t, 1, (postgresEngine{}).bootAttempts())
}

func TestGeneratedPasswordSatisfiesEngineRequirements(t *testing.T) {
	for range 20 {
		creds, err := NewCredentials()
		require.NoError(t, err)
		require.Greater(t, len(creds.Password), 8)
		require.True(t, strings.ContainsAny(creds.Password, "abcdefghijklmnopqrstuvwxyz"))
		require.True(t, strings.ContainsAny(creds.Password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		require.True(t, strings.ContainsAny(creds.Password, "0123456789"))
		require.True(t, strings.ContainsAny(creds.Password, "!"))
		require.NotContains(t, creds.Password, "-")
		require.NotContains(t, creds.Password, "=")
	}
}

func TestProvisionValidation(t *testing.T) {
	t.Run("missing schema files fail before Docker access", func(t *testing.T) {
		gc := owner("gateway-controller", components.Postgres)
		gc.DB.Schema[components.Postgres] = []string{"does/not/exist.sql"}
		_, err := Provision(context.Background(), DatabaseOptions{
			Network:  &Network{name: "test", block: "test"},
			RepoRoot: t.TempDir(), Requests: []Request{{Def: gc, Type: components.Postgres}},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "does/not/exist.sql")
	})

	t.Run("embedded engines need no server", func(t *testing.T) {
		gc := &components.Definition{
			Name: "gateway-controller", Image: components.ImageRef{Ref: "gc:test"}, Alias: "gateway-controller",
			DB: &components.DBContract{
				Supported:    []components.DBType{components.SQLite},
				SelfMigrates: []components.DBType{components.SQLite},
				Env: func(d components.DSN) map[string]string {
					return map[string]string{"GATEWAY_DB_PATH": d.FilePath}
				},
			},
		}
		p, err := Provision(context.Background(), DatabaseOptions{Requests: []Request{{Def: gc, Type: components.SQLite}}})
		require.NoError(t, err)
		require.Empty(t, p.Servers)
		require.Equal(t, "/app/data/gateway_controller.db",
			p.Env[KeyFor("gateway-controller", 0)]["GATEWAY_DB_PATH"])
	})
}

func TestPlanDeterminism(t *testing.T) {
	requests := []Request{
		{Def: owner("platform-api"), Type: components.Postgres},
		{Def: owner("gateway-controller"), Type: components.SQLServer},
		{Def: owner("api-portal"), Type: components.Postgres},
	}
	first, err := BuildPlan(requests)
	require.NoError(t, err)

	for range 20 {
		again, err := BuildPlan(requests)
		require.NoError(t, err)
		require.Equal(t, first.StoreIDs(), again.StoreIDs())
		require.Equal(t, len(first.Servers), len(again.Servers))
		for i := range first.Servers {
			require.Equal(t, first.Servers[i].Type, again.Servers[i].Type)
			require.Equal(t, first.Servers[i].Stores, again.Servers[i].Stores)
		}
	}
}

func TestDatabaseName(t *testing.T) {
	t.Run("hyphens become underscores", func(t *testing.T) {
		require.Equal(t, "gateway_controller", databaseName("gateway-controller", 0))
	})

	t.Run("replicas are suffixed by position", func(t *testing.T) {
		require.Equal(t, "gateway_controller_2", databaseName("gateway-controller", 1))
	})

	t.Run("a name with no usable characters still yields something legal", func(t *testing.T) {
		require.Equal(t, "store", databaseName("---", 0))
	})

	t.Run("names are deterministic", func(t *testing.T) {
		require.Equal(t, databaseName("platform-api", 0), databaseName("platform-api", 0))
	})
}

func TestLaunchWithRetrySucceedsAfterFailures(t *testing.T) {
	calls := 0
	c, err := launchWithRetry(context.Background(), "sqlserver", 3,
		func(context.Context) (*Container, error) {
			calls++
			if calls < 3 {
				return nil, errors.New("container exited with code 1")
			}
			return &Container{}, nil
		})
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 3, calls)
}

func TestLaunchWithRetryReturnsLastErrorWhenBudgetSpent(t *testing.T) {
	calls := 0
	boom := errors.New("container exited with code 1")
	_, err := launchWithRetry(context.Background(), "sqlserver", 3,
		func(context.Context) (*Container, error) {
			calls++
			return nil, boom
		})
	require.ErrorIs(t, err, boom)
	require.Equal(t, 3, calls)
}

func TestLaunchWithRetrySingleAttemptFailsFast(t *testing.T) {
	calls := 0
	_, err := launchWithRetry(context.Background(), "postgres", 1,
		func(context.Context) (*Container, error) {
			calls++
			return nil, errors.New("boom")
		})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestLaunchWithRetryStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := launchWithRetry(ctx, "sqlserver", 3,
		func(context.Context) (*Container, error) {
			calls++
			cancel()
			return nil, errors.New("container exited with code 1")
		})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls, "a dead run must not keep booting containers")
}

func TestEngineBootAttempts(t *testing.T) {
	require.Equal(t, 3, sqlServerEngine{}.bootAttempts(),
		"sqlserver retries: the arm64 SQL Edge translator crash is probabilistic")
	require.Equal(t, 1, postgresEngine{}.bootAttempts(),
		"postgres fails fast: it has no known probabilistic boot crash")
}

type probeFunc func(ctx context.Context, url string) (int, error)

func (f probeFunc) Probe(ctx context.Context, url string) (int, error) { return f(ctx, url) }

func healthyDef(timeout, interval time.Duration) *components.Definition {
	return &components.Definition{
		Name:  "gateway-controller",
		Image: components.ImageRef{Ref: "gc:test"},
		Alias: "gateway-controller",
		Endpoints: []components.Endpoint{
			{Name: "admin", Port: 9092, Scheme: "http", AwaitListening: true},
		},
		Health: &components.HealthCheck{
			Endpoint: "admin", Path: "/api/admin/v1/health",
			ExpectStatus: 200, Timeout: timeout, Interval: interval,
		},
	}
}

func instanceFor(t *testing.T, def *components.Definition) *components.Instance {
	t.Helper()
	inst, err := components.NewInstance(def, 0, 1, "127.0.0.1", map[int]int{9092: 49200})
	require.NoError(t, err)
	return inst
}

func TestAwaitHealthy(t *testing.T) {
	t.Run("a component with no health check is ready at started", func(t *testing.T) {
		def := healthyDef(time.Second, 10*time.Millisecond)
		def.Health = nil
		require.NoError(t, AwaitHealthy(context.Background(), instanceFor(t, def), probeFunc(
			func(context.Context, string) (int, error) {
				t.Fatal("should not probe a component with no health check")
				return 0, nil
			})))
	})

	t.Run("passes as soon as the declared status appears", func(t *testing.T) {
		def := healthyDef(2*time.Second, 10*time.Millisecond)
		var calls atomic.Int32
		err := AwaitHealthy(context.Background(), instanceFor(t, def), probeFunc(
			func(context.Context, string) (int, error) {
				if calls.Add(1) < 3 {
					return 0, errors.New("connection refused")
				}
				return 200, nil
			}))
		require.NoError(t, err)
		require.EqualValues(t, 3, calls.Load(), "should stop probing once ready")
	})

	t.Run("probes the endpoint URL joined with the health path", func(t *testing.T) {
		def := healthyDef(time.Second, 10*time.Millisecond)
		var seen string
		require.NoError(t, AwaitHealthy(context.Background(), instanceFor(t, def), probeFunc(
			func(_ context.Context, url string) (int, error) {
				seen = url
				return 200, nil
			})))
		require.Equal(t, "http://127.0.0.1:49200/api/admin/v1/health", seen)
	})

	t.Run("a listening-but-never-ready component fails rather than passing", func(t *testing.T) {
		def := healthyDef(120*time.Millisecond, 20*time.Millisecond)
		err := AwaitHealthy(context.Background(), instanceFor(t, def), probeFunc(
			func(context.Context, string) (int, error) { return 503, nil }))
		require.Error(t, err)
		require.ErrorContains(t, err, "did not become healthy")
		require.ErrorContains(t, err, "last status 503 (wanted 200)")
		require.ErrorContains(t, err, "partial boot")
	})

	t.Run("never answering is distinguished from answering wrongly", func(t *testing.T) {
		def := healthyDef(120*time.Millisecond, 20*time.Millisecond)
		err := AwaitHealthy(context.Background(), instanceFor(t, def), probeFunc(
			func(context.Context, string) (int, error) { return 0, errors.New("dial tcp: refused") }))
		require.ErrorContains(t, err, "last error: dial tcp: refused")
		require.NotContains(t, err.Error(), "last status")
	})

	t.Run("the failure names the URL and the attempt count", func(t *testing.T) {
		def := healthyDef(100*time.Millisecond, 20*time.Millisecond)
		err := AwaitHealthy(context.Background(), instanceFor(t, def), probeFunc(
			func(context.Context, string) (int, error) { return 500, nil }))
		require.ErrorContains(t, err, "http://127.0.0.1:49200/api/admin/v1/health")
		require.ErrorContains(t, err, "time(s)")
		require.ErrorContains(t, err, "gateway-controller")
	})

	t.Run("cancellation stops promptly instead of burning the window", func(t *testing.T) {
		def := healthyDef(30*time.Second, 10*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())
		start := time.Now()
		err := AwaitHealthy(ctx, instanceFor(t, def), probeFunc(
			func(context.Context, string) (int, error) {
				cancel()
				return 503, nil
			}))
		require.ErrorIs(t, err, context.Canceled)
		require.Less(t, time.Since(start), 5*time.Second, "should not wait out the 30s budget")
	})

	t.Run("an unresolvable health endpoint is reported clearly", func(t *testing.T) {
		def := healthyDef(time.Second, 10*time.Millisecond)
		def.Health.Endpoint = "does-not-exist"
		err := AwaitHealthy(context.Background(), instanceFor(t, def), nil)
		require.ErrorContains(t, err, `no endpoint "does-not-exist"`)
	})
}

func TestHTTPProber(t *testing.T) {
	t.Run("reports the observed status", func(t *testing.T) {
		srv := newTestHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}), false)

		code, err := NewHTTPProber(time.Second).Probe(context.Background(), srv.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusTeapot, code)
	})

	t.Run("tolerates a self-signed certificate", func(t *testing.T) {
		srv := newTestHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}), true)

		code, err := NewHTTPProber(time.Second).Probe(context.Background(), srv.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, code)
	})

	t.Run("a dead address is an error, not a status", func(t *testing.T) {
		_, err := NewHTTPProber(500*time.Millisecond).Probe(context.Background(), "http://127.0.0.1:1/health")
		require.Error(t, err)
	})
}

func TestPhaseString(t *testing.T) {
	require.Equal(t, "started", PhaseStarted.String())
	require.Equal(t, "healthy", PhaseHealthy.String())
	require.Equal(t, "schema-applied", PhaseSchemaApplied.String())
	require.Less(t, int(PhaseStarted), int(PhaseHealthy))
	require.Less(t, int(PhaseHealthy), int(PhaseSchemaApplied))
}
