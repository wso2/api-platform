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
	"io"
	"sync"
	"time"
)

// script is shared by the fakes of one test: it records which driver methods were
// called and returns the error configured for each.
type script struct {
	mu      sync.Mutex
	errs    map[string]error
	calls   []string
	invalid bool
	closed  bool
}

func newScript() *script { return &script{errs: map[string]error{}} }

func (s *script) call(method string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, method)
	return s.errs[method]
}

func (s *script) fail(method string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs[method] = err
}

func (s *script) called(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if c == method {
			n++
		}
	}
	return n
}

func (s *script) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}

// connKind selects which optional interfaces a fake connection implements.
type connKind int

const (
	// fullKind implements every optional connection interface.
	fullKind connKind = iota
	// minimalKind implements only driver.Conn.
	minimalKind
	// legacyKind implements driver.Conn plus the context-free Execer and Queryer.
	legacyKind
	// resetterKind implements driver.Conn plus driver.SessionResetter, as pgx does.
	resetterKind
	// validatorKind implements driver.Conn plus driver.Validator.
	validatorKind
)

type fakeConnector struct {
	s    *script
	kind connKind
	// plainStmt makes prepared statements implement only driver.Stmt.
	plainStmt bool
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) {
	if err := c.s.call("Connect"); err != nil {
		return nil, err
	}
	base := baseConn{s: c.s, plainStmt: c.plainStmt}
	switch c.kind {
	case minimalKind:
		return &base, nil
	case legacyKind:
		return &legacyConn{baseConn: base}, nil
	case resetterKind:
		return &fakeResetterConn{baseConn: base}, nil
	case validatorKind:
		return &fakeValidatorConn{baseConn: base}, nil
	default:
		return &fullConn{baseConn: base}, nil
	}
}

func (c *fakeConnector) Driver() driver.Driver { return fakeDriver{s: c.s} }

// closingConnector is a connector that holds resources of its own.
type closingConnector struct{ fakeConnector }

func (c *closingConnector) Close() error { return c.s.call("ConnectorClose") }

type fakeDriver struct{ s *script }

func (d fakeDriver) Open(string) (driver.Conn, error) {
	if err := d.s.call("Open"); err != nil {
		return nil, err
	}
	return &fullConn{baseConn: baseConn{s: d.s}}, nil
}

type baseConn struct {
	s         *script
	plainStmt bool
}

func (c *baseConn) newStmt() driver.Stmt {
	if c.plainStmt {
		return &baseStmt{s: c.s}
	}
	return &fullStmt{baseStmt: baseStmt{s: c.s}}
}

func (c *baseConn) Prepare(string) (driver.Stmt, error) {
	if err := c.s.call("Prepare"); err != nil {
		return nil, err
	}
	return c.newStmt(), nil
}

func (c *baseConn) Close() error { return c.s.call("Close") }

func (c *baseConn) Begin() (driver.Tx, error) {
	if err := c.s.call("Begin"); err != nil {
		return nil, err
	}
	return &fakeTx{s: c.s}, nil
}

type fullConn struct{ baseConn }

func (c *fullConn) Ping(context.Context) error { return c.s.call("Ping") }

func (c *fullConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	if err := c.s.call("ExecContext"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *fullConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if err := c.s.call("QueryContext"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

func (c *fullConn) PrepareContext(context.Context, string) (driver.Stmt, error) {
	if err := c.s.call("PrepareContext"); err != nil {
		return nil, err
	}
	return c.newStmt(), nil
}

func (c *fullConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if err := c.s.call("BeginTx"); err != nil {
		return nil, err
	}
	return &fakeTx{s: c.s}, nil
}

func (c *fullConn) ResetSession(context.Context) error { return c.s.call("ResetSession") }

func (c *fullConn) IsValid() bool {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.calls = append(c.s.calls, "IsValid")
	return !c.s.invalid
}

// CheckNamedValue accepts any argument, including types database/sql cannot convert.
func (c *fullConn) CheckNamedValue(*driver.NamedValue) error { return c.s.call("CheckNamedValue") }

type fakeResetterConn struct{ baseConn }

func (c *fakeResetterConn) ResetSession(context.Context) error { return c.s.call("ResetSession") }

type fakeValidatorConn struct{ baseConn }

func (c *fakeValidatorConn) IsValid() bool { return c.s.call("IsValid") == nil }

type legacyConn struct{ baseConn }

func (c *legacyConn) Exec(string, []driver.Value) (driver.Result, error) {
	if err := c.s.call("Exec"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *legacyConn) Query(string, []driver.Value) (driver.Rows, error) {
	if err := c.s.call("Query"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

type baseStmt struct{ s *script }

func (s *baseStmt) Close() error  { return s.s.call("StmtClose") }
func (s *baseStmt) NumInput() int { return -1 }

func (s *baseStmt) Exec([]driver.Value) (driver.Result, error) {
	if err := s.s.call("StmtExec"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (s *baseStmt) Query([]driver.Value) (driver.Rows, error) {
	if err := s.s.call("StmtQuery"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

type fullStmt struct{ baseStmt }

func (s *fullStmt) ExecContext(context.Context, []driver.NamedValue) (driver.Result, error) {
	if err := s.s.call("StmtExecContext"); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (s *fullStmt) QueryContext(context.Context, []driver.NamedValue) (driver.Rows, error) {
	if err := s.s.call("StmtQueryContext"); err != nil {
		return nil, err
	}
	return &fakeRows{}, nil
}

func (s *fullStmt) CheckNamedValue(*driver.NamedValue) error { return s.s.call("StmtCheckNamedValue") }

type fakeTx struct{ s *script }

func (t *fakeTx) Commit() error   { return t.s.call("Commit") }
func (t *fakeTx) Rollback() error { return t.s.call("Rollback") }

// fakeRows yields a single row holding 1.
type fakeRows struct{ done bool }

func (r *fakeRows) Columns() []string { return []string{"v"} }
func (r *fakeRows) Close() error      { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(1)
	return nil
}

// fakeClock is a manually advanced clock, safe for concurrent use.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}
