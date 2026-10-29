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
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	mssql "github.com/microsoft/go-mssqldb"
)

// SQL Server error numbers that mean the server cannot serve the connection.
const (
	mssqlLoginFailed      = 18456
	mssqlShutdownProgress = 6005
	mssqlShutdownRequest  = 6006
)

// IsConnectionFailure reports whether err means the database could not be reached or a
// connection to it broke, and of which kind. SQL errors, constraint violations, missing
// rows, a caller's cancellation and a query running past its own deadline are not
// connection failures: they say nothing about whether the database is available.
func IsConnectionFailure(err error) (bool, FailureKind) {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, NoFailure
	}
	if errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true, kindOf(err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true, kindOf(err)
	}

	var connectErr *pgconn.ConnectError
	if errors.As(err, &connectErr) || pgconn.SafeToRetry(err) {
		return true, kindOf(err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && isPostgresConnectionState(pgErr.Code) {
		return true, Unreachable
	}

	var msErr mssql.Error
	if errors.As(err, &msErr) {
		switch msErr.Number {
		case mssqlLoginFailed, mssqlShutdownProgress, mssqlShutdownRequest:
			return true, Unreachable
		}
	}

	var liteErr sqlite3.Error
	if errors.As(err, &liteErr) {
		switch liteErr.Code {
		case sqlite3.ErrCantOpen, sqlite3.ErrIoErr, sqlite3.ErrNotADB, sqlite3.ErrCorrupt:
			return true, Unreachable
		}
	}
	return false, NoFailure
}

// connectFailure classifies the outcome of a connect or ping. Any error counts, unless
// the caller cancelled the operation.
func connectFailure(err error) (bool, FailureKind) {
	if err == nil || errors.Is(err, context.Canceled) {
		return false, NoFailure
	}
	return true, kindOf(err)
}

// kindOf reports whether a connection failure was caused by a deadline or a network
// timeout.
func kindOf(err error) FailureKind {
	if errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err) {
		return Timeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return Timeout
	}
	return Unreachable
}

// isPostgresConnectionState reports whether a PostgreSQL SQLSTATE means the connection
// cannot be used: a connection exception (class 08), an authorization failure (class 28),
// or the server shutting down or not accepting connections (57P01 to 57P03).
func isPostgresConnectionState(code string) bool {
	switch {
	case strings.HasPrefix(code, "08"), strings.HasPrefix(code, "28"):
		return true
	case code == "57P01", code == "57P02", code == "57P03":
		return true
	}
	return false
}
