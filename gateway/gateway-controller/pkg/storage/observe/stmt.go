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
	"database/sql/driver"
)

// stmt wraps a prepared statement. Drivers without direct exec and query support, such
// as SQL Server, run every operation through a prepared statement, so its outcomes are
// recorded here.
type stmt struct {
	stmt driver.Stmt
	conn *conn
	// ctx is the context the statement was prepared under, used to attribute the
	// context-free Exec and Query calls.
	ctx context.Context
}

var (
	_ driver.Stmt              = (*stmt)(nil)
	_ driver.StmtExecContext   = (*stmt)(nil)
	_ driver.StmtQueryContext  = (*stmt)(nil)
	_ driver.NamedValueChecker = (*stmt)(nil)
)

func (s *stmt) Close() error {
	return s.stmt.Close()
}

func (s *stmt) NumInput() int {
	return s.stmt.NumInput()
}

// Exec is required by driver.Stmt; database/sql itself calls ExecContext.
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	r, err := s.stmt.Exec(args)
	s.conn.record(s.ctx, err)
	return r, err
}

// Query is required by driver.Stmt; database/sql itself calls QueryContext.
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	r, err := s.stmt.Query(args)
	s.conn.record(s.ctx, err)
	return r, err
}

func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := s.stmt.(driver.StmtExecContext); ok {
		r, err := ec.ExecContext(ctx, args)
		s.conn.record(ctx, err)
		return r, err
	}
	values, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	r, err := s.stmt.Exec(values)
	s.conn.record(ctx, err)
	return r, err
}

func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := s.stmt.(driver.StmtQueryContext); ok {
		r, err := qc.QueryContext(ctx, args)
		s.conn.record(ctx, err)
		return r, err
	}
	values, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	r, err := s.stmt.Query(values)
	s.conn.record(ctx, err)
	return r, err
}

// CheckNamedValue applies the statement's own argument checker, then the connection's,
// in the order database/sql consults them.
func (s *stmt) CheckNamedValue(nv *driver.NamedValue) error {
	if nvc, ok := s.stmt.(driver.NamedValueChecker); ok {
		return nvc.CheckNamedValue(nv)
	}
	return s.conn.CheckNamedValue(nv)
}
