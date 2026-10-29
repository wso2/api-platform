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
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	sqlite3 "github.com/mattn/go-sqlite3"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// timeoutError is a net.Error that timed out.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// safeToRetryError is an error pgx raises before anything reached the server.
type safeToRetryError struct{}

func (safeToRetryError) Error() string     { return "conn closed" }
func (safeToRetryError) SafeToRetry() bool { return true }

func TestIsConnectionFailure(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	tests := []struct {
		name     string
		err      error
		counted  bool
		wantKind FailureKind
	}{
		{"success", nil, false, FailureNone},
		{"caller canceled", context.Canceled, false, FailureNone},
		{"wrapped cancel", fmt.Errorf("query: %w", context.Canceled), false, FailureNone},
		{"query over its own deadline", context.DeadlineExceeded, false, FailureNone},
		{"no rows", sql.ErrNoRows, false, FailureNone},
		{"plain error", errors.New("syntax error at or near SELECT"), false, FailureNone},
		{"bad connection", driver.ErrBadConn, true, FailureUnreachable},
		{"wrapped bad connection", fmt.Errorf("exec: %w", driver.ErrBadConn), true, FailureUnreachable},
		{"eof", io.EOF, true, FailureUnreachable},
		{"unexpected eof", io.ErrUnexpectedEOF, true, FailureUnreachable},
		{"refused", syscall.ECONNREFUSED, true, FailureUnreachable},
		{"reset", syscall.ECONNRESET, true, FailureUnreachable},
		{"broken pipe", syscall.EPIPE, true, FailureUnreachable},
		{"dial refused", refused, true, FailureUnreachable},
		{"network timeout", timeoutError{}, true, FailureTimeout},
		{"wrapped network timeout", fmt.Errorf("read: %w", &net.OpError{Op: "read", Err: timeoutError{}}), true, FailureTimeout},
		{"pg safe to retry", safeToRetryError{}, true, FailureUnreachable},
		{"pg connection exception", &pgconn.PgError{Code: "08006"}, true, FailureUnreachable},
		{"pg connection does not exist", &pgconn.PgError{Code: "08003"}, true, FailureUnreachable},
		{"pg authentication", &pgconn.PgError{Code: "28P01"}, true, FailureUnreachable},
		{"pg admin shutdown", &pgconn.PgError{Code: "57P01"}, true, FailureUnreachable},
		{"pg crash shutdown", &pgconn.PgError{Code: "57P02"}, true, FailureUnreachable},
		{"pg cannot connect now", &pgconn.PgError{Code: "57P03"}, true, FailureUnreachable},
		{"pg query canceled", &pgconn.PgError{Code: "57014"}, false, FailureNone},
		{"pg unique violation", &pgconn.PgError{Code: "23505"}, false, FailureNone},
		{"pg serialization failure", &pgconn.PgError{Code: "40001"}, false, FailureNone},
		{"mssql login failed", mssql.Error{Number: 18456}, true, FailureUnreachable},
		{"mssql shutting down", mssql.Error{Number: 6005}, true, FailureUnreachable},
		{"mssql shut down by request", mssql.Error{Number: 6006}, true, FailureUnreachable},
		{"mssql unique violation", mssql.Error{Number: 2627}, false, FailureNone},
		{"sqlite cannot open", sqlite3.Error{Code: sqlite3.ErrCantOpen}, true, FailureUnreachable},
		{"sqlite io error", sqlite3.Error{Code: sqlite3.ErrIoErr}, true, FailureUnreachable},
		{"sqlite not a database", sqlite3.Error{Code: sqlite3.ErrNotADB}, true, FailureUnreachable},
		{"sqlite corrupt", sqlite3.Error{Code: sqlite3.ErrCorrupt}, true, FailureUnreachable},
		{"sqlite constraint", sqlite3.Error{Code: sqlite3.ErrConstraint}, false, FailureNone},
		{"sqlite busy", sqlite3.Error{Code: sqlite3.ErrBusy}, false, FailureNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counted, kind := IsConnectionFailure(tt.err)
			assert.Equal(t, tt.counted, counted)
			assert.Equal(t, tt.wantKind, kind)
		})
	}
}

// A real pgx connect failure is counted.
func TestIsConnectionFailurePostgresConnectError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = pgconn.Connect(ctx, "postgres://user@"+addr+"/db?sslmode=disable")
	require.Error(t, err)
	var connectErr *pgconn.ConnectError
	require.ErrorAs(t, err, &connectErr)

	counted, kind := IsConnectionFailure(err)
	assert.True(t, counted)
	assert.Equal(t, FailureUnreachable, kind)
}

func TestFailureKind(t *testing.T) {
	assert.Equal(t, FailureTimeout, failureKind(context.DeadlineExceeded))
	assert.Equal(t, FailureTimeout, failureKind(fmt.Errorf("dial: %w", os.ErrDeadlineExceeded)))
	assert.Equal(t, FailureTimeout, failureKind(timeoutError{}))
	assert.Equal(t, FailureUnreachable, failureKind(errRefused))
	assert.Equal(t, FailureUnreachable, failureKind(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}))
}
