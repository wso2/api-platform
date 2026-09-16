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
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
	_ "github.com/mattn/go-sqlite3"    // registers the "sqlite3" driver

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// OpenComponentDB opens a direct connection to the SQL store a component owns, for a step
// that must assert on what is actually persisted rather than on what a product API chooses
// to return.
//
// embeddedService names the compose service holding the store when the engine is embedded
// (SQLite lives inside the owning application's own container, not a database server this
// block started) - see Store.Embedded. It is ignored for Postgres/SQLServer, which are
// reached over the network instead. The caller must call the returned close function when
// done; for an embedded store this also removes the local copy of the database file made to
// read it.
func (t *Topology) OpenComponentDB(ctx context.Context, component, embeddedService string) (*sql.DB, func() error, error) {
	if t == nil || t.Storage == nil || t.Storage.Plan == nil {
		return nil, nil, fmt.Errorf("runtime: block has no storage plan")
	}
	store, ok := t.Storage.Plan.StoreFor(component, 0)
	if !ok {
		return nil, nil, fmt.Errorf("runtime: component %q has no assigned store", component)
	}

	if store.Embedded {
		return t.openEmbeddedSQLite(ctx, embeddedService, store.Database)
	}

	server, ok := t.Storage.Servers[store.Type]
	if !ok {
		return nil, nil, fmt.Errorf("runtime: no %s server is running in this block", store.Type)
	}
	host := server.Instance.Host()
	port, err := server.Instance.MappedPort("sql")
	if err != nil {
		return nil, nil, err
	}

	switch store.Type {
	case components.Postgres:
		db, err := sql.Open("pgx", postgresDSN(host, port, t.Storage.Credentials, store.Database))
		if err != nil {
			return nil, nil, fmt.Errorf("runtime: opening postgres connection: %w", err)
		}
		return db, db.Close, nil
	case components.SQLServer:
		db, err := sql.Open("sqlserver", sqlServerDSN(host, port, t.Storage.Credentials, store.Database))
		if err != nil {
			return nil, nil, fmt.Errorf("runtime: opening sqlserver connection: %w", err)
		}
		return db, db.Close, nil
	default:
		return nil, nil, fmt.Errorf("runtime: %s has no direct-access driver", store.Type)
	}
}

// postgresDSN builds a driver DSN against the mapped host port. Host-side coordinates, not
// the internal docker-network alias postgresEngine.dsn resolves for a container's own
// environment - this connection is dialled from the test process itself.
func postgresDSN(host string, port int, creds Credentials, database string) string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(creds.User, creds.Password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// openEmbeddedSQLite copies the SQLite database file out of the container that owns it and
// opens the local copy. A snapshot, not a live connection: the framework has no way to share a
// live file handle with the owning process, and the container may still be writing to it, so a
// caller polling for an eventually-consistent value must re-copy on every attempt rather than
// reusing one open *sql.DB across a retry loop.
//
// The owning process runs in WAL mode (see pkg/storage/sqlite.go's _journal_mode=WAL), so a
// recent commit can live entirely in the -wal file with the main .db file unchanged. Copying
// only the .db file would silently read stale data. The -wal/-shm sidecars are copied
// best-effort alongside it - absent when everything is already checkpointed - and the local
// copy is opened read-write (never mode=ro) so SQLite performs its own WAL replay on open,
// exactly as it would for any other database carrying a WAL.
func (t *Topology) openEmbeddedSQLite(ctx context.Context, service, database string) (*sql.DB, func() error, error) {
	if service == "" {
		return nil, nil, fmt.Errorf("runtime: reading an embedded SQLite store requires the owning service name")
	}
	stack, resolvedService, err := t.ServiceControl(service)
	if err != nil {
		return nil, nil, err
	}

	dir, err := os.MkdirTemp("", "apip-it-sqlite-*")
	if err != nil {
		return nil, nil, fmt.Errorf("runtime: staging a local sqlite copy: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(dir) }

	dbPath := filepath.Join(dir, database+".db")
	if err := copyContainerFile(ctx, stack, resolvedService, fmt.Sprintf("/app/data/%s.db", database), dbPath); err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("runtime: copying %s's database file: %w", service, err)
	}
	// Best-effort: absent whenever the owner has nothing outstanding to checkpoint.
	_ = copyContainerFile(ctx, stack, resolvedService, fmt.Sprintf("/app/data/%s.db-wal", database), dbPath+"-wal")
	_ = copyContainerFile(ctx, stack, resolvedService, fmt.Sprintf("/app/data/%s.db-shm", database), dbPath+"-shm")

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("runtime: opening local sqlite copy: %w", err)
	}
	return db, func() error {
		closeErr := db.Close()
		removeErr := cleanup()
		if closeErr != nil {
			return closeErr
		}
		return removeErr
	}, nil
}

// copyContainerFile copies one file from a service's container to a local path.
func copyContainerFile(ctx context.Context, stack *ComposeStack, service, containerPath, localPath string) error {
	data, err := stack.CopyFileFromContainer(ctx, service, containerPath)
	if err != nil {
		return err
	}
	return os.WriteFile(localPath, data, 0o600)
}
