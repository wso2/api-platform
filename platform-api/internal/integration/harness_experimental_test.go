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

//go:build integration && experimental

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
	it := openITDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := it.db.InitSchema("../../plugins/eventgateway/schema/schema.sql", logger); err != nil {
		t.Fatalf("InitSchema experimental (%s) failed: %v", it.driver, err)
	}
	return it
}
