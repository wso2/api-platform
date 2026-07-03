//go:build integration && experimental

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

import (
	"io"
	"log/slog"
	"testing"
)

// openExperimentalITDB opens the test database with the core schema applied
// (via openITDB) and then additionally applies the eventgateway plugin schema,
// making websub_apis, websub_api_hmac_secrets, and webbroker_apis tables
// available. Use this in tests that exercise experimental WebSub/WebBroker
// functionality.
func openExperimentalITDB(t *testing.T) *itDB {
	t.Helper()
	it, err := openITDB()
	if err != nil {
		t.Fatalf("openITDB failed: %v", err)
	}
	t.Cleanup(it.cleanup)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := it.db.InitSchema("../../plugins/eventgateway/schema/schema.sql", logger); err != nil {
		t.Fatalf("InitSchema experimental (%s) failed: %v", it.driver, err)
	}
	return it
}

// execT runs a `?`-placeholder statement and fails the test on error. It adapts
// the godog-style itDB.exec (which returns an error) to the traditional
// testing.T flow used by the experimental tests.
func (it *itDB) execT(t *testing.T, query string, args ...any) {
	t.Helper()
	if err := it.exec(query, args...); err != nil {
		t.Fatalf("%v", err)
	}
}

// countT returns the matching row count and fails the test on error.
func (it *itDB) countT(t *testing.T, table, col, val string) int {
	t.Helper()
	n, err := it.count(table, col, val)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return n
}
