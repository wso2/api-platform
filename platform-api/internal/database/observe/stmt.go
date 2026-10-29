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
	"database/sql/driver"
)

// stmt records the outcome of every execution of a prepared statement.
type stmt struct {
	base driver.Stmt
	conn *conn
}

// convertingStmt exposes the driver statement's column converter, which database/sql
// consults only when the statement implements driver.ColumnConverter.
type convertingStmt struct{ *stmt }

func (s convertingStmt) ColumnConverter(idx int) driver.ValueConverter {
	return s.base.(driver.ColumnConverter).ColumnConverter(idx)
}

func wrapStmt(base driver.Stmt, c *conn) driver.Stmt {
	s := &stmt{base: base, conn: c}
	if _, ok := base.(driver.ColumnConverter); ok {
		return convertingStmt{s}
	}
	return s
}

func (s *stmt) Close() error {
	return s.base.Close()
}

func (s *stmt) NumInput() int {
	return s.base.NumInput()
}

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	res, err := s.base.Exec(args)
	s.conn.window.Record(context.Background(), err)
	return res, err
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := s.base.Query(args)
	s.conn.window.Record(context.Background(), err)
	return rows, err
}

func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	var res driver.Result
	var err error
	if e, ok := s.base.(driver.StmtExecContext); ok {
		res, err = e.ExecContext(ctx, args)
	} else {
		var values []driver.Value
		if values, err = namedValueToValue(args); err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		res, err = s.base.Exec(values)
	}
	s.conn.window.Record(ctx, err)
	return res, err
}

func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	var rows driver.Rows
	var err error
	if q, ok := s.base.(driver.StmtQueryContext); ok {
		rows, err = q.QueryContext(ctx, args)
	} else {
		var values []driver.Value
		if values, err = namedValueToValue(args); err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		rows, err = s.base.Query(values)
	}
	s.conn.window.Record(ctx, err)
	return rows, err
}

// CheckNamedValue uses the statement's own argument checker, or else the connection's,
// which is the order database/sql consults them in.
func (s *stmt) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := s.base.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return s.conn.CheckNamedValue(nv)
}
