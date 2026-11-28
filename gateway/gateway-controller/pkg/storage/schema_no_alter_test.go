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

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// alterStatement matches an ALTER TABLE outside a comment.
var alterStatement = regexp.MustCompile(`(?i)^\s*ALTER\s+TABLE\b`)

// knownAlters records schema files still carrying an ALTER. It is empty: the
// certificate columns that used to sit at the bottom of the Postgres and SQL
// Server files now live in
// migrations/2026.10.0.1/020_certificate_tls_columns.{postgres,sqlserver}.sql.
//
// Keep it empty. An entry here is a violation waiting to be cleaned up, not a
// place to park a new one.
var knownAlters = map[string]int{}

// TestSchemaFilesDeclareOnly enforces R11-NO-ALTER: a schema file is the
// declarative definition of what a fresh install gets, so it contains CREATE
// and nothing else. An ALTER there applies only to a database that already
// exists, re-runs on every startup for every version, and cannot tell an
// install from an upgrade. Column changes belong in a migration step.
func TestSchemaFilesDeclareOnly(t *testing.T) {
	for _, file := range schemaFiles {
		b, err := os.ReadFile(file)
		assert.NilError(t, err)

		var found []string
		for i, line := range strings.Split(string(b), "\n") {
			if alterStatement.MatchString(line) {
				found = append(found, fmt.Sprintf("line %d: %s", i+1, strings.TrimSpace(line)))
			}
		}

		base := filepath.Base(file)
		allowed := knownAlters[base]
		if len(found) == allowed {
			continue
		}
		if len(found) < allowed {
			t.Errorf("%s now has %d ALTER statements, fewer than the %d recorded in knownAlters.\n"+
				"Lower the count in knownAlters (or delete the entry) in the same change.", base, len(found), allowed)
			continue
		}
		sort.Strings(found)
		t.Errorf("%s has %d ALTER statements, %d more than allowed (R11-NO-ALTER).\n"+
			"A schema file is CREATE only. Put the column change in a migration step under\n"+
			"pkg/storage/migrations/<release>/ instead.\n  %s",
			base, len(found), len(found)-allowed, strings.Join(found, "\n  "))
	}
}
