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
	"crypto/tls"
	"crypto/x509"
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
	// mssqlCannotOpenDatabase is a login that cannot open the database it asked for. The
	// server also returns it, with 18456, while that database is still recovering after
	// a restart, so it is never treated as a misconfiguration.
	mssqlCannotOpenDatabase = 4060
)

// IsMisconfiguration reports whether err, from connecting to the database, is one that
// retrying cannot fix because the connection settings are wrong: the server rejected
// the credentials (PostgreSQL class 28, SQL Server 18456), the database does not exist
// (PostgreSQL 3D000), or the server's TLS certificate failed verification. An
// unreachable host, a refused connection or a timeout is not: the database may simply
// not be up yet. Nor is a SQL Server login failure that comes with 4060: SQL Server
// reports a database still recovering after a restart that way, as well as one that
// does not exist, so startup keeps waiting for it.
func IsMisconfiguration(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "28") || pgErr.Code == "3D000"
	}
	var msErr mssql.Error
	if errors.As(err, &msErr) {
		return msErr.SQLErrorNumber() == mssqlLoginFailed && !mssqlReports(msErr, mssqlCannotOpenDatabase)
	}
	var verifyErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	if errors.As(err, &verifyErr) || errors.As(err, &unknownAuthority) ||
		errors.As(err, &hostnameErr) || errors.As(err, &invalidCert) {
		return true
	}
	return isMSSQLCertificateRejected(err)
}

// The SQL Server driver formats the TLS handshake error of an encrypted (non-strict)
// login with %v, which drops the certificate error's type, so its text is matched
// instead: the driver's prefix followed by Go's certificate verification failure.
const (
	mssqlTLSHandshakeFailed = "TLS Handshake failed: "
	tlsCertificateRejected  = "tls: failed to verify certificate"
)

// isMSSQLCertificateRejected reports whether err is the SQL Server driver's TLS
// handshake failure caused by the server's certificate failing verification.
func isMSSQLCertificateRejected(err error) bool {
	msg := err.Error()
	i := strings.Index(msg, mssqlTLSHandshakeFailed)
	return i >= 0 && strings.Contains(msg[i:], tlsCertificateRejected)
}

// mssqlReports reports whether SQL Server returned error number n in the same response
// as err. The driver surfaces the last error of a response and keeps every error in All.
func mssqlReports(err mssql.Error, n int32) bool {
	if err.Number == n {
		return true
	}
	for _, e := range err.All {
		if e.Number == n {
			return true
		}
	}
	return false
}

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
