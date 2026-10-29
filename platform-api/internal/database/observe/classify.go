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
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgconn"
	sqlite3 "github.com/mattn/go-sqlite3"
	mssql "github.com/microsoft/go-mssqldb"
)

// SQL Server error numbers that mean the server cannot serve the platform at all.
const (
	mssqlLoginFailed       = 18456
	mssqlShutdownInProcess = 6005
	mssqlShutdownByRequest = 6006
)

// IsConnectionFailure reports whether err, returned by a database operation, means the
// database could not be reached or used, and if so of which kind. Only such failures
// count against database health. Errors the database itself answered with (syntax,
// constraint and serialization errors, no rows) are not connection failures, nor is a
// caller giving up, nor an operation exceeding its caller's own deadline: a slow query
// is not an outage.
func IsConnectionFailure(err error) (bool, FailureKind) {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, FailureNone
	}
	if isNetworkFailure(err) || isPostgresConnectionFailure(err) ||
		isSQLServerConnectionFailure(err) || isSQLiteConnectionFailure(err) {
		return true, failureKind(err)
	}
	return false, FailureNone
}

// isNetworkFailure reports a broken or refused connection, whichever driver saw it.
func isNetworkFailure(err error) bool {
	var netErr net.Error
	return errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.As(err, &netErr)
}

// isPostgresConnectionFailure reports a failed connect, an error raised before anything
// reached the server, or a server error in the connection-exception (08) or
// authentication (28) class, or one saying the server is shutting down or not yet
// accepting connections.
func isPostgresConnectionFailure(err error) bool {
	var connectErr *pgconn.ConnectError
	if errors.As(err, &connectErr) || pgconn.SafeToRetry(err) {
		return true
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "57P01", "57P02", "57P03":
		return true
	}
	return strings.HasPrefix(pgErr.Code, "08") || strings.HasPrefix(pgErr.Code, "28")
}

// isSQLServerConnectionFailure reports a failed login or a server that is shutting down.
func isSQLServerConnectionFailure(err error) bool {
	var msErr mssql.Error
	if !errors.As(err, &msErr) {
		return false
	}
	switch msErr.SQLErrorNumber() {
	case mssqlLoginFailed, mssqlShutdownInProcess, mssqlShutdownByRequest:
		return true
	}
	return false
}

// isSQLiteConnectionFailure reports a database file that cannot be opened, read or
// written, or is not a usable database.
func isSQLiteConnectionFailure(err error) bool {
	var liteErr sqlite3.Error
	if !errors.As(err, &liteErr) {
		return false
	}
	switch liteErr.Code {
	case sqlite3.ErrCantOpen, sqlite3.ErrIoErr, sqlite3.ErrNotADB, sqlite3.ErrCorrupt:
		return true
	}
	return false
}

// failureKind tells a timeout apart from any other connection failure.
func failureKind(err error) FailureKind {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) ||
		pgconn.Timeout(err) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return FailureTimeout
	}
	return FailureUnreachable
}
