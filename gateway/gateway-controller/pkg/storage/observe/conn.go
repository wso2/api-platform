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
	"sync/atomic"
)

// conn wraps a driver connection. It implements the optional connection interfaces
// database/sql looks for and, when the wrapped connection lacks one, behaves the way
// database/sql does without it. driver.SessionResetter and driver.Validator are the
// exception: see wrapConn.
type conn struct {
	conn   driver.Conn
	window *Window
	// abandoned is set when an operation ended because its caller gave up or ran out
	// of time. Drivers may close the connection in that case, so the broken-connection
	// error reported when it is next reused is a consequence of the caller, not of the
	// database, and is not counted.
	abandoned atomic.Bool
}

var (
	_ driver.Conn               = (*conn)(nil)
	_ driver.Pinger             = (*conn)(nil)
	_ driver.ExecerContext      = (*conn)(nil)
	_ driver.QueryerContext     = (*conn)(nil)
	_ driver.ConnPrepareContext = (*conn)(nil)
	_ driver.ConnBeginTx        = (*conn)(nil)
	_ driver.NamedValueChecker  = (*conn)(nil)
)

// database/sql keeps a connection after a rolled-back transaction only when the
// connection implements both driver.SessionResetter and driver.Validator, so the
// wrapper exposes those two exactly when the driver does, through these variants.
type (
	resettingConn           struct{ *conn }
	validatingConn          struct{ *conn }
	resettingValidatingConn struct{ *conn }
)

func (c resettingConn) ResetSession(ctx context.Context) error { return c.resetSession(ctx) }

func (c validatingConn) IsValid() bool { return c.isValid() }

func (c resettingValidatingConn) ResetSession(ctx context.Context) error {
	return c.resetSession(ctx)
}

func (c resettingValidatingConn) IsValid() bool { return c.isValid() }

// wrapConn wraps dc in the variant whose method set matches dc's.
func wrapConn(dc driver.Conn, w *Window) driver.Conn {
	c := &conn{conn: dc, window: w}
	_, resets := dc.(driver.SessionResetter)
	_, validates := dc.(driver.Validator)
	switch {
	case resets && validates:
		return resettingValidatingConn{c}
	case resets:
		return resettingConn{c}
	case validates:
		return validatingConn{c}
	}
	return c
}

// record counts the outcome of an operation. driver.ErrSkip is not an outcome: it asks
// database/sql to retry the operation another way, which is recorded in turn.
func (c *conn) record(ctx context.Context, err error) {
	if err == driver.ErrSkip {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		c.abandoned.Store(true)
	}
	c.window.Record(ctx, err)
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	s, err := c.conn.Prepare(query)
	c.record(context.Background(), err)
	if err != nil {
		return nil, err
	}
	return &stmt{stmt: s, conn: c, ctx: context.Background()}, nil
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var (
		s   driver.Stmt
		err error
	)
	if pc, ok := c.conn.(driver.ConnPrepareContext); ok {
		s, err = pc.PrepareContext(ctx, query)
	} else {
		s, err = c.conn.Prepare(query)
		if err == nil {
			select {
			case <-ctx.Done():
				_ = s.Close()
				s, err = nil, ctx.Err()
			default:
			}
		}
	}
	c.record(ctx, err)
	if err != nil {
		return nil, err
	}
	return &stmt{stmt: s, conn: c, ctx: ctx}, nil
}

func (c *conn) Close() error {
	return c.conn.Close()
}

// Begin is required by driver.Conn; database/sql itself calls BeginTx.
func (c *conn) Begin() (driver.Tx, error) {
	t, err := c.conn.Begin()
	c.record(context.Background(), err)
	if err != nil {
		return nil, err
	}
	return &tx{tx: t, conn: c, ctx: context.Background()}, nil
}

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	t, err := c.beginTx(ctx, opts)
	c.record(ctx, err)
	if err != nil {
		return nil, err
	}
	return &tx{tx: t, conn: c, ctx: ctx}, nil
}

// beginTx starts a transaction the way database/sql does: through BeginTx when the
// driver supports it, otherwise through Begin, refusing options Begin cannot honour.
func (c *conn) beginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := c.conn.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	if opts.Isolation != driver.IsolationLevel(sql.LevelDefault) {
		return nil, errors.New("sql: driver does not support non-default isolation level")
	}
	if opts.ReadOnly {
		return nil, errors.New("sql: driver does not support read-only transactions")
	}
	t, err := c.conn.Begin()
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		_ = t.Rollback()
		return nil, ctx.Err()
	default:
	}
	return t, nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := c.conn.(driver.ExecerContext); ok {
		r, err := ec.ExecContext(ctx, query, args)
		c.record(ctx, err)
		return r, err
	}
	// database/sql still honours the context-free interface.
	if e, ok := c.conn.(driver.Execer); ok {
		values, err := namedValueToValue(args)
		if err != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		r, err := e.Exec(query, values)
		c.record(ctx, err)
		return r, err
	}
	return nil, driver.ErrSkip
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := c.conn.(driver.QueryerContext); ok {
		r, err := qc.QueryContext(ctx, query, args)
		c.record(ctx, err)
		return r, err
	}
	// database/sql still honours the context-free interface.
	if q, ok := c.conn.(driver.Queryer); ok {
		values, err := namedValueToValue(args)
		if err != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		r, err := q.Query(query, values)
		c.record(ctx, err)
		return r, err
	}
	return nil, driver.ErrSkip
}

// Ping checks the connection. Every error other than the caller's cancellation counts
// as a connection failure.
func (c *conn) Ping(ctx context.Context) error {
	var err error
	if p, ok := c.conn.(driver.Pinger); ok {
		err = p.Ping(ctx)
	}
	c.window.recordConnect(ctx, err)
	return err
}

// resetSession prepares a pooled connection for reuse. A driver reports a connection it
// found broken with driver.ErrBadConn, which counts as a connection failure unless the
// connection was abandoned by its previous caller.
func (c *conn) resetSession(ctx context.Context) error {
	err := c.conn.(driver.SessionResetter).ResetSession(ctx)
	if c.abandoned.Swap(false) && errors.Is(err, driver.ErrBadConn) {
		return err
	}
	c.record(ctx, err)
	return err
}

// isValid reports whether the connection may return to the pool. It is not a round
// trip to the database, and a driver may mark a connection invalid after an operation
// merely ran past its own deadline, so it is not recorded: a real connection failure is
// recorded by the operation that hit it.
func (c *conn) isValid() bool {
	return c.conn.(driver.Validator).IsValid()
}

func (c *conn) CheckNamedValue(nv *driver.NamedValue) error {
	if nvc, ok := c.conn.(driver.NamedValueChecker); ok {
		return nvc.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}
