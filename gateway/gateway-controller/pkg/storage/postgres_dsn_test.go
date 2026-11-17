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

package storage

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The startup log names the connection target from the parsed DSN and never logs the
// DSN itself, whatever form carries the password.
func TestOpenPostgresLogsTargetWithoutPassword(t *testing.T) {
	for _, dsn := range []string{
		"postgres://gw:s3cret@db.invalid:5433/gateway",
		"postgres://:s3cret@db.invalid:5433/gateway?user=gw",
		"postgres://db.invalid:5433/gateway?user=gw&password=s3cret",
		`host=db.invalid port=5433 user=gw password=a\ s3cret dbname=gateway`,
		"host=db.invalid port=5433 user=gw password=s3cret application_name=https://probe dbname=gateway",
	} {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		store, err := Open(BackendConfig{Type: "postgres", Postgres: PostgresConnectionConfig{DSN: dsn}}, logger)
		require.NoError(t, err, dsn)
		_ = store.Close()

		logged := buf.String()
		assert.Contains(t, logged, "PostgreSQL storage initialized", dsn)
		assert.Contains(t, logged, "host=db.invalid port=5433 database=gateway user=gw", dsn)
		assert.NotContains(t, logged, "s3cret", dsn)
		assert.NotContains(t, logged, "dsn=", dsn)
	}
}

// An invalid DSN must fail when the storage is opened, not on the first connection,
// so startup does not keep retrying a configuration that can never work. The error
// must not carry the password, in whichever form the DSN gives it.
func TestOpenRejectsInvalidPostgresDSN(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, dsn := range []string{
		"postgres://gw:s3cret@db.invalid/gw?connect_timeout=invalid",
		"postgres://gw@db.invalid/gw?password=s3cret&connect_timeout=invalid",
		"host=db.invalid user=gw password=s3cret connect_timeout=invalid",
	} {
		_, err := Open(BackendConfig{Type: "postgres", Postgres: PostgresConnectionConfig{DSN: dsn}}, logger)
		require.Error(t, err, dsn)
		assert.Contains(t, err.Error(), "failed to parse postgres dsn", dsn)
		assert.NotContains(t, err.Error(), "s3cret", dsn)
	}
}
