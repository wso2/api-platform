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
)

// Errors database/sql itself returns when a driver without transaction options support
// is asked for options it cannot honor.
var (
	errIsolationUnsupported = errors.New("sql: driver does not support non-default isolation level")
	errReadOnlyUnsupported  = errors.New("sql: driver does not support read-only transactions")
)

// conn records the outcome of every operation on a driver connection. It implements
// each optional context-aware interface, falling back the way database/sql does when
// the driver lacks it.
type conn struct {
	base   driver.Conn
	window *Window
}

// database/sql keeps a connection after a rolled-back transaction only when it
// implements both driver.SessionResetter and driver.Validator, so the wrapper exposes
// those two exactly when the driver does, through these variants.
type (
	resettingConn           struct{ *conn }
	validatingConn          struct{ *conn }
	resettingValidatingConn struct{ *conn }
)

func (c resettingConn) ResetSession(ctx context.Context) error { return c.resetSession(ctx) }

func (c validatingConn) IsValid() bool { return c.isValid() }

func (c resettingValidatingConn) ResetSession(ctx context.Context) error { return c.resetSession(ctx) }

func (c resettingValidatingConn) IsValid() bool { return c.isValid() }

func wrapConn(base driver.Conn, w *Window) driver.Conn {
	c := &conn{base: base, window: w}
	_, resets := base.(driver.SessionResetter)
	_, validates := base.(driver.Validator)
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

// resetSession resets the session before the pool hands the connection out again.
// Drivers use it to detect a connection that broke while idle.
func (c *conn) resetSession(ctx context.Context) error {
	err := c.base.(driver.SessionResetter).ResetSession(ctx)
	c.window.Record(ctx, err)
	return err
}

// isValid reports whether the connection may go back into the pool. It is not a round
// trip to the database, and a driver marks a connection invalid after the failed
// operation that was already recorded, so it is not recorded again.
func (c *conn) isValid() bool {
	return c.base.(driver.Validator).IsValid()
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	s, err := c.base.Prepare(query)
	c.window.Record(context.Background(), err)
	if err != nil {
		return nil, err
	}
	return wrapStmt(s, c), nil
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	s, err := c.prepareContext(ctx, query)
	c.window.Record(ctx, err)
	if err != nil {
		return nil, err
	}
	return wrapStmt(s, c), nil
}

func (c *conn) prepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.base.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, query)
	}
	s, err := c.base.Prepare(query)
	if err == nil {
		select {
		default:
		case <-ctx.Done():
			_ = s.Close()
			return nil, ctx.Err()
		}
	}
	return s, err
}

func (c *conn) Close() error {
	return c.base.Close()
}

// Begin starts a transaction without a context. database/sql always uses BeginTx.
func (c *conn) Begin() (driver.Tx, error) {
	t, err := c.base.Begin()
	return c.startedTx(context.Background(), t, err)
}

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.base.(driver.ConnBeginTx); ok {
		t, err := b.BeginTx(ctx, opts)
		return c.startedTx(ctx, t, err)
	}
	if opts.Isolation != driver.IsolationLevel(sql.LevelDefault) {
		return nil, errIsolationUnsupported
	}
	if opts.ReadOnly {
		return nil, errReadOnlyUnsupported
	}
	t, err := c.base.Begin()
	if err == nil {
		select {
		default:
		case <-ctx.Done():
			_ = t.Rollback()
			err = ctx.Err()
		}
	}
	return c.startedTx(ctx, t, err)
}

func (c *conn) startedTx(ctx context.Context, t driver.Tx, err error) (driver.Tx, error) {
	c.window.Record(ctx, err)
	if err != nil {
		return nil, err
	}
	return &tx{base: t, window: c.window}, nil
}

// Ping checks the connection. A failed ping is always a connection failure.
func (c *conn) Ping(ctx context.Context) error {
	p, ok := c.base.(driver.Pinger)
	if !ok {
		return nil
	}
	err := p.Ping(ctx)
	c.window.recordConnection(ctx, err)
	return err
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	var res driver.Result
	var err error
	switch e := c.base.(type) {
	case driver.ExecerContext:
		res, err = e.ExecContext(ctx, query, args)
	case driver.Execer:
		var values []driver.Value
		if values, err = namedValueToValue(args); err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		res, err = e.Exec(query, values)
	default:
		return nil, driver.ErrSkip
	}
	if !errors.Is(err, driver.ErrSkip) {
		c.window.Record(ctx, err)
	}
	return res, err
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var rows driver.Rows
	var err error
	switch q := c.base.(type) {
	case driver.QueryerContext:
		rows, err = q.QueryContext(ctx, query, args)
	case driver.Queryer:
		var values []driver.Value
		if values, err = namedValueToValue(args); err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		rows, err = q.Query(query, values)
	default:
		return nil, driver.ErrSkip
	}
	if !errors.Is(err, driver.ErrSkip) {
		c.window.Record(ctx, err)
	}
	return rows, err
}

// CheckNamedValue lets the driver convert arguments its own way; ErrSkip hands the
// argument to database/sql's default conversion.
func (c *conn) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.base.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}
