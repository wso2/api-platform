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

package observe

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	sqlite3 "github.com/mattn/go-sqlite3"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBasic(db *fakeDB) driver.Conn  { return &basicConn{db: db} }
func newLegacy(db *fakeDB) driver.Conn { return &legacyConn{basicConn{db: db}} }
func newFull(db *fakeDB) driver.Conn   { return &fullConn{basicConn{db: db}} }

// openObserved opens a pool over the fake driver through Wrap, with one connection so
// that every operation reuses it.
func openObserved(t *testing.T, newConn func(*fakeDB) driver.Conn) (*sql.DB, *fakeDB, *Window, *fakeClock) {
	t.Helper()
	fdb := newFakeDB()
	clk := newFakeClock()
	w := NewWindow(30*time.Second, clk.Now)
	db := sql.OpenDB(Wrap(&fakeConnector{db: fdb, newConn: newConn}, w))
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db, fdb, w, clk
}

// openPlain opens the same fake driver without the wrapper, to compare behaviour.
func openPlain(t *testing.T, newConn func(*fakeDB) driver.Conn) (*sql.DB, *fakeDB) {
	t.Helper()
	fdb := newFakeDB()
	db := sql.OpenDB(&fakeConnector{db: fdb, newConn: newConn})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db, fdb
}

func snapshot(w *Window, clk *fakeClock) Snapshot { return w.Snapshot(clk.Now()) }

func TestWrapFullDriverPassesThroughAndRecords(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "UPDATE t SET a = ?", 1)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT 1").Scan(&n))
	assert.Equal(t, 1, n)
	stmt, err := db.PrepareContext(ctx, "INSERT INTO t VALUES (?)")
	require.NoError(t, err)
	_, err = stmt.ExecContext(ctx, 1)
	require.NoError(t, err)
	rows, err := stmt.QueryContext(ctx, 1)
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	require.NoError(t, stmt.Close())
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	require.NoError(t, db.PingContext(ctx))

	for _, method := range []string{"Connect", "ExecContext", "QueryContext", "PrepareContext", "StmtExecContext",
		"StmtQueryContext", "BeginTx", "Commit", "Rollback", "Ping", "ResetSession", "StmtCheckNamedValue",
		"ConnCheckNamedValue", "IsValid"} {
		assert.NotZero(t, fdb.count(method), "%s was not passed through", method)
	}
	for _, method := range []string{"Prepare", "Begin", "StmtExec", "StmtQuery"} {
		assert.Zero(t, fdb.count(method), "%s used although the context-aware method exists", method)
	}
	s := snapshot(w, clk)
	assert.Zero(t, s.Failures)
	// Connect, Exec, Query, Prepare, stmt Exec and Query, two BeginTx, Commit, Rollback,
	// Ping, plus one session reset for every reuse of the pooled connection.
	assert.Equal(t, 11+fdb.count("ResetSession"), s.Attempts)
	assert.Equal(t, clk.Now(), s.LastActivity)
}

func TestWrapRecordsFailuresAndReturnsDriverErrorsUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target func(err error) bool
	}{
		{"postgres", &pgconn.PgError{Code: "08006"}, func(err error) bool {
			var e *pgconn.PgError
			return errors.As(err, &e) && e.Code == "08006"
		}},
		{"sqlite", sqlite3.Error{Code: sqlite3.ErrIoErr}, func(err error) bool {
			var e sqlite3.Error
			return errors.As(err, &e) && e.Code == sqlite3.ErrIoErr
		}},
		{"sql server", mssql.Error{Number: 18456}, func(err error) bool {
			var e mssql.Error
			return errors.As(err, &e) && e.Number == 18456
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, fdb, w, clk := openObserved(t, newFull)
			fdb.fail("ExecContext", tt.err)

			_, err := db.ExecContext(context.Background(), "UPDATE t SET a = 1")

			require.Error(t, err)
			assert.True(t, tt.target(err), "driver error type lost: %T %v", err, err)
			s := snapshot(w, clk)
			assert.Equal(t, 1, s.Failures)
			assert.Equal(t, FailureUnreachable, s.LastFailureKind)
		})
	}
}

func TestWrapQueryErrorsDoNotCount(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	fdb.fail("ExecContext", &pgconn.PgError{Code: "23505"})
	fdb.fail("QueryContext", context.DeadlineExceeded)

	_, err := db.ExecContext(context.Background(), "INSERT")
	require.Error(t, err)
	_, err = db.QueryContext(context.Background(), "SELECT")
	require.Error(t, err)

	s := snapshot(w, clk)
	assert.Equal(t, 3+fdb.count("ResetSession"), s.Attempts, "connect, exec, query and session resets")
	assert.Zero(t, s.Failures)
}

func TestWrapFailedConnectIsCounted(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	fdb.fail("Connect", errors.New("password authentication failed"))

	require.Error(t, db.PingContext(context.Background()))

	s := snapshot(w, clk)
	assert.Positive(t, s.Failures)
	assert.Equal(t, s.Attempts, s.Failures)
}

func TestWrapFailedPingIsCounted(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	fdb.fail("Ping", errors.New("server closed the connection"))

	require.Error(t, db.PingContext(context.Background()))

	assert.Equal(t, 1, snapshot(w, clk).Failures)
}

// A driver reports a connection that broke while idle from ResetSession, as pgx does
// after its idle ping fails; database/sql swallows that error and reconnects, but the
// wrapper still counts it.
func TestWrapCountsBadConnectionFromSessionReset(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, "UPDATE")
	require.NoError(t, err)
	fdb.failNext("ResetSession", driver.ErrBadConn)

	_, err = db.ExecContext(ctx, "UPDATE")

	require.NoError(t, err, "database/sql retries on a fresh connection")
	assert.Equal(t, 2, fdb.count("Connect"))
	assert.Equal(t, 1, snapshot(w, clk).Failures)
}

func TestWrapHealthPingIsNotActivity(t *testing.T) {
	db, _, w, clk := openObserved(t, newFull)

	require.NoError(t, db.PingContext(WithHealthPing(context.Background())))

	s := snapshot(w, clk)
	assert.Equal(t, 2, s.Attempts, "connect and ping")
	assert.Equal(t, clk.Now(), s.LastPing)
	assert.True(t, s.LastActivity.IsZero())

	clk.Advance(time.Second)
	require.NoError(t, db.PingContext(context.Background()))
	s = snapshot(w, clk)
	assert.Equal(t, clk.Now(), s.LastActivity)
	assert.Equal(t, clk.Now().Add(-time.Second), s.LastPing)
}

// A driver with none of the optional interfaces behaves through the wrapper exactly as
// it does without it.
func TestWrapBasicDriverFallsBackLikeDatabaseSQL(t *testing.T) {
	observed, ofdb, w, clk := openObserved(t, newBasic)
	plain, pfdb := openPlain(t, newBasic)
	ctx := context.Background()

	for _, db := range []*sql.DB{observed, plain} {
		_, err := db.ExecContext(ctx, "UPDATE t SET a = ?", 1)
		require.NoError(t, err)
		var n int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT ?", 1).Scan(&n))
		require.NoError(t, db.PingContext(ctx))
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		_, err = db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		assert.EqualError(t, err, "sql: driver does not support non-default isolation level")
		_, err = db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		assert.EqualError(t, err, "sql: driver does not support read-only transactions")
		_, err = db.ExecContext(ctx, "UPDATE t SET a = @a", sql.Named("a", 1))
		assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
	}
	for _, method := range []string{"Prepare", "StmtExec", "StmtQuery", "Begin", "Commit", "StmtClose", "Connect"} {
		assert.Equal(t, pfdb.count(method), ofdb.count(method), method)
	}
	assert.Zero(t, snapshot(w, clk).Failures)
	assert.Positive(t, snapshot(w, clk).Attempts)
}

func TestWrapLegacyExecerAndQueryer(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newLegacy)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "UPDATE t SET a = ?", 1)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT ?", 1).Scan(&n))
	assert.Equal(t, 1, fdb.count("ConnExec"))
	assert.Equal(t, 1, fdb.count("ConnQuery"))
	assert.Zero(t, fdb.count("Prepare"))

	_, err = db.ExecContext(ctx, "UPDATE t SET a = @a", sql.Named("a", 1))
	assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
	_, err = db.QueryContext(ctx, "SELECT @a", sql.Named("a", 1))
	assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")

	fdb.fail("ConnExec", driver.ErrSkip)
	_, err = db.ExecContext(ctx, "UPDATE")
	require.NoError(t, err, "ErrSkip falls back to a prepared statement")
	assert.Equal(t, 1, fdb.count("StmtExec"))
	assert.Equal(t, 5, snapshot(w, clk).Attempts, "connect, exec, query, prepare, statement exec")
}

func TestConnFallbacksStopOnDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fdb := newFakeDB()
	w := NewWindow(time.Minute, nil)

	legacy := wrapConn(newLegacy(fdb), w).(*conn)
	_, err := legacy.ExecContext(ctx, "UPDATE", nil)
	assert.ErrorIs(t, err, context.Canceled)
	_, err = legacy.QueryContext(ctx, "SELECT", nil)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, fdb.count("ConnExec")+fdb.count("ConnQuery"))

	basic := wrapConn(newBasic(fdb), w).(*conn)
	_, err = basic.PrepareContext(ctx, "SELECT")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, fdb.count("StmtClose"), "the statement prepared too late is closed")
	_, err = basic.BeginTx(ctx, driver.TxOptions{})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, fdb.count("Rollback"), "the transaction begun too late is rolled back")
	assert.NoError(t, basic.Ping(ctx), "a driver without Pinger always answers")

	s, err := basic.Prepare("SELECT")
	require.NoError(t, err)
	_, err = s.(driver.StmtExecContext).ExecContext(ctx, nil)
	assert.ErrorIs(t, err, context.Canceled)
	_, err = s.(driver.StmtQueryContext).QueryContext(ctx, nil)
	assert.ErrorIs(t, err, context.Canceled)
	_, err = s.(driver.StmtExecContext).ExecContext(context.Background(), []driver.NamedValue{{Name: "a", Ordinal: 1}})
	assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
	_, err = s.(driver.StmtQueryContext).QueryContext(context.Background(), []driver.NamedValue{{Name: "a", Ordinal: 1}})
	assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
	assert.Zero(t, fdb.count("StmtExec")+fdb.count("StmtQuery"))

	assert.Zero(t, w.Snapshot(time.Now()).Failures)
}

// The context-free methods database/sql never calls on the wrapper still pass through
// and record.
func TestConnContextFreeMethods(t *testing.T) {
	fdb := newFakeDB()
	clk := newFakeClock()
	w := NewWindow(time.Minute, clk.Now)
	c := wrapConn(newFull(fdb), w).(resettingValidatingConn)

	s, err := c.Prepare("SELECT")
	require.NoError(t, err)
	_, err = s.Exec(nil)
	require.NoError(t, err)
	_, err = s.Query(nil)
	require.NoError(t, err)
	assert.Equal(t, -1, s.NumInput())
	tx, err := c.Begin()
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, c.Close())

	fdb.fail("Prepare", driver.ErrBadConn)
	fdb.fail("Begin", driver.ErrBadConn)
	_, err = c.Prepare("SELECT")
	assert.ErrorIs(t, err, driver.ErrBadConn)
	_, err = c.Begin()
	assert.ErrorIs(t, err, driver.ErrBadConn)

	snap := w.Snapshot(clk.Now())
	assert.Equal(t, 7, snap.Attempts)
	assert.Equal(t, 2, snap.Failures)
	for _, method := range []string{"Prepare", "StmtExec", "StmtQuery", "Begin", "Commit", "Close"} {
		assert.NotZero(t, fdb.count(method), method)
	}
}

// The wrapper exposes SessionResetter and Validator exactly when the driver does,
// because database/sql keeps a connection after a rollback only when both exist.
func TestWrapConnMatchesResetterAndValidator(t *testing.T) {
	w := NewWindow(time.Minute, nil)
	tests := []struct {
		name                string
		conn                driver.Conn
		resetter, validator bool
	}{
		{"neither", newBasic(newFakeDB()), false, false},
		{"resetter only", &resettingOnlyConn{basicConn{db: newFakeDB()}}, true, false},
		{"validator only", &validatingOnlyConn{basicConn{db: newFakeDB()}}, false, true},
		{"both", newFull(newFakeDB()), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapConn(tt.conn, w)
			r, isResetter := wrapped.(driver.SessionResetter)
			v, isValidator := wrapped.(driver.Validator)
			assert.Equal(t, tt.resetter, isResetter)
			assert.Equal(t, tt.validator, isValidator)
			for _, iface := range []bool{
				implements[driver.Pinger](wrapped), implements[driver.ExecerContext](wrapped),
				implements[driver.QueryerContext](wrapped), implements[driver.ConnPrepareContext](wrapped),
				implements[driver.ConnBeginTx](wrapped), implements[driver.NamedValueChecker](wrapped),
			} {
				assert.True(t, iface)
			}
			if isResetter {
				assert.NoError(t, r.ResetSession(context.Background()))
			}
			if isValidator {
				assert.True(t, v.IsValid())
			}
		})
	}
}

func implements[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

// A connection the driver marks invalid is dropped from the pool, and the check is not
// itself counted as a failure.
func TestWrapInvalidConnectionIsDroppedNotCounted(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	fdb.valid = false

	_, err := db.ExecContext(context.Background(), "UPDATE")
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), "UPDATE")
	require.NoError(t, err)

	assert.Equal(t, 2, fdb.count("Connect"), "the invalid connection was not reused")
	assert.Zero(t, snapshot(w, clk).Failures)
}

// The statement's own argument checker comes first, then the connection's, as in
// database/sql.
func TestStmtCheckNamedValueOrder(t *testing.T) {
	fdb := newFakeDB()
	w := NewWindow(time.Minute, nil)
	full := wrapConn(newFull(fdb), w).(resettingValidatingConn)
	nv := &driver.NamedValue{Ordinal: 1, Value: 1}

	s := wrapStmt(&fullStmt{basicStmt{db: fdb}}, full.conn).(driver.NamedValueChecker)
	assert.ErrorIs(t, s.CheckNamedValue(nv), driver.ErrSkip)
	assert.Equal(t, 1, fdb.count("StmtCheckNamedValue"))
	assert.Zero(t, fdb.count("ConnCheckNamedValue"))

	s = wrapStmt(&basicStmt{db: fdb}, full.conn).(driver.NamedValueChecker)
	assert.ErrorIs(t, s.CheckNamedValue(nv), driver.ErrSkip)
	assert.Equal(t, 1, fdb.count("ConnCheckNamedValue"))

	basic := wrapConn(newBasic(fdb), w).(*conn)
	s = wrapStmt(&basicStmt{db: fdb}, basic).(driver.NamedValueChecker)
	assert.ErrorIs(t, s.CheckNamedValue(nv), driver.ErrSkip)

	rejected := errors.New("unsupported type")
	fdb.fail("ConnCheckNamedValue", rejected)
	assert.ErrorIs(t, full.CheckNamedValue(nv), rejected)
}

// A statement's column converter is still offered to database/sql.
func TestStmtColumnConverterPassesThrough(t *testing.T) {
	fdb := newFakeDB()
	w := NewWindow(time.Minute, nil)
	c := wrapConn(newBasic(fdb), w).(*conn)

	s := wrapStmt(&columnConvertingStmt{basicStmt{db: fdb}}, c)
	cc, ok := s.(driver.ColumnConverter)
	require.True(t, ok)
	assert.Equal(t, driver.DefaultParameterConverter, cc.ColumnConverter(0))
	assert.Equal(t, 1, fdb.count("ColumnConverter"))

	_, ok = wrapStmt(&basicStmt{db: fdb}, c).(driver.ColumnConverter)
	assert.False(t, ok)
}

func TestConnectorPassesThroughDriverAndClose(t *testing.T) {
	w := NewWindow(time.Minute, nil)
	fdb := newFakeDB()

	plain := Wrap(&fakeConnector{db: fdb, newConn: newBasic}, w)
	assert.Equal(t, fakeDriver{}, plain.Driver())
	assert.NoError(t, plain.(interface{ Close() error }).Close())

	closing := Wrap(&closingConnector{fakeConnector{db: fdb, newConn: newBasic}}, w)
	fdb.fail("ConnectorClose", errRefused)
	assert.ErrorIs(t, closing.(interface{ Close() error }).Close(), errRefused)
	assert.Equal(t, 1, fdb.count("ConnectorClose"))
}

func TestTxFailuresAreRecorded(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	fdb.fail("Commit", driver.ErrBadConn)

	tx, err := db.Begin()
	require.NoError(t, err)
	assert.ErrorIs(t, tx.Commit(), driver.ErrBadConn)

	assert.Equal(t, 1, snapshot(w, clk).Failures)
}

func TestStmtFailuresAreRecorded(t *testing.T) {
	db, fdb, w, clk := openObserved(t, newFull)
	fdb.fail("StmtExecContext", io.ErrUnexpectedEOF)
	fdb.fail("StmtQueryContext", driver.ErrBadConn)

	stmt, err := db.Prepare("UPDATE")
	require.NoError(t, err)
	_, err = stmt.Exec()
	require.Error(t, err)
	_, err = stmt.Query()
	require.Error(t, err)
	require.NoError(t, stmt.Close())

	assert.Positive(t, snapshot(w, clk).Failures)
	fdb.fail("PrepareContext", driver.ErrBadConn)
	_, err = db.Prepare("UPDATE")
	require.Error(t, err)
	fdb.fail("BeginTx", driver.ErrBadConn)
	_, err = db.Begin()
	require.Error(t, err)
	fdb.fail("QueryContext", driver.ErrBadConn)
	_, err = db.Query("SELECT")
	require.Error(t, err)
}
