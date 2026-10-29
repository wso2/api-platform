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

package observe

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openDB opens a database/sql pool over a fake connector through the wrapper.
func openDB(t *testing.T, c driver.Connector, w *Window) *sql.DB {
	t.Helper()
	db := sql.OpenDB(Wrap(c, w))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestWindow() (*Window, *fakeClock) {
	clk := newFakeClock()
	return NewWindow(30*time.Second, clk.Now), clk
}

func snapshot(w *Window) Snapshot { return w.Snapshot(w.now()) }

func TestFullDriverOperationsPassThroughAndAreRecorded(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s}, w)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "UPDATE t SET v = ?", 1)
	require.NoError(t, err)
	var v int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT v FROM t").Scan(&v))
	assert.Equal(t, 1, v)

	st, err := db.PrepareContext(ctx, "SELECT v FROM t WHERE id = ?")
	require.NoError(t, err)
	_, err = st.ExecContext(ctx, 1)
	require.NoError(t, err)
	require.NoError(t, st.QueryRowContext(ctx, 1).Scan(&v))
	require.NoError(t, st.Close())

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "DELETE FROM t")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	require.NoError(t, db.PingContext(ctx))

	for method, want := range map[string]int{
		"Connect": 1, "ExecContext": 2, "QueryContext": 1, "PrepareContext": 1,
		"StmtExecContext": 1, "StmtQueryContext": 1, "BeginTx": 2, "Commit": 1,
		"Rollback": 1, "Ping": 1, "StmtCheckNamedValue": 2,
	} {
		assert.Equal(t, want, s.called(method), method)
	}
	assert.Positive(t, s.called("ResetSession"), "reused connections are reset")
	assert.Positive(t, s.called("IsValid"), "connections are validated on return to the pool")
	assert.Positive(t, s.called("CheckNamedValue"), "the connection's argument checker is consulted")
	assert.Zero(t, s.called("Prepare")+s.called("Begin"), "context-free methods are not used when the driver has the context variants")

	snap := w.Snapshot(clk.Now())
	resets := s.called("ResetSession")
	assert.Equal(t, 12+resets, snap.Attempts, "every operation and session reset is one attempt")
	assert.Zero(t, snap.Failures)
	assert.Equal(t, clk.Now(), snap.LastActivity)
	assert.True(t, snap.LastPing.IsZero())
	assert.Equal(t, NoFailure, snap.LastFailure)
}

func TestMinimalDriverFallsBackLikeDatabaseSQL(t *testing.T) {
	for _, plain := range []bool{false, true} {
		t.Run(fmt.Sprintf("plain statement %v", plain), func(t *testing.T) {
			s := newScript()
			w, clk := newTestWindow()
			db := openDB(t, &fakeConnector{s: s, kind: minimalKind, plainStmt: plain}, w)
			ctx := context.Background()

			_, err := db.ExecContext(ctx, "UPDATE t SET v = ?", 1)
			require.NoError(t, err)
			var v int
			require.NoError(t, db.QueryRowContext(ctx, "SELECT v FROM t").Scan(&v))
			assert.Equal(t, 1, v)
			require.NoError(t, db.PingContext(ctx))

			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			require.NoError(t, tx.Commit())

			_, err = db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
			assert.EqualError(t, err, "sql: driver does not support non-default isolation level")
			_, err = db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			assert.EqualError(t, err, "sql: driver does not support read-only transactions")

			assert.Equal(t, 2, s.called("Prepare"), "exec and query go through a prepared statement")
			assert.Equal(t, 1, s.called("Begin"))
			if plain {
				assert.Equal(t, 1, s.called("StmtExec"))
				assert.Equal(t, 1, s.called("StmtQuery"))
			} else {
				assert.Equal(t, 1, s.called("StmtExecContext"))
				assert.Equal(t, 1, s.called("StmtQueryContext"))
			}
			snap := w.Snapshot(clk.Now())
			assert.Zero(t, snap.Failures)
			// connect, two prepares, two statement runs, ping, three begins, commit
			assert.Equal(t, 10, snap.Attempts)
		})
	}
}

func TestLegacyDriverUsesContextFreeExecAndQuery(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s, kind: legacyKind}, w)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "UPDATE t SET v = ?", 1)
	require.NoError(t, err)
	var v int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT v FROM t").Scan(&v))
	assert.Equal(t, 1, s.called("Exec"))
	assert.Equal(t, 1, s.called("Query"))
	assert.Zero(t, s.called("Prepare"))

	_, err = db.ExecContext(ctx, "UPDATE t SET v = :v", sql.Named("v", 1))
	assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
	_, err = db.QueryContext(ctx, "SELECT v FROM t WHERE v = :v", sql.Named("v", 1))
	assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
	assert.Equal(t, 3, w.Snapshot(clk.Now()).Attempts, "connect, exec and query")
}

func TestDriverErrorsAreReturnedUnchanged(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "uk_name"}
	msErr := mssql.Error{Number: 2627, Message: "duplicate key"}
	liteErr := sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintUnique}

	t.Run("postgres exec", func(t *testing.T) {
		s := newScript()
		s.fail("ExecContext", pgErr)
		w, clk := newTestWindow()
		_, err := openDB(t, &fakeConnector{s: s}, w).Exec("INSERT INTO t VALUES (1)")
		var got *pgconn.PgError
		require.ErrorAs(t, err, &got)
		assert.Same(t, pgErr, got)
		assert.Zero(t, w.Snapshot(clk.Now()).Failures, "a constraint violation is not a connection failure")
	})
	t.Run("sql server prepared statement", func(t *testing.T) {
		s := newScript()
		s.fail("StmtExecContext", msErr)
		_, err := openDB(t, &fakeConnector{s: s, kind: minimalKind}, nil).Exec("INSERT INTO t VALUES (1)")
		var got mssql.Error
		require.ErrorAs(t, err, &got)
		assert.Equal(t, int32(2627), got.Number)
	})
	t.Run("sqlite query", func(t *testing.T) {
		s := newScript()
		s.fail("QueryContext", liteErr)
		_, err := openDB(t, &fakeConnector{s: s}, nil).Query("SELECT 1")
		var got sqlite3.Error
		require.ErrorAs(t, err, &got)
		assert.Equal(t, sqlite3.ErrConstraintUnique, got.ExtendedCode)
	})
	t.Run("commit", func(t *testing.T) {
		s := newScript()
		s.fail("Commit", pgErr)
		tx, err := openDB(t, &fakeConnector{s: s}, nil).Begin()
		require.NoError(t, err)
		var got *pgconn.PgError
		require.ErrorAs(t, tx.Commit(), &got)
	})
}

func TestConnectionFailuresAreCounted(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	tests := []struct {
		name     string
		method   string
		err      error
		failures int
		kind     FailureKind
	}{
		{name: "connect refused", method: "Connect", err: refused, failures: 1, kind: Unreachable},
		{name: "connect with any error", method: "Connect", err: errors.New("tls handshake failure"), failures: 1, kind: Unreachable},
		{name: "connect deadline", method: "Connect", err: context.DeadlineExceeded, failures: 1, kind: Timeout},
		{name: "connect cancelled by the caller", method: "Connect", err: context.Canceled, failures: 0, kind: NoFailure},
		{name: "exec on a broken connection", method: "ExecContext", err: io.ErrUnexpectedEOF, failures: 1, kind: Unreachable},
		{name: "exec deadline", method: "ExecContext", err: context.DeadlineExceeded, failures: 0, kind: NoFailure},
		{name: "exec syntax error", method: "ExecContext", err: &pgconn.PgError{Code: "42601"}, failures: 0, kind: NoFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScript()
			s.fail(tt.method, tt.err)
			w, clk := newTestWindow()
			_, err := openDB(t, &fakeConnector{s: s}, w).Exec("UPDATE t SET v = 1")
			require.ErrorIs(t, err, tt.err)

			snap := w.Snapshot(clk.Now())
			assert.Positive(t, snap.Attempts)
			assert.Equal(t, tt.failures, min(snap.Failures, 1))
			assert.Equal(t, tt.kind, snap.LastFailure)
		})
	}
}

func TestPingErrorsAlwaysCount(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s}, w)
	require.NoError(t, db.Ping())

	s.fail("Ping", errors.New("server closed the connection"))
	require.Error(t, db.Ping())

	snap := w.Snapshot(clk.Now())
	assert.Equal(t, 1, snap.Failures)
	assert.Equal(t, Unreachable, snap.LastFailure)
}

func TestBrokenConnectionFoundOnReuseIsCounted(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s}, w)
	_, err := db.Exec("UPDATE t SET v = 1")
	require.NoError(t, err)

	// The pooled connection died while idle; database/sql discards it and dials anew.
	s.fail("ResetSession", driver.ErrBadConn)
	_, err = db.Exec("UPDATE t SET v = 1")
	require.NoError(t, err)

	assert.Equal(t, 2, s.called("Connect"))
	snap := w.Snapshot(clk.Now())
	assert.Equal(t, 1, snap.Failures)
	assert.Equal(t, Unreachable, snap.LastFailure)
}

func TestBrokenConnectionAfterCallerGaveUpIsNotCounted(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			s := newScript()
			w, clk := newTestWindow()
			db := openDB(t, &fakeConnector{s: s}, w)

			// A driver may close the connection when its caller gives up mid-query.
			s.fail("ExecContext", cause)
			_, err := db.Exec("UPDATE t SET v = 1")
			require.ErrorIs(t, err, cause)
			s.fail("ExecContext", nil)
			s.fail("ResetSession", driver.ErrBadConn)
			_, err = db.Exec("UPDATE t SET v = 1")
			require.NoError(t, err)

			assert.Zero(t, w.Snapshot(clk.Now()).Failures)
		})
	}
}

func TestInvalidConnectionIsDiscardedButNotCounted(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s}, w)
	// A driver may mark a connection invalid after a query merely ran past its deadline.
	s.invalid = true

	_, err := db.Exec("UPDATE t SET v = 1")
	require.NoError(t, err)
	_, err = db.Exec("UPDATE t SET v = 1")
	require.NoError(t, err)

	assert.Equal(t, 2, s.called("Connect"), "an invalid connection is not reused")
	assert.Equal(t, 2, s.called("IsValid"))
	snap := w.Snapshot(clk.Now())
	assert.Zero(t, snap.Failures)
	assert.Equal(t, 4, snap.Attempts, "two connects and two execs")
	assert.Equal(t, NoFailure, snap.LastFailure)
}

func TestWrappedConnExposesTheDriversResetAndValidity(t *testing.T) {
	tests := []struct {
		name                string
		kind                connKind
		resetter, validator bool
	}{
		{name: "neither", kind: minimalKind},
		{name: "resetter only", kind: resetterKind, resetter: true},
		{name: "validator only", kind: validatorKind, validator: true},
		{name: "both", kind: fullKind, resetter: true, validator: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newScript()
			raw, err := (&fakeConnector{s: s, kind: tt.kind}).Connect(context.Background())
			require.NoError(t, err)
			_, rawResets := raw.(driver.SessionResetter)
			_, rawValidates := raw.(driver.Validator)
			require.Equal(t, tt.resetter, rawResets)
			require.Equal(t, tt.validator, rawValidates)

			wrapped, err := Wrap(&fakeConnector{s: s, kind: tt.kind}, nil).Connect(context.Background())
			require.NoError(t, err)
			resetter, resets := wrapped.(driver.SessionResetter)
			validator, validates := wrapped.(driver.Validator)
			assert.Equal(t, tt.resetter, resets)
			assert.Equal(t, tt.validator, validates)
			for _, iface := range []bool{
				implements[driver.Pinger](wrapped), implements[driver.ExecerContext](wrapped),
				implements[driver.QueryerContext](wrapped), implements[driver.ConnPrepareContext](wrapped),
				implements[driver.ConnBeginTx](wrapped), implements[driver.NamedValueChecker](wrapped),
			} {
				assert.True(t, iface, "the other optional interfaces are always present")
			}

			if resets {
				require.NoError(t, resetter.ResetSession(context.Background()))
				assert.Equal(t, 1, s.called("ResetSession"))
			}
			if validates {
				assert.True(t, validator.IsValid())
				assert.Equal(t, 1, s.called("IsValid"))
			}
		})
	}
}

func implements[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

// database/sql discards the connection of a transaction rolled back by its context
// unless the connection can both reset its session and report its validity. The
// wrapper must not change that decision.
func TestCancelledTransactionKeepsConnectionAsTheRawDriverDoes(t *testing.T) {
	connectsAfterCancelledTx := func(t *testing.T, kind connKind, wrap bool) int {
		t.Helper()
		s := newScript()
		var c driver.Connector = &fakeConnector{s: s, kind: kind}
		if wrap {
			c = Wrap(c, nil)
		}
		db := sql.OpenDB(c)
		defer db.Close()

		ctx, cancel := context.WithCancel(context.Background())
		_, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		cancel()
		require.Eventually(t, func() bool { return s.called("Rollback") == 1 }, 2*time.Second, time.Millisecond)
		require.Eventually(t, func() bool { return db.Stats().InUse == 0 }, 2*time.Second, time.Millisecond)

		require.NoError(t, db.Ping())
		return s.called("Connect")
	}
	for _, kind := range []connKind{minimalKind, resetterKind, validatorKind, fullKind} {
		raw := connectsAfterCancelledTx(t, kind, false)
		wrapped := connectsAfterCancelledTx(t, kind, true)
		assert.Equal(t, raw, wrapped, "connection kind %d", kind)
	}
	assert.Equal(t, 2, connectsAfterCancelledTx(t, resetterKind, true), "a resetter-only connection is discarded")
	assert.Equal(t, 1, connectsAfterCancelledTx(t, fullKind, true), "a resetting, validating connection is kept")
}

func TestHealthPingIsNotApplicationActivity(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s}, w)
	pingCtx := WithHealthPing(context.Background())

	require.NoError(t, db.PingContext(pingCtx))
	pingedAt := clk.Now()
	clk.Advance(time.Second)
	require.NoError(t, db.PingContext(pingCtx), "a reused connection is reset under the ping's context")

	snap := w.Snapshot(clk.Now())
	assert.True(t, snap.LastActivity.IsZero(), "the health ping must not look like traffic")
	assert.Equal(t, pingedAt.Add(time.Second), snap.LastPing)
	assert.Positive(t, snap.Attempts)

	clk.Advance(time.Second)
	_, err := db.Exec("UPDATE t SET v = 1")
	require.NoError(t, err)
	snap = w.Snapshot(clk.Now())
	assert.Equal(t, clk.Now(), snap.LastActivity)
	assert.Equal(t, pingedAt.Add(time.Second), snap.LastPing)
}

func TestHealthPingFailureIsCounted(t *testing.T) {
	s := newScript()
	s.fail("Connect", &net.OpError{Op: "dial", Err: timeoutError{}})
	w, clk := newTestWindow()
	db := openDB(t, &fakeConnector{s: s}, w)

	require.Error(t, db.PingContext(WithHealthPing(context.Background())))

	snap := w.Snapshot(clk.Now())
	assert.Positive(t, snap.Failures)
	assert.Equal(t, Timeout, snap.LastFailure)
	assert.True(t, snap.LastActivity.IsZero())
	assert.Equal(t, clk.Now(), snap.LastPing)
}

func TestConnectorPassThrough(t *testing.T) {
	t.Run("driver", func(t *testing.T) {
		s := newScript()
		c := Wrap(&fakeConnector{s: s}, nil)
		assert.Equal(t, fakeDriver{s: s}, c.Driver())
	})
	t.Run("close reaches a closable connector", func(t *testing.T) {
		s := newScript()
		db := sql.OpenDB(Wrap(&closingConnector{fakeConnector{s: s}}, nil))
		require.NoError(t, db.Close())
		assert.Equal(t, 1, s.called("ConnectorClose"))
	})
	t.Run("close without a closable connector", func(t *testing.T) {
		db := sql.OpenDB(Wrap(&fakeConnector{s: newScript()}, nil))
		assert.NoError(t, db.Close())
	})
	t.Run("nil window records privately", func(t *testing.T) {
		c := Wrap(&fakeConnector{s: newScript()}, nil).(*connector)
		require.NotNil(t, c.window)
		db := sql.OpenDB(c)
		defer db.Close()
		require.NoError(t, db.Ping())
		assert.Equal(t, 2, snapshot(c.window).Attempts)
	})
}

func TestDSNConnector(t *testing.T) {
	s := newScript()
	d := fakeDriver{s: s}
	c := DSNConnector(d, "file:test.db")
	assert.Equal(t, d, c.Driver())

	w, clk := newTestWindow()
	db := openDB(t, c, w)
	require.NoError(t, db.Ping())
	assert.Equal(t, 1, s.called("Open"))

	s.fail("Open", errors.New("unable to open database file"))
	_, err := db.Conn(context.Background())
	require.NoError(t, err, "the idle connection is reused")
	require.NoError(t, db.Close())
	db = openDB(t, c, w)
	require.Error(t, db.Ping())
	assert.Equal(t, 1, w.Snapshot(clk.Now()).Failures)
}

// The context-free and fallback paths below are not reachable through database/sql
// with the fakes above, so they are exercised on the wrapper directly.

func TestConnContextFreeMethods(t *testing.T) {
	s := newScript()
	w, clk := newTestWindow()
	c := &conn{conn: &fullConn{baseConn: baseConn{s: s}}, window: w}

	st, err := c.Prepare("SELECT 1")
	require.NoError(t, err)
	_, err = st.Exec(nil)
	require.NoError(t, err)
	_, err = st.(*stmt).Query(nil)
	require.NoError(t, err)
	assert.Equal(t, -1, st.NumInput())
	require.NoError(t, st.Close())

	t1, err := c.Begin()
	require.NoError(t, err)
	require.NoError(t, t1.Commit())
	require.NoError(t, c.Close())

	s.fail("Prepare", io.EOF)
	_, err = c.Prepare("SELECT 1")
	require.ErrorIs(t, err, io.EOF)
	s.fail("Begin", io.EOF)
	_, err = c.Begin()
	require.ErrorIs(t, err, io.EOF)

	snap := w.Snapshot(clk.Now())
	assert.Equal(t, 7, snap.Attempts)
	assert.Equal(t, 2, snap.Failures)
	assert.Equal(t, []string{"Prepare", "StmtExec", "StmtQuery", "StmtClose", "Begin", "Commit", "Close", "Prepare", "Begin"}, s.calls)
}

func TestConnFallbacksHonourCancelledContext(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("prepare", func(t *testing.T) {
		s := newScript()
		c := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
		_, err := c.PrepareContext(cancelled, "SELECT 1")
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, []string{"Prepare", "StmtClose"}, s.calls)
	})
	t.Run("prepare failure", func(t *testing.T) {
		s := newScript()
		s.fail("Prepare", io.EOF)
		c := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
		_, err := c.PrepareContext(context.Background(), "SELECT 1")
		require.ErrorIs(t, err, io.EOF)
	})
	t.Run("begin", func(t *testing.T) {
		s := newScript()
		c := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
		_, err := c.BeginTx(cancelled, driver.TxOptions{})
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, []string{"Begin", "Rollback"}, s.calls)
	})
	t.Run("begin failure", func(t *testing.T) {
		s := newScript()
		s.fail("Begin", io.EOF)
		c := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
		_, err := c.BeginTx(context.Background(), driver.TxOptions{})
		require.ErrorIs(t, err, io.EOF)
	})
	t.Run("legacy exec and query", func(t *testing.T) {
		s := newScript()
		c := &conn{conn: &legacyConn{baseConn: baseConn{s: s}}, window: NewWindow(time.Second, nil)}
		_, err := c.ExecContext(cancelled, "UPDATE t SET v = 1", nil)
		require.ErrorIs(t, err, context.Canceled)
		_, err = c.QueryContext(cancelled, "SELECT 1", nil)
		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, s.calls)
	})
	t.Run("statement exec and query", func(t *testing.T) {
		s := newScript()
		c := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
		st := &stmt{stmt: &baseStmt{s: s}, conn: c, ctx: context.Background()}
		_, err := st.ExecContext(cancelled, nil)
		require.ErrorIs(t, err, context.Canceled)
		_, err = st.QueryContext(cancelled, nil)
		require.ErrorIs(t, err, context.Canceled)
		named := []driver.NamedValue{{Name: "v", Ordinal: 1, Value: int64(1)}}
		_, err = st.ExecContext(context.Background(), named)
		assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
		_, err = st.QueryContext(context.Background(), named)
		assert.EqualError(t, err, "sql: driver does not support the use of Named Parameters")
		assert.Empty(t, s.calls)
	})
}

func TestMinimalConnOptionalMethods(t *testing.T) {
	s := newScript()
	c := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
	ctx := context.Background()

	_, err := c.ExecContext(ctx, "UPDATE t SET v = 1", nil)
	assert.Same(t, driver.ErrSkip, err)
	_, err = c.QueryContext(ctx, "SELECT 1", nil)
	assert.Same(t, driver.ErrSkip, err)
	assert.Same(t, driver.ErrSkip, c.CheckNamedValue(&driver.NamedValue{}))
	assert.NoError(t, c.Ping(ctx))
	assert.Empty(t, s.calls)
	assert.Equal(t, 1, snapshot(c.window).Attempts, "only the ping is an attempt")
}

func TestStatementArgumentCheckerOrder(t *testing.T) {
	s := newScript()
	full := &conn{conn: &fullConn{baseConn: baseConn{s: s}}, window: NewWindow(time.Second, nil)}

	own := &stmt{stmt: &fullStmt{baseStmt: baseStmt{s: s}}, conn: full}
	require.NoError(t, own.CheckNamedValue(&driver.NamedValue{}))
	assert.Equal(t, []string{"StmtCheckNamedValue"}, s.calls)

	s.reset()
	plain := &stmt{stmt: &baseStmt{s: s}, conn: full}
	require.NoError(t, plain.CheckNamedValue(&driver.NamedValue{}))
	assert.Equal(t, []string{"CheckNamedValue"}, s.calls, "the connection's checker applies when the statement has none")

	s.reset()
	minimal := &conn{conn: &baseConn{s: s}, window: NewWindow(time.Second, nil)}
	plain = &stmt{stmt: &baseStmt{s: s}, conn: minimal}
	assert.Same(t, driver.ErrSkip, plain.CheckNamedValue(&driver.NamedValue{}))
}

func TestConnectionCheckerAcceptsDriverSpecificArguments(t *testing.T) {
	type custom struct{ v int }
	s := newScript()
	db := openDB(t, &fakeConnector{s: s, plainStmt: true}, nil)

	_, err := db.Exec("UPDATE t SET v = ?", custom{v: 1})
	require.NoError(t, err, "the driver's own checker accepts what database/sql cannot convert")
	assert.Equal(t, 1, s.called("CheckNamedValue"))
}

// timeoutError is a network error that reports a timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// retryableError is a pgx error raised before anything was sent to the server.
type retryableError struct{}

func (retryableError) Error() string     { return "dial failed" }
func (retryableError) SafeToRetry() bool { return true }

func TestIsConnectionFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
		kind FailureKind
	}{
		{name: "nil", err: nil},
		{name: "caller cancelled", err: context.Canceled},
		{name: "wrapped caller cancelled", err: fmt.Errorf("query: %w", context.Canceled)},
		{name: "query deadline", err: context.DeadlineExceeded},
		{name: "no rows", err: sql.ErrNoRows},
		{name: "plain error", err: errors.New("boom")},
		{name: "bad connection", err: driver.ErrBadConn, want: true, kind: Unreachable},
		{name: "wrapped bad connection", err: fmt.Errorf("exec: %w", driver.ErrBadConn), want: true, kind: Unreachable},
		{name: "eof", err: io.EOF, want: true, kind: Unreachable},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF, want: true, kind: Unreachable},
		{name: "connection refused", err: syscall.ECONNREFUSED, want: true, kind: Unreachable},
		{name: "connection reset", err: syscall.ECONNRESET, want: true, kind: Unreachable},
		{name: "broken pipe", err: syscall.EPIPE, want: true, kind: Unreachable},
		{name: "network error", err: &net.OpError{Op: "read", Err: errors.New("closed")}, want: true, kind: Unreachable},
		{name: "network timeout", err: &net.OpError{Op: "read", Err: timeoutError{}}, want: true, kind: Timeout},
		{name: "dns failure", err: &net.DNSError{Err: "no such host", Name: "db"}, want: true, kind: Unreachable},
		{name: "postgres connect error", err: &pgconn.ConnectError{Config: &pgconn.Config{}}, want: true, kind: Unreachable},
		{name: "postgres safe to retry", err: retryableError{}, want: true, kind: Unreachable},
		{name: "postgres connection exception", err: &pgconn.PgError{Code: "08006"}, want: true, kind: Unreachable},
		{name: "postgres authentication failure", err: &pgconn.PgError{Code: "28P01"}, want: true, kind: Unreachable},
		{name: "postgres admin shutdown", err: &pgconn.PgError{Code: "57P01"}, want: true, kind: Unreachable},
		{name: "postgres crash shutdown", err: &pgconn.PgError{Code: "57P02"}, want: true, kind: Unreachable},
		{name: "postgres cannot connect now", err: &pgconn.PgError{Code: "57P03"}, want: true, kind: Unreachable},
		{name: "postgres query cancelled", err: &pgconn.PgError{Code: "57014"}},
		{name: "postgres unique violation", err: &pgconn.PgError{Code: "23505"}},
		{name: "postgres serialization failure", err: &pgconn.PgError{Code: "40001"}},
		{name: "sql server login failed", err: mssql.Error{Number: 18456}, want: true, kind: Unreachable},
		{name: "sql server shutting down", err: mssql.Error{Number: 6005}, want: true, kind: Unreachable},
		{name: "sql server shut down by request", err: mssql.Error{Number: 6006}, want: true, kind: Unreachable},
		{name: "sql server unique violation", err: mssql.Error{Number: 2627}},
		{name: "sqlite cannot open", err: sqlite3.Error{Code: sqlite3.ErrCantOpen}, want: true, kind: Unreachable},
		{name: "sqlite io error", err: sqlite3.Error{Code: sqlite3.ErrIoErr, ExtendedCode: sqlite3.ErrIoErrRead}, want: true, kind: Unreachable},
		{name: "sqlite not a database", err: sqlite3.Error{Code: sqlite3.ErrNotADB}, want: true, kind: Unreachable},
		{name: "sqlite corrupt", err: sqlite3.Error{Code: sqlite3.ErrCorrupt}, want: true, kind: Unreachable},
		{name: "sqlite busy", err: sqlite3.Error{Code: sqlite3.ErrBusy}},
		{name: "sqlite constraint", err: sqlite3.Error{Code: sqlite3.ErrConstraint}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, kind := IsConnectionFailure(tt.err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.kind, kind)
		})
	}
}

func TestConnectFailureKinds(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
		kind FailureKind
	}{
		{name: "nil", err: nil},
		{name: "caller cancelled", err: context.Canceled},
		{name: "deadline", err: fmt.Errorf("connect: %w", context.DeadlineExceeded), want: true, kind: Timeout},
		{name: "network timeout", err: &net.OpError{Op: "dial", Err: timeoutError{}}, want: true, kind: Timeout},
		{name: "authentication", err: &pgconn.PgError{Code: "28P01"}, want: true, kind: Unreachable},
		{name: "any error", err: errors.New("x509: certificate signed by unknown authority"), want: true, kind: Unreachable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, kind := connectFailure(tt.err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.kind, kind)
		})
	}
}

func TestWindowSlidesAndExpires(t *testing.T) {
	clk := newFakeClock()
	w := NewWindow(5*time.Second, clk.Now)
	ctx := context.Background()

	w.Record(ctx, nil)
	w.Record(ctx, driver.ErrBadConn)
	clk.Advance(2 * time.Second)
	w.Record(ctx, nil)
	snap := w.Snapshot(clk.Now())
	assert.Equal(t, 3, snap.Attempts)
	assert.Equal(t, 1, snap.Failures)

	clk.Advance(3 * time.Second)
	snap = w.Snapshot(clk.Now())
	assert.Equal(t, 1, snap.Attempts, "outcomes older than the window are dropped")
	assert.Zero(t, snap.Failures)
	assert.Equal(t, Unreachable, snap.LastFailure, "the last failure kind outlives the window")

	clk.Advance(5 * time.Second)
	w.Record(ctx, context.DeadlineExceeded)
	snap = w.Snapshot(clk.Now())
	assert.Equal(t, 1, snap.Attempts, "a reused bucket starts empty")
	assert.Zero(t, snap.Failures)
	assert.Equal(t, clk.Now(), snap.LastActivity)

	assert.Zero(t, w.Snapshot(clk.Now().Add(-time.Minute)).Attempts, "buckets newer than the snapshot time are ignored")
}

func TestWindowLengthRoundsUpToWholeSeconds(t *testing.T) {
	assert.Len(t, NewWindow(1500*time.Millisecond, nil).buckets, 2)
	assert.Len(t, NewWindow(0, nil).buckets, 1)
	assert.Len(t, NewWindow(-time.Second, nil).buckets, 1)
	assert.NotNil(t, NewWindow(time.Second, nil).now)
}

func TestNilWindowIgnoresEverything(t *testing.T) {
	var w *Window
	assert.NotPanics(t, func() {
		w.Record(context.Background(), driver.ErrBadConn)
		w.recordConnect(context.Background(), errors.New("refused"))
	})
	assert.Equal(t, Snapshot{}, w.Snapshot(time.Now()))
}

func TestWindowConcurrentUse(t *testing.T) {
	w := NewWindow(30*time.Second, nil)
	pingCtx := WithHealthPing(context.Background())
	const workers, perWorker = 8, 500
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				switch j % 4 {
				case 0:
					w.Record(context.Background(), driver.ErrBadConn)
				case 1:
					w.Record(pingCtx, nil)
				case 2:
					w.recordConnect(context.Background(), errors.New("refused"))
				default:
					_ = w.Snapshot(time.Now())
				}
			}
		}(i)
	}
	wg.Wait()
	snap := w.Snapshot(time.Now())
	assert.Equal(t, workers*perWorker*3/4, snap.Attempts)
	assert.Equal(t, workers*perWorker/2, snap.Failures)
}

func TestConcurrentDatabaseUse(t *testing.T) {
	w := NewWindow(30*time.Second, nil)
	db := openDB(t, &fakeConnector{s: newScript()}, w)
	db.SetMaxOpenConns(4)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, err := db.Exec("UPDATE t SET v = 1")
				assert.NoError(t, err)
			}
		}()
	}
	wg.Wait()
	assert.GreaterOrEqual(t, w.Snapshot(time.Now()).Attempts, 800)
}
