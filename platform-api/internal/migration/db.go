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

package migration

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Register the pgx stdlib driver under the name "pgx" — the SAME driver v2's
	// own internal/database uses. This matters for §B.3: v2 wrote its TIMESTAMP
	// columns through this codec (discardTimeZone keeps wall-clock digits), so
	// reading v1 through the identical codec reproduces exactly what was stored.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const pgxDriver = "pgx"

// openDB opens and pings a Postgres endpoint. Connection pool sizes are modest —
// the migration is a single sequential pipeline, not a serving workload.
func openDB(ctx context.Context, conn DBConn) (*sql.DB, error) {
	dsn, err := conn.DSN()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(pgxDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", conn.Redacted(), err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s: %w", conn.Redacted(), err)
	}
	return db, nil
}

// Endpoints holds the two live connections for a run. In forward mode Source is
// v1 and Target is v2; in reverse mode the roles are swapped by the caller.
type Endpoints struct {
	Source *sql.DB
	Target *sql.DB

	// SourceIsV1 records the physical direction so kernels/migrators can key
	// timezone and split/recombine behavior off it.
	SourceIsV1 bool
}

// Close releases both connections.
func (e *Endpoints) Close() {
	if e.Source != nil {
		_ = e.Source.Close()
	}
	if e.Target != nil && e.Target != e.Source {
		_ = e.Target.Close()
	}
}

// withTx runs fn inside a transaction on db, committing on success and rolling
// back on any error or panic. This is the batch boundary: a failed row rolls back
// its whole batch (unless --continue-on-error is set, handled by the caller).
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// scanCount runs a single-row COUNT/scalar query and returns the int result.
func scanCount(ctx context.Context, q queryer, query string, args ...any) (int64, error) {
	var n int64
	if err := q.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// queryer is satisfied by both *sql.DB and *sql.Tx, so helpers can run against
// either a pooled connection or an open transaction.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
