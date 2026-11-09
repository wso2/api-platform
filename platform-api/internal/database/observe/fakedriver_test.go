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
	"io"
	"sync"
)

// fakeDB is the state behind the fake driver: the error each method returns and how
// often each method was called.
type fakeDB struct {
	mu    sync.Mutex
	errs  map[string]error
	calls map[string]int
	// failOnce makes a method's error apply to its next call only.
	failOnce map[string]bool
	valid    bool
}

func newFakeDB() *fakeDB {
	return &fakeDB{errs: map[string]error{}, calls: map[string]int{}, failOnce: map[string]bool{}, valid: true}
}

func (f *fakeDB) fail(method string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs[method] = err
}

func (f *fakeDB) failNext(method string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs[method] = err
	f.failOnce[method] = true
}

func (f *fakeDB) call(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[method]++
	err := f.errs[method]
	if f.failOnce[method] {
		delete(f.errs, method)
		delete(f.failOnce, method)
	}
	return err
}

func (f *fakeDB) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

// fakeDriver is the driver a fake connector reports.
type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, driver.ErrSkip }

// fakeConnector opens connections built by newConn.
type fakeConnector struct {
	db      *fakeDB
	newConn func(*fakeDB) driver.Conn
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) {
	if err := c.db.call("Connect"); err != nil {
		return nil, err
	}
	return c.newConn(c.db), nil
}

func (c *fakeConnector) Driver() driver.Driver { return fakeDriver{} }

// closingConnector also implements io.Closer.
type closingConnector struct{ fakeConnector }

func (c *closingConnector) Close() error { return c.db.call("ConnectorClose") }

// basicConn implements only driver.Conn.
type basicConn struct{ db *fakeDB }

func (c *basicConn) Prepare(string) (driver.Stmt, error) {
	if err := c.db.call("Prepare"); err != nil {
		return nil, err
	}
	return &basicStmt{db: c.db}, nil
}

func (c *basicConn) Close() error { return c.db.call("Close") }

func (c *basicConn) Begin() (driver.Tx, error) {
	if err := c.db.call("Begin"); err != nil {
		return nil, err
	}
	return &fakeTx{db: c.db}, nil
}

// legacyConn adds the context-free Execer and Queryer.
type legacyConn struct{ basicConn }

func (c *legacyConn) Exec(string, []driver.Value) (driver.Result, error) {
	if err := c.db.call("ConnExec"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *legacyConn) Query(string, []driver.Value) (driver.Rows, error) {
	if err := c.db.call("ConnQuery"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

// fullConn implements every optional connection interface.
type fullConn struct{ basicConn }

func (c *fullConn) Ping(context.Context) error { return c.db.call("Ping") }

func (c *fullConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	if err := c.db.call("ExecContext"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *fullConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if err := c.db.call("QueryContext"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

func (c *fullConn) PrepareContext(context.Context, string) (driver.Stmt, error) {
	if err := c.db.call("PrepareContext"); err != nil {
		return nil, err
	}
	return &fullStmt{basicStmt{db: c.db}}, nil
}

func (c *fullConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if err := c.db.call("BeginTx"); err != nil {
		return nil, err
	}
	return &fakeTx{db: c.db}, nil
}

func (c *fullConn) CheckNamedValue(*driver.NamedValue) error {
	if err := c.db.call("ConnCheckNamedValue"); err != nil {
		return err
	}
	return driver.ErrSkip
}

func (c *fullConn) ResetSession(context.Context) error { return c.db.call("ResetSession") }

func (c *fullConn) IsValid() bool {
	c.db.call("IsValid")
	c.db.mu.Lock()
	defer c.db.mu.Unlock()
	return c.db.valid
}

// resettingOnlyConn implements driver.SessionResetter but not driver.Validator.
type resettingOnlyConn struct{ basicConn }

func (c *resettingOnlyConn) ResetSession(context.Context) error { return c.db.call("ResetSession") }

// validatingOnlyConn implements driver.Validator but not driver.SessionResetter.
type validatingOnlyConn struct{ basicConn }

func (c *validatingOnlyConn) IsValid() bool { return c.db.call("IsValid") == nil }

// basicStmt implements only driver.Stmt.
type basicStmt struct{ db *fakeDB }

func (s *basicStmt) Close() error  { return s.db.call("StmtClose") }
func (s *basicStmt) NumInput() int { return -1 }

func (s *basicStmt) Exec([]driver.Value) (driver.Result, error) {
	if err := s.db.call("StmtExec"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (s *basicStmt) Query([]driver.Value) (driver.Rows, error) {
	if err := s.db.call("StmtQuery"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

// fullStmt adds the context-aware statement interfaces and its own argument checker.
type fullStmt struct{ basicStmt }

func (s *fullStmt) ExecContext(context.Context, []driver.NamedValue) (driver.Result, error) {
	if err := s.db.call("StmtExecContext"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (s *fullStmt) QueryContext(context.Context, []driver.NamedValue) (driver.Rows, error) {
	if err := s.db.call("StmtQueryContext"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

func (s *fullStmt) CheckNamedValue(*driver.NamedValue) error {
	if err := s.db.call("StmtCheckNamedValue"); err != nil {
		return err
	}
	return driver.ErrSkip
}

// columnConvertingStmt adds a column converter.
type columnConvertingStmt struct{ basicStmt }

func (s *columnConvertingStmt) ColumnConverter(int) driver.ValueConverter {
	s.db.call("ColumnConverter")
	return driver.DefaultParameterConverter
}

type fakeTx struct{ db *fakeDB }

func (t *fakeTx) Commit() error   { return t.db.call("Commit") }
func (t *fakeTx) Rollback() error { return t.db.call("Rollback") }

// fakeRows returns a single row holding 1.
type fakeRows struct{ done bool }

func (r *fakeRows) Columns() []string { return []string{"n"} }
func (r *fakeRows) Close() error      { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(1)
	return nil
}
